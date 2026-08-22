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

// Compile-time check that Store implements store.Store interface.
var _ store.Store = (*Store)(nil)

// ── DTOs ──────────────────────────────────────────────────────────────────────

// Private types that mirror the JSON structure exactly.
// They are used only by Load and Save; no other package sees them.
// Entities are stored as flat, normalized arrays to act as the single
// source of truth, independent of the Registry's runtime map indexes.
// time.Time fields serialise as RFC 3339 automatically via encoding/json.
type registryDTO struct {
	Providers []providerDTO `json:"providers,omitempty"`
	Users     []userDTO     `json:"users,omitempty"`
	Projects  []projectDTO  `json:"projects,omitempty"`
	Accounts  []accountDTO  `json:"accounts,omitempty"`
	Sessions  []sessionDTO  `json:"sessions,omitempty"`
}

type providerDTO struct {
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
	UserEmail    string     `json:"user_email"`
	ProviderName string     `json:"provider_name"`
	PauseUntil   *time.Time `json:"pause_until,omitempty"`
	Projects     []string   `json:"projects,omitempty"`
}

type sessionDTO struct {
	UserEmail    string      `json:"user_email"`
	ProviderName string      `json:"provider_name"`
	ProjectName  string      `json:"project_name,omitempty"`
	Topic        string      `json:"topic,omitempty"`
	Queue        []promptDTO `json:"queue,omitempty"`
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

	for _, pdto := range dto.Providers {
		if err := reg.AddProvider(model.NewProvider(pdto.Name, pdto.HasProjects, pdto.HasDeepResearch)); err != nil {
			return nil, fmt.Errorf("store: load provider %q: %w", pdto.Name, err)
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
// flat DTO representation. It requires Users, Providers, and Projects to
// be fully populated in the Registry beforehand. This step also applies
// active pause states and assigns associated Projects to each Account.
func loadAccounts(dto *registryDTO, reg *model.Registry) error {
	for _, adto := range dto.Accounts {
		u := reg.User(adto.UserEmail)
		p := reg.Provider(adto.ProviderName)
		acc, err := reg.AddAccount(u, p)
		if err != nil {
			return fmt.Errorf("store: load account %q|%q: %w", adto.UserEmail, adto.ProviderName, err)
		}

		if adto.PauseUntil != nil && !adto.PauseUntil.IsZero() {
			if err := reg.PauseAccount(acc, *adto.PauseUntil); err != nil {
				return fmt.Errorf("store: restore pause state for account %q|%q: %w", adto.UserEmail, adto.ProviderName, err)
			}
			// acc must be re-read because PauseAccount replaces the registry's Account via Copy-on-Write
			acc = reg.Account(u, p)
		}

		for _, pName := range adto.Projects {
			proj := reg.Project(pName)
			if err := reg.AddProjectToAccount(acc, proj); err != nil {
				return fmt.Errorf("store: assign project %q to account %q|%q: %w", pName, adto.UserEmail, adto.ProviderName, err)
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
		p := reg.Provider(sdto.ProviderName)
		acc := reg.Account(u, p)
		if acc == nil {
			return fmt.Errorf("store: load session at index %d: account %q|%q not found", i, sdto.UserEmail, sdto.ProviderName)
		}

		var proj *model.Project
		if sdto.ProjectName != "" {
			proj = reg.Project(sdto.ProjectName)
			if proj == nil {
				return fmt.Errorf("store: load session at index %d: project %q not found", i, sdto.ProjectName)
			}
		}

		var queue []model.Prompt
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

// Save writes the registry to the JSON file atomically.
// The data is written to a temporary file in the same directory and renamed
// into place. An interrupted write therefore never corrupts the existing file.
// The parent directory is created if it does not yet exist.
func (s *Store) Save(reg *model.Registry) error {
	dto, err := buildDTO(reg)
	if err != nil {
		return err
	}
	data, err := stdjson.MarshalIndent(dto, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("store: create directory for %q: %w", s.path, err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("store: write %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		err = fmt.Errorf("store: rename %q to %q: %w", tmp, s.path, err)
		if rmErr := os.Remove(tmp); rmErr != nil {
			err = errors.Join(err, rmErr)
		}
		return err
	}
	return nil
}

// buildDTO constructs the flat JSON DTOs using the Registry's deterministic getters.
// buildDTO constructs the flat JSON DTOs using the Registry's getters,
// explicitly sorting all slices to guarantee deterministic output and
// stable session ID reconstruction.
func buildDTO(reg *model.Registry) (registryDTO, error) {
	var dto registryDTO

	for _, p := range reg.Providers() {
		dto.Providers = append(dto.Providers, providerDTO{
			Name:            p.Name(),
			HasProjects:     p.HasProjects(),
			HasDeepResearch: p.HasDeepResearch(),
		})
	}

	for _, u := range reg.Users() {
		dto.Users = append(dto.Users, userDTO{
			Email: u.Email(),
			Name:  u.Name(),
		})
	}

	for _, p := range reg.Projects() {
		dto.Projects = append(dto.Projects, projectDTO{
			Name:         p.Name(),
			Description:  p.Description(),
			Instructions: p.Instructions(),
		})
	}

	for _, acc := range reg.Accounts() {
		adto := accountDTO{
			UserEmail:    acc.UserEmail(),
			ProviderName: acc.ProviderName(),
		}

		if !acc.PauseUntil().IsZero() {
			t := acc.PauseUntil()
			adto.PauseUntil = &t
		}

		projects, err := reg.AccountProjects(acc)
		if err != nil {
			return registryDTO{}, fmt.Errorf("store: read projects for account %q|%q: %w", acc.UserEmail(), acc.ProviderName(), err)
		}
		for _, p := range projects {
			adto.Projects = append(adto.Projects, p.Name())
		}

		dto.Accounts = append(dto.Accounts, adto)
	}

	// reg.Sessions() guarantees deterministic slice ordering by ID.
	// Saving in this order ensures strict chronological re-creation on Load.
	for _, s := range reg.Sessions() {
		sdto := sessionDTO{
			UserEmail:    s.AccountKey().Email,
			ProviderName: s.AccountKey().Provider,
			ProjectName:  s.ProjectName(),
			Topic:        s.Topic(),
		}

		for _, prompt := range s.Queue() {
			sdto.Queue = append(sdto.Queue, promptDTO{
				Text:          prompt.Text(),
				SpawnsSession: prompt.SpawnsSession(),
			})
		}

		dto.Sessions = append(dto.Sessions, sdto)
	}

	return dto, nil
}
