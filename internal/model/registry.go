package model

import (
	"fmt"
	"strings"
)

// ── Provider ──────────────────────────────────────────────────────────────────

// AddProvider inserts a provider into the registry.
// Returns an error when the provider is nil, has an empty name, or a provider
// with the same name already exists.
func (r *Registry) AddProvider(p *Provider) error {
	if p == nil {
		return fmt.Errorf("add provider: nil pointer")
	}
	if strings.TrimSpace(p.name) == "" {
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
	if strings.TrimSpace(u.email) == "" {
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

// RemoveSession removes a session from both session indexes and detaches it
// from all projects that referenced it.
func (r *Registry) RemoveSession(userEmail, providerName string) error {
	s := r.SessionAt(userEmail, providerName)
	if s == nil {
		return fmt.Errorf("remove session: no session for %q/%q", userEmail, providerName)
	}
	// Detach this session from every project that referenced it.
	for projectName := range r.projectsBySession[userEmail][providerName] {
		delete(r.sessionsByProject[projectName][userEmail], providerName)
		if len(r.sessionsByProject[projectName][userEmail]) == 0 {
			delete(r.sessionsByProject[projectName], userEmail)
		}
		if len(r.sessionsByProject[projectName]) == 0 {
			delete(r.sessionsByProject, projectName)
		}
	}
	delete(r.projectsBySession[userEmail], providerName)
	if len(r.projectsBySession[userEmail]) == 0 {
		delete(r.projectsBySession, userEmail)
	}
	// Remove from session indexes.
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
// Returns an error when the project is nil, has an empty name, or a project
// with the same name already exists.
// The project starts with no session associations; use AttachProject to
// associate it with one or more sessions.
func (r *Registry) AddProject(p *Project) error {
	if p == nil {
		return fmt.Errorf("add project: nil pointer")
	}
	if strings.TrimSpace(p.name) == "" {
		return fmt.Errorf("add project: name required")
	}
	if _, exists := r.projects[p.name]; exists {
		return fmt.Errorf("add project: %q already exists", p.name)
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

// UnattachedProjects returns all projects not associated with any session.
// Returns a non-nil empty slice when none are found.
func (r *Registry) UnattachedProjects() []*Project {
	result := make([]*Project, 0)
	for name, p := range r.projects {
		if len(r.sessionsByProject[name]) == 0 {
			result = append(result, p)
		}
	}
	return result
}

// ProjectsBySession returns all projects associated with the given session.
// Returns a non-nil empty slice when none are found or s is nil.
func (r *Registry) ProjectsBySession(s *Session) []*Project {
	result := make([]*Project, 0)
	if s == nil {
		return result
	}
	for _, p := range r.projectsBySession[s.user.email][s.provider.name] {
		result = append(result, p)
	}
	return result
}

// SessionsByProject returns all sessions associated with the given project.
// Returns a non-nil empty slice when none are found or the project is unknown.
func (r *Registry) SessionsByProject(projectName string) []*Session {
	result := make([]*Session, 0)
	for _, providers := range r.sessionsByProject[projectName] {
		for _, s := range providers {
			result = append(result, s)
		}
	}
	return result
}

// AttachProject associates a project with a session.
// Returns an error when the project is not found, the session is nil or not
// registered in this registry, the session's provider does not support
// projects, or the project is already associated with this session.
func (r *Registry) AttachProject(projectName string, session *Session) error {
	if r.projects[projectName] == nil {
		return fmt.Errorf("attach project: %q not found", projectName)
	}
	if session == nil {
		return fmt.Errorf("attach project: session is nil")
	}
	if r.SessionAt(session.user.email, session.provider.name) != session {
		return fmt.Errorf("attach project: session %q/%q is not registered in this registry",
			session.user.email, session.provider.name)
	}
	if !session.provider.HasProjects {
		return fmt.Errorf("attach project: provider %q does not support projects",
			session.provider.name)
	}
	if r.sessionsByProject[projectName][session.user.email][session.provider.name] != nil {
		return fmt.Errorf("attach project: %q is already attached to session %q/%q",
			projectName, session.user.email, session.provider.name)
	}
	// Update sessionsByProject index.
	if r.sessionsByProject[projectName] == nil {
		r.sessionsByProject[projectName] = make(map[string]map[string]*Session)
	}
	if r.sessionsByProject[projectName][session.user.email] == nil {
		r.sessionsByProject[projectName][session.user.email] = make(map[string]*Session)
	}
	r.sessionsByProject[projectName][session.user.email][session.provider.name] = session
	// Update projectsBySession index.
	if r.projectsBySession[session.user.email] == nil {
		r.projectsBySession[session.user.email] = make(map[string]map[string]*Project)
	}
	if r.projectsBySession[session.user.email][session.provider.name] == nil {
		r.projectsBySession[session.user.email][session.provider.name] = make(map[string]*Project)
	}
	r.projectsBySession[session.user.email][session.provider.name][projectName] = r.projects[projectName]
	return nil
}

// DetachProject removes the association between a project and a specific session.
// Returns an error when the project is not found, the session is nil, or the
// project is not currently associated with that session.
func (r *Registry) DetachProject(projectName string, session *Session) error {
	if r.projects[projectName] == nil {
		return fmt.Errorf("detach project: %q not found", projectName)
	}
	if session == nil {
		return fmt.Errorf("detach project: session is nil")
	}
	if r.sessionsByProject[projectName][session.user.email][session.provider.name] == nil {
		return fmt.Errorf("detach project: %q is not attached to session %q/%q",
			projectName, session.user.email, session.provider.name)
	}
	// Remove from sessionsByProject index.
	delete(r.sessionsByProject[projectName][session.user.email], session.provider.name)
	if len(r.sessionsByProject[projectName][session.user.email]) == 0 {
		delete(r.sessionsByProject[projectName], session.user.email)
	}
	if len(r.sessionsByProject[projectName]) == 0 {
		delete(r.sessionsByProject, projectName)
	}
	// Remove from projectsBySession index.
	delete(r.projectsBySession[session.user.email][session.provider.name], projectName)
	if len(r.projectsBySession[session.user.email][session.provider.name]) == 0 {
		delete(r.projectsBySession[session.user.email], session.provider.name)
	}
	if len(r.projectsBySession[session.user.email]) == 0 {
		delete(r.projectsBySession, session.user.email)
	}
	return nil
}

// RemoveProject removes a project from the registry and cleans up all indexes.
// Any session whose LastQuestion references this project has its project
// pointer cleared to prevent a dangling reference.
func (r *Registry) RemoveProject(name string) error {
	p, exists := r.projects[name]
	if !exists {
		return fmt.Errorf("remove project: %q not found", name)
	}
	// Clear LastQuestion pointers in all sessions that reference this project.
	for _, providers := range r.sessionsByUser {
		for _, s := range providers {
			if s.LastQuestion != nil && s.LastQuestion.project == p {
				s.LastQuestion.project = nil
			}
		}
	}
	// Remove from both project-session indexes.
	for userEmail, providers := range r.sessionsByProject[name] {
		for providerName := range providers {
			delete(r.projectsBySession[userEmail][providerName], name)
			if len(r.projectsBySession[userEmail][providerName]) == 0 {
				delete(r.projectsBySession[userEmail], providerName)
			}
			if len(r.projectsBySession[userEmail]) == 0 {
				delete(r.projectsBySession, userEmail)
			}
		}
	}
	delete(r.sessionsByProject, name)
	delete(r.projects, name)
	return nil
}
