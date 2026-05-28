package views

import (
	"github.com/rivo/tview"
)

// ── Context hint strings ──────────────────────────────────────────────────────
// Pre-defined hint lines for each focus context. Views pass these to
// app.SetHints in their focus handlers. Key names are bold via tview color tags.

const (
	// HintsSessionsPanel is shown when the sessions list has focus.
	HintsSessionsPanel = "[::b]n[-::-] new  [::b]e[-::-] edit  [::b]d[-::-] del" +
		"  [::b]p[-::-] providers  [::b]s[-::-] unattached  [::b]Tab[-::-] focus  [::b]?[-::-] help  [::b]q[-::-] quit"

	// HintsProjectsPanel is shown when the projects list has focus.
	HintsProjectsPanel = "[::b]n[-::-] new  [::b]e[-::-] edit  [::b]d[-::-] del" +
		"  [::b]Space[-::-] toggle  [::b]Tab[-::-] focus  [::b]?[-::-] help  [::b]q[-::-] quit"

	// HintsProvidersPanel is shown when the providers screen has focus.
	HintsProvidersPanel = "[::b]n[-::-] new  [::b]e[-::-] edit  [::b]d[-::-] del" +
		"  [::b]Tab[-::-] focus  [::b]?[-::-] help  [::b]q[-::-] quit"

	// HintsUnattachedPanel is shown when the unattached projects screen has focus.
	HintsUnattachedPanel = "[::b]n[-::-] new  [::b]e[-::-] edit  [::b]d[-::-] del" +
		"  [::b]Tab[-::-] focus  [::b]?[-::-] help  [::b]q[-::-] quit"

	// HintsForm is shown when a form modal is open.
	HintsForm = "[::b]Ctrl+S[-::-] save  [::b]Esc[-::-] cancel"

	// HintsConfirmModal is shown when a confirmation dialog is open.
	HintsConfirmModal = "[::b]Enter[-::-] confirm  [::b]Esc[-::-] cancel"

	// HintsHelpOverlay is shown when the help overlay is open.
	HintsHelpOverlay = "[::b]Esc[-::-] close"
)

// ── StatusBar ─────────────────────────────────────────────────────────────────

// StatusBar is the two-section footer bar at the bottom of the screen.
// The left section shows context-sensitive keyboard hints; the right section
// shows a temporary status message (informational or error).
// Both sections are updated in-place — no redraw is triggered from here;
// tview's draw loop picks up the changes on the next cycle.
type StatusBar struct {
	*tview.Flex
	hintsView   *tview.TextView
	messageView *tview.TextView
}

// NewStatusBar constructs an empty StatusBar. The hint line defaults to the
// sessions panel hints; the message section is empty.
func NewStatusBar() *StatusBar {
	hints := tview.NewTextView().
		SetDynamicColors(true).
		SetText(HintsSessionsPanel)

	message := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)

	flex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(hints, 0, 1, false).
		AddItem(message, 0, 1, false)

	return &StatusBar{
		Flex:        flex,
		hintsView:   hints,
		messageView: message,
	}
}

// SetHints replaces the keyboard-hint line. Use the pre-defined Hints*
// constants to keep display text consistent across views.
func (s *StatusBar) SetHints(hints string) {
	s.hintsView.SetText(hints)
}

// SetMessage sets the right-side status message.
// Pass an empty string to clear it.
// Callers may include tview color tags (e.g. "[red]save failed[-]").
func (s *StatusBar) SetMessage(msg string) {
	s.messageView.SetText(msg)
}
