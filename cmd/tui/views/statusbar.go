package views

import (
	"github.com/rivo/tview"
)

// ── Context hint strings ──────────────────────────────────────────────────────
// Pre-defined hint lines for each focus context, split in two: Specific is
// the upper, context-specific line; Common is the lower line for roles that
// repeat across contexts with the same general meaning regardless of the
// exact key (edit, delete, help, quit, focus, cancel/close). Views pass both
// to app.SetHints in their focus handlers. Key names are bold via tview color
// tags.

var (
	// HintsAccountsPanelSpecific is the context-specific hint line shown when
	// the accounts list has focus.
	HintsAccountsPanelSpecific = hints{
		{string(KeyNew), "new"},
		{"F2", "rename"},
		{string(KeyProviders), "providers"},
		{string(KeyProjects), "projects"},
	}.String()

	// HintsAccountsPanelCommon is the common hint line shown when the
	// accounts list has focus.
	HintsAccountsPanelCommon = hints{
		{"Enter", "edit"},
		{"Del", "del"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{"Tab", "focus"},
	}.String()

	// HintsDetailPanelSpecific is the context-specific hint line shown when
	// the sessions sub-list in the detail panel has focus.
	HintsDetailPanelSpecific = hints{
		{string(KeyNew), "new"},
		{string(KeyQueue), "queue"},
		{"Space", "continue"},
	}.String()

	// HintsDetailPanelCommon is the common hint line shown when the sessions
	// sub-list in the detail panel has focus.
	HintsDetailPanelCommon = hints{
		{string(KeySessionFields), "edit"},
		{"Del", "end"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{"Tab", "focus"},
	}.String()

	// HintsProvidersPanelSpecific is the context-specific hint line shown when
	// the providers screen has focus.
	HintsProvidersPanelSpecific = hints{
		{string(KeyNew), "new"},
	}.String()

	// HintsProvidersPanelCommon is the common hint line shown when the
	// providers screen has focus.
	HintsProvidersPanelCommon = hints{
		{"Enter", "edit"},
		{"Del", "del"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{"Tab", "focus"},
	}.String()

	// HintsProjectsPanelSpecific is the context-specific hint line shown when
	// the projects screen has focus.
	HintsProjectsPanelSpecific = hints{
		{string(KeyNew), "new"},
		{string(KeyGrant), "grant"},
		{string(KeyRevoke), "revoke"},
	}.String()

	// HintsProjectsPanelCommon is the common hint line shown when the
	// projects screen has focus.
	HintsProjectsPanelCommon = hints{
		{"Enter", "edit"},
		{"Del", "del"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{"Tab", "focus"},
	}.String()

	// HintsFormSpecific is the context-specific hint line shown when a form
	// modal is open.
	HintsFormSpecific = hints{
		{"Ctrl+S", "save"},
	}.String()

	// HintsFormCommon is the common hint line shown when a form modal is
	// open.
	HintsFormCommon = hints{
		{"Esc", "cancel"},
	}.String()

	// HintsQueueEditorSpecific is the context-specific hint line shown when
	// the queue editor modal is open.
	HintsQueueEditorSpecific = hints{
		{string(KeyNew), "add"},
		{string(KeyMoveUp), "up"},
		{string(KeyMoveDown), "down"},
	}.String()

	// HintsQueueEditorCommon is the common hint line shown when the queue
	// editor modal is open.
	HintsQueueEditorCommon = hints{
		{"Enter", "edit"},
		{"Del", "del"},
		{string(KeyHelp), "help"},
		{string(KeyQuit), "quit"},
		{"Esc", "close"},
	}.String()

	// HintsConfirmModalSpecific is the context-specific hint line shown when a
	// confirmation dialog is open. It is empty: a confirm dialog has no
	// context-specific action beyond confirm/cancel, both common roles.
	HintsConfirmModalSpecific = hints{}.String()

	// HintsConfirmModalCommon is the common hint line shown when a
	// confirmation dialog is open.
	HintsConfirmModalCommon = hints{
		{"Enter", "confirm"},
		{"Esc", "cancel"},
	}.String()

	// HintsHelpOverlaySpecific is the context-specific hint line shown when
	// the help overlay is open. It is empty, for the same reason as
	// HintsConfirmModalSpecific.
	HintsHelpOverlaySpecific = hints{}.String()

	// HintsHelpOverlayCommon is the common hint line shown when the help
	// overlay is open.
	HintsHelpOverlayCommon = hints{
		{"Esc", "close"},
	}.String()
)

// ── StatusBar ─────────────────────────────────────────────────────────────────

// StatusBar is the two-row footer at the bottom of the screen. The upper row
// shows the context-specific hint line; the lower row splits between the
// common hint line (left) and a temporary status message (right,
// informational or error). All three sections are updated in-place — no
// redraw is triggered from here; tview's draw loop picks up the changes on
// the next cycle.
type StatusBar struct {
	*tview.Flex
	specificHintsView *tview.TextView
	commonHintsView   *tview.TextView
	messageView       *tview.TextView
}

// NewStatusBar constructs an empty StatusBar. The hint lines default to the
// accounts panel hints; the message section is empty.
func NewStatusBar() *StatusBar {
	specificHints := tview.NewTextView().
		SetDynamicColors(true).
		SetText(HintsAccountsPanelSpecific)

	commonHints := tview.NewTextView().
		SetDynamicColors(true).
		SetText(HintsAccountsPanelCommon)

	message := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)

	lowerRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(commonHints, 0, 3, false).
		AddItem(message, 0, 1, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(specificHints, 1, 0, false).
		AddItem(lowerRow, 1, 0, false)

	return &StatusBar{
		Flex:              flex,
		specificHintsView: specificHints,
		commonHintsView:   commonHints,
		messageView:       message,
	}
}

// SetHints replaces both keyboard-hint lines. Use the pre-defined
// Hints*Specific / Hints*Common constant pairs to keep display text
// consistent across views.
func (s *StatusBar) SetHints(specific, common string) {
	s.specificHintsView.SetText(specific)
	s.commonHintsView.SetText(common)
}

// SetMessage sets the right-side status message.
// Pass an empty string to clear it.
// Callers may include tview colour tags (e.g. "[red]save failed[-]").
func (s *StatusBar) SetMessage(msg string) {
	s.messageView.SetText(msg)
}
