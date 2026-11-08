package panels

import (
	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"

	"fmt"
	"slices"

	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const notesListTitle = " Notes"

type noteColumn int

const (
	noteColumnProject noteColumn = iota
	noteColumnAccount
	noteColumnText
)

// NotesPanel lists the visible notes for the account in view.
type NotesPanel struct {
	*tview.List
	app            views.Host
	account        *model.Account
	filter         *views.ProjectFilterState
	notes          []*model.Note
	width          int
	refreshing     bool
	onFilterChange func()
	onSessionStart func(*model.Session)
}

// NewNotesPanel constructs the notes pane.
func NewNotesPanel(app views.Host) *NotesPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	views.SetListTitle(list, notesListTitle+" ", 0, 0)

	p := &NotesPanel{
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
		app.SetHints(views.HintsNotesPanelSpecific, views.HintsNotesPanelCommon)
	})
	list.SetInputCapture(p.handleKey)
	views.WireListDoubleClick(list, func() { p.handleEdit() })
	return p
}

// SetFilter shares the main-screen project filter with the pane.
func (p *NotesPanel) SetFilter(filter *views.ProjectFilterState) {
	if filter == nil {
		filter = views.NewProjectFilterState()
	}
	p.filter = filter
}

// SetOnFilterChange registers a callback invoked after the filter advances.
func (p *NotesPanel) SetOnFilterChange(fn func()) { p.onFilterChange = fn }

// SetOnSessionStart registers a callback invoked with a newly started session.
func (p *NotesPanel) SetOnSessionStart(fn func(*model.Session)) { p.onSessionStart = fn }

// SetAccount sets the account in view and refreshes the pane.
func (p *NotesPanel) SetAccount(acc *model.Account) {
	p.account = acc
	p.Refresh()
}

// SetWidth updates the inner width and refreshes when it changes.
func (p *NotesPanel) SetWidth(width int) {
	if width == p.width {
		return
	}
	p.width = width
	p.Refresh()
}

// Refresh repopulates the pane from the service state.
func (p *NotesPanel) Refresh() {
	svc := p.app.Service()

	acc := resolvePanelAccount(svc, p.account, p.filter)
	p.account = acc

	prevID := uint64(0)
	prevIndex := p.GetCurrentItem()
	if note := p.SelectedNote(); note != nil {
		prevID = note.ID()
	}

	var notes []*model.Note
	if acc != nil {
		visible, err := svc.VisibleNotes(acc.UserEmail(), acc.VendorName(), p.filter.Value())
		if err == nil {
			notes = visible
		}
	}
	p.notes = notes

	p.refreshing = true
	p.Clear()
	for _, line := range noteRows(notes, svc, p.width) {
		p.AddItem(line, "", 0, nil)
	}
	p.refreshing = false

	index := 0
	found := false
	for i, note := range p.notes {
		if note.ID() == prevID {
			index = i
			found = true
			break
		}
	}
	if !found && len(p.notes) > 0 {
		index = min(max(prevIndex, 0), len(p.notes)-1)
	}
	if len(p.notes) > 0 {
		p.SetCurrentItem(index)
	}
	p.setTitle(p.GetCurrentItem())
}

// SelectedNote returns the selected note, or nil.
func (p *NotesPanel) SelectedNote() *model.Note {
	idx := p.GetCurrentItem()
	if idx < 0 || idx >= len(p.notes) {
		return nil
	}
	return p.notes[idx]
}

func (p *NotesPanel) setTitle(index int) {
	base := notesListTitle + p.filter.TitleSuffix() + " "
	views.SetListTitle(p.List, base, index, len(p.notes))
}

func (p *NotesPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case views.KeyEdit:
		return p.handleEdit()
	case views.KeyInsert:
		return p.handleInsert()
	case views.KeyDelete:
		return p.handleDelete()
	case views.KeyCancel:
		views.ConfirmQuit(p.app, views.HintsNotesPanelSpecific, views.HintsNotesPanelCommon)
		return nil
	default:
	}

	if result, ok := views.DispatchPanelKey(event, p.app, views.HintsNotesPanelSpecific, views.HintsNotesPanelCommon); ok {
		return result
	}

	if event.Rune() == views.KeyFilter {
		p.advanceFilter()
		return nil
	}
	return event
}

func (p *NotesPanel) handleEdit() *tcell.EventKey {
	note := p.SelectedNote()
	if note == nil {
		return nil
	}
	views.ShowModal(p.app, forms.NewNoteForm(p.app, note, p.account, nil, p.closeForm), views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (p *NotesPanel) handleInsert() *tcell.EventKey {
	views.ShowModal(p.app, forms.NewNoteForm(p.app, nil, p.account, nil, p.closeForm), views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (p *NotesPanel) handleDelete() *tcell.EventKey {
	note := p.SelectedNote()
	if note == nil {
		return nil
	}
	modal := views.NewConfirmModal(
		deleteNoteMessage(note.Text()),
		func() {
			if err := p.app.Mutate(func(svc *service.Service) error {
				return svc.DeleteNote(note.ID())
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

func (p *NotesPanel) advanceFilter() {
	p.filter.Cycle()
	if p.onFilterChange != nil {
		p.onFilterChange()
		return
	}
	p.Refresh()
}

func (p *NotesPanel) closeForm() {
	p.app.SetHints(views.HintsNotesPanelSpecific, views.HintsNotesPanelCommon)
	p.app.HideModal()
}

func deleteNoteMessage(text string) string {
	label := tview.Escape(views.TruncateWidth(firstLine(text), 40))
	return fmt.Sprintf("Delete note\n%q?", label)
}

func noteRows(notes []*model.Note, svc *service.Service, width int) []string {
	raw := make([][]string, len(notes))
	hasProject := false
	hasAccount := false
	for i, note := range notes {
		project := tview.Escape(views.TruncateWidth(note.ProjectName(), views.ProjectColumnWidth))
		account := ""
		if key, ok := note.AccountKey(); ok {
			account = tview.Escape(views.TruncateWidth(views.UserDisplayName(svc, key.Email), views.NameColumnWidthCap))
		}
		raw[i] = []string{project, account, tview.Escape(firstLine(note.Text()))}
		hasProject = hasProject || project != ""
		hasAccount = hasAccount || account != ""
	}

	columns := make([]noteColumn, 0, 3)
	if hasProject {
		columns = append(columns, noteColumnProject)
	}
	if hasAccount {
		columns = append(columns, noteColumnAccount)
	}
	columns = append(columns, noteColumnText)

	if width > 0 {
		for {
			remaining := noteTextRemaining(raw, columns, width)
			if remaining >= 12 {
				break
			}
			if slices.Contains(columns, noteColumnAccount) {
				columns = removeNoteColumn(columns, noteColumnAccount)
				continue
			}
			if slices.Contains(columns, noteColumnProject) {
				columns = removeNoteColumn(columns, noteColumnProject)
				continue
			}
			break
		}
		remaining := max(noteTextRemaining(raw, columns, width), 1)
		for i, note := range notes {
			raw[i][noteColumnText] = tview.Escape(views.TruncateWidth(firstLine(note.Text()), remaining))
		}
	}

	rows := make([][]string, len(raw))
	for i := range raw {
		for _, column := range columns {
			rows[i] = append(rows[i], raw[i][column])
		}
	}
	lines, _ := views.FormatColumns(views.DropEmptyColumns(rows))
	return lines
}

func noteTextRemaining(raw [][]string, columns []noteColumn, width int) int {
	rows := make([][]string, len(raw))
	for i := range raw {
		for _, column := range columns {
			if column == noteColumnText {
				rows[i] = append(rows[i], "")
			} else {
				rows[i] = append(rows[i], raw[i][column])
			}
		}
	}
	_, used := views.FormatColumns(rows)
	return width - used
}

func removeNoteColumn(columns []noteColumn, remove noteColumn) []noteColumn {
	next := make([]noteColumn, 0, len(columns))
	for _, column := range columns {
		if column != remove {
			next = append(next, column)
		}
	}
	return next
}
