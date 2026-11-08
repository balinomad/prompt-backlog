package forms_test

import (
	"testing"

	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestInputFieldFinishedFunc(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	form := formFromCenteredModal(t, forms.NewUserForm(app, nil, func() {}))
	field, ok := form.GetFormItemByLabel("Name").(*forms.InputField)
	if !ok {
		t.Fatalf("Name is %T, want *forms.InputField", form.GetFormItemByLabel("Name"))
	}

	var got tcell.Key
	field.SetFinishedFunc(func(key tcell.Key) { got = key })
	event := field.GetInputCapture()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if event != nil {
		t.Errorf("Enter returned %v, want nil", event)
	}
	if got != tcell.KeyEnter {
		t.Errorf("finished key = %v, want Enter", got)
	}

	got = 0
	field.SetFinishedFunc(nil)
	event = field.GetInputCapture()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if event != nil {
		t.Errorf("Enter with nil handler returned %v, want nil", event)
	}
	if got != 0 {
		t.Errorf("finished key = %v, want no callback", got)
	}

	event = field.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if event == nil {
		t.Error("printable key was consumed")
	}
}

func TestInputFieldUsesOSClipboard(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	form := formFromCenteredModal(t, forms.NewUserForm(app, nil, func() {}))
	field, ok := form.GetFormItemByLabel("Name").(*forms.InputField)
	if !ok {
		t.Fatalf("Name is %T, want *forms.InputField", form.GetFormItemByLabel("Name"))
	}

	field.SetText("first", false)
	field.Select(0, 5)
	field.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone), func(tview.Primitive) {})
	if app.CopiedText != "first" {
		t.Errorf("copied = %q, want first", app.CopiedText)
	}

	app.ClipboardText = "second"
	field.Select(0, 5)
	field.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModNone), func(tview.Primitive) {})
	if got := field.GetText(); got != "second" {
		t.Errorf("text = %q, want second", got)
	}
}
