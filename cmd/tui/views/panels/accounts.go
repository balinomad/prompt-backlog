// Package panels contains the TUI panels and their shared panel scaffold.
package panels

import (
	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"

	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const accountsListTitle = " Accounts "

// minAccountsPanelWidth is wide enough to show the list title and border.
const minAccountsPanelWidth = len(accountsListTitle+"0/0 ") + 2

// accountRow holds the display cells for one account row.
type accountRow struct {
	vendor  string
	name    string
	pause   string
	project string
}

// AccountsPanel is the left panel that lists all accounts with their
// rate-limit status. Selecting an account notifies the detail panel via
// the onSelect callback wired by the host views.Host.
type AccountsPanel struct {
	*tview.List
	app            views.Host
	accounts       []*model.Account // mirrors list order; rebuilt on every Refresh
	onSelect       func(*model.Account)
	onFilterChange func()
	filter         *views.ProjectFilterState
	refreshing     bool // suppresses onSelect during Refresh
	maxRowWidth    int  // widest rendered row, excluding the border
}

// NewAccountsPanel constructs the accounts list. Call SetOnSelect to wire
// the detail panel before the first Refresh.
func NewAccountsPanel(app views.Host) *AccountsPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	views.SetListTitle(list, accountsListTitle, 0, 0)

	p := &AccountsPanel{
		List:   list,
		app:    app,
		filter: views.NewProjectFilterState(),
	}

	// Navigation changes: notify the detail panel unless we are mid-refresh.
	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		views.SetListTitle(list, accountsListTitle, index, len(p.accounts))
		if p.refreshing {
			return
		}
		if p.onSelect != nil && index >= 0 && index < len(p.accounts) {
			p.onSelect(p.accounts[index])
		}
	})

	views.WirePanelFocus(list, func() {
		app.SetHints(views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
	})

	list.SetInputCapture(p.handleKey)
	views.WireListDoubleClick(list, func() { p.handleEdit() })

	return p
}

// SetOnSelect registers a callback that fires whenever the selected account
// changes. Called by the host views.Host to wire this panel to the detail panel.
func (p *AccountsPanel) SetOnSelect(fn func(*model.Account)) {
	p.onSelect = fn
}

// SetFilter shares the main-screen project filter with the panel.
func (p *AccountsPanel) SetFilter(filter *views.ProjectFilterState) {
	if filter == nil {
		filter = views.NewProjectFilterState()
	}
	p.filter = filter
}

// SetOnFilterChange registers a callback invoked after the filter advances.
func (p *AccountsPanel) SetOnFilterChange(fn func()) {
	p.onFilterChange = fn
}

// Selected returns the currently highlighted account, or nil when the list
// is empty.
func (p *AccountsPanel) Selected() *model.Account {
	idx := p.GetCurrentItem()
	if idx < 0 || idx >= len(p.accounts) {
		return nil
	}
	return p.accounts[idx]
}

// RequiredWidth returns the outer width needed to render every row without
// clipping, including the list border.
func (p *AccountsPanel) RequiredWidth() int {
	return max(p.maxRowWidth+2, minAccountsPanelWidth)
}

// Refresh repopulates the list from the current service state.
// The previously selected account is re-selected by identity (its
// AccountKey) if it still exists; otherwise the first item is selected.
// Callers must not hold the tview draw lock when calling Refresh.
func (p *AccountsPanel) Refresh() {
	svc := p.app.Service()
	all := svc.Accounts()
	sortAccountsByAvailability(all)

	// Remember current selection by identity before clearing.
	var prevKey model.AccountKey
	if a := p.Selected(); a != nil {
		prevKey = a.Key()
	}

	// Resolve only the values rendered in the list. Vendor, user, and
	// project values are capped with an ellipsis, while pause status keeps its
	// full rendered text, including day offsets such as "12:34 -5". Optional
	// columns are included only when at least one rendered row has a value.
	rawRows, hasPause, hasProject, err := accountRows(svc, all, time.Now())
	if err != nil {
		p.app.SetError(err.Error())
		return
	}

	rows := make([][]string, len(rawRows))
	for i, row := range rawRows {
		rows[i] = []string{row.vendor, row.name}
		if hasPause {
			rows[i] = append(rows[i], row.pause)
		}
		if hasProject {
			rows[i] = append(rows[i], row.project)
		}
	}
	lines, width := views.FormatColumns(rows)

	p.refreshing = true
	p.accounts = all
	p.Clear()
	p.maxRowWidth = width
	for _, line := range lines {
		p.AddItem(line, "", 0, nil)
	}
	p.refreshing = false

	// Restore selection by identity.
	restored := false
	for i, a := range p.accounts {
		if a.Key() == prevKey {
			p.SetCurrentItem(i)
			restored = true
			break
		}
	}
	if !restored && len(p.accounts) > 0 {
		p.SetCurrentItem(0)
	}

	// Notify detail panel of the current selection after rebuild — even
	// when the list is now empty, so a deleted last account correctly
	// clears the detail panel via SetAccount(nil) instead of leaving it
	// stale. This is now the only trigger for DetailPanel.Refresh(); see
	// app.refreshAll.
	if p.onSelect != nil {
		p.onSelect(p.Selected())
	}

	views.SetListTitle(p.List, accountsListTitle, p.GetCurrentItem(), len(p.accounts))
}

// handleKey processes keyboard input for this panel.
func (p *AccountsPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case views.KeyInsert:
		return p.handleNew()
	case views.KeyEdit:
		return p.handleEdit()
	case views.KeyDelete:
		return p.handleDelete()
	case views.KeyCancel:
		views.ConfirmQuit(p.app, views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
		return nil
	default:
	}

	if result, ok := views.DispatchPanelKey(event, p.app, views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon); ok {
		return result
	}

	if event.Rune() == views.KeyFilter {
		if err := p.filter.Cycle(); err != nil {
			reportPanelError(p.app, err)
			return nil
		}
		if p.onFilterChange != nil {
			p.onFilterChange()
		}
		return nil
	}
	return event
}

func (p *AccountsPanel) handleNew() *tcell.EventKey {
	modal := forms.NewAccountForm(p.app, nil, func() {
		p.app.SetHints(views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
		p.app.HideModal()
	})
	views.ShowModal(p.app, modal, views.HintsFormSpecific, views.HintsFormCommon)
	return nil
}

func (p *AccountsPanel) handleEdit() *tcell.EventKey {
	a := p.Selected()
	if a == nil {
		return nil
	}
	modal := forms.NewAccountForm(p.app, a, func() {
		p.app.SetHints(views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
		p.app.HideModal()
	})
	views.ShowModal(p.app, modal, views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (p *AccountsPanel) handleDelete() *tcell.EventKey {
	a := p.Selected()
	if a == nil {
		return nil
	}
	email := a.UserEmail()
	vendor := a.VendorName()
	label := fmt.Sprintf("%s / %s", tview.Escape(email), tview.Escape(vendor))
	modal := views.NewConfirmModal(
		fmt.Sprintf("Remove account\n%s?\nAll its sessions and project grants must be removed first.", label),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.DeleteAccount(email, vendor)
			}); err != nil {
				views.SetErrorStatus(p.app, err)
			}
			p.app.SetHints(views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
			p.app.HideModal()
		},
	)
	views.ShowModal(p.app, modal, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	return nil
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// accountRows renders the account list cells and reports whether the optional
// pause and project columns are needed.
func accountRows(svc *service.Service, accounts []*model.Account, now time.Time) ([]accountRow, bool, bool, error) {
	grantedProjects, err := grantedProjectIDs(svc, accounts)
	if err != nil {
		return nil, false, false, err
	}
	rows := make([]accountRow, len(accounts))
	hasPause := false
	hasProject := false
	for i, a := range accounts {
		userName, err := views.UserDisplayName(svc, a.UserEmail())
		if err != nil {
			return nil, false, false, fmt.Errorf("account user: %w", err)
		}
		pause := pauseStatusText(a.PauseUntil(), now)
		project, err := latestGrantedProject(svc, grantedProjects, a)
		if err != nil {
			return nil, false, false, err
		}
		hasPause = hasPause || pause != ""
		hasProject = hasProject || project != ""
		rows[i] = accountRow{
			vendor:  tview.Escape(views.TruncateWidth(a.VendorName(), views.VendorColumnWidthCap)),
			name:    tview.Escape(views.TruncateWidth(userName, views.NameColumnWidthCap)),
			pause:   pause,
			project: tview.Escape(views.TruncateWidth(project, views.ProjectColumnWidth)),
		}
	}
	return rows, hasPause, hasProject, nil
}

// latestGrantedProject returns the account's latest session project when that
// project is still granted to the account, or "" otherwise.
func latestGrantedProject(svc *service.Service, granted map[model.AccountKey]map[uint64]struct{}, acc *model.Account) (string, error) {
	latest, ok, err := svc.LatestSession(acc.UserEmail(), acc.VendorName())
	if err != nil {
		return "", fmt.Errorf("latest session for %q/%q: %w", acc.UserEmail(), acc.VendorName(), err)
	}
	if !ok || latest.ProjectID() == 0 {
		return "", nil
	}
	if _, ok := granted[acc.Key()][latest.ProjectID()]; !ok {
		return "", nil
	}
	return latest.ProjectName(), nil
}

// grantedProjectIDs returns the granted projects per account.
func grantedProjectIDs(svc *service.Service, accounts []*model.Account) (map[model.AccountKey]map[uint64]struct{}, error) {
	granted := make(map[model.AccountKey]map[uint64]struct{}, len(accounts))
	for _, a := range accounts {
		projects, err := svc.AccountProjects(a.UserEmail(), a.VendorName())
		if err != nil {
			return nil, fmt.Errorf("account projects for %q/%q: %w", a.UserEmail(), a.VendorName(), err)
		}
		set := make(map[uint64]struct{}, len(projects))
		for _, p := range projects {
			set[p.ID()] = struct{}{}
		}
		granted[a.Key()] = set
	}
	return granted, nil
}

// pauseStatusText renders an account's pause status in local time: "" for a
// zero time, otherwise the formatted pause time coloured red while paused and
// green once available.
func pauseStatusText(until, now time.Time) string {
	if until.IsZero() {
		return ""
	}
	timeStr, isPast := formatPauseUntil(until.Local(), now.Local())
	if isPast {
		return fmt.Sprintf("[green]%s[-]", timeStr)
	}
	return fmt.Sprintf("[red]%s[-]", timeStr)
}

// formatPauseUntil formats t relative to now and reports whether it has
// passed. Same-day times render as "HH:MM"; other days add a signed calendar
// day offset. Callers must pass local times.
func formatPauseUntil(t, now time.Time) (string, bool) {
	isPast := t.Before(now)
	ty, tm, td := t.Date()
	ny, nm, nd := now.Date()
	if ty == ny && tm == nm && td == nd {
		return t.Format("15:04"), isPast
	}
	t0 := time.Date(ty, tm, td, 12, 0, 0, 0, time.UTC)
	n0 := time.Date(ny, nm, nd, 12, 0, 0, 0, time.UTC)
	days := int(t0.Sub(n0).Hours() / 24)
	return fmt.Sprintf("%s %+d", t.Format("15:04"), days), isPast
}

// sortAccountsByAvailability orders accounts by PauseUntil ascending, keeping
// incoming order for ties.
func sortAccountsByAvailability(accounts []*model.Account) {
	slices.SortStableFunc(accounts, func(a, b *model.Account) int {
		return a.PauseUntil().Compare(b.PauseUntil())
	})
}

// ── AccountInfoPanel ──────────────────────────────────────────────────────────

// AccountInfoPanel shows static information for the selected account. It
// spans the full width below the Accounts and Sessions panels.
type AccountInfoPanel struct {
	*tview.TextView
	app     views.Host
	account *model.Account
}

// NewAccountInfoPanel constructs the account information panel. Call
// SetAccount after the accounts panel has provided a selection.
func NewAccountInfoPanel(app views.Host) *AccountInfoPanel {
	info := views.NewInfoView("Account Details")

	return &AccountInfoPanel{
		TextView: info,
		app:      app,
	}
}

// SetAccount sets the account to display and refreshes the panel.
func (p *AccountInfoPanel) SetAccount(acc *model.Account) {
	p.account = acc
	p.renderInfo(acc)
}

// RequiredHeight returns the outer height needed to render the current text
// at outerWidth, including the border and any wrapped lines.
func (p *AccountInfoPanel) RequiredHeight(outerWidth int) int {
	return views.InfoTextHeight(p.GetText(false), outerWidth)
}

func (p *AccountInfoPanel) renderInfo(acc *model.Account) {
	if acc == nil {
		p.SetText("(no account selected)")
		return
	}
	svc := p.app.Service()
	email := acc.UserEmail()
	vendor := acc.VendorName()
	userName, err := views.UserDisplayName(svc, email)
	if err != nil {
		reportPanelError(p.app, err)
		return
	}

	var sb strings.Builder

	projects, err := svc.AccountProjects(email, vendor)
	projectsText := views.DimText("none")
	if err != nil {
		p.app.SetError("Account projects: " + err.Error())
		projectsText = views.DimText("unavailable")
	} else if len(projects) > 0 {
		latestProjectName := ""
		if latest, ok, err := svc.LatestSession(email, vendor); err != nil {
			p.app.SetError("Account details: " + err.Error())
		} else if ok {
			latestProjectName = latest.ProjectName()
		}

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
		projectsText = strings.Join(names, ", ")
	}
	views.WriteInfoRows(&sb, []views.InfoRow{
		{Label: "User", Value: fmt.Sprintf("%s (%s)", tview.Escape(userName), tview.Escape(email))},
		{Label: "Vendor", Value: tview.Escape(vendor)},
		{Label: "Projects", Value: projectsText},
	})

	p.SetText(strings.TrimSuffix(sb.String(), "\n"))
}
