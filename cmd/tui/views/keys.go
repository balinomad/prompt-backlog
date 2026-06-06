package views

import "github.com/gdamore/tcell/v2"

// ── Rune keys (single character, checked via event.Rune()) ───────────────────

const (
	// KeyNew creates a new item (session, project, or provider depending on focus).
	KeyNew rune = 'n'

	// KeyEdit opens the edit form for the selected item.
	KeyEdit rune = 'e'

	// KeyDelete opens the delete confirmation modal for the selected item.
	KeyDelete rune = 'd'

	// KeyToggle toggles the mid-answer flag on the selected project or last
	// question. Space is the standard TUI convention for toggling item state.
	KeyToggle rune = ' '

	// KeyHelp opens the help overlay.
	KeyHelp rune = '?'

	// KeyQuit initiates the quit flow (confirmation modal when data is unsaved).
	KeyQuit rune = 'q'

	// KeyProviders switches to the providers management screen.
	KeyProviders rune = 'p'
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
