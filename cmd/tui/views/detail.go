package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// projectColumnWidth caps project names in the sessions list and account rows.
const projectColumnWidth = 14

// ── DetailPanel ───────────────────────────────────────────────────────────────

// DetailPanel occupies the right side of the screen. The upper section lists
// the selected account's sessions (topic, project, queue); the lower section
// shows static account information (user, provider, pause state, granted
// projects).
type DetailPanel struct {
	*tview.Flex
	sessionsList *tview.List
	infoView     *tview.TextView
	app          App
	account      *model.Account   // currently displayed account; may be nil
	sessions     []*model.Session // mirrors sessionsList items; rebuilt on Refresh
}

// NewDetailPanel constructs the detail panel. Call Refresh after the
// accounts panel has provided an initial selection.
func NewDetailPanel(app App) *DetailPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Sessions ")

	info := tview.NewTextView().SetDynamicColors(true)
	info.SetBorder(true)
	info.SetTitle(" Account ")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(info, 8, 0, false) // fixed 8-row info panel (4 content lines + border + spare)

	p := &DetailPanel{
		Flex:         flex,
		sessionsList: list,
		infoView:     info,
		app:          app,
	}

	wirePanelFocus(list, func() {
		app.SetHints(HintsDetailPanelSpecific, HintsDetailPanelCommon)
	})

	list.SetInputCapture(p.handleKey)

	return p
}

// SetAccount sets the account to display and refreshes the panel.
// Called by the host App when the accounts panel selection changes.
func (p *DetailPanel) SetAccount(acc *model.Account) {
	p.account = acc
	p.Refresh()
}

// Selected returns the currently highlighted session, or nil when the list
// is empty or no account is selected.
func (p *DetailPanel) Selected() *model.Session {
	idx := p.sessionsList.GetCurrentItem()
	if idx < 0 || idx >= len(p.sessions) {
		return nil
	}
	return p.sessions[idx]
}

// Refresh repopulates the panel from the current service state.
// The account reference is re-validated against the registry so that a
// deleted account gracefully clears the panel rather than showing stale data.
func (p *DetailPanel) Refresh() {
	svc := p.app.Service()

	// Re-validate: look the account up by identity in case it was deleted.
	var acc *model.Account
	if p.account != nil {
		resolved, err := svc.Account(p.account.UserEmail(), p.account.ProviderName())
		if err == nil {
			acc = resolved
		}
	}
	p.account = acc

	// Remember selection by session ID before clearing
	prevID, hadPrev := uint64(0), false
	if s := p.Selected(); s != nil {
		prevID, hadPrev = s.ID(), true
	}

	p.sessionsList.Clear()
	p.sessions = nil
	if acc != nil {
		sessions, err := svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
		if err == nil {
			p.sessions = sessions
		}
		for _, s := range p.sessions {
			p.sessionsList.AddItem(sessionItemText(s), "", 0, nil)
		}
	}

	// Restore selection by session ID
	restored := false
	if hadPrev {
		for i, s := range p.sessions {
			if s.ID() == prevID {
				p.sessionsList.SetCurrentItem(i)
				restored = true
				break
			}
		}
	}
	if !restored && len(p.sessions) > 0 {
		p.sessionsList.SetCurrentItem(0)
	}

	p.renderInfo(acc)
}

// ── Private: rendering ────────────────────────────────────────────────────────

func (p *DetailPanel) renderInfo(acc *model.Account) {
	if acc == nil {
		p.infoView.SetText("  (no account selected)")
		return
	}
	svc := p.app.Service()
	email := acc.UserEmail()
	provider := acc.ProviderName()

	userName := email
	if u := svc.User(email); u != nil {
		userName = u.Name()
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "  [::b]User[-::-]     %s (%s)\n", tview.Escape(userName), tview.Escape(email))
	fmt.Fprintf(&sb, "  [::b]Provider[-::-] %s\n", tview.Escape(provider))
	fmt.Fprintf(&sb, "  [::b]Paused[-::-]   %s\n", pauseStatusText(acc.PauseUntil(), time.Now()))

	projects, err := svc.AccountProjects(email, provider)
	if err != nil {
		// Account was already validated above; a resolution error here would
		// indicate a concurrent deletion. Render with no projects rather
		// than surfacing a status message from a passive read
		projects = nil
	}
	if len(projects) == 0 {
		fmt.Fprintf(&sb, "  [::b]Projects[-::-] %s", dimText("none"))
	} else {
		// No sessions, or no project on the latest one, leaves order and
		// colour untouched below.
		latestProjectName := latestSessionProjectName(p.sessions)

		names := make([]string, 0, len(projects))
		var highlighted string
		for _, proj := range projects {
			if proj.Name() == latestProjectName {
				highlighted = fmt.Sprintf("[green]%s[-]", tview.Escape(proj.Name()))
				continue
			}
			names = append(names, tview.Escape(proj.Name()))
		}
		if highlighted != "" {
			names = append([]string{highlighted}, names...)
		}
		fmt.Fprintf(&sb, "  [::b]Projects[-::-] %s", strings.Join(names, ", "))
	}

	p.infoView.SetText(sb.String())
}

// ── Private: key handling ─────────────────────────────────────────────────────

func (p *DetailPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == KeyDelete {
		return p.handleDelete()
	}

	if result, ok := dispatchPanelKey(event, p.app, HintsDetailPanelSpecific, HintsDetailPanelCommon); ok {
		return result
	}

	switch event.Rune() {
	case KeyNew:
		return p.handleNew()
	case KeySessionFields:
		return p.handleEdit()
	case KeyQueue:
		return p.handleQueue()
	case KeyToggle:
		return p.handleToggle()
	}
	return event
}

func (p *DetailPanel) handleNew() *tcell.EventKey {
	if p.account == nil {
		return nil
	}
	modal := NewSessionForm(p.app, p.account, nil, func() {
		p.app.SetHints(HintsDetailPanelSpecific, HintsDetailPanelCommon)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsFormSpecific, HintsFormCommon)
	return nil
}

func (p *DetailPanel) handleEdit() *tcell.EventKey {
	s := p.Selected()
	if s == nil || p.account == nil {
		return nil
	}
	modal := NewSessionForm(p.app, p.account, s, func() {
		p.app.SetHints(HintsDetailPanelSpecific, HintsDetailPanelCommon)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsFormSpecific, HintsFormCommon)
	return nil
}

func (p *DetailPanel) handleDelete() *tcell.EventKey {
	s := p.Selected()
	if s == nil || p.account == nil {
		return nil
	}
	email := p.account.UserEmail()
	provider := p.account.ProviderName()
	id := s.ID()
	label := s.Topic()
	if label == "" {
		label = fmt.Sprintf("session #%d", id)
	}
	modal := NewConfirmModal(
		fmt.Sprintf("End session\n%q?", tview.Escape(label)),
		func() {
			if err := p.app.Mutate(func(svc *service.Service) error {
				return svc.EndSession(email, provider, id)
			}); err != nil {
				p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			}
			p.app.SetHints(HintsDetailPanelSpecific, HintsDetailPanelCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(HintsDetailPanelSpecific, HintsDetailPanelCommon)
			p.app.HideModal()
		},
	)
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsConfirmModalSpecific, HintsConfirmModalCommon)
	return nil
}

func (p *DetailPanel) handleQueue() *tcell.EventKey {
	s := p.Selected()
	if s == nil || p.account == nil {
		return nil
	}
	modal := NewQueueEditor(p.app, p.account, s, func() {
		p.app.SetHints(HintsDetailPanelSpecific, HintsDetailPanelCommon)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsQueueEditorSpecific, HintsQueueEditorCommon)
	return nil
}

func (p *DetailPanel) handleToggle() *tcell.EventKey {
	s := p.Selected()
	if s == nil || p.account == nil {
		return nil
	}
	email := p.account.UserEmail()
	provider := p.account.ProviderName()
	id := s.ID()
	queue := toggleContinuePrompt(s.Queue())
	if err := p.app.Mutate(func(svc *service.Service) error {
		return svc.UpdateSessionPrompts(email, provider, id, queue)
	}); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
	}
	return nil
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// latestSessionProjectName returns the project name of the latest session in
// sessions, or "" when there are no sessions. AccountSessions sorts by ID, so
// the last entry is the most recently added session.
func latestSessionProjectName(sessions []*model.Session) string {
	if len(sessions) == 0 {
		return ""
	}
	return sessions[len(sessions)-1].ProjectName()
}

// sessionItemText formats a session row for the list: topic, project, and
// queue length. A pending trailing "continue" prompt — the queue-based
// equivalent of the old mid-answer flag — is highlighted in yellow.
func sessionItemText(s *model.Session) string {
	topic := tview.Escape(truncate(s.Topic(), 26))
	if topic == "" {
		topic = dimText("(untitled)")
	}
	proj := tview.Escape(truncate(s.ProjectName(), projectColumnWidth))
	if proj == "" {
		proj = dimText("—")
	}
	queue := s.Queue()
	info := fmt.Sprintf("queue: %d", len(queue))
	if isContinuePending(queue) {
		info = fmt.Sprintf("[yellow]%s ↻[-]", info)
	}
	return fmt.Sprintf("%-28s  %-16s  %s", topic, proj, info)
}

// isContinuePending reports whether queue's final entry is the "continue"
// convenience prompt toggled by KeyToggle.
func isContinuePending(queue []model.Prompt) bool {
	if len(queue) == 0 {
		return false
	}
	return queue[len(queue)-1] == model.NewPrompt("continue", false)
}

// toggleContinuePrompt returns queue with a trailing "continue" prompt
// appended, or removed if already present. Prompt is comparable via == —
// see model.TestPrompt_Comparable — so equality here is exact-field, not
// pointer, comparison.
func toggleContinuePrompt(queue []model.Prompt) []model.Prompt {
	if isContinuePending(queue) {
		return queue[:len(queue)-1]
	}
	return append(queue, model.NewPrompt("continue", false))
}

// truncate shortens s to maxRunes, appending "…" when truncated.
func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-1]) + "…"
}

// dimText wraps s in tview's dim color tag.
func dimText(s string) string {
	return "[::d]" + s + "[-::-]"
}
