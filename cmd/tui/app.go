// Package main implements the tui application.
package main

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/apperr"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// AppName is the application's display name, used in dialog titles and
// CLI output.
const AppName = "llm-desk"

// pauseStatusRefreshInterval is how often the accounts list re-evaluates
// pause-time status so an account's colour flips without a mutation.
const pauseStatusRefreshInterval = time.Minute

const numericKeypadMode = "\x1b>"

const (
	// statusBarHeight is the fixed outer height reserved below the content.
	statusBarHeight = 2
	// minMainTopHeight keeps the Accounts and Sessions panels visible when
	// the terminal is short and the Account panel needs more rows.
	minMainTopHeight = 3
)

// Compile-time check that App implements views.App.
var _ views.App = (*App)(nil)

// App is the concrete host application. It owns the tview event loop and the
// application service layer. All views interact with it exclusively through
// the views.App interface.
type App struct {
	tv        *tview.Application
	svc       *service.Service
	pages     *tview.Pages
	mainFlex  *tview.Flex
	mainTop   *tview.Flex
	prevFocus tview.Primitive // saved by ShowModal, restored by HideModal

	statusBar   *views.StatusBar
	accounts    *views.AccountsPanel
	detail      *views.DetailPanel
	accountInfo *views.AccountInfoPanel
	providers   *views.ProvidersPanel
	projects    *views.ProjectsPanel
}

// New constructs the fully wired application around svc. The caller is
// responsible for constructing svc (e.g. via service.New) and handling any
// load error before calling New.
// Returns an error if svc is nil.
func New(svc *service.Service) (*App, error) {
	if svc == nil {
		return nil, fmt.Errorf("new app: service %w", apperr.ErrNil)
	}
	a := &App{
		tv:  tview.NewApplication(),
		svc: svc,
	}
	a.build()
	return a, nil
}

// Run starts the tview event loop and blocks until the application stops. A
// background ticker refreshes pause-time status.
func (a *App) Run() error {
	ticker := time.NewTicker(pauseStatusRefreshInterval)
	done := make(chan struct{})
	defer func() {
		ticker.Stop()
		close(done)
	}()

	// tcell switches the terminal to application keypad mode at startup. In
	// that mode most terminals send the numeric keypad as SS3 sequences
	// (ESC O p..y) that tcell v2 cannot decode as digits, so numpad digits
	// never reach an input field. Switch back to numeric keypad mode
	// (DECKPNM) once the screen is up. Best effort: a screen without a tty
	// is left untouched.
	var numericKeypad sync.Once
	a.tv.SetAfterDrawFunc(func(screen tcell.Screen) {
		numericKeypad.Do(func() {
			tty, ok := screen.Tty()
			if !ok {
				return
			}
			if _, err := tty.Write([]byte(numericKeypadMode)); err != nil {
				a.statusBar.SetMessage(fmt.Sprintf("[red]numeric keypad: %v[-]", err))
			}
		})
	})

	go func() {
		for {
			select {
			case <-ticker.C:
				a.tv.QueueUpdateDraw(func() {
					a.refreshAccounts()
				})
			case <-done:
				return
			}
		}
	}()

	if err := a.tv.Run(); err != nil {
		return fmt.Errorf("run tview: %w", err)
	}
	return nil
}

// ── views.App implementation ──────────────────────────────────────────────────

// Service returns the current application service layer.
// Views read from this; mutations go through Mutate.
func (a *App) Service() *service.Service {
	return a.svc
}

// AppName returns the application's display name.
func (a *App) AppName() string {
	return AppName
}

// Mutate applies fn to the service. If fn returns nil, the result is
// persisted to disk and all views are refreshed. Views are refreshed
// regardless of whether the save succeeded so the in-memory state is
// always reflected. A save failure is wrapped in views.ErrSaveFailed,
// distinguishing it from fn's own domain error — see the views.App
// interface doc comment.
// Returns an error if fn is nil.
func (a *App) Mutate(fn func(*service.Service) error) error {
	if fn == nil {
		return fmt.Errorf("mutate: fn %w", apperr.ErrNil)
	}
	if err := fn(a.svc); err != nil {
		return err
	}
	saveErr := a.svc.Save()
	a.refreshAll()
	if saveErr != nil {
		return fmt.Errorf("%w: %w", views.ErrSaveFailed, saveErr)
	}
	a.statusBar.SetMessage("[green]saved ✓[-]")
	return nil
}

// ShowModal displays p as a centred transparent overlay. The currently focused
// primitive is saved so HideModal can restore it.
// Returns an error if p is nil, including a non-nil interface wrapping a typed
// nil pointer.
func (a *App) ShowModal(p tview.Primitive) error {
	if isNilPrimitive(p) {
		return fmt.Errorf("show modal: primitive %w", apperr.ErrNil)
	}

	a.prevFocus = a.tv.GetFocus()
	a.pages.AddPage("modal", p, true, true)
	a.tv.SetFocus(p)
	return nil
}

// isNilPrimitive reports whether p is nil, including a non-nil interface
// wrapping a typed nil pointer.
func isNilPrimitive(p tview.Primitive) bool {
	if p == nil {
		return true
	}
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// HideModal removes the modal overlay and restores focus to the primitive
// that was active before ShowModal was called.
func (a *App) HideModal() {
	a.pages.RemovePage("modal")
	if a.prevFocus != nil {
		a.tv.SetFocus(a.prevFocus)
		a.prevFocus = nil
	}
}

// SetFocus moves keyboard focus to p.
func (a *App) SetFocus(p tview.Primitive) {
	a.tv.SetFocus(p)
}

// SwitchToProviders refreshes and shows the providers management screen.
func (a *App) SwitchToProviders() {
	a.switchTo("providers", a.providers.Refresh, a.providers)
}

// SwitchToMain shows the main accounts, sessions, and account-info screen
// and returns focus to the accounts list.
func (a *App) SwitchToMain() {
	a.pages.SwitchToPage("main")
	a.tv.SetFocus(a.accounts)
}

// SwitchToProjects refreshes and shows the projects management screen.
func (a *App) SwitchToProjects() {
	a.switchTo("projects", a.projects.Refresh, a.projects)
}

func (a *App) switchTo(page string, refresh func(), focus tview.Primitive) {
	refresh()
	a.pages.SwitchToPage(page)
	a.tv.SetFocus(focus)
}

// SetStatus displays msg in the right section of the status bar.
// An empty string clears any previous message.
// Callers may include tview colour tags (e.g. "[red]error[-]").
func (a *App) SetStatus(msg string) {
	a.statusBar.SetMessage(msg)
}

// SetHints replaces the keyboard-hint lines in the status bar.
func (a *App) SetHints(specific, common string) {
	a.statusBar.SetHints(specific, common)
}

// Quit stops the tview event loop.
func (a *App) Quit() {
	a.tv.Stop()
}

// ── Private ───────────────────────────────────────────────────────────────────

// build constructs the full UI, wires panels together, populates data, and
// handles first-run state. Called exactly once from New.
func (a *App) build() {
	a.statusBar = views.NewStatusBar()
	a.accounts = views.NewAccountsPanel(a)
	a.detail = views.NewDetailPanel(a)
	a.accountInfo = views.NewAccountInfoPanel(a)
	a.providers = views.NewProvidersPanel(a)
	a.projects = views.NewProjectsPanel(a)

	// Wire account-list selection changes to both account views.
	a.accounts.SetOnSelect(func(acc *model.Account) {
		a.detail.SetAccount(acc)
		a.accountInfo.SetAccount(acc)
	})

	// Top row: accounts panel (sized to its widest row) beside sessions.
	a.mainTop = tview.NewFlex().
		AddItem(a.accounts, 0, 1, true).
		AddItem(a.detail, 0, 2, false)

	// Main page: top row above a full-width account panel.
	a.mainFlex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.mainTop, 0, 1, true).
		AddItem(a.accountInfo, 5, 0, false)

	// Pages for full-screen switching.
	// The "modal" page is added/removed dynamically by ShowModal/HideModal.
	a.pages = tview.NewPages().
		AddPage("main", a.mainFlex, true, true).
		AddPage("providers", a.providers, true, false).
		AddPage("projects", a.projects, true, false)

	// Root layout: content area above, status bar pinned to the bottom.
	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.statusBar, statusBarHeight, 0, false)

	a.tv.SetRoot(root, true).EnableMouse(true)
	a.tv.SetInputCapture(a.handleGlobalKeys)
	a.tv.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, height := screen.Size()
		a.resizeDetailPanels(width, height)
		return false
	})

	// Initial data load.
	a.refreshAll()
	a.tv.SetFocus(a.accounts)

	// First-run: no providers means nothing can be done yet.
	// Send the operator to the providers screen with a helpful prompt.
	if len(a.svc.Providers()) == 0 {
		a.SwitchToProviders()
		a.statusBar.SetMessage(
			"No providers configured — press [::b]Ins[-::-] to add one.")
	}
}

// refreshAll repopulates every panel from the current in-memory registry.
// Called after every successful mutation and when a modal is hidden.
func (a *App) refreshAll() {
	// The detail and account-info panels are refreshed indirectly:
	// accounts.Refresh() always invokes the onSelect callback wired in
	// build(), which calls both SetAccount methods. Explicit calls here
	// would run them a second time for no benefit.
	a.refreshAccounts()
	a.providers.Refresh()
	a.projects.Refresh()
}

// refreshAccounts repopulates the accounts panel and sizes it to fit the
// widest rendered row.
func (a *App) refreshAccounts() {
	a.accounts.Refresh()
	a.mainTop.ResizeItem(a.accounts, a.accounts.RequiredWidth(), 0)
}

// resizeDetailPanels sizes every bottom detail panel to the height its
// current text needs.
func (a *App) resizeDetailPanels(screenWidth, screenHeight int) {
	a.resizeAccountInfo(screenWidth, screenHeight)

	available := screenHeight - statusBarHeight
	if available <= 0 {
		return
	}
	if a.providers != nil {
		a.providers.ResizeInfo(screenWidth, available)
	}
	if a.projects != nil {
		a.projects.ResizeInfo(screenWidth, available)
	}
}

// resizeAccountInfo sizes the full-width account panel to the height its
// current text needs at screenWidth. The top row keeps at least
// minMainTopHeight rows so Accounts and Sessions remain usable on short
// terminals.
func (a *App) resizeAccountInfo(screenWidth, screenHeight int) {
	if a.mainFlex == nil || a.accountInfo == nil {
		return
	}
	available := screenHeight - statusBarHeight
	if available <= 0 {
		return
	}
	maxInfoHeight := available - minMainTopHeight
	if maxInfoHeight < 1 {
		a.mainFlex.ResizeItem(a.accountInfo, 0, 0)
		return
	}
	infoHeight := min(a.accountInfo.RequiredHeight(screenWidth), maxInfoHeight)
	a.mainFlex.ResizeItem(a.accountInfo, infoHeight, 0)
}

// handleGlobalKeys toggles focus between the accounts and detail panels on
// Tab/Shift+Tab. It is a no-op whenever the front page isn't "main" —
// including while a modal is shown, since ShowModal's AddPage("modal", ...)
// becomes the front page — so it never intercepts a Form's own Tab-based
// field navigation.
func (a *App) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != views.KeyFocusNext && event.Key() != views.KeyFocusPrev {
		return event
	}
	if name, _ := a.pages.GetFrontPage(); name != "main" {
		return event
	}
	if a.tv.GetFocus() == a.accounts {
		a.tv.SetFocus(a.detail)
	} else {
		a.tv.SetFocus(a.accounts)
	}
	return nil
}
