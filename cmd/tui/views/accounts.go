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

// providerColumnWidthCap and nameColumnWidthCap bound the padded columns so a
// single unusually long value cannot push the pause and project columns off
// screen.
const (
	providerColumnWidthCap = 12
	nameColumnWidthCap     = 20
)

// AccountsPanel is the left panel that lists all accounts with their
// rate-limit status. Selecting an account notifies the detail panel via
// the onSelect callback wired by the host App.
type AccountsPanel struct {
	*tview.List
	app         App
	accounts    []*model.Account // mirrors list order; rebuilt on every Refresh
	onSelect    func(*model.Account)
	refreshing  bool // suppresses onSelect during Refresh
	maxRowWidth int  // widest rendered or reserved row, excluding the border
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

	// Resolve each account's user name, latest project, and the display
	// widths. Provider, user, and project widths come from the longest
	// store-wide values, so a long value that is configured but not currently
	// in a row is still accounted for. They are capped so one unusually long
	// value cannot push the pause column off screen. The pause width is
	// measured from the full rendered status, including day offsets such as
	// "12:34 -5".
	now := time.Now()
	providerWidth := 0
	for _, provider := range svc.Providers() {
		providerWidth = max(providerWidth, len([]rune(provider.Name())))
	}
	providerWidth = min(providerWidth, providerColumnWidthCap)

	nameWidth := 0
	for _, u := range svc.Users() {
		nameWidth = max(nameWidth, len([]rune(u.Name())))
	}

	names := make([]string, len(all))
	latestProjects := make([]string, len(all))
	var pauseWidth int
	for i, a := range all {
		names[i] = a.UserEmail()
		if u := svc.User(a.UserEmail()); u != nil {
			names[i] = u.Name()
		} else {
			nameWidth = max(nameWidth, len([]rune(names[i])))
		}
		latestProjects[i] = latestGrantedProject(svc, a)
		pauseWidth = max(pauseWidth, tview.TaggedStringWidth(pauseStatusText(a.PauseUntil(), now)))
	}
	nameWidth = min(nameWidth, nameColumnWidthCap)

	projectWidth := 0
	for _, project := range svc.Projects() {
		projectWidth = max(projectWidth, len([]rune(project.Name())))
	}
	projectWidth = min(projectWidth, projectColumnWidth)

	p.refreshing = true
	p.accounts = all
	p.Clear()
	p.maxRowWidth = 0
	for i, a := range p.accounts {
		row := accountItemText(a, names[i], latestProjects[i], providerWidth, nameWidth, projectWidth, now)
		p.maxRowWidth = max(p.maxRowWidth, tview.TaggedStringWidth(row))
		p.AddItem(row, "", 0, nil)
	}
	p.refreshing = false

	// Reserve the project column even when no current row has a project, so a
	// later project assignment does not force a resize to avoid clipping.
	if len(all) > 0 && projectWidth > 0 {
		reserved := providerWidth + 2 + nameWidth + 2 + pauseWidth + 2 + projectWidth
		p.maxRowWidth = max(p.maxRowWidth, reserved)
	}

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
	case KeyEdit:
		return p.handleEdit()
	case KeyDelete:
		return p.handleDelete()
	case KeyRenameUser:
		return p.handleRenameUser()
	default:
		// Every other special key falls through to dispatchPanelKey and
		// the rune switch below.
	}

	if result, ok := dispatchPanelKey(event, p.app, HintsAccountsPanelSpecific, HintsAccountsPanelCommon); ok {
		return result
	}

	switch event.Rune() {
	case KeyNew:
		return p.handleNew()
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

func (p *AccountsPanel) handleRenameUser() *tcell.EventKey {
	a := p.Selected()
	if a == nil {
		return nil
	}
	modal := NewUserForm(p.app, a, func() {
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

// accountItemText formats one account as a row: provider, user name,
// pause status, then the latest project when there is one. Provider and
// user name are padded to providerWidth/nameWidth, and the project is
// truncated to projectWidth; the caller computes those once per batch from
// the longest store-wide values, capped so one unusually long value cannot
// push the pause column off screen. now is shared with the caller's width
// measurement so the row and its measured width use the same pause status.
// userName is resolved by the caller; a missing user record is the caller's
// own fallback to handle, not this function's.
func accountItemText(a *model.Account, userName, latestProject string, providerWidth, nameWidth, projectWidth int, now time.Time) string {
	provider := tview.Escape(truncate(a.ProviderName(), providerWidth))
	name := tview.Escape(truncate(userName, nameWidth))
	pause := pauseStatusText(a.PauseUntil(), now)
	row := fmt.Sprintf("%-*s  %-*s  %s", providerWidth, provider, nameWidth, name, pause)
	if latestProject != "" && projectWidth > 0 {
		row += "  " + tview.Escape(truncate(latestProject, projectWidth))
	}
	return row
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
// so they cannot drift from each other, or from this rule, the way
// accountItemText and DetailPanel.renderInfo once already did.
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
