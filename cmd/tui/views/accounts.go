package views

import (
	"fmt"
	"slices"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// minAccountsPanelWidth is wide enough to show the list title and border.
const minAccountsPanelWidth = len(" Accounts ") + 2

// vendorColumnWidthCap and nameColumnWidthCap bound the account columns so
// a single unusually long value cannot push later columns off screen.
const (
	vendorColumnWidthCap = 12
	nameColumnWidthCap   = 20
)

// accountRow holds the display cells for one account row.
type accountRow struct {
	vendor  string
	name    string
	pause   string
	project string
}

// AccountsPanel is the left panel that lists all accounts with their
// rate-limit status. Selecting an account notifies the detail panel via
// the onSelect callback wired by the host App.
type AccountsPanel struct {
	*tview.List
	app         App
	accounts    []*model.Account // mirrors list order; rebuilt on every Refresh
	onSelect    func(*model.Account)
	refreshing  bool // suppresses onSelect during Refresh
	maxRowWidth int  // widest rendered row, excluding the border
}

// NewAccountsPanel constructs the accounts list. Call SetOnSelect to wire
// the detail panel before the first Refresh.
func NewAccountsPanel(app App) *AccountsPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Accounts ")

	p := &AccountsPanel{
		List: list,
		app:  app,
	}

	// Navigation changes: notify the detail panel unless we are mid-refresh.
	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if p.refreshing {
			return
		}
		if p.onSelect != nil && index >= 0 && index < len(p.accounts) {
			p.onSelect(p.accounts[index])
		}
	})

	wirePanelFocus(list, func() {
		app.SetHints(HintsAccountsPanelSpecific, HintsAccountsPanelCommon)
	})

	list.SetInputCapture(p.handleKey)

	return p
}

// SetOnSelect registers a callback that fires whenever the selected account
// changes. Called by the host App to wire this panel to the detail panel.
func (p *AccountsPanel) SetOnSelect(fn func(*model.Account)) {
	p.onSelect = fn
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
	now := time.Now()
	latestProjects := latestSessionProjects(svc.Sessions())
	grantedProjects := grantedProjectNames(svc, all)
	rawRows := make([]accountRow, len(all))
	hasPause := false
	hasProject := false
	for i, a := range all {
		userName := a.UserEmail()
		if u := svc.User(a.UserEmail()); u != nil {
			userName = u.Name()
		}
		pause := pauseStatusText(a.PauseUntil(), now)
		project := latestGrantedProject(latestProjects, grantedProjects, a.Key())
		hasPause = hasPause || pause != ""
		hasProject = hasProject || project != ""

		rawRows[i] = accountRow{
			vendor:  tview.Escape(truncateWidth(a.VendorName(), vendorColumnWidthCap)),
			name:    tview.Escape(truncateWidth(userName, nameColumnWidthCap)),
			pause:   pause,
			project: tview.Escape(truncateWidth(project, projectColumnWidth)),
		}
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
	lines, width := formatColumns(rows)

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
}

// handleKey processes keyboard input for this panel.
func (p *AccountsPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case KeyInsert:
		return p.handleNew()
	case KeyEdit:
		return p.handleEdit()
	case KeyDelete:
		return p.handleDelete()
	default:
	}

	if result, ok := dispatchPanelKey(event, p.app, HintsAccountsPanelSpecific, HintsAccountsPanelCommon); ok {
		return result
	}

	return event
}

func (p *AccountsPanel) handleNew() *tcell.EventKey {
	modal := NewAccountForm(p.app, nil, func() {
		p.app.SetHints(HintsAccountsPanelSpecific, HintsAccountsPanelCommon)
		p.app.HideModal()
	})
	showModal(p.app, modal, HintsFormSpecific, HintsFormCommon)
	return nil
}

func (p *AccountsPanel) handleEdit() *tcell.EventKey {
	a := p.Selected()
	if a == nil {
		return nil
	}
	modal := NewAccountForm(p.app, a, func() {
		p.app.SetHints(HintsAccountsPanelSpecific, HintsAccountsPanelCommon)
		p.app.HideModal()
	})
	showModal(p.app, modal, HintsFormSpecific, HintsFormCommon)
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
	modal := NewConfirmModal(
		fmt.Sprintf("Remove account\n%s?\nAll its sessions and project grants must be removed first.", label),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.DeleteAccount(email, vendor)
			}); err != nil {
				setErrorStatus(p.app, err)
			}
			p.app.SetHints(HintsAccountsPanelSpecific, HintsAccountsPanelCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(HintsAccountsPanelSpecific, HintsAccountsPanelCommon)
			p.app.HideModal()
		},
	)
	showModal(p.app, modal, HintsConfirmModalSpecific, HintsConfirmModalCommon)
	return nil
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// latestSessionProjects returns the latest session's project name per account.
// sessions must be ordered by ID ascending.
func latestSessionProjects(sessions []*model.Session) map[model.AccountKey]string {
	latest := make(map[model.AccountKey]string, len(sessions))
	for _, s := range sessions {
		latest[s.AccountKey()] = s.ProjectName()
	}
	return latest
}

// latestGrantedProject returns the latest session's project for key when that
// project is still granted to the account, or "" otherwise.
func latestGrantedProject(latest map[model.AccountKey]string, granted map[model.AccountKey]map[string]struct{}, key model.AccountKey) string {
	name := latest[key]
	if name == "" {
		return ""
	}
	if _, ok := granted[key][name]; !ok {
		return ""
	}
	return name
}

// grantedProjectNames returns the granted project names per account.
func grantedProjectNames(svc *service.Service, accounts []*model.Account) map[model.AccountKey]map[string]struct{} {
	granted := make(map[model.AccountKey]map[string]struct{}, len(accounts))
	for _, a := range accounts {
		projects, err := svc.AccountProjects(a.UserEmail(), a.VendorName())
		if err != nil {
			continue
		}
		set := make(map[string]struct{}, len(projects))
		for _, p := range projects {
			set[p.Name()] = struct{}{}
		}
		granted[a.Key()] = set
	}
	return granted
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
