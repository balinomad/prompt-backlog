// Package model defines the domain entities for prompt-backlog and the
// concurrency-safe Registry that owns their lifecycle.
//
// Vendor, User, Project, and Account are identified by an immutable natural
// key. Mutable fields are updated only through Registry's Copy-on-Write
// methods. Session IDs are ephemeral surrogate keys regenerated on restart.
package model

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/balinomad/prompt-backlog/internal/apperr"
)

// TimeFormat is the canonical display and parse layout for all timestamps.
const TimeFormat = "2006-01-02 15:04"

// ParseTime parses s using TimeFormat, interpreted in the operator's
// local time zone, and returns the equivalent UTC instant. Local, not
// UTC: TimeFormat carries no zone of its own, so treating typed input as
// UTC shifts it by the local offset from what the operator actually typed.
func ParseTime(s string) (time.Time, error) {
	t, err := time.ParseInLocation(TimeFormat, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
	}
	return t.UTC(), nil
}

// FormatTime converts t to the operator's local time zone and formats it
// using TimeFormat, the inverse of ParseTime. All PauseUntil values are
// stored in UTC (see ParseTime); this is the one place conversion back to
// local time happens for display.
func FormatTime(t time.Time) string {
	return t.Local().Format(TimeFormat)
}

// NormalizeEmail trims whitespace from the email and converts it to lowercase.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizeText trims whitespace from the text.
func NormalizeText(text string) string {
	return strings.TrimSpace(text)
}

// ── Vendor ──────────────────────────────────────────────────────────────────

// Vendor describes an AI service and its capabilities.
//
// name is the natural key; it is immutable after construction.
// A name change means a new vendor, not an update.
//
// Capabilities are updated via Registry.UpdateVendor using Copy-on-Write.
type Vendor struct {
	name            string
	hasProjects     bool
	hasDeepResearch bool
}

// NewVendor constructs a Vendor. The name is permanent.
func NewVendor(name string, hasProjects, hasDeepResearch bool) *Vendor {
	return &Vendor{
		name:            NormalizeText(name),
		hasProjects:     hasProjects,
		hasDeepResearch: hasDeepResearch,
	}
}

// Name returns the vendor's immutable name.
func (p *Vendor) Name() string { return p.name }

// HasProjects returns whether the vendor supports projects.
func (p *Vendor) HasProjects() bool { return p.hasProjects }

// HasDeepResearch returns whether the vendor supports deep research.
func (p *Vendor) HasDeepResearch() bool { return p.hasDeepResearch }

// ── User ──────────────────────────────────────────────────────────────────────

// User holds identity and login credentials for one operator account.
//
// email is the natural key; it is immutable after construction.
// A changed email means a new user, not an update.
//
// Name is updated via Registry.UpdateUser using Copy-on-Write.
type User struct {
	email string
	name  string
}

// NewUser constructs a User with a normalised, immutable email.
// email is trimmed and lowercased before storage.
func NewUser(email, name string) *User {
	return &User{
		email: NormalizeEmail(email),
		name:  NormalizeText(name),
	}
}

// Email returns the user's immutable email address.
func (u *User) Email() string { return u.email }

// Name returns the user's display name.
func (u *User) Name() string { return u.name }

// ── Project ───────────────────────────────────────────────────────────────────

// Project is identified by name (natural key, globally unique across all projects).
//
// name is immutable after construction. A name change means a new project.
// Account associations are managed exclusively through Registry methods
// (AddProjectToAccount, RemoveProjectFromAccount), which enforce vendor
// capability validation (Vendor.HasProjects).
//
// Description and Instructions are updated via Registry.UpdateProject using Copy-on-Write.
type Project struct {
	id           uint64
	name         string
	description  string
	instructions string
}

// NewProject constructs a Project with a normalised name and a zero ID. The
// registry assigns the ID when the project is added or restored.
func NewProject(name, description, instructions string) *Project {
	return newProject(0, name, description, instructions)
}

// newProject constructs a Project with every field set.
func newProject(id uint64, name, description, instructions string) *Project {
	return &Project{
		id:           id,
		name:         NormalizeText(name),
		description:  NormalizeText(description),
		instructions: NormalizeText(instructions),
	}
}

// ID returns the project's surrogate key.
func (p *Project) ID() uint64 { return p.id }

// Name returns the project's name.
func (p *Project) Name() string { return p.name }

// Description returns the project's description.
func (p *Project) Description() string { return p.description }

// Instructions returns the project's instructions.
func (p *Project) Instructions() string { return p.instructions }

// ── Account ───────────────────────────────────────────────────────────────────

// AccountKey is the Account's composite natural key (based on user and vendor).
type AccountKey struct {
	Email  string
	Vendor string
}

// String returns a string representation of the AccountKey, used as a
// stable sort key when ordering accounts.
func (k AccountKey) String() string {
	return k.Email + "|" + k.Vendor
}

// Account represents a unique User + Vendor combination.
// It is the primary owner of sessions and holds its pause state.
type Account struct {
	id         uint64
	key        AccountKey
	pauseUntil time.Time
}

// NewAccount constructs an Account using natural keys with a zero ID. The
// registry assigns the ID when the account is added or restored.
func NewAccount(userEmail, vendorName string) *Account {
	return newAccount(0, userEmail, vendorName)
}

// newAccount constructs an Account with every field set.
func newAccount(id uint64, userEmail, vendorName string) *Account {
	return &Account{
		id: id,
		key: AccountKey{
			Email:  NormalizeEmail(userEmail),
			Vendor: NormalizeText(vendorName),
		},
	}
}

// ID returns the account's surrogate key.
func (a *Account) ID() uint64 { return a.id }

// Key returns the account's immutable composite key.
func (a *Account) Key() AccountKey { return a.key }

// UserEmail returns the email of the associated user.
func (a *Account) UserEmail() string { return a.key.Email }

// VendorName returns the name of the associated vendor.
func (a *Account) VendorName() string { return a.key.Vendor }

// PauseUntil returns the time until which the account is paused. No
// session under this account should be used before this time.
func (a *Account) PauseUntil() time.Time { return a.pauseUntil }

// ── Session ───────────────────────────────────────────────────────────────────

// SessionStatus describes the lifecycle state of a session.
type SessionStatus int

const (
	// SessionStatusActive is the normal state of a new session.
	SessionStatusActive SessionStatus = iota
	// SessionStatusNeedsClosing marks a session that should be closed soon.
	SessionStatusNeedsClosing
	// SessionStatusClosed marks a session that has been closed.
	SessionStatusClosed
)

// Name returns the canonical persistence value of s.
func (s SessionStatus) Name() (string, error) {
	switch s {
	case SessionStatusActive:
		return "active", nil
	case SessionStatusNeedsClosing:
		return "needs_closing", nil
	case SessionStatusClosed:
		return "closed", nil
	default:
		return "", apperr.Invariantf("session status %d is not valid", s)
	}
}

// String returns the canonical persistence value of s. Invalid values render
// as a diagnostic rather than a plausible status.
func (s SessionStatus) String() string {
	name, err := s.Name()
	if err != nil {
		return fmt.Sprintf("SessionStatus(%d)", s)
	}
	return name
}

// Valid reports whether s is a known session status.
func (s SessionStatus) Valid() bool {
	return s >= SessionStatusActive && s <= SessionStatusClosed
}

// ParseSessionStatus parses a canonical persistence value.
func ParseSessionStatus(value string) (SessionStatus, error) {
	switch value {
	case "active":
		return SessionStatusActive, nil
	case "needs_closing":
		return SessionStatusNeedsClosing, nil
	case "closed":
		return SessionStatusClosed, nil
	default:
		return SessionStatusActive, fmt.Errorf("invalid session status %q", value)
	}
}

// Session holds the runtime state of one active conversation.
type Session struct {
	accountKey  AccountKey
	accountID   uint64
	id          uint64
	projectID   uint64
	projectName string
	name        string
	url         string
	status      SessionStatus
	updatedAt   time.Time
	queue       []Prompt
}

// NewSession constructs a Session for the given account. id is the
// surrogate key assigned by Registry.AddSession; projectName, name, and
// queue are the session's initial mutable state. URL starts empty, status
// starts active, and updated-at starts zero.
func NewSession(accKey AccountKey, id uint64, projectName, name string, queue []Prompt) *Session {
	return newSession(accKey, 0, 0, id, projectName, name, "", SessionStatusActive, time.Time{}, queue)
}

// newSession constructs a Session with every field set. The registry uses it
// for creation and restore.
func newSession(accKey AccountKey, accountID, projectID, id uint64, projectName, name, url string, status SessionStatus, updatedAt time.Time, queue []Prompt) *Session {
	return &Session{
		accountKey:  accKey,
		accountID:   accountID,
		id:          id,
		projectID:   projectID,
		projectName: NormalizeText(projectName),
		name:        NormalizeText(name),
		url:         NormalizeText(url),
		status:      status,
		updatedAt:   updatedAt,
		queue:       slices.Clone(queue),
	}
}

// ID returns the session's surrogate key.
func (s *Session) ID() uint64 { return s.id }

// AccountKey returns the session's account key.
func (s *Session) AccountKey() AccountKey { return s.accountKey }

// AccountID returns the session's account surrogate key.
func (s *Session) AccountID() uint64 { return s.accountID }

// ProjectID returns the session's project surrogate key, or 0 for no project.
func (s *Session) ProjectID() uint64 { return s.projectID }

// ProjectName returns the name of the project associated with the session, or an empty string.
func (s *Session) ProjectName() string { return s.projectName }

// Name returns the session's name.
func (s *Session) Name() string { return s.name }

// URL returns the session's conversation URL, or an empty string.
func (s *Session) URL() string { return s.url }

// Status returns the session's lifecycle status.
func (s *Session) Status() SessionStatus { return s.status }

// UpdatedAt returns the time of the session's creation or last queue change,
// or the zero time for a session restored without that field.
func (s *Session) UpdatedAt() time.Time { return s.updatedAt }

// Queue returns a copy of the prompts queued for this session.
func (s *Session) Queue() []Prompt {
	return slices.Clone(s.queue)
}

// ── Prompt ────────────────────────────────────────────────────────────────────

// Prompt represents a queued action for a session.
type Prompt struct {
	text string // Example: "continue", "Proceed to step 2"
}

// NewPrompt constructs a Prompt.
func NewPrompt(text string) Prompt {
	return Prompt{text: NormalizeText(text)}
}

// Text returns the prompt's action text.
func (p Prompt) Text() string { return p.text }

// ── Note ──────────────────────────────────────────────────────────────────────

// Note is a stored future work item. It is never attached to a session.
//
// A note is immutable after construction. Every change replaces the registry
// entry with a modified copy.
type Note struct {
	id          uint64
	text        string
	projectID   uint64
	projectName string
	accountKey  AccountKey
	hasAccount  bool
}

// newNote constructs a Note with every field set. The registry uses it for
// creation and restore.
func newNote(id uint64, text string, projectID uint64, projectName string, accountKey AccountKey, hasAccount bool) *Note {
	return &Note{
		id:          id,
		text:        NormalizeText(text),
		projectID:   projectID,
		projectName: NormalizeText(projectName),
		accountKey:  accountKey,
		hasAccount:  hasAccount,
	}
}

// NewNote constructs a Note with the given ID and fields. The registry
// normally assigns IDs; this constructor exists for restore and tests.
func NewNote(id uint64, text string, projectID uint64, projectName string, accountKey AccountKey, hasAccount bool) *Note {
	return newNote(id, text, projectID, projectName, accountKey, hasAccount)
}

// ID returns the note's surrogate key.
func (n *Note) ID() uint64 { return n.id }

// Text returns the note's text.
func (n *Note) Text() string { return n.text }

// ProjectID returns the note's project surrogate key, or 0 for no project.
func (n *Note) ProjectID() uint64 { return n.projectID }

// ProjectName returns the name of the project associated with the note, or an
// empty string.
func (n *Note) ProjectName() string { return n.projectName }

// AccountKey returns the note's account key and whether the note has an
// account.
func (n *Note) AccountKey() (AccountKey, bool) {
	return n.accountKey, n.hasAccount
}

// ProjectFilter selects items by project name. The zero value selects every
// item.
type ProjectFilter struct {
	kind ProjectFilterKind
	id   uint64
}

// ProjectFilterKind identifies the kind of project filter.
type ProjectFilterKind uint8

const (
	// ProjectFilterAll accepts every project.
	ProjectFilterAll ProjectFilterKind = iota
	// ProjectFilterNone accepts only an empty project name.
	ProjectFilterNone
	// ProjectFilterNamed accepts only one project ID.
	ProjectFilterNamed
)

// AllProjectFilter returns a filter that accepts every project name.
func AllProjectFilter() ProjectFilter {
	return ProjectFilter{kind: ProjectFilterAll}
}

// NoneProjectFilter returns a filter that accepts only an empty project name.
func NoneProjectFilter() ProjectFilter {
	return ProjectFilter{kind: ProjectFilterNone}
}

// NamedProjectFilter returns a filter that accepts only projectID.
func NamedProjectFilter(projectID uint64) ProjectFilter {
	return ProjectFilter{kind: ProjectFilterNamed, id: projectID}
}

// Kind returns the filter's kind.
func (f ProjectFilter) Kind() ProjectFilterKind {
	return f.kind
}

// Validate returns an invariant error when the filter holds an unknown kind.
func (f ProjectFilter) Validate() error {
	switch f.kind {
	case ProjectFilterAll, ProjectFilterNone, ProjectFilterNamed:
		return nil
	default:
		return apperr.Invariantf("project filter kind %d is not valid", f.kind)
	}
}

// Matches reports whether projectID satisfies the filter.
func (f ProjectFilter) Matches(projectID uint64) (bool, error) {
	if err := f.Validate(); err != nil {
		return false, err
	}
	switch f.kind {
	case ProjectFilterAll:
		return true, nil
	case ProjectFilterNone:
		return projectID == 0, nil
	case ProjectFilterNamed:
		return projectID == f.id, nil
	default:
		return false, apperr.Invariantf("project filter kind %d is not valid", f.kind)
	}
}

// IsAll reports whether the filter accepts every project name.
func (f ProjectFilter) IsAll() bool {
	return f.kind == ProjectFilterAll
}

// IsNone reports whether the filter accepts only an empty project name.
func (f ProjectFilter) IsNone() bool {
	return f.kind == ProjectFilterNone
}

// ProjectID returns the filter's project ID and whether the filter names one.
func (f ProjectFilter) ProjectID() (uint64, bool) {
	if f.kind != ProjectFilterNamed {
		return 0, false
	}
	return f.id, true
}
