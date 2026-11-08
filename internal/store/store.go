// Package store defines the persistence contract for the registry.
package store

import "github.com/balinomad/prompt-backlog/internal/model"

// Store is the persistence contract for the registry.
// Implementations must be safe to call from a single goroutine only;
// the TUI is single-threaded and no locking is required.
//
// A new implementation (e.g. SQLite) satisfies this interface without any
// changes to the model or TUI packages. Save and Load always operate on
// the whole graph from the caller's side — that is the stable contract.
// A backend with row-level granularity is expected to exploit it
// internally: diff the incoming *Registry against what is already
// durable and write only what changed, inside one transaction. Callers
// never need to know whether an implementation does that or simply
// rewrites everything; both satisfy this interface identically.
type Store interface {
	// Load reads the registry from the underlying storage.
	// Returns a fully initialised *Registry with all pointer references resolved.
	// It returns an empty registry (not an error) when no data has been persisted yet.
	Load() (*model.Registry, error)

	// Save writes the registry to the underlying storage.
	// The call is synchronous; when Save returns without error the data is durable.
	Save(*model.Registry) error
}
