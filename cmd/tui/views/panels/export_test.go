package panels

import "github.com/balinomad/llm-desk/internal/model"

// FormatPauseUntil exposes formatPauseUntil for black-box tests.
var FormatPauseUntil = formatPauseUntil

// NoteRows exposes noteRows for black-box tests.
var NoteRows = noteRows

// DeleteNoteMessage exposes deleteNoteMessage for black-box tests.
var DeleteNoteMessage = deleteNoteMessage

// DeleteSessionMessage exposes deleteSessionMessage for black-box tests.
var DeleteSessionMessage = deleteSessionMessage

// OpenPauseForSession exposes openPauseForSession for black-box tests.
func OpenPauseForSession(p *SessionsPanel, session *model.Session) {
	p.openPauseForSession(session)
}
