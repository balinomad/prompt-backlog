package views

import (
	"fmt"
	"sort"
	"time"

	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SessionsPanel is the left panel that lists all sessions with their
// rate-limit status. Selecting a session notifies the detail panel via
// the onSelect callback wired by the host App.
type SessionsPanel struct {
	*tview.List
	app        App
	sessions   []*model.Session // mirrors list order; rebuilt on every Refresh
	onSelect   func(*model.Session)
	refreshing bool // suppresses onSelect during Refresh
}

// NewSessionsPanel constructs the sessions list. Call SetOnSelect to wire
// the detail panel before the first Refresh.
func NewSessionsPanel(app App) *SessionsPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Sessions ")

	p := &SessionsPanel{
		List: list,
		app:  app,
	}

	// Navigation changes: notify the detail panel unless we are mid-refresh.
	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if p.refreshing {
			return
		}
		if p.onSelect != nil && index >= 0 && index < len(p.sessions) {
			p.onSelect(p.sessions[index])
		}
	})

	list.SetFocusFunc(func() {
		app.SetHints(HintsSessionsPanel)
	})

	list.SetInputCapture(p.handleKey)

	return p
}

// SetOnSelect registers a callback that fires whenever the selected session
// changes. Called by the host App to wire this panel to the detail panel.
func (p *SessionsPanel) SetOnSelect(fn func(*model.Session)) {
	p.onSelect = fn
}

// Selected returns the currently highlighted session, or nil when the list
// is empty.
func (p *SessionsPanel) Selected() *model.Session {
	idx := p.GetCurrentItem()
	if idx < 0 || idx >= len(p.sessions) {
		return nil
	}
	return p.sessions[idx]
}

// Refresh repopulates the list from the current registry state.
// The previously selected session is re-selected by identity (email + provider)
// if it still exists; otherwise the first item is selected.
// Callers must not hold the tview draw lock when calling Refresh.
func (p *SessionsPanel) Refresh() {
	reg := p.app.Registry()
	all := reg.AllSessions()
	sortSessionsByAvailability(all)

	// Remember current selection by identity before clearing.
	var prevEmail, prevProvider string
	if s := p.Selected(); s != nil {
		prevEmail = s.User().Email()
		prevProvider = s.Provider().Name()
	}

	p.refreshing = true
	p.sessions = all
	p.Clear()
	for _, s := range p.sessions {
		p.AddItem(sessionItemText(s), "", 0, nil)
	}
	p.refreshing = false

	// Restore selection by identity.
	restored := false
	for i, s := range p.sessions {
		if s.User().Email() == prevEmail && s.Provider().Name() == prevProvider {
			p.SetCurrentItem(i)
			restored = true
			break
		}
	}
	if !restored && len(p.sessions) > 0 {
		p.SetCurrentItem(0)
	}

	// Notify detail panel of the current selection after rebuild.
	if p.onSelect != nil && len(p.sessions) > 0 {
		idx := p.GetCurrentItem()
		if idx >= 0 && idx < len(p.sessions) {
			p.onSelect(p.sessions[idx])
		}
	}
}

// handleKey processes keyboard input for this panel.
func (p *SessionsPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	// Rune keys.
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case KeyNew:
			p.app.SetHints(HintsForm)
			p.app.ShowModal(NewSessionForm(p.app, nil, func() {
				p.app.SetHints(HintsSessionsPanel)
				p.app.HideModal()
			}))
			return nil

		case KeyEdit:
			if s := p.Selected(); s != nil {
				p.app.SetHints(HintsForm)
				p.app.ShowModal(NewSessionForm(p.app, s, func() {
					p.app.SetHints(HintsSessionsPanel)
					p.app.HideModal()
				}))
			}
			return nil

		case KeyDelete:
			if s := p.Selected(); s != nil {
				label := fmt.Sprintf("%s / %s", s.User().Email(), s.Provider().Name())
				p.app.SetHints(HintsConfirmModal)
				p.app.ShowModal(NewConfirmModal(
					fmt.Sprintf("Remove session\n%s?", label),
					func() {
						if err := p.app.Mutate(func(r *model.Registry) error {
							return r.RemoveSession(s.User().Email(), s.Provider().Name())
						}); err != nil {
							p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
						}
						p.app.SetHints(HintsSessionsPanel)
						p.app.HideModal()
					},
					func() {
						p.app.SetHints(HintsSessionsPanel)
						p.app.HideModal()
					},
				))
			}
			return nil

		case KeyProviders:
			p.app.SwitchToProviders()
			return nil

		case KeyUnattached:
			p.app.SwitchToUnattached()
			return nil

		case KeyHelp:
			p.app.SetHints(HintsHelpOverlay)
			p.app.ShowModal(NewHelpOverlay(p.app))
			return nil

		case KeyQuit:
			p.app.SetHints(HintsConfirmModal)
			p.app.ShowModal(NewConfirmModal(
				"Quit llm-desk?",
				func() { p.app.Quit() },
				func() {
					p.app.SetHints(HintsSessionsPanel)
					p.app.HideModal()
				},
			))
			return nil
		}
	}
	return event
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// sessionItemText formats a session as a single line for the list.
// Format: "email / provider" with the rate-limit time appended when set.
// A past NextPromptAt is shown in red; a future one in the default colour.
func sessionItemText(s *model.Session) string {
	base := fmt.Sprintf("%s / %s", s.User().Email(), s.Provider().Name())
	if s.NextPromptAt == nil {
		return base
	}
	timeStr, isPast := formatNextPromptAt(s.NextPromptAt)
	if isPast {
		return fmt.Sprintf("%-40s [red]%s[-]", base, timeStr)
	}
	return fmt.Sprintf("%-40s %s", base, timeStr)
}

// formatNextPromptAt formats a NextPromptAt timestamp for inline display.
// Returns the formatted string and whether the time is in the past.
// Same-day times are shown as "HH:MM"; other days as "MM-DD HH:MM".
func formatNextPromptAt(t *time.Time) (string, bool) {
	now := time.Now()
	isPast := t.Before(now)
	y, m, d := t.Date()
	ny, nm, nd := now.Date()
	if y == ny && m == nm && d == nd {
		return t.Format("15:04"), isPast
	}
	return t.Format("01-02 15:04"), isPast
}

// sortSessionsByAvailability sorts sessions so that available sessions
// (no timer, or timer in the past) appear first, followed by waiting
// sessions ordered by soonest available time.
func sortSessionsByAvailability(sessions []*model.Session) {
	now := time.Now()
	sort.SliceStable(sessions, func(i, j int) bool {
		ti := sessions[i].NextPromptAt
		tj := sessions[j].NextPromptAt
		iAvail := ti == nil || ti.Before(now)
		jAvail := tj == nil || tj.Before(now)
		if iAvail != jAvail {
			return iAvail
		}
		if ti == nil || iAvail {
			return false // stable among available
		}
		return ti.Before(*tj)
	})
}
