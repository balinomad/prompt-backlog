package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/balinomad/llm-desk/internal/apperr"
)

// Entity-specific — wraps its category, so both checks succeed on the
// same error: errors.Is(err, ErrVendorNotFound) and errors.Is(err, apperr.ErrNotFound).
var (
	ErrVendorNotFound = fmt.Errorf("vendor %w", apperr.ErrNotFound)
	ErrVendorExists   = fmt.Errorf("vendor %w", apperr.ErrExists)
	ErrVendorStale    = fmt.Errorf("vendor %w", apperr.ErrStale)
	ErrVendorNil      = fmt.Errorf("vendor %w", apperr.ErrNil)

	ErrUserNotFound = fmt.Errorf("user %w", apperr.ErrNotFound)
	ErrUserExists   = fmt.Errorf("user %w", apperr.ErrExists)
	ErrUserStale    = fmt.Errorf("user %w", apperr.ErrStale)
	ErrUserNil      = fmt.Errorf("user %w", apperr.ErrNil)

	ErrProjectNotFound     = fmt.Errorf("project %w", apperr.ErrNotFound)
	ErrProjectExists       = fmt.Errorf("project %w", apperr.ErrExists)
	ErrProjectStale        = fmt.Errorf("project %w", apperr.ErrStale)
	ErrProjectNil          = fmt.Errorf("project %w", apperr.ErrNil)
	ErrProjectsUnsupported = fmt.Errorf("projects %w", apperr.ErrUnsupported)

	ErrAccountNotFound = fmt.Errorf("account %w", apperr.ErrNotFound)
	ErrAccountExists   = fmt.Errorf("account %w", apperr.ErrExists)
	ErrAccountStale    = fmt.Errorf("account %w", apperr.ErrStale)
	ErrAccountNil      = fmt.Errorf("account %w", apperr.ErrNil)

	ErrSessionNotFound = fmt.Errorf("session %w", apperr.ErrNotFound)
	ErrSessionStale    = fmt.Errorf("session %w", apperr.ErrStale)
	ErrSessionNil      = fmt.Errorf("session %w", apperr.ErrNil)

	ErrEmailRequired = fmt.Errorf("email %w", apperr.ErrRequired)
	ErrKeyRequired   = fmt.Errorf("key %w", apperr.ErrRequired)
	ErrNameRequired  = fmt.Errorf("name %w", apperr.ErrRequired)
	ErrTextRequired  = fmt.Errorf("text %w", apperr.ErrRequired)
)

// ValidateVendor returns an error when the vendor is invalid.
func ValidateVendor(p *Vendor) error {
	switch {
	case p == nil:
		return ErrVendorNil
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
		errs = append(errs, apperr.ErrEmailInvalid)
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
		if acc.key.Vendor == "" {
			errs = append(errs, fmt.Errorf("vendor %w", ErrNameRequired))
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
