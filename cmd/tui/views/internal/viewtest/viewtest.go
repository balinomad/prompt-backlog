// Package viewtest provides shared test doubles, key events, and assertions
// for view tests.
package viewtest

import (
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"
	"github.com/balinomad/llm-desk/internal/store"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Must fails the test immediately when err is non-nil.
func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// fakeStore is a minimal in-memory store.Store.
var _ store.Store = (*fakeStore)(nil)

type fakeStore struct {
	reg *model.Registry
}

// Load returns the stored registry.
func (s *fakeStore) Load() (*model.Registry, error) {
	if s.reg == nil {
		return model.NewRegistry(), nil
	}
	return s.reg, nil
}

// Save stores reg.
func (s *fakeStore) Save(reg *model.Registry) error {
	s.reg = reg
	return nil
}

// NewService wraps reg in a service backed by an in-memory store.
func NewService(t *testing.T, reg *model.Registry) *service.Service {
	t.Helper()
	svc, err := service.New(&fakeStore{reg: reg})
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	return svc
}

// FakeApp implements views.Host and records calls for assertions.
type FakeApp struct {
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
	ClipboardText string
	ReadErr       error
	Quitted       bool
}

var _ views.Host = (*FakeApp)(nil)

// Service returns the fake service.
func (m *FakeApp) Service() *service.Service { return m.Svc }

// Mutate applies fn unless MutateErr is set.
func (m *FakeApp) Mutate(fn func(*service.Service) error) error {
	if m.MutateErr != nil {
		return m.MutateErr
	}
	return fn(m.Svc)
}

// ShowModal records p unless ShowModalErr is set.
func (m *FakeApp) ShowModal(p tview.Primitive) error {
	if m.ShowModalErr != nil {
		return m.ShowModalErr
	}
	m.Modals = append(m.Modals, p)
	m.Modal = p
	return nil
}

// HideModal pops the recorded modal.
func (m *FakeApp) HideModal() {
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
func (m *FakeApp) ReplaceTopModal(p tview.Primitive) error {
	if len(m.Modals) == 0 {
		m.Modals = append(m.Modals, p)
		m.Modal = p
		return nil
	}
	m.Modals[len(m.Modals)-1] = p
	m.Modal = p
	return nil
}

// SetFocus records p as the focused primitive and focuses it.
func (m *FakeApp) SetFocus(p tview.Primitive) {
	if m.Focused != nil && m.Focused != p {
		m.Focused.Blur()
	}
	m.Focused = p
	if p != nil {
		p.Focus(func(tview.Primitive) {})
	}
}

// SwitchToMain records the main screen.
func (m *FakeApp) SwitchToMain() { m.SwitchedTo = "main" }

// SwitchToVendors records the vendors screen.
func (m *FakeApp) SwitchToVendors() { m.SwitchedTo = "vendors" }

// SwitchToProjects records the projects screen.
func (m *FakeApp) SwitchToProjects() { m.SwitchedTo = "projects" }

// SwitchToUsers records the users screen.
func (m *FakeApp) SwitchToUsers() { m.SwitchedTo = "users" }

// SetStatus records msg and clears notice and error state.
func (m *FakeApp) SetStatus(msg string) {
	m.Status = msg
	m.Notice = ""
	m.ErrorStatus = ""
}

// SetNotice records msg and clears status and error state.
func (m *FakeApp) SetNotice(msg string) {
	m.Status = msg
	m.Notice = msg
	m.ErrorStatus = ""
}

// SetError records msg and clears status and notice state.
func (m *FakeApp) SetError(msg string) {
	m.Status = msg
	m.Notice = ""
	m.ErrorStatus = msg
}

// SetHints records the hint lines.
func (m *FakeApp) SetHints(specific, common string) {
	m.HintsSpecific = specific
	m.HintsCommon = common
}

// Quit records that quit was requested.
func (m *FakeApp) Quit() { m.Quitted = true }

// AppName returns the application name.
func (m *FakeApp) AppName() string { return "llm-desk" }

// CopyToClipboard records text and returns CopyErr.
func (m *FakeApp) CopyToClipboard(text string) error {
	m.CopiedText = text
	return m.CopyErr
}

// ReadFromClipboard returns ClipboardText and ReadErr.
func (m *FakeApp) ReadFromClipboard() (string, error) {
	return m.ClipboardText, m.ReadErr
}

// NewFakeApp returns a fake wired with the shared populated registry.
func NewFakeApp(t *testing.T) *FakeApp {
	t.Helper()
	reg := model.NewRegistry()
	Must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	Must(t, reg.AddVendor(model.NewVendor("chatgpt", false, false)))
	Must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	Must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))

	aliceClaude, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	Must(t, err)
	_, err = reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
	Must(t, err)

	Must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	Must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Alpha")))
	Must(t, reg.AddProject(model.NewProject("Beta", "", "")))
	Must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Beta")))
	Must(t, reg.AddProject(model.NewProject("Orphan", "", "")))

	return &FakeApp{Svc: NewService(t, reg)}
}

// RuneKey returns a key event for a printable rune.
func RuneKey(r rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)
}

// EscKey returns a key event for Escape.
func EscKey() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
}

// SpecialKey returns a key event for a non-rune key.
func SpecialKey(k tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(k, 0, tcell.ModNone)
}

// AssertHints fails when app's current hints differ from the expected pair.
func AssertHints(t *testing.T, app *FakeApp, wantSpecific, wantCommon string) {
	t.Helper()
	if app.HintsSpecific != wantSpecific || app.HintsCommon != wantCommon {
		t.Errorf("hints = (%q, %q), want (%q, %q)", app.HintsSpecific, app.HintsCommon, wantSpecific, wantCommon)
	}
}

// ConfirmModalFrom returns the *tview.Modal in p.
func ConfirmModalFrom(t *testing.T, p tview.Primitive) *tview.Modal {
	t.Helper()
	modal, ok := views.UnwrapModal(p).(*tview.Modal)
	if !ok {
		t.Fatalf("expected *tview.Modal, got %T", p)
	}
	return modal
}
