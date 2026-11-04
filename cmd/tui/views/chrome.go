package views

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SetErrorStatus displays err in the separate error area.
func SetErrorStatus(app Host, err error) {
	app.SetError(err.Error())
}

// ── Hint blocks ───────────────────────────────────────────────────────────────

// hint is one "[key] description" fragment, e.g. "[::b]?[-::-] help".
// label is string(KeyX) for a rune-bound key, or a fixed name ("Tab",
// "Esc") for a tcell.Key-matched one.
type hint struct {
	label string
	desc  string
}

const (
	hintLabelColumn = 18
	hintSeparator   = " · "
)

// String renders the hint as inline tview markup.
func (h hint) String() string {
	return "[::b]" + h.label + "[-::-] " + h.desc
}

// helpLine renders the hint as a help-overlay row: label padded to a
// fixed column, then the description. Uses rune length, not byte
// length, since some labels contain multi-byte characters.
func (h hint) helpLine() string {
	pad := max(hintLabelColumn-len([]rune(h.label)), 1)
	return "  [::b]" + h.label + "[-::-]" + strings.Repeat(" ", pad) + h.desc
}

// hints is a sequence of hints, joined for either display form.
type hints []hint

// String joins every hint inline with a dim middle-dot separator.
func (hs hints) String() string {
	parts := make([]string, len(hs))
	for i, h := range hs {
		parts[i] = h.String()
	}
	return strings.Join(parts, DimText(hintSeparator))
}

// HelpLines joins every hint as help-overlay reference rows, each
// newline-terminated.
func (hs hints) HelpLines() string {
	var sb strings.Builder
	for _, h := range hs {
		sb.WriteString(h.helpLine())
		sb.WriteString("\n")
	}
	return sb.String()
}

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
	hintFilter   = hint{string(KeyFilter), "project filter"}
	hintSave     = hint{"Ctrl+S", "save"}
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
		{"Tab", "sessions"}, {"Shift+Tab", "notes"},
		hintPause, hintAdd, hintDelete,
	}.String()

	// HintsAccountsPanelCommon is the common hint line shown when the
	// accounts list has focus.
	HintsAccountsPanelCommon = hints{
		hintFilter, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsSessionsPanelSpecific is the context-specific hint line shown when
	// the Sessions pane has focus.
	HintsSessionsPanelSpecific = hints{
		hintEdit, {"Ins", "new session"}, {"Del", "delete"},
		{"Tab", "notes"}, {"Shift+Tab", "accounts"},
	}.String()

	// HintsSessionsPanelCommon is the common hint line shown when the Sessions
	// pane has focus.
	HintsSessionsPanelCommon = hints{
		hintFilter, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsNotesPanelSpecific is the context-specific hint line shown when
	// the Notes pane has focus.
	HintsNotesPanelSpecific = hints{
		hintEdit, hintAdd, hintDelete,
		{"Tab", "accounts"}, {"Shift+Tab", "sessions"},
	}.String()

	// HintsNotesPanelCommon is the common hint line shown when the Notes pane
	// has focus.
	HintsNotesPanelCommon = hints{
		hintFilter, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
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
		{"Tab", "accounts"}, {"Shift+Tab", "notes"},
		hintEdit, hintAdd, hintDelete,
	}.String()

	// HintsProjectsPanelCommon is the common hint line shown when the
	// projects list has focus.
	HintsProjectsPanelCommon = hints{
		hintAccounts, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsProjectAccountsSpecific is the context-specific hint line shown
	// when the granted-accounts list has focus.
	HintsProjectAccountsSpecific = hints{
		{"Tab", "project notes"}, {"Shift+Tab", "projects"},
		hintAdd, hintDelete,
	}.String()

	// HintsProjectAccountsCommon is the common hint line shown when the
	// granted-accounts list has focus.
	HintsProjectAccountsCommon = hints{
		hintAccounts, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsProjectNotesSpecific is the context-specific hint line shown when
	// the project notes list has focus.
	HintsProjectNotesSpecific = hints{
		{"Tab", "projects"}, {"Shift+Tab", "project accounts"},
		hintEdit, hintAdd, hintDelete,
	}.String()

	// HintsProjectNotesCommon is the common hint line shown when the project
	// notes list has focus.
	HintsProjectNotesCommon = hints{
		hintAccounts, hintVendors, hintProjects, hintUsers, hintHelp, hintQuit,
	}.String()

	// HintsFormSpecific is the context-specific hint line shown when a form
	// modal is open. It is empty because the Save button is the submit path.
	HintsFormSpecific = hints{}.String()

	// HintsFormCommon is the common hint line shown when a form modal is
	// open.
	HintsFormCommon = hints{
		{"Tab", "next"}, hintSave, hintCancel,
	}.String()

	// HintsPromptListSpecific is the context-specific hint line shown when the
	// prompt list in the Edit Prompt modal has focus.
	HintsPromptListSpecific = hints{
		{"Enter", "edit"}, hintAdd, hintDelete,
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
		HintsSessionsPanelSpecific,
		HintsNotesPanelSpecific,
		HintsVendorsPanelSpecific,
		HintsUsersPanelSpecific,
		HintsProjectsPanelSpecific,
		HintsProjectAccountsSpecific,
		HintsProjectNotesSpecific,
		HintsFormSpecific,
		HintsConfirmModalSpecific,
		HintsHelpOverlaySpecific,
	}
	common := []string{
		HintsAccountsPanelCommon,
		HintsSessionsPanelCommon,
		HintsNotesPanelCommon,
		HintsVendorsPanelCommon,
		HintsUsersPanelCommon,
		HintsProjectsPanelCommon,
		HintsProjectAccountsCommon,
		HintsProjectNotesCommon,
		HintsFormCommon,
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
	s.messageAreaView.SetTextColor(ColorAccent)
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

// ── Confirm modal ─────────────────────────────────────────────────────────────

// ShowModal displays modal and sets the matching hint lines. A ShowModal
// failure is reported in the status bar.
func ShowModal(app Host, modal tview.Primitive, specific, common string) {
	if err := app.ShowModal(modal); err != nil {
		SetErrorStatus(app, err)
		return
	}
	app.SetHints(specific, common)
}

// NewConfirmModal returns a centered confirmation dialog with Cancel and
// Confirm buttons. Cancel is focused by default to prevent accidental
// destructive actions.
//
// onConfirm is called when the operator selects Confirm.
// onCancel is called when the operator selects Cancel or presses Esc.
// The caller is responsible for showing and hiding the modal via
// app.ShowModal and app.HideModal.
func NewConfirmModal(message string, onConfirm, onCancel func()) tview.Primitive {
	modal := tview.NewModal().
		SetText(message).
		// Cancel is listed first so it receives initial focus — safe default
		// for destructive operations.
		AddButtons([]string{"Cancel", "Confirm"}).
		SetDoneFunc(func(_ int, buttonLabel string) {
			if buttonLabel == "Confirm" {
				onConfirm()
			} else {
				onCancel()
			}
		})

	// tview.Modal does not handle Esc natively; capture it explicitly.
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == KeyCancel {
			onCancel()
			return nil
		}
		return event
	})

	return NewModalState(modal, modal)
}

// ConfirmQuit opens the "Quit llm-desk?" confirmation modal, shared by
// every panel's KeyQuit case. returnSpecific/returnCommon are the hint
// lines restored if the operator cancels, so each panel returns to its
// own hint lines rather than a neighbour's.
func ConfirmQuit(app Host, returnSpecific, returnCommon string) {
	modal := NewConfirmModal(
		"Quit "+app.AppName()+"?",
		func() { app.Quit() },
		func() {
			app.SetHints(returnSpecific, returnCommon)
			app.HideModal()
		},
	)
	ShowModal(app, modal, HintsConfirmModalSpecific, HintsConfirmModalCommon)
}

// ── Help overlay ──────────────────────────────────────────────────────────────

// NewHelpOverlay returns a scrollable keyboard-reference overlay. Esc
// calls onClose.
func NewHelpOverlay(onClose func()) tview.Primitive {
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetText(helpText)
	text.SetBackgroundColor(tview.Styles.ContrastBackgroundColor)

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == KeyCancel {
			onClose()
			return nil
		}
		return event
	})

	frame := tview.NewFrame(text).
		SetBorders(1, 1, 0, 0, 2, 2).
		AddText(" Keyboard Reference ", true, tview.AlignCenter, tcell.ColorWhite)
	frame.SetBorder(true)
	frame.SetBackgroundColor(tview.Styles.ContrastBackgroundColor)

	return NewModalState(CenteredModal(frame, 60, 22), frame)
}

// showHelp opens the help overlay, shared by every panel's KeyHelp case.
// returnSpecific/returnCommon are the hint lines restored when the overlay
// closes, so each panel returns to its own hint lines rather than a
// neighbour's.
func showHelp(app Host, returnSpecific, returnCommon string) {
	onClose := func() {
		app.SetHints(returnSpecific, returnCommon)
		app.HideModal()
	}
	ShowModal(app, NewHelpOverlay(onClose), HintsHelpOverlaySpecific, HintsHelpOverlayCommon)
}

// ErrorModal returns a simple message modal for conditions where a form
// cannot be shown (e.g. no vendors configured).
func ErrorModal(message string, onClose func()) tview.Primitive {
	return NewConfirmModal(message, onClose, onClose)
}

// CenteredModal wraps content in a flex overlay so that it appears centred on
// screen at the given width. height is content's own height, not the overlay's.
func CenteredModal(content tview.Primitive, width, height int) tview.Primitive {
	const padding = 1
	row := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewBox(), padding, 0, false).
		AddItem(content, width, 1, true).
		AddItem(tview.NewBox(), padding, 0, false).
		AddItem(nil, 0, 1, false)

	column := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewBox(), padding, 0, false).
		AddItem(row, height, 1, true).
		AddItem(tview.NewBox(), padding, 0, false).
		AddItem(nil, 0, 1, false)

	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(column, width+2, 1, true).
		AddItem(nil, 0, 1, false)
}

// helpSection is one titled block of key-reference rows in the help
// overlay.
type helpSection struct {
	title string
	rows  hints
}

// String renders the section title followed by its rows.
func (s helpSection) String() string {
	return "[::b]" + s.title + "[-::-]\n" + s.rows.HelpLines()
}

// helpSections is the content of the help overlay, grouped by screen.
// Rune-bound keys use string(KeyX); special keys (no printable rune) use
// a literal display string instead.
var helpSections = []helpSection{
	{"Navigation", hints{
		{"↑ ↓", "Move through list items"},
		{"Tab", "Switch to next pane"},
		{"Shift+Tab", "Switch to previous pane"},
	}},
	{"Accounts list", hints{
		{"Enter", "Pause selected account"},
		{"Ins", "Add account"},
		{"Del", "Delete selected account"},
		{string(KeyFilter), "Cycle project filter"},
	}},
	{"Sessions pane", hints{
		{"Enter", "Edit prompt"},
		{"Ins", "Start new session"},
		{"Del", "Delete session"},
		{string(KeyFilter), "Cycle project filter"},
	}},
	{"Notes pane", hints{
		{"Enter", "Edit selected note"},
		{"Ins", "Add note"},
		{"Del", "Delete selected note"},
		{string(KeyFilter), "Cycle project filter"},
	}},
	{"Vendors screen", hints{
		{"Enter", "Edit selected vendor"},
		{"Ins", "Add vendor"},
		{"Del", "Delete selected vendor"},
	}},
	{"Users screen", hints{
		{"Enter", "Edit selected user"},
		{"Ins", "Add user"},
		{"Del", "Delete selected user"},
	}},
	{"Projects screen", hints{
		{"Enter", "Edit selected project"},
		{"Ins", "Add project"},
		{"Del", "Delete selected project"},
		{"Tab", "Switch to project accounts"},
	}},
	{"Project accounts", hints{
		{"Ins", "Add account to project"},
		{"Del", "Delete selected account from project"},
		{"Tab", "Switch to project notes"},
	}},
	{"Project notes", hints{
		{"Ins", "Add note to project"},
		{"Enter", "Edit selected note"},
		{"Del", "Delete selected note"},
		{"Tab", "Switch to projects"},
	}},
	{"Forms", hints{
		{"Tab", "Next field"},
		{"Ctrl+S", "Save"},
		{"Esc", "Cancel / close"},
	}},
	{"Global", hints{
		{string(KeyVendors), "Vendors screen"},
		{string(KeyProjects), "Projects screen"},
		{string(KeyUsers), "Users screen"},
		{string(KeyHelp), "This help screen"},
		{string(KeyQuit), "Quit"},
	}},
}

// helpText is the content of the help overlay, built from helpSections.
var helpText = buildHelpText()

func buildHelpText() string {
	var sb strings.Builder
	sb.WriteString("\n")
	for i, s := range helpSections {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(s.String())
	}
	return sb.String()
}
