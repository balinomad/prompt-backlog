package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── Rune keys (single character, checked via event.Rune()) ───────────────────

const (
	// KeyHelp opens the help overlay.
	KeyHelp rune = '?'

	// KeyQuit initiates the quit flow (confirmation modal).
	KeyQuit rune = 'q'

	// KeyVendors switches to the vendors management screen.
	KeyVendors rune = 'v'

	// KeyProjects switches to the projects management screen.
	KeyProjects rune = 'p'

	// KeyUsers switches to the users management screen.
	KeyUsers rune = 'u'

	// KeyStart starts a session from the selected note.
	KeyStart rune = 's'

	// KeyFilter advances the project filter.
	KeyFilter rune = 'f'
)

// ── Special keys (checked via event.Key()) ───────────────────────────────────

const (
	// KeyEdit opens the edit form for the selected item.
	KeyEdit = tcell.KeyEnter

	// KeyDelete deletes the selected item; in the ProjectsPanel account list
	// it deletes the selected account from the project.
	KeyDelete = tcell.KeyDelete

	// KeyInsert adds a new item; in the ProjectsPanel account list it opens
	// the account picker to add an account to the selected project.
	KeyInsert = tcell.KeyInsert

	// KeySave submits the active form. It is a keyboard convenience only:
	// the Save button is the primary submit path, and the shortcut is not
	// shown in the status bar or help overlay.
	KeySave = tcell.KeyCtrlS

	// KeyCancel closes the active form or modal without saving.
	KeyCancel = tcell.KeyEscape

	// KeyFocusNext moves focus to the next panel on multi-pane screens.
	KeyFocusNext = tcell.KeyTab

	// KeyFocusPrev moves focus to the previous panel on multi-pane screens.
	KeyFocusPrev = tcell.KeyBacktab
)

// DispatchPanelKey applies the input filtering and Help/Quit handling
// shared by every full-screen panel. The second return is true when event
// was fully handled here — a non-rune event, KeyHelp, or KeyQuit — and
// the caller should return the first value unchanged. It is false for any
// other rune, leaving the caller's own switch to dispatch it.
func DispatchPanelKey(event *tcell.EventKey, app Host, specific, common string) (*tcell.EventKey, bool) {
	if event.Key() != tcell.KeyRune {
		return event, true
	}
	switch event.Rune() {
	case KeyHelp:
		showHelp(app, specific, common)
		return nil, true
	case KeyQuit:
		confirmQuit(app, specific, common)
		return nil, true
	}
	return nil, false
}

// ── Focus/border wiring ───────────────────────────────────────────────────────

// accentColor is used for the focused pane border and the notice area.
const accentColor = tcell.ColorYellow

// WirePanelFocus registers list's focus and blur callbacks: onFocus runs
// first (e.g. status-bar hints), then the border turns yellow while list
// has focus, reverting to the current default border colour on blur.
// Shared by every focusable list, including both lists in ProjectsPanel.
// Modals opened via Host.ShowModal are out of scope here.
func WirePanelFocus(list *tview.List, onFocus func()) {
	list.SetFocusFunc(func() {
		onFocus()
		list.SetBorderColor(accentColor)
	})
	list.SetBlurFunc(func() {
		list.SetBorderColor(tview.Styles.BorderColor)
	})
}

// WireListDoubleClick calls action when the operator double-clicks inside
// list. The first click of the double click has already selected the row, so
// action can use the list's current item. A double click on the border is
// ignored.
func WireListDoubleClick(list *tview.List, action func()) {
	list.SetMouseCapture(func(mouseAction tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if mouseAction != tview.MouseLeftDoubleClick || !list.InInnerRect(event.Position()) {
			return mouseAction, event
		}
		action()
		return tview.MouseConsumed, nil
	})
}
