package views_test

import (
	"strings"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── construction ──────────────────────────────────────────────────────────────

func TestNewAccountsPanel(t *testing.T) {
	t.Parallel()
	if p := views.NewAccountsPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil AccountsPanel")
	}
}

func TestAccountsPanel_SelectedBeforeRefresh(t *testing.T) {
	t.Parallel()
	p := views.NewAccountsPanel(newMockApp(t))
	if a := p.Selected(); a != nil {
		t.Errorf("expected nil Selected before Refresh, got %v", a)
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestAccountsPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*testing.T, *mockApp)
		wantCount int
	}{
		{
			name:      "two accounts",
			setup:     func(_ *testing.T, _ *mockApp) {},
			wantCount: 2,
		},
		{
			name: "empty registry",
			setup: func(t *testing.T, app *mockApp) {
				t.Helper()
				reg := model.NewRegistry()
				must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
				app.svc = newTestService(t, reg)
			},
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			tc.setup(t, app)
			p := views.NewAccountsPanel(app)
			p.Refresh()
			if got := p.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestAccountsPanel_Refresh_SelectedNotNil(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()
	if p.Selected() == nil {
		t.Error("expected non-nil Selected after Refresh")
	}
}

func TestAccountsPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()
	p.SetCurrentItem(1)

	want := p.Selected()
	if want == nil {
		t.Fatal("second item should not be nil")
	}
	email := want.UserEmail()
	provider := want.ProviderName()

	p.Refresh()

	got := p.Selected()
	if got == nil {
		t.Fatal("selection should survive Refresh")
	}
	if got.UserEmail() != email || got.ProviderName() != provider {
		t.Errorf("selection changed: got %s/%s, want %s/%s",
			got.UserEmail(), got.ProviderName(), email, provider)
	}
}

func TestAccountsPanel_Refresh_CallsOnSelect(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	var called bool
	p.SetOnSelect(func(_ *model.Account) { called = true })
	p.Refresh()
	if !called {
		t.Error("onSelect should be called after Refresh")
	}
}

// TestAccountsPanel_Refresh_ClearsDetailPanelOnLastAccountDeleted reaches
// DetailPanel's unexported sessionsList/infoView fields via the exported
// Flex.GetItem: DetailPanel puts its list at index 0, its info view at
// index 1 (see detail.go). The info-text check is a stronger signal than
// the old p.account != nil field check — it distinguishes "no account"
// from "account with zero sessions" by what's actually rendered.
func TestAccountsPanel_Refresh_ClearsDetailPanelOnLastAccountDeleted(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)

	app := &mockApp{svc: newTestService(t, reg)}
	accounts := views.NewAccountsPanel(app)
	detail := views.NewDetailPanel(app)
	accounts.SetOnSelect(detail.SetAccount)
	accounts.Refresh()

	detailInfo, ok := detail.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 1, got %T", detail.GetItem(1))
	}
	if strings.Contains(detailInfo.GetText(false), "no account selected") {
		t.Fatal("detail panel should show the sole account before deletion")
	}

	must(t, reg.DeleteAccount(acc))
	accounts.Refresh()

	if !strings.Contains(detailInfo.GetText(false), "no account selected") {
		t.Error("detail panel should be cleared after the last account is deleted")
	}
	detailList, ok := detail.GetItem(0).(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List at index 0, got %T", detail.GetItem(0))
	}
	if detailList.GetItemCount() != 0 {
		t.Error("detail panel's session list should be empty after the last account is deleted")
	}
}

// ── keyboard handlers ─────────────────────────────────────────────────────────

func TestAccountsPanel_Keys_SwitchScreens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		key        rune
		wantSwitch string
	}{
		{"providers", views.KeyProviders, "providers"},
		{"projects", views.KeyProjects, "projects"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := views.NewAccountsPanel(app)
			p.Refresh()
			capture := p.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(runeKey(tc.key))
			if app.switchedTo != tc.wantSwitch {
				t.Errorf("got switchedTo=%q, want %q", app.switchedTo, tc.wantSwitch)
			}
		})
	}
}

func TestAccountsPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       rune
		wantHints string
	}{
		{"new", views.KeyNew, views.HintsForm},
		{"edit", views.KeyEdit, views.HintsForm},
		{"delete", views.KeyDelete, views.HintsConfirmModal},
		{"rename user", views.KeyRenameUser, views.HintsForm},
		{"help", views.KeyHelp, views.HintsHelpOverlay},
		{"quit", views.KeyQuit, views.HintsConfirmModal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := views.NewAccountsPanel(app)
			p.Refresh()
			capture := p.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(runeKey(tc.key))
			if app.modal == nil {
				t.Errorf("key %q: expected modal to be shown", string(tc.key))
			}
			if app.hints != tc.wantHints {
				t.Errorf("key %q: hints got %q, want %q", string(tc.key), app.hints, tc.wantHints)
			}
		})
	}
}

func TestAccountsPanel_EmptyList_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  rune
	}{
		{"delete", views.KeyDelete},
		{"edit", views.KeyEdit},
		{"rename user", views.KeyRenameUser},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			app.svc = newTestService(t, reg)
			p := views.NewAccountsPanel(app)
			p.Refresh()
			capture := p.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(runeKey(tc.key))
			if app.modal != nil {
				t.Errorf("key %q with empty list should not open a modal", string(tc.key))
			}
		})
	}
}

// ── sortAccountsByAvailability ────────────────────────────────────────────────

// TestAccountItemText_EscapesEmailAndProvider replaces the old direct call
// to the unexported accountItemText: builds an account with bracketed
// email/provider through a real registry, refreshes, and reads the
// rendered row back via the embedded *tview.List's exported GetItemText.
func TestAccountItemText_EscapesEmailAndProvider(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude]blue", false, false)))
	must(t, reg.AddUser(model.NewUser("alice[red]@example.com", "Alice")))
	_, err := reg.AddAccount(reg.User("alice[red]@example.com"), reg.Provider("claude]blue"))
	must(t, err)

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewAccountsPanel(app)
	p.Refresh()

	got, _ := p.GetItemText(0)
	if !strings.Contains(got, tview.Escape("alice[red]@example.com")) {
		t.Errorf("got %q, want it to contain the escaped email", got)
	}
	if !strings.Contains(got, tview.Escape("claude]blue")) {
		t.Errorf("got %q, want it to contain the escaped provider", got)
	}
}

// TestSortAccountsByAvailability replaces the old direct call to the
// unexported sortAccountsByAvailability: two accounts share one registry
// (firstAcc pinned to farFuture; secondAcc varies by case), and each case
// checks which one Refresh() places first via GetItemText — the same
// three pairwise comparisons the original table exercised (unpaused,
// past, and near-future all sort ahead of a far-future pause), just
// identified by email instead of by comparing time.Time values directly.
func TestSortAccountsByAvailability(t *testing.T) {
	t.Parallel()
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	nearFuture := now.Add(1 * time.Hour)
	farFuture := now.Add(3 * time.Hour)

	cases := []struct {
		name          string
		secondPauseAt time.Time // zero means never paused
	}{
		{"unpaused before future", time.Time{}},
		{"past before future", past},
		{"sooner future before later future", nearFuture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("p", false, false)))
			must(t, reg.AddUser(model.NewUser("first@x.com", "First")))
			must(t, reg.AddUser(model.NewUser("second@x.com", "Second")))
			firstAcc, err := reg.AddAccount(reg.User("first@x.com"), reg.Provider("p"))
			must(t, err)
			_, err = reg.AddAccount(reg.User("second@x.com"), reg.Provider("p"))
			must(t, err)
			must(t, reg.PauseAccount(firstAcc, farFuture))
			if !tc.secondPauseAt.IsZero() {
				secondAcc := reg.Account(reg.User("second@x.com"), reg.Provider("p"))
				must(t, reg.PauseAccount(secondAcc, tc.secondPauseAt))
			}

			app := &mockApp{svc: newTestService(t, reg)}
			p := views.NewAccountsPanel(app)
			p.Refresh()

			got, _ := p.GetItemText(0)
			if !strings.Contains(got, "second@x.com") {
				t.Errorf("first item = %q, want it to contain %q (more available than first@x.com, paused to farFuture)", got, "second@x.com")
			}
		})
	}
}

// TestAccountsPanel_SetCurrentItem_TriggersOnSelect covers the
// SetChangedFunc wiring specifically, distinct from
// TestAccountsPanel_Refresh_CallsOnSelect: Refresh calls onSelect
// directly and guards SetCurrentItem with p.refreshing while it runs, so
// it never exercises the changed-callback path. Navigating the list
// outside a refresh (arrow keys, in real use) does, and that path has its
// own onSelect != nil / index-bounds guard worth verifying independently.
func TestAccountsPanel_SetCurrentItem_TriggersOnSelect(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh() // populates the list; selection starts at index 0

	var selected *model.Account
	p.SetOnSelect(func(a *model.Account) { selected = a })

	p.SetCurrentItem(1)

	if selected == nil {
		t.Fatal("expected onSelect to fire when the selection changes outside Refresh")
	}
	if want := p.Selected(); selected.Key() != want.Key() {
		t.Errorf("onSelect received %+v, want %+v", selected.Key(), want.Key())
	}
}

// TestAccountsPanel_NonRuneKey_PassesThrough covers dispatchPanelKey's
// non-rune early return (keys.go), shared by every panel that calls it.
// Exercised here via AccountsPanel as the vehicle, same principle as
// addCtrlS being tested through one form for all eight callers.
func TestAccountsPanel_NonRuneKey_PassesThrough(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()

	event := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	if got := p.GetInputCapture()(event); got != event {
		t.Error("non-rune key must pass through unchanged")
	}
}

// TestAccountsPanel_New_SaveReturnsToPanel closes the loop that
// TestAccountsPanel_Keys_ShowModal leaves open: that test only checks
// that pressing 'n' opens a modal with the right hints, not that
// completing the form hands control back to the panel correctly. The
// onClose closure passed to NewAccountForm (SetHints + HideModal) is
// otherwise never actually invoked by any existing test.
func TestAccountsPanel_New_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()

	capture := p.GetInputCapture()
	capture(runeKey(views.KeyNew))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Email", "carol@example.com")
	setInputField(t, form, "Name", "Carol")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hints != views.HintsAccountsPanel {
		t.Errorf("hints after save: got %q, want %q", app.hints, views.HintsAccountsPanel)
	}
	if _, err := app.svc.Account("carol@example.com", "chatgpt"); err != nil {
		t.Errorf("account not created: %v", err)
	}
}

// TestAccountsPanel_Edit_SaveReturnsToPanel is the same round trip as
// TestAccountsPanel_New_SaveReturnsToPanel for handleEdit's own onClose
// closure.
func TestAccountsPanel_Edit_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh() // selects alice|claude first (sorted by key, both unpaused)

	capture := p.GetInputCapture()
	capture(runeKey(views.KeyEdit))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Pause until", "2026-12-31 23:59")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hints != views.HintsAccountsPanel {
		t.Errorf("hints after save: got %q, want %q", app.hints, views.HintsAccountsPanel)
	}
}

// TestAccountsPanel_Delete_Cancel is the accounts.go counterpart to
// TestDetailPanel_DeleteSession_Cancel: confirming the delete is
// button-click-gated and unreachable black box (see that test's comment
// for why), so this covers the reachable half, cancelling leaves the
// account untouched and hands control back to the panel.
func TestAccountsPanel_Delete_Cancel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()

	before := len(app.svc.Accounts())

	capture := p.GetInputCapture()
	capture(runeKey(views.KeyDelete))

	confirmModal, ok := app.modal.(*tview.Modal)
	if !ok {
		t.Fatalf("expected *tview.Modal, got %T", app.modal)
	}
	confirmModal.GetInputCapture()(escKey())

	if app.modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	if app.hints != views.HintsAccountsPanel {
		t.Errorf("hints after cancel: got %q, want %q", app.hints, views.HintsAccountsPanel)
	}
	if got := len(app.svc.Accounts()); got != before {
		t.Errorf("account count changed after cancel: got %d, want %d (unchanged)", got, before)
	}
}

// TestAccountsPanel_RenameUser_SaveReturnsToPanel is the same round trip
// as TestAccountsPanel_New_SaveReturnsToPanel for handleRenameUser's own
// onClose closure.
func TestAccountsPanel_RenameUser_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh() // selects alice|claude first

	capture := p.GetInputCapture()
	capture(runeKey(views.KeyRenameUser))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Name", "Alice Smith")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hints != views.HintsAccountsPanel {
		t.Errorf("hints after save: got %q, want %q", app.hints, views.HintsAccountsPanel)
	}
	if got := app.svc.User("alice@example.com").Name(); got != "Alice Smith" {
		t.Errorf("Name() = %q, want %q", got, "Alice Smith")
	}
}
