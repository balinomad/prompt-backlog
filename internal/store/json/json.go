// Package json implements store.Store using JSON files.
package json

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/store"
)

const (
	jsonIndent = "  "
	dirPerm    = 0o700
	filePerm   = 0o600
	tempSuffix = ".tmp"
)

// Compile-time check that Store implements store.Store interface.
var _ store.Store = (*Store)(nil)

// ── DTOs ──────────────────────────────────────────────────────────────────────

// Private types that mirror the JSON structure exactly.
// They are used only by Load and Save; no other package sees them.
// Entities are stored as flat, normalized arrays to act as the single
// source of truth, independent of the Registry's runtime map indexes.
// time.Time fields serialise as RFC 3339 automatically via encoding/json.
type registryDTO struct {
	Vendors  []vendorDTO  `json:"vendors,omitempty"`
	Users    []userDTO    `json:"users,omitempty"`
	Projects []projectDTO `json:"projects,omitempty"`
	Accounts []accountDTO `json:"accounts,omitempty"`
	Sessions []sessionDTO `json:"sessions,omitempty"`
}

type vendorDTO struct {
	Name            string `json:"name"`
	HasProjects     bool   `json:"has_projects"`
	HasDeepResearch bool   `json:"has_deep_research"`
}

type userDTO struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type projectDTO struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

type accountDTO struct {
	UserEmail  string     `json:"user_email"`
	VendorName string     `json:"vendor_name"`
	PauseUntil *time.Time `json:"pause_until,omitempty"`
	Projects   []string   `json:"projects,omitempty"`
}

type sessionDTO struct {
	UserEmail   string      `json:"user_email"`
	VendorName  string      `json:"vendor_name"`
	ProjectName string      `json:"project_name,omitempty"`
	Topic       string      `json:"topic,omitempty"`
	Queue       []promptDTO `json:"queue,omitempty"`
}

type promptDTO struct {
	Text          string `json:"text"`
	SpawnsSession bool   `json:"spawns_session"`
}

// ── Store ─────────────────────────────────────────────────────────────────────

// Store persists the registry as a human-readable, indented JSON file.
// All writes are atomic: data is written to a temporary file in the same
// directory and renamed into place, so an interrupted write never corrupts
// the existing file.
type Store struct {
	path string
}

// New returns a Store that reads from and writes to path.
func New(path string) *Store {
	return &Store{path: path}
}

// ── Load ──────────────────────────────────────────────────────────────────────

// Load reads the registry from the JSON file.
// When the file does not exist, Load returns an empty Registry and no error;
// this is the expected first-run state.
// A zero-byte file is treated as a failed write and returns an error.
// All other I/O and parse failures are returned with context.
func (s *Store) Load() (*model.Registry, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.NewRegistry(), nil
		}
		return model.NewRegistry(), fmt.Errorf("store: read %q: %w", s.path, err)
	}
	if len(data) == 0 {
		return model.NewRegistry(), fmt.Errorf("store: %q is empty (possible failed write)", s.path)
	}

	var dto registryDTO
	if err := stdjson.Unmarshal(data, &dto); err != nil {
		return model.NewRegistry(), fmt.Errorf("store: parse %q: %w", s.path, err)
	}

	return buildRegistry(&dto)
}

// buildRegistry constructs a fully resolved Registry from flat DTO arrays.
// Resolution order is mandatory: base entities first, join entities second,
// sessions last. The Registry builds its internal maps automatically as
// Add* methods are called.
func buildRegistry(dto *registryDTO) (*model.Registry, error) {
	reg := model.NewRegistry()

	for _, pdto := range dto.Vendors {
		if err := reg.AddVendor(model.NewVendor(pdto.Name, pdto.HasProjects, pdto.HasDeepResearch)); err != nil {
			return nil, fmt.Errorf("store: load vendor %q: %w", pdto.Name, err)
		}
	}

	for _, udto := range dto.Users {
		if err := reg.AddUser(model.NewUser(udto.Email, udto.Name)); err != nil {
			return nil, fmt.Errorf("store: load user %q: %w", udto.Email, err)
		}
	}

	for _, pdto := range dto.Projects {
		if err := reg.AddProject(model.NewProject(pdto.Name, pdto.Description, pdto.Instructions)); err != nil {
			return nil, fmt.Errorf("store: load project %q: %w", pdto.Name, err)
		}
	}

	if err := loadAccounts(dto, reg); err != nil {
		return nil, err
	}
	if err := loadSessions(dto, reg); err != nil {
		return nil, err
	}

	return reg, nil
}

// loadAccounts resolves and attaches Accounts to the Registry using the
// flat DTO representation. It requires Users, Vendors, and Projects to
// be fully populated in the Registry beforehand. This step also applies
// active pause states and assigns associated Projects to each Account.
func loadAccounts(dto *registryDTO, reg *model.Registry) error {
	for _, adto := range dto.Accounts {
		u := reg.User(adto.UserEmail)
		p := reg.Vendor(adto.VendorName)
		acc, err := reg.AddAccount(u, p)
		if err != nil {
			return fmt.Errorf("store: load account %q|%q: %w", adto.UserEmail, adto.VendorName, err)
		}

		if adto.PauseUntil != nil && !adto.PauseUntil.IsZero() {
			if err := reg.PauseAccount(acc, *adto.PauseUntil); err != nil {
				return fmt.Errorf("store: restore pause state for account %q|%q: %w", adto.UserEmail, adto.VendorName, err)
			}
			// acc must be re-read because PauseAccount replaces the registry's Account via Copy-on-Write
			acc = reg.Account(u, p)
		}

		for _, pName := range adto.Projects {
			proj := reg.Project(pName)
			if err := reg.AddProjectToAccount(acc, proj); err != nil {
				return fmt.Errorf("store: assign project %q to account %q|%q: %w", pName, adto.UserEmail, adto.VendorName, err)
			}
		}
	}
	return nil
}

// loadSessions resolves and appends Sessions to their respective Accounts
// within the Registry. It requires Accounts and Projects to be fully
// populated beforehand. This step converts flat prompt queues into model
// instances and triggers ephemeral ID assignment for each Session.
func loadSessions(dto *registryDTO, reg *model.Registry) error {
	for i, sdto := range dto.Sessions {
		u := reg.User(sdto.UserEmail)
		p := reg.Vendor(sdto.VendorName)
		acc := reg.Account(u, p)
		if acc == nil {
			return fmt.Errorf("store: load session at index %d: account %q|%q not found", i, sdto.UserEmail, sdto.VendorName)
		}

		var proj *model.Project
		if sdto.ProjectName != "" {
			proj = reg.Project(sdto.ProjectName)
			if proj == nil {
				return fmt.Errorf("store: load session at index %d: project %q not found", i, sdto.ProjectName)
			}
		}

		var queue []model.Prompt
		if len(sdto.Queue) > 0 {
			queue = make([]model.Prompt, 0, len(sdto.Queue))
		}
		for _, prompt := range sdto.Queue {
			queue = append(queue, model.NewPrompt(prompt.Text, prompt.SpawnsSession))
		}

		// AddSession automatically assigns the next incrementing ephemeral ID.
		if _, err := reg.AddSession(acc, proj, sdto.Topic, queue); err != nil {
			return fmt.Errorf("store: load session at index %d: %w", i, err)
		}
	}
	return nil
}

// ── Save ──────────────────────────────────────────────────────────────────────

// Save writes the registry to the JSON file atomically and durably.
// The data is written to a temporary file in the same directory, synced, and
// renamed into place. An interrupted write therefore never corrupts the
// existing file. The parent directory is created if it does not yet exist.
func (s *Store) Save(reg *model.Registry) error {
	dto, err := buildDTO(reg)
	if err != nil {
		return err
	}
	data, err := stdjson.MarshalIndent(dto, "", jsonIndent)
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err = os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("store: create directory for %q: %w", s.path, err)
	}
	tmp := s.path + tempSuffix
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePerm) //nolint:gosec // tmp is derived from the configured store path
	if err != nil {
		return fmt.Errorf("store: write %q: %w", tmp, err)
	}
	if _, writeErr := f.Write(data); writeErr != nil {
		err = fmt.Errorf("store: write %q: %w", tmp, writeErr)
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return err
	}
	if syncErr := f.Sync(); syncErr != nil {
		err = fmt.Errorf("store: sync %q: %w", tmp, syncErr)
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return err
	}
	if closeErr := f.Close(); closeErr != nil {
		return fmt.Errorf("store: close %q: %w", tmp, closeErr)
	}
	if renameErr := os.Rename(tmp, s.path); renameErr != nil {
		err = fmt.Errorf("store: rename %q to %q: %w", tmp, s.path, renameErr)
		if rmErr := os.Remove(tmp); rmErr != nil {
			err = errors.Join(err, rmErr)
		}
		return err
	}
	if syncErr := syncDir(dir); syncErr != nil {
		return fmt.Errorf("store: sync directory: %w", syncErr)
	}
	return nil
}

// syncDir flushes the directory entry for dir to stable storage.
func syncDir(dir string) (err error) {
	d, err := os.Open(dir) //nolint:gosec // dir is derived from the configured store path
	if err != nil {
		return fmt.Errorf("open %q: %w", dir, err)
	}
	defer func() {
		if closeErr := d.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %q: %w", dir, closeErr))
		}
	}()
	if syncErr := d.Sync(); syncErr != nil {
		return fmt.Errorf("sync %q: %w", dir, syncErr)
	}
	return nil
}

// buildDTO constructs the flat JSON DTOs using the Registry's getters,
// explicitly sorting all slices to guarantee deterministic output and
// stable session ID reconstruction.
func buildDTO(reg *model.Registry) (registryDTO, error) {
	vendors := reg.Vendors()
	dto := registryDTO{
		Vendors: make([]vendorDTO, 0, len(vendors)),
	}
	for _, p := range vendors {
		dto.Vendors = append(dto.Vendors, vendorDTO{
			Name:            p.Name(),
			HasProjects:     p.HasProjects(),
			HasDeepResearch: p.HasDeepResearch(),
		})
	}

	users := reg.Users()
	dto.Users = make([]userDTO, 0, len(users))
	for _, u := range users {
		dto.Users = append(dto.Users, userDTO{
			Email: u.Email(),
			Name:  u.Name(),
		})
	}

	projects := reg.Projects()
	dto.Projects = make([]projectDTO, 0, len(projects))
	for _, p := range projects {
		dto.Projects = append(dto.Projects, projectDTO{
			Name:         p.Name(),
			Description:  p.Description(),
			Instructions: p.Instructions(),
		})
	}

	accounts := reg.Accounts()
	dto.Accounts = make([]accountDTO, 0, len(accounts))
	for _, acc := range accounts {
		adto := accountDTO{
			UserEmail:  acc.UserEmail(),
			VendorName: acc.VendorName(),
		}

		if !acc.PauseUntil().IsZero() {
			t := acc.PauseUntil()
			adto.PauseUntil = &t
		}

		accProjects, err := reg.AccountProjects(acc)
		if err != nil {
			return registryDTO{}, fmt.Errorf("store: read projects for account %q|%q: %w", acc.UserEmail(), acc.VendorName(), err)
		}
		adto.Projects = make([]string, 0, len(accProjects))
		for _, p := range accProjects {
			adto.Projects = append(adto.Projects, p.Name())
		}

		dto.Accounts = append(dto.Accounts, adto)
	}

	// reg.Sessions() guarantees deterministic slice ordering by ID.
	// Saving in this order ensures strict chronological re-creation on Load.
	sessions := reg.Sessions()
	dto.Sessions = make([]sessionDTO, 0, len(sessions))
	for _, s := range sessions {
		sdto := sessionDTO{
			UserEmail:   s.AccountKey().Email,
			VendorName:  s.AccountKey().Vendor,
			ProjectName: s.ProjectName(),
			Topic:       s.Topic(),
		}

		queue := s.Queue()
		sdto.Queue = make([]promptDTO, 0, len(queue))
		for _, prompt := range queue {
			sdto.Queue = append(sdto.Queue, promptDTO{
				Text:          prompt.Text(),
				SpawnsSession: prompt.SpawnsSession(),
			})
		}

		dto.Sessions = append(dto.Sessions, sdto)
	}

	return dto, nil
}
