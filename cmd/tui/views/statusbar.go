package views

import (
	"github.com/rivo/tview"
)

// ── Context hint strings ──────────────────────────────────────────────────────
// Pre-defined hint lines for each focus context. Views pass these to
// app.SetHints in their focus handlers. Key names are bold via tview color tags.

var (
	// HintsAccountsPanel is shown when the accounts list has focus.
	HintsAccountsPanel = hints{
		{string(KeyNew), "new"},
		{string(KeyEdit), "edit"},
		{string(KeyDelete), "del"},
		{string(KeyRenameUser), "rename"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{string(KeyProviders), "providers"},
		{string(KeyProjects), "projects"},
		{"Tab", "focus"},
	}.String()

	// HintsDetailPanel is shown when the sessions sub-list in the detail panel
	// has focus.
	HintsDetailPanel = hints{
		{string(KeyNew), "new"},
		{string(KeyEdit), "edit"},
		{string(KeyDelete), "end"},
		{string(KeyQueue), "queue"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{"Space", "continue"},
		{"Tab", "focus"},
	}.String()

	// HintsProvidersPanel is shown when the providers screen has focus.
	HintsProvidersPanel = hints{
		{string(KeyNew), "new"},
		{string(KeyEdit), "edit"},
		{string(KeyDelete), "del"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{"Tab", "focus"},
	}.String()

	// HintsProjectsPanel is shown when the projects screen has focus.
	HintsProjectsPanel = hints{
		{string(KeyNew), "new"},
		{string(KeyEdit), "edit"},
		{string(KeyDelete), "del"},
		{string(KeyGrant), "grant"},
		{string(KeyRevoke), "revoke"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{"Tab", "focus"},
	}.String()

	// HintsForm is shown when a form modal is open.
	HintsForm = hints{
		{"Ctrl+S", "save"},
		{"Esc", "cancel"},
	}.String()

	// HintsQueueEditor is shown when the queue editor modal is open.
	HintsQueueEditor = hints{
		{string(KeyNew), "add"},
		{string(KeyEdit), "edit"},
		{string(KeyDelete), "del"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{string(KeyMoveUp), "up"},
		{string(KeyMoveDown), "down"},
		{"Esc", "close"},
	}.String()

	// HintsConfirmModal is shown when a confirmation dialog is open.
	HintsConfirmModal = hints{
		{"Enter", "confirm"},
		{"Esc", "cancel"},
	}.String()

	// HintsHelpOverlay is shown when the help overlay is open.
	HintsHelpOverlay = hints{
		{"Esc", "close"},
	}.String()
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
// accounts panel hints; the message section is empty.
func NewStatusBar() *StatusBar {
	hints := tview.NewTextView().
		SetDynamicColors(true).
		SetText(HintsAccountsPanel)

	message := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)

	flex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(hints, 0, 3, false).
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
