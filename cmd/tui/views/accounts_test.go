package views

import (
	"fmt"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
)

// ── construction ──────────────────────────────────────────────────────────────

func TestNewAccountsPanel(t *testing.T) {
	t.Parallel()
	if p := NewAccountsPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil AccountsPanel")
	}
}

func TestAccountsPanel_SelectedBeforeRefresh(t *testing.T) {
	t.Parallel()
	p := NewAccountsPanel(newMockApp(t))
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
			p := NewAccountsPanel(app)
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
	p := NewAccountsPanel(app)
	p.Refresh()
	if p.Selected() == nil {
		t.Error("expected non-nil Selected after Refresh")
	}
}

func TestAccountsPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewAccountsPanel(app)
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
	p := NewAccountsPanel(app)
	var called bool
	p.SetOnSelect(func(_ *model.Account) { called = true })
	p.Refresh()
	if !called {
		t.Error("onSelect should be called after Refresh")
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
		{"providers", KeyProviders, "providers"},
		{"projects", KeyProjects, "projects"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewAccountsPanel(app)
			p.Refresh()
			p.handleKey(runeKey(tc.key))
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
		{"new", KeyNew, HintsForm},
		{"edit", KeyEdit, HintsForm},
		{"delete", KeyDelete, HintsConfirmModal},
		{"rename user", KeyRenameUser, HintsForm},
		{"help", KeyHelp, HintsHelpOverlay},
		{"quit", KeyQuit, HintsConfirmModal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewAccountsPanel(app)
			p.Refresh()
			p.handleKey(runeKey(tc.key))
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
		{"delete", KeyDelete},
		{"edit", KeyEdit},
		{"rename user", KeyRenameUser},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			app.svc = newTestService(t, reg)
			p := NewAccountsPanel(app)
			p.Refresh()
			p.handleKey(runeKey(tc.key))
			if app.modal != nil {
				t.Errorf("key %q with empty list should not open a modal", string(tc.key))
			}
		})
	}
}

// ── sortAccountsByAvailability ────────────────────────────────────────────────

// buildPausedAccount returns an account paused until pauseAt, or never paused
// when pauseAt is the zero value. Account.PauseUntil has no exported setter
// (unlike the old Session.NextPromptAt), so pause state can only be produced
// through a real Registry's PauseAccount, via copy-on-write.
func buildPausedAccount(t *testing.T, email string, pauseAt time.Time) *model.Account {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("p", false, false)))
	must(t, reg.AddUser(model.NewUser(email, "X")))
	acc, err := reg.AddAccount(reg.User(email), reg.Provider("p"))
	must(t, err)
	if !pauseAt.IsZero() {
		must(t, reg.PauseAccount(acc, pauseAt))
		acc = reg.Account(reg.User(email), reg.Provider("p"))
	}
	return acc
}

func TestSortAccountsByAvailability(t *testing.T) {
	t.Parallel()
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	nearFuture := now.Add(1 * time.Hour)
	farFuture := now.Add(3 * time.Hour)

	cases := []struct {
		name      string
		input     []time.Time // zero value means "never paused"
		wantFirst time.Time   // zero value means first account should be unpaused
	}{
		{
			name:      "unpaused before future",
			input:     []time.Time{farFuture, {}},
			wantFirst: time.Time{},
		},
		{
			name:      "past before future",
			input:     []time.Time{farFuture, past},
			wantFirst: past,
		},
		{
			name:      "sooner future before later future",
			input:     []time.Time{farFuture, nearFuture},
			wantFirst: nearFuture,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			accounts := make([]*model.Account, len(tc.input))
			for i, pauseAt := range tc.input {
				accounts[i] = buildPausedAccount(t, fmt.Sprintf("user%d@x.com", i), pauseAt)
			}
			sortAccountsByAvailability(accounts)
			first := accounts[0]
			if tc.wantFirst.IsZero() {
				if !first.PauseUntil().IsZero() {
					t.Errorf("want unpaused first, got PauseUntil=%v", first.PauseUntil())
				}
			} else {
				if first.PauseUntil().IsZero() || !first.PauseUntil().Equal(tc.wantFirst) {
					t.Errorf("want first=%v, got %v", tc.wantFirst, first.PauseUntil())
				}
			}
		})
	}
}

// ── formatPauseUntil ──────────────────────────────────────────────────────────

func TestFormatPauseUntil(t *testing.T) {
	t.Parallel()
	now := time.Now()
	sameDay := time.Date(now.Year(), now.Month(), now.Day(), 14, 30, 0, 0, now.Location())
	otherDay := time.Date(2027, 6, 15, 9, 0, 0, 0, time.Local)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name       string
		t          time.Time
		wantText   string
		wantIsPast bool
	}{
		{"same day format", sameDay, "14:30", sameDay.Before(now)},
		{"other day format", otherDay, "06-15 09:00", false},
		{"past is past", past, past.Format("15:04"), true},
		{"future not past", future, future.Format("15:04"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotText, gotIsPast := formatPauseUntil(tc.t)
			if gotText != tc.wantText {
				t.Errorf("text: got %q, want %q", gotText, tc.wantText)
			}
			if gotIsPast != tc.wantIsPast {
				t.Errorf("isPast: got %v, want %v", gotIsPast, tc.wantIsPast)
			}
		})
	}
}
