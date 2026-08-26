package views

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ── Rune keys (single character, checked via event.Rune()) ───────────────────

const (
	// KeyNew creates a new item (account, session, project, or provider
	// depending on focus).
	KeyNew rune = 'n'

	// KeyEdit opens the edit form for the selected item.
	KeyEdit rune = 'e'

	// KeyDelete opens the delete confirmation modal for the selected item.
	KeyDelete rune = 'd'

	// KeyToggle toggles a "continue" prompt entry on the selected session's
	// queue. Space is the standard TUI convention for toggling item state.
	KeyToggle rune = ' '

	// KeyHelp opens the help overlay.
	KeyHelp rune = 'h'

	// KeyQuit initiates the quit flow (confirmation modal).
	KeyQuit rune = 'q'

	// KeyProviders switches to the providers management screen.
	KeyProviders rune = 'r'

	// KeyProjects switches to the projects management screen.
	KeyProjects rune = 'p'

	// KeyQueue opens the queue editor for the selected session.
	KeyQueue rune = 'u'

	// KeyGrant grants the selected project to an account (ProjectsPanel).
	KeyGrant rune = 'g'

	// KeyRevoke revokes the selected project from an account (ProjectsPanel).
	KeyRevoke rune = 'x'

	// KeyMoveUp moves the selected queue entry one position earlier.
	KeyMoveUp rune = 'K'

	// KeyMoveDown moves the selected queue entry one position later.
	KeyMoveDown rune = 'J'

	// KeyRenameUser opens the rename form for the user backing the
	// selected account (AccountsPanel).
	KeyRenameUser rune = 'R'
)

// ── Special keys (checked via event.Key()) ───────────────────────────────────

const (
	// KeySave submits the active form.
	// Ctrl+S is borrowed from editor convention; it is unambiguous in forms.
	KeySave = tcell.KeyCtrlS

	// KeyCancel closes the active form or modal without saving.
	KeyCancel = tcell.KeyEscape

	// KeyFocusNext rotates focus forward through the visible panels.
	KeyFocusNext = tcell.KeyTab

	// KeyFocusPrev rotates focus backward through the visible panels.
	KeyFocusPrev = tcell.KeyBacktab
)

// ── Hint blocks ───────────────────────────────────────────────────────────────

// hint is one "[key] description" fragment, e.g. "[::b]h[-::-] help".
// label is string(KeyX) for a rune-bound key, or a fixed name ("Tab",
// "Esc") for a tcell.Key-matched one.
type hint struct {
	label string
	desc  string
}

// String renders the hint as inline tview markup.
func (h hint) String() string {
	return "[::b]" + h.label + "[-::-] " + h.desc
}

// helpLine renders the hint as a help-overlay row: label padded to a
// fixed column, then the description. Uses rune length, not byte
// length, since some labels contain multi-byte characters.
func (h hint) helpLine() string {
	const labelColumn = 18
	pad := max(labelColumn-len([]rune(h.label)), 1)
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
func dispatchPanelKey(event *tcell.EventKey, app App, hints string) (*tcell.EventKey, bool) {
	if event.Key() != tcell.KeyRune {
		return event, true
	}
	switch event.Rune() {
	case KeyHelp:
		showHelp(app, hints)
		return nil, true
	case KeyQuit:
		confirmQuit(app, hints)
		return nil, true
	}
	return nil, false
}
