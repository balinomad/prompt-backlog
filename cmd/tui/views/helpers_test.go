package views

import (
	"testing"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── must ──────────────────────────────────────────────────────────────────────

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── mockApp ───────────────────────────────────────────────────────────────────

// mockApp implements App and records every call for assertion in tests.
type mockApp struct {
	registry   model.Registry
	mutateErr  error
	modal      tview.Primitive
	switchedTo string // "main" | "providers"
	status     string
	hints      string
	quitted    bool
}

var _ App = (*mockApp)(nil)

func (m *mockApp) Registry() model.Registry { return m.registry }

func (m *mockApp) Mutate(fn func(*model.Registry) error) error {
	if m.mutateErr != nil {
		return m.mutateErr
	}
	return fn(&m.registry)
}

func (m *mockApp) ShowModal(p tview.Primitive) { m.modal = p }
func (m *mockApp) HideModal()                  { m.modal = nil }
func (m *mockApp) SwitchToMain()               { m.switchedTo = "main" }
func (m *mockApp) SwitchToProviders()          { m.switchedTo = "providers" }
func (m *mockApp) SetStatus(msg string)        { m.status = msg }
func (m *mockApp) SetHints(hints string)       { m.hints = hints }
func (m *mockApp) Quit()                       { m.quitted = true }

// newMockApp returns a mock wired with a fully populated registry.
//
//	Providers: "claude" (HasProjects=true), "chatgpt" (HasProjects=false)
//	Users:     alice@example.com / Alice, bob@example.com / Bob
//	Sessions:  alice → claude, bob → chatgpt
//	Projects:  "Alpha" and "Beta" (attached: alice/claude), "Orphan" (no sessions)
func newMockApp(t *testing.T) *mockApp {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddProvider(model.NewProvider("chatgpt", false, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("alice@example.com"),
		reg.ProviderByName("claude"),
	)))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("bob@example.com"),
		reg.ProviderByName("chatgpt"),
	)))
	must(t, reg.AddProject(model.NewProject("Alpha")))
	must(t, reg.AttachProject("Alpha", reg.SessionAt("alice@example.com", "claude")))
	must(t, reg.AddProject(model.NewProject("Beta")))
	must(t, reg.AttachProject("Beta", reg.SessionAt("alice@example.com", "claude")))
	must(t, reg.AddProject(model.NewProject("Orphan")))
	return &mockApp{registry: reg}
}

// runeKey returns a key event for a printable rune.
func runeKey(r rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)
}

// escKey returns a key event for the Escape key.
func escKey() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
}
