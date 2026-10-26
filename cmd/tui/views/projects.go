package views

import (
	"fmt"
	"slices"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProjectsPanel is a full-screen view for managing projects: creating,
// editing, deleting, and granting/revoking account access. The operator
// navigates to it with 'p' and returns to the main view with Esc.
type ProjectsPanel struct {
	*tview.Flex
	list        *tview.List
	accountList *tview.List
	infoView    *tview.TextView
	app         App
	projects    []*model.Project // mirrors list order; rebuilt on Refresh
	accounts    []*model.Account // mirrors accountList order; granted accounts for selected project
	vendorWidth int
	nameWidth   int
	refreshing  bool
}

// NewProjectsPanel constructs the projects management screen.
func NewProjectsPanel(app App) *ProjectsPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Projects ")

	accountList := tview.NewList().ShowSecondaryText(false)
	accountList.SetBorder(true)
	accountList.SetTitle(" Project Accounts ")

	info := newInfoView("Project Details")

	body := tview.NewFlex().
		AddItem(list, 0, 1, true).
		AddItem(accountList, 0, 1, false)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, true).
		AddItem(info, 0, 0, false) // Height is set from content before each draw.

	p := &ProjectsPanel{
		Flex:        flex,
		list:        list,
		accountList: accountList,
		infoView:    info,
		app:         app,
	}

	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if p.refreshing {
			return
		}
		// tview invokes the changed callback before updating currentItem, so
		// use the callback's index rather than selectedProject here.
		proj := p.projectAt(index)
		p.refreshAccountList(proj, false)
		p.renderInfo(proj)
	})

	wirePanelFocus(list, func() {
		app.SetHints(HintsProjectsPanelSpecific, HintsProjectsPanelCommon)
	})
	wirePanelFocus(accountList, func() {
		app.SetHints(HintsProjectAccountsSpecific, HintsProjectAccountsCommon)
	})

	list.SetInputCapture(p.handleProjectKey)
	accountList.SetInputCapture(p.handleAccountKey)

	return p
}

// Refresh repopulates the list from the current service state.
func (p *ProjectsPanel) Refresh() {
	svc := p.app.Service()
	all := svc.Projects() // already sorted by name
	p.vendorWidth, p.nameWidth = projectAccountWidths(svc)

	var prevName string
	if proj := p.selectedProject(); proj != nil {
		prevName = proj.Name()
	}

	p.refreshing = true
	p.projects = all
	p.list.Clear()
	for _, proj := range p.projects {
		p.list.AddItem(projectItemText(proj), "", 0, nil)
	}

	// Restore selection by name.
	restored := false
	for i, proj := range p.projects {
		if proj.Name() == prevName {
			p.list.SetCurrentItem(i)
			restored = true
			break
		}
	}
	if !restored && len(p.projects) > 0 {
		p.list.SetCurrentItem(0)
	}
	p.refreshing = false

	proj := p.selectedProject()
	p.refreshAccountList(proj, true)
	p.renderInfo(proj)
}

// RequiredInfoHeight returns the outer height needed to render the current
// detail text at outerWidth, including the border and wrapped lines.
func (p *ProjectsPanel) RequiredInfoHeight(outerWidth int) int {
	return infoTextHeight(p.infoView.GetText(false), outerWidth)
}

// ResizeInfo sizes the detail panel to its content at outerWidth, leaving at
// least minListPanelHeight rows for the lists.
func (p *ProjectsPanel) ResizeInfo(outerWidth, availableHeight int) {
	p.ResizeItem(
		p.infoView,
		fitInfoHeight(p.RequiredInfoHeight(outerWidth), availableHeight, minListPanelHeight),
		0,
	)
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
func (p *ProjectsPanel) refreshAccountList(proj *model.Project, preserveSelection bool) {
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
			accounts = nil
		}
	}
	sortProjectAccounts(p.app.Service(), accounts)
	vendorWidth, nameWidth := p.vendorWidth, p.nameWidth

	rows := make([][]string, len(accounts))
	for i, acc := range accounts {
		rows[i] = []string{
			tview.Escape(truncateWidth(acc.VendorName(), vendorWidth)),
			tview.Escape(truncateWidth(projectAccountUserName(p.app.Service(), acc), nameWidth)),
		}
	}
	lines, _ := formatColumns(rows)

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
		desc = dimText("none")
	} else {
		desc = tview.Escape(desc)
	}

	instr := proj.Instructions()
	if instr == "" {
		instr = dimText("none")
	} else {
		instr = tview.Escape(instr)
	}

	writeInfoRows(&sb, []infoRow{
		{label: "Name", value: tview.Escape(proj.Name())},
		{label: "Description", value: desc},
		{label: "Instructions", value: instr},
	})

	p.infoView.SetText(strings.TrimSuffix(sb.String(), "\n"))
}

// handleProjectKey processes keyboard input while the project list has focus.
func (p *ProjectsPanel) handleProjectKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case KeyCancel:
		p.app.SwitchToMain()
		return nil
	case KeyFocusNext, KeyFocusPrev:
		p.app.SetFocus(p.accountList)
		return nil
	case KeyInsert:
		return p.handleNew()
	case KeyEdit:
		return p.handleEdit()
	case KeyDelete:
		return p.handleDelete()
	default:
	}

	if result, ok := dispatchPanelKey(event, p.app, HintsProjectsPanelSpecific, HintsProjectsPanelCommon); ok {
		return result
	}

	return event
}

// handleAccountKey processes keyboard input while the granted-accounts list
// has focus.
func (p *ProjectsPanel) handleAccountKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case KeyCancel:
		p.app.SetFocus(p.list)
		return nil
	case KeyFocusNext, KeyFocusPrev:
		p.app.SetFocus(p.list)
		return nil
	case KeyDelete:
		return p.handleRevokeSelected()
	case KeyInsert:
		return p.handleGrant(HintsProjectAccountsSpecific, HintsProjectAccountsCommon)
	default:
	}

	if result, ok := dispatchPanelKey(event, p.app, HintsProjectAccountsSpecific, HintsProjectAccountsCommon); ok {
		return result
	}

	return event
}

func (p *ProjectsPanel) handleNew() *tcell.EventKey {
	modal := NewProjectForm(p.app, nil, func() {
		p.app.SetHints(HintsProjectsPanelSpecific, HintsProjectsPanelCommon)
		p.app.HideModal()
	})
	showModal(p.app, modal, HintsFormSpecific, HintsFormCommon)
	return nil
}

func (p *ProjectsPanel) handleEdit() *tcell.EventKey {
	proj := p.selectedProject()
	if proj == nil {
		return nil
	}
	modal := NewProjectForm(p.app, proj, func() {
		p.app.SetHints(HintsProjectsPanelSpecific, HintsProjectsPanelCommon)
		p.app.HideModal()
	})
	showModal(p.app, modal, HintsFormSpecific, HintsFormCommon)
	return nil
}

func (p *ProjectsPanel) handleDelete() *tcell.EventKey {
	proj := p.selectedProject()
	if proj == nil {
		return nil
	}
	name := proj.Name()
	modal := NewConfirmModal(
		fmt.Sprintf("Delete project %q?\nIt must not be granted to any account.", tview.Escape(name)),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.DeleteProject(name)
			}); err != nil {
				setErrorStatus(p.app, err)
			}
			p.app.SetHints(HintsProjectsPanelSpecific, HintsProjectsPanelCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(HintsProjectsPanelSpecific, HintsProjectsPanelCommon)
			p.app.HideModal()
		},
	)
	showModal(p.app, modal, HintsConfirmModalSpecific, HintsConfirmModalCommon)
	return nil
}

func (p *ProjectsPanel) handleGrant(specific, common string) *tcell.EventKey {
	proj := p.selectedProject()
	if proj == nil {
		return nil
	}
	modal := NewGrantForm(p.app, proj, func() {
		p.app.SetHints(specific, common)
		p.app.HideModal()
	})
	showModal(p.app, modal, HintsFormSpecific, HintsFormCommon)
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
	modal := NewConfirmModal(
		fmt.Sprintf("Remove account\n%s\nfrom project %q?", label, tview.Escape(name)),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.RevokeProject(email, vendor, name)
			}); err != nil {
				setErrorStatus(p.app, err)
			}
			p.app.SetHints(HintsProjectAccountsSpecific, HintsProjectAccountsCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(HintsProjectAccountsSpecific, HintsProjectAccountsCommon)
			p.app.HideModal()
		},
	)
	showModal(p.app, modal, HintsConfirmModalSpecific, HintsConfirmModalCommon)
	return nil
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// projectAccountUserName returns the account's user display name, falling
// back to the email when the user record is missing.
func projectAccountUserName(svc *service.Service, acc *model.Account) string {
	if u := svc.User(acc.UserEmail()); u != nil {
		return u.Name()
	}
	return acc.UserEmail()
}

// projectAccountText formats one granted account as Vendor │ User for the
// grant form. The vendor and user name are truncated to vendorWidth and
// nameWidth; both widths are capped by the shared project-screen limits.
func projectAccountText(svc *service.Service, acc *model.Account, vendorWidth, nameWidth int) string {
	vendor := truncateWidth(acc.VendorName(), vendorWidth)
	name := truncateWidth(projectAccountUserName(svc, acc), nameWidth)
	return vendor + columnSeparator + name
}

// projectAccountWidths returns the vendor and user-name widths used by the
// Projects screen's granted-accounts list and grant form. They use store-wide
// values, including the email fallback for accounts without a user record,
// and the shared project-screen caps.
func projectAccountWidths(svc *service.Service) (int, int) {
	var vendorWidth, nameWidth int
	for _, vendor := range svc.Vendors() {
		vendorWidth = max(vendorWidth, plainWidth(vendor.Name()))
	}
	vendorWidth = min(vendorWidth, vendorColumnWidthCap)

	for _, u := range svc.Users() {
		nameWidth = max(nameWidth, plainWidth(u.Name()))
	}
	for _, acc := range svc.Accounts() {
		if svc.User(acc.UserEmail()) == nil {
			nameWidth = max(nameWidth, plainWidth(acc.UserEmail()))
		}
	}
	nameWidth = min(nameWidth, nameColumnWidthCap)
	return vendorWidth, nameWidth
}

// sortProjectAccounts orders accounts alphabetically by vendor, then by
// user name, using a case-insensitive comparison. Equal pairs keep their
// incoming order.
func sortProjectAccounts(svc *service.Service, accounts []*model.Account) {
	type key struct {
		vendor string
		user   string
	}
	keys := make(map[model.AccountKey]key, len(accounts))
	for _, acc := range accounts {
		keys[acc.Key()] = key{
			vendor: strings.ToLower(acc.VendorName()),
			user:   strings.ToLower(projectAccountUserName(svc, acc)),
		}
	}
	slices.SortStableFunc(accounts, func(a, b *model.Account) int {
		ka, kb := keys[a.Key()], keys[b.Key()]
		if c := strings.Compare(ka.vendor, kb.vendor); c != 0 {
			return c
		}
		return strings.Compare(ka.user, kb.user)
	})
}

// projectItemText formats a project row for the list.
func projectItemText(p *model.Project) string {
	return insetListRow(tview.Escape(p.Name()))
}
