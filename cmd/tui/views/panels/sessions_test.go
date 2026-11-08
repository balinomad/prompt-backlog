package panels_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/cmd/tui/views/panels"

	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type sessionsFixture struct {
	app *fakeApp
	reg *model.Registry
	acc *model.Account
	s1  *model.Session
	s2  *model.Session
	s3  *model.Session
}

func itemText(t *testing.T, p *panels.SessionsPanel, index int) string {
	t.Helper()
	main, _ := p.GetItemText(index)
	return main
}

func newSessionsFixture(t *testing.T) sessionsFixture {
	t.Helper()
	reg := model.NewRegistry()
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	reg.SetClock(func() time.Time { return now })

	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddVendor(model.NewVendor("chatgpt", false, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProject(model.NewProject("Beta", "", "")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
	must(t, reg.AddProjectToAccount(acc, reg.Project("Beta")))
	_, err = reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
	must(t, err)

	now = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	s1, err := reg.AddSession(acc, reg.Project("Alpha"), "Alpha session", []model.Prompt{model.NewPrompt("first prompt")})
	must(t, err)
	now = time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	s2, err := reg.AddSession(acc, reg.Project("Beta"), "Beta session", []model.Prompt{model.NewPrompt("beta one"), model.NewPrompt("beta two")})
	must(t, err)
	s3, err := reg.AddSession(acc, nil, "", nil)
	must(t, err)

	return sessionsFixture{
		app: &fakeApp{Svc: newService(t, reg)},
		reg: reg,
		acc: acc,
		s1:  s1,
		s2:  s2,
		s3:  s3,
	}
}

func (f sessionsFixture) panel(t *testing.T) *panels.SessionsPanel {
	t.Helper()
	p := panels.NewSessionsPanel(f.app)
	p.SetAccount(f.acc)
	return p
}

func TestSessionsPanel_Rows(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)

	if got := p.GetItemCount(); got != 3 {
		t.Fatalf("rows = %d, want 3", got)
	}
	first := itemText(t, p, 0)
	if !strings.Contains(first, "(untitled)") || !strings.Contains(first, "0") {
		t.Errorf("first row = %q", first)
	}
	second := itemText(t, p, 1)
	if !strings.Contains(second, "Beta") || !strings.Contains(second, "Beta session") || !strings.Contains(second, "2") {
		t.Errorf("second row = %q", second)
	}
	third := itemText(t, p, 2)
	if !strings.Contains(third, "Alpha") || !strings.Contains(third, "Alpha session") || !strings.Contains(third, "1") {
		t.Errorf("third row = %q", third)
	}
	if got := p.GetTitle(); !strings.Contains(got, "Sessions") || !strings.Contains(got, "1/3") {
		t.Errorf("title = %q, want Sessions 1/3", got)
	}
}

func TestSessionsPanel_ActivityOrder(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)
	if !strings.Contains(itemText(t, p, 0), "(untitled)") {
		t.Errorf("first row = %q, want untitled", itemText(t, p, 0))
	}
	if !strings.Contains(itemText(t, p, 1), "Beta session") {
		t.Errorf("second row = %q, want Beta", itemText(t, p, 1))
	}
	if !strings.Contains(itemText(t, p, 2), "Alpha session") {
		t.Errorf("third row = %q, want Alpha", itemText(t, p, 2))
	}
}

func TestSessionsPanel_PromptSaveMovesSessionToTop(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	f.reg.SetClock(func() time.Time { return time.Date(2026, 10, 30, 11, 0, 0, 0, time.UTC) })
	must(t, f.app.Svc.UpdateSessionPrompts("alice@example.com", "claude", f.s1.ID(), []model.Prompt{model.NewPrompt("saved")}))

	p := f.panel(t)
	if row := itemText(t, p, 0); !strings.Contains(row, "Alpha session") {
		t.Errorf("first row = %q, want Alpha session", row)
	}
}

func TestSessionsPanel_StatusMarkers(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)
	must(t, f.reg.UpdateSessionStatus(f.s2, model.SessionStatusNeedsClosing))
	must(t, f.reg.UpdateSessionStatus(f.s1, model.SessionStatusClosed))
	p.Refresh()

	first := itemText(t, p, 0)
	if strings.Contains(first, "▲") || strings.Contains(first, "✓") {
		t.Errorf("active row = %q, want no marker", first)
	}
	second := itemText(t, p, 1)
	if !strings.Contains(second, "2") || !strings.Contains(second, "[yellow]▲[-]") {
		t.Errorf("needs-closing row = %q, want count and yellow triangle", second)
	}
	if strings.Index(second, "▲") < strings.Index(second, "2") {
		t.Errorf("status marker = %q, want marker after count", second)
	}
	third := itemText(t, p, 2)
	if !strings.HasPrefix(third, "[gray]") || !strings.HasSuffix(third, "[-]") || !strings.Contains(third, "✓") {
		t.Errorf("closed row = %q, want the whole row gray with a check marker", third)
	}
}

func TestSessionsPanel_SessionNameUsesAvailableWidth(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	long := strings.Repeat("x", 60)
	must(t, f.app.Svc.UpdateSessionName("alice@example.com", "claude", f.s1.ID(), long))

	p := f.panel(t)
	p.SetWidth(80)
	if row := itemText(t, p, 2); !strings.Contains(row, long) {
		t.Errorf("row = %q, want the full session name at width 80", row)
	}

	p.SetWidth(30)
	row := itemText(t, p, 2)
	if strings.Contains(row, long) {
		t.Errorf("row = %q, want the session name truncated at width 30", row)
	}
	if got := tview.TaggedStringWidth(row); got > 30 {
		t.Errorf("row width = %d, want at most 30", got)
	}
}

func TestSessionsPanel_ProjectColumnHidden(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	filter := views.NewProjectFilterState()
	p := panels.NewSessionsPanel(f.app)
	p.SetFilter(filter)
	p.SetAccount(f.acc)

	filter.Cycle() // Alpha
	filter.Cycle() // Beta
	filter.Cycle() // None
	p.Refresh()

	row := itemText(t, p, 0)
	if strings.Count(row, "│") != 1 {
		t.Errorf("row = %q, want no project column", row)
	}
	if !strings.Contains(row, "(untitled)") || !strings.Contains(row, "0") {
		t.Errorf("row = %q", row)
	}
}

func TestSessionsPanel_SelectionPreserved(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)
	p.SetCurrentItem(1)
	p.Refresh()
	if got := p.GetCurrentItem(); got != 1 {
		t.Errorf("selection = %d, want 1", got)
	}

	must(t, f.reg.DeleteSession(f.s2))
	p.Refresh()
	if got := p.GetCurrentItem(); got != 0 {
		t.Errorf("selection = %d, want 0 after removing the selected session", got)
	}
}

func TestSessionsPanel_Filter(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	filter := views.NewProjectFilterState()
	p := panels.NewSessionsPanel(f.app)
	p.SetFilter(filter)
	p.SetAccount(f.acc)

	filter.Cycle() // Alpha
	p.Refresh()
	if got := p.GetItemCount(); got != 1 || !strings.Contains(itemText(t, p, 0), "Alpha session") {
		t.Errorf("Alpha filter rows = %d first = %q", got, itemText(t, p, 0))
	}

	filter.Cycle() // Beta
	p.Refresh()
	if got := p.GetItemCount(); got != 1 || !strings.Contains(itemText(t, p, 0), "Beta session") {
		t.Errorf("Beta filter rows = %d first = %q", got, itemText(t, p, 0))
	}

	filter.Cycle() // None
	p.Refresh()
	if got := p.GetItemCount(); got != 1 || !strings.Contains(itemText(t, p, 0), "(untitled)") {
		t.Errorf("None filter rows = %d first = %q", got, itemText(t, p, 0))
	}
}

func TestSessionsPanel_NoAccount(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := panels.NewSessionsPanel(f.app)
	p.SetAccount(nil)
	if got := p.GetItemCount(); got != 0 {
		t.Errorf("rows = %d, want 0", got)
	}
	if got := p.GetTitle(); !strings.Contains(got, "0/0") {
		t.Errorf("title = %q, want 0/0", got)
	}
}

func TestSessionsPanel_Enter(t *testing.T) {
	t.Parallel()

	t.Run("single prompt opens edit form", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)
		p.SetCurrentItem(2)
		p.GetInputCapture()(specialKey(views.KeyEdit))
		if f.app.Modal == nil {
			t.Fatal("Enter did not open the prompt form")
		}
	})

	t.Run("empty session opens add form", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)
		p.SetCurrentItem(0)
		p.GetInputCapture()(specialKey(views.KeyEdit))
		if f.app.Modal == nil {
			t.Fatal("Enter did not open the add prompt form")
		}
	})

	t.Run("multiple prompts temporarily open the head prompt", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)
		p.SetCurrentItem(1)
		p.GetInputCapture()(specialKey(views.KeyEdit))
		if f.app.Modal == nil {
			t.Fatal("Enter did not open the head prompt form")
		}
	})
}

func TestSessionsPanel_Insert(t *testing.T) {
	t.Parallel()

	t.Run("no account does nothing", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)
		p.SetAccount(nil)
		p.GetInputCapture()(specialKey(views.KeyInsert))
		if f.app.Modal != nil {
			t.Error("Ins opened a modal without an account")
		}
	})

	t.Run("opens the start form with no rows", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)
		bob, err := f.app.Svc.Account("bob@example.com", "chatgpt")
		must(t, err)
		p.SetAccount(bob)
		p.GetInputCapture()(specialKey(views.KeyInsert))
		form := formFromCenteredModal(t, f.app.Modal)
		if got := form.GetTitle(); got != " Start Session " {
			t.Errorf("title = %q, want Start Session", got)
		}
	})

	t.Run("opens the start form with a selected row", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)
		p.SetCurrentItem(1)
		p.GetInputCapture()(specialKey(views.KeyInsert))
		form := formFromCenteredModal(t, f.app.Modal)
		if got := form.GetTitle(); got != " Start Session " {
			t.Errorf("title = %q, want Start Session", got)
		}
	})
}

func TestSessionsPanel_Delete(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)
	p.SetCurrentItem(2)
	p.GetInputCapture()(specialKey(views.KeyDelete))
	confirmModal(t, f.app)
	p.Refresh()
	if f.app.Svc.Session(f.s1.ID()) != nil {
		t.Error("session still exists after confirming end")
	}
}

func TestSessionsPanel_EscOpensQuitConfirmation(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)
	p.GetInputCapture()(specialKey(views.KeyCancel))
	if f.app.Modal == nil {
		t.Fatal("Esc did not open the quit confirmation")
	}
}

func TestSessionsPanel_FilterKeys(t *testing.T) {
	t.Parallel()

	t.Run("f advances the filter and calls the callback", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)
		called := false
		p.SetOnFilterChange(func() {
			called = true
			p.Refresh()
		})
		p.GetInputCapture()(runeKey(views.KeyFilter))
		if !called {
			t.Error("filter callback was not called")
		}
		if got := p.GetTitle(); !strings.Contains(got, "(Alpha)") {
			t.Errorf("title = %q, want (Alpha)", got)
		}
	})

	t.Run("f without an account cycles to None", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)
		p.SetAccount(nil)
		p.GetInputCapture()(runeKey(views.KeyFilter))
		if got := p.GetTitle(); !strings.Contains(got, "(no project)") {
			t.Errorf("title = %q, want (no project)", got)
		}
	})
}

func TestSessionsPanel_DoubleClickOpensPromptForm(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)
	p.SetCurrentItem(1)

	doubleClickList(t, p.List)

	if f.app.Modal == nil {
		t.Fatal("double click did not open the prompt form")
	}
}

func TestSessionsPanel_SelectedSession(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)
	p.SetCurrentItem(2)
	if got := p.SelectedSession(); got == nil || got.ID() != f.s1.ID() {
		t.Errorf("SelectedSession() = %v, want session %d", got, f.s1.ID())
	}
}

func TestSessionsPanel_EnterWithoutSelection(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := panels.NewSessionsPanel(f.app)
	p.SetAccount(nil)
	p.GetInputCapture()(specialKey(views.KeyEdit))
	if f.app.Modal != nil {
		t.Error("Enter opened a modal without a selected session")
	}
}

func TestSessionsPanel_DeleteWithoutSelection(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := panels.NewSessionsPanel(f.app)
	p.SetAccount(nil)
	p.GetInputCapture()(specialKey(views.KeyDelete))
	if f.app.Modal != nil {
		t.Error("Delete opened a modal without a selected session")
	}
}

func TestSessionsPanel_StartSession(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)
	before := len(f.app.Svc.Sessions())

	p.GetInputCapture()(specialKey(views.KeyInsert))
	form := formFromCenteredModal(t, f.app.Modal)
	triggerSave(t, form)

	if got := len(f.app.Svc.Sessions()); got != before+1 {
		t.Errorf("session count = %d, want %d", got, before+1)
	}
	if f.app.Modal != nil {
		t.Error("start form stayed open after a successful start")
	}
	if p.SelectedSession() == nil {
		t.Error("started session was not selected")
	}
}

func TestSessionsPanel_StartSessionFailure(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	f.app.MutateErr = errors.New("save failed")
	p := f.panel(t)

	p.GetInputCapture()(specialKey(views.KeyInsert))
	form := formFromCenteredModal(t, f.app.Modal)
	triggerSave(t, form)

	if f.app.ErrorStatus == "" {
		t.Error("expected a status message reporting the start failure")
	}
	if f.app.Modal == nil {
		t.Error("start form closed after a failed start")
	}
}

func TestSessionsPanel_DeleteFailure(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	f.app.MutateErr = errors.New("delete failed")
	p := f.panel(t)
	p.SetCurrentItem(2)

	p.GetInputCapture()(specialKey(views.KeyDelete))
	confirmModal(t, f.app)

	if f.app.ErrorStatus == "" {
		t.Error("expected a status message reporting the delete failure")
	}
	if f.app.Modal != nil {
		t.Error("confirm modal stayed open after the delete failure")
	}
}

func TestSessionsPanel_ShowSession(t *testing.T) {
	t.Parallel()
	f := newSessionsFixture(t)
	p := f.panel(t)

	p.ShowSession(f.s1)
	if got := p.SelectedSession(); got == nil || got.ID() != f.s1.ID() {
		t.Errorf("ShowSession selected %v, want session %d", got, f.s1.ID())
	}

	p.ShowSession(nil)
	if got := p.SelectedSession(); got == nil || got.ID() != f.s1.ID() {
		t.Errorf("ShowSession(nil) selected %v, want session %d", got, f.s1.ID())
	}

	must(t, f.reg.DeleteSession(f.s1))
	p.ShowSession(f.s1)
	if got := p.SelectedSession(); got != nil && got.ID() == f.s1.ID() {
		t.Errorf("ShowSession selected deleted session %d", f.s1.ID())
	}
}

func TestSessionsPanel_OpenPauseForSession(t *testing.T) {
	t.Parallel()

	t.Run("success opens the account form", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)

		panels.OpenPauseForSession(p, f.s1)

		if f.app.Modal == nil {
			t.Fatal("pause did not open the account form")
		}
		if got := formFromCenteredModal(t, f.app.Modal).GetTitle(); got != " Pause Account " {
			t.Errorf("title = %q, want Pause Account", got)
		}
	})

	t.Run("missing account reports an error", func(t *testing.T) {
		t.Parallel()
		f := newSessionsFixture(t)
		p := f.panel(t)

		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("ghost@example.com", "Ghost")))
		acc, err := reg.AddAccount(reg.User("ghost@example.com"), reg.Vendor("claude"))
		must(t, err)
		session, err := reg.AddSession(acc, nil, "", nil)
		must(t, err)

		panels.OpenPauseForSession(p, session)

		if f.app.ErrorStatus == "" {
			t.Error("expected a status message for the missing account")
		}
		if f.app.Modal != nil {
			t.Error("pause opened a modal for a missing account")
		}
	})
}

func TestDeleteSessionMessage(t *testing.T) {
	t.Parallel()

	if got, want := panels.DeleteSessionMessage("Name", 1, 2), "Delete session\n\"Name\"?\nIts 2 queued prompts will be deleted."; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if got, want := panels.DeleteSessionMessage("", 7, 0), "Delete session\n\"session #7\"?"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func confirmModal(t *testing.T, app *fakeApp) {
	t.Helper()
	modal := confirmModalFrom(t, app.Modal)
	modal.SetFocus(1)
	modal.InputHandler()(specialKey(tcell.KeyEnter), func(tview.Primitive) {})
}
