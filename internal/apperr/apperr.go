// Package apperr defines the generic error categories shared across every
// layer of prompt-backlog. These are deliberately type-free: they carry no
// reference to model's domain types, so any layer — including ones that
// must not depend on internal/model directly — can classify an error by
// category with errors.Is, without importing the domain types that
// produced it.
//
// internal/model wraps these into entity-specific sentinels (e.g.
// model.ErrVendorNotFound wraps ErrNotFound); callers that need to know
// which entity failed match on the model sentinel; callers that only need
// the failure category match on these directly.
package apperr

import "errors"

var (
	// ErrNotFound indicates the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrExists indicates an entity with the same natural key already exists.
	ErrExists = errors.New("already exists")
	// ErrStale indicates the caller's copy of an entity no longer matches
	// the live registry entry — it was replaced by a concurrent update.
	ErrStale = errors.New("is stale")
	// ErrNil indicates a required entity argument was nil.
	ErrNil = errors.New("is nil")
	// ErrRequired indicates a required field was empty.
	ErrRequired = errors.New("required")
	// ErrEmailInvalid indicates a malformed email address.
	ErrEmailInvalid = errors.New("invalid email format")
	// ErrUnsupported indicates the operation is not supported in this context.
	ErrUnsupported = errors.New("not supported")
)
