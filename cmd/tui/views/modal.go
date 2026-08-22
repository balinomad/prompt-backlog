package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── Confirm modal ─────────────────────────────────────────────────────────────

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
// every panel's KeyQuit case. returnHints is the hint line restored if the
// operator cancels, so each panel returns to its own hint line rather than
// a neighbour's.
func confirmQuit(app App, returnHints string) *tcell.EventKey {
	modal := NewConfirmModal(
		"Quit "+app.AppName()+"?",
		func() { app.Quit() },
		func() {
			app.SetHints(returnHints)
			app.HideModal()
		},
	)
	if err := app.ShowModal(modal); err != nil {
		app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	app.SetHints(HintsConfirmModal)
	return nil
}

// ── Help overlay ──────────────────────────────────────────────────────────────

// NewHelpOverlay returns a scrollable keyboard-reference overlay. Esc
// closes it and restores returnHints, the hint line shown before the
// overlay opened.
func NewHelpOverlay(app App, returnHints string) tview.Primitive {
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetText(helpText)
	text.SetBackgroundColor(tview.Styles.ContrastBackgroundColor)

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == KeyCancel {
			app.SetHints(returnHints)
			app.HideModal()
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
// returnHints is the hint line restored when the overlay closes, so each
// panel returns to its own hint line rather than a neighbour's.
func showHelp(app App, returnHints string) *tcell.EventKey {
	if err := app.ShowModal(NewHelpOverlay(app, returnHints)); err != nil {
		app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	app.SetHints(HintsHelpOverlay)
	return nil
}

// errorModal returns a simple message modal for conditions where a form
// cannot be shown (e.g. no providers configured).
func errorModal(message string, onClose func()) tview.Primitive {
	return NewConfirmModal(message, onClose, onClose)
}

// centeredModal wraps content in a flex overlay so that it appears
// centred on screen at a fixed width of 64 columns. height is content's
// own height, not the overlay's.
func centeredModal(content tview.Primitive, height int) tview.Primitive {
	const width = 64
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(content, height, 1, true).
			AddItem(nil, 0, 1, false),
			width, 1, true).
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
// Uses string(KeyX) throughout so a rebind needs no second edit.
var helpSections = []helpSection{
	{"Navigation", hints{
		{"↑ ↓", "Move through list items"},
		{"Tab", "Rotate focus forward"},
		{"Shift+Tab", "Rotate focus backward"},
	}},
	{"Accounts list", hints{
		{string(KeyNew), "New account"},
		{string(KeyEdit), "Edit selected account"},
		{string(KeyDelete), "Delete selected account"},
		{string(KeyProviders), "Providers screen"},
		{string(KeyProjects), "Projects screen"},
	}},
	{"Session detail", hints{
		{string(KeyNew), "New session"},
		{string(KeyEdit), "Edit selected session"},
		{string(KeyDelete), "End selected session"},
		{string(KeyQueue), "Open queue editor"},
		{"Space", `Toggle "continue" prompt`},
	}},
	{"Providers screen", hints{
		{string(KeyNew), "New provider"},
		{string(KeyEdit), "Edit selected provider"},
		{string(KeyDelete), "Delete selected provider"},
	}},
	{"Projects screen", hints{
		{string(KeyNew), "New project"},
		{string(KeyEdit), "Edit selected project"},
		{string(KeyDelete), "Delete selected project"},
		{string(KeyGrant), "Grant to an account"},
		{string(KeyRevoke), "Revoke from an account"},
	}},
	{"Queue editor", hints{
		{string(KeyNew), "New prompt"},
		{string(KeyEdit), "Edit selected prompt"},
		{string(KeyDelete), "Delete selected prompt"},
		{string(KeyMoveUp), "Move prompt up"},
		{string(KeyMoveDown), "Move prompt down"},
	}},
	{"Forms", hints{
		{"Ctrl+S", "Save"},
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
