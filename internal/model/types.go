// Package model contains types and data structures for the data model.
package model

import (
	"fmt"
	"time"
)

// TimeFormat is the canonical display and parse format for all timestamps.
// Using Go's reference time: Mon Jan 2 15:04:05 MST 2006.
const TimeFormat = "2006-01-02 15:04"

// Registry is the root persistent document.
// All fields are unexported to protect the dual session indexes and the
// project-session indexes from falling out of sync. All access is via methods only.
//
// In-place mutation via pointer is the only permitted update pattern;
// map values are never replaced, only mutated through their pointer.
// This guarantees that pointer references held by other structs (e.g.
// Session.user, Session.provider, LastQuestion.project) remain valid for
// the lifetime of the registry.
type Registry struct {
	providers          map[string]*Provider
	users              map[string]*User
	sessionsByUser     map[string]map[string]*Session // [userEmail][providerName]
	sessionsByProvider map[string]map[string]*Session // [providerName][userEmail]
	projects           map[string]*Project
	// Many-to-many project↔session association indexes.
	// sessionsByProject[projectName][userEmail][providerName] → *Session
	// projectsBySession[userEmail][providerName][projectName] → *Project
	sessionsByProject map[string]map[string]map[string]*Session
	projectsBySession map[string]map[string]map[string]*Project
}

// NewRegistry returns an empty Registry with all maps initialised.
// At least one Provider must be added before Users or Sessions can be created.
func NewRegistry() Registry {
	return Registry{
		providers:          make(map[string]*Provider),
		users:              make(map[string]*User),
		sessionsByUser:     make(map[string]map[string]*Session),
		sessionsByProvider: make(map[string]map[string]*Session),
		projects:           make(map[string]*Project),
		sessionsByProject:  make(map[string]map[string]map[string]*Session),
		projectsBySession:  make(map[string]map[string]map[string]*Project),
	}
}

// Provider describes an AI service and its capabilities.
//
// name is the natural key; it is immutable after construction.
// A name change means a new provider, not an update.
//
// HasProjects and HasDeepResearch are in-place mutable via direct field access.
type Provider struct {
	name            string
	HasProjects     bool
	HasDeepResearch bool
}

// NewProvider constructs a Provider. name is permanent.
func NewProvider(name string, hasProjects, hasDeepResearch bool) *Provider {
	return &Provider{
		name:            name,
		HasProjects:     hasProjects,
		HasDeepResearch: hasDeepResearch,
	}
}

// Name returns the provider's immutable name.
func (p *Provider) Name() string { return p.name }

// User holds identity and login credentials for one operator account.
//
// email is the natural key; it is immutable after construction.
// A changed email means a new user, not an update.
//
// Name is in-place mutable via direct field access.
type User struct {
	email string
	Name  string
}

// NewUser constructs a User. email is permanent.
func NewUser(email, name string) *User {
	return &User{email: email, Name: name}
}

// Email returns the user's immutable email address.
func (u *User) Email() string { return u.email }

// Session holds the runtime state of one active conversation.
//
// user and provider are immutable pointer references set at construction.
// They are safe to follow at any time because User and Provider are only
// ever mutated in place; they are never replaced in their registry maps.
//
// LastQuestion and NextPromptAt are in-place mutable via direct field access.
type Session struct {
	user         *User
	provider     *Provider
	LastQuestion *LastQuestion
	NextPromptAt *time.Time
}

// NewSession constructs a Session. Both user and provider must be non-nil.
func NewSession(user *User, provider *Provider) *Session {
	return &Session{user: user, provider: provider}
}

// User returns the session's user. Never nil for a valid session.
func (s *Session) User() *User { return s.user }

// Provider returns the session's provider. Never nil for a valid session.
func (s *Session) Provider() *Provider { return s.provider }

// Project is identified by name (natural key, globally unique across all projects).
//
// name is immutable after construction. A name change means a new project.
// Session associations are managed exclusively through Registry methods
// (AttachProject, DetachProject), which enforce provider capability validation.
// A project with no associated sessions is informally called unattached.
//
// Description and Instructions are in-place mutable via direct field access.
type Project struct {
	name         string
	Description  string
	Instructions string
}

// NewProject constructs a Project with no session associations.
func NewProject(name string) *Project {
	return &Project{name: name}
}

// Name returns the project's immutable name.
func (p *Project) Name() string { return p.name }

// LastQuestion records where a session was interrupted.
//
// project is a pointer reference set at construction and never changed;
// the entire LastQuestion is replaced when the session state changes.
// It is nil when the question was not related to any project.
//
// Topic, MidAnswer, and NextPrompt are in-place mutable via direct field access.
//
// Valid combinations:
//   - project nil, Topic set: last reply finished, no continuation planned.
//   - MidAnswer true: reply was cut off; send "continue" first on resuming.
//   - NextPrompt set: follow-up to send after resuming (after "continue" if
//     MidAnswer is also true).
//   - project non-nil: exchange was in the context of that Project.
type LastQuestion struct {
	project    *Project
	Topic      string
	MidAnswer  bool
	NextPrompt string
}

// NewLastQuestion constructs a LastQuestion. project may be nil.
func NewLastQuestion(project *Project) *LastQuestion {
	return &LastQuestion{project: project}
}

// Project returns the associated project, or nil if the question was off-topic.
func (lq *LastQuestion) Project() *Project { return lq.project }

// ParseTime parses s using TimeFormat in the local timezone.
func ParseTime(s string) (time.Time, error) {
	t, err := time.ParseInLocation(TimeFormat, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
	}
	return t, nil
}

// FormatTime formats t using TimeFormat.
func FormatTime(t time.Time) string {
	return t.Format(TimeFormat)
}
