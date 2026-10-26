package model

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
)

// compareFold compares a and b case-insensitively, for name-based sorting.
// Vendors, Projects, and AccountKey (whose vendor segment is not
// case-normalised the way User.email already is) are ordered with this;
// email-keyed sorts don't need it since NormalizeEmail lowercases first.
func compareFold(a, b string) int {
	return cmp.Compare(strings.ToLower(a), strings.ToLower(b))
}

func addIndex[K, V comparable](idx map[K]map[V]struct{}, key K, value V) {
	set, ok := idx[key]
	if !ok {
		set = make(map[V]struct{})
		idx[key] = set
	}
	set[value] = struct{}{}
}

func removeIndex[K, V comparable](idx map[K]map[V]struct{}, key K, value V) {
	set, ok := idx[key]
	if !ok {
		return
	}
	delete(set, value)
	if len(set) == 0 {
		delete(idx, key)
	}
}

// ── Registry ──────────────────────────────────────────────────────────────────

// Registry is the root in-memory index structure.
// It is safe for concurrent use. The embedded sync.RWMutex protects all internal
// maps and the surrogate key generator from concurrent access.
//
// Copy-on-Write is the permitted update pattern. Entities are immutable once
// constructed. Updates replace the pointer in the map under the Registry's lock.
type Registry struct {
	mu sync.RWMutex // Protects all maps and lastSessionID

	// lastSessionID is the most recently issued session ID, shared by all
	// accounts — it is not scoped per account. Starts at 0 when no sessions
	// exist; AddSession assigns lastSessionID+1 to the next session.
	lastSessionID uint64

	vendors  map[string]*Vendor  // [vendorName]
	users    map[string]*User    // [userEmail]
	projects map[string]*Project // [projectName]
	accounts map[AccountKey]*Account
	sessions map[uint64]*Session // [sessionID]

	// One-to-many lookup sets
	accountSessions map[AccountKey]map[uint64]struct{} // [accountKey][sessionID]
	projectSessions map[string]map[uint64]struct{}     // [projectName][sessionID]

	// Many-to-many Account <-> Project association sets
	accountProjects map[AccountKey]map[string]struct{} // [accountKey][projectName]
	projectAccounts map[string]map[AccountKey]struct{} // [projectName][accountKey]
}

// NewRegistry returns an empty Registry with all maps initialised.
func NewRegistry() *Registry {
	return &Registry{
		lastSessionID:   0,
		vendors:         make(map[string]*Vendor),
		users:           make(map[string]*User),
		projects:        make(map[string]*Project),
		accounts:        make(map[AccountKey]*Account),
		sessions:        make(map[uint64]*Session),
		accountSessions: make(map[AccountKey]map[uint64]struct{}),
		projectSessions: make(map[string]map[uint64]struct{}),
		accountProjects: make(map[AccountKey]map[string]struct{}),
		projectAccounts: make(map[string]map[AccountKey]struct{}),
	}
}

// ── Vendor ──────────────────────────────────────────────────────────────────

// AddVendor inserts a vendor into the registry.
// Returns an error when the vendor is nil, has an empty name, or already exists.
func (r *Registry) AddVendor(p *Vendor) error {
	if err := ValidateVendor(p); err != nil {
		return fmt.Errorf("add vendor: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.vendors[p.name]; exists {
		return fmt.Errorf("add vendor %q: %w", p.name, ErrVendorExists)
	}
	r.vendors[p.name] = p
	return nil
}

// Vendor returns the vendor with the given name, or nil if not found.
func (r *Registry) Vendor(name string) *Vendor {
	name = NormalizeText(name)

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.vendors[name]
}

// Vendors returns all vendors sorted by name.
func (r *Registry) Vendors() []*Vendor {
	r.mu.RLock()
	vendors := slices.Collect(maps.Values(r.vendors))
	r.mu.RUnlock()

	slices.SortFunc(vendors, func(a, b *Vendor) int { return compareFold(a.name, b.name) })
	return vendors
}

// DeleteVendor removes a vendor from the registry.
// Returns ErrVendorNotFound if no vendor with that name exists,
// ErrVendorStale if p no longer matches the live registry entry, or
// an error if it has associated accounts.
func (r *Registry) DeleteVendor(p *Vendor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyVendor(p); err != nil {
		return fmt.Errorf("delete vendor: %w", err)
	}
	for _, acc := range r.accounts {
		if acc.key.Vendor == p.name {
			return fmt.Errorf("delete vendor %q: it has associated accounts", p.name)
		}
	}
	delete(r.vendors, p.name)
	return nil
}

// UpdateVendor replaces an existing vendor's capability flags via
// Copy-on-Write. current must be the exact *Vendor the caller most
// recently read from the registry (e.g. via Registry.Vendor); next
// carries the desired new values under the same name. name itself cannot
// change through this method — it is Vendor's natural key.
//
// Returns ErrVendorNotFound if no vendor with that name exists, or
// ErrVendorStale if current no longer matches the live registry entry.
func (r *Registry) UpdateVendor(current, next *Vendor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyVendor(current); err != nil {
		return fmt.Errorf("update vendor: current %w", err)
	}
	if err := ValidateVendor(next); err != nil {
		return fmt.Errorf("update vendor: next %w", err)
	}
	if current.name != next.name {
		return fmt.Errorf("update vendor: name mismatch %q != %q", current.name, next.name)
	}
	r.vendors[next.name] = next
	return nil
}

// verifyVendor returns ErrVendorNil if p is nil, ErrVendorNotFound if no
// vendor with that name exists, or ErrVendorStale if p no longer matches the
// live registry entry.
//
// The caller must hold the registry read lock.
func (r *Registry) verifyVendor(p *Vendor) error {
	if p == nil {
		return ErrVendorNil
	}
	live, exists := r.vendors[p.name]
	if !exists {
		return fmt.Errorf("%q %w", p.name, ErrVendorNotFound)
	}
	if live != p {
		return fmt.Errorf("%q %w", p.name, ErrVendorStale)
	}
	return nil
}

// ── User ──────────────────────────────────────────────────────────────────────

// AddUser inserts a user into the registry.
// Returns an error when the user is nil, has an empty email, or already exists.
func (r *Registry) AddUser(u *User) error {
	if err := ValidateUser(u); err != nil {
		return fmt.Errorf("add user: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[u.email]; exists {
		return fmt.Errorf("add user %q: %w", u.email, ErrUserExists)
	}
	r.users[u.email] = u
	return nil
}

// User returns the user with the given email, or nil if not found.
func (r *Registry) User(email string) *User {
	email = NormalizeEmail(email)

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.users[email]
}

// Users returns all users sorted by name, then email.
func (r *Registry) Users() []*User {
	r.mu.RLock()
	users := slices.Collect(maps.Values(r.users))
	r.mu.RUnlock()

	slices.SortFunc(users, func(a, b *User) int {
		if c := compareFold(a.name, b.name); c != 0 {
			return c
		}
		return cmp.Compare(a.email, b.email)
	})
	return users
}

// DeleteUser removes a user from the registry.
// Returns ErrUserNotFound if no user with that email exists,
// ErrUserStale if u no longer matches the live registry entry, or an
// error if it has associated accounts.
func (r *Registry) DeleteUser(u *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyUser(u); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	for _, acc := range r.accounts {
		if acc.key.Email == u.email {
			return fmt.Errorf("delete user %q: user has associated accounts", u.email)
		}
	}
	delete(r.users, u.email)
	return nil
}

// UpdateUser replaces an existing user's display name via Copy-on-Write.
// current must be the exact *User the caller most recently read from the
// registry; next carries the desired new value under the same email.
// email itself cannot change through this method — it is User's natural key.
//
// Returns ErrUserNotFound if no user with that email exists, or
// ErrUserStale if current no longer matches the live registry entry.
func (r *Registry) UpdateUser(current, next *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyUser(current); err != nil {
		return fmt.Errorf("update user: current %w", err)
	}
	if err := ValidateUser(next); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if current.email != next.email {
		return fmt.Errorf("update user: email mismatch %q != %q", current.email, next.email)
	}
	r.users[next.email] = next
	return nil
}

// verifyUser returns ErrUserNil if u is nil, ErrUserNotFound if no user with that
// email exists, or ErrUserStale if u no longer matches the live registry entry.
//
// The caller must hold the registry read lock.
func (r *Registry) verifyUser(u *User) error {
	if u == nil {
		return ErrUserNil
	}
	live, exists := r.users[u.email]
	if !exists {
		return fmt.Errorf("%q %w", u.email, ErrUserNotFound)
	}
	if live != u {
		return fmt.Errorf("%q %w", u.email, ErrUserStale)
	}
	return nil
}

// ── Project ───────────────────────────────────────────────────────────────────

// AddProject inserts a project into the registry.
// Returns an error when the project is nil, has an empty name, or already exists.
func (r *Registry) AddProject(p *Project) error {
	if err := ValidateProject(p); err != nil {
		return fmt.Errorf("add project: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[p.name]; exists {
		return fmt.Errorf("add project %q: %w", p.name, ErrProjectExists)
	}
	r.projects[p.name] = p
	return nil
}

// Project returns the project with the given name, or nil if not found.
func (r *Registry) Project(name string) *Project {
	name = NormalizeText(name)

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.projects[name]
}

// Projects returns all projects sorted by name.
func (r *Registry) Projects() []*Project {
	r.mu.RLock()
	projects := slices.Collect(maps.Values(r.projects))
	r.mu.RUnlock()

	slices.SortFunc(projects, func(a, b *Project) int { return compareFold(a.name, b.name) })
	return projects
}

// DeleteProject removes a project and cleans up all links.
// Returns ErrProjectNotFound if no project with that name exists,
// ErrProjectStale if p no longer matches the live registry entry, or
// an error if it is assigned to any account or has active sessions.
func (r *Registry) DeleteProject(p *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyProject(p); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if accs, exists := r.projectAccounts[p.name]; exists && len(accs) > 0 {
		return fmt.Errorf("delete project %q: it is assigned to %d accounts", p.name, len(accs))
	}
	if sessions, exists := r.projectSessions[p.name]; exists && len(sessions) > 0 {
		return fmt.Errorf("delete project %q: it has %d active sessions", p.name, len(sessions))
	}

	delete(r.projectAccounts, p.name)
	delete(r.projectSessions, p.name)
	delete(r.projects, p.name)
	return nil
}

// UpdateProject replaces an existing project's description and instructions
// via Copy-on-Write. current must be the exact *Project the caller most
// recently read from the registry (e.g. via Registry.Project); next carries
// the desired new values under the same name. name itself cannot change
// through this method — it is Project's natural key.
//
// Returns ErrProjectNotFound if no project with that name exists, or
// ErrProjectStale if current no longer matches the live registry entry
// (it was replaced by another update since current was read).
func (r *Registry) UpdateProject(current, next *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyProject(current); err != nil {
		return fmt.Errorf("update project: current %w", err)
	}
	if err := ValidateProject(next); err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	if current.name != next.name {
		return fmt.Errorf("update project: name mismatch %q != %q", current.name, next.name)
	}
	r.projects[next.name] = next
	return nil
}

// ProjectAccounts returns all accounts where this project is assigned,
// sorted by key string. Returns ErrProjectNil, ErrProjectNotFound, or
// ErrProjectStale if p does not resolve. A non-nil, empty slice with a
// nil error means p resolved but has no assigned accounts.
func (r *Registry) ProjectAccounts(p *Project) ([]*Account, error) {
	r.mu.RLock()
	if err := r.verifyProject(p); err != nil {
		r.mu.RUnlock()
		return nil, fmt.Errorf("project accounts: %w", err)
	}

	m := r.projectAccounts[p.name]
	result := make([]*Account, 0, len(m))
	for accKey := range m {
		if acc := r.accounts[accKey]; acc != nil {
			result = append(result, acc)
		}
	}
	r.mu.RUnlock()

	slices.SortFunc(result, func(a, b *Account) int { return compareFold(a.key.String(), b.key.String()) })
	return result, nil
}

// ProjectSessions returns all sessions for a specific project, sorted by
// ID. Returns ErrProjectNil, ErrProjectNotFound, or ErrProjectStale if p
// does not resolve. A non-nil, empty slice with a nil error means p
// resolved but has no sessions.
func (r *Registry) ProjectSessions(p *Project) ([]*Session, error) {
	r.mu.RLock()
	if err := r.verifyProject(p); err != nil {
		r.mu.RUnlock()
		return nil, fmt.Errorf("project sessions: %w", err)
	}

	m := r.projectSessions[p.name]
	result := make([]*Session, 0, len(m))
	for sessID := range m {
		if sess := r.sessions[sessID]; sess != nil {
			result = append(result, sess)
		}
	}
	r.mu.RUnlock()

	slices.SortFunc(result, func(a, b *Session) int { return cmp.Compare(a.id, b.id) })
	return result, nil
}

// verifyProject returns ErrProjectNil if p is nil, ErrProjectNotFound if no
// project with that name exists, or ErrProjectStale if p no longer matches the
// live registry entry.
//
// The caller must hold the registry read lock.
func (r *Registry) verifyProject(p *Project) error {
	if p == nil {
		return ErrProjectNil
	}
	live, exists := r.projects[p.name]
	if !exists {
		return fmt.Errorf("%q %w", p.name, ErrProjectNotFound)
	}
	if live != p {
		return fmt.Errorf("%q %w", p.name, ErrProjectStale)
	}
	return nil
}

// ── Account ───────────────────────────────────────────────────────────────────

// AddAccount inserts an account into the registry.
// Returns an error if the user or vendor is not registered in the registry
// or the account already exists.
func (r *Registry) AddAccount(u *User, p *Vendor) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyVendor(p); err != nil {
		return nil, fmt.Errorf("add account: %w", err)
	}
	if err := r.verifyUser(u); err != nil {
		return nil, fmt.Errorf("add account: %w", err)
	}
	acc := NewAccount(u.email, p.name)
	if _, exists := r.accounts[acc.key]; exists {
		return nil, fmt.Errorf("add account: %w for user %q and vendor %q", ErrAccountExists, u.email, p.name)
	}
	r.accounts[acc.key] = acc
	return acc, nil
}

// Account returns the account with the given user and vendor, or nil if not found.
func (r *Registry) Account(u *User, p *Vendor) *Account {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.verifyVendor(p); err != nil {
		return nil
	}
	if err := r.verifyUser(u); err != nil {
		return nil
	}
	if acc, exists := r.accounts[AccountKey{Email: u.email, Vendor: p.name}]; exists {
		return acc
	}
	return nil
}

// Accounts returns all accounts sorted by their composite key string.
func (r *Registry) Accounts() []*Account {
	r.mu.RLock()
	accounts := slices.Collect(maps.Values(r.accounts))
	r.mu.RUnlock()

	slices.SortFunc(accounts, func(a, b *Account) int { return compareFold(a.key.String(), b.key.String()) })
	return accounts
}

// DeleteAccount removes an account.
// Returns ErrAccountNotFound if no such account exists, ErrAccountStale
// if acc no longer matches the live registry entry, or an error if it
// has active sessions or assigned projects.
func (r *Registry) DeleteAccount(acc *Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyAccount(acc); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if sessions, exists := r.accountSessions[acc.key]; exists && len(sessions) > 0 {
		return fmt.Errorf("delete account: the account for user %q and vendor %q has %d active sessions", acc.key.Email, acc.key.Vendor, len(sessions))
	}
	if projects, exists := r.accountProjects[acc.key]; exists && len(projects) > 0 {
		return fmt.Errorf("delete account: %d projects assigned for user %q and vendor %q", len(projects), acc.key.Email, acc.key.Vendor)
	}

	delete(r.accountSessions, acc.key)
	delete(r.accountProjects, acc.key)
	delete(r.accounts, acc.key)
	return nil
}

// PauseAccount pauses acc until the given time using Copy-on-Write; no
// session under the account should be used before then. Returns
// ErrAccountNotFound if no such account exists, or ErrAccountStale if
// acc no longer matches the live registry entry.
func (r *Registry) PauseAccount(acc *Account, until time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyAccount(acc); err != nil {
		return fmt.Errorf("pause account: %w", err)
	}

	accCopy := *acc
	accCopy.pauseUntil = until
	r.accounts[acc.key] = &accCopy
	return nil
}

// AddProjectToAccount associates a project with an account. Returns an
// error if the account's vendor does not support projects
// (Vendor.HasProjects), or if the project is already assigned.
func (r *Registry) AddProjectToAccount(acc *Account, p *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyAccount(acc); err != nil {
		return fmt.Errorf("add project to account: %w", err)
	}
	if err := r.verifyProject(p); err != nil {
		return fmt.Errorf("add project to account: %w", err)
	}
	prov := r.vendors[acc.key.Vendor]
	if prov == nil {
		// Unreachable in practice: DeleteVendor refuses while any account
		// still references it.
		return fmt.Errorf("add project to account: vendor %q %w", acc.key.Vendor, ErrVendorNotFound)
	}
	if !prov.hasProjects {
		return fmt.Errorf("add project to account: vendor %q %w", acc.key.Vendor, ErrProjectsUnsupported)
	}
	if _, exists := r.accountProjects[acc.key][p.name]; exists {
		return fmt.Errorf("add project to account: %q is already assigned to this account", p.name)
	}

	addIndex(r.accountProjects, acc.key, p.name)
	addIndex(r.projectAccounts, p.name, acc.key)
	return nil
}

// RemoveProjectFromAccount removes the association between a project and an account.
// Returns an error if the account has active sessions currently using this project.
func (r *Registry) RemoveProjectFromAccount(acc *Account, p *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyAccount(acc); err != nil {
		return fmt.Errorf("remove project from account: %w", err)
	}
	if err := r.verifyProject(p); err != nil {
		return fmt.Errorf("remove project from account: %w", err)
	}
	if _, exists := r.accountProjects[acc.key][p.name]; !exists {
		return fmt.Errorf("remove project from account: %q is not assigned to this account", p.name)
	}

	if sessions, exists := r.accountSessions[acc.key]; exists {
		for id := range sessions {
			if r.sessions[id].projectName == p.name {
				return fmt.Errorf("remove project from account: account has active sessions using %q", p.name)
			}
		}
	}

	removeIndex(r.accountProjects, acc.key, p.name)
	removeIndex(r.projectAccounts, p.name, acc.key)
	return nil
}

// AccountProjects returns all projects assigned to an account, sorted by
// name. Returns ErrAccountNil, ErrAccountNotFound, or ErrAccountStale if
// acc does not resolve. A non-nil, empty slice with a nil error means acc
// resolved but has no assigned projects.
func (r *Registry) AccountProjects(acc *Account) ([]*Project, error) {
	r.mu.RLock()
	if err := r.verifyAccount(acc); err != nil {
		r.mu.RUnlock()
		return nil, fmt.Errorf("account projects: %w", err)
	}

	m := r.accountProjects[acc.key]
	result := make([]*Project, 0, len(m))
	for pName := range m {
		if p := r.projects[pName]; p != nil {
			result = append(result, p)
		}
	}
	r.mu.RUnlock()

	slices.SortFunc(result, func(a, b *Project) int { return compareFold(a.name, b.name) })
	return result, nil
}

// AccountSessions returns all sessions for a given account, sorted by ID.
// Returns ErrAccountNil, ErrAccountNotFound, or ErrAccountStale if acc
// does not resolve. A non-nil, empty slice with a nil error means acc
// resolved but has no sessions.
func (r *Registry) AccountSessions(acc *Account) ([]*Session, error) {
	r.mu.RLock()
	if err := r.verifyAccount(acc); err != nil {
		r.mu.RUnlock()
		return nil, fmt.Errorf("account sessions: %w", err)
	}

	m := r.accountSessions[acc.key]
	result := make([]*Session, 0, len(m))
	for sessID := range m {
		if sess := r.sessions[sessID]; sess != nil {
			result = append(result, sess)
		}
	}
	r.mu.RUnlock()

	slices.SortFunc(result, func(a, b *Session) int { return cmp.Compare(a.id, b.id) })
	return result, nil
}

// verifyAccount returns ErrAccountNil if acc is nil, ErrAccountNotFound
// if no account with that user and vendor exists, or ErrAccountStale
// if acc no longer matches the live registry entry.
//
// The caller must hold the registry read lock.
func (r *Registry) verifyAccount(acc *Account) error {
	if acc == nil {
		return ErrAccountNil
	}
	live, exists := r.accounts[acc.key]
	if !exists {
		return fmt.Errorf("%w for user %q and vendor %q", ErrAccountNotFound, acc.key.Email, acc.key.Vendor)
	}
	if live != acc {
		return fmt.Errorf("%w for user %q and vendor %q", ErrAccountStale, acc.key.Email, acc.key.Vendor)
	}
	return nil
}

// ── Session ───────────────────────────────────────────────────────────────────

// AddSession creates a new session for acc and assigns it the next
// session ID, then updates all relevant indexes. IDs are unique across
// the whole registry, not scoped per account, and are not persisted —
// numbering restarts at 1 each time the registry is constructed.
//
// Returns ErrAccountNotFound if no such account exists, ErrAccountStale
// if acc no longer matches the live registry entry, ErrProjectNotFound
// or ErrProjectStale if project is set but doesn't match the live
// registry entry, or an error if project is set but not assigned to
// this account.
func (r *Registry) AddSession(acc *Account, project *Project, topic string, queue []Prompt) (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyAccount(acc); err != nil {
		return nil, fmt.Errorf("add session: %w", err)
	}

	for i, p := range queue {
		if err := ValidatePrompt(p); err != nil {
			return nil, fmt.Errorf("add session: invalid message at index %d: %w", i, err)
		}
	}

	var projName string
	if project != nil {
		if err := r.verifyProject(project); err != nil {
			return nil, fmt.Errorf("add session: %w", err)
		}
		if _, assigned := r.accountProjects[acc.key][project.name]; !assigned {
			return nil, fmt.Errorf("add session: project %q is not assigned to this account", project.name)
		}
		projName = project.name
	}

	r.lastSessionID++
	id := r.lastSessionID
	sess := NewSession(acc.key, id, projName, topic, queue)

	r.sessions[id] = sess
	addIndex(r.accountSessions, acc.key, id)
	if projName != "" {
		addIndex(r.projectSessions, projName, id)
	}

	return sess, nil
}

// Session returns the session with the given ID, or nil if not found.
// Session IDs are unique across the whole registry, not scoped per account.
func (r *Registry) Session(id uint64) *Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.sessions[id]
}

// Sessions returns all sessions sorted by ID.
func (r *Registry) Sessions() []*Session {
	r.mu.RLock()
	sessions := slices.Collect(maps.Values(r.sessions))
	r.mu.RUnlock()

	slices.SortFunc(sessions, func(a, b *Session) int { return cmp.Compare(a.id, b.id) })
	return sessions
}

// DeleteSession removes a session and cleans up indexes.
// Returns ErrSessionNotFound if no such session exists, or
// ErrSessionStale if s no longer matches the live registry entry.
func (r *Registry) DeleteSession(sess *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifySession(sess); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	removeIndex(r.accountSessions, sess.accountKey, sess.id)
	if sess.projectName != "" {
		removeIndex(r.projectSessions, sess.projectName, sess.id)
	}
	delete(r.sessions, sess.id)

	return nil
}

// UpdateSessionTopic updates a session's high-level topic via Copy-on-Write.
// sess must be the exact pointer most recently read from the registry; the
// caller must re-fetch after a successful update.
//
// Returns ErrSessionNotFound if no such session exists, or ErrSessionStale
// if sess no longer matches the live registry entry.
func (r *Registry) UpdateSessionTopic(sess *Session, topic string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifySession(sess); err != nil {
		return fmt.Errorf("update session topic: %w", err)
	}
	newTopic := NormalizeText(topic)
	if sess.topic == newTopic {
		return nil
	}
	sessCopy := *sess
	sessCopy.topic = newTopic
	r.sessions[sess.id] = &sessCopy
	return nil
}

// UpdateSessionProject reassigns a session to a different project, or
// clears it when project is nil, via Copy-on-Write. sess must be the
// exact pointer most recently read from the registry; the caller must
// re-fetch after a successful update.
//
// Returns ErrSessionNotFound if no such session exists, ErrSessionStale
// if sess no longer matches the live registry entry, ErrProjectNotFound
// or ErrProjectStale if project is set but doesn't match the live
// registry entry, or an error if project is set but not assigned to
// this account.
func (r *Registry) UpdateSessionProject(sess *Session, project *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifySession(sess); err != nil {
		return fmt.Errorf("update session project: %w", err)
	}

	var nextProjName string
	if project != nil {
		if err := r.verifyProject(project); err != nil {
			return fmt.Errorf("update session project: %w", err)
		}
		nextProjName = project.name

		if _, assigned := r.accountProjects[sess.accountKey][nextProjName]; !assigned {
			return fmt.Errorf("update session project: project %q is not assigned to this account", nextProjName)
		}
	}

	// Early return to prevent unnecessary CoW allocations
	if sess.projectName == nextProjName {
		return nil
	}

	if sess.projectName != "" {
		removeIndex(r.projectSessions, sess.projectName, sess.id)
	}

	sessCopy := *sess
	sessCopy.projectName = nextProjName
	r.sessions[sess.id] = &sessCopy

	if nextProjName != "" {
		addIndex(r.projectSessions, nextProjName, sess.id)
	}
	return nil
}

// UpdateSessionPrompts overwrites a session's pending prompt queue via
// Copy-on-Write. sess must be the exact pointer most recently read from
// the registry; the caller must re-fetch after a successful update.
//
// Returns ErrSessionNotFound if no such session exists, ErrSessionStale
// if sess no longer matches the live registry entry, or an error if any
// prompt in queue is invalid.
func (r *Registry) UpdateSessionPrompts(sess *Session, queue []Prompt) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifySession(sess); err != nil {
		return fmt.Errorf("update session messages: %w", err)
	}

	for i, p := range queue {
		if err := ValidatePrompt(p); err != nil {
			return fmt.Errorf("update session messages: invalid message at index %d: %w", i, err)
		}
	}

	// slices.Equal works because Prompt contains only comparable primitive fields
	if slices.Equal(sess.queue, queue) {
		return nil
	}
	sessCopy := *sess
	sessCopy.queue = slices.Clone(queue)
	r.sessions[sess.id] = &sessCopy
	return nil
}

// verifySession returns ErrSessionNil if sess is nil, ErrSessionNotFound if
// the session does not exist, or ErrSessionStale if sess no longer matches
// the live registry entry.
//
// The caller must hold the registry read lock.
func (r *Registry) verifySession(sess *Session) error {
	if sess == nil {
		return ErrSessionNil
	}
	live, exists := r.sessions[sess.id]
	if !exists {
		return fmt.Errorf("#%d %w", sess.id, ErrSessionNotFound)
	}
	if live != sess {
		return fmt.Errorf("#%d %w", sess.id, ErrSessionStale)
	}
	return nil
}
