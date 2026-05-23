package model

import "fmt"

// ── Provider ──────────────────────────────────────────────────────────────────

// AddProvider inserts a provider into the registry.
// Returns an error when the provider is nil, has an empty name, or a provider
// with the same name already exists.
func (r *Registry) AddProvider(p *Provider) error {
	if p == nil {
		return fmt.Errorf("add provider: nil pointer")
	}
	if p.name == "" {
		return fmt.Errorf("add provider: name required")
	}
	if _, exists := r.providers[p.name]; exists {
		return fmt.Errorf("add provider: %q already exists", p.name)
	}
	r.providers[p.name] = p
	return nil
}

// ProviderByName returns the provider with the given name, or nil if not found.
func (r *Registry) ProviderByName(name string) *Provider {
	return r.providers[name]
}

// AllProviders returns all providers. Order is not guaranteed; callers sort
// for display.
func (r *Registry) AllProviders() []*Provider {
	result := make([]*Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

// RemoveProvider removes a provider from the registry.
// Returns an error when the provider is not found or has active sessions.
// All sessions for this provider must be removed before it can be removed.
func (r *Registry) RemoveProvider(name string) error {
	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("remove provider: %q not found", name)
	}
	if len(r.sessionsByProvider[name]) > 0 {
		return fmt.Errorf("remove provider: %q has active sessions; remove them first", name)
	}
	delete(r.providers, name)
	return nil
}

// ── User ──────────────────────────────────────────────────────────────────────

// AddUser inserts a user into the registry.
// Returns an error when the user is nil, has an empty email, or a user with
// the same email already exists.
func (r *Registry) AddUser(u *User) error {
	if u == nil {
		return fmt.Errorf("add user: nil pointer")
	}
	if u.email == "" {
		return fmt.Errorf("add user: email required")
	}
	if _, exists := r.users[u.email]; exists {
		return fmt.Errorf("add user: %q already exists", u.email)
	}
	r.users[u.email] = u
	return nil
}

// UserByEmail returns the user with the given email, or nil if not found.
func (r *Registry) UserByEmail(email string) *User {
	return r.users[email]
}

// AllUsers returns all users. Order is not guaranteed; callers sort for display.
func (r *Registry) AllUsers() []*User {
	result := make([]*User, 0, len(r.users))
	for _, u := range r.users {
		result = append(result, u)
	}
	return result
}

// RemoveUser removes a user from the registry.
// Returns an error when the user is not found or has active sessions.
// All sessions for this user must be removed before the user can be removed.
func (r *Registry) RemoveUser(email string) error {
	if _, exists := r.users[email]; !exists {
		return fmt.Errorf("remove user: %q not found", email)
	}
	if len(r.sessionsByUser[email]) > 0 {
		return fmt.Errorf("remove user: %q has active sessions; remove them first", email)
	}
	delete(r.users, email)
	return nil
}

// ── Session ───────────────────────────────────────────────────────────────────

// AddSession inserts a session into both session indexes.
// Returns an error when the session is nil, its user or provider is nil,
// the user or provider is not registered in this registry, or a session
// for the same user+provider pair already exists.
// Pointer identity is used to verify registry membership.
func (r *Registry) AddSession(s *Session) error {
	if s == nil {
		return fmt.Errorf("add session: nil pointer")
	}
	if s.user == nil {
		return fmt.Errorf("add session: user is nil")
	}
	if s.provider == nil {
		return fmt.Errorf("add session: provider is nil")
	}
	if r.users[s.user.email] != s.user {
		return fmt.Errorf("add session: user %q is not registered in this registry",
			s.user.email)
	}
	if r.providers[s.provider.name] != s.provider {
		return fmt.Errorf("add session: provider %q is not registered in this registry",
			s.provider.name)
	}
	if r.SessionAt(s.user.email, s.provider.name) != nil {
		return fmt.Errorf("add session: a session for %q/%q already exists",
			s.user.email, s.provider.name)
	}
	if r.sessionsByUser[s.user.email] == nil {
		r.sessionsByUser[s.user.email] = make(map[string]*Session)
	}
	r.sessionsByUser[s.user.email][s.provider.name] = s
	if r.sessionsByProvider[s.provider.name] == nil {
		r.sessionsByProvider[s.provider.name] = make(map[string]*Session)
	}
	r.sessionsByProvider[s.provider.name][s.user.email] = s
	return nil
}

// SessionAt returns the session for the given user email and provider name,
// or nil when no such session exists. O(1) in both indexes.
func (r *Registry) SessionAt(userEmail, providerName string) *Session {
	providers := r.sessionsByUser[userEmail]
	if providers == nil {
		return nil
	}
	return providers[providerName]
}

// AllSessions returns all sessions across all users and providers.
// Order is not guaranteed; callers sort for display.
func (r *Registry) AllSessions() []*Session {
	result := make([]*Session, 0)
	for _, providers := range r.sessionsByUser {
		for _, s := range providers {
			result = append(result, s)
		}
	}
	return result
}

// SessionsByUser returns all sessions for the given user email.
// Returns a non-nil empty slice when no sessions are found. O(1) outer lookup.
func (r *Registry) SessionsByUser(email string) []*Session {
	result := make([]*Session, 0)
	for _, s := range r.sessionsByUser[email] {
		result = append(result, s)
	}
	return result
}

// SessionsByProvider returns all sessions for the given provider name.
// Returns a non-nil empty slice when no sessions are found. O(1) outer lookup.
func (r *Registry) SessionsByProvider(providerName string) []*Session {
	result := make([]*Session, 0)
	for _, s := range r.sessionsByProvider[providerName] {
		result = append(result, s)
	}
	return result
}

// RemoveSession removes a session from both indexes.
// Projects attached to this session are detached rather than deleted;
// they become unattached projects.
func (r *Registry) RemoveSession(userEmail, providerName string) error {
	s := r.SessionAt(userEmail, providerName)
	if s == nil {
		return fmt.Errorf("remove session: no session for %q/%q", userEmail, providerName)
	}
	for _, p := range r.projects {
		if p.session == s {
			p.session = nil // in-place mutation: project becomes unattached
		}
	}
	delete(r.sessionsByUser[userEmail], providerName)
	if len(r.sessionsByUser[userEmail]) == 0 {
		delete(r.sessionsByUser, userEmail)
	}
	delete(r.sessionsByProvider[providerName], userEmail)
	if len(r.sessionsByProvider[providerName]) == 0 {
		delete(r.sessionsByProvider, providerName)
	}
	return nil
}

// ── Project ───────────────────────────────────────────────────────────────────

// AddProject inserts a project into the registry.
// Returns an error when the project is nil, has an empty name, a project with
// the same name already exists, or the session's provider does not support
// projects.
func (r *Registry) AddProject(p *Project) error {
	if p == nil {
		return fmt.Errorf("add project: nil pointer")
	}
	if p.name == "" {
		return fmt.Errorf("add project: name required")
	}
	if _, exists := r.projects[p.name]; exists {
		return fmt.Errorf("add project: %q already exists", p.name)
	}
	if p.session != nil && !p.session.provider.HasProjects {
		return fmt.Errorf("add project: provider %q does not support projects",
			p.session.provider.name)
	}
	r.projects[p.name] = p
	return nil
}

// ProjectByName returns the project with the given name, or nil if not found.
func (r *Registry) ProjectByName(name string) *Project {
	return r.projects[name]
}

// AllProjects returns all projects. Order is not guaranteed; callers sort
// for display.
func (r *Registry) AllProjects() []*Project {
	result := make([]*Project, 0, len(r.projects))
	for _, p := range r.projects {
		result = append(result, p)
	}
	return result
}

// UnattachedProjects returns all projects not attached to any session.
// Returns a non-nil empty slice when none are found.
func (r *Registry) UnattachedProjects() []*Project {
	result := make([]*Project, 0)
	for _, p := range r.projects {
		if p.session == nil {
			result = append(result, p)
		}
	}
	return result
}

// ProjectsBySession returns all projects attached to the given session.
// Returns a non-nil empty slice when none are found.
func (r *Registry) ProjectsBySession(s *Session) []*Project {
	result := make([]*Project, 0)
	for _, p := range r.projects {
		if p.session == s {
			result = append(result, p)
		}
	}
	return result
}

// AttachProject sets the session for an existing project.
// Returns an error when the project is not found, the session is nil, or
// the session's provider does not support projects.
// To make a project unattached, use DetachProject.
func (r *Registry) AttachProject(projectName string, session *Session) error {
	p := r.projects[projectName]
	if p == nil {
		return fmt.Errorf("attach project: %q not found", projectName)
	}
	if session == nil {
		return fmt.Errorf("attach project: session is nil — use DetachProject to make a project unattached")
	}
	if !session.provider.HasProjects {
		return fmt.Errorf("attach project: provider %q does not support projects",
			session.provider.name)
	}
	p.session = session // in-place mutation
	return nil
}

// DetachProject makes an existing project unattached by clearing its session.
// The project remains in the registry.
func (r *Registry) DetachProject(projectName string) error {
	p := r.projects[projectName]
	if p == nil {
		return fmt.Errorf("detach project: %q not found", projectName)
	}
	p.session = nil // in-place mutation
	return nil
}

// RemoveProject removes a project from the registry.
// Any session whose LastQuestion references this project has its project
// pointer cleared to prevent a dangling reference.
func (r *Registry) RemoveProject(name string) error {
	p, exists := r.projects[name]
	if !exists {
		return fmt.Errorf("remove project: %q not found", name)
	}
	for _, providers := range r.sessionsByUser {
		for _, s := range providers {
			if s.LastQuestion != nil && s.LastQuestion.project == p {
				s.LastQuestion.project = nil // in-place mutation
			}
		}
	}
	delete(r.projects, name)
	return nil
}
