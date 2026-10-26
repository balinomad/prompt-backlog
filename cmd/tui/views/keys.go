package views

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── Rune keys (single character, checked via event.Rune()) ───────────────────

const (
	// KeyToggle toggles a "continue" prompt entry on the selected session's
	// queue. Space is the standard TUI convention for toggling item state.
	KeyToggle rune = ' '

	// KeyHelp opens the help overlay.
	KeyHelp rune = '?'

	// KeyQuit initiates the quit flow (confirmation modal).
	KeyQuit rune = 'q'

	// KeyProviders switches to the providers management screen.
	KeyProviders rune = 'r'

	// KeyProjects switches to the projects management screen.
	KeyProjects rune = 'p'

	// KeyQueue opens the queue editor for the selected session.
	KeyQueue rune = 'u'
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

// ── Hint blocks ───────────────────────────────────────────────────────────────

// hint is one "[key] description" fragment, e.g. "[::b]?[-::-] help".
// label is string(KeyX) for a rune-bound key, or a fixed name ("Tab",
// "Esc") for a tcell.Key-matched one.
type hint struct {
	label string
	desc  string
}

const hintLabelColumn = 18

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

// String joins every hint inline, matching the status bar's existing
// two-space separator.
func (hs hints) String() string {
	parts := make([]string, len(hs))
	for i, h := range hs {
		parts[i] = h.String()
	}
	return strings.Join(parts, "  ")
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

// dispatchPanelKey applies the input filtering and Help/Quit handling
// shared by every full-screen panel. The second return is true when event
// was fully handled here — a non-rune event, KeyHelp, or KeyQuit — and
// the caller should return the first value unchanged. It is false for any
// other rune, leaving the caller's own switch to dispatch it.
func dispatchPanelKey(event *tcell.EventKey, app App, specific, common string) (*tcell.EventKey, bool) {
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

// wirePanelFocus registers list's focus and blur callbacks: onFocus runs
// first (e.g. status-bar hints), then the border turns yellow while list
// has focus, reverting to the current default border colour on blur.
// Shared by every focusable list, including both lists in ProjectsPanel.
// Modals opened via App.ShowModal are out of scope here.
func wirePanelFocus(list *tview.List, onFocus func()) {
	list.SetFocusFunc(func() {
		onFocus()
		list.SetBorderColor(tcell.ColorYellow)
	})
	list.SetBlurFunc(func() {
		list.SetBorderColor(tview.Styles.BorderColor)
	})
}
