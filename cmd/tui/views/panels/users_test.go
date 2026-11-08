package panels_test

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views/panels"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func usersPanelParts(t *testing.T, p *panels.UsersPanel) (*tview.List, *tview.TextView) {
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

func TestNewUsersPanel(t *testing.T) {
	t.Parallel()
	p := panels.NewUsersPanel(newFakeApp(t))
	if p == nil {
		t.Fatal("expected non-nil UsersPanel")
	}
	_, info := usersPanelParts(t, p)
	if got := info.GetTitle(); got != " User Details " {
		t.Errorf("GetTitle() = %q, want %q", got, " User Details ")
	}
}

func TestUsersPanel_Refresh_SortedByName(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewUsersPanel(app)
	p.Refresh()
	list, _ := usersPanelParts(t, p)

	if got := list.GetItemCount(); got != 2 {
		t.Fatalf("item count: got %d, want 2", got)
	}
	row0, _ := list.GetItemText(0)
	row1, _ := list.GetItemText(1)
	if !strings.Contains(row0, "Alice") || !strings.Contains(row0, "alice@example.com") {
		t.Errorf("row 0 = %q, want Alice and alice@example.com", row0)
	}
	if !strings.Contains(row1, "Bob") || !strings.Contains(row1, "bob@example.com") {
		t.Errorf("row 1 = %q, want Bob and bob@example.com", row1)
	}
}

func TestUsersPanel_RowText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		userName  string
		email     string
		wantEqual string
		wantParts []string
	}{
		{
			name:      "normal",
			userName:  "Alice",
			email:     "alice@example.com",
			wantEqual: " Alice │ alice@example.com ",
		},
		{
			name:      "long values truncated",
			userName:  "abcdefghijklmnopqrstuvwxyz",
			email:     "abcdefghijklmnopqrstuvwxyz@example.com",
			wantEqual: " abcdefghijklmnopqrs… │ abcdefghijklmnopqrstuvwxyz@exam… ",
		},
		{
			name:      "brackets escaped",
			userName:  "A[red]",
			email:     "b]ob@example.com",
			wantParts: []string{tview.Escape("A[red]"), tview.Escape("b]ob@example.com")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddUser(model.NewUser(tc.email, tc.userName)))
			app := &fakeApp{Svc: newService(t, reg)}
			p := panels.NewUsersPanel(app)
			p.Refresh()
			list, _ := usersPanelParts(t, p)
			got, _ := list.GetItemText(0)
			if tc.wantEqual != "" && got != tc.wantEqual {
				t.Errorf("row = %q, want %q", got, tc.wantEqual)
			}
			for _, want := range tc.wantParts {
				if !strings.Contains(got, want) {
					t.Errorf("row = %q, want it to contain %q", got, want)
				}
			}
		})
	}
}

func TestUsersPanel_InfoPanel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewUsersPanel(app)
	p.Refresh()
	_, info := usersPanelParts(t, p)
	got := info.GetText(false)
	for _, want := range []string{"Name", "Email", "Alice", "alice@example.com"} {
		if !strings.Contains(got, want) {
			t.Errorf("info = %q, want it to contain %q", got, want)
		}
	}
}

func TestUsersPanel_New_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewUsersPanel(app)
	p.Refresh()
	list, _ := usersPanelParts(t, p)

	list.GetInputCapture()(specialKey(views.KeyInsert))
	form := formFromCenteredModal(t, app.Modal)
	setInputField(t, form, "Email", "carol@example.com")
	setInputField(t, form, "Name", "Carol")
	triggerSave(t, form)

	if app.Modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsUsersPanelSpecific, views.HintsUsersPanelCommon)
	if app.Svc.User("carol@example.com") == nil {
		t.Error("user not created")
	}
}

func TestUsersPanel_Edit_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewUsersPanel(app)
	p.Refresh()
	list, _ := usersPanelParts(t, p)

	list.GetInputCapture()(specialKey(views.KeyEdit))
	form := formFromCenteredModal(t, app.Modal)
	setInputField(t, form, "Name", "Alice Smith")
	triggerSave(t, form)

	if app.Modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsUsersPanelSpecific, views.HintsUsersPanelCommon)
	if got := app.Svc.User("alice@example.com").Name(); got != "Alice Smith" {
		t.Errorf("Name() = %q, want %q", got, "Alice Smith")
	}
}

func TestUsersPanel_Delete_Cancel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewUsersPanel(app)
	p.Refresh()
	list, _ := usersPanelParts(t, p)
	before := len(app.Svc.Users())

	list.GetInputCapture()(specialKey(views.KeyDelete))
	confirmModal := confirmModalFrom(t, app.Modal)
	confirmModal.GetInputCapture()(escKey())

	if app.Modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	assertHints(t, app, views.HintsUsersPanelSpecific, views.HintsUsersPanelCommon)
	if got := len(app.Svc.Users()); got != before {
		t.Errorf("user count changed after cancel: got %d, want %d (unchanged)", got, before)
	}
}

func TestUsersPanel_Delete(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	must(t, app.Svc.AddUser("carol@example.com", "Carol"))
	p := panels.NewUsersPanel(app)
	p.Refresh()
	list, _ := usersPanelParts(t, p)
	list.SetCurrentItem(2)

	list.GetInputCapture()(specialKey(views.KeyDelete))
	confirmModal := confirmModalFrom(t, app.Modal)
	confirmModal.SetFocus(1)
	confirmModal.InputHandler()(specialKey(tcell.KeyEnter), func(tview.Primitive) {})

	if app.Svc.User("carol@example.com") != nil {
		t.Error("user still exists after confirming deletion")
	}
	if app.Modal != nil {
		t.Error("expected modal to be hidden after deletion")
	}
	assertHints(t, app, views.HintsUsersPanelSpecific, views.HintsUsersPanelCommon)
}

func TestUsersPanel_Keys_NoHiddenAliases(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewUsersPanel(app)
	p.Refresh()
	list, _ := usersPanelParts(t, p)

	for _, key := range []rune{views.KeyVendors, views.KeyProjects, views.KeyUsers} {
		event := tcell.NewEventKey(tcell.KeyRune, key, tcell.ModNone)
		if got := list.GetInputCapture()(event); got != event {
			t.Errorf("rune %q must pass through unchanged", key)
		}
		if app.SwitchedTo != "" {
			t.Errorf("rune %q must not switch screens, got %q", key, app.SwitchedTo)
		}
	}
}
