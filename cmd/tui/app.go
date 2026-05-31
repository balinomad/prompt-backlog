package main

import (
	"fmt"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/store"

	"github.com/rivo/tview"
)

// Compile-time check that App implements views.App.
var _ views.App = (*App)(nil)

// App is the concrete host application. It owns the tview event loop, the
// persistent store, and the in-memory registry. All views interact with it
// exclusively through the views.App interface.
type App struct {
	tv        *tview.Application
	store     store.Store
	registry  model.Registry
	pages     *tview.Pages
	prevFocus tview.Primitive // saved by ShowModal, restored by HideModal

	statusBar  *views.StatusBar
	sessions   *views.SessionsPanel
	detail     *views.DetailPanel
	providers  *views.ProvidersPanel
	unattached *views.UnattachedPanel
}

// New loads the registry from s and constructs the fully wired application.
// Returns an error only when the store fails to load.
func New(s store.Store) (*App, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}
	a := &App{
		tv:       tview.NewApplication(),
		store:    s,
		registry: reg,
	}
	a.build()
	return a, nil
}

// Run starts the tview event loop. It blocks until the application stops.
func (a *App) Run() error {
	return a.tv.Run()
}

// ── views.App implementation ──────────────────────────────────────────────────

// Registry returns the current in-memory registry.
// Views read from this; mutations go through Mutate.
func (a *App) Registry() model.Registry {
	return a.registry
}

// Mutate applies fn to the registry. If fn returns nil, the result is
// persisted to disk and all views are refreshed. Views are refreshed
// regardless of whether the save succeeded so the in-memory state is
// always reflected. The save error, if any, is returned to the caller.
func (a *App) Mutate(fn func(*model.Registry) error) error {
	if err := fn(&a.registry); err != nil {
		return err
	}
	saveErr := a.store.Save(a.registry)
	a.refreshAll()
	return saveErr
}

// ShowModal displays p as a centred transparent overlay. The currently focused
// primitive is saved so HideModal can restore it.
func (a *App) ShowModal(p tview.Primitive) {
	a.prevFocus = a.tv.GetFocus()
	a.pages.AddPage("modal", p, true, true)
	a.tv.SetFocus(p)
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

// SwitchToUnattached refreshes and shows the unattached projects screen.
func (a *App) SwitchToUnattached() {
	a.unattached.Refresh()
	a.pages.SwitchToPage("unattached")
	a.tv.SetFocus(a.unattached)
}

// SwitchToMain shows the main sessions + detail screen and returns focus to
// the sessions list.
func (a *App) SwitchToMain() {
	a.pages.SwitchToPage("main")
	a.tv.SetFocus(a.sessions)
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
	a.sessions = views.NewSessionsPanel(a)
	a.detail = views.NewDetailPanel(a)
	a.providers = views.NewProvidersPanel(a)
	a.unattached = views.NewUnattachedPanel(a)

	// Wire session-list selection changes to the detail panel.
	a.sessions.SetOnSelect(a.detail.SetSession)

	// Main split: sessions panel (1 part) beside detail panel (2 parts).
	mainFlex := tview.NewFlex().
		AddItem(a.sessions, 0, 1, true).
		AddItem(a.detail, 0, 2, false)

	// Pages for full-screen switching.
	// The "modal" page is added/removed dynamically by ShowModal/HideModal.
	a.pages = tview.NewPages().
		AddPage("main", mainFlex, true, true).
		AddPage("providers", a.providers, true, false).
		AddPage("unattached", a.unattached, true, false)

	// Root layout: content area above, status bar pinned to the bottom.
	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.statusBar, 1, 0, false)

	a.tv.SetRoot(root, true).EnableMouse(true)

	// Initial data load.
	a.refreshAll()
	a.tv.SetFocus(a.sessions)

	// First-run: no providers means nothing can be done yet.
	// Send the operator to the providers screen with a helpful prompt.
	if len(a.registry.AllProviders()) == 0 {
		a.SwitchToProviders()
		a.statusBar.SetMessage(
			"No providers configured — press [::b]n[-::-] to add one.")
	}
}

// refreshAll repopulates every panel from the current in-memory registry.
// Called after every successful mutation and when a modal is hidden.
func (a *App) refreshAll() {
	a.sessions.Refresh()
	a.detail.Refresh()
	a.providers.Refresh()
	a.unattached.Refresh()
}
