package views

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/internal/model"

	"github.com/rivo/tview"
)

// ── construction ──────────────────────────────────────────────────────────────

func TestNewDetailPanel(t *testing.T) {
	t.Parallel()
	if p := NewDetailPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil DetailPanel")
	}
}

// ── SetAccount / Refresh ──────────────────────────────────────────────────────

// newDetailTestApp returns a mock wired with a single account that has two
// sessions attached, plus one granted project. Defined locally rather than
// added to newMockApp in helpers_test.go: that shared fixture deliberately
// carries no sessions, and this file is the only current caller that needs
// them.
//
//	Account:  alice|claude, project "Alpha" granted
//	Sessions: #1 "Architecture review" / Alpha / queue=[continue] (pending)
//	          #2 untitled / no project / empty queue
func newDetailTestApp(t *testing.T) (*mockApp, *model.Account) {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
	_, err = reg.AddSession(acc, reg.Project("Alpha"), "Architecture review",
		[]model.Prompt{model.NewPrompt("continue", false)})
	must(t, err)
	_, err = reg.AddSession(acc, nil, "", nil)
	must(t, err)

	app := &mockApp{svc: newTestService(t, reg)}
	return app, acc
}

func TestDetailPanel_SetAccount_Nil(t *testing.T) {
	t.Parallel()
	p := NewDetailPanel(newMockApp(t))
	p.SetAccount(nil)
	if p.account != nil {
		t.Error("account should be nil after SetAccount(nil)")
	}
}

func TestDetailPanel_SetAccount_ItemCount(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := NewDetailPanel(app)
	p.SetAccount(acc)
	if got := p.sessionsList.GetItemCount(); got != 2 {
		t.Errorf("expected 2 list items, got %d", got)
	}
}

func TestDetailPanel_Refresh_ClearsOnDeletedAccount(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	app := &mockApp{svc: newTestService(t, reg)}

	p := NewDetailPanel(app)
	p.SetAccount(acc)

	must(t, reg.DeleteAccount(acc))
	p.Refresh()
	if p.account != nil {
		t.Error("account should be nil after deletion from registry")
	}
	if p.sessionsList.GetItemCount() != 0 {
		t.Error("list should be empty after account deleted")
	}
}

// ── keyboard handlers ─────────────────────────────────────────────────────────

func TestDetailPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       rune
		wantHints string
	}{
		{"help", KeyHelp, HintsHelpOverlay},
		{"quit", KeyQuit, HintsConfirmModal},
		{"new session", KeyNew, HintsForm},
		{"edit session", KeyEdit, HintsForm},
		{"delete session", KeyDelete, HintsConfirmModal},
		{"queue editor", KeyQueue, HintsQueueEditor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc := newDetailTestApp(t)
			p := NewDetailPanel(app)
			p.SetAccount(acc)
			p.sessionsList.SetCurrentItem(0)
			p.handleKey(runeKey(tc.key))
			if app.modal == nil {
				t.Errorf("key %q: expected modal", string(tc.key))
			}
			if app.hints != tc.wantHints {
				t.Errorf("key %q: hints got %q, want %q", string(tc.key), app.hints, tc.wantHints)
			}
		})
	}
}

func TestDetailPanel_NoSessions_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  rune
	}{
		{"edit", KeyEdit},
		{"delete", KeyDelete},
		{"queue", KeyQueue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
			must(t, err)
			app := &mockApp{svc: newTestService(t, reg)}

			p := NewDetailPanel(app)
			p.SetAccount(acc)
			p.handleKey(runeKey(tc.key))
			if app.modal != nil {
				t.Errorf("key %q with no sessions should not open a modal", string(tc.key))
			}
		})
	}
}

func TestDetailPanel_Toggle_Continue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		initQueue   []model.Prompt
		wantLen     int
		wantPending bool
	}{
		{"empty queue gets continue appended", nil, 1, true},
		{"pending continue gets removed", []model.Prompt{model.NewPrompt("continue", false)}, 0, false},
		{"non-continue tail is kept, continue appended", []model.Prompt{model.NewPrompt("do X", false)}, 2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
			must(t, err)
			sess, err := reg.AddSession(acc, nil, "topic", tc.initQueue)
			must(t, err)
			app := &mockApp{svc: newTestService(t, reg)}

			p := NewDetailPanel(app)
			p.SetAccount(acc)
			p.sessionsList.SetCurrentItem(0)
			p.handleKey(runeKey(KeyToggle))

			updated := app.svc.Session(sess.ID())
			if updated == nil {
				t.Fatal("session not found after toggle")
			}
			q := updated.Queue()
			if len(q) != tc.wantLen {
				t.Fatalf("queue length: got %d, want %d", len(q), tc.wantLen)
			}
			if isContinuePending(q) != tc.wantPending {
				t.Errorf("isContinuePending: got %v, want %v", isContinuePending(q), tc.wantPending)
			}
		})
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func TestSessionItemText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		topic        string
		project      string
		queue        []model.Prompt
		wantContains []string
	}{
		{"untitled no project no queue", "", "", nil, []string{"(untitled)", "queue: 0"}},
		{"topic and project shown", "Architecture review", "Alpha", nil, []string{"Architecture review", "Alpha", "queue: 0"}},
		{"pending continue reflected in count", "topic", "", []model.Prompt{model.NewPrompt("continue", false)}, []string{"queue: 1"}},
		{"bracket in topic and project is escaped", "plan [x]", "proj]y", nil, []string{tview.Escape("plan [x]"), tview.Escape("proj]y")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key := model.AccountKey{Email: "a@x.com", Provider: "p"}
			s := model.NewSession(key, 1, tc.project, tc.topic, tc.queue)
			got := sessionItemText(s)
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("got %q, want it to contain %q", got, want)
				}
			}
		})
	}
}

func TestIsContinuePending(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		queue []model.Prompt
		want  bool
	}{
		{"empty queue", nil, false},
		{"last is continue", []model.Prompt{model.NewPrompt("continue", false)}, true},
		{"last is not continue", []model.Prompt{model.NewPrompt("do X", false)}, false},
		{"continue not last", []model.Prompt{model.NewPrompt("continue", false), model.NewPrompt("do X", false)}, false},
		{"continue with spawnsSession true does not match", []model.Prompt{model.NewPrompt("continue", true)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isContinuePending(tc.queue); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestToggleContinuePrompt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		queue    []model.Prompt
		wantLen  int
		wantLast string
	}{
		{"appends to empty", nil, 1, "continue"},
		{"removes trailing continue", []model.Prompt{model.NewPrompt("continue", false)}, 0, ""},
		{"appends after other prompt", []model.Prompt{model.NewPrompt("do X", false)}, 2, "continue"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := toggleContinuePrompt(tc.queue)
			if len(got) != tc.wantLen {
				t.Fatalf("length: got %d, want %d", len(got), tc.wantLen)
			}
			if tc.wantLen > 0 && got[len(got)-1].Text() != tc.wantLast {
				t.Errorf("last entry: got %q, want %q", got[len(got)-1].Text(), tc.wantLast)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"shorter than max unchanged", "hi", 10, "hi"},
		{"exactly max unchanged", "hello", 5, "hello"},
		{"longer truncated to max runes", "hello world", 6, "hello…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncate(tc.input, tc.max)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if len([]rune(got)) > tc.max {
				t.Errorf("result longer than max: %d > %d", len([]rune(got)), tc.max)
			}
		})
	}
}
