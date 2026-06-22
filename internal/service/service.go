// Package service is the application layer between callers and model:
// it resolves primitive inputs — strings, bools, times — into model
// entities and delegates to model.Registry for storage, validation,
// and concurrency control. Errors from Registry are returned unchanged,
// so callers can match on model's sentinel errors with errors.Is.
package service

import (
	"fmt"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
)

// ── Service ───────────────────────────────────────────────────────────────────

// Service acts as the application layer, resolving primitive UI inputs
// into domain entities before delegating to the Registry.
type Service struct {
	reg *model.Registry
}

// New creates a new Service backed by the given Registry.
// Returns an error when the Registry is nil.
func New(reg *model.Registry) (*Service, error) {
	if reg == nil {
		return nil, fmt.Errorf("new service: registry is nil")
	}
	return &Service{reg: reg}, nil
}

// ── Provider ──────────────────────────────────────────────────────────────────

// AddProvider inserts a provider into the registry.
// Returns an error when the provider is nil, has an empty name, or already exists.
func (s *Service) AddProvider(name string, hasProjects, hasDeepResearch bool) error {
	return s.reg.AddProvider(model.NewProvider(name, hasProjects, hasDeepResearch))
}

// DeleteProvider removes a provider from the registry.
// Returns an error when the provider is not found or has associated accounts.
func (s *Service) DeleteProvider(name string) error {
	p := s.reg.Provider(name)
	if p == nil {
		return fmt.Errorf("delete provider: %q %w", name, model.ErrProviderNotFound)
	}
	return s.reg.DeleteProvider(p)
}

// UpdateProvider updates a provider's capabilities.
func (s *Service) UpdateProvider(name string, hasProjects, hasDeepResearch bool) error {
	current := s.reg.Provider(name)
	if current == nil {
		return fmt.Errorf("update provider: %q %w", name, model.ErrProviderNotFound)
	}
	return s.reg.UpdateProvider(current, model.NewProvider(name, hasProjects, hasDeepResearch))
}

// ── User ──────────────────────────────────────────────────────────────────────

// AddUser inserts a user into the registry.
// Returns an error when the user is nil, has an empty email, or already exists.
func (s *Service) AddUser(email, name string) error {
	return s.reg.AddUser(model.NewUser(email, name))
}

// DeleteUser removes a user from the registry.
// Returns an error when the user is not found or has associated accounts.
func (s *Service) DeleteUser(email string) error {
	u := s.reg.User(email)
	if u == nil {
		return fmt.Errorf("delete user: %q %w", email, model.ErrUserNotFound)
	}
	return s.reg.DeleteUser(u)
}

// UpdateUser updates a user's display name safely.
func (s *Service) UpdateUser(email, name string) error {
	current := s.reg.User(email)
	if current == nil {
		return fmt.Errorf("update user: %q %w", email, model.ErrUserNotFound)
	}
	return s.reg.UpdateUser(current, model.NewUser(email, name))
}

// ── Project ───────────────────────────────────────────────────────────────────

// AddProject inserts a project into the registry.
// Returns an error when the project has an empty name, or already exists.
func (s *Service) AddProject(name, description, instructions string) error {
	return s.reg.AddProject(model.NewProject(name, description, instructions))
}

// DeleteProject removes a project and cleans up all links.
// Returns an error if the project is assigned to any account or has active sessions.
func (s *Service) DeleteProject(name string) error {
	p := s.reg.Project(name)
	if p == nil {
		return fmt.Errorf("delete project: %q %w", name, model.ErrProjectNotFound)
	}
	return s.reg.DeleteProject(p)
}

// UpdateProject updates a project's configuration safely.
func (s *Service) UpdateProject(name, description, instructions string) error {
	current := s.reg.Project(name)
	if current == nil {
		return fmt.Errorf("update project: %q %w", name, model.ErrProjectNotFound)
	}
	return s.reg.UpdateProject(current, model.NewProject(name, description, instructions))
}

// ── Account ───────────────────────────────────────────────────────────────────

// AddAccount inserts an account into the registry.
// Returns an error if the user or provider is not registered in the registry.
func (s *Service) AddAccount(userEmail, providerName string) error {
	u := s.reg.User(userEmail)
	if u == nil {
		return fmt.Errorf("add account: %q %w", userEmail, model.ErrUserNotFound)
	}
	p := s.reg.Provider(providerName)
	if p == nil {
		return fmt.Errorf("add account: %q %w", providerName, model.ErrProviderNotFound)
	}
	_, err := s.reg.AddAccount(u, p)
	return err
}

// DeleteAccount removes an account.
// Returns an error if the account not found or has active sessions or assigned projects.
func (s *Service) DeleteAccount(userEmail, providerName string) error {
	acc, err := s.account(userEmail, providerName)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return s.reg.DeleteAccount(acc)
}

// PauseAccount pauses an account until the given time; no session under
// the account should be used before then.
func (s *Service) PauseAccount(userEmail, providerName string, until time.Time) error {
	acc, err := s.account(userEmail, providerName)
	if err != nil {
		return fmt.Errorf("pause account: %w", err)
	}
	return s.reg.PauseAccount(acc, until)
}

// AddProjectToAccount grants an account access to a specific project.
func (s *Service) AddProjectToAccount(userEmail, providerName, projectName string) error {
	acc, err := s.account(userEmail, providerName)
	if err != nil {
		return fmt.Errorf("add project to account: %w", err)
	}

	p := s.reg.Project(projectName)
	if p == nil {
		return fmt.Errorf("add project to account: project %q %w", projectName, model.ErrProjectNotFound)
	}

	return s.reg.AddProjectToAccount(acc, p)
}

// RemoveProjectFromAccount revokes an account's access to a specific project.
func (s *Service) RemoveProjectFromAccount(userEmail, providerName, projectName string) error {
	acc, err := s.account(userEmail, providerName)
	if err != nil {
		return fmt.Errorf("remove project from account: %w", err)
	}

	p := s.reg.Project(projectName)
	if p == nil {
		return fmt.Errorf("remove project from account: project %q %w", projectName, model.ErrProjectNotFound)
	}

	return s.reg.RemoveProjectFromAccount(acc, p)
}

// account performs existence checks for User and Provider before resolving the Account.
func (s *Service) account(userEmail, providerName string) (*model.Account, error) {
	u := s.reg.User(userEmail)
	if u == nil {
		return nil, fmt.Errorf("%q %w", userEmail, model.ErrUserNotFound)
	}
	p := s.reg.Provider(providerName)
	if p == nil {
		return nil, fmt.Errorf("%q %w", providerName, model.ErrProviderNotFound)
	}
	acc := s.reg.Account(u, p)
	if acc == nil {
		return nil, fmt.Errorf("account for user %q and provider %q: %w", userEmail, providerName, model.ErrAccountNotFound)
	}
	return acc, nil
}

// ── Session ───────────────────────────────────────────────────────────────────

// AddSession inserts a session into the registry, assigns it an ephemeral ID,
// and updates all relevant indexes.
func (s *Service) AddSession(userEmail, providerName, projectName, topic string, queue []model.Prompt) (*model.Session, error) {
	acc, err := s.account(userEmail, providerName)
	if err != nil {
		return nil, fmt.Errorf("add session: %w", err)
	}

	var p *model.Project
	if projectName != "" {
		p = s.reg.Project(projectName)
		if p == nil {
			return nil, fmt.Errorf("add session: project %q %w", projectName, model.ErrProjectNotFound)
		}
	}

	return s.reg.AddSession(acc, p, topic, queue)
}

// DeleteSession removes a session from an account and cleans up indexes.
func (s *Service) DeleteSession(userEmail, providerName string, id uint64) error {
	sess, err := s.session(userEmail, providerName, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return s.reg.DeleteSession(sess)
}

// UpdateSessionTopic updates the high-level topic of a session.
func (s *Service) UpdateSessionTopic(userEmail, providerName string, id uint64, topic string) error {
	sess, err := s.session(userEmail, providerName, id)
	if err != nil {
		return fmt.Errorf("update session topic: %w", err)
	}
	return s.reg.UpdateSessionTopic(sess, topic)
}

// UpdateSessionProject reassigns a session to a different project.
func (s *Service) UpdateSessionProject(userEmail, providerName string, id uint64, projectName string) error {
	sess, err := s.session(userEmail, providerName, id)
	if err != nil {
		return fmt.Errorf("update session project: %w", err)
	}

	var p *model.Project
	if projectName != "" {
		p = s.reg.Project(projectName)
		if p == nil {
			return fmt.Errorf("update session project: project %q %w", projectName, model.ErrProjectNotFound)
		}
	}

	return s.reg.UpdateSessionProject(sess, p)
}

// UpdateSessionPrompts overwrites the pending prompt queue for a session.
func (s *Service) UpdateSessionPrompts(userEmail, providerName string, id uint64, queue []model.Prompt) error {
	sess, err := s.session(userEmail, providerName, id)
	if err != nil {
		return fmt.Errorf("update session prompts: %w", err)
	}
	return s.reg.UpdateSessionPrompts(sess, queue)
}

// session returns the session for the given ID, verifying it belongs to the given user and provider.
func (s *Service) session(userEmail, providerName string, id uint64) (*model.Session, error) {
	acc, err := s.account(userEmail, providerName)
	if err != nil {
		return nil, err
	}
	sess := s.reg.Session(id)
	if sess == nil {
		return nil, fmt.Errorf("session %d: %w", id, model.ErrSessionNotFound)
	}
	if sess.AccountKey() != acc.Key() {
		// Return a generic not found error to avoid leaking existence of IDs belonging to other users.
		return nil, fmt.Errorf("session %d: %w", id, model.ErrNotFound)
	}
	return sess, nil
}
