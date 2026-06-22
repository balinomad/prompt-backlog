// Package model defines the domain entities for llm-desk — Provider,
// User, Account, Project, and Session — plus Registry, the
// concurrency-safe in-memory store that owns their lifecycle.
//
// Provider, User, Project, and Account are identified by an immutable
// natural key fixed at construction. Mutable fields are updated only
// through Registry's Copy-on-Write methods.
//
// All entities, including Sessions and their queued Prompts, are fully
// persistent domain data. The only ephemeral state is the Session ID,
// which acts as a temporary surrogate key for UI convenience and is
// regenerated sequentially whenever the process restarts.
package model

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// TimeFormat is the canonical display and parse format for all timestamps.
// Using Go's reference time: Mon Jan 2 15:04:05 MST 2006.
const TimeFormat = "2006-01-02 15:04"

// ParseTime parses s using TimeFormat in UTC.
func ParseTime(s string) (time.Time, error) {
	t, err := time.ParseInLocation(TimeFormat, s, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
	}
	return t, nil
}

// FormatTime formats t using TimeFormat.
func FormatTime(t time.Time) string {
	return t.Format(TimeFormat)
}

// NormalizeEmail trims whitespace from the email and converts it to lowercase.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizeText trims whitespace from the text.
func NormalizeText(text string) string {
	return strings.TrimSpace(text)
}

// ── Provider ──────────────────────────────────────────────────────────────────

// Provider describes an AI service and its capabilities.
//
// name is the natural key; it is immutable after construction.
// A name change means a new provider, not an update.
//
// Capabilities are updated via Registry.UpdateProvider using Copy-on-Write.
type Provider struct {
	name            string
	hasProjects     bool
	hasDeepResearch bool
}

// NewProvider constructs a Provider. The name is permanent.
func NewProvider(name string, hasProjects, hasDeepResearch bool) *Provider {
	return &Provider{
		name:            NormalizeText(name),
		hasProjects:     hasProjects,
		hasDeepResearch: hasDeepResearch,
	}
}

// Name returns the provider's immutable name.
func (p *Provider) Name() string { return p.name }

// HasProjects returns whether the provider supports projects.
func (p *Provider) HasProjects() bool { return p.hasProjects }

// HasDeepResearch returns whether the provider supports deep research.
func (p *Provider) HasDeepResearch() bool { return p.hasDeepResearch }

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
// (AddProjectToAccount, RemoveProjectFromAccount), which enforce provider
// capability validation (Provider.HasProjects).
//
// Description and Instructions are updated via Registry.UpdateProject using Copy-on-Write.
type Project struct {
	name         string
	description  string
	instructions string
}

// NewProject constructs a Project with a normalised, immutable name.
// name is trimmed before storage.
func NewProject(name, description, instructions string) *Project {
	return &Project{
		name:         NormalizeText(name),
		description:  NormalizeText(description),
		instructions: NormalizeText(instructions),
	}
}

// Name returns the project's immutable name.
func (p *Project) Name() string { return p.name }

// Description returns the project's description.
func (p *Project) Description() string { return p.description }

// Instructions returns the project's instructions.
func (p *Project) Instructions() string { return p.instructions }

// ── Account ───────────────────────────────────────────────────────────────────

// AccountKey is the Account's composite natural key (based on user and provider).
type AccountKey struct {
	Email    string
	Provider string
}

// String returns a string representation of the AccountKey, used as a
// stable sort key when ordering accounts.
func (k AccountKey) String() string {
	return k.Email + "|" + k.Provider
}

// Account represents a unique User + Provider combination.
// It is the primary owner of sessions and holds its pause state.
type Account struct {
	key        AccountKey
	pauseUntil time.Time
}

// NewAccount constructs an Account using natural keys.
func NewAccount(userEmail, providerName string) *Account {
	return &Account{
		key: AccountKey{
			Email:    NormalizeEmail(userEmail),
			Provider: NormalizeText(providerName),
		},
	}
}

// Key returns the account's immutable composite key.
func (a *Account) Key() AccountKey { return a.key }

// UserEmail returns the email of the associated user.
func (a *Account) UserEmail() string { return a.key.Email }

// ProviderName returns the name of the associated provider.
func (a *Account) ProviderName() string { return a.key.Provider }

// PauseUntil returns the time until which the account is paused. No
// session under this account should be used before this time.
func (a *Account) PauseUntil() time.Time { return a.pauseUntil }

// ── Session ───────────────────────────────────────────────────────────────────

// Session holds the runtime state of one active conversation.
type Session struct {
	accountKey  AccountKey
	id          uint64
	projectName string   // Optional; empty if no project is assigned
	topic       string   // High-level subject
	queue       []Prompt // Actions to execute upon resumption
}

// NewSession constructs a Session for the given account. id is the
// ephemeral surrogate key assigned by Registry.AddSession; projectName,
// topic, and queue are the session's initial mutable state.
func NewSession(accKey AccountKey, id uint64, projectName, topic string, queue []Prompt) *Session {
	return &Session{
		accountKey:  accKey,
		id:          id,
		projectName: NormalizeText(projectName),
		topic:       NormalizeText(topic),
		queue:       slices.Clone(queue),
	}
}

// ID returns the session's ephemeral surrogate key.
func (s *Session) ID() uint64 { return s.id }

// AccountKey returns the session's account key.
func (s *Session) AccountKey() AccountKey { return s.accountKey }

// ProjectName returns the name of the project associated with the session, or an empty string.
func (s *Session) ProjectName() string { return s.projectName }

// Topic returns the session's high-level subject.
func (s *Session) Topic() string { return s.topic }

// Queue returns a copy of the pending prompts queued for this session.
func (s *Session) Queue() []Prompt {
	return slices.Clone(s.queue)
}

// ── Prompt ────────────────────────────────────────────────────────────────────

// Prompt represents a queued action for a session.
type Prompt struct {
	text          string // e.g., "continue", "Proceed to step 2"
	spawnsSession bool   // If true, close current session and spawn a new one
}

// NewPrompt constructs a Prompt.
func NewPrompt(text string, spawnsSession bool) Prompt {
	return Prompt{
		text:          NormalizeText(text),
		spawnsSession: spawnsSession,
	}
}

// Text returns the prompt's action text.
func (p Prompt) Text() string { return p.text }

// SpawnsSession returns whether the prompt requires a new session.
func (p Prompt) SpawnsSession() bool { return p.spawnsSession }
