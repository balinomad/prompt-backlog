package views

import (
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
)

// ── construction ──────────────────────────────────────────────────────────────

func TestNewSessionsPanel(t *testing.T) {
	t.Parallel()
	if p := NewSessionsPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil SessionsPanel")
	}
}

func TestSessionsPanel_SelectedBeforeRefresh(t *testing.T) {
	t.Parallel()
	p := NewSessionsPanel(newMockApp(t))
	if s := p.Selected(); s != nil {
		t.Errorf("expected nil Selected before Refresh, got %v", s)
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestSessionsPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*mockApp)
		wantCount int
	}{
		{
			name:      "two sessions",
			setup:     func(_ *mockApp) {},
			wantCount: 2,
		},
		{
			name: "empty registry",
			setup: func(app *mockApp) {
				app.registry = model.NewRegistry()
				must(t, app.registry.AddProvider(model.NewProvider("claude", true, false)))
			},
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			tc.setup(app)
			p := NewSessionsPanel(app)
			p.Refresh()
			if got := p.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestSessionsPanel_Refresh_SelectedNotNil(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewSessionsPanel(app)
	p.Refresh()
	if p.Selected() == nil {
		t.Error("expected non-nil Selected after Refresh")
	}
}

func TestSessionsPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewSessionsPanel(app)
	p.Refresh()
	p.SetCurrentItem(1)

	want := p.Selected()
	if want == nil {
		t.Fatal("second item should not be nil")
	}
	email := want.User().Email()
	provider := want.Provider().Name()

	p.Refresh()

	got := p.Selected()
	if got == nil {
		t.Fatal("selection should survive Refresh")
	}
	if got.User().Email() != email || got.Provider().Name() != provider {
		t.Errorf("selection changed: got %s/%s, want %s/%s",
			got.User().Email(), got.Provider().Name(), email, provider)
	}
}

func TestSessionsPanel_Refresh_CallsOnSelect(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewSessionsPanel(app)
	var called bool
	p.SetOnSelect(func(_ *model.Session) { called = true })
	p.Refresh()
	if !called {
		t.Error("onSelect should be called after Refresh")
	}
}

// ── keyboard handlers ─────────────────────────────────────────────────────────

func TestSessionsPanel_Keys_SwitchScreens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		key        rune
		wantSwitch string
	}{
		{"providers", KeyProviders, "providers"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewSessionsPanel(app)
			p.Refresh()
			p.handleKey(runeKey(tc.key))
			if app.switchedTo != tc.wantSwitch {
				t.Errorf("got switchedTo=%q, want %q", app.switchedTo, tc.wantSwitch)
			}
		})
	}
}

func TestSessionsPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       rune
		wantHints string
	}{
		{"new", KeyNew, HintsForm},
		{"edit", KeyEdit, HintsForm},
		{"delete", KeyDelete, HintsConfirmModal},
		{"help", KeyHelp, HintsHelpOverlay},
		{"quit", KeyQuit, HintsConfirmModal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewSessionsPanel(app)
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

func TestSessionsPanel_EmptyList_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  rune
	}{
		{"delete", KeyDelete},
		{"edit", KeyEdit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			app.registry = model.NewRegistry()
			must(t, app.registry.AddProvider(model.NewProvider("claude", true, false)))
			p := NewSessionsPanel(app)
			p.Refresh()
			p.handleKey(runeKey(tc.key))
			if app.modal != nil {
				t.Errorf("key %q with empty list should not open a modal", string(tc.key))
			}
		})
	}
}

// ── sortSessionsByAvailability ────────────────────────────────────────────────

func TestSortSessionsByAvailability(t *testing.T) {
	t.Parallel()
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	nearFuture := now.Add(1 * time.Hour)
	farFuture := now.Add(3 * time.Hour)

	cases := []struct {
		name      string
		input     []*time.Time
		wantFirst *time.Time // nil means first session should have nil NextPromptAt
	}{
		{
			name:      "nil before future",
			input:     []*time.Time{&farFuture, nil},
			wantFirst: nil,
		},
		{
			name:      "past before future",
			input:     []*time.Time{&farFuture, &past},
			wantFirst: &past,
		},
		{
			name:      "sooner future before later future",
			input:     []*time.Time{&farFuture, &nearFuture},
			wantFirst: &nearFuture,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sessions := make([]*model.Session, len(tc.input))
			for i, ts := range tc.input {
				u := model.NewUser("x@x.com", "X")
				prov := model.NewProvider("p", false, false)
				s := model.NewSession(u, prov)
				s.NextPromptAt = ts
				sessions[i] = s
			}
			sortSessionsByAvailability(sessions)
			first := sessions[0]
			if tc.wantFirst == nil {
				if first.NextPromptAt != nil {
					t.Errorf("want nil NextPromptAt first, got %v", first.NextPromptAt)
				}
			} else {
				if first.NextPromptAt == nil || !first.NextPromptAt.Equal(*tc.wantFirst) {
					t.Errorf("want first=%v, got %v", tc.wantFirst, first.NextPromptAt)
				}
			}
		})
	}
}

// ── formatNextPromptAt ────────────────────────────────────────────────────────

func TestFormatNextPromptAt(t *testing.T) {
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
			gotText, gotIsPast := formatNextPromptAt(&tc.t)
			if gotText != tc.wantText {
				t.Errorf("text: got %q, want %q", gotText, tc.wantText)
			}
			if gotIsPast != tc.wantIsPast {
				t.Errorf("isPast: got %v, want %v", gotIsPast, tc.wantIsPast)
			}
		})
	}
}
