package forms_test

import (
	"testing"

	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"
	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func newSessionFormFixture(t *testing.T) (*fakeApp, *model.Session) {
	t.Helper()
	app := newFakeApp(t)
	sess, err := app.Svc.StartSession("alice@example.com", "claude", service.SessionFields{
		Name:        "Name",
		ProjectName: "Alpha",
	})
	must(t, err)
	return app, sess
}

func sessionFormAccount(t *testing.T, app *fakeApp, email, vendor string) *model.Account {
	t.Helper()
	acc, err := app.Svc.Account(email, vendor)
	must(t, err)
	return acc
}

func sessionFormOptions(t *testing.T, app *fakeApp, sess *model.Session, onClose func()) *forms.SessionFormOptions {
	t.Helper()
	return &forms.SessionFormOptions{
		Title:       "Edit Session",
		SubmitLabel: "Save",
		Account:     sessionFormAccount(t, app, "alice@example.com", "claude"),
		Values: service.SessionFields{
			Name:        sess.Name(),
			URL:         sess.URL(),
			ProjectName: sess.ProjectName(),
		},
		OnSubmit: func(fields service.SessionFields) error {
			if err := app.Svc.UpdateSession("alice@example.com", "claude", sess.ID(), fields); err != nil {
				return err
			}
			onClose()
			return nil
		},
		OnCancel: onClose,
	}
}

func TestNewSessionForm_Shape(t *testing.T) {
	t.Parallel()
	app, sess := newSessionFormFixture(t)
	form := formFromCenteredModal(t, forms.NewSessionForm(app, sessionFormOptions(t, app, sess, func() {})))

	if got := form.GetTitle(); got != " Edit Session " {
		t.Errorf("title = %q, want Edit Session", got)
	}
	for _, label := range []string{"Name", "URL", "Project"} {
		if form.GetFormItemByLabel(label) == nil {
			t.Errorf("missing form item %q", label)
		}
	}
	if got := form.GetButton(0).GetLabel(); got != "Save" {
		t.Errorf("first button = %q, want Save", got)
	}
	if got := form.GetButton(1).GetLabel(); got != "Cancel" {
		t.Errorf("second button = %q, want Cancel", got)
	}
}

//nolint:errcheck // The form item types are known.
func TestNewSessionForm_InitialValues(t *testing.T) {
	t.Parallel()
	app, sess := newSessionFormFixture(t)
	must(t, app.Svc.UpdateSessionName("alice@example.com", "claude", sess.ID(), "Architecture review"))
	must(t, app.Svc.UpdateSessionURL("alice@example.com", "claude", sess.ID(), "https://example.com/chat/1"))
	sess = app.Svc.Session(sess.ID())

	form := formFromCenteredModal(t, forms.NewSessionForm(app, sessionFormOptions(t, app, sess, func() {})))
	if got := form.GetFormItemByLabel("Name").(*forms.InputField).GetText(); got != "Architecture review" {
		t.Errorf("Name = %q", got)
	}
	if got := form.GetFormItemByLabel("URL").(*forms.InputField).GetText(); got != "https://example.com/chat/1" {
		t.Errorf("URL = %q", got)
	}
	dropdown := form.GetFormItemByLabel("Project").(*tview.DropDown)
	idx, _ := dropdown.GetCurrentOption()
	if idx != 1 {
		t.Errorf("Project index = %d, want 1 (Alpha)", idx)
	}
}

//nolint:errcheck // The form item types are known.
func TestNewSessionForm_ProjectOptions(t *testing.T) {
	t.Parallel()
	app, sess := newSessionFormFixture(t)
	form := formFromCenteredModal(t, forms.NewSessionForm(app, sessionFormOptions(t, app, sess, func() {})))
	dropdown := form.GetFormItemByLabel("Project").(*tview.DropDown)

	want := []string{"none", "Alpha", "Beta"}
	if got := dropdown.GetOptionCount(); got != len(want) {
		t.Fatalf("option count = %d, want %d", got, len(want))
	}
	for i, name := range want {
		dropdown.SetCurrentOption(i)
		idx, got := dropdown.GetCurrentOption()
		if idx != i || got != name {
			t.Errorf("option %d = %q (index %d), want %q", i, got, idx, name)
		}
	}
}

//nolint:errcheck // The form item types are known.
func TestNewSessionForm_Save(t *testing.T) {
	t.Parallel()
	app, sess := newSessionFormFixture(t)
	form := formFromCenteredModal(t, forms.NewSessionForm(app, sessionFormOptions(t, app, sess, func() {})))

	form.GetFormItemByLabel("Name").(*forms.InputField).SetText("  New Name  ", false)
	form.GetFormItemByLabel("URL").(*forms.InputField).SetText("  https://example.com/chat/2  ", false)
	form.GetFormItemByLabel("Project").(*tview.DropDown).SetCurrentOption(2) // Beta
	pressButton(t, form.GetButton(0))

	got := app.Svc.Session(sess.ID())
	if got.Name() != "New Name" {
		t.Errorf("Name() = %q", got.Name())
	}
	if got.URL() != "https://example.com/chat/2" {
		t.Errorf("URL() = %q", got.URL())
	}
	if got.ProjectName() != "Beta" {
		t.Errorf("ProjectName() = %q, want Beta", got.ProjectName())
	}
}

//nolint:errcheck // The form item types are known.
func TestNewSessionForm_ProjectFailureStopsLaterChanges(t *testing.T) {
	t.Parallel()
	app, sess := newSessionFormFixture(t)
	form := formFromCenteredModal(t, forms.NewSessionForm(app, sessionFormOptions(t, app, sess, func() {})))

	form.GetFormItemByLabel("Name").(*forms.InputField).SetText("should not apply", false)
	form.GetFormItemByLabel("Project").(*tview.DropDown).SetCurrentOption(2) // Beta
	must(t, app.Svc.RevokeProject("alice@example.com", "claude", "Beta"))
	pressButton(t, form.GetButton(0))

	if app.ErrorStatus == "" {
		t.Error("expected an error status for the failed project change")
	}
	if got := app.Svc.Session(sess.ID()).Name(); got != "Name" {
		t.Errorf("Name() = %q, want the original value", got)
	}
}

//nolint:errcheck // The form item types are known.
func TestNewSessionForm_CtrlSAndCancel(t *testing.T) {
	t.Parallel()
	app, sess := newSessionFormFixture(t)
	closed := false
	form := formFromCenteredModal(t, forms.NewSessionForm(app, sessionFormOptions(t, app, sess, func() { closed = true })))

	form.GetFormItemByLabel("Name").(*forms.InputField).SetText("via ctrl-s", false)
	form.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone), func(tview.Primitive) {})
	if !closed {
		t.Error("Ctrl+S did not close the form")
	}
	if got := app.Svc.Session(sess.ID()).Name(); got != "via ctrl-s" {
		t.Errorf("Name() = %q, want via ctrl-s", got)
	}
}

func TestNewSessionForm_StartShape(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc := sessionFormAccount(t, app, "alice@example.com", "claude")
	form := formFromCenteredModal(t, forms.NewSessionForm(app, &forms.SessionFormOptions{
		Title:       "Start Session",
		SubmitLabel: "Start",
		Account:     acc,
		OnSubmit:    func(service.SessionFields) error { return nil },
		OnCancel:    func() {},
	}))

	if got := form.GetTitle(); got != " Start Session " {
		t.Errorf("title = %q, want Start Session", got)
	}
	if got := form.GetButton(0).GetLabel(); got != "Start" {
		t.Errorf("first button = %q, want Start", got)
	}
}
