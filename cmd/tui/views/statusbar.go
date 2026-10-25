package views

import (
	"github.com/rivo/tview"
)

// ── Context hint strings ──────────────────────────────────────────────────────
// Pre-defined hint lines for each focus context, split in two: Specific is
// the upper row of item actions for the focused list or modal; Common is the
// lower row of navigation, escape, help, and quit hints. Views pass both to
// app.SetHints in their focus handlers. Key names are bold via tview color
// tags.

var (
	// Canonical hint fragments. Reuse these so repeated actions keep the
	// same wording everywhere.
	hintEdit      = hint{"Enter", "edit"}
	hintAdd       = hint{"Ins", "add"}
	hintDelete    = hint{"Del", "delete"}
	hintQueue     = hint{"u", "queue"}
	hintContinue  = hint{"Space", "continue"}
	hintMoveUp    = hint{"Ctrl+↑", "up"}
	hintMoveDown  = hint{"Ctrl+↓", "down"}
	hintHelp      = hint{"?", "help"}
	hintQuit      = hint{"q", "quit"}
	hintBack      = hint{"Esc", "back"}
	hintClose     = hint{"Esc", "close"}
	hintCancel    = hint{"Esc", "cancel"}
	hintConfirm   = hint{"Enter", "confirm"}
	hintProviders = hint{"r", "providers"}
	hintProjects  = hint{"p", "projects"}

	// HintsAccountsPanelSpecific is the context-specific hint line shown when
	// the accounts list has focus.
	HintsAccountsPanelSpecific = hints{
		hintEdit, hintAdd, hintDelete,
	}.String()

	// HintsAccountsPanelCommon is the common hint line shown when the
	// accounts list has focus.
	HintsAccountsPanelCommon = hints{
		{"Tab", "sessions"}, hintProviders, hintProjects, hintHelp, hintQuit,
	}.String()

	// HintsDetailPanelSpecific is the context-specific hint line shown when
	// the sessions sub-list in the detail panel has focus.
	HintsDetailPanelSpecific = hints{
		hintEdit, hintAdd, hintDelete, hintQueue, hintContinue,
	}.String()

	// HintsDetailPanelCommon is the common hint line shown when the sessions
	// sub-list in the detail panel has focus.
	HintsDetailPanelCommon = hints{
		{"Tab", "accounts"}, hintHelp, hintQuit,
	}.String()

	// HintsProvidersPanelSpecific is the context-specific hint line shown when
	// the providers screen has focus.
	HintsProvidersPanelSpecific = hints{
		hintEdit, hintAdd, hintDelete,
	}.String()

	// HintsProvidersPanelCommon is the common hint line shown when the
	// providers screen has focus.
	HintsProvidersPanelCommon = hints{
		hintBack, hintHelp, hintQuit,
	}.String()

	// HintsProjectsPanelSpecific is the context-specific hint line shown when
	// the projects list has focus.
	HintsProjectsPanelSpecific = hints{
		hintEdit, hintAdd, hintDelete,
	}.String()

	// HintsProjectsPanelCommon is the common hint line shown when the
	// projects list has focus.
	HintsProjectsPanelCommon = hints{
		{"Tab", "accounts"}, hintBack, hintHelp, hintQuit,
	}.String()

	// HintsProjectAccountsSpecific is the context-specific hint line shown
	// when the granted-accounts list has focus.
	HintsProjectAccountsSpecific = hints{
		hintAdd, hintDelete,
	}.String()

	// HintsProjectAccountsCommon is the common hint line shown when the
	// granted-accounts list has focus.
	HintsProjectAccountsCommon = hints{
		{"Tab", "projects"}, hintBack, hintHelp, hintQuit,
	}.String()

	// HintsFormSpecific is the context-specific hint line shown when a form
	// modal is open. It is empty because the Save button is the submit path.
	HintsFormSpecific = hints{}.String()

	// HintsFormCommon is the common hint line shown when a form modal is
	// open.
	HintsFormCommon = hints{
		{"Tab", "next"}, hintCancel,
	}.String()

	// HintsQueueEditorSpecific is the context-specific hint line shown when
	// the queue editor modal is open.
	HintsQueueEditorSpecific = hints{
		hintEdit, hintAdd, hintDelete, hintMoveUp, hintMoveDown,
	}.String()

	// HintsQueueEditorCommon is the common hint line shown when the queue
	// editor modal is open.
	HintsQueueEditorCommon = hints{
		hintClose, hintHelp, hintQuit,
	}.String()

	// HintsConfirmModalSpecific is the context-specific hint line shown when a
	// confirmation dialog is open.
	HintsConfirmModalSpecific = hints{
		hintConfirm,
	}.String()

	// HintsConfirmModalCommon is the common hint line shown when a
	// confirmation dialog is open.
	HintsConfirmModalCommon = hints{
		hintCancel,
	}.String()

	// HintsHelpOverlaySpecific is the context-specific hint line shown when
	// the help overlay is open. It is empty because the overlay has no
	// context-specific action.
	HintsHelpOverlaySpecific = hints{}.String()

	// HintsHelpOverlayCommon is the common hint line shown when the help
	// overlay is open.
	HintsHelpOverlayCommon = hints{
		hintClose,
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
