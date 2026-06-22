package model

import (
	"errors"
	"fmt"
	"strings"
)

// Shared categories — for generic dispatch (e.g. mapping any registry
// failure to an HTTP status) regardless of entity type.
var (
	ErrNotFound     = errors.New("not found")
	ErrExists       = errors.New("already exists")
	ErrStale        = errors.New("is stale")
	ErrNil          = errors.New("is nil")
	ErrRequired     = errors.New("required")
	ErrEmailInvalid = errors.New("invalid email format")
	ErrUnsupported  = errors.New("not supported")
)

// Entity-specific — wraps its category, so both checks succeed on the
// same error: errors.Is(err, ErrProviderNotFound) and errors.Is(err, ErrNotFound).
var (
	ErrProviderNotFound = fmt.Errorf("provider %w", ErrNotFound)
	ErrProviderExists   = fmt.Errorf("provider %w", ErrExists)
	ErrProviderStale    = fmt.Errorf("provider %w", ErrStale)
	ErrProviderNil      = fmt.Errorf("provider %w", ErrNil)

	ErrUserNotFound = fmt.Errorf("user %w", ErrNotFound)
	ErrUserExists   = fmt.Errorf("user %w", ErrExists)
	ErrUserStale    = fmt.Errorf("user %w", ErrStale)
	ErrUserNil      = fmt.Errorf("user %w", ErrNil)

	ErrProjectNotFound     = fmt.Errorf("project %w", ErrNotFound)
	ErrProjectExists       = fmt.Errorf("project %w", ErrExists)
	ErrProjectStale        = fmt.Errorf("project %w", ErrStale)
	ErrProjectNil          = fmt.Errorf("project %w", ErrNil)
	ErrProjectsUnsupported = fmt.Errorf("projects %w", ErrUnsupported)

	ErrAccountNotFound = fmt.Errorf("account %w", ErrNotFound)
	ErrAccountExists   = fmt.Errorf("account %w", ErrExists)
	ErrAccountStale    = fmt.Errorf("account %w", ErrStale)
	ErrAccountNil      = fmt.Errorf("account %w", ErrNil)

	ErrSessionNotFound = fmt.Errorf("session %w", ErrNotFound)
	ErrSessionStale    = fmt.Errorf("session %w", ErrStale)
	ErrSessionNil      = fmt.Errorf("session %w", ErrNil)

	ErrEmailRequired = fmt.Errorf("email %w", ErrRequired)
	ErrKeyRequired   = fmt.Errorf("key %w", ErrRequired)
	ErrNameRequired  = fmt.Errorf("name %w", ErrRequired)
	ErrTextRequired  = fmt.Errorf("text %w", ErrRequired)
)

// ValidateProvider returns an error when the provider is invalid.
func ValidateProvider(p *Provider) error {
	switch {
	case p == nil:
		return ErrProviderNil
	case p.name == "":
		return ErrNameRequired
	default:
		return nil
	}
}

// ValidateUser returns an error when the user is invalid.
func ValidateUser(u *User) error {
	if u == nil {
		return ErrUserNil
	}
	var errs []error
	if u.email == "" {
		errs = append(errs, ErrEmailRequired)
	} else if !isValidEmail(u.email) {
		errs = append(errs, ErrEmailInvalid)
	}
	if u.name == "" {
		errs = append(errs, ErrNameRequired)
	}
	return errJoin(errs)
}

// ValidateProject returns an error when the project is invalid.
func ValidateProject(p *Project) error {
	switch {
	case p == nil:
		return ErrProjectNil
	case p.name == "":
		return ErrNameRequired
	default:
		return nil
	}
}

// ValidateAccount returns an error when the account is invalid.
func ValidateAccount(acc *Account) error {
	if acc == nil {
		return ErrAccountNil
	}
	var errs []error
	if acc.key == (AccountKey{}) {
		errs = append(errs, ErrKeyRequired)
	} else {
		if acc.key.Email == "" {
			errs = append(errs, fmt.Errorf("user %w", ErrEmailRequired))
		}
		if acc.key.Provider == "" {
			errs = append(errs, fmt.Errorf("provider %w", ErrNameRequired))
		}
	}
	return errJoin(errs)
}

// ValidatePrompt returns an error when the prompt is invalid.
func ValidatePrompt(p Prompt) error {
	if p.text == "" {
		return ErrTextRequired
	}
	return nil
}

// isValidEmail returns true when s has exactly one "@", a non-empty local part
// before it, and at least one "." in the domain after it.
// Intentionally light-touch for a personal tool; not RFC 5321 compliant.
func isValidEmail(s string) bool {
	local, domain, found := strings.Cut(s, "@")
	if !found || local == "" || domain == "" {
		return false
	}
	if strings.Contains(domain, "@") {
		return false
	}
	return strings.Contains(domain, ".")
}

// errJoin joins a slice of errors into a single error.
func errJoin(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return errors.Join(errs...)
	}
}
