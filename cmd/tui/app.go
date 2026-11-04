// Package main implements the tui application.
package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/cmd/tui/views/panels"
	"github.com/balinomad/llm-desk/internal/apperr"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.design/x/clipboard"
)

// AppName is the application's display name, used in dialog titles and
// CLI output.
const AppName = "llm-desk"

const savedStatusMessage = "[green]saved ✓[-]"

// statusMessageTimeout is how long non-error status and notice messages stay
// visible before they clear automatically.
const statusMessageTimeout = 5 * time.Second

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
	// minMainTopHeight keeps the Accounts, Sessions and Notes panels visible
	// when the terminal is short and the Account panel needs more rows.
	minMainTopHeight = 6
)

// Compile-time check that App implements views.Host.
var _ views.Host = (*App)(nil)

type modalEntry struct {
	page        string
	primitive   tview.Primitive
	rootFocus   tview.Primitive
	resumeFocus tview.Primitive
	prevSpec    string
	prevComm    string
}

// App is the concrete host application. It owns the tview event loop and the
// application service layer. All views interact with it exclusively through
// the views.Host interface.
type App struct {
	tv        *tview.Application
	svc       *service.Service
	pages     *tview.Pages
	root      *tview.Flex
	mainFlex  *tview.Flex
	mainTop   *tview.Flex
	modals    []modalEntry
	nextModal int
	hintsSpec string
	hintsComm string
	tooSmall  bool

	copyFn        func(string) error
	clipboardInit sync.Once
	clipboardErr  error

	statusBar   *views.StatusBar
	accounts    *panels.AccountsPanel
	sessions    *panels.SessionsPanel
	notes       *panels.NotesPanel
	mainRight   *tview.Flex
	accountInfo *panels.AccountInfoPanel
	vendors     *panels.VendorsPanel
	projects    *panels.ProjectsPanel
	users       *panels.UsersPanel

	statusMu         sync.Mutex
	statusTimer      *time.Timer
	statusGeneration uint64
	statusTimeout    time.Duration
	statusQueue      func(func())
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
		tv:            tview.NewApplication(),
		svc:           svc,
		statusTimeout: statusMessageTimeout,
	}
	a.copyFn = a.writeClipboard
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
		a.cancelStatusTimer()
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

// ── views.Host implementation ──────────────────────────────────────────────────

// Service returns the current application service layer.
// Views read from this; mutations go through Mutate.
func (a *App) Service() *service.Service {
	return a.svc
}

// AppName returns the application's display name.
func (a *App) AppName() string {
	return AppName
}

// CopyToClipboard writes text to the system clipboard through the replaceable
// copy function.
func (a *App) CopyToClipboard(text string) error {
	if a.copyFn == nil {
		return fmt.Errorf("copy to clipboard: function %w", apperr.ErrNil)
	}
	if err := a.copyFn(text); err != nil {
		return fmt.Errorf("copy to clipboard: %w", err)
	}
	return nil
}

// writeClipboard initialises the clipboard once and writes text to it.
func (a *App) writeClipboard(text string) error {
	a.clipboardInit.Do(func() {
		a.clipboardErr = clipboard.Init()
	})
	if a.clipboardErr != nil {
		return fmt.Errorf("init clipboard: %w", a.clipboardErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := clipboard.Write(ctx, clipboard.FmtText, []byte(text)); err != nil {
		return fmt.Errorf("write clipboard: %w", err)
	}
	return nil
}

// Mutate applies fn to the service. If fn returns nil, the result is
// persisted to disk and all views are refreshed. Views are refreshed
// regardless of whether the save succeeded so the in-memory state is
// always reflected. A save failure is wrapped in views.ErrSaveFailed,
// distinguishing it from fn's own domain error — see the views.Host
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

// modalMouseGuard prevents mouse events that a modal does not consume from
// reaching the page underneath it.
type modalMouseGuard struct {
	tview.Primitive
}

// MouseHandler forwards mouse events to the modal and consumes any event the
// modal does not handle.
func (g *modalMouseGuard) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if handler := g.Primitive.MouseHandler(); handler != nil {
			consumed, capture := handler(action, event, setFocus)
			if consumed || capture != nil {
				return true, capture
			}
		}
		return true, nil
	}
}

// ShowModal pushes p as a centred transparent overlay above the current
// modal, if any. The current focus and hint lines are saved for HideModal.
// Returns an error if p is nil, including a non-nil interface wrapping a typed
// nil pointer.
func (a *App) ShowModal(p tview.Primitive) error {
	if isNilPrimitive(p) {
		return fmt.Errorf("show modal: primitive %w", apperr.ErrNil)
	}

	focus := a.tv.GetFocus()
	if len(a.modals) > 0 {
		parent := &a.modals[len(a.modals)-1]
		parent.resumeFocus = focus
		if state, ok := parent.primitive.(views.ModalState); ok {
			state.SetModalActive(false)
		}
	}
	entry := modalEntry{
		page:      fmt.Sprintf("modal-%d", a.nextModal),
		primitive: p,
		rootFocus: focus,
		prevSpec:  a.hintsSpec,
		prevComm:  a.hintsComm,
	}
	a.nextModal++
	if state, ok := p.(views.ModalState); ok {
		state.SetModalActive(true)
	}
	a.pages.AddPage(entry.page, &modalMouseGuard{Primitive: p}, true, true)
	a.modals = append(a.modals, entry)
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

// HideModal pops the top modal and restores the parent modal, or the screen
// that was active before the first modal was pushed.
func (a *App) HideModal() {
	if len(a.modals) == 0 {
		return
	}
	top := a.modals[len(a.modals)-1]
	a.modals = a.modals[:len(a.modals)-1]
	a.pages.RemovePage(top.page)
	if len(a.modals) > 0 {
		parent := &a.modals[len(a.modals)-1]
		if state, ok := parent.primitive.(views.ModalState); ok {
			state.SetModalActive(true)
		}
		focus := parent.resumeFocus
		if focus == nil {
			focus = parent.primitive
		}
		a.tv.SetFocus(focus)
		a.SetHints(top.prevSpec, top.prevComm)
		return
	}
	if top.rootFocus != nil {
		a.tv.SetFocus(top.rootFocus)
	}
	a.SetHints(top.prevSpec, top.prevComm)
}

// ReplaceTopModal swaps the top modal primitive without changing the stack
// depth. The caller must already have popped any child modal.
func (a *App) ReplaceTopModal(p tview.Primitive) error {
	if isNilPrimitive(p) {
		return fmt.Errorf("replace modal: primitive %w", apperr.ErrNil)
	}
	if len(a.modals) == 0 {
		return errors.New("replace modal: no modal")
	}
	top := &a.modals[len(a.modals)-1]
	spec, comm := a.hintsSpec, a.hintsComm
	a.pages.RemovePage(top.page)
	top.primitive = p
	if state, ok := p.(views.ModalState); ok {
		state.SetModalActive(true)
	}
	a.pages.AddPage(top.page, &modalMouseGuard{Primitive: p}, true, true)
	a.tv.SetFocus(p)
	a.SetHints(spec, comm)
	return nil
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
// any notice or error. A non-empty message clears automatically after
// statusMessageTimeout; an empty string clears it immediately.
// Callers may include tview colour tags (e.g. "[green]saved[-]").
func (a *App) SetStatus(msg string) {
	a.scheduleStatusClear(msg, func() { a.statusBar.SetStatus("") })
	a.statusBar.SetStatus(msg)
}

// SetNotice displays msg in the separate notice area below the common hints.
// It clears any inline status and error. A non-empty message clears
// automatically after statusMessageTimeout; an empty string clears it
// immediately.
func (a *App) SetNotice(msg string) {
	a.scheduleStatusClear(msg, func() { a.statusBar.SetNotice("") })
	a.statusBar.SetNotice(msg)
}

// SetError displays msg in the separate red error area below the common
// hints. It clears any inline status and notice. Errors remain visible until
// another message replaces them or an empty string clears them.
func (a *App) SetError(msg string) {
	a.cancelStatusTimer()
	a.statusBar.SetError(msg)
}

// SetHints replaces the keyboard-hint lines in the status bar.
func (a *App) SetHints(specific, common string) {
	a.hintsSpec = specific
	a.hintsComm = common
	a.statusBar.SetHints(specific, common)
}

// scheduleStatusClear replaces any pending status timer with one that clears
// the message after statusTimeout. An empty message cancels the timer without
// scheduling a new one.
func (a *App) scheduleStatusClear(msg string, clearFn func()) {
	a.statusMu.Lock()
	defer a.statusMu.Unlock()

	if a.statusTimer != nil {
		a.statusTimer.Stop()
		a.statusTimer = nil
	}
	a.statusGeneration++
	if msg == "" {
		return
	}

	timeout := a.statusTimeout
	if timeout <= 0 {
		timeout = statusMessageTimeout
	}
	generation := a.statusGeneration
	a.statusTimer = time.AfterFunc(timeout, func() {
		a.queueStatusUpdate(func() {
			a.statusMu.Lock()
			if a.statusGeneration != generation {
				a.statusMu.Unlock()
				return
			}
			a.statusGeneration++
			a.statusTimer = nil
			a.statusMu.Unlock()
			clearFn()
		})
	})
}

// cancelStatusTimer stops any pending clear and invalidates its generation.
func (a *App) cancelStatusTimer() {
	a.statusMu.Lock()
	if a.statusTimer != nil {
		a.statusTimer.Stop()
		a.statusTimer = nil
	}
	a.statusGeneration++
	a.statusMu.Unlock()
}

// queueStatusUpdate runs fn on the tview event loop. A non-nil statusQueue
// replaces the event-loop queue, allowing tests to run the update
// synchronously.
func (a *App) queueStatusUpdate(fn func()) {
	if a.statusQueue != nil {
		a.statusQueue(fn)
		return
	}
	a.tv.QueueUpdateDraw(fn)
}

// Quit stops the tview event loop.
func (a *App) Quit() {
	a.cancelStatusTimer()
	a.tv.Stop()
}

// ── Private ───────────────────────────────────────────────────────────────────

// build constructs the full UI, wires panels together, populates data, and
// handles first-run state. Called exactly once from New.
func (a *App) build() {
	a.statusBar = views.NewStatusBar()
	a.accounts = panels.NewAccountsPanel(a)
	a.sessions = panels.NewSessionsPanel(a)
	a.notes = panels.NewNotesPanel(a)
	a.accountInfo = panels.NewAccountInfoPanel(a)
	a.vendors = panels.NewVendorsPanel(a)
	a.projects = panels.NewProjectsPanel(a)
	a.users = panels.NewUsersPanel(a)

	filter := views.NewProjectFilterState()
	a.accounts.SetFilter(filter)
	a.sessions.SetFilter(filter)
	a.notes.SetFilter(filter)

	// Wire account-list selection changes to both panes and the info panel.
	a.accounts.SetOnSelect(func(acc *model.Account) {
		a.sessions.SetAccount(acc)
		a.notes.SetAccount(acc)
		a.accountInfo.SetAccount(acc)
	})

	refreshFilter := func() {
		a.sessions.Refresh()
		a.notes.Refresh()
	}
	a.accounts.SetOnFilterChange(refreshFilter)
	a.sessions.SetOnFilterChange(refreshFilter)
	a.notes.SetOnFilterChange(refreshFilter)
	a.notes.SetOnSessionStart(func(sess *model.Session) {
		a.sessions.ShowSession(sess)
	})

	// Right column: Sessions above Notes.
	a.mainRight = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.sessions, 0, 1, false).
		AddItem(a.notes, 0, 1, false)

	// Top row: accounts panel (sized to its widest row) beside the right column.
	a.mainTop = tview.NewFlex().
		AddItem(a.accounts, 0, 1, true).
		AddItem(a.mainRight, 0, 2, false)

	// Main page: top row above a full-width account panel.
	a.mainFlex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.mainTop, 0, 1, true).
		AddItem(a.accountInfo, 5, 0, false)

	// Pages for full-screen switching.
	// Modal pages are added and removed dynamically by ShowModal/HideModal.
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
	a.resizeMainRight(screenWidth, screenHeight, statusHeight)

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

// resizeMainRight sizes the Sessions and Notes panes in the right column.
func (a *App) resizeMainRight(screenWidth, screenHeight, statusHeight int) {
	if a.mainRight == nil || a.accountInfo == nil {
		return
	}
	available := screenHeight - statusHeight
	if available <= 0 {
		return
	}
	maxInfoHeight := available - minMainTopHeight
	infoHeight := 0
	if maxInfoHeight >= 1 {
		infoHeight = min(a.accountInfo.RequiredHeight(screenWidth), maxInfoHeight)
	}
	topHeight := available - infoHeight
	if topHeight <= 0 {
		return
	}
	notesHeight := max(3, topHeight/3)
	sessionsHeight := topHeight - notesHeight
	a.mainRight.ResizeItem(a.sessions, sessionsHeight, 0)
	a.mainRight.ResizeItem(a.notes, notesHeight, 0)

	rightWidth := screenWidth - a.accounts.RequiredWidth()
	innerWidth := max(rightWidth-2, 0)
	a.sessions.SetWidth(innerWidth)
	a.notes.SetWidth(innerWidth)
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
// only to the main page; screen switching is disabled while any modal is open,
// so a form or editor keeps its own rune input. All input is blocked while the
// terminal is too small.
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
		a.cycleMainFocus(event.Key() == views.KeyFocusNext)
		return nil
	}

	if event.Key() != tcell.KeyRune {
		return event
	}
	if len(a.modals) > 0 {
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

// cycleMainFocus moves focus through Accounts, Sessions and Notes.
func (a *App) cycleMainFocus(forward bool) {
	order := []tview.Primitive{a.accounts, a.sessions, a.notes}
	current := a.tv.GetFocus()
	index := 0
	for i, p := range order {
		if p == current || p.HasFocus() {
			index = i
			break
		}
	}
	if forward {
		index = (index + 1) % len(order)
	} else {
		index = (index + len(order) - 1) % len(order)
	}
	a.tv.SetFocus(order[index])
}

// handleGlobalMouse blocks mouse input while the terminal is too small.
func (a *App) handleGlobalMouse(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if a.tooSmall {
		return nil, action
	}
	return event, action
}
