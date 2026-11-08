package model

import (
	"cmp"
	"errors"
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
	mu sync.RWMutex // Protects all maps and the ID counters

	// now returns the registry clock time in UTC. It is replaceable per
	// registry instance so tests can control time.
	now func() time.Time

	// lastSessionID is the most recently issued session ID, shared by all
	// accounts — it is not scoped per account. Starts at 0 when no sessions
	// exist; AddSession assigns lastSessionID+1 to the next session.
	lastSessionID uint64

	// lastAccountID is the most recently issued account ID.
	lastAccountID uint64

	// lastProjectID is the most recently issued project ID.
	lastProjectID uint64

	// lastNoteID is the most recently issued note ID, shared by all notes.
	lastNoteID uint64

	vendors      map[string]*Vendor  // [vendorName]
	users        map[string]*User    // [userEmail]
	projects     map[string]*Project // [projectName]
	projectsByID map[uint64]*Project
	accounts     map[AccountKey]*Account
	accountsByID map[uint64]*Account
	sessions     map[uint64]*Session // [sessionID]
	notes        map[uint64]*Note    // [noteID]

	// One-to-many lookup sets
	accountSessions map[AccountKey]map[uint64]struct{} // [accountKey][sessionID]
	projectSessions map[uint64]map[uint64]struct{}     // [projectID][sessionID]

	// Many-to-many Account <-> Project association sets
	accountProjects map[AccountKey]map[uint64]struct{} // [accountKey][projectID]
	projectAccounts map[uint64]map[AccountKey]struct{} // [projectID][accountKey]
}

// NewRegistry returns an empty Registry with all maps initialised.
func NewRegistry() *Registry {
	return &Registry{
		now:             utcNow,
		lastSessionID:   0,
		vendors:         make(map[string]*Vendor),
		users:           make(map[string]*User),
		projects:        make(map[string]*Project),
		projectsByID:    make(map[uint64]*Project),
		accounts:        make(map[AccountKey]*Account),
		accountsByID:    make(map[uint64]*Account),
		sessions:        make(map[uint64]*Session),
		notes:           make(map[uint64]*Note),
		accountSessions: make(map[AccountKey]map[uint64]struct{}),
		projectSessions: make(map[uint64]map[uint64]struct{}),
		accountProjects: make(map[AccountKey]map[uint64]struct{}),
		projectAccounts: make(map[uint64]map[AccountKey]struct{}),
	}
}

// utcNow returns the current time in UTC.
func utcNow() time.Time {
	return time.Now().UTC()
}

// SetClock replaces the registry clock. A nil clock restores the default UTC
// clock. Tests use this to control updated-at values.
func (r *Registry) SetClock(now func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if now == nil {
		r.now = utcNow
		return
	}
	r.now = now
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

// AddProject inserts a project into the registry and assigns the next project
// ID. Returns an error when the project is nil, has an empty name, or already
// exists.
func (r *Registry) AddProject(p *Project) error {
	if err := ValidateProject(p); err != nil {
		return fmt.Errorf("add project: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.projects[p.name]; exists {
		return fmt.Errorf("add project %q: %w", p.name, ErrProjectExists)
	}
	r.lastProjectID++
	p.id = r.lastProjectID
	r.projects[p.name] = p
	r.projectsByID[p.id] = p
	return nil
}

// ProjectByID returns the project with the given ID, or nil if not found.
func (r *Registry) ProjectByID(id uint64) *Project {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.projectsByID[id]
}

// RestoreProject inserts a project with the given ID. It validates the name,
// refuses a zero or duplicate ID, refuses a duplicate name, and raises the
// project counter to at least id. It is used only by the JSON store.
func (r *Registry) RestoreProject(name, description, instructions string, id uint64) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if id == 0 {
		return nil, errors.New("restore project: id 0 is invalid")
	}
	if _, exists := r.projectsByID[id]; exists {
		return nil, fmt.Errorf("restore project: id %d already exists", id)
	}
	p := newProject(id, name, description, instructions)
	if err := ValidateProject(p); err != nil {
		return nil, fmt.Errorf("restore project: %w", err)
	}
	if _, exists := r.projects[p.name]; exists {
		return nil, fmt.Errorf("restore project %q: %w", p.name, ErrProjectExists)
	}
	r.projects[p.name] = p
	r.projectsByID[id] = p
	if id > r.lastProjectID {
		r.lastProjectID = id
	}
	return p, nil
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
// an error if it is assigned to any account, has active sessions or has notes.
func (r *Registry) DeleteProject(p *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyProject(p); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if accs, exists := r.projectAccounts[p.id]; exists && len(accs) > 0 {
		return fmt.Errorf("delete project %q: it is assigned to %d accounts", p.name, len(accs))
	}
	if sessions, exists := r.projectSessions[p.id]; exists && len(sessions) > 0 {
		return fmt.Errorf("delete project %q: it has %d active sessions", p.name, len(sessions))
	}
	if count := r.countProjectNotes(p.id); count > 0 {
		return fmt.Errorf("delete project %q: it has %d notes", p.name, count)
	}

	delete(r.projectAccounts, p.id)
	delete(r.projectSessions, p.id)
	delete(r.projects, p.name)
	delete(r.projectsByID, p.id)
	return nil
}

// UpdateProject replaces an existing project's description and instructions
// via Copy-on-Write. current must be the exact *Project the caller most
// recently read from the registry. The name and ID cannot change through this
// method; use RenameProject to change the name.
//
// Returns ErrProjectNotFound if no project with that name exists, or
// ErrProjectStale if current no longer matches the live registry entry.
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
	updated := *current
	updated.description = next.description
	updated.instructions = next.instructions
	r.projects[updated.name] = &updated
	r.projectsByID[updated.id] = &updated
	return nil
}

// RenameProject replaces a project's name via Copy-on-Write. current must be
// the exact *Project the caller most recently read from the registry. The ID,
// description and instructions are preserved. Sessions and notes that
// reference the project are updated with the new display name.
//
// Returns ErrProjectNil, ErrProjectNotFound or ErrProjectStale for current,
// ErrNameRequired for an empty name, or ErrProjectExists when another project
// already has the new name.
func (r *Registry) RenameProject(current *Project, newName string) (*Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyProject(current); err != nil {
		return nil, fmt.Errorf("rename project: current %w", err)
	}
	newName = NormalizeText(newName)
	if newName == "" {
		return nil, fmt.Errorf("rename project: %w", ErrNameRequired)
	}
	if newName == current.name {
		return current, nil
	}
	if _, exists := r.projects[newName]; exists {
		return nil, fmt.Errorf("rename project %q to %q: %w", current.name, newName, ErrProjectExists)
	}

	next := *current
	next.name = newName
	delete(r.projects, current.name)
	r.projects[next.name] = &next
	r.projectsByID[next.id] = &next

	for id, session := range r.sessions {
		if session.projectID != next.id {
			continue
		}
		updated := *session
		updated.projectName = next.name
		r.sessions[id] = &updated
	}
	for id, note := range r.notes {
		if note.projectID != next.id {
			continue
		}
		updated := *note
		updated.projectName = next.name
		r.notes[id] = &updated
	}
	return &next, nil
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

	m := r.projectAccounts[p.id]
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

	m := r.projectSessions[p.id]
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
	if p.id != 0 {
		live, exists := r.projectsByID[p.id]
		if !exists {
			return fmt.Errorf("%q %w", p.name, ErrProjectNotFound)
		}
		if live != p {
			return fmt.Errorf("%q %w", p.name, ErrProjectStale)
		}
		return nil
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

// AddAccount inserts an account into the registry and assigns the next
// account ID. Returns an error if the user or vendor is not registered in the
// registry or the account already exists.
func (r *Registry) AddAccount(u *User, p *Vendor) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyVendor(p); err != nil {
		return nil, fmt.Errorf("add account: %w", err)
	}
	if err := r.verifyUser(u); err != nil {
		return nil, fmt.Errorf("add account: %w", err)
	}
	key := AccountKey{Email: u.email, Vendor: p.name}
	if _, exists := r.accounts[key]; exists {
		return nil, fmt.Errorf("add account: %w for user %q and vendor %q", ErrAccountExists, u.email, p.name)
	}
	r.lastAccountID++
	acc := newAccount(r.lastAccountID, u.email, p.name)
	r.accounts[acc.key] = acc
	r.accountsByID[acc.id] = acc
	return acc, nil
}

// AccountByID returns the account with the given ID, or nil if not found.
func (r *Registry) AccountByID(id uint64) *Account {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.accountsByID[id]
}

// RestoreAccount inserts an account with the given ID. It validates the user
// and vendor, refuses a zero or duplicate ID, refuses a duplicate natural key,
// and raises the account counter to at least id. It is used only by the JSON
// store.
func (r *Registry) RestoreAccount(u *User, p *Vendor, id uint64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if id == 0 {
		return nil, errors.New("restore account: id 0 is invalid")
	}
	if _, exists := r.accountsByID[id]; exists {
		return nil, fmt.Errorf("restore account: id %d already exists", id)
	}
	if err := r.verifyVendor(p); err != nil {
		return nil, fmt.Errorf("restore account: %w", err)
	}
	if err := r.verifyUser(u); err != nil {
		return nil, fmt.Errorf("restore account: %w", err)
	}
	acc := newAccount(id, u.email, p.name)
	if _, exists := r.accounts[acc.key]; exists {
		return nil, fmt.Errorf("restore account: %w for user %q and vendor %q", ErrAccountExists, u.email, p.name)
	}
	r.accounts[acc.key] = acc
	r.accountsByID[id] = acc
	if id > r.lastAccountID {
		r.lastAccountID = id
	}
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
	if count := r.countAccountNotes(acc.key); count > 0 {
		return fmt.Errorf("delete account: the account for user %q and vendor %q has %d notes", acc.key.Email, acc.key.Vendor, count)
	}

	delete(r.accountSessions, acc.key)
	delete(r.accountProjects, acc.key)
	delete(r.accounts, acc.key)
	delete(r.accountsByID, acc.id)
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
	r.accountsByID[acc.id] = &accCopy
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
	if _, exists := r.accountProjects[acc.key][p.id]; exists {
		return fmt.Errorf("add project to account: %q is already assigned to this account", p.name)
	}

	addIndex(r.accountProjects, acc.key, p.id)
	addIndex(r.projectAccounts, p.id, acc.key)
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
	if _, exists := r.accountProjects[acc.key][p.id]; !exists {
		return fmt.Errorf("remove project from account: %q is not assigned to this account", p.name)
	}

	if sessions, exists := r.accountSessions[acc.key]; exists {
		for id := range sessions {
			if r.sessions[id].projectID == p.id {
				return fmt.Errorf("remove project from account: account has active sessions using %q", p.name)
			}
		}
	}
	if count := r.countAccountProjectNotes(acc.key, p.id); count > 0 {
		return fmt.Errorf("remove project from account: account for user %q and vendor %q has %d notes for project %q", acc.key.Email, acc.key.Vendor, count, p.name)
	}

	removeIndex(r.accountProjects, acc.key, p.id)
	removeIndex(r.projectAccounts, p.id, acc.key)
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
	for projectID := range m {
		if p := r.projectsByID[projectID]; p != nil {
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

// AccountSessionsActivityOrder returns all sessions for a given account in
// activity order: non-closed sessions before closed sessions, then updated-at
// descending, then ID descending within each group. Sessions with a zero
// updated-at follow all sessions with a non-zero updated-at in their group.
// Returns ErrAccountNil, ErrAccountNotFound, or ErrAccountStale if acc
// does not resolve.
func (r *Registry) AccountSessionsActivityOrder(acc *Account) ([]*Session, error) {
	r.mu.RLock()
	if err := r.verifyAccount(acc); err != nil {
		r.mu.RUnlock()
		return nil, fmt.Errorf("account sessions activity order: %w", err)
	}

	m := r.accountSessions[acc.key]
	result := make([]*Session, 0, len(m))
	for sessID := range m {
		if sess := r.sessions[sessID]; sess != nil {
			result = append(result, sess)
		}
	}
	r.mu.RUnlock()

	slices.SortFunc(result, func(a, b *Session) int {
		if aClosed, bClosed := a.status == SessionStatusClosed, b.status == SessionStatusClosed; aClosed != bClosed {
			if aClosed {
				return 1
			}
			return -1
		}
		if c := b.updatedAt.Compare(a.updatedAt); c != 0 {
			return c
		}
		return cmp.Compare(b.id, a.id)
	})
	return result, nil
}

// LatestSession returns the account's latest session: the session with the
// greatest non-zero updated-at, or the greater ID when updated-at values are
// equal. It returns false when the account has no session with a non-zero
// updated-at.
// Returns ErrAccountNil, ErrAccountNotFound, or ErrAccountStale if acc
// does not resolve.
func (r *Registry) LatestSession(acc *Account) (*Session, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.verifyAccount(acc); err != nil {
		return nil, false, fmt.Errorf("latest session: %w", err)
	}

	var latest *Session
	for sessID := range r.accountSessions[acc.key] {
		sess := r.sessions[sessID]
		if sess == nil || sess.updatedAt.IsZero() {
			continue
		}
		if latest == nil || sess.updatedAt.After(latest.updatedAt) ||
			(sess.updatedAt.Equal(latest.updatedAt) && sess.id > latest.id) {
			latest = sess
		}
	}
	if latest == nil {
		return nil, false, nil
	}
	return latest, true, nil
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
func (r *Registry) AddSession(acc *Account, project *Project, name string, queue []Prompt) (*Session, error) {
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

	var projID uint64
	var projName string
	if project != nil {
		if err := r.verifyProject(project); err != nil {
			return nil, fmt.Errorf("add session: %w", err)
		}
		if _, assigned := r.accountProjects[acc.key][project.id]; !assigned {
			return nil, fmt.Errorf("add session: project %q is not assigned to this account", project.name)
		}
		projID = project.id
		projName = project.name
	}

	r.lastSessionID++
	id := r.lastSessionID
	updatedAt := r.now().UTC()
	sess := newSession(acc.key, acc.id, projID, id, projName, name, "", SessionStatusActive, updatedAt, queue)

	r.sessions[id] = sess
	addIndex(r.accountSessions, acc.key, id)
	if projID != 0 {
		addIndex(r.projectSessions, projID, id)
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

// StartSessionAtomic creates a session from a validated set of inputs under
// one write-lock acquisition. When note is set, promptText is required and the
// note is deleted after the session is created.
//
// Every input is validated before the first mutation. On failure the registry
// is unchanged.
func (r *Registry) StartSessionAtomic(acc *Account, project *Project, name, url, promptText string, note *Note) (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	plan, err := r.planStartSession(acc, project, name, url, promptText, note)
	if err != nil {
		return nil, err
	}
	return r.applyStartSession(plan), nil
}

// startSessionPlan holds validated inputs for StartSessionAtomic.
type startSessionPlan struct {
	acc        *Account
	project    *Project
	name       string
	url        string
	promptText string
	note       *Note
}

// planStartSession validates every input for StartSessionAtomic. The caller
// must hold the registry write lock.
func (r *Registry) planStartSession(acc *Account, project *Project, name, url, promptText string, note *Note) (startSessionPlan, error) {
	plan := startSessionPlan{}
	if err := r.verifyAccount(acc); err != nil {
		return plan, fmt.Errorf("start session: %w", err)
	}
	if err := r.validateStartProject(acc, project); err != nil {
		return plan, err
	}
	if err := r.validateStartNote(acc, note, project); err != nil {
		return plan, err
	}

	normalizedPrompt := NormalizeText(promptText)
	if err := validateStartPrompt(note, normalizedPrompt); err != nil {
		return plan, err
	}

	return startSessionPlan{
		acc:        acc,
		project:    project,
		name:       name,
		url:        NormalizeText(url),
		promptText: normalizedPrompt,
		note:       note,
	}, nil
}

// validateStartProject validates the project input for StartSessionAtomic.
// The caller must hold the registry write lock.
func (r *Registry) validateStartProject(acc *Account, project *Project) error {
	if project == nil {
		return nil
	}
	if err := r.verifyProject(project); err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	if _, assigned := r.accountProjects[acc.key][project.id]; !assigned {
		return fmt.Errorf("start session: project %q is not assigned to this account", project.name)
	}
	return nil
}

// validateStartNote validates the note input for StartSessionAtomic. The
// caller must hold the registry write lock.
func (r *Registry) validateStartNote(acc *Account, note *Note, project *Project) error {
	if note == nil {
		return nil
	}
	if err := r.verifyNote(note); err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	if !r.noteVisibleForAccount(note, acc) {
		return fmt.Errorf("start session: note #%d is not visible for this account", note.id)
	}
	if note.projectID == 0 {
		return nil
	}
	if project == nil || project.id != note.projectID {
		return fmt.Errorf("start session: note project %q must be the given existing project", note.projectName)
	}
	return nil
}

// validateStartPrompt validates the prompt input for StartSessionAtomic.
func validateStartPrompt(note *Note, promptText string) error {
	if promptText != "" {
		if err := ValidatePrompt(NewPrompt(promptText)); err != nil {
			return fmt.Errorf("start session: %w", err)
		}
	}
	if note != nil && promptText == "" {
		return errors.New("start session: prompt text is required when starting from a note")
	}
	return nil
}

// applyStartSession performs the mutations planned by planStartSession. The
// caller must hold the registry write lock.
func (r *Registry) applyStartSession(plan startSessionPlan) *Session {
	var queue []Prompt
	if plan.promptText != "" {
		queue = []Prompt{NewPrompt(plan.promptText)}
	}
	var projectID uint64
	var projectName string
	if plan.project != nil {
		projectID = plan.project.id
		projectName = plan.project.name
	}
	r.lastSessionID++
	sess := newSession(plan.acc.key, plan.acc.id, projectID, r.lastSessionID, projectName, plan.name, plan.url, SessionStatusActive, r.now().UTC(), queue)
	r.sessions[sess.id] = sess
	addIndex(r.accountSessions, plan.acc.key, sess.id)
	if projectID != 0 {
		addIndex(r.projectSessions, projectID, sess.id)
	}
	if plan.note != nil {
		delete(r.notes, plan.note.id)
	}
	return sess
}

// RestoreSession inserts a session with every field supplied, including its
// ID. It validates I1 and I2, refuses a zero or duplicate ID, and raises the
// session counter to at least id. It is used only by the JSON store.
func (r *Registry) RestoreSession(acc *Account, id uint64, project *Project, name, url string, status SessionStatus, updatedAt time.Time, queue []Prompt) (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if id == 0 {
		return nil, errors.New("restore session: id 0 is invalid")
	}
	if _, exists := r.sessions[id]; exists {
		return nil, fmt.Errorf("restore session: id %d already exists", id)
	}
	if err := r.verifyAccount(acc); err != nil {
		return nil, fmt.Errorf("restore session: %w", err)
	}
	if !status.Valid() {
		return nil, fmt.Errorf("restore session: invalid status %d", status)
	}
	for i, p := range queue {
		if err := ValidatePrompt(p); err != nil {
			return nil, fmt.Errorf("restore session: invalid prompt at index %d: %w", i, err)
		}
	}

	var projID uint64
	var projName string
	if project != nil {
		if err := r.verifyProject(project); err != nil {
			return nil, fmt.Errorf("restore session: %w", err)
		}
		if _, assigned := r.accountProjects[acc.key][project.id]; !assigned {
			return nil, fmt.Errorf("restore session: project %q is not assigned to this account", project.name)
		}
		projID = project.id
		projName = project.name
	}

	if !updatedAt.IsZero() {
		updatedAt = updatedAt.UTC()
	}
	sess := newSession(acc.key, acc.id, projID, id, projName, name, url, status, updatedAt, queue)
	r.sessions[id] = sess
	addIndex(r.accountSessions, acc.key, id)
	if projID != 0 {
		addIndex(r.projectSessions, projID, id)
	}
	if id > r.lastSessionID {
		r.lastSessionID = id
	}
	return sess, nil
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
	if sess.projectID != 0 {
		removeIndex(r.projectSessions, sess.projectID, sess.id)
	}
	delete(r.sessions, sess.id)

	return nil
}

// UpdateSessionName updates a session's name via Copy-on-Write.
// sess must be the exact pointer most recently read from the registry; the
// caller must re-fetch after a successful update.
//
// Returns ErrSessionNotFound if no such session exists, or ErrSessionStale
// if sess no longer matches the live registry entry.
func (r *Registry) UpdateSessionName(sess *Session, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifySession(sess); err != nil {
		return fmt.Errorf("update session name: %w", err)
	}
	newName := NormalizeText(name)
	if sess.name == newName {
		return nil
	}
	sessCopy := *sess
	sessCopy.name = newName
	r.sessions[sess.id] = &sessCopy
	return nil
}

// UpdateSession updates a session's name, URL, and project under one write
// lock. Every input is validated before the first mutation. An unchanged
// value is a no-op. Updated-at is not changed.
//
// Returns ErrSessionNotFound if no such session exists, ErrSessionStale if
// sess no longer matches the live registry entry, or a project error when the
// project is missing, stale, or not assigned to the session's account.
func (r *Registry) UpdateSession(sess *Session, name, url string, project *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifySession(sess); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	var nextProjectID uint64
	var nextProjectName string
	if project != nil {
		if err := r.verifyProject(project); err != nil {
			return fmt.Errorf("update session: %w", err)
		}
		if _, assigned := r.accountProjects[sess.accountKey][project.id]; !assigned {
			return fmt.Errorf("update session: project %q is not assigned to this account", project.name)
		}
		nextProjectID = project.id
		nextProjectName = project.name
	}

	nextName := NormalizeText(name)
	nextURL := NormalizeText(url)
	if sess.name == nextName && sess.url == nextURL && sess.projectID == nextProjectID {
		return nil
	}

	if sess.projectID != nextProjectID {
		if sess.projectID != 0 {
			removeIndex(r.projectSessions, sess.projectID, sess.id)
		}
		if nextProjectID != 0 {
			addIndex(r.projectSessions, nextProjectID, sess.id)
		}
	}

	sessCopy := *sess
	sessCopy.name = nextName
	sessCopy.url = nextURL
	sessCopy.projectID = nextProjectID
	sessCopy.projectName = nextProjectName
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

	var nextProjID uint64
	var nextProjName string
	if project != nil {
		if err := r.verifyProject(project); err != nil {
			return fmt.Errorf("update session project: %w", err)
		}
		nextProjID = project.id
		nextProjName = project.name

		if _, assigned := r.accountProjects[sess.accountKey][nextProjID]; !assigned {
			return fmt.Errorf("update session project: project %q is not assigned to this account", nextProjName)
		}
	}

	// Early return to prevent unnecessary CoW allocations
	if sess.projectID == nextProjID {
		return nil
	}

	if sess.projectID != 0 {
		removeIndex(r.projectSessions, sess.projectID, sess.id)
	}

	sessCopy := *sess
	sessCopy.projectID = nextProjID
	sessCopy.projectName = nextProjName
	r.sessions[sess.id] = &sessCopy

	if nextProjID != 0 {
		addIndex(r.projectSessions, nextProjID, sess.id)
	}
	return nil
}

// UpdateSessionURL updates a session's conversation URL via Copy-on-Write.
// sess must be the exact pointer most recently read from the registry; the
// caller must re-fetch after a successful update. The URL is normalised. An
// unchanged value is a no-op. Updated-at is not changed.
//
// Returns ErrSessionNotFound if no such session exists, or ErrSessionStale
// if sess no longer matches the live registry entry.
func (r *Registry) UpdateSessionURL(sess *Session, url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifySession(sess); err != nil {
		return fmt.Errorf("update session url: %w", err)
	}
	newURL := NormalizeText(url)
	if sess.url == newURL {
		return nil
	}
	sessCopy := *sess
	sessCopy.url = newURL
	r.sessions[sess.id] = &sessCopy
	return nil
}

// UpdateSessionStatus updates a session's lifecycle status via Copy-on-Write.
// sess must be the exact pointer most recently read from the registry; the
// caller must re-fetch after a successful update. Any valid status may
// transition to any other valid status. An unchanged value is a no-op.
// Updated-at is not changed.
//
// Returns ErrSessionNotFound if no such session exists, ErrSessionStale if
// sess no longer matches the live registry entry, or an error for an invalid
// status.
func (r *Registry) UpdateSessionStatus(sess *Session, status SessionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifySession(sess); err != nil {
		return fmt.Errorf("update session status: %w", err)
	}
	if !status.Valid() {
		return fmt.Errorf("update session status: invalid status %d", status)
	}
	if sess.status == status {
		return nil
	}
	sessCopy := *sess
	sessCopy.status = status
	r.sessions[sess.id] = &sessCopy
	return nil
}

// UpdateSessionPrompts overwrites a session's prompt queue via
// Copy-on-Write. sess must be the exact pointer most recently read from
// the registry; the caller must re-fetch after a successful update. A
// changed queue sets updated-at from the registry clock.
//
// Returns ErrSessionNotFound if no such session exists, ErrSessionStale
// if sess no longer matches the live registry entry, or an error if any
// prompt in queue is invalid.
func (r *Registry) UpdateSessionPrompts(sess *Session, queue []Prompt) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifySession(sess); err != nil {
		return fmt.Errorf("update session prompts: %w", err)
	}

	for i, p := range queue {
		if err := ValidatePrompt(p); err != nil {
			return fmt.Errorf("update session prompts: invalid prompt at index %d: %w", i, err)
		}
	}

	// slices.Equal works because Prompt contains only comparable primitive fields
	if slices.Equal(sess.queue, queue) {
		return nil
	}
	sessCopy := *sess
	sessCopy.queue = slices.Clone(queue)
	sessCopy.updatedAt = r.now().UTC()
	r.sessions[sess.id] = &sessCopy
	return nil
}

// ── Note ──────────────────────────────────────────────────────────────────────

// AddNote inserts a note for text, an optional project, and an optional
// account. The note receives the next note ID and is returned. It validates
// I3 to I6.
//
// Returns ErrTextRequired for empty text, ErrProjectNotFound or
// ErrProjectStale for an unresolved project, ErrAccountNotFound or
// ErrAccountStale for an unresolved account, or an error when the project is
// not granted to the account.
func (r *Registry) AddNote(text string, project *Project, acc *Account) (*Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fields, err := r.noteFields(text, project, acc)
	if err != nil {
		return nil, fmt.Errorf("add note: %w", err)
	}

	r.lastNoteID++
	n := newNote(r.lastNoteID, fields.text, fields.projectID, fields.projectName, fields.accountKey, fields.hasAccount)
	r.notes[n.id] = n
	return n, nil
}

// UpdateNote replaces a note's text, project and account via Copy-on-Write.
// current must be the exact pointer most recently read from the registry. It
// validates I3 to I6. When every value is unchanged the operation changes
// nothing.
//
// Returns ErrNoteNil, ErrNoteNotFound or ErrNoteStale for current, or the
// validation errors described by AddNote.
func (r *Registry) UpdateNote(current *Note, text string, project *Project, acc *Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyNote(current); err != nil {
		return fmt.Errorf("update note: current %w", err)
	}
	fields, err := r.noteFields(text, project, acc)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}

	if current.text == fields.text && current.projectID == fields.projectID &&
		current.hasAccount == fields.hasAccount && (!fields.hasAccount || current.accountKey == fields.accountKey) {
		return nil
	}

	next := *current
	next.text = fields.text
	next.projectID = fields.projectID
	next.projectName = fields.projectName
	next.accountKey = fields.accountKey
	next.hasAccount = fields.hasAccount
	r.notes[next.id] = &next
	return nil
}

// DeleteNote removes a note. Returns ErrNoteNil, ErrNoteNotFound or
// ErrNoteStale when n does not resolve.
func (r *Registry) DeleteNote(n *Note) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.verifyNote(n); err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	delete(r.notes, n.id)
	return nil
}

// Note returns the note with the given ID, or nil if not found.
func (r *Registry) Note(id uint64) *Note {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.notes[id]
}

// Notes returns all notes sorted by ID.
func (r *Registry) Notes() []*Note {
	r.mu.RLock()
	notes := slices.Collect(maps.Values(r.notes))
	r.mu.RUnlock()

	slices.SortFunc(notes, func(a, b *Note) int { return cmp.Compare(a.id, b.id) })
	return notes
}

// RestoreNote inserts a note with every field supplied, including its ID. It
// validates I3 to I6, refuses a zero or duplicate ID, and raises the note
// counter to at least id. It is used only by the JSON store.
func (r *Registry) RestoreNote(id uint64, text string, project *Project, acc *Account) (*Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if id == 0 {
		return nil, errors.New("restore note: id 0 is invalid")
	}
	if _, exists := r.notes[id]; exists {
		return nil, fmt.Errorf("restore note: id %d already exists", id)
	}
	fields, err := r.noteFields(text, project, acc)
	if err != nil {
		return nil, fmt.Errorf("restore note: %w", err)
	}

	n := newNote(id, fields.text, fields.projectID, fields.projectName, fields.accountKey, fields.hasAccount)
	r.notes[id] = n
	if id > r.lastNoteID {
		r.lastNoteID = id
	}
	return n, nil
}

// VisibleNotes returns the notes visible for acc that match filter, sorted by
// ID. The visibility rule is defined by the note visibility table.
//
// Returns ErrAccountNil, ErrAccountNotFound or ErrAccountStale when acc does
// not resolve.
func (r *Registry) VisibleNotes(acc *Account, filter ProjectFilter) ([]*Note, error) {
	r.mu.RLock()
	if err := r.verifyAccount(acc); err != nil {
		r.mu.RUnlock()
		return nil, fmt.Errorf("visible notes: %w", err)
	}

	result := make([]*Note, 0)
	for _, n := range r.notes {
		if !filter.Matches(n.projectID) {
			continue
		}
		if !r.noteVisibleForAccount(n, acc) {
			continue
		}
		result = append(result, n)
	}
	r.mu.RUnlock()

	slices.SortFunc(result, func(a, b *Note) int { return cmp.Compare(a.id, b.id) })
	return result, nil
}

// ProjectNotes returns every note whose project is p, regardless of account,
// sorted by ID.
//
// Returns ErrProjectNil, ErrProjectNotFound or ErrProjectStale when p does not
// resolve.
func (r *Registry) ProjectNotes(p *Project) ([]*Note, error) {
	r.mu.RLock()
	if err := r.verifyProject(p); err != nil {
		r.mu.RUnlock()
		return nil, fmt.Errorf("project notes: %w", err)
	}

	result := make([]*Note, 0)
	for _, n := range r.notes {
		if n.projectID == p.id {
			result = append(result, n)
		}
	}
	r.mu.RUnlock()

	slices.SortFunc(result, func(a, b *Note) int { return cmp.Compare(a.id, b.id) })
	return result, nil
}

// noteVisibleForAccount applies the note visibility table. A note with no
// account is global when it has no project or when its project has no granted
// accounts. Otherwise it is visible only to accounts granted that project.
// The caller must hold the registry read lock.
func (r *Registry) noteVisibleForAccount(n *Note, acc *Account) bool {
	if n.hasAccount {
		return n.accountKey == acc.key
	}
	if n.projectID == 0 {
		return true
	}
	if len(r.projectAccounts[n.projectID]) == 0 {
		return true
	}
	_, granted := r.accountProjects[acc.key][n.projectID]
	return granted
}

// countAccountNotes returns the number of notes assigned to key. The caller
// must hold the registry lock.
func (r *Registry) countAccountNotes(key AccountKey) int {
	count := 0
	for _, n := range r.notes {
		if n.hasAccount && n.accountKey == key {
			count++
		}
	}
	return count
}

// countProjectNotes returns the number of notes assigned to projectName. The
// caller must hold the registry lock.
func (r *Registry) countProjectNotes(projectID uint64) int {
	count := 0
	for _, n := range r.notes {
		if n.projectID == projectID {
			count++
		}
	}
	return count
}

// countAccountProjectNotes returns the number of notes assigned to both key
// and projectName. The caller must hold the registry lock.
func (r *Registry) countAccountProjectNotes(key AccountKey, projectID uint64) int {
	count := 0
	for _, n := range r.notes {
		if n.hasAccount && n.accountKey == key && n.projectID == projectID {
			count++
		}
	}
	return count
}

// noteValues holds the normalised and validated fields shared by note
// creation and update.
type noteValues struct {
	text        string
	projectID   uint64
	projectName string
	accountKey  AccountKey
	hasAccount  bool
}

// noteFields normalises and validates the fields shared by note creation and
// update. The caller must hold the registry write lock.
func (r *Registry) noteFields(text string, project *Project, acc *Account) (noteValues, error) {
	normalized := NormalizeText(text)
	if normalized == "" {
		return noteValues{}, ErrTextRequired
	}

	values := noteValues{text: normalized}
	if project != nil {
		if err := r.verifyProject(project); err != nil {
			return noteValues{}, err
		}
		values.projectID = project.id
		values.projectName = project.name
	}

	if acc != nil {
		if err := r.verifyAccount(acc); err != nil {
			return noteValues{}, err
		}
		values.accountKey = acc.key
		values.hasAccount = true
	}

	if project != nil && acc != nil {
		if _, ok := r.accountProjects[acc.key][project.id]; !ok {
			return noteValues{}, fmt.Errorf("project %q is not assigned to this account", project.name)
		}
	}

	return values, nil
}

// verifyNote returns ErrNoteNil if n is nil, ErrNoteNotFound if the note does
// not exist, or ErrNoteStale if n no longer matches the live registry entry.
//
// The caller must hold the registry lock.
func (r *Registry) verifyNote(n *Note) error {
	if n == nil {
		return ErrNoteNil
	}
	live, exists := r.notes[n.id]
	if !exists {
		return fmt.Errorf("#%d %w", n.id, ErrNoteNotFound)
	}
	if live != n {
		return fmt.Errorf("#%d %w", n.id, ErrNoteStale)
	}
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
