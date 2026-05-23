package store

import "github.com/balinomad/llm-desk/internal/model"

// Store is the persistence contract for the registry.
// Implementations must be safe to call from a single goroutine only;
// the TUI is single-threaded and no locking is required.
//
// A new implementation (e.g. SQLite) satisfies this interface without any
// changes to the model or TUI packages.
type Store interface {
	// Load reads the registry from the underlying storage.
	// Returns a fully initialised Registry with all pointer references resolved.
	// Returns an empty Registry (not an error) when no data has been persisted yet.
	Load() (model.Registry, error)

	// Save writes the registry to the underlying storage.
	// The call is synchronous; when Save returns without error the data is durable.
	Save(reg model.Registry) error
}
