package views

import (
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/rivo/tview"
)

// App is the interface that all views use to interact with the host application.
// The concrete implementation lives in cmd/tui/app.go.
//
// Dependency direction: views define what they need; the host satisfies it.
// This keeps views decoupled from the concrete App struct and testable in
// isolation with a mock implementation.
type App interface {
	// Registry returns the current in-memory registry.
	// Views read from this; mutations must go through Mutate.
	Registry() model.Registry

	// Mutate applies fn to the registry and, if fn returns nil, persists the
	// result to disk. It then refreshes all views regardless of whether the
	// save succeeded.
	// Two distinct errors may be returned:
	//   - fn's error: a domain or validation failure; the registry is unchanged.
	//   - a save error: the mutation was applied in memory but not persisted.
	// Callers display fn errors inline (e.g. inside a form); save errors are
	// the caller's responsibility to surface, typically via SetStatus.
	Mutate(fn func(*model.Registry) error) error

	// ShowModal displays p as a centered overlay above the current screen.
	// Only one modal may be shown at a time; a second call replaces the first.
	ShowModal(p tview.Primitive)

	// HideModal removes the current modal overlay and restores focus to the
	// component that was active before ShowModal was called.
	HideModal()

	// SwitchToProviders switches the display to the providers management screen.
	SwitchToProviders()

	// SwitchToUnattached switches the main display to the unattached projects
	// screen.
	SwitchToUnattached()

	// SwitchToMain switches the display back to the sessions + detail screen.
	SwitchToMain()

	// SetStatus displays msg in the status bar. An empty string clears it.
	// Callers may include tview color tags (e.g. "[red]error[-]").
	SetStatus(msg string)

	// SetHints replaces the keyboard-hint line in the status bar.
	// Views call this in their focus handler to show context-relevant bindings.
	// Use the pre-defined Hints* constants from statusbar.go.
	SetHints(hints string)

	// Quit stops the application. Called by the quit confirmation modal after
	// the operator confirms.
	Quit()
}
