package views

import "github.com/gdamore/tcell/v2"

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
	KeyHelp rune = '?'

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
