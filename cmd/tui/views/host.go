// Package views contains all views for the tui.
package views

import (
	"errors"

	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/rivo/tview"
)

// ErrSaveFailed wraps a persistence error returned by Mutate when fn's
// mutation succeeded in memory but the subsequent save to disk failed.
// Callers can distinguish this from a domain/validation error (fn's own
// error, registry unchanged) via errors.Is(err, ErrSaveFailed).
var ErrSaveFailed = errors.New("save failed")

// Host is the interface that all views use to interact with the host application.
// The concrete implementation lives in cmd/tui/app.go.
//
// Dependency direction: views define what they need; the host satisfies it.
// This keeps views decoupled from the concrete host application and testable in
// isolation with a mock implementation.
type Host interface {
	// Service returns the application service layer.
	// Views read from this; mutations must go through Mutate.
	Service() *service.Service

	// AppName returns the application's display name, for use in dialog
	// and status text.
	AppName() string

	// Mutate applies fn against the service and, if fn returns nil, persists
	// the result to disk. It then refreshes all views regardless of whether
	// the save succeeded.
	// Three distinct errors may be returned:
	//   - fn is nil: a caller bug; no mutation is attempted.
	//   - fn's error: a domain or validation failure; the registry is unchanged.
	//   - a save error, wrapping ErrSaveFailed: the mutation was applied in
	//     memory but not persisted. errors.Is(err, ErrSaveFailed) tells the
	//     two apart.
	// Callers display fn errors inline (e.g. inside a form); save errors are
	// the caller's responsibility to surface, typically via SetStatus.
	Mutate(fn func(*service.Service) error) error

	// ShowModal pushes p as a centered overlay above the current modal, if
	// any. The parent modal remains visible but inactive.
	// Returns an error if p is nil.
	ShowModal(p tview.Primitive) error

	// HideModal pops the top modal overlay and restores the parent modal, or
	// the component that was active before the first modal was pushed.
	HideModal()

	// ReplaceTopModal swaps the top modal primitive without changing the
	// stack depth. Any child modal must already have been popped.
	ReplaceTopModal(p tview.Primitive) error

	// SetFocus moves keyboard focus to p.
	SetFocus(p tview.Primitive)

	// SwitchToVendors switches the display to the vendors management screen.
	SwitchToVendors()

	// SwitchToProjects switches the display to the projects management screen.
	SwitchToProjects()

	// SwitchToUsers switches the display to the users management screen.
	SwitchToUsers()

	// SwitchToMain switches the display back to the accounts + detail screen.
	SwitchToMain()

	// SetStatus displays an inline status message in the status bar. It clears
	// any notice or error. A non-empty message clears automatically; an empty
	// string clears it immediately.
	// Callers may include tview colour tags (e.g. "[green]saved[-]").
	SetStatus(msg string)

	// SetNotice displays msg in the separate notice area below the common
	// hints. It clears any inline status and error. A non-empty message clears
	// automatically; an empty string clears it immediately.
	SetNotice(msg string)

	// SetError displays msg in the separate red error area below the common
	// hints. It clears any inline status and notice. Errors remain visible
	// until another message replaces them or an empty string clears them.
	SetError(msg string)

	// Fatal records err and stops the event loop. It is reserved for
	// programmer errors, typically invariant violations. It returns
	// immediately; callers must return without further UI work. Run returns
	// err after the terminal is restored.
	Fatal(err error)

	// SetHints replaces the keyboard-hint lines in the status bar. Use the
	// pre-defined Hints*Specific/Hints*Common pairs from chrome.go.
	SetHints(specific, common string)

	// Quit stops the application. Called by the quit confirmation modal after
	// the operator confirms.
	Quit()

	// CopyToClipboard writes text to the system clipboard.
	CopyToClipboard(text string) error

	// ReadFromClipboard returns the current text from the system clipboard.
	// It returns an empty string when the clipboard holds no text.
	ReadFromClipboard() (string, error)
}
