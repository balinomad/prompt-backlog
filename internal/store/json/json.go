package json

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/store"
)

// Compile-time check that Store implements store.Store interface.
var _ store.Store = (*Store)(nil)

// ── DTOs ──────────────────────────────────────────────────────────────────────
// Private types that mirror the JSON structure exactly.
// They are used only by Load and Save; no other package sees them.
// time.Time fields serialise as RFC 3339 automatically via encoding/json.

type registryDTO struct {
	Providers map[string]providerDTO           `json:"providers"`
	Users     map[string]userDTO               `json:"users"`
	Sessions  map[string]map[string]sessionDTO `json:"sessions"`
	Projects  map[string]projectDTO            `json:"projects"`
}

type providerDTO struct {
	HasProjects     bool `json:"has_projects"`
	HasDeepResearch bool `json:"has_deep_research"`
}

type userDTO struct {
	Name string `json:"name"`
}

type sessionDTO struct {
	LastQuestion *lastQuestionDTO `json:"last_question,omitempty"`
	NextPromptAt *time.Time       `json:"next_prompt_at,omitempty"`
}

type lastQuestionDTO struct {
	ProjectName string `json:"project_name,omitempty"`
	Topic       string `json:"topic,omitempty"`
	MidAnswer   bool   `json:"mid_answer"`
	NextPrompt  string `json:"next_prompt,omitempty"`
}

// sessionRefDTO is a lightweight reference to a session by its natural keys.
type sessionRefDTO struct {
	UserEmail string `json:"user_email"`
	Provider  string `json:"provider"`
}

// projectDTO stores a project with an optional list of session associations.
// An empty or absent Sessions slice means the project is unattached.
type projectDTO struct {
	Sessions     []sessionRefDTO `json:"sessions,omitempty"`
	Description  string          `json:"description,omitempty"`
	Instructions string          `json:"instructions,omitempty"`
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
func (s *Store) Load() (model.Registry, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.NewRegistry(), nil
		}
		return model.Registry{}, fmt.Errorf("store: read %q: %w", s.path, err)
	}
	if len(data) == 0 {
		return model.Registry{}, fmt.Errorf("store: %q is empty (possible failed write)", s.path)
	}

	var dto registryDTO
	if err := stdjson.Unmarshal(data, &dto); err != nil {
		return model.Registry{}, fmt.Errorf("store: parse %q: %w", s.path, err)
	}

	return buildRegistry(dto)
}

// buildRegistry constructs a fully resolved Registry from a DTO.
// Resolution order is mandatory:
//
//  1. Providers       — no dependencies
//  2. Users           — no dependencies
//  3. Sessions        — depend on Providers and Users
//  4. Projects        — depend on Sessions (for attachment)
//  5. LastQuestions   — depend on Projects; second pass over sessions
func buildRegistry(dto registryDTO) (model.Registry, error) {
	reg := model.NewRegistry()

	providers, err := resolveProviders(dto, &reg)
	if err != nil {
		return model.Registry{}, err
	}
	users, err := resolveUsers(dto, &reg)
	if err != nil {
		return model.Registry{}, err
	}
	sessions, err := resolveSessions(dto, &reg, providers, users)
	if err != nil {
		return model.Registry{}, err
	}
	projects, err := resolveProjects(dto, &reg, sessions)
	if err != nil {
		return model.Registry{}, err
	}
	if err := resolveLastQuestions(dto, sessions, projects); err != nil {
		return model.Registry{}, err
	}
	return reg, nil
}

func resolveProviders(dto registryDTO, reg *model.Registry) (map[string]*model.Provider, error) {
	index := make(map[string]*model.Provider, len(dto.Providers))
	for name, pdto := range dto.Providers {
		p := model.NewProvider(name, pdto.HasProjects, pdto.HasDeepResearch)
		if err := reg.AddProvider(p); err != nil {
			return nil, fmt.Errorf("store: load providers: %w", err)
		}
		index[name] = p
	}
	return index, nil
}

func resolveUsers(dto registryDTO, reg *model.Registry) (map[string]*model.User, error) {
	index := make(map[string]*model.User, len(dto.Users))
	for email, udto := range dto.Users {
		u := model.NewUser(email, udto.Name)
		if err := reg.AddUser(u); err != nil {
			return nil, fmt.Errorf("store: load users: %w", err)
		}
		index[email] = u
	}
	return index, nil
}

func resolveSessions(
	dto registryDTO,
	reg *model.Registry,
	providers map[string]*model.Provider,
	users map[string]*model.User,
) (map[string]map[string]*model.Session, error) {
	index := make(map[string]map[string]*model.Session)
	for userEmail, providerSessions := range dto.Sessions {
		user := users[userEmail]
		if user == nil {
			return nil, fmt.Errorf("store: session references unknown user %q", userEmail)
		}
		for providerName, sdto := range providerSessions {
			provider := providers[providerName]
			if provider == nil {
				return nil, fmt.Errorf("store: session references unknown provider %q", providerName)
			}
			s := model.NewSession(user, provider)
			s.NextPromptAt = sdto.NextPromptAt
			// LastQuestion is completed in resolveLastQuestions after projects are loaded.
			if err := reg.AddSession(s); err != nil {
				return nil, fmt.Errorf("store: load sessions: %w", err)
			}
			if index[userEmail] == nil {
				index[userEmail] = make(map[string]*model.Session)
			}
			index[userEmail][providerName] = s
		}
	}
	return index, nil
}

// resolveProjects adds each project to the registry and attaches it to every
// session listed in its Sessions slice.
func resolveProjects(
	dto registryDTO,
	reg *model.Registry,
	sessions map[string]map[string]*model.Session,
) (map[string]*model.Project, error) {
	index := make(map[string]*model.Project, len(dto.Projects))
	for name, pdto := range dto.Projects {
		p := model.NewProject(name)
		p.Description = pdto.Description
		p.Instructions = pdto.Instructions
		if err := reg.AddProject(p); err != nil {
			return nil, fmt.Errorf("store: load projects: %w", err)
		}
		for _, ref := range pdto.Sessions {
			if ref.UserEmail == "" || ref.Provider == "" {
				return nil, fmt.Errorf(
					"store: project %q has session reference with empty user_email or provider",
					name)
			}
			session := sessions[ref.UserEmail][ref.Provider]
			if session == nil {
				return nil, fmt.Errorf(
					"store: project %q references unknown session %q/%q",
					name, ref.UserEmail, ref.Provider)
			}
			if err := reg.AttachProject(name, session); err != nil {
				return nil, fmt.Errorf("store: attach project %q to %q/%q: %w",
					name, ref.UserEmail, ref.Provider, err)
			}
		}
		index[name] = p
	}
	return index, nil
}

// resolveLastQuestions performs a second pass over sessions to build each
// LastQuestion with its fully resolved project pointer.
// Sessions with no last_question in the DTO are left with LastQuestion nil.
func resolveLastQuestions(
	dto registryDTO,
	sessions map[string]map[string]*model.Session,
	projects map[string]*model.Project,
) error {
	for userEmail, providerSessions := range dto.Sessions {
		for providerName, sdto := range providerSessions {
			if sdto.LastQuestion == nil {
				continue
			}
			lqdto := sdto.LastQuestion
			var project *model.Project
			if lqdto.ProjectName != "" {
				project = projects[lqdto.ProjectName]
				if project == nil {
					return fmt.Errorf(
						"store: last_question in session %q/%q references unknown project %q",
						userEmail, providerName, lqdto.ProjectName)
				}
			}
			lq := model.NewLastQuestion(project)
			lq.Topic = lqdto.Topic
			lq.MidAnswer = lqdto.MidAnswer
			lq.NextPrompt = lqdto.NextPrompt
			sessions[userEmail][providerName].LastQuestion = lq
		}
	}
	return nil
}

// ── Save ──────────────────────────────────────────────────────────────────────

// Save writes the registry to the JSON file atomically.
// The data is written to a temporary file in the same directory and renamed
// into place. An interrupted write therefore never corrupts the existing file.
// The parent directory is created if it does not yet exist.
func (s *Store) Save(reg model.Registry) error {
	dto := buildDTO(reg)
	data, err := stdjson.MarshalIndent(dto, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("store: create directory for %q: %w", s.path, err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
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

func buildDTO(reg model.Registry) registryDTO {
	return registryDTO{
		Providers: buildProvidersDTO(reg),
		Users:     buildUsersDTO(reg),
		Sessions:  buildSessionsDTO(reg),
		Projects:  buildProjectsDTO(reg),
	}
}

func buildProvidersDTO(reg model.Registry) map[string]providerDTO {
	providers := reg.AllProviders()
	out := make(map[string]providerDTO, len(providers))
	for _, p := range providers {
		out[p.Name()] = providerDTO{
			HasProjects:     p.HasProjects,
			HasDeepResearch: p.HasDeepResearch,
		}
	}
	return out
}

func buildUsersDTO(reg model.Registry) map[string]userDTO {
	users := reg.AllUsers()
	out := make(map[string]userDTO, len(users))
	for _, u := range users {
		out[u.Email()] = userDTO{Name: u.Name}
	}
	return out
}

func buildSessionsDTO(reg model.Registry) map[string]map[string]sessionDTO {
	out := make(map[string]map[string]sessionDTO)
	for _, s := range reg.AllSessions() {
		userEmail := s.User().Email()
		providerName := s.Provider().Name()
		if out[userEmail] == nil {
			out[userEmail] = make(map[string]sessionDTO)
		}
		out[userEmail][providerName] = buildSessionDTO(s)
	}
	return out
}

func buildSessionDTO(s *model.Session) sessionDTO {
	sdto := sessionDTO{NextPromptAt: s.NextPromptAt}
	if s.LastQuestion == nil {
		return sdto
	}
	lqdto := &lastQuestionDTO{
		Topic:      s.LastQuestion.Topic,
		MidAnswer:  s.LastQuestion.MidAnswer,
		NextPrompt: s.LastQuestion.NextPrompt,
	}
	if p := s.LastQuestion.Project(); p != nil {
		lqdto.ProjectName = p.Name()
	}
	sdto.LastQuestion = lqdto
	return sdto
}

func buildProjectsDTO(reg model.Registry) map[string]projectDTO {
	projects := reg.AllProjects()
	out := make(map[string]projectDTO, len(projects))
	for _, p := range projects {
		sessions := reg.SessionsByProject(p.Name())

		refs := make([]sessionRefDTO, 0, len(sessions))
		for _, s := range sessions {
			refs = append(refs, sessionRefDTO{
				UserEmail: s.User().Email(),
				Provider:  s.Provider().Name(),
			})
		}

		sort.Slice(refs, func(i, j int) bool {
			if refs[i].UserEmail != refs[j].UserEmail {
				return refs[i].UserEmail < refs[j].UserEmail
			}
			return refs[i].Provider < refs[j].Provider
		})

		pdto := projectDTO{
			Description:  p.Description,
			Instructions: p.Instructions,
		}
		if len(refs) > 0 {
			pdto.Sessions = refs
		}
		out[p.Name()] = pdto
	}
	return out
}
