package panels_test

import (
	"strings"
	"testing"

	"github.com/balinomad/prompt-backlog/cmd/tui/views/panels"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/internal/model"

	"github.com/gdamore/tcell/v2"
)

type notesFixture struct {
	app *fakeApp
	acc *model.Account
	n1  *model.Note
	n2  *model.Note
	n3  *model.Note
	n4  *model.Note
}

func newNotesFixture(t *testing.T) notesFixture {
	t.Helper()
	app := newFakeApp(t)
	acc, err := app.Svc.Account("alice@example.com", "claude")
	must(t, err)
	n1, err := app.Svc.AddNote("one", "Alpha", "", "")
	must(t, err)
	n2, err := app.Svc.AddNote("two", "", "alice@example.com", "claude")
	must(t, err)
	n3, err := app.Svc.AddNote("three", "", "", "")
	must(t, err)
	n4, err := app.Svc.AddNote("four", "Alpha", "alice@example.com", "claude")
	must(t, err)
	return notesFixture{app: app, acc: acc, n1: n1, n2: n2, n3: n3, n4: n4}
}

func noteItemText(t *testing.T, p *panels.NotesPanel, index int) string {
	t.Helper()
	main, _ := p.GetItemText(index)
	return main
}

func (f notesFixture) panel(t *testing.T) *panels.NotesPanel {
	t.Helper()
	p := panels.NewNotesPanel(f.app)
	p.SetAccount(f.acc)
	p.SetWidth(80)
	return p
}

func TestNotesPanel_Rows(t *testing.T) {
	t.Parallel()
	f := newNotesFixture(t)
	p := f.panel(t)

	if got := p.GetItemCount(); got != 4 {
		t.Fatalf("rows = %d, want 4", got)
	}
	if row := noteItemText(t, p, 0); !strings.Contains(row, "Alpha") || !strings.Contains(row, "one") {
		t.Errorf("row 0 = %q", row)
	}
	if row := noteItemText(t, p, 1); !strings.Contains(row, "Alice") || !strings.Contains(row, "two") {
		t.Errorf("row 1 = %q", row)
	}
	if row := noteItemText(t, p, 2); !strings.Contains(row, "three") {
		t.Errorf("row 2 = %q", row)
	}
	if row := noteItemText(t, p, 3); !strings.Contains(row, "Alpha") || !strings.Contains(row, "Alice") || !strings.Contains(row, "four") {
		t.Errorf("row 3 = %q", row)
	}
	if got := p.GetTitle(); !strings.Contains(got, "Notes") || !strings.Contains(got, "1/4") {
		t.Errorf("title = %q, want Notes 1/4", got)
	}
}

func TestNotesPanel_Visibility(t *testing.T) {
	t.Parallel()
	f := newNotesFixture(t)
	bob, err := f.app.Svc.Account("bob@example.com", "chatgpt")
	must(t, err)
	p := panels.NewNotesPanel(f.app)
	p.SetAccount(bob)
	p.SetWidth(80)

	if got := p.GetItemCount(); got != 1 {
		t.Fatalf("rows = %d, want 1 (global note only)", got)
	}
	if row := noteItemText(t, p, 0); !strings.Contains(row, "three") {
		t.Errorf("row = %q, want the global note", row)
	}
}

func TestNotesPanel_Filter(t *testing.T) {
	t.Parallel()
	f := newNotesFixture(t)
	filter := views.NewProjectFilterState()
	p := panels.NewNotesPanel(f.app)
	p.SetFilter(filter)
	p.SetAccount(f.acc)
	p.SetWidth(80)

	must(t, filter.Cycle()) // Alpha
	p.Refresh()
	if got := p.GetItemCount(); got != 2 {
		t.Fatalf("Alpha rows = %d, want 2", got)
	}
	if !strings.Contains(noteItemText(t, p, 0), "one") || !strings.Contains(noteItemText(t, p, 1), "four") {
		t.Errorf("Alpha rows = %q, %q", noteItemText(t, p, 0), noteItemText(t, p, 1))
	}

	must(t, filter.Cycle()) // Beta
	p.Refresh()
	if got := p.GetItemCount(); got != 0 {
		t.Errorf("Beta rows = %d, want 0", got)
	}

	must(t, filter.Cycle()) // None
	p.Refresh()
	if got := p.GetItemCount(); got != 2 {
		t.Fatalf("None rows = %d, want 2", got)
	}
	if !strings.Contains(noteItemText(t, p, 0), "two") || !strings.Contains(noteItemText(t, p, 1), "three") {
		t.Errorf("None rows = %q, %q", noteItemText(t, p, 0), noteItemText(t, p, 1))
	}
}

func TestNotesPanel_Selection(t *testing.T) {
	t.Parallel()

	t.Run("selected note stays by ID", func(t *testing.T) {
		t.Parallel()
		f := newNotesFixture(t)
		p := f.panel(t)
		p.SetCurrentItem(1)
		p.Refresh()
		if got := p.SelectedNote(); got == nil || got.ID() != f.n2.ID() {
			t.Errorf("selected = %v, want note 2", got)
		}
	})

	t.Run("removed note selects the nearest index", func(t *testing.T) {
		t.Parallel()
		f := newNotesFixture(t)
		p := f.panel(t)
		p.SetCurrentItem(1)
		must(t, f.app.Svc.DeleteNote(f.n2.ID()))
		p.Refresh()
		if got := p.SelectedNote(); got == nil || got.ID() != f.n3.ID() {
			t.Errorf("selected = %v, want note 3", got)
		}
	})
}

func TestNotesPanel_EscOpensQuitConfirmation(t *testing.T) {
	t.Parallel()
	f := newNotesFixture(t)
	p := f.panel(t)
	p.GetInputCapture()(specialKey(views.KeyCancel))
	if f.app.Modal == nil {
		t.Fatal("Esc did not open the quit confirmation")
	}
}

func TestNotesPanel_Keys(t *testing.T) {
	t.Parallel()

	t.Run("Enter opens the edit form", func(t *testing.T) {
		t.Parallel()
		f := newNotesFixture(t)
		p := f.panel(t)
		p.SetCurrentItem(0)
		p.GetInputCapture()(specialKey(views.KeyEdit))
		if got := centeredModalTitle(t, f.app.Modal); got != " Edit Note " {
			t.Errorf("title = %q, want Edit Note", got)
		}
	})

	t.Run("Ins opens the add form", func(t *testing.T) {
		t.Parallel()
		f := newNotesFixture(t)
		p := f.panel(t)
		p.GetInputCapture()(specialKey(views.KeyInsert))
		if got := centeredModalTitle(t, f.app.Modal); got != " New Note " {
			t.Errorf("title = %q, want New Note", got)
		}
	})

	t.Run("Del confirms and deletes", func(t *testing.T) {
		t.Parallel()
		f := newNotesFixture(t)
		p := f.panel(t)
		p.SetCurrentItem(0)
		p.GetInputCapture()(specialKey(views.KeyDelete))
		confirmModal(t, f.app)
		p.Refresh()
		if f.app.Svc.Note(f.n1.ID()) != nil {
			t.Error("note still exists after confirming delete")
		}
	})

	t.Run("f advances the filter and calls the callback", func(t *testing.T) {
		t.Parallel()
		f := newNotesFixture(t)
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

	t.Run("empty pane ignores note actions", func(t *testing.T) {
		t.Parallel()
		f := newNotesFixture(t)
		for _, note := range f.app.Svc.Notes() {
			must(t, f.app.Svc.DeleteNote(note.ID()))
		}
		p := f.panel(t)
		if got := p.GetItemCount(); got != 0 {
			t.Fatalf("rows = %d, want 0", got)
		}
		for _, key := range []*tcell.EventKey{
			specialKey(views.KeyEdit),
			specialKey(views.KeyDelete),
		} {
			p.GetInputCapture()(key)
			if f.app.Modal != nil {
				t.Errorf("key opened a modal on an empty pane")
			}
		}
	})
}

//nolint:cyclop // Form construction has many independent UI branches.
func TestNoteRows_ColumnRemoval(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	if err := reg.AddVendor(model.NewVendor("claude", true, false)); err != nil {
		t.Fatal(err)
	}
	if err := reg.AddUser(model.NewUser("alice@example.com", "Alice")); err != nil {
		t.Fatal(err)
	}
	if err := reg.AddProject(model.NewProject("Alpha", "", "")); err != nil {
		t.Fatal(err)
	}
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	if err != nil {
		t.Fatal(err)
	}
	if addErr := reg.AddProjectToAccount(acc, reg.Project("Alpha")); addErr != nil {
		t.Fatal(addErr)
	}
	if _, addErr := reg.AddNote("note", reg.Project("Alpha"), acc); addErr != nil {
		t.Fatal(addErr)
	}
	svc := newService(t, reg)
	notes := reg.Notes()

	wide, err := panels.NoteRows(notes, svc, 100)
	must(t, err)
	if len(wide) != 1 || !strings.Contains(wide[0], "Alpha") || !strings.Contains(wide[0], "Alice") || !strings.Contains(wide[0], "note") {
		t.Errorf("wide row = %v", wide)
	}

	medium, err := panels.NoteRows(notes, svc, 29)
	must(t, err)
	if len(medium) != 1 || strings.Contains(medium[0], "Alice") || !strings.Contains(medium[0], "Alpha") || !strings.Contains(medium[0], "note") {
		t.Errorf("medium row = %v, want account removed and project kept", medium)
	}

	narrow, err := panels.NoteRows(notes, svc, 20)
	must(t, err)
	if len(narrow) != 1 || strings.Contains(narrow[0], "Alpha") || strings.Contains(narrow[0], "Alice") || !strings.Contains(narrow[0], "note") {
		t.Errorf("narrow row = %v, want only the text", narrow)
	}

	unbounded, err := panels.NoteRows(notes, svc, 0)
	must(t, err)
	if len(unbounded) != 1 || !strings.Contains(unbounded[0], "Alpha") || !strings.Contains(unbounded[0], "Alice") {
		t.Errorf("unbounded row = %v, want every column", unbounded)
	}
}

func TestDeleteNoteMessage(t *testing.T) {
	t.Parallel()
	if got, want := panels.DeleteNoteMessage("first line\nsecond"), "Delete note\n\"first line\"?"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	long := strings.Repeat("x", 60)
	got := panels.DeleteNoteMessage(long)
	if !strings.Contains(got, "…") {
		t.Errorf("message = %q, want an ellipsis", got)
	}
}

func TestNotesPanel_DoubleClickOpensEditForm(t *testing.T) {
	t.Parallel()
	f := newNotesFixture(t)
	p := f.panel(t)
	p.SetCurrentItem(0)

	doubleClickList(t, p.List)

	if got := centeredModalTitle(t, f.app.Modal); got != " Edit Note " {
		t.Errorf("title = %q, want Edit Note", got)
	}
}
