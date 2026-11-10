package panels

import (
	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"

	"fmt"
	"strings"

	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	projectsListTitle        = " Projects "
	projectAccountsListTitle = " Project Accounts "
	projectNotesListTitle    = " Project Notes "
)

// ProjectsPanel is a full-screen view for managing projects: creating,
// editing, deleting, and granting/revoking account access. The operator
// navigates to it with 'p' and returns to the main view with Esc.
type ProjectsPanel struct {
	*tview.Flex
	body        *tview.Flex
	rightColumn *tview.Flex
	list        *tview.List
	accountList *tview.List
	noteList    *tview.List
	infoView    *tview.TextView
	app         views.Host
	projects    []*model.Project // mirrors list order; rebuilt on Refresh
	accounts    []*model.Account // mirrors accountList order; granted accounts for selected project
	notes       []*model.Note    // mirrors noteList order; project notes for selected project
	vendorWidth int
	nameWidth   int
	refreshing  bool
}

// NewProjectsPanel constructs the projects management screen.
func NewProjectsPanel(app views.Host) *ProjectsPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	views.SetListTitle(list, projectsListTitle, 0, 0)

	accountList := tview.NewList().ShowSecondaryText(false)
	accountList.SetBorder(true)
	views.SetListTitle(accountList, projectAccountsListTitle, 0, 0)

	noteList := tview.NewList().ShowSecondaryText(false)
	noteList.SetBorder(true)
	views.SetListTitle(noteList, projectNotesListTitle, 0, 0)

	info := views.NewInfoView("Project Details")

	rightColumn := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(accountList, 0, 1, false).
		AddItem(noteList, 0, 1, false)

	body := tview.NewFlex().
		AddItem(list, 0, 1, true).
		AddItem(rightColumn, 0, 1, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, true).
		AddItem(info, 0, 0, false) // Height is set from content before each draw.

	p := &ProjectsPanel{
		Flex:        flex,
		body:        body,
		rightColumn: rightColumn,
		list:        list,
		accountList: accountList,
		noteList:    noteList,
		infoView:    info,
		app:         app,
	}

	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		views.SetListTitle(list, projectsListTitle, index, len(p.projects))
		if p.refreshing {
			return
		}
		// tview invokes the changed callback before updating currentItem, so
		// use the callback's index rather than selectedProject here.
		proj := p.projectAt(index)
		if err := p.refreshAccountList(proj, false); err != nil {
			p.app.SetError(err.Error())
			return
		}
		if err := p.refreshNoteList(proj, false); err != nil {
			p.app.SetError(err.Error())
			return
		}
		p.renderInfo(proj)
	})

	accountList.SetChangedFunc(func(index int, _, _ string, _ rune) {
		views.SetListTitle(accountList, projectAccountsListTitle, index, len(p.accounts))
	})

	noteList.SetChangedFunc(func(index int, _, _ string, _ rune) {
		views.SetListTitle(noteList, projectNotesListTitle, index, len(p.notes))
	})

	views.WirePanelFocus(list, func() {
		app.SetHints(views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
	})
	views.WirePanelFocus(accountList, func() {
		app.SetHints(views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)
	})
	views.WirePanelFocus(noteList, func() {
		app.SetHints(views.HintsProjectNotesSpecific, views.HintsProjectNotesCommon)
	})

	list.SetInputCapture(p.handleProjectKey)
	accountList.SetInputCapture(p.handleAccountKey)
	noteList.SetInputCapture(p.handleNoteKey)

	views.WireListDoubleClick(list, func() { p.handleEdit() })
	views.WireListDoubleClick(noteList, func() { p.handleEditNote() })

	return p
}

// Refresh repopulates the list from the current service state.
func (p *ProjectsPanel) Refresh() {
	svc := p.app.Service()
	all := svc.Projects() // already sorted by name
	vendorWidth, nameWidth, err := views.ProjectAccountWidths(svc)
	if err != nil {
		reportPanelError(p.app, err)
		return
	}
	p.vendorWidth, p.nameWidth = vendorWidth, nameWidth

	var prevID uint64
	hasPrev := false
	if proj := p.selectedProject(); proj != nil {
		prevID = proj.ID()
		hasPrev = true
	}

	p.refreshing = true
	p.projects = all
	p.list.Clear()
	for _, proj := range p.projects {
		p.list.AddItem(projectItemText(proj), "", 0, nil)
	}

	// Restore selection by project ID so a rename keeps the selection.
	restored := false
	if hasPrev {
		for i, proj := range p.projects {
			if proj.ID() == prevID {
				p.list.SetCurrentItem(i)
				restored = true
				break
			}
		}
	}
	if !restored && len(p.projects) > 0 {
		p.list.SetCurrentItem(0)
	}
	p.refreshing = false

	views.SetListTitle(p.list, projectsListTitle, p.list.GetCurrentItem(), len(p.projects))

	proj := p.selectedProject()
	if err := p.refreshAccountList(proj, true); err != nil {
		p.app.SetError(err.Error())
		return
	}
	if err := p.refreshNoteList(proj, true); err != nil {
		p.app.SetError(err.Error())
		return
	}
	p.renderInfo(proj)
}

// RequiredInfoHeight returns the outer height needed to render the current
// detail text at outerWidth, including the border and wrapped lines.
func (p *ProjectsPanel) RequiredInfoHeight(outerWidth int) int {
	return views.InfoTextHeight(p.infoView.GetText(false), outerWidth)
}

// ResizeInfo sizes the detail panel to its content at outerWidth, leaving at
// least views.MinListPanelHeight rows for the lists.
func (p *ProjectsPanel) ResizeInfo(outerWidth, availableHeight int) {
	infoHeight := views.FitInfoHeight(p.RequiredInfoHeight(outerWidth), availableHeight, views.MinListPanelHeight)
	p.ResizeItem(p.infoView, infoHeight, 0)
	bodyHeight := availableHeight - infoHeight
	if bodyHeight <= 0 || p.rightColumn == nil {
		return
	}
	accountHeight := max(3, bodyHeight/2)
	noteHeight := max(3, bodyHeight-accountHeight)
	if accountHeight+noteHeight > bodyHeight {
		accountHeight = max(3, bodyHeight-noteHeight)
	}
	p.rightColumn.ResizeItem(p.accountList, accountHeight, 0)
	p.rightColumn.ResizeItem(p.noteList, noteHeight, 0)
}

// selectedProject returns the currently highlighted project, or nil when
// the list is empty.
func (p *ProjectsPanel) selectedProject() *model.Project {
	return p.projectAt(p.list.GetCurrentItem())
}

// projectAt returns the project at index, or nil when index is out of range.
func (p *ProjectsPanel) projectAt(index int) *model.Project {
	if index < 0 || index >= len(p.projects) {
		return nil
	}
	return p.projects[index]
}

// selectedAccount returns the currently highlighted granted account, or nil
// when the account list is empty.
func (p *ProjectsPanel) selectedAccount() *model.Account {
	idx := p.accountList.GetCurrentItem()
	if idx < 0 || idx >= len(p.accounts) {
		return nil
	}
	return p.accounts[idx]
}

// refreshAccountList repopulates the granted-accounts list for proj.
// preserveSelection keeps the selected account by key when it still exists;
// otherwise the nearest row is selected. Without preservation the first row
// is selected, which is the expected behaviour after a project change.
func (p *ProjectsPanel) refreshAccountList(proj *model.Project, preserveSelection bool) error {
	prevKey := model.AccountKey{}
	prevIdx := 0
	if preserveSelection {
		prevIdx = p.accountList.GetCurrentItem()
		if acc := p.selectedAccount(); acc != nil {
			prevKey = acc.Key()
		}
	}

	var accounts []*model.Account
	if proj != nil {
		var err error
		accounts, err = p.app.Service().ProjectAccounts(proj.Name())
		if err != nil {
			if !isMissingRead(err) {
				return fmt.Errorf("project accounts: %w", err)
			}
			accounts = nil
		}
	}
	if err := views.SortProjectAccounts(p.app.Service(), accounts); err != nil {
		return fmt.Errorf("sort project accounts: %w", err)
	}
	vendorWidth, nameWidth := p.vendorWidth, p.nameWidth

	rows := make([][]string, len(accounts))
	for i, acc := range accounts {
		user, err := views.ProjectAccountUserName(p.app.Service(), acc)
		if err != nil {
			return fmt.Errorf("project account user: %w", err)
		}
		rows[i] = []string{
			tview.Escape(views.TruncateWidth(acc.VendorName(), vendorWidth)),
			tview.Escape(views.TruncateWidth(user, nameWidth)),
		}
	}
	lines, _ := views.FormatColumns(rows)

	p.accounts = accounts
	p.accountList.Clear()
	for _, line := range lines {
		p.accountList.AddItem(line, "", 0, nil)
	}

	restored := false
	if preserveSelection {
		for i, acc := range p.accounts {
			if acc.Key() == prevKey {
				p.accountList.SetCurrentItem(i)
				restored = true
				break
			}
		}
	}
	if !restored && len(p.accounts) > 0 {
		prevIdx = min(max(prevIdx, 0), len(p.accounts)-1)
		p.accountList.SetCurrentItem(prevIdx)
	}

	views.SetListTitle(p.accountList, projectAccountsListTitle, p.accountList.GetCurrentItem(), len(p.accounts))
	return nil
}

// refreshNoteList repopulates the project notes list for proj.
func (p *ProjectsPanel) refreshNoteList(proj *model.Project, preserveSelection bool) error {
	prevID := uint64(0)
	prevIndex := 0
	if preserveSelection {
		prevIndex = p.noteList.GetCurrentItem()
		if note := p.SelectedNote(); note != nil {
			prevID = note.ID()
		}
	}

	var notes []*model.Note
	if proj != nil {
		got, err := p.app.Service().ProjectNotes(proj.Name())
		if err != nil {
			if !isMissingRead(err) {
				return fmt.Errorf("project notes: %w", err)
			}
			got = nil
		}
		notes = got
	}

	rows := make([][]string, len(notes))
	for i, note := range notes {
		vendor, user := "", ""
		if key, ok := note.AccountKey(); ok {
			name, err := views.ProjectAccountUserNameForKey(p.app.Service(), key)
			if err != nil {
				return fmt.Errorf("note account user: %w", err)
			}
			vendor = tview.Escape(views.TruncateWidth(key.Vendor, views.VendorColumnWidthCap))
			user = tview.Escape(views.TruncateWidth(name, views.NameColumnWidthCap))
		}
		rows[i] = []string{vendor, user, tview.Escape(firstLine(note.Text()))}
	}
	lines, _ := views.FormatColumns(views.DropEmptyColumns(rows))

	p.notes = notes
	p.noteList.Clear()
	for _, line := range lines {
		p.noteList.AddItem(line, "", 0, nil)
	}

	restored := false
	if preserveSelection {
		for i, note := range p.notes {
			if note.ID() == prevID {
				p.noteList.SetCurrentItem(i)
				restored = true
				break
			}
		}
	}
	if !restored && len(p.notes) > 0 {
		prevIndex = min(max(prevIndex, 0), len(p.notes)-1)
		p.noteList.SetCurrentItem(prevIndex)
	}
	views.SetListTitle(p.noteList, projectNotesListTitle, p.noteList.GetCurrentItem(), len(p.notes))
	return nil
}

// SelectedNote returns the selected project note, or nil.
func (p *ProjectsPanel) SelectedNote() *model.Note {
	idx := p.noteList.GetCurrentItem()
	if idx < 0 || idx >= len(p.notes) {
		return nil
	}
	return p.notes[idx]
}

// handleNoteKey processes keyboard input while the project notes list has
// focus.
func (p *ProjectsPanel) handleNoteKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case views.KeyCancel:
		p.app.SwitchToMain()
		return nil
	case views.KeyFocusNext:
		p.app.SetFocus(p.list)
		return nil
	case views.KeyFocusPrev:
		p.app.SetFocus(p.accountList)
		return nil
	case views.KeyInsert:
		return p.handleNewNote()
	case views.KeyEdit:
		return p.handleEditNote()
	case views.KeyDelete:
		return p.handleDeleteNote()
	default:
	}

	if result, ok := views.DispatchPanelKey(event, p.app, views.HintsProjectNotesSpecific, views.HintsProjectNotesCommon); ok {
		return result
	}
	return event
}

func (p *ProjectsPanel) handleNewNote() *tcell.EventKey {
	proj := p.selectedProject()
	if proj == nil {
		return nil
	}
	modal := forms.NewNoteForm(p.app, nil, nil, proj, func() {
		p.app.SetHints(views.HintsProjectNotesSpecific, views.HintsProjectNotesCommon)
		p.app.HideModal()
	})
	views.ShowModal(p.app, modal, views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (p *ProjectsPanel) handleEditNote() *tcell.EventKey {
	proj := p.selectedProject()
	note := p.SelectedNote()
	if proj == nil || note == nil {
		return nil
	}
	modal := forms.NewNoteForm(p.app, note, nil, proj, func() {
		p.app.SetHints(views.HintsProjectNotesSpecific, views.HintsProjectNotesCommon)
		p.app.HideModal()
	})
	views.ShowModal(p.app, modal, views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (p *ProjectsPanel) handleDeleteNote() *tcell.EventKey {
	note := p.SelectedNote()
	if note == nil {
		return nil
	}
	modal := views.NewConfirmModal(
		deleteNoteMessage(note.Text()),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.DeleteNote(note.ID())
			}); err != nil {
				views.SetErrorStatus(p.app, err)
			}
			p.app.SetHints(views.HintsProjectNotesSpecific, views.HintsProjectNotesCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(views.HintsProjectNotesSpecific, views.HintsProjectNotesCommon)
			p.app.HideModal()
		},
	)
	views.ShowModal(p.app, modal, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	return nil
}

// renderInfo populates the detail panel for the selected project.
func (p *ProjectsPanel) renderInfo(proj *model.Project) {
	if proj == nil {
		p.infoView.SetText("(no project selected)")
		return
	}

	var sb strings.Builder

	desc := proj.Description()
	if desc == "" {
		desc = views.DimText("none")
	} else {
		desc = tview.Escape(desc)
	}

	instr := proj.Instructions()
	if instr == "" {
		instr = views.DimText("none")
	} else {
		instr = tview.Escape(instr)
	}

	views.WriteInfoRows(&sb, []views.InfoRow{
		{Label: "Name", Value: tview.Escape(proj.Name())},
		{Label: "Description", Value: desc},
		{Label: "Instructions", Value: instr},
	})

	p.infoView.SetText(strings.TrimSuffix(sb.String(), "\n"))
}

// handleProjectKey processes keyboard input while the project list has focus.
func (p *ProjectsPanel) handleProjectKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case views.KeyCancel:
		p.app.SwitchToMain()
		return nil
	case views.KeyFocusNext:
		p.app.SetFocus(p.accountList)
		return nil
	case views.KeyFocusPrev:
		p.app.SetFocus(p.noteList)
		return nil
	case views.KeyInsert:
		return p.handleNew()
	case views.KeyEdit:
		return p.handleEdit()
	case views.KeyDelete:
		return p.handleDelete()
	default:
	}

	if result, ok := views.DispatchPanelKey(event, p.app, views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon); ok {
		return result
	}

	return event
}

// handleAccountKey processes keyboard input while the granted-accounts list
// has focus.
func (p *ProjectsPanel) handleAccountKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case views.KeyCancel:
		p.app.SwitchToMain()
		return nil
	case views.KeyFocusNext:
		p.app.SetFocus(p.noteList)
		return nil
	case views.KeyFocusPrev:
		p.app.SetFocus(p.list)
		return nil
	case views.KeyDelete:
		return p.handleRevokeSelected()
	case views.KeyInsert:
		return p.handleGrant(views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)
	default:
	}

	if result, ok := views.DispatchPanelKey(event, p.app, views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon); ok {
		return result
	}

	return event
}

func (p *ProjectsPanel) handleNew() *tcell.EventKey {
	modal := forms.NewProjectForm(p.app, nil, func() {
		p.app.SetHints(views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
		p.app.HideModal()
	})
	views.ShowModal(p.app, modal, views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (p *ProjectsPanel) handleEdit() *tcell.EventKey {
	proj := p.selectedProject()
	if proj == nil {
		return nil
	}
	modal := forms.NewProjectForm(p.app, proj, func() {
		p.app.SetHints(views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
		p.app.HideModal()
	})
	views.ShowModal(p.app, modal, views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (p *ProjectsPanel) handleDelete() *tcell.EventKey {
	proj := p.selectedProject()
	if proj == nil {
		return nil
	}
	name := proj.Name()
	modal := views.NewConfirmModal(
		fmt.Sprintf("Delete project %q?\nIt must not be granted to any account.", tview.Escape(name)),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.DeleteProject(name)
			}); err != nil {
				views.SetErrorStatus(p.app, err)
			}
			p.app.SetHints(views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
			p.app.HideModal()
		},
	)
	views.ShowModal(p.app, modal, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	return nil
}

func (p *ProjectsPanel) handleGrant(specific, common string) *tcell.EventKey {
	proj := p.selectedProject()
	if proj == nil {
		return nil
	}
	modal := forms.NewGrantForm(p.app, proj, func() {
		p.app.SetHints(specific, common)
		p.app.HideModal()
	})
	views.ShowModal(p.app, modal, views.HintsFormSpecific, views.HintsFormCommon)
	return nil
}

// handleRevokeSelected asks for confirmation, then revokes the selected
// account's grant from the selected project.
func (p *ProjectsPanel) handleRevokeSelected() *tcell.EventKey {
	proj := p.selectedProject()
	acc := p.selectedAccount()
	if proj == nil || acc == nil {
		return nil
	}
	email := acc.UserEmail()
	vendor := acc.VendorName()
	name := proj.Name()
	label := fmt.Sprintf("%s / %s", tview.Escape(email), tview.Escape(vendor))
	modal := views.NewConfirmModal(
		fmt.Sprintf("Remove account\n%s\nfrom project %q?", label, tview.Escape(name)),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.RevokeProject(email, vendor, name)
			}); err != nil {
				views.SetErrorStatus(p.app, err)
			}
			p.app.SetHints(views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)
			p.app.HideModal()
		},
	)
	views.ShowModal(p.app, modal, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	return nil
}

// projectItemText formats a project row for the list.
func projectItemText(p *model.Project) string {
	return views.InsetListRow(tview.Escape(p.Name()))
}
