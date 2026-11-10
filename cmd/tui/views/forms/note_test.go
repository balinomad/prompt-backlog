package forms_test

import (
	"testing"

	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"
	"github.com/balinomad/prompt-backlog/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func noteFormAccount(t *testing.T, app *fakeApp) *model.Account {
	t.Helper()
	acc, err := app.Svc.Account("alice@example.com", "claude")
	must(t, err)
	return acc
}

func noteFormFromModal(t *testing.T, p tview.Primitive) *forms.NoteForm {
	t.Helper()
	form, ok := forms.NoteFormFromPrimitive(p)
	if !ok {
		t.Fatalf("expected note form, got %T", p)
	}
	return form
}

func noteFormButton(t *testing.T, form *forms.NoteForm, label string) *tview.Button {
	t.Helper()
	button := form.Button(label)
	if button == nil {
		t.Fatalf("button %q not found", label)
	}
	return button
}

func TestNewNoteForm_MainAdd(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc := noteFormAccount(t, app)
	form := noteFormFromModal(t, forms.NewNoteForm(app, nil, acc, nil, func() {}))

	if got := form.Title(); got != " New Note " {
		t.Errorf("title = %q, want New Note", got)
	}
	project := form.Project()
	wantProjects := []string{"none", "Alpha", "Beta", "Orphan"}
	if got := project.GetOptionCount(); got != len(wantProjects) {
		t.Fatalf("project options = %d, want %d", got, len(wantProjects))
	}
	for i, name := range wantProjects {
		if got := dropdownOption(t, project, i); got != name {
			t.Errorf("project option %d = %q, want %q", i, got, name)
		}
	}

	account := form.Account()
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

	form.TextArea().SetText("note text", true)
	form.Project().SetCurrentOption(2)
	pressButton(t, noteFormButton(t, form, "Save"))

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
	app := newFakeApp(t)
	acc := noteFormAccount(t, app)
	note, err := app.Svc.AddNote("old text", "Beta", "alice@example.com", "claude")
	must(t, err)

	form := noteFormFromModal(t, forms.NewNoteForm(app, note, acc, nil, func() {}))
	if got := form.Title(); got != " Edit Note " {
		t.Errorf("title = %q, want Edit Note", got)
	}
	if got := form.TextArea().GetText(); got != "old text" {
		t.Errorf("Text = %q, want old text", got)
	}
	idx, _ := form.Project().GetCurrentOption()
	if idx != 2 {
		t.Errorf("project preselection = %d, want 2 (Beta)", idx)
	}
	idx, _ = form.Account().GetCurrentOption()
	if idx != 1 {
		t.Errorf("account preselection = %d, want 1", idx)
	}

	form.TextArea().SetText("new text", true)
	pressButton(t, noteFormButton(t, form, "Save"))
	if got := app.Svc.Note(note.ID()).Text(); got != "new text" {
		t.Errorf("Text() = %q, want new text", got)
	}
}

func TestNewNoteForm_EditStartsAtTop(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc := noteFormAccount(t, app)
	text := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8"
	note, err := app.Svc.AddNote(text, "Beta", "alice@example.com", "claude")
	must(t, err)

	form := noteFormFromModal(t, forms.NewNoteForm(app, note, acc, nil, func() {}))
	row, column := form.TextArea().GetOffset()
	if row != 0 || column != 0 {
		t.Errorf("initial offset = (%d, %d), want (0, 0)", row, column)
	}
}

func TestNewNoteForm_ClipboardActionsLayout(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc := noteFormAccount(t, app)
	modal := forms.NewNoteForm(app, nil, acc, nil, func() {})

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	modal.SetRect(0, 0, 80, 24)
	modal.Draw(screen)

	form := noteFormFromModal(t, modal)
	tx, ty, _, th := form.TextArea().GetRect()
	cx, cy, cw, ch := noteFormButton(t, form, "Clear").GetRect()
	px, py, pw, ph := noteFormButton(t, form, "Copy").GetRect()
	ax, ay, aw, ah := noteFormButton(t, form, "Paste").GetRect()

	if cx >= tx {
		t.Errorf("Clear x = %d, want left of text x = %d", cx, tx)
	}
	if cx != px || cx != ax {
		t.Errorf("clipboard button x = %d, %d, %d, want equal", cx, px, ax)
	}
	if cw != pw || cw != aw {
		t.Errorf("clipboard button width = %d, %d, %d, want equal", cw, pw, aw)
	}
	if py != cy+ch+1 {
		t.Errorf("Clear bottom = %d, Copy top = %d, want one blank row", cy+ch, py)
	}
	if cy != ay+ah+1 {
		t.Errorf("Paste bottom = %d, Clear top = %d, want one blank row", ay+ah, cy)
	}
	if py+ph != ty+th {
		t.Errorf("Copy bottom = %d, Text bottom = %d, want aligned", py+ph, ty+th)
	}
}

func TestNewNoteForm_DropDownSpacing(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc := noteFormAccount(t, app)
	modal := forms.NewNoteForm(app, nil, acc, nil, func() {})

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	modal.SetRect(0, 0, 80, 24)
	modal.Draw(screen)

	form := noteFormFromModal(t, modal)
	_, py, _, ph := form.Project().GetRect()
	_, ay, _, ah := form.Account().GetRect()
	if ay != py+ph+1 {
		t.Errorf("Project bottom = %d, Account top = %d, want one blank row", py+ph, ay)
	}
	_, sy, _, _ := noteFormButton(t, form, "Save").GetRect()
	if sy != ay+ah+1 {
		t.Errorf("Account bottom = %d, Save top = %d, want one blank row", ay+ah, sy)
	}
}

func TestNewNoteForm_ClipboardButtons(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc := noteFormAccount(t, app)
	form := noteFormFromModal(t, forms.NewNoteForm(app, nil, acc, nil, func() {}))
	textArea := form.TextArea()
	textArea.SetText("first", false)

	pressButton(t, noteFormButton(t, form, "Copy"))
	if app.CopiedText != "first" {
		t.Errorf("copied = %q, want first", app.CopiedText)
	}

	app.ClipboardText = "second"
	textArea.Select(0, 5)
	pressButton(t, noteFormButton(t, form, "Paste"))
	if got := textArea.GetText(); got != "second" {
		t.Errorf("text = %q, want second", got)
	}

	pressButton(t, noteFormButton(t, form, "Clear"))
	if got := textArea.GetText(); got != "" {
		t.Errorf("text = %q, want empty", got)
	}
	if app.Focused != textArea {
		t.Errorf("focus = %T, want text area", app.Focused)
	}
}

func TestNewNoteForm_MainEditWithoutAccount(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc := noteFormAccount(t, app)
	note, err := app.Svc.AddNote("text", "Alpha", "", "")
	must(t, err)

	form := noteFormFromModal(t, forms.NewNoteForm(app, note, acc, nil, func() {}))
	idx, _ := form.Account().GetCurrentOption()
	if idx != 0 {
		t.Errorf("account preselection = %d, want 0 (none)", idx)
	}
}

func TestNewNoteForm_ProjectsAdd(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	project := app.Svc.Project("Alpha")
	form := noteFormFromModal(t, forms.NewNoteForm(app, nil, nil, project, func() {}))

	projectDropdown := form.Project()
	projectDropdown.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	if projectDropdown.IsOpen() {
		t.Error("Project dropdown opened despite being disabled")
	}
	if got := dropdownOption(t, projectDropdown, 0); got != "Alpha" {
		t.Errorf("project option = %q, want Alpha", got)
	}

	account := form.Account()
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

	form.TextArea().SetText("project note", true)
	account.SetCurrentOption(1)
	pressButton(t, noteFormButton(t, form, "Save"))

	notes, err := app.Svc.ProjectNotes("Alpha")
	must(t, err)
	if len(notes) != 1 || notes[0].Text() != "project note" {
		t.Fatalf("project notes = %v, want one note", notes)
	}
	if key, ok := notes[0].AccountKey(); !ok || key.Email != "alice@example.com" {
		t.Errorf("note account = %v, %v; want alice, true", key, ok)
	}
}

func TestNewNoteForm_ProjectsEdit(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	project := app.Svc.Project("Alpha")
	note, err := app.Svc.AddNote("text", "Alpha", "alice@example.com", "claude")
	must(t, err)

	form := noteFormFromModal(t, forms.NewNoteForm(app, note, nil, project, func() {}))
	idx, _ := form.Account().GetCurrentOption()
	if idx != 1 {
		t.Errorf("account preselection = %d, want 1", idx)
	}
}

func TestNewNoteForm_SaveError(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc := noteFormAccount(t, app)
	closed := false
	form := noteFormFromModal(t, forms.NewNoteForm(app, nil, acc, nil, func() { closed = true }))

	form.TextArea().SetText("text", true)
	form.Project().SetCurrentOption(3) // Orphan, not granted
	pressButton(t, noteFormButton(t, form, "Save"))

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

func TestNewNoteForm_EmptyText(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc := noteFormAccount(t, app)
	form := noteFormFromModal(t, forms.NewNoteForm(app, nil, acc, nil, func() {}))
	form.TextArea().SetText("   ", true)
	pressButton(t, noteFormButton(t, form, "Save"))
	if app.ErrorStatus == "" {
		t.Error("expected a validation error")
	}
	if len(app.Svc.Notes()) != 0 {
		t.Error("note was created despite empty text")
	}
}
