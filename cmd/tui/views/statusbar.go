package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── Context hint strings ──────────────────────────────────────────────────────
// Pre-defined hint lines for each focus context: Specific is the upper
// in-screen action and focus row; Common is the lower global navigation,
// help, and quit row.

var (
	// Canonical hint fragments. Reuse these so repeated actions keep the
	// same wording everywhere.
	hintEdit     = hint{"Enter", "edit"}
	hintPause    = hint{"Enter", "pause"}
	hintAdd      = hint{"Ins", "add"}
	hintDelete   = hint{"Del", "delete"}
	hintQueue    = hint{string(KeyQueue), "messages"}
	hintContinue = hint{"Space", "continue"}
	hintMoveUp   = hint{"Ctrl+↑", "up"}
	hintMoveDown = hint{"Ctrl+↓", "down"}
	hintHelp     = hint{string(KeyHelp), "help"}
	hintQuit     = hint{string(KeyQuit), "quit"}
	hintAccounts = hint{"Esc", "accounts"}
	hintClose    = hint{"Esc", "close"}
	hintCancel   = hint{"Esc", "cancel"}
	hintConfirm  = hint{"Enter", "confirm"}
	hintVendors  = hint{string(KeyVendors), "vendors"}
	hintProjects = hint{string(KeyProjects), "projects"}
	hintUsers    = hint{string(KeyUsers), "users"}

	// HintsAccountsPanelSpecific is the context-specific hint line shown when
	// the accounts list has focus.
	HintsAccountsPanelSpecific = hints{
		{"Tab", "sessions"}, hintPause, hintAdd, hintDelete,
	}.String()

	// HintsAccountsPanelCommon is the common hint line shown when the
	// accounts list has focus.
	HintsAccountsPanelCommon = hints{
		hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsDetailPanelSpecific is the context-specific hint line shown when
	// the sessions sub-list in the detail panel has focus.
	HintsDetailPanelSpecific = hints{
		{"Tab", "accounts"}, hintEdit, hintAdd, hintDelete, hintQueue, hintContinue,
	}.String()

	// HintsDetailPanelCommon is the common hint line shown when the sessions
	// sub-list in the detail panel has focus.
	HintsDetailPanelCommon = hints{
		hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsVendorsPanelSpecific is the context-specific hint line shown when
	// the vendors screen has focus.
	HintsVendorsPanelSpecific = hints{
		hintEdit, hintAdd, hintDelete,
	}.String()

	// HintsVendorsPanelCommon is the common hint line shown when the
	// vendors screen has focus.
	HintsVendorsPanelCommon = hints{
		hintAccounts, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsUsersPanelSpecific is the context-specific hint line shown when
	// the users screen has focus.
	HintsUsersPanelSpecific = hints{
		hintEdit, hintAdd, hintDelete,
	}.String()

	// HintsUsersPanelCommon is the common hint line shown when the users
	// screen has focus.
	HintsUsersPanelCommon = hints{
		hintAccounts, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsProjectsPanelSpecific is the context-specific hint line shown when
	// the projects list has focus.
	HintsProjectsPanelSpecific = hints{
		{"Tab", "project accounts"}, hintEdit, hintAdd, hintDelete,
	}.String()

	// HintsProjectsPanelCommon is the common hint line shown when the
	// projects list has focus.
	HintsProjectsPanelCommon = hints{
		hintAccounts, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsProjectAccountsSpecific is the context-specific hint line shown
	// when the granted-accounts list has focus.
	HintsProjectAccountsSpecific = hints{
		{"Tab", "projects"}, hintAdd, hintDelete,
	}.String()

	// HintsProjectAccountsCommon is the common hint line shown when the
	// granted-accounts list has focus.
	HintsProjectAccountsCommon = hints{
		hintAccounts, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
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

// MinimumWidth returns the smallest terminal width that fits the longest
// specific hint, or the longest common hint plus the inline status separator
// and maxInlineMessageWidth.
func MinimumWidth(maxInlineMessageWidth int) int {
	specific := []string{
		HintsAccountsPanelSpecific,
		HintsDetailPanelSpecific,
		HintsVendorsPanelSpecific,
		HintsUsersPanelSpecific,
		HintsProjectsPanelSpecific,
		HintsProjectAccountsSpecific,
		HintsFormSpecific,
		HintsQueueEditorSpecific,
		HintsConfirmModalSpecific,
		HintsHelpOverlaySpecific,
	}
	common := []string{
		HintsAccountsPanelCommon,
		HintsDetailPanelCommon,
		HintsVendorsPanelCommon,
		HintsUsersPanelCommon,
		HintsProjectsPanelCommon,
		HintsProjectAccountsCommon,
		HintsFormCommon,
		HintsQueueEditorCommon,
		HintsConfirmModalCommon,
		HintsHelpOverlayCommon,
	}
	return max(
		longestHintWidth(specific),
		longestHintWidth(common)+StatusMessageSeparatorWidth+maxInlineMessageWidth,
	)
}

func longestHintWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, tview.TaggedStringWidth(line))
	}
	return width
}

// ── StatusBar ─────────────────────────────────────────────────────────────────

const (
	// StatusBarBaseHeight is the height without a notice or error.
	StatusBarBaseHeight = 2
	// StatusMessageSeparatorWidth separates common hints from an inline status.
	StatusMessageSeparatorWidth = 2
	// maxStatusMessageRows caps the wrapped notice or error area.
	maxStatusMessageRows = 3
)

// StatusBar is the footer at the bottom of the screen. It has a specific hint
// row, a common hint row with an optional inline status message, and a
// separate wrapped notice or error area below them.
type StatusBar struct {
	*tview.Flex
	specificHintsView *tview.TextView
	commonHintsView   *tview.TextView
	messageView       *tview.TextView
	messageSpacer     *tview.Box
	messageAreaView   *tview.TextView
	lowerRow          *tview.Flex

	width           int
	messageText     string
	messageAreaText string
}

// NewStatusBar constructs an empty StatusBar. The hint lines default to the
// accounts panel hints; all message areas are empty.
func NewStatusBar() *StatusBar {
	specificHints := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetText(HintsAccountsPanelSpecific)

	commonHints := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetText(HintsAccountsPanelCommon)

	message := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetTextAlign(tview.AlignLeft)

	spacer := tview.NewBox()
	filler := tview.NewBox()

	lowerRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(commonHints, 0, 0, false).
		AddItem(spacer, 0, 0, false).
		AddItem(filler, 0, 1, false).
		AddItem(message, 0, 0, false)

	messageArea := tview.NewTextView().
		SetWrap(true).
		SetWordWrap(true).
		SetTextAlign(tview.AlignLeft)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(specificHints, 1, 0, false).
		AddItem(lowerRow, 1, 0, false).
		AddItem(messageArea, 0, 0, false)

	return &StatusBar{
		Flex:              flex,
		specificHintsView: specificHints,
		commonHintsView:   commonHints,
		messageView:       message,
		messageSpacer:     spacer,
		messageAreaView:   messageArea,
		lowerRow:          lowerRow,
	}
}

// SetHints replaces both keyboard-hint lines. Use the pre-defined
// Hints*Specific / Hints*Common constant pairs to keep display text
// consistent across views.
func (s *StatusBar) SetHints(specific, common string) {
	s.specificHintsView.SetText(specific)
	s.commonHintsView.SetText(common)
	s.updateLayout()
}

// SetStatus sets the inline status message and clears any notice or error.
// Callers may include tview colour tags (e.g. "[green]saved[-]").
func (s *StatusBar) SetStatus(msg string) {
	s.messageText = msg
	s.messageView.SetText(msg)
	s.clearMessageArea()
	s.updateLayout()
}

// SetNotice sets the separate notice area and clears the inline status and
// any error. The notice uses the active-pane yellow.
func (s *StatusBar) SetNotice(msg string) {
	s.messageText = ""
	s.messageView.SetText("")
	s.messageAreaText = msg
	s.messageAreaView.SetTextColor(accentColor)
	s.messageAreaView.SetText(msg)
	s.updateLayout()
}

// SetError sets the separate error area in red and clears the inline status
// and any notice.
func (s *StatusBar) SetError(msg string) {
	s.messageText = ""
	s.messageView.SetText("")
	s.messageAreaText = msg
	s.messageAreaView.SetTextColor(tcell.ColorRed)
	s.messageAreaView.SetText(msg)
	s.updateLayout()
}

// Layout updates the message row and message area for width, and returns the
// total status-bar height.
func (s *StatusBar) Layout(width int) int {
	s.width = width
	s.updateLayout()
	return s.requiredHeight()
}

func (s *StatusBar) clearMessageArea() {
	s.messageAreaText = ""
	s.messageAreaView.SetText("")
}

func (s *StatusBar) updateLayout() {
	if s.width <= 0 {
		return
	}
	messageWidth := 0
	separatorWidth := 0
	if s.messageText != "" {
		messageWidth = min(tview.TaggedStringWidth(s.messageText), s.width)
		separatorWidth = StatusMessageSeparatorWidth
	}
	s.lowerRow.ResizeItem(s.commonHintsView, tview.TaggedStringWidth(s.commonHintsView.GetText(false)), 0)
	s.lowerRow.ResizeItem(s.messageSpacer, separatorWidth, 0)
	s.lowerRow.ResizeItem(s.messageView, messageWidth, 0)
	s.ResizeItem(s.messageAreaView, s.messageAreaRows(), 0)
}

func (s *StatusBar) requiredHeight() int {
	return StatusBarBaseHeight + s.messageAreaRows()
}

func (s *StatusBar) messageAreaRows() int {
	if s.width <= 0 || s.messageAreaText == "" {
		return 0
	}
	return min(len(tview.WordWrap(s.messageAreaText, s.width)), maxStatusMessageRows)
}
