package model

import (
	"fmt"
	"strings"
)

// ── FieldError ────────────────────────────────────────────────────────────────

// FieldError is a validation error tied to a specific field name.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string {
	return e.Field + ": " + e.Message
}

// ── Single-entity validators ──────────────────────────────────────────────────

// ValidateProvider returns all validation errors for a provider.
func ValidateProvider(p *Provider) []error {
	if p == nil {
		return []error{fmt.Errorf("provider is nil")}
	}
	var errs []error
	if strings.TrimSpace(p.name) == "" {
		errs = append(errs, &FieldError{Field: "name", Message: "required"})
	}
	return errs
}

// ValidateUser returns all validation errors for a user.
func ValidateUser(u *User) []error {
	if u == nil {
		return []error{fmt.Errorf("user is nil")}
	}
	var errs []error
	email := strings.TrimSpace(u.email)
	switch {
	case email == "":
		errs = append(errs, &FieldError{Field: "email", Message: "required"})
	case !isValidEmail(email):
		errs = append(errs, &FieldError{Field: "email", Message: "must contain exactly one @ with a non-empty local part and a domain containing ."})
	}
	if strings.TrimSpace(u.Name) == "" {
		errs = append(errs, &FieldError{Field: "name", Message: "required"})
	}
	return errs
}

// ValidateSession returns all validation errors for a session.
func ValidateSession(s *Session) []error {
	if s == nil {
		return []error{fmt.Errorf("session is nil")}
	}
	var errs []error
	if s.user == nil {
		errs = append(errs, &FieldError{Field: "user", Message: "required"})
	}
	if s.provider == nil {
		errs = append(errs, &FieldError{Field: "provider", Message: "required"})
	}
	return errs
}

// ValidateProject returns all validation errors for a project.
func ValidateProject(p *Project) []error {
	if p == nil {
		return []error{fmt.Errorf("project is nil")}
	}
	var errs []error
	if strings.TrimSpace(p.name) == "" {
		errs = append(errs, &FieldError{Field: "name", Message: "required"})
	}
	return errs
}

// ValidateLastQuestion returns all validation errors for a last question.
// All field combinations are currently valid; this function exists as a hook
// for future constraints such as maximum prompt length.
func ValidateLastQuestion(_ *LastQuestion) []error {
	return nil
}

// ── Context-aware validators ──────────────────────────────────────────────────

// ValidateProjectAssignment verifies that the given session supports projects.
// Call this before constructing a NewSessionProject or calling AttachProject
// to surface a user-friendly error before the operation is attempted.
// Passing a nil session is valid — it means the project will be unattached.
func ValidateProjectAssignment(s *Session) []error {
	if s == nil {
		return nil // unattached project — always valid
	}
	var errs []error
	if s.provider == nil {
		errs = append(errs, &FieldError{Field: "provider", Message: "session has no provider"})
		return errs
	}
	if !s.provider.HasProjects {
		errs = append(errs, &FieldError{
			Field:   "provider",
			Message: fmt.Sprintf("%q does not support projects", s.provider.name),
		})
	}
	return errs
}

// ── Display helpers ───────────────────────────────────────────────────────────

// FormatErrors joins a slice of errors into a semicolon-separated string
// suitable for display in a status bar or modal.
// Returns "" when errs is empty or nil.
func FormatErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "; ")
}

// ── Private helpers ───────────────────────────────────────────────────────────

// isValidEmail returns true when s has exactly one "@", a non-empty local part
// before it, and at least one "." in the domain after it.
// Intentionally light-touch for a personal tool; not RFC 5321 compliant.
func isValidEmail(s string) bool {
	parts := strings.SplitN(s, "@", 3)
	if len(parts) != 2 {
		return false
	}
	local, domain := parts[0], parts[1]
	return len(local) > 0 && strings.Contains(domain, ".")
}
