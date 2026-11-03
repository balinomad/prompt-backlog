package forms_test

import (
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views/forms"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func noteFormAccount(t *testing.T, app *mockApp) *model.Account {
	t.Helper()
	acc, err := app.Svc.Account("alice@example.com", "claude")
	must(t, err)
	return acc
}

//nolint:cyclop // Form test with independent assertions.
func TestNewNoteForm_MainAdd(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc := noteFormAccount(t, app)
	form := formFromCenteredModal(t, forms.NewNoteForm(app, nil, acc, nil, func() {}))

	if got := form.GetTitle(); got != " New Note " {
		t.Errorf("title = %q, want New Note", got)
	}
	project, ok := form.GetFormItemByLabel("Project").(*tview.DropDown)
	if !ok {
		t.Fatal("Project is not a dropdown")
	}
	wantProjects := []string{"none", "Alpha", "Beta", "Orphan"}
	if got := project.GetOptionCount(); got != len(wantProjects) {
		t.Fatalf("project options = %d, want %d", got, len(wantProjects))
	}
	for i, name := range wantProjects {
		if got := dropdownOption(t, project, i); got != name {
			t.Errorf("project option %d = %q, want %q", i, got, name)
		}
	}

	account, ok := form.GetFormItemByLabel("Account").(*tview.DropDown)
	if !ok {
		t.Fatal("Account is not a dropdown")
	}
	if got := account.GetOptionCount(); got != 2 {
		t.Fatalf("account options = %d, want 2", got)
	}
	idx, _ := account.GetCurrentOption()
	if idx != 1 {
		t.Errorf("account preselection = %d, want 1", idx)
	}
	if got := dropdownOption(t, account, 0); got != "none" {
		t.Errorf("account option 0 = %q, want none", got)
	}
	if got := dropdownOption(t, account, 1); got != "claude · Alice" {
		t.Errorf("account option 1 = %q, want claude · Alice", got)
	}

	form.GetFormItemByLabel("Text").(*tview.TextArea).SetText("note text", true) //nolint:errcheck // The item type is known.
	form.GetFormItemByLabel("Project").(*tview.DropDown).SetCurrentOption(2)     //nolint:errcheck // The item type is known.
	pressButton(t, form.GetButton(0))

	notes := app.Svc.Notes()
	if len(notes) != 1 {
		t.Fatalf("notes = %d, want 1", len(notes))
	}
	if notes[0].Text() != "note text" || notes[0].ProjectName() != "Beta" {
		t.Errorf("note = text %q project %q", notes[0].Text(), notes[0].ProjectName())
	}
	if key, ok := notes[0].AccountKey(); !ok || key.Email != "alice@example.com" {
		t.Errorf("note account = %v, %v; want alice, true", key, ok)
	}
}

func TestNewNoteForm_MainEdit(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc := noteFormAccount(t, app)
	note, err := app.Svc.AddNote("old text", "Beta", "alice@example.com", "claude")
	must(t, err)

	form := formFromCenteredModal(t, forms.NewNoteForm(app, note, acc, nil, func() {}))
	if got := form.GetTitle(); got != " Edit Note " {
		t.Errorf("title = %q, want Edit Note", got)
	}
	if got := form.GetFormItemByLabel("Text").(*tview.TextArea).GetText(); got != "old text" { //nolint:errcheck // The item type is known.
		t.Errorf("Text = %q, want old text", got)
	}
	idx, _ := form.GetFormItemByLabel("Project").(*tview.DropDown).GetCurrentOption() //nolint:errcheck // The item type is known.
	if idx != 2 {
		t.Errorf("project preselection = %d, want 2 (Beta)", idx)
	}
	idx, _ = form.GetFormItemByLabel("Account").(*tview.DropDown).GetCurrentOption() //nolint:errcheck // The item type is known.
	if idx != 1 {
		t.Errorf("account preselection = %d, want 1", idx)
	}

	form.GetFormItemByLabel("Text").(*tview.TextArea).SetText("new text", true) //nolint:errcheck // The item type is known.
	pressButton(t, form.GetButton(0))
	if got := app.Svc.Note(note.ID()).Text(); got != "new text" {
		t.Errorf("Text() = %q, want new text", got)
	}
}

//nolint:errcheck // The form item types are known.
func TestNewNoteForm_MainEditWithoutAccount(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc := noteFormAccount(t, app)
	note, err := app.Svc.AddNote("text", "Alpha", "", "")
	must(t, err)

	form := formFromCenteredModal(t, forms.NewNoteForm(app, note, acc, nil, func() {}))
	idx, _ := form.GetFormItemByLabel("Account").(*tview.DropDown).GetCurrentOption()
	if idx != 0 {
		t.Errorf("account preselection = %d, want 0 (none)", idx)
	}
}

//nolint:errcheck // The form item types are known.
func TestNewNoteForm_ProjectsAdd(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	project := app.Svc.Project("Alpha")
	form := formFromCenteredModal(t, forms.NewNoteForm(app, nil, nil, project, func() {}))

	projectDropdown := form.GetFormItemByLabel("Project").(*tview.DropDown)
	projectDropdown.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	if projectDropdown.IsOpen() {
		t.Error("Project dropdown opened despite being disabled")
	}
	if got := dropdownOption(t, projectDropdown, 0); got != "Alpha" {
		t.Errorf("project option = %q, want Alpha", got)
	}

	account, ok := form.GetFormItemByLabel("Account").(*tview.DropDown)
	if !ok {
		t.Fatal("Account is not a dropdown")
	}
	if got := account.GetOptionCount(); got != 2 {
		t.Fatalf("account options = %d, want 2", got)
	}
	idx, _ := account.GetCurrentOption()
	if idx != 0 {
		t.Errorf("account preselection = %d, want 0 (none)", idx)
	}
	if got := dropdownOption(t, account, 1); got != "claude · Alice" {
		t.Errorf("account option 1 = %q, want claude · Alice", got)
	}

	form.GetFormItemByLabel("Text").(*tview.TextArea).SetText("project note", true)
	account.SetCurrentOption(1)
	pressButton(t, form.GetButton(0))

	notes, err := app.Svc.ProjectNotes("Alpha")
	must(t, err)
	if len(notes) != 1 || notes[0].Text() != "project note" {
		t.Fatalf("project notes = %v, want one note", notes)
	}
	if key, ok := notes[0].AccountKey(); !ok || key.Email != "alice@example.com" {
		t.Errorf("note account = %v, %v; want alice, true", key, ok)
	}
}

//nolint:errcheck // The form item types are known.
func TestNewNoteForm_ProjectsEdit(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	project := app.Svc.Project("Alpha")
	note, err := app.Svc.AddNote("text", "Alpha", "alice@example.com", "claude")
	must(t, err)

	form := formFromCenteredModal(t, forms.NewNoteForm(app, note, nil, project, func() {}))
	idx, _ := form.GetFormItemByLabel("Account").(*tview.DropDown).GetCurrentOption()
	if idx != 1 {
		t.Errorf("account preselection = %d, want 1", idx)
	}
}

//nolint:errcheck // The form item types are known.
func TestNewNoteForm_SaveError(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc := noteFormAccount(t, app)
	closed := false
	form := formFromCenteredModal(t, forms.NewNoteForm(app, nil, acc, nil, func() { closed = true }))

	form.GetFormItemByLabel("Text").(*tview.TextArea).SetText("text", true)
	form.GetFormItemByLabel("Project").(*tview.DropDown).SetCurrentOption(3) // Orphan, not granted
	pressButton(t, form.GetButton(0))

	if app.ErrorStatus == "" {
		t.Error("expected an error status")
	}
	if closed {
		t.Error("form closed after a failed save")
	}
	if len(app.Svc.Notes()) != 0 {
		t.Error("note was created despite the invariant failure")
	}
}

//nolint:errcheck // The form item types are known.
func TestNewNoteForm_EmptyText(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc := noteFormAccount(t, app)
	form := formFromCenteredModal(t, forms.NewNoteForm(app, nil, acc, nil, func() {}))
	form.GetFormItemByLabel("Text").(*tview.TextArea).SetText("   ", true)
	pressButton(t, form.GetButton(0))
	if app.ErrorStatus == "" {
		t.Error("expected a validation error")
	}
	if len(app.Svc.Notes()) != 0 {
		t.Error("note was created despite empty text")
	}
}
