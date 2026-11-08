package panels

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"

	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const sessionsListTitle = " Sessions"

// SessionsPanel lists the account's sessions in activity order. Each row is one
// session; the prompt queue is opened in a form rather than inline.
type SessionsPanel struct {
	*tview.List
	app      views.Host
	account  *model.Account
	filter   *views.ProjectFilterState
	sessions []*model.Session
	width    int

	refreshing     bool
	onFilterChange func()
}

// NewSessionsPanel constructs the sessions pane.
func NewSessionsPanel(app views.Host) *SessionsPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	views.SetListTitle(list, sessionsListTitle+" ", 0, 0)

	p := &SessionsPanel{
		List:   list,
		app:    app,
		filter: views.NewProjectFilterState(),
	}
	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if p.refreshing {
			return
		}
		p.setTitle(index)
	})
	views.WirePanelFocus(list, func() {
		app.SetHints(views.HintsSessionsPanelSpecific, views.HintsSessionsPanelCommon)
	})
	list.SetInputCapture(p.handleKey)
	views.WireListDoubleClick(list, func() { p.handleEnter() })
	return p
}

// SetOnFilterChange registers a callback invoked after the filter advances.
func (p *SessionsPanel) SetOnFilterChange(fn func()) {
	p.onFilterChange = fn
}

// SetFilter shares the main-screen project filter with the pane.
func (p *SessionsPanel) SetFilter(filter *views.ProjectFilterState) {
	if filter == nil {
		filter = views.NewProjectFilterState()
	}
	p.filter = filter
}

// SetAccount sets the account in view and refreshes the pane.
func (p *SessionsPanel) SetAccount(acc *model.Account) {
	p.account = acc
	p.Refresh()
}

// SetWidth updates the inner pane width and refreshes when it changes.
func (p *SessionsPanel) SetWidth(width int) {
	if width == p.width {
		return
	}
	p.width = width
	p.Refresh()
}

// Refresh repopulates the pane from the service state.
func (p *SessionsPanel) Refresh() {
	svc := p.app.Service()

	acc := resolvePanelAccount(svc, p.account, p.filter)
	p.account = acc

	var sessions []*model.Session
	if acc != nil {
		all, err := svc.AccountSessionsActivityOrder(acc.UserEmail(), acc.VendorName())
		if err == nil {
			for _, s := range all {
				if p.filter.Value().Matches(s.ProjectID()) {
					sessions = append(sessions, s)
				}
			}
		}
	}

	prevID := uint64(0)
	if selected := p.selectedSession(); selected != nil {
		prevID = selected.ID()
	}
	p.rebuild(sessions, prevID)
}

// SelectedSession returns the selected session, or nil.
func (p *SessionsPanel) SelectedSession() *model.Session {
	return p.selectedSession()
}

func (p *SessionsPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case views.KeyEdit:
		return p.handleEnter()
	case views.KeyInsert:
		return p.handleStart()
	case views.KeyDelete:
		return p.handleDeleteSession()
	case views.KeyCancel:
		views.ConfirmQuit(p.app, views.HintsSessionsPanelSpecific, views.HintsSessionsPanelCommon)
		return nil
	default:
	}

	if result, ok := views.DispatchPanelKey(event, p.app, views.HintsSessionsPanelSpecific, views.HintsSessionsPanelCommon); ok {
		return result
	}

	if event.Rune() == views.KeyFilter {
		p.advanceFilter()
		return nil
	}
	return event
}

func (p *SessionsPanel) handleEnter() *tcell.EventKey {
	session := p.selectedSession()
	if session == nil {
		return nil
	}
	mode := forms.PromptFormEdit
	if len(session.Queue()) == 0 {
		mode = forms.PromptFormAdd
	}
	views.ShowModal(p.app, forms.NewPromptForm(p.app, session, forms.PromptFormOptions{
		Mode:    mode,
		Index:   0,
		OnClose: p.closeForm,
		OnSaveAndPause: func() {
			p.openPauseForSession(session)
		},
	}), views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (p *SessionsPanel) openPauseForSession(session *model.Session) {
	account, err := p.app.Service().Account(session.AccountKey().Email, session.AccountKey().Vendor)
	if err != nil {
		p.app.SetError(err.Error())
		return
	}
	views.ShowModal(p.app, forms.NewAccountForm(p.app, account, p.closeForm), views.HintsFormSpecific, views.HintsTextFormCommon)
}

func (p *SessionsPanel) handleDeleteSession() *tcell.EventKey {
	session := p.selectedSession()
	if session == nil {
		return nil
	}
	email := session.AccountKey().Email
	vendor := session.AccountKey().Vendor

	modal := views.NewConfirmModal(
		deleteSessionMessage(session.Name(), session.ID(), len(session.Queue())),
		func() {
			if err := p.app.Mutate(func(svc *service.Service) error {
				return svc.DeleteSession(email, vendor, session.ID())
			}); err != nil {
				views.SetErrorStatus(p.app, err)
			}
			p.closeForm()
		},
		p.closeForm,
	)
	views.ShowModal(p.app, modal, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	return nil
}

func (p *SessionsPanel) handleStart() *tcell.EventKey {
	if p.account == nil {
		return nil
	}
	account := p.account
	onSubmit := func(fields service.SessionFields) error {
		var started *model.Session
		if err := p.app.Mutate(func(svc *service.Service) error {
			sess, err := svc.StartSession(account.UserEmail(), account.VendorName(), fields)
			if err != nil {
				return err
			}
			started = sess
			return nil
		}); err != nil {
			return fmt.Errorf("start session: %w", err)
		}
		if started != nil {
			p.selectSessionByID(started.ID())
		}
		p.closeForm()
		return nil
	}
	views.ShowModal(p.app, forms.NewSessionForm(p.app, &forms.SessionFormOptions{
		Title:       "Start Session",
		SubmitLabel: "Start",
		Account:     account,
		OnSubmit:    onSubmit,
		OnCancel:    p.closeForm,
	}), views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (p *SessionsPanel) advanceFilter() {
	p.filter.Cycle()
	if p.onFilterChange != nil {
		p.onFilterChange()
		return
	}
	p.Refresh()
}

func (p *SessionsPanel) closeForm() {
	p.app.SetHints(views.HintsSessionsPanelSpecific, views.HintsSessionsPanelCommon)
	p.app.HideModal()
}

func deleteSessionMessage(name string, id uint64, queueLen int) string {
	label := name
	if label == "" {
		label = fmt.Sprintf("session #%d", id)
	}
	message := fmt.Sprintf("Delete session\n%q?", tview.Escape(label))
	if queueLen > 0 {
		message += fmt.Sprintf("\nIts %d queued prompts will be deleted.", queueLen)
	}
	return message
}

func (p *SessionsPanel) selectSessionByID(id uint64) {
	for i, session := range p.sessions {
		if session.ID() == id {
			p.SetCurrentItem(i)
			p.setTitle(i)
			return
		}
	}
}

// ShowSession selects the session in the list.
func (p *SessionsPanel) ShowSession(sess *model.Session) {
	if sess == nil {
		return
	}
	p.Refresh()
	p.selectSessionByID(sess.ID())
}

func (p *SessionsPanel) rebuild(sessions []*model.Session, prevID uint64) {
	p.refreshing = true
	p.sessions = sessions
	p.Clear()

	nameWidth := sessionNameWidth(p.width, sessions)
	cells := make([][]string, len(sessions))
	for i, s := range sessions {
		cells[i] = sessionRowCells(s, nameWidth)
	}
	lines, _ := views.FormatColumns(views.DropEmptyColumns(cells))
	for i, line := range lines {
		if sessions[i].Status() == model.SessionStatusClosed {
			line = "[gray]" + line + "[-]"
		}
		p.AddItem(line, "", 0, nil)
	}
	p.refreshing = false

	index := 0
	if prevID != 0 {
		for i, s := range sessions {
			if s.ID() == prevID {
				index = i
				break
			}
		}
	}
	if len(sessions) > 0 {
		p.SetCurrentItem(index)
	}
	p.setTitle(p.GetCurrentItem())
}

func (p *SessionsPanel) selectedSession() *model.Session {
	idx := p.GetCurrentItem()
	if idx < 0 || idx >= len(p.sessions) {
		return nil
	}
	return p.sessions[idx]
}

func (p *SessionsPanel) setTitle(index int) {
	base := sessionsListTitle + p.filter.TitleSuffix() + " "
	views.SetListTitle(p.List, base, index, len(p.sessions))
}

const minSessionNameWidth = 8

func sessionNameWidth(available int, sessions []*model.Session) int {
	if available <= 0 {
		return views.SessionNameColumnWidth
	}

	hasProject := false
	hasMarker := false
	projectWidth := 0
	countWidth := 0
	for _, s := range sessions {
		if s.ProjectName() != "" {
			hasProject = true
			projectWidth = max(projectWidth, views.PlainWidth(views.TruncateWidth(s.ProjectName(), views.ProjectColumnWidth)))
		}
		countWidth = max(countWidth, len(strconv.Itoa(len(s.Queue()))))
		hasMarker = hasMarker || s.Status() != model.SessionStatusActive
	}

	columns := 2 // session name and prompt count
	fixed := 2   // left and right insets
	if hasProject {
		columns++
		fixed += projectWidth
	}
	fixed += countWidth
	if hasMarker {
		columns++
		fixed++
	}
	fixed += tview.TaggedStringWidth(views.ColumnSeparator) * (columns - 1)

	return max(minSessionNameWidth, available-fixed)
}

func sessionRowCells(s *model.Session, nameWidth int) []string {
	project := ""
	if s.ProjectName() != "" {
		project = tview.Escape(views.TruncateWidth(s.ProjectName(), views.ProjectColumnWidth))
	}

	rawName := s.Name()
	dimName := rawName == "" && s.Status() != model.SessionStatusClosed
	if rawName == "" {
		rawName = "(untitled)"
	}
	name := tview.Escape(views.TruncateWidth(rawName, nameWidth))
	if dimName {
		name = views.DimText(name)
	}
	name = views.PadWidth(name, nameWidth)

	marker := ""
	switch s.Status() {
	case model.SessionStatusActive:
	case model.SessionStatusNeedsClosing:
		marker = "[yellow]▲[-]"
	case model.SessionStatusClosed:
		marker = "✓"
	}

	return []string{
		project,
		name,
		strconv.Itoa(len(s.Queue())),
		marker,
	}
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}
