package views

import (
	"fmt"

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
		"Quit llm-desk?",
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
// closes it via app.HideModal and restores returnHints — the hint line
// the caller was showing before the overlay opened — so each panel
// returns to its own hint line rather than a neighbour's. Mirrors
// confirmQuit's returnHints parameter for the same underlying problem.
func NewHelpOverlay(app App, returnHints string) tview.Primitive {
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetText(helpText)

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

// ── Error modal ───────────────────────────────────────────────────────────────

// errorModal returns a simple message modal for conditions where a form
// cannot be shown (e.g. no providers configured).
func errorModal(message string, onClose func()) tview.Primitive {
	return NewConfirmModal(message, onClose, onClose)
}

// ── centeredModal ─────────────────────────────────────────────────────────────

// centeredModal wraps content in a transparent flex overlay so that it
// appears centred on screen at a fixed width of 64 columns. height is the
// vertical dimension of the inner content, not the overlay.
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

// helpText is the content of the help overlay.
// tview color tags format key names in bold.
const helpText = `
[::b]Navigation[-::-]
  [::b]↑ ↓[-::-]               Move through list items
  [::b]Tab[-::-]               Rotate focus forward
  [::b]Shift+Tab[-::-]         Rotate focus backward

[::b]Accounts list[-::-]
  [::b]n[-::-]                 New account
  [::b]e[-::-]                 Edit selected account
  [::b]d[-::-]                 Delete selected account
  [::b]r[-::-]                 Providers screen
  [::b]p[-::-]                 Projects screen

[::b]Session detail[-::-]
  [::b]n[-::-]                 New session
  [::b]e[-::-]                 Edit selected session
  [::b]d[-::-]                 End selected session
  [::b]u[-::-]                 Open queue editor
  [::b]Space[-::-]             Toggle "continue" prompt

[::b]Providers screen[-::-]
  [::b]n[-::-]                 New provider
  [::b]e[-::-]                 Edit selected provider
  [::b]d[-::-]                 Delete selected provider

[::b]Projects screen[-::-]
  [::b]n[-::-]                 New project
  [::b]e[-::-]                 Edit selected project
  [::b]d[-::-]                 Delete selected project
  [::b]g[-::-]                 Grant to an account
  [::b]x[-::-]                 Revoke from an account

[::b]Queue editor[-::-]
  [::b]n[-::-]                 New prompt
  [::b]e[-::-]                 Edit selected prompt
  [::b]d[-::-]                 Delete selected prompt
  [::b]K[-::-]                 Move prompt up
  [::b]J[-::-]                 Move prompt down

[::b]Forms[-::-]
  [::b]Ctrl+S[-::-]            Save
  [::b]Esc[-::-]               Cancel / close

[::b]Global[-::-]
  [::b]?[-::-]                 This help screen
  [::b]q[-::-]                 Quit
`
