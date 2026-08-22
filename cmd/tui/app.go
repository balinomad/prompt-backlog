// Package main implements the tui application.
package main

import (
	"fmt"
	"reflect"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/apperr"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/rivo/tview"
)

// AppName is the application's display name, used in dialog titles and
// CLI output.
const AppName = "llm-desk"

// Compile-time check that App implements views.App.
var _ views.App = (*App)(nil)

// App is the concrete host application. It owns the tview event loop and the
// application service layer. All views interact with it exclusively through
// the views.App interface.
type App struct {
	tv        *tview.Application
	svc       *service.Service
	pages     *tview.Pages
	prevFocus tview.Primitive // saved by ShowModal, restored by HideModal

	statusBar *views.StatusBar
	accounts  *views.AccountsPanel
	detail    *views.DetailPanel
	providers *views.ProvidersPanel
	projects  *views.ProjectsPanel
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

// Run starts the tview event loop. It blocks until the application stops.
func (a *App) Run() error {
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
// always reflected. The save error, if any, is returned to the caller.
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
	if saveErr == nil {
		a.statusBar.SetMessage("[green]saved ✓[-]")
	}
	return saveErr
}

// ShowModal displays p as a centred transparent overlay. The currently focused
// primitive is saved so HideModal can restore it.
// Returns an error if p is nil — including a non-nil interface wrapping a
// nil concrete pointer, which a plain p == nil check would miss.
func (a *App) ShowModal(p tview.Primitive) error {
	if isNilPrimitive(p) {
		return fmt.Errorf("show modal: primitive %w", apperr.ErrNil)
	}
	a.prevFocus = a.tv.GetFocus()
	a.pages.AddPage("modal", p, true, true)
	a.tv.SetFocus(p)
	return nil
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

// SwitchToProviders refreshes and shows the providers management screen.
func (a *App) SwitchToProviders() {
	a.providers.Refresh()
	a.pages.SwitchToPage("providers")
	a.tv.SetFocus(a.providers)
}

// SwitchToMain shows the main accounts + detail screen and returns focus to
// the accounts list.
func (a *App) SwitchToMain() {
	a.pages.SwitchToPage("main")
	a.tv.SetFocus(a.accounts)
}

// SwitchToProjects refreshes and shows the projects management screen.
func (a *App) SwitchToProjects() {
	a.projects.Refresh()
	a.pages.SwitchToPage("projects")
	a.tv.SetFocus(a.projects)
}

// SetStatus displays msg in the right section of the status bar.
// An empty string clears any previous message.
// Callers may include tview colour tags (e.g. "[red]error[-]").
func (a *App) SetStatus(msg string) {
	a.statusBar.SetMessage(msg)
}

// SetHints replaces the keyboard-hint line in the left section of the status bar.
func (a *App) SetHints(hints string) {
	a.statusBar.SetHints(hints)
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
	a.providers = views.NewProvidersPanel(a)
	a.projects = views.NewProjectsPanel(a)

	// Wire account-list selection changes to the detail panel.
	a.accounts.SetOnSelect(a.detail.SetAccount)

	// Main split: accounts panel (1 part) beside detail panel (2 parts).
	mainFlex := tview.NewFlex().
		AddItem(a.accounts, 0, 1, true).
		AddItem(a.detail, 0, 2, false)

	// Pages for full-screen switching.
	// The "modal" page is added/removed dynamically by ShowModal/HideModal.
	a.pages = tview.NewPages().
		AddPage("main", mainFlex, true, true).
		AddPage("providers", a.providers, true, false).
		AddPage("projects", a.projects, true, false)

	// Root layout: content area above, status bar pinned to the bottom.
	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.statusBar, 1, 0, false)

	a.tv.SetRoot(root, true).EnableMouse(true)

	// Initial data load.
	a.refreshAll()
	a.tv.SetFocus(a.accounts)

	// First-run: no providers means nothing can be done yet.
	// Send the operator to the providers screen with a helpful prompt.
	if len(a.svc.Providers()) == 0 {
		a.SwitchToProviders()
		a.statusBar.SetMessage(
			"No providers configured — press [::b]n[-::-] to add one.")
	}
}

// refreshAll repopulates every panel from the current in-memory registry.
// Called after every successful mutation and when a modal is hidden.
func (a *App) refreshAll() {
	a.accounts.Refresh()
	a.detail.Refresh()
	a.providers.Refresh()
	a.projects.Refresh()
}

// isNilPrimitive reports whether p is nil — including the case where p is a
// non-nil interface wrapping a nil concrete pointer (e.g. a *tview.Flex(nil)
// returned as a tview.Primitive), which p == nil does not detect.
func isNilPrimitive(p tview.Primitive) bool {
	if p == nil {
		return true
	}
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}
