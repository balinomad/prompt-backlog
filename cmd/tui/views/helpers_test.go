package views_test

import (
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"
	"github.com/balinomad/llm-desk/internal/store"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── must ──────────────────────────────────────────────────────────────────────

// must fails the test immediately when err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── mockStore ─────────────────────────────────────────────────────────────────

// mockStore is a minimal in-memory store.Store, used only to construct a
// working *service.Service for tests without touching the filesystem.
var _ store.Store = (*mockStore)(nil)

type mockStore struct {
	reg *model.Registry
}

func (s *mockStore) Load() (*model.Registry, error) {
	if s.reg == nil {
		return model.NewRegistry(), nil
	}
	return s.reg, nil
}

func (s *mockStore) Save(reg *model.Registry) error {
	s.reg = reg
	return nil
}

// newTestService wraps reg in a *service.Service via mockStore.
func newTestService(t *testing.T, reg *model.Registry) *service.Service {
	t.Helper()
	svc, err := service.New(&mockStore{reg: reg})
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	return svc
}

// ── mockApp ───────────────────────────────────────────────────────────────────

// mockApp implements App and records every call for assertion in tests.
type mockApp struct {
	svc          *service.Service
	mutateErr    error
	showModalErr error
	modal        tview.Primitive
	switchedTo   string // "main" | "providers" | "projects"
	status       string
	hints        string
	quitted      bool
}

var _ views.App = (*mockApp)(nil)

func (m *mockApp) Service() *service.Service { return m.svc }

func (m *mockApp) Mutate(fn func(*service.Service) error) error {
	if m.mutateErr != nil {
		return m.mutateErr
	}
	return fn(m.svc)
}

func (m *mockApp) ShowModal(p tview.Primitive) error {
	if m.showModalErr != nil {
		return m.showModalErr
	}
	m.modal = p
	return nil
}

func (m *mockApp) HideModal()            { m.modal = nil }
func (m *mockApp) SwitchToMain()         { m.switchedTo = "main" }
func (m *mockApp) SwitchToProviders()    { m.switchedTo = "providers" }
func (m *mockApp) SwitchToProjects()     { m.switchedTo = "projects" }
func (m *mockApp) SetStatus(msg string)  { m.status = msg }
func (m *mockApp) SetHints(hints string) { m.hints = hints }
func (m *mockApp) Quit()                 { m.quitted = true }
func (m *mockApp) AppName() string       { return "llm-desk" }

// newMockApp returns a mock wired with a fully populated registry.
// Deliberately excludes Sessions: their shape is being redesigned (account-
// scoped, multiple per account) — adding placeholder session data now would
// just be guessed and redone when sessions.go/detail.go are rewritten.
//
//	Providers: "claude" (HasProjects=true), "chatgpt" (HasProjects=false)
//	Users:     alice@example.com / Alice, bob@example.com / Bob
//	Accounts:  alice|claude, bob|chatgpt
//	Projects:  "Alpha" and "Beta" (granted to alice|claude), "Orphan" (no accounts)
func newMockApp(t *testing.T) *mockApp {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddProvider(model.NewProvider("chatgpt", false, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))

	aliceClaude, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	_, err = reg.AddAccount(reg.User("bob@example.com"), reg.Provider("chatgpt"))
	must(t, err)

	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Alpha")))
	must(t, reg.AddProject(model.NewProject("Beta", "", "")))
	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Beta")))
	must(t, reg.AddProject(model.NewProject("Orphan", "", "")))

	return &mockApp{svc: newTestService(t, reg)}
}

// runeKey returns a key event for a printable rune.
func runeKey(r rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)
}

// escKey returns a key event for the Escape key.
func escKey() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
}

// specialKey returns a key event for a non-rune special key (Enter,
// Delete, F2, etc.) — anything checked via event.Key() rather than
// event.Rune().
func specialKey(k tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(k, 0, tcell.ModNone)
}
