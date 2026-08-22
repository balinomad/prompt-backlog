// Package service is the application layer between callers and model: it
// resolves primitive inputs — strings, bools, times — into model entities
// and delegates to model.Registry for storage, validation, and concurrency
// control where an operation maps onto exactly one Registry call. Errors
// from Registry are returned unchanged in that case, so callers can match
// on model's sentinel errors with errors.Is; the one category-only check
// in this file (resolveSession's cross-account guard) matches on
// internal/apperr directly instead, since model.ErrNotFound carries no
// information a bare category check needs.
//
// Validation is not duplicated here: model.Validate* remains the single
// enforcement point — every Registry.Add*/Update* call already runs it,
// regardless of caller. The ValidateX methods below are read-only
// passthroughs to those same functions, for UI forms that want pre-flight
// feedback before attempting a write; they check exactly what model.ValidateX
// checks, no more (uniqueness is still Add*'s job, composable separately
// via the read methods below).
//
// Two methods, in "Workflows" at the end of this file, are not 1:1
// wrappers: StartSession and CreateAccount both compose more than one
// Registry call, each creating the user on demand via the shared
// resolveOrCreateUser helper — because the caller has no reason to know
// whether the user already exists. They diverge on the account itself:
// StartSession resolves-or-creates it, since the caller only wants a
// session and doesn't care whether the account backing it is new;
// CreateAccount requires it be newly created, since that is the explicit
// action the caller invoked. That composition is this package's actual
// reason to exist as a layer distinct from Registry; everywhere else,
// Service is deliberately thin, and that thinness is correct, not a
// shortcut.
package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/balinomad/llm-desk/internal/apperr"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/store"
)

// ── Service ───────────────────────────────────────────────────────────────────

// Service acts as the application layer, resolving primitive UI inputs
// into domain entities before delegating to the Registry. It owns the
// full persistence lifecycle: New loads, Save persists. Mutating methods
// never call Save themselves — the caller decides when a batch of calls
// is committed, exactly as before; only the storage dependency moved.
type Service struct {
	reg *model.Registry
	st  store.Store
}

// New loads the registry from st and returns a ready-to-use Service.
// Returns an error when st is nil or the load fails.
func New(st store.Store) (*Service, error) {
	if st == nil {
		return nil, fmt.Errorf("new service: store %w", apperr.ErrNil)
	}
	reg, err := st.Load()
	if err != nil {
		return nil, fmt.Errorf("new service: load: %w", err)
	}
	if reg == nil {
		return nil, errors.New("new service: store returned a nil registry")
	}
	return &Service{
		reg: reg,
		st:  st,
	}, nil
}

// Save persists the current registry state. Call this once after a batch
// of mutating calls succeeds — not after each individual call — so a
// single UI action that touches more than one aggregate still produces
// exactly one write.
func (s *Service) Save() error {
	if err := s.st.Save(s.reg); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

// ── Resolution helpers (private) ───────────────────────────────────────────────

// resolveProvider resolves name to its current *model.Provider, or returns
// a "<verb> provider: ... not found" error when it does not exist.
func (s *Service) resolveProvider(verb, name string) (*model.Provider, error) {
	p := s.reg.Provider(name)
	if p == nil {
		return nil, fmt.Errorf("%s provider: %q %w", verb, name, model.ErrProviderNotFound)
	}
	return p, nil
}

// resolveUser resolves email to its current *model.User, or returns a
// "<verb> user: ... not found" error when it does not exist.
func (s *Service) resolveUser(verb, email string) (*model.User, error) {
	u := s.reg.User(email)
	if u == nil {
		return nil, fmt.Errorf("%s user: %q %w", verb, email, model.ErrUserNotFound)
	}
	return u, nil
}

// resolveProject resolves name to its current *model.Project, or returns
// a "<verb> project: ... not found" error when it does not exist.
func (s *Service) resolveProject(verb, name string) (*model.Project, error) {
	p := s.reg.Project(name)
	if p == nil {
		return nil, fmt.Errorf("%s project: %q %w", verb, name, model.ErrProjectNotFound)
	}
	return p, nil
}

// resolveAccount resolves (userEmail, providerName) to its current
// *model.Account, checking user and provider existence first so the error
// names whichever is actually missing.
func (s *Service) resolveAccount(userEmail, providerName string) (*model.Account, error) {
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

// resolveSession resolves id to its current *model.Session, verifying it
// belongs to the account for (userEmail, providerName).
func (s *Service) resolveSession(userEmail, providerName string, id uint64) (*model.Session, error) {
	acc, err := s.resolveAccount(userEmail, providerName)
	if err != nil {
		return nil, err
	}
	sess := s.reg.Session(id)
	if sess == nil {
		return nil, fmt.Errorf("session %d: %w", id, model.ErrSessionNotFound)
	}
	if sess.AccountKey() != acc.Key() {
		// Generic not-found, not a permission error: avoid confirming that
		// an ID belonging to a different account exists at all.
		return nil, fmt.Errorf("session %d: %w", id, apperr.ErrNotFound)
	}
	return sess, nil
}

// ── Provider ──────────────────────────────────────────────────────────────────

// Provider returns the provider with the given name, or nil if not found.
func (s *Service) Provider(name string) *model.Provider { return s.reg.Provider(name) }

// Providers returns all providers sorted by name.
func (s *Service) Providers() []*model.Provider { return s.reg.Providers() }

// ValidateProvider reports whether (name, hasProjects, hasDeepResearch)
// would form a valid provider, without inserting anything.
func (s *Service) ValidateProvider(name string, hasProjects, hasDeepResearch bool) error {
	return model.ValidateProvider(model.NewProvider(name, hasProjects, hasDeepResearch))
}

// AddProvider inserts a provider into the registry.
// Returns an error when the provider is nil, has an empty name, or already exists.
func (s *Service) AddProvider(name string, hasProjects, hasDeepResearch bool) error {
	return s.reg.AddProvider(model.NewProvider(name, hasProjects, hasDeepResearch))
}

// UpdateProvider updates a provider's capabilities.
func (s *Service) UpdateProvider(name string, hasProjects, hasDeepResearch bool) error {
	current, err := s.resolveProvider("update", name)
	if err != nil {
		return err
	}
	return s.reg.UpdateProvider(current, model.NewProvider(name, hasProjects, hasDeepResearch))
}

// DeleteProvider removes a provider from the registry.
// Returns an error when the provider is not found or has associated accounts.
func (s *Service) DeleteProvider(name string) error {
	p, err := s.resolveProvider("delete", name)
	if err != nil {
		return err
	}
	return s.reg.DeleteProvider(p)
}

// ── User ──────────────────────────────────────────────────────────────────────

// User returns the user with the given email, or nil if not found.
func (s *Service) User(email string) *model.User { return s.reg.User(email) }

// Users returns all users sorted by email.
func (s *Service) Users() []*model.User { return s.reg.Users() }

// ValidateUser reports whether (email, name) would form a valid user,
// without inserting anything.
func (s *Service) ValidateUser(email, name string) error {
	return model.ValidateUser(model.NewUser(email, name))
}

// AddUser inserts a user into the registry.
// Returns an error when the user is nil, has an empty email, or already exists.
func (s *Service) AddUser(email, name string) error {
	return s.reg.AddUser(model.NewUser(email, name))
}

// UpdateUser updates a user's display name safely.
func (s *Service) UpdateUser(email, name string) error {
	current, err := s.resolveUser("update", email)
	if err != nil {
		return err
	}
	return s.reg.UpdateUser(current, model.NewUser(email, name))
}

// DeleteUser removes a user from the registry.
// Returns an error when the user is not found or has associated accounts.
func (s *Service) DeleteUser(email string) error {
	u, err := s.resolveUser("delete", email)
	if err != nil {
		return err
	}
	return s.reg.DeleteUser(u)
}

// ── Project ───────────────────────────────────────────────────────────────────

// Project returns the project with the given name, or nil if not found.
func (s *Service) Project(name string) *model.Project { return s.reg.Project(name) }

// Projects returns all projects sorted by name.
func (s *Service) Projects() []*model.Project { return s.reg.Projects() }

// ProjectAccounts returns all accounts the named project is assigned to.
func (s *Service) ProjectAccounts(name string) ([]*model.Account, error) {
	p, err := s.resolveProject("resolve", name)
	if err != nil {
		return nil, err
	}
	return s.reg.ProjectAccounts(p)
}

// ProjectSessions returns all sessions attached to the named project.
func (s *Service) ProjectSessions(name string) ([]*model.Session, error) {
	p, err := s.resolveProject("resolve", name)
	if err != nil {
		return nil, err
	}
	return s.reg.ProjectSessions(p)
}

// ValidateProject reports whether (name, description, instructions) would
// form a valid project, without inserting anything.
func (s *Service) ValidateProject(name, description, instructions string) error {
	return model.ValidateProject(model.NewProject(name, description, instructions))
}

// AddProject inserts a project into the registry.
// Returns an error when the project has an empty name, or already exists.
func (s *Service) AddProject(name, description, instructions string) error {
	return s.reg.AddProject(model.NewProject(name, description, instructions))
}

// UpdateProject updates a project's configuration safely.
func (s *Service) UpdateProject(name, description, instructions string) error {
	current, err := s.resolveProject("update", name)
	if err != nil {
		return err
	}
	return s.reg.UpdateProject(current, model.NewProject(name, description, instructions))
}

// DeleteProject removes a project and cleans up all links.
// Returns an error if the project is assigned to any account or has active sessions.
func (s *Service) DeleteProject(name string) error {
	p, err := s.resolveProject("delete", name)
	if err != nil {
		return err
	}
	return s.reg.DeleteProject(p)
}

// ── Account ───────────────────────────────────────────────────────────────────

// Account returns the account for the given user and provider.
func (s *Service) Account(userEmail, providerName string) (*model.Account, error) {
	return s.resolveAccount(userEmail, providerName)
}

// Accounts returns all accounts sorted by composite key.
func (s *Service) Accounts() []*model.Account { return s.reg.Accounts() }

// AccountProjects returns all projects assigned to the account for the
// given user and provider.
func (s *Service) AccountProjects(userEmail, providerName string) ([]*model.Project, error) {
	acc, err := s.resolveAccount(userEmail, providerName)
	if err != nil {
		return nil, fmt.Errorf("account projects: %w", err)
	}
	return s.reg.AccountProjects(acc)
}

// AccountSessions returns all sessions for the account for the given user
// and provider.
func (s *Service) AccountSessions(userEmail, providerName string) ([]*model.Session, error) {
	acc, err := s.resolveAccount(userEmail, providerName)
	if err != nil {
		return nil, fmt.Errorf("account sessions: %w", err)
	}
	return s.reg.AccountSessions(acc)
}

// ValidateAccount reports whether (email, provider) would form a valid
// account, without inserting anything. Checks presence only — email
// format is validated separately when the underlying user is created
// (see model.ValidateUser via CreateAccount/StartSession).
func (s *Service) ValidateAccount(email, provider string) error {
	return model.ValidateAccount(model.NewAccount(email, provider))
}

// AddAccount inserts an account into the registry.
// Returns an error if the user or provider is not registered. This is the
// strict counterpart to StartSession's on-demand creation — use this when
// the user is expected to already exist, so a typo'd email fails loudly
// instead of silently registering a new person.
func (s *Service) AddAccount(userEmail, providerName string) error {
	u, err := s.resolveUser("add account", userEmail)
	if err != nil {
		return err
	}
	p, err := s.resolveProvider("add account", providerName)
	if err != nil {
		return err
	}
	_, err = s.reg.AddAccount(u, p)
	return err
}

// DeleteAccount removes an account.
// Returns an error if the account is not found or has active sessions or
// assigned projects.
func (s *Service) DeleteAccount(userEmail, providerName string) error {
	acc, err := s.resolveAccount(userEmail, providerName)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return s.reg.DeleteAccount(acc)
}

// PauseAccount pauses an account until the given time; no session under
// the account should be used before then.
func (s *Service) PauseAccount(userEmail, providerName string, until time.Time) error {
	acc, err := s.resolveAccount(userEmail, providerName)
	if err != nil {
		return fmt.Errorf("pause account: %w", err)
	}
	return s.reg.PauseAccount(acc, until)
}

// GrantProject gives an account access to a specific project.
func (s *Service) GrantProject(userEmail, providerName, projectName string) error {
	acc, err := s.resolveAccount(userEmail, providerName)
	if err != nil {
		return fmt.Errorf("grant project: %w", err)
	}
	p, err := s.resolveProject("grant", projectName)
	if err != nil {
		return err
	}
	return s.reg.AddProjectToAccount(acc, p)
}

// RevokeProject removes an account's access to a specific project.
func (s *Service) RevokeProject(userEmail, providerName, projectName string) error {
	acc, err := s.resolveAccount(userEmail, providerName)
	if err != nil {
		return fmt.Errorf("revoke project: %w", err)
	}
	p, err := s.resolveProject("revoke", projectName)
	if err != nil {
		return err
	}
	return s.reg.RemoveProjectFromAccount(acc, p)
}

// ── Session ───────────────────────────────────────────────────────────────────

// Session returns the session with the given ID, or nil if not found.
func (s *Service) Session(id uint64) *model.Session { return s.reg.Session(id) }

// Sessions returns all sessions sorted by ID.
func (s *Service) Sessions() []*model.Session { return s.reg.Sessions() }

// UpdateSessionTopic updates the high-level topic of a session.
func (s *Service) UpdateSessionTopic(userEmail, providerName string, id uint64, topic string) error {
	sess, err := s.resolveSession(userEmail, providerName, id)
	if err != nil {
		return fmt.Errorf("update session topic: %w", err)
	}
	return s.reg.UpdateSessionTopic(sess, topic)
}

// UpdateSessionProject reassigns a session to a different project.
func (s *Service) UpdateSessionProject(userEmail, providerName string, id uint64, projectName string) error {
	sess, err := s.resolveSession(userEmail, providerName, id)
	if err != nil {
		return fmt.Errorf("update session project: %w", err)
	}

	var p *model.Project
	if projectName != "" {
		p, err = s.resolveProject("resolve", projectName)
		if err != nil {
			return err
		}
	}

	return s.reg.UpdateSessionProject(sess, p)
}

// UpdateSessionPrompts overwrites the pending prompt queue for a session.
func (s *Service) UpdateSessionPrompts(userEmail, providerName string, id uint64, queue []model.Prompt) error {
	sess, err := s.resolveSession(userEmail, providerName, id)
	if err != nil {
		return fmt.Errorf("update session prompts: %w", err)
	}
	return s.reg.UpdateSessionPrompts(sess, queue)
}

// EndSession removes a session from its account and cleans up indexes —
// the counterpart to StartSession.
func (s *Service) EndSession(userEmail, providerName string, id uint64) error {
	sess, err := s.resolveSession(userEmail, providerName, id)
	if err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	return s.reg.DeleteSession(sess)
}

// ── Workflows (cross-aggregate) ────────────────────────────────────────────────

// resolveOrCreateUser returns the user for email, creating one with name if
// none exists yet. An existing user's name is left untouched. Shared by
// every Workflows entry point that treats the user as an on-demand
// prerequisite rather than a strict precondition.
func (s *Service) resolveOrCreateUser(email, name string) (*model.User, error) {
	if u := s.reg.User(email); u != nil {
		return u, nil
	}
	u := model.NewUser(email, name)
	if err := s.reg.AddUser(u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// ensureAccount returns the account for (userEmail, providerName), creating
// the user and/or the account first if either does not yet exist. userName
// is used only when a new user must be created; an existing user's name is
// left untouched. providerName must already exist — providers are managed
// explicitly and are never created implicitly. Private: StartSession is the
// only sanctioned caller of this lenient path; AddAccount is the public,
// strict alternative.
func (s *Service) ensureAccount(userEmail, userName, providerName string) (*model.Account, error) {
	p, err := s.resolveProvider("resolve", providerName)
	if err != nil {
		return nil, err
	}

	u, err := s.resolveOrCreateUser(userEmail, userName)
	if err != nil {
		return nil, err
	}

	if acc := s.reg.Account(u, p); acc != nil {
		return acc, nil
	}
	return s.reg.AddAccount(u, p)
}

// StartSession is the entry point for the "new session" UI action — the
// counterpart to EndSession. The operator supplies only user-facing
// fields; the user and account underneath are resolved or created on
// demand via ensureAccount, so the caller never needs to know whether
// either already exists. projectName is optional ("" for none); when set,
// it must already be granted to the account — StartSession does not grant
// projects implicitly.
func (s *Service) StartSession(userEmail, userName, providerName, projectName, topic string, queue []model.Prompt) (*model.Session, error) {
	acc, err := s.ensureAccount(userEmail, userName, providerName)
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}

	var p *model.Project
	if projectName != "" {
		p, err = s.resolveProject("resolve", projectName)
		if err != nil {
			return nil, fmt.Errorf("start session: %w", err)
		}
	}

	return s.reg.AddSession(acc, p, topic, queue)
}

// CreateAccount is the entry point for the "new account" UI action. The
// user is resolved or created on demand via resolveOrCreateUser, the same
// helper StartSession uses, so the caller never needs to know whether it
// already exists. Unlike ensureAccount, the account itself is never
// resolved leniently: this is an explicit creation, so an account that
// already exists for (userEmail, providerName) is reported as
// model.ErrAccountExists rather than returned silently. It does not
// create a session — StartSession is the only entry point for that.
func (s *Service) CreateAccount(userEmail, userName, providerName string) (*model.Account, error) {
	u, err := s.resolveOrCreateUser(userEmail, userName)
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	p, err := s.resolveProvider("create account", providerName)
	if err != nil {
		return nil, err
	}

	return s.reg.AddAccount(u, p)
}
