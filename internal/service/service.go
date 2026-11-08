// Package service is the application layer between callers and model. It
// resolves primitive inputs into model entities, delegates storage and
// validation to model.Registry, and composes multi-step workflows.
//
// Validation is not duplicated: Registry.Add*/Update* already enforce
// model.Validate*. The ValidateX methods are read-only passthroughs for
// pre-flight UI feedback. StartSession is the only multi-call workflow; it
// requires an existing user and creates the account on demand.
package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/balinomad/prompt-backlog/internal/apperr"
	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/store"
)

// ── Service ───────────────────────────────────────────────────────────────────

// Service resolves primitive inputs into domain entities and delegates to
// Registry. It owns the persistence lifecycle: New loads, Save persists.
// Mutating methods do not call Save; the caller commits a batch.
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

// Save persists the current registry state. Call it once after a batch of
// mutating calls succeeds, not after each individual call.
func (s *Service) Save() error {
	if err := s.st.Save(s.reg); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

// ── Resolution helpers (private) ───────────────────────────────────────────────

// resolveVendor resolves name to its current *model.Vendor, or returns
// a "<verb> vendor: ... not found" error when it does not exist.
func (s *Service) resolveVendor(verb, name string) (*model.Vendor, error) {
	p := s.reg.Vendor(name)
	if p == nil {
		return nil, fmt.Errorf("%s vendor: %q %w", verb, name, model.ErrVendorNotFound)
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

// resolveAccount resolves (userEmail, vendorName) to its current
// *model.Account, checking user and vendor existence first so the error
// names whichever is actually missing.
func (s *Service) resolveAccount(userEmail, vendorName string) (*model.Account, error) {
	u := s.reg.User(userEmail)
	if u == nil {
		return nil, fmt.Errorf("%q %w", userEmail, model.ErrUserNotFound)
	}
	p := s.reg.Vendor(vendorName)
	if p == nil {
		return nil, fmt.Errorf("%q %w", vendorName, model.ErrVendorNotFound)
	}
	acc := s.reg.Account(u, p)
	if acc == nil {
		return nil, fmt.Errorf("account for user %q and vendor %q: %w", userEmail, vendorName, model.ErrAccountNotFound)
	}
	return acc, nil
}

// resolveSession resolves id to its current *model.Session, verifying it
// belongs to the account for (userEmail, vendorName).
func (s *Service) resolveSession(userEmail, vendorName string, id uint64) (*model.Session, error) {
	acc, err := s.resolveAccount(userEmail, vendorName)
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

// ── Vendor ──────────────────────────────────────────────────────────────────

// Vendor returns the vendor with the given name, or nil if not found.
func (s *Service) Vendor(name string) *model.Vendor { return s.reg.Vendor(name) }

// Vendors returns all vendors sorted by name.
func (s *Service) Vendors() []*model.Vendor { return s.reg.Vendors() }

// ValidateVendor reports whether (name, hasProjects, hasDeepResearch)
// would form a valid vendor, without inserting anything.
func (s *Service) ValidateVendor(name string, hasProjects, hasDeepResearch bool) error {
	return model.ValidateVendor(model.NewVendor(name, hasProjects, hasDeepResearch))
}

// AddVendor inserts a vendor into the registry.
// Returns an error when the vendor is nil, has an empty name, or already exists.
func (s *Service) AddVendor(name string, hasProjects, hasDeepResearch bool) error {
	return s.reg.AddVendor(model.NewVendor(name, hasProjects, hasDeepResearch))
}

// UpdateVendor updates a vendor's capabilities.
func (s *Service) UpdateVendor(name string, hasProjects, hasDeepResearch bool) error {
	current, err := s.resolveVendor("update", name)
	if err != nil {
		return err
	}
	return s.reg.UpdateVendor(current, model.NewVendor(name, hasProjects, hasDeepResearch))
}

// DeleteVendor removes a vendor from the registry.
// Returns an error when the vendor is not found or has associated accounts.
func (s *Service) DeleteVendor(name string) error {
	p, err := s.resolveVendor("delete", name)
	if err != nil {
		return err
	}
	return s.reg.DeleteVendor(p)
}

// ── User ──────────────────────────────────────────────────────────────────────

// User returns the user with the given email, or nil if not found.
func (s *Service) User(email string) *model.User { return s.reg.User(email) }

// Users returns all users sorted by name.
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

// ProjectByID returns the project with the given ID, or nil if not found.
func (s *Service) ProjectByID(id uint64) *model.Project { return s.reg.ProjectByID(id) }

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

// RenameProject changes a project's name while preserving its ID and all
// references. Returns the renamed project.
func (s *Service) RenameProject(oldName, newName string) (*model.Project, error) {
	current, err := s.resolveProject("rename", oldName)
	if err != nil {
		return nil, err
	}
	renamed, err := s.reg.RenameProject(current, newName)
	if err != nil {
		return nil, fmt.Errorf("rename project: %w", err)
	}
	return renamed, nil
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

// Account returns the account for the given user and vendor.
func (s *Service) Account(userEmail, vendorName string) (*model.Account, error) {
	return s.resolveAccount(userEmail, vendorName)
}

// Accounts returns all accounts sorted by composite key.
func (s *Service) Accounts() []*model.Account { return s.reg.Accounts() }

// AccountProjects returns all projects assigned to the account for the
// given user and vendor.
func (s *Service) AccountProjects(userEmail, vendorName string) ([]*model.Project, error) {
	acc, err := s.resolveAccount(userEmail, vendorName)
	if err != nil {
		return nil, fmt.Errorf("account projects: %w", err)
	}
	return s.reg.AccountProjects(acc)
}

// AccountSessions returns all sessions for the account for the given user
// and vendor.
func (s *Service) AccountSessions(userEmail, vendorName string) ([]*model.Session, error) {
	acc, err := s.resolveAccount(userEmail, vendorName)
	if err != nil {
		return nil, fmt.Errorf("account sessions: %w", err)
	}
	return s.reg.AccountSessions(acc)
}

// ValidateAccount reports whether (email, vendor) would form a valid
// account. It checks presence only; email format is validated when the
// underlying user is created.
func (s *Service) ValidateAccount(email, vendor string) error {
	return model.ValidateAccount(model.NewAccount(email, vendor))
}

// AddAccount inserts an account for an existing user and vendor. It is the
// strict counterpart to StartSession's on-demand creation.
func (s *Service) AddAccount(userEmail, vendorName string) error {
	u, err := s.resolveUser("add account", userEmail)
	if err != nil {
		return err
	}
	p, err := s.resolveVendor("add account", vendorName)
	if err != nil {
		return err
	}
	_, err = s.reg.AddAccount(u, p)
	return err
}

// DeleteAccount removes an account.
// Returns an error if the account is not found or has active sessions or
// assigned projects.
func (s *Service) DeleteAccount(userEmail, vendorName string) error {
	acc, err := s.resolveAccount(userEmail, vendorName)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return s.reg.DeleteAccount(acc)
}

// PauseAccount pauses an account until the given time; no session under
// the account should be used before then.
func (s *Service) PauseAccount(userEmail, vendorName string, until time.Time) error {
	acc, err := s.resolveAccount(userEmail, vendorName)
	if err != nil {
		return fmt.Errorf("pause account: %w", err)
	}
	return s.reg.PauseAccount(acc, until)
}

// GrantProject gives an account access to a specific project.
func (s *Service) GrantProject(userEmail, vendorName, projectName string) error {
	acc, err := s.resolveAccount(userEmail, vendorName)
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
func (s *Service) RevokeProject(userEmail, vendorName, projectName string) error {
	acc, err := s.resolveAccount(userEmail, vendorName)
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

// SessionFields are the user-editable session fields shared by create and
// update use cases.
type SessionFields struct {
	Name        string
	URL         string
	ProjectName string
}

// Session returns the session with the given ID, or nil if not found.
func (s *Service) Session(id uint64) *model.Session { return s.reg.Session(id) }

// Sessions returns all sessions sorted by ID.
func (s *Service) Sessions() []*model.Session { return s.reg.Sessions() }

// UpdateSessionName updates the name of a session.
func (s *Service) UpdateSessionName(userEmail, vendorName string, id uint64, name string) error {
	sess, err := s.resolveSession(userEmail, vendorName, id)
	if err != nil {
		return fmt.Errorf("update session name: %w", err)
	}
	return s.reg.UpdateSessionName(sess, name)
}

// UpdateSessionProject reassigns a session to a different project.
func (s *Service) UpdateSessionProject(userEmail, vendorName string, id uint64, projectName string) error {
	sess, err := s.resolveSession(userEmail, vendorName, id)
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

// UpdateSessionURL updates a session's conversation URL.
func (s *Service) UpdateSessionURL(userEmail, vendorName string, id uint64, url string) error {
	sess, err := s.resolveSession(userEmail, vendorName, id)
	if err != nil {
		return fmt.Errorf("update session url: %w", err)
	}
	return s.reg.UpdateSessionURL(sess, url)
}

// UpdateSessionStatus updates a session's lifecycle status.
func (s *Service) UpdateSessionStatus(userEmail, vendorName string, id uint64, status model.SessionStatus) error {
	sess, err := s.resolveSession(userEmail, vendorName, id)
	if err != nil {
		return fmt.Errorf("update session status: %w", err)
	}
	return s.reg.UpdateSessionStatus(sess, status)
}

// UpdateSession updates a session's name, URL, and project atomically.
func (s *Service) UpdateSession(userEmail, vendorName string, id uint64, fields SessionFields) error {
	sess, err := s.resolveSession(userEmail, vendorName, id)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	var project *model.Project
	if fields.ProjectName != "" {
		project, err = s.resolveProject("resolve", fields.ProjectName)
		if err != nil {
			return fmt.Errorf("update session: %w", err)
		}
	}
	return s.reg.UpdateSession(sess, fields.Name, fields.URL, project)
}

// LatestSession returns the account's latest session, or false when the
// account has no session with a non-zero updated-at.
func (s *Service) LatestSession(userEmail, vendorName string) (*model.Session, bool, error) {
	acc, err := s.resolveAccount(userEmail, vendorName)
	if err != nil {
		return nil, false, fmt.Errorf("latest session: %w", err)
	}
	sess, ok, err := s.reg.LatestSession(acc)
	if err != nil {
		return nil, false, fmt.Errorf("latest session: %w", err)
	}
	return sess, ok, nil
}

// AccountSessionsActivityOrder returns the account's sessions in activity
// order: non-closed before closed, then updated-at descending, then ID
// descending within each group.
func (s *Service) AccountSessionsActivityOrder(userEmail, vendorName string) ([]*model.Session, error) {
	acc, err := s.resolveAccount(userEmail, vendorName)
	if err != nil {
		return nil, fmt.Errorf("account sessions activity order: %w", err)
	}
	return s.reg.AccountSessionsActivityOrder(acc)
}

// ── Note ──────────────────────────────────────────────────────────────────────

// Note returns the note with the given ID, or nil if not found.
func (s *Service) Note(id uint64) *model.Note { return s.reg.Note(id) }

// Notes returns all notes sorted by ID.
func (s *Service) Notes() []*model.Note { return s.reg.Notes() }

// VisibleNotes returns the notes visible for the account for the given filter.
func (s *Service) VisibleNotes(userEmail, vendorName string, filter model.ProjectFilter) ([]*model.Note, error) {
	acc, err := s.resolveAccount(userEmail, vendorName)
	if err != nil {
		return nil, fmt.Errorf("visible notes: %w", err)
	}
	return s.reg.VisibleNotes(acc, filter)
}

// ProjectNotes returns every note assigned to the named project.
func (s *Service) ProjectNotes(projectName string) ([]*model.Note, error) {
	p, err := s.resolveProject("resolve", projectName)
	if err != nil {
		return nil, fmt.Errorf("project notes: %w", err)
	}
	return s.reg.ProjectNotes(p)
}

// AddNote creates a note. An empty project name means no project. An empty
// user email together with an empty vendor name means no account; exactly one
// empty is an error wrapping ErrKeyRequired.
func (s *Service) AddNote(text, projectName, userEmail, vendorName string) (*model.Note, error) {
	project, acc, err := s.resolveNoteRefs(projectName, userEmail, vendorName)
	if err != nil {
		return nil, fmt.Errorf("add note: %w", err)
	}
	n, err := s.reg.AddNote(text, project, acc)
	if err != nil {
		return nil, fmt.Errorf("add note: %w", err)
	}
	return n, nil
}

// UpdateNote updates a note. Project and account arguments have the same
// meaning as for AddNote.
func (s *Service) UpdateNote(id uint64, text, projectName, userEmail, vendorName string) error {
	note := s.reg.Note(id)
	if note == nil {
		return fmt.Errorf("update note %d: %w", id, model.ErrNoteNotFound)
	}
	project, acc, err := s.resolveNoteRefs(projectName, userEmail, vendorName)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	if err := s.reg.UpdateNote(note, text, project, acc); err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	return nil
}

// DeleteNote removes a note by ID.
func (s *Service) DeleteNote(id uint64) error {
	note := s.reg.Note(id)
	if note == nil {
		return fmt.Errorf("delete note %d: %w", id, model.ErrNoteNotFound)
	}
	if err := s.reg.DeleteNote(note); err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}

// resolveNoteRefs resolves the optional project and account inputs shared by
// AddNote and UpdateNote.
func (s *Service) resolveNoteRefs(projectName, userEmail, vendorName string) (*model.Project, *model.Account, error) {
	var project *model.Project
	if projectName != "" {
		p, err := s.resolveProject("resolve", projectName)
		if err != nil {
			return nil, nil, err
		}
		project = p
	}

	emailEmpty := model.NormalizeEmail(userEmail) == ""
	vendorEmpty := model.NormalizeText(vendorName) == ""
	switch {
	case emailEmpty && vendorEmpty:
		return project, nil, nil
	case emailEmpty || vendorEmpty:
		return nil, nil, fmt.Errorf("note account %w", model.ErrKeyRequired)
	default:
		acc, err := s.resolveAccount(userEmail, vendorName)
		if err != nil {
			return nil, nil, err
		}
		return project, acc, nil
	}
}

// UpdateSessionPrompts overwrites the pending prompt queue for a session.
func (s *Service) UpdateSessionPrompts(userEmail, vendorName string, id uint64, queue []model.Prompt) error {
	sess, err := s.resolveSession(userEmail, vendorName, id)
	if err != nil {
		return fmt.Errorf("update session messages: %w", err)
	}
	return s.reg.UpdateSessionPrompts(sess, queue)
}

// DeleteSession removes a session from its account and cleans up indexes.
func (s *Service) DeleteSession(userEmail, vendorName string, id uint64) error {
	sess, err := s.resolveSession(userEmail, vendorName, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return s.reg.DeleteSession(sess)
}

// ── Workflows (cross-aggregate) ────────────────────────────────────────────────

// StartSession creates a session with the given fields. The prompt queue is
// empty; prompts are added separately.
func (s *Service) StartSession(userEmail, vendorName string, fields SessionFields) (*model.Session, error) {
	acc, err := s.resolveAccount(userEmail, vendorName)
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}
	var project *model.Project
	if fields.ProjectName != "" {
		project, err = s.resolveProject("resolve", fields.ProjectName)
		if err != nil {
			return nil, fmt.Errorf("start session: %w", err)
		}
	}
	sess, err := s.reg.StartSessionAtomic(acc, project, fields.Name, fields.URL, "", nil)
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}
	return sess, nil
}
