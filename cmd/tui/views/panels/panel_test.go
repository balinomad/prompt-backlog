package panels_test

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views/panels"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"
	"github.com/balinomad/llm-desk/internal/store"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type panelContract interface {
	tview.Primitive
	Refresh()
	RequiredInfoHeight(int) int
	ResizeInfo(int, int)
	GetItem(int) tview.Primitive
}

func panelParts(t *testing.T, p panelContract) (*tview.List, *tview.TextView) {
	t.Helper()
	list, ok := p.GetItem(0).(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List at index 0, got %T", p.GetItem(0))
	}
	info, ok := p.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 1, got %T", p.GetItem(1))
	}
	return list, info
}

func assertKeyOpensModal(t *testing.T, newPanel func(*mockApp) panelContract, key *tcell.EventKey, wantSpecific, wantCommon string) {
	t.Helper()
	app := newMockApp(t)
	p := newPanel(app)
	p.Refresh()
	list, _ := panelParts(t, p)
	list.GetInputCapture()(key)
	if app.Modal == nil {
		t.Fatal("expected a modal")
	}
	assertHints(t, app, wantSpecific, wantCommon)
}

func runPanelContract(t *testing.T, newPanel func(*mockApp) panelContract, wantItems int, wantInfoTitle string) {
	t.Helper()

	t.Run("refresh populates list", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		list, info := panelParts(t, p)
		if got := list.GetItemCount(); got != wantItems {
			t.Errorf("item count: got %d, want %d", got, wantItems)
		}
		if got := info.GetTitle(); got != wantInfoTitle {
			t.Errorf("info title = %q, want %q", got, wantInfoTitle)
		}
	})

	t.Run("selection restored by key", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		list, _ := panelParts(t, p)
		if list.GetItemCount() < 2 {
			t.Skip("needs at least two items")
		}
		list.SetCurrentItem(1)
		want, _ := list.GetItemText(1)
		p.Refresh()
		got, _ := list.GetItemText(list.GetCurrentItem())
		if got != want {
			t.Errorf("selection changed: got %q, want %q", got, want)
		}
	})

	t.Run("esc switches to main", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		list, _ := panelParts(t, p)
		list.GetInputCapture()(escKey())
		if app.SwitchedTo != "main" {
			t.Errorf("switchedTo = %q, want main", app.SwitchedTo)
		}
	})

	t.Run("tab consumed", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		list, _ := panelParts(t, p)
		if got := list.GetInputCapture()(specialKey(views.KeyFocusNext)); got != nil {
			t.Error("Tab must be consumed")
		}
	})

	t.Run("insert opens form", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, specialKey(views.KeyInsert), views.HintsFormSpecific, views.HintsFormCommon)
	})

	t.Run("enter opens form", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsFormCommon)
	})

	t.Run("delete opens confirm", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	})

	t.Run("help opens overlay", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, runeKey(views.KeyHelp), views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon)
	})

	t.Run("quit opens confirm", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, runeKey(views.KeyQuit), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	})

	t.Run("empty list guards", func(t *testing.T) {
		t.Parallel()
		app := &mockApp{Svc: newTestService(t, model.NewRegistry())}
		p := newPanel(app)
		p.Refresh()
		list, _ := panelParts(t, p)
		if got := list.GetItemCount(); got != 0 {
			t.Fatalf("item count: got %d, want 0", got)
		}
		list.GetInputCapture()(specialKey(views.KeyDelete))
		if app.Modal != nil {
			t.Error("Del on an empty list must not open a modal")
		}
		list.GetInputCapture()(specialKey(views.KeyEdit))
		if app.Modal != nil {
			t.Error("Enter on an empty list must not open a modal")
		}
	})

	t.Run("info resize", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		if got := p.RequiredInfoHeight(80); got <= 0 {
			t.Errorf("RequiredInfoHeight(80) = %d, want positive", got)
		}
		p.ResizeInfo(80, 24)
	})
}

func TestVendorsPanel_Contract(t *testing.T) {
	t.Parallel()
	runPanelContract(t, func(app *mockApp) panelContract {
		return panels.NewVendorsPanel(app)
	}, 2, " Vendor Details ")
}

func TestUsersPanel_Contract(t *testing.T) {
	t.Parallel()
	runPanelContract(t, func(app *mockApp) panelContract {
		return panels.NewUsersPanel(app)
	}, 2, " User Details ")
}

func TestPanels_DoubleClickOpensEditForm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		newPanel  func(*mockApp) panelContract
		wantTitle string
	}{
		{
			name: "vendors",
			newPanel: func(app *mockApp) panelContract {
				return panels.NewVendorsPanel(app)
			},
			wantTitle: "Edit Vendor",
		},
		{
			name: "users",
			newPanel: func(app *mockApp) panelContract {
				return panels.NewUsersPanel(app)
			},
			wantTitle: "Edit User",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := newMockApp(t)
			p := tc.newPanel(app)
			p.Refresh()
			list, _ := panelParts(t, p)

			doubleClickList(t, list)

			form := formFromCenteredModal(t, app.Modal)
			if got := form.GetTitle(); !strings.Contains(got, tc.wantTitle) {
				t.Errorf("title = %q, want it to contain %q", got, tc.wantTitle)
			}
		})
	}
}

func TestListTitles_Panels(t *testing.T) {
	t.Parallel()
	t.Run("vendors", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := panels.NewVendorsPanel(app)
		p.Refresh()
		list, _ := vendorsPanelParts(t, p)
		if got, want := list.GetTitle(), " Vendors 1/2 "; got != want {
			t.Errorf("initial title = %q, want %q", got, want)
		}
		list.SetCurrentItem(1)
		if got, want := list.GetTitle(), " Vendors 2/2 "; got != want {
			t.Errorf("selected title = %q, want %q", got, want)
		}
		app.Svc = newTestService(t, model.NewRegistry())
		p.Refresh()
		if got, want := list.GetTitle(), " Vendors 0/0 "; got != want {
			t.Errorf("empty title = %q, want %q", got, want)
		}
	})

	t.Run("users", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := panels.NewUsersPanel(app)
		p.Refresh()
		list, _ := usersPanelParts(t, p)
		if got, want := list.GetTitle(), " Users 1/2 "; got != want {
			t.Errorf("initial title = %q, want %q", got, want)
		}
		list.SetCurrentItem(1)
		if got, want := list.GetTitle(), " Users 2/2 "; got != want {
			t.Errorf("selected title = %q, want %q", got, want)
		}
		app.Svc = newTestService(t, model.NewRegistry())
		p.Refresh()
		if got, want := list.GetTitle(), " Users 0/0 "; got != want {
			t.Errorf("empty title = %q, want %q", got, want)
		}
	})
}

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
func (m *mockApp) SetFocus(p tview.Primitive) {
	if m.Focused != nil && m.Focused != p {
		m.Focused.Blur()
	}
	m.Focused = p
	if p != nil {
		p.Focus(func(tview.Primitive) {})
	}
}

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

// specialKey returns a key event for a non-rune key.
func specialKey(k tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(k, 0, tcell.ModNone)
}

// formFromCenteredModal returns the form inside a centered modal wrapper.
func formFromCenteredModal(t *testing.T, p tview.Primitive) *tview.Form {
	t.Helper()
	p = views.UnwrapModal(p)
	outer, ok := p.(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex, got %T", p)
	}
	column, ok := outer.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected column *tview.Flex, got %T", outer.GetItem(1))
	}
	row, ok := column.GetItem(2).(*tview.Flex)
	if !ok {
		t.Fatalf("expected row *tview.Flex, got %T", column.GetItem(2))
	}
	form, ok := row.GetItem(2).(*tview.Form)
	if !ok {
		t.Fatalf("expected *tview.Form, got %T", row.GetItem(2))
	}
	return form
}

// setInputField sets an input field's text.
func setInputField(t *testing.T, form *tview.Form, label, text string) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	field, ok := item.(*tview.InputField)
	if !ok {
		t.Fatalf("expected %q to be *tview.InputField, got %T", label, item)
	}
	field.SetText(text)
}

// setTextArea sets a text area's text.
func setTextArea(t *testing.T, form *tview.Form, label, text string) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	field, ok := item.(*tview.TextArea)
	if !ok {
		t.Fatalf("expected %q to be *tview.TextArea, got %T", label, item)
	}
	field.SetText(text, true)
}

// setCheckbox checks a checkbox.
func setCheckbox(t *testing.T, form *tview.Form, label string) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	cb, ok := item.(*tview.Checkbox)
	if !ok {
		t.Fatalf("expected %q to be *tview.Checkbox, got %T", label, item)
	}
	cb.SetChecked(true)
}

// setDropDown selects a dropdown option.
func setDropDown(t *testing.T, form *tview.Form, label string, index int) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	dd, ok := item.(*tview.DropDown)
	if !ok {
		t.Fatalf("expected %q to be *tview.DropDown, got %T", label, item)
	}
	dd.SetCurrentOption(index)
}

// triggerSave invokes the form's Ctrl+S capture.
func triggerSave(t *testing.T, form *tview.Form) {
	t.Helper()
	capture := form.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil form input capture (addCtrlS)")
	}
	capture(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone))
}

// dropdownOption returns the display text of one dropdown option.
func dropdownOption(t *testing.T, dropdown *tview.DropDown, index int) string {
	t.Helper()
	dropdown.SetCurrentOption(index)
	_, option := dropdown.GetCurrentOption()
	return option
}

// doubleClickList invokes list's mouse capture with a double click.
func doubleClickList(t *testing.T, list *tview.List) {
	t.Helper()
	list.SetRect(0, 0, 40, 10)
	capture := list.GetMouseCapture()
	if capture == nil {
		t.Fatal("list has no mouse capture")
	}
	event := tcell.NewEventMouse(2, 2, tcell.ButtonPrimary, tcell.ModNone)
	action, gotEvent := capture(tview.MouseLeftDoubleClick, event)
	if action != tview.MouseConsumed || gotEvent != nil {
		t.Fatalf("mouse capture = (%v, %v), want (%v, nil)", action, gotEvent, tview.MouseConsumed)
	}
}
