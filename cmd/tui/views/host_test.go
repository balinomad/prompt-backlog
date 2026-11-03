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

// must fails the test immediately when err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// mockStore is a minimal in-memory store.Store.
var _ store.Store = (*mockStore)(nil)

type mockStore struct {
	reg *model.Registry
}

// Load returns the stored registry.
func (s *mockStore) Load() (*model.Registry, error) {
	if s.reg == nil {
		return model.NewRegistry(), nil
	}
	return s.reg, nil
}

// Save stores reg.
func (s *mockStore) Save(reg *model.Registry) error {
	s.reg = reg
	return nil
}

// newTestService wraps reg in a service backed by an in-memory store.
func newTestService(t *testing.T, reg *model.Registry) *service.Service {
	t.Helper()
	svc, err := service.New(&mockStore{reg: reg})
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	return svc
}

// mockApp implements views.Host and records calls for assertions.
type mockApp struct {
	Svc           *service.Service
	MutateErr     error
	ShowModalErr  error
	Modal         tview.Primitive
	Modals        []tview.Primitive
	Focused       tview.Primitive
	SwitchedTo    string
	Status        string
	Notice        string
	ErrorStatus   string
	HintsSpecific string
	HintsCommon   string
	CopiedText    string
	CopyErr       error
	Quitted       bool
}

var _ views.Host = (*mockApp)(nil)

// Service returns the mock service.
func (m *mockApp) Service() *service.Service { return m.Svc }

// Mutate applies fn unless MutateErr is set.
func (m *mockApp) Mutate(fn func(*service.Service) error) error {
	if m.MutateErr != nil {
		return m.MutateErr
	}
	return fn(m.Svc)
}

// ShowModal records p unless ShowModalErr is set.
func (m *mockApp) ShowModal(p tview.Primitive) error {
	if m.ShowModalErr != nil {
		return m.ShowModalErr
	}
	m.Modals = append(m.Modals, p)
	m.Modal = p
	return nil
}

// HideModal pops the recorded modal.
func (m *mockApp) HideModal() {
	if len(m.Modals) == 0 {
		m.Modal = nil
		return
	}
	m.Modals = m.Modals[:len(m.Modals)-1]
	if len(m.Modals) == 0 {
		m.Modal = nil
		return
	}
	m.Modal = m.Modals[len(m.Modals)-1]
}

// ReplaceTopModal replaces the recorded top modal.
func (m *mockApp) ReplaceTopModal(p tview.Primitive) error {
	if len(m.Modals) == 0 {
		m.Modals = append(m.Modals, p)
		m.Modal = p
		return nil
	}
	m.Modals[len(m.Modals)-1] = p
	m.Modal = p
	return nil
}

// SetFocus records p as the focused primitive.
func (m *mockApp) SetFocus(p tview.Primitive) { m.Focused = p }

// SwitchToMain records the main screen.
func (m *mockApp) SwitchToMain() { m.SwitchedTo = "main" }

// SwitchToVendors records the vendors screen.
func (m *mockApp) SwitchToVendors() { m.SwitchedTo = "vendors" }

// SwitchToProjects records the projects screen.
func (m *mockApp) SwitchToProjects() { m.SwitchedTo = "projects" }

// SwitchToUsers records the users screen.
func (m *mockApp) SwitchToUsers() { m.SwitchedTo = "users" }

// SetStatus records msg and clears notice and error state.
func (m *mockApp) SetStatus(msg string) {
	m.Status = msg
	m.Notice = ""
	m.ErrorStatus = ""
}

// SetNotice records msg and clears status and error state.
func (m *mockApp) SetNotice(msg string) {
	m.Status = msg
	m.Notice = msg
	m.ErrorStatus = ""
}

// SetError records msg and clears status and notice state.
func (m *mockApp) SetError(msg string) {
	m.Status = msg
	m.Notice = ""
	m.ErrorStatus = msg
}

// SetHints records the hint lines.
func (m *mockApp) SetHints(specific, common string) {
	m.HintsSpecific = specific
	m.HintsCommon = common
}

// Quit records that quit was requested.
func (m *mockApp) Quit() { m.Quitted = true }

// AppName returns the application name.
func (m *mockApp) AppName() string { return "llm-desk" }

// CopyToClipboard records text and returns CopyErr.
func (m *mockApp) CopyToClipboard(text string) error {
	m.CopiedText = text
	return m.CopyErr
}

// assertHints fails when app's current hints differ from the expected pair.
func assertHints(t *testing.T, app *mockApp, wantSpecific, wantCommon string) {
	t.Helper()
	if app.HintsSpecific != wantSpecific || app.HintsCommon != wantCommon {
		t.Errorf("hints = (%q, %q), want (%q, %q)", app.HintsSpecific, app.HintsCommon, wantSpecific, wantCommon)
	}
}

// confirmModalFrom returns the *tview.Modal in p.
func confirmModalFrom(t *testing.T, p tview.Primitive) *tview.Modal {
	t.Helper()
	modal, ok := views.UnwrapModal(p).(*tview.Modal)
	if !ok {
		t.Fatalf("expected *tview.Modal, got %T", p)
	}
	return modal
}

// newMockApp returns a mock wired with the shared populated registry.
func newMockApp(t *testing.T) *mockApp {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddVendor(model.NewVendor("chatgpt", false, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))

	aliceClaude, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	_, err = reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
	must(t, err)

	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Alpha")))
	must(t, reg.AddProject(model.NewProject("Beta", "", "")))
	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Beta")))
	must(t, reg.AddProject(model.NewProject("Orphan", "", "")))

	return &mockApp{Svc: newTestService(t, reg)}
}

// runeKey returns a key event for a printable rune.
func runeKey(r rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)
}

// escKey returns a key event for Escape.
func escKey() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
}
