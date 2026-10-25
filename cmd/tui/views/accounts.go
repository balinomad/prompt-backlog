package views

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// minAccountsPanelWidth is wide enough to show the list title and border.
const minAccountsPanelWidth = len(" Accounts ") + 2

// providerColumnWidthCap and nameColumnWidthCap bound the account columns so
// a single unusually long value cannot push later columns off screen.
const (
	providerColumnWidthCap = 12
	nameColumnWidthCap     = 20
)

// accountRow holds the display cells for one account row.
type accountRow struct {
	provider string
	name     string
	pause    string
	project  string
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

	// Resolve only the values rendered in the list. Provider, user, and
	// project values are capped with an ellipsis, while pause status keeps its
	// full rendered text, including day offsets such as "12:34 -5". Optional
	// columns are included only when at least one rendered row has a value.
	now := time.Now()
	rawRows := make([]accountRow, len(all))
	hasPause := false
	hasProject := false
	for i, a := range all {
		userName := a.UserEmail()
		if u := svc.User(a.UserEmail()); u != nil {
			userName = u.Name()
		}
		pause := pauseStatusText(a.PauseUntil(), now)
		project := latestGrantedProject(svc, a)
		hasPause = hasPause || pause != ""
		hasProject = hasProject || project != ""

		rawRows[i] = accountRow{
			provider: tview.Escape(truncateWidth(a.ProviderName(), providerColumnWidthCap)),
			name:     tview.Escape(truncateWidth(userName, nameColumnWidthCap)),
			pause:    pause,
			project:  tview.Escape(truncateWidth(project, projectColumnWidth)),
		}
	}

	rows := make([][]string, len(rawRows))
	for i, row := range rawRows {
		rows[i] = []string{row.provider, row.name}
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
		// Every other special key falls through to dispatchPanelKey and
		// the rune switch below.
	}

	if result, ok := dispatchPanelKey(event, p.app, HintsAccountsPanelSpecific, HintsAccountsPanelCommon); ok {
		return result
	}

	switch event.Rune() {
	case KeyProviders:
		p.app.SwitchToProviders()
		return nil
	case KeyProjects:
		p.app.SwitchToProjects()
		return nil
	}
	return event
}

func (p *AccountsPanel) handleNew() *tcell.EventKey {
	modal := NewAccountForm(p.app, nil, func() {
		p.app.SetHints(HintsAccountsPanelSpecific, HintsAccountsPanelCommon)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsFormSpecific, HintsFormCommon)
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
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsFormSpecific, HintsFormCommon)
	return nil
}

func (p *AccountsPanel) handleDelete() *tcell.EventKey {
	a := p.Selected()
	if a == nil {
		return nil
	}
	email := a.UserEmail()
	provider := a.ProviderName()
	label := fmt.Sprintf("%s / %s", tview.Escape(email), tview.Escape(provider))
	modal := NewConfirmModal(
		fmt.Sprintf("Remove account\n%s?\nAll its sessions and project grants must be removed first.", label),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.DeleteAccount(email, provider)
			}); err != nil {
				p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			}
			p.app.SetHints(HintsAccountsPanelSpecific, HintsAccountsPanelCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(HintsAccountsPanelSpecific, HintsAccountsPanelCommon)
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

// ── Package-level helpers ─────────────────────────────────────────────────────

// latestGrantedProject returns the project name of the account's latest
// session when that project is still granted to the account, or "" otherwise.
// Resolution errors degrade to "", matching the detail panel's passive reads.
func latestGrantedProject(svc *service.Service, a *model.Account) string {
	sessions, err := svc.AccountSessions(a.UserEmail(), a.ProviderName())
	if err != nil {
		return ""
	}
	projectName := latestSessionProjectName(sessions)
	if projectName == "" {
		return ""
	}
	projects, err := svc.AccountProjects(a.UserEmail(), a.ProviderName())
	if err != nil {
		return ""
	}
	if slices.ContainsFunc(projects, func(p *model.Project) bool {
		return p.Name() == projectName
	}) {
		return projectName
	}
	return ""
}

// pauseStatusText is the single, shared rendering of an account's pause
// status: "" if until is the zero value (never paused); otherwise
// formatPauseUntil's text, coloured red while still in the future (not
// yet available) or green once past (available again). Converts until
// and now to local time before comparing or formatting: PauseUntil is
// stored in UTC (see model.ParseTime) so it survives persistence
// correctly, but the same-day / different-day split and the day count
// must follow the operator's own calendar, that's what they're actually
// looking at, not UTC's. Every place pause status is shown calls this,
// so they cannot drift from each other, or from this rule.
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

// formatPauseUntil formats a PauseUntil timestamp for display, relative
// to now, and reports whether it has already passed. Same-day times show
// as "HH:MM"; other days as "HH:MM ±N", where N is the number of
// calendar days between the two dates, not a rounded 24-hour count, so a
// same-day comparison near midnight, or a day count spanning a DST
// change, isn't thrown off by either. now is a parameter, not an
// internal time.Now() call, so the comparison is deterministic — see
// TestFormatPauseUntil. Callers needing the same-day split and day count
// to follow the operator's own calendar rather than whatever zone t and
// now happen to carry must convert to local time first — see
// pauseStatusText, the one production caller, which does this.
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

// sortAccountsByAvailability sorts accounts by PauseUntil ascending: never
// paused (zero value) or already past always sorts before any future
// pause, and accounts stay ordered by that same value throughout — not
// just grouped at the available/paused boundary, which previously left
// available accounts in whatever order Accounts() happened to hand them
// instead of by time. Ties (e.g. two never-paused accounts) keep their
// incoming order via the stable sort.
func sortAccountsByAvailability(accounts []*model.Account) {
	sort.SliceStable(accounts, func(i, j int) bool {
		return accounts[i].PauseUntil().Before(accounts[j].PauseUntil())
	})
}
