package views

import (
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

// ── Help overlay ──────────────────────────────────────────────────────────────

// NewHelpOverlay returns a scrollable keyboard-reference overlay.
// Esc closes the overlay by calling app.HideModal.
func NewHelpOverlay(app App) tview.Primitive {
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetText(helpText)

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == KeyCancel {
			app.SetHints(HintsSessionsPanel)
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

// helpText is the content of the help overlay.
// tview color tags format key names in bold.
const helpText = `
[::b]Navigation[-::-]
  [::b]↑ ↓[-::-]               Move through list items
  [::b]Tab[-::-]               Rotate focus forward
  [::b]Shift+Tab[-::-]         Rotate focus backward
  [::b]Enter[-::-]             Expand session → focus projects

[::b]Actions[-::-]
  [::b]n[-::-]                 New item
  [::b]e[-::-]                 Edit selected item
  [::b]d[-::-]                 Delete selected item
  [::b]Space[-::-]             Toggle mid-answer flag

[::b]Screens[-::-]
  [::b]s[-::-]                 Unattached projects screen

[::b]Forms[-::-]
  [::b]Ctrl+S[-::-]            Save
  [::b]Esc[-::-]               Cancel / close

[::b]Global[-::-]
  [::b]?[-::-]                 This help screen
  [::b]q[-::-]                 Quit
`
