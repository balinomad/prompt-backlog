package panels

import "github.com/balinomad/prompt-backlog/internal/model"

// FormatPauseUntil exposes formatPauseUntil for black-box tests.
var FormatPauseUntil = formatPauseUntil

// GrantedProjectIDs exposes grantedProjectIDs for black-box tests.
var GrantedProjectIDs = grantedProjectIDs

// LatestGrantedProject exposes latestGrantedProject for black-box tests.
var LatestGrantedProject = latestGrantedProject

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

// ResolvePanelAccount exposes resolvePanelAccount for black-box tests.
var ResolvePanelAccount = resolvePanelAccount

// IsPanelAccountGone exposes isPanelAccountGone for black-box tests.
var IsPanelAccountGone = isPanelAccountGone
