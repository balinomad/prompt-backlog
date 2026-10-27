// Package main implements the tui application.
package main

import (
	"context"
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

const savedStatusMessage = "[green]saved ✓[-]"

// pauseStatusRefreshInterval is how often the accounts list re-evaluates
// pause-time status so an account's colour flips without a mutation.
const pauseStatusRefreshInterval = time.Minute

const numericKeypadMode = "\x1b>"

// minTerminalHeight is the smallest supported terminal height.
const minTerminalHeight = 24

// minTerminalWidth is the smallest supported terminal width. It covers the
// longest hint line or the longest common hint plus an inline status message.
var minTerminalWidth = max(80, views.MinimumWidth(tview.TaggedStringWidth(savedStatusMessage)))

const (
	// statusBarHeight is the base outer height reserved below the content.
	statusBarHeight = views.StatusBarBaseHeight
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
	root      *tview.Flex
	mainFlex  *tview.Flex
	mainTop   *tview.Flex
	prevFocus tview.Primitive // saved by ShowModal, restored by HideModal
	tooSmall  bool

	statusBar   *views.StatusBar
	accounts    *views.AccountsPanel
	detail      *views.DetailPanel
	accountInfo *views.AccountInfoPanel
	vendors     *views.VendorsPanel
	projects    *views.ProjectsPanel
	users       *views.UsersPanel
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

// Run starts the tview event loop and blocks until the application stops or
// ctx is cancelled. A background ticker refreshes pause-time status.
func (a *App) Run(ctx context.Context) error {
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
				a.SetError(fmt.Sprintf("numeric keypad: %v", err))
			}
		})
		// Stop here as well in case ctx was cancelled before Run created the
		// screen; Stop is a no-op until a screen exists.
		if ctx.Err() != nil {
			go a.tv.Stop()
		}
	})

	go func() {
		select {
		case <-ctx.Done():
			a.tv.Stop()
		case <-done:
		}
	}()

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
	a.SetStatus(savedStatusMessage)
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

// SwitchToVendors refreshes and shows the vendors management screen.
func (a *App) SwitchToVendors() {
	a.switchTo("vendors", a.vendors.Refresh, a.vendors)
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

// SwitchToUsers refreshes and shows the users management screen.
func (a *App) SwitchToUsers() {
	a.switchTo("users", a.users.Refresh, a.users)
}

func (a *App) switchTo(page string, refresh func(), focus tview.Primitive) {
	refresh()
	a.pages.SwitchToPage(page)
	a.tv.SetFocus(focus)
}

// SetStatus displays an inline status message in the status bar. It clears
// any notice or error. An empty string clears it.
// Callers may include tview colour tags (e.g. "[green]saved[-]").
func (a *App) SetStatus(msg string) {
	a.statusBar.SetStatus(msg)
}

// SetNotice displays msg in the separate notice area below the common hints.
// It clears any inline status and error. An empty string clears it.
func (a *App) SetNotice(msg string) {
	a.statusBar.SetNotice(msg)
}

// SetError displays msg in the separate red error area below the common
// hints. It clears any inline status and notice. An empty string clears it.
func (a *App) SetError(msg string) {
	a.statusBar.SetError(msg)
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
	a.vendors = views.NewVendorsPanel(a)
	a.projects = views.NewProjectsPanel(a)
	a.users = views.NewUsersPanel(a)

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
		AddPage("vendors", a.vendors, true, false).
		AddPage("projects", a.projects, true, false).
		AddPage("users", a.users, true, false)

	// Root layout: content area above, status bar pinned to the bottom.
	a.root = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.statusBar, statusBarHeight, 0, false)

	a.tv.SetRoot(a.root, true).EnableMouse(true)
	a.tv.SetInputCapture(a.handleGlobalKeys)
	a.tv.SetMouseCapture(a.handleGlobalMouse)
	a.tv.SetBeforeDrawFunc(a.beforeDraw)

	// Initial data load.
	a.refreshAll()
	a.tv.SetFocus(a.accounts)

	// First-run: no vendors means nothing can be done yet.
	// Send the operator to the vendors screen with a helpful prompt.
	if len(a.svc.Vendors()) == 0 {
		a.SwitchToVendors()
		a.SetNotice("No vendors configured. Press Ins to add one.")
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
	a.vendors.Refresh()
	a.projects.Refresh()
	a.users.Refresh()
}

// refreshAccounts repopulates the accounts panel and sizes it to fit the
// widest rendered row.
func (a *App) refreshAccounts() {
	a.accounts.Refresh()
	a.mainTop.ResizeItem(a.accounts, a.accounts.RequiredWidth(), 0)
}

// resizeDetailPanels sizes every bottom detail panel to the height its
// current text needs.
func (a *App) resizeDetailPanels(screenWidth, screenHeight, statusHeight int) {
	a.resizeAccountInfo(screenWidth, screenHeight, statusHeight)

	if a.detail != nil {
		if _, _, width, _ := a.detail.GetInnerRect(); width > 0 {
			a.detail.SetWidth(width)
		}
	}

	available := screenHeight - statusHeight
	if available <= 0 {
		return
	}
	if a.vendors != nil {
		a.vendors.ResizeInfo(screenWidth, available)
	}
	if a.projects != nil {
		a.projects.ResizeInfo(screenWidth, available)
	}
	if a.users != nil {
		a.users.ResizeInfo(screenWidth, available)
	}
}

// resizeAccountInfo sizes the full-width account panel to the height its
// current text needs at screenWidth. The top row keeps at least
// minMainTopHeight rows so Accounts and Sessions remain usable on short
// terminals.
func (a *App) resizeAccountInfo(screenWidth, screenHeight, statusHeight int) {
	if a.mainFlex == nil || a.accountInfo == nil {
		return
	}
	available := screenHeight - statusHeight
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

// beforeDraw replaces the UI with a resize message when the terminal is too
// small, and otherwise resizes the detail panels.
func (a *App) beforeDraw(screen tcell.Screen) bool {
	width, height := screen.Size()
	if terminalTooSmall(width, height) {
		a.tooSmall = true
		drawTerminalTooSmall(screen, width, height)
		return true
	}
	a.tooSmall = false
	statusHeight := a.statusBar.Layout(width)
	a.root.ResizeItem(a.statusBar, statusHeight, 0)
	a.resizeDetailPanels(width, height, statusHeight)
	return false
}

func terminalTooSmall(width, height int) bool {
	return width < minTerminalWidth || height < minTerminalHeight
}

// drawTerminalTooSmall renders the minimum-size message and hides the cursor.
func drawTerminalTooSmall(screen tcell.Screen, width, height int) {
	lines := []string{
		"Terminal too small",
		"",
		fmt.Sprintf("Minimum: %dx%d", minTerminalWidth, minTerminalHeight),
		fmt.Sprintf("Current: %dx%d", width, height),
		"Resize to continue.",
	}
	startY := max((height-len(lines))/2, 0)
	for i, line := range lines {
		tview.Print(screen, line, 0, startY+i, width, tview.AlignCenter, tview.Styles.PrimaryTextColor)
	}
	screen.HideCursor()
}

// handleGlobalKeys handles focus switching on Tab/Shift+Tab and screen
// switching on the vendor, project, and user keys. Focus switching applies
// only to the main page; screen switching applies on every page except
// "modal", so a form or editor keeps its own rune input. All input is blocked
// while the terminal is too small.
func (a *App) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
	if a.tooSmall {
		if event.Key() == tcell.KeyCtrlC {
			return event
		}
		return nil
	}

	if event.Key() == views.KeyFocusNext || event.Key() == views.KeyFocusPrev {
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

	if event.Key() != tcell.KeyRune {
		return event
	}
	if name, _ := a.pages.GetFrontPage(); name == "modal" {
		return event
	}

	switch event.Rune() {
	case views.KeyVendors:
		a.SwitchToVendors()
		return nil
	case views.KeyProjects:
		a.SwitchToProjects()
		return nil
	case views.KeyUsers:
		a.SwitchToUsers()
		return nil
	}
	return event
}

// handleGlobalMouse blocks mouse input while the terminal is too small.
func (a *App) handleGlobalMouse(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if a.tooSmall {
		return nil, action
	}
	return event, action
}
