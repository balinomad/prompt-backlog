package views

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── Confirm modal ─────────────────────────────────────────────────────────────

// showModal displays modal and sets the matching hint lines. A ShowModal
// failure is reported in the status bar.
func showModal(app App, modal tview.Primitive, specific, common string) {
	if err := app.ShowModal(modal); err != nil {
		setErrorStatus(app, err)
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

	return modal
}

// confirmQuit opens the "Quit llm-desk?" confirmation modal, shared by
// every panel's KeyQuit case. returnSpecific/returnCommon are the hint
// lines restored if the operator cancels, so each panel returns to its
// own hint lines rather than a neighbour's.
func confirmQuit(app App, returnSpecific, returnCommon string) {
	modal := NewConfirmModal(
		"Quit "+app.AppName()+"?",
		func() { app.Quit() },
		func() {
			app.SetHints(returnSpecific, returnCommon)
			app.HideModal()
		},
	)
	showModal(app, modal, HintsConfirmModalSpecific, HintsConfirmModalCommon)
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

	// Centre the overlay: horizontal and vertical flex spacers surround a
	// fixed-size box (60 columns × 22 rows).
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, 22, 1, true).
			AddItem(nil, 0, 1, false),
			60, 1, true).
		AddItem(nil, 0, 1, false)
}

// showHelp opens the help overlay, shared by every panel's KeyHelp case.
// returnSpecific/returnCommon are the hint lines restored when the overlay
// closes, so each panel returns to its own hint lines rather than a
// neighbour's.
func showHelp(app App, returnSpecific, returnCommon string) {
	onClose := func() {
		app.SetHints(returnSpecific, returnCommon)
		app.HideModal()
	}
	showModal(app, NewHelpOverlay(onClose), HintsHelpOverlaySpecific, HintsHelpOverlayCommon)
}

// errorModal returns a simple message modal for conditions where a form
// cannot be shown (e.g. no vendors configured).
func errorModal(message string, onClose func()) tview.Primitive {
	return NewConfirmModal(message, onClose, onClose)
}

// centeredModal wraps content in a flex overlay so that it appears centred on
// screen at the given width. height is content's own height, not the overlay's.
func centeredModal(content tview.Primitive, width, height int) tview.Primitive {
	const padding = 1 // rows of solid background directly touching content
	inner := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewBox(), padding, 0, false).
		AddItem(content, height, 1, true).
		AddItem(tview.NewBox(), padding, 0, false).
		AddItem(nil, 0, 1, false)

	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(inner, width, 1, true).
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
		{"Tab", "Switch focus (multi-pane)"},
		{"Shift+Tab", "Switch focus back (multi-pane)"},
	}},
	{"Accounts list", hints{
		{"Enter", "Pause selected account"},
		{"Ins", "Add account"},
		{"Del", "Delete selected account"},
		{string(KeyVendors), "Vendors screen"},
		{string(KeyProjects), "Projects screen"},
		{string(KeyUsers), "Users screen"},
	}},
	{"Session detail", hints{
		{"Enter", "Edit selected session"},
		{"Ins", "Add session"},
		{"Del", "Delete selected session"},
		{string(KeyQueue), "Open messages editor"},
		{"Space", `Toggle "continue" message`},
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
		{"Tab", "Switch to projects"},
	}},
	{"Messages editor", hints{
		{"Enter", "Edit selected message"},
		{"Ins", "Add message"},
		{"Del", "Delete selected message"},
		{"Ctrl+↑", "Move message up"},
		{"Ctrl+↓", "Move message down"},
	}},
	{"Forms", hints{
		{"Tab", "Next field"},
		{"Esc", "Cancel / close"},
	}},
	{"Global", hints{
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
