package model

import "time"

// TimeFormat is the canonical display and parse format for all timestamps.
// Using Go's reference time: Mon Jan 2 15:04:05 MST 2006.
const TimeFormat = "2006-01-02 15:04"

// Registry is the root persistent document.
// All fields are unexported to protect the dual session indexes from
// falling out of sync. All access is via methods only.
//
// In-place mutation via pointer is the only permitted update pattern;
// map values are never replaced, only mutated through their pointer.
// This guarantees that pointer references held by other structs (e.g.
// Session.user, Session.provider, Project.session) remain valid for
// the lifetime of the registry.
type Registry struct {
	providers          map[string]*Provider
	users              map[string]*User
	sessionsByUser     map[string]map[string]*Session // [userEmail][providerName]
	sessionsByProvider map[string]map[string]*Session // [providerName][userEmail]
	projects           map[string]*Project
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
	HasProjects     bool `json:"has_projects"`
	HasDeepResearch bool `json:"has_deep_research"`
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
	Name  string `json:"name"`
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
	LastQuestion *LastQuestion `json:"last_question,omitempty"`
	NextPromptAt *time.Time    `json:"next_prompt_at,omitempty"`
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
//
// session is set at construction and may be changed only through registry
// methods (attach/detach), which enforce provider capability validation.
// It is nil for standalone projects.
//
// Description and Instructions are in-place mutable via direct field access.
type Project struct {
	name         string
	session      *Session
	Description  string `json:"description,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

// NewProject constructs a standalone project (session is nil).
func NewProject(name string) *Project {
	return &Project{name: name}
}

// NewSessionProject constructs a project attached to a session.
// session must be non-nil.
func NewSessionProject(name string, session *Session) *Project {
	return &Project{name: name, session: session}
}

// Name returns the project's immutable name.
func (p *Project) Name() string { return p.name }

// Session returns the session this project belongs to, or nil if standalone.
func (p *Project) Session() *Session { return p.session }

// IsUnattached reports whether the project is not attached to any session.
func (p *Project) IsUnattached() bool { return p.session == nil }

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
	Topic      string `json:"topic,omitempty"`
	MidAnswer  bool   `json:"mid_answer"`
	NextPrompt string `json:"next_prompt,omitempty"`
}

// NewLastQuestion constructs a LastQuestion. project may be nil.
func NewLastQuestion(project *Project) *LastQuestion {
	return &LastQuestion{project: project}
}

// Project returns the associated project, or nil if the question was off-topic.
func (lq *LastQuestion) Project() *Project { return lq.project }

// ParseTime parses s using TimeFormat in the local timezone.
func ParseTime(s string) (time.Time, error) {
	return time.ParseInLocation(TimeFormat, s, time.Local)
}

// FormatTime formats t using TimeFormat.
func FormatTime(t time.Time) string {
	return t.Format(TimeFormat)
}
