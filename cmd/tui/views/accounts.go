package views

import (
	"fmt"
	"sort"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// AccountsPanel is the left panel that lists all accounts with their
// rate-limit status. Selecting an account notifies the detail panel via
// the onSelect callback wired by the host App.
type AccountsPanel struct {
	*tview.List
	app        App
	accounts   []*model.Account // mirrors list order; rebuilt on every Refresh
	onSelect   func(*model.Account)
	refreshing bool // suppresses onSelect during Refresh
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

	list.SetFocusFunc(func() {
		app.SetHints(HintsAccountsPanel)
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

// Refresh repopulates the list from the current service state.
// The previously selected account is re-selected by identity (its
// AccountKey) if it still exists; otherwise the first item is selected.
// Callers must not hold the tview draw lock when calling Refresh.
func (p *AccountsPanel) Refresh() {
	all := p.app.Service().Accounts()
	sortAccountsByAvailability(all)

	// Remember current selection by identity before clearing.
	var prevKey model.AccountKey
	if a := p.Selected(); a != nil {
		prevKey = a.Key()
	}

	p.refreshing = true
	p.accounts = all
	p.Clear()
	for _, a := range p.accounts {
		p.AddItem(accountItemText(a), "", 0, nil)
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
	if event.Key() != tcell.KeyRune {
		return event
	}

	switch event.Rune() {
	case KeyNew:
		return p.handleNew()
	case KeyEdit:
		return p.handleEdit()
	case KeyDelete:
		return p.handleDelete()
	case KeyProviders:
		p.app.SwitchToProviders()
		return nil
	case KeyProjects:
		p.app.SwitchToProjects()
		return nil
	case KeyHelp:
		return showHelp(p.app, HintsAccountsPanel)
	case KeyQuit:
		return confirmQuit(p.app, HintsAccountsPanel)
	}
	return event
}

func (p *AccountsPanel) handleNew() *tcell.EventKey {
	modal := NewAccountForm(p.app, nil, func() {
		p.app.SetHints(HintsAccountsPanel)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsForm)
	return nil
}

func (p *AccountsPanel) handleEdit() *tcell.EventKey {
	a := p.Selected()
	if a == nil {
		return nil
	}
	modal := NewAccountForm(p.app, a, func() {
		p.app.SetHints(HintsAccountsPanel)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsForm)
	return nil
}

func (p *AccountsPanel) handleDelete() *tcell.EventKey {
	a := p.Selected()
	if a == nil {
		return nil
	}
	email := a.UserEmail()
	provider := a.ProviderName()
	label := fmt.Sprintf("%s / %s", email, provider)
	modal := NewConfirmModal(
		fmt.Sprintf("Remove account\n%s?\nAll its sessions and project grants must be removed first.", label),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.DeleteAccount(email, provider)
			}); err != nil {
				p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			}
			p.app.SetHints(HintsAccountsPanel)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(HintsAccountsPanel)
			p.app.HideModal()
		},
	)
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsConfirmModal)
	return nil
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// accountItemText formats an account as a single line for the list.
// Format: "email / provider" with the pause time appended when set.
// A past PauseUntil is shown in red; a future one in the default colour.
func accountItemText(a *model.Account) string {
	base := fmt.Sprintf("%s / %s", a.UserEmail(), a.ProviderName())
	if a.PauseUntil().IsZero() {
		return base
	}
	timeStr, isPast := formatPauseUntil(a.PauseUntil())
	if isPast {
		return fmt.Sprintf("%-40s [red]%s[-]", base, timeStr)
	}
	return fmt.Sprintf("%-40s %s", base, timeStr)
}

// formatPauseUntil formats a PauseUntil timestamp for inline display.
// Returns the formatted string and whether the time is in the past.
// Same-day times are shown as "HH:MM"; other days as "MM-DD HH:MM".
func formatPauseUntil(t time.Time) (string, bool) {
	now := time.Now()
	isPast := t.Before(now)
	y, m, d := t.Date()
	ny, nm, nd := now.Date()
	if y == ny && m == nm && d == nd {
		return t.Format("15:04"), isPast
	}
	return t.Format("01-02 15:04"), isPast
}

// sortAccountsByAvailability sorts accounts so that available accounts (no
// pause, or pause in the past) appear first, followed by paused accounts
// ordered by soonest available time.
func sortAccountsByAvailability(accounts []*model.Account) {
	now := time.Now()
	sort.SliceStable(accounts, func(i, j int) bool {
		ti := accounts[i].PauseUntil()
		tj := accounts[j].PauseUntil()
		iAvail := ti.IsZero() || ti.Before(now)
		jAvail := tj.IsZero() || tj.Before(now)
		if iAvail != jAvail {
			return iAvail
		}
		if ti.IsZero() || iAvail {
			return false // stable among available
		}
		return ti.Before(tj)
	})
}
