package forms_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"
	"github.com/balinomad/prompt-backlog/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func newPromptFormApp(t *testing.T, queue []model.Prompt) (*fakeApp, *model.Session) {
	t.Helper()
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
	sess, err := reg.AddSession(acc, reg.Project("Alpha"), "Name", queue)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeApp{Svc: newService(t, reg)}, sess
}

func drawPromptForm(t *testing.T, pf *forms.PromptForm) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	modal := pf.Primitive()
	modal.SetRect(0, 0, 80, 24)
	modal.Draw(screen)
}

func confirmModalFrom(t *testing.T, app *fakeApp) *tview.Modal {
	t.Helper()
	modal, ok := views.UnwrapModal(app.Modal).(*tview.Modal)
	if !ok {
		t.Fatalf("expected *tview.Modal, got %T", app.Modal)
	}
	return modal
}

func pressConfirm(t *testing.T, modal *tview.Modal) {
	t.Helper()
	modal.SetFocus(1)
	modal.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
}

func pressCancel(t *testing.T, modal *tview.Modal) {
	t.Helper()
	modal.GetInputCapture()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
}

//nolint:cyclop // Layout test checks independent geometry assertions.
func TestPromptForm_Layout(t *testing.T) {
	t.Parallel()

	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	drawPromptForm(t, pf)

	if got := pf.Label().GetText(false); got != "Prompt" {
		t.Errorf("label = %q, want Prompt", got)
	}
	if got := pf.Status().GetLabel(); got != "Status" {
		t.Errorf("status label = %q, want Status", got)
	}
	if got, want := pf.Editor().GetTitle(), " Edit Prompts 1/2 "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if got := pf.Status().GetOptionCount(); got != 3 {
		t.Errorf("status options = %d, want 3", got)
	}

	editorX, _, editorW, editorH := pf.Editor().GetRect()
	if editorH > 24 {
		t.Errorf("editor height = %d, want at most 24", editorH)
	}
	tx, ty, tw, th := pf.TextArea().GetRect()
	cx, cy, cw, ch := pf.ClearButton().GetRect()
	px, py, pw, ph := pf.CopyButton().GetRect()
	ax, ay, aw, ah := pf.PasteButton().GetRect()
	sx, sy, sw, sh := pf.SessionButton().GetRect()
	fx, fy, fw, fh := pf.Status().GetRect()
	ppx, _, ppw, _ := pf.SavePauseButton().GetRect()
	vx, vy, vw, vh := pf.SaveButton().GetRect()
	lx, ly, lw, lh := pf.CancelButton().GetRect()
	deleteButton := pf.DeleteButton()
	if deleteButton == nil {
		t.Fatal("editor has no delete button")
	}
	rightButtonX, _, rightButtonW, _ := deleteButton.GetRect()

	for name, r := range map[string][4]int{
		"text":    {tx, ty, tw, th},
		"clear":   {cx, cy, cw, ch},
		"copy":    {px, py, pw, ph},
		"paste":   {ax, ay, aw, ah},
		"session": {sx, sy, sw, sh},
		"status":  {fx, fy, fw, fh},
		"save":    {vx, vy, vw, vh},
		"cancel":  {lx, ly, lw, lh},
	} {
		if r[0] < editorX || r[0]+r[2] > editorX+editorW {
			t.Errorf("%s rect %v is outside the editor width %d", name, r, editorW)
		}
	}
	if cw != pw || cw != aw {
		t.Errorf("Clear width = %d, Copy width = %d, Paste width = %d, want equal", cw, pw, aw)
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
	if sy >= ty {
		t.Errorf("Session row y = %d, want above the prompt field y = %d", sy, ty)
	}
	list := pf.PromptList()
	if list == nil {
		t.Fatal("editor has no prompt list")
	}
	if got := list.GetItemCount(); got != 2 {
		t.Errorf("list items = %d, want 2", got)
	}
	if got, _ := list.GetItemText(0); got != "1. first" {
		t.Errorf("first list item = %q, want 1. first", got)
	}
	listX, listY, listW, listH := list.GetRect()
	if listH != 4 {
		t.Errorf("list height = %d, want 4 for two prompts", listH)
	}
	if listY <= sy {
		t.Errorf("list y = %d, want below the session row y = %d", listY, sy)
	}
	if listY+listH > ty {
		t.Errorf("list bottom = %d, want above the prompt field y = %d", listY+listH, ty)
	}
	if listX < editorX || listX+listW > editorX+editorW {
		t.Errorf("list rect %v is outside the editor width %d", []int{listX, listY, listW, listH}, editorW)
	}
	if got, want := fx+forms.PromptSideWidth+1, tx; got != want {
		t.Errorf("status field x = %d, want prompt text x = %d", got, want)
	}
	if got, want := fw, forms.PromptSideWidth+1+tview.TaggedStringWidth("Needs closing")+tview.TaggedStringWidth(forms.DropDownArrow); got != want {
		t.Errorf("status width = %d, want %d", got, want)
	}
	if got := (ppx - editorX) - (editorX + editorW - (rightButtonX + rightButtonW)); got < -1 || got > 1 {
		t.Errorf("button-row margins differ by %d", got)
	}
	if vx <= ppx+ppw {
		t.Errorf("Save x = %d, want after Save & Pause", vx)
	}
}

func TestPromptForm_PromptFieldStyle(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	modal := pf.Primitive()
	modal.SetRect(0, 0, 80, 24)
	modal.Draw(screen)

	tx, ty, _, _ := pf.TextArea().GetRect()
	_, style, _ := screen.Get(tx+1, ty+1)
	_, bg, _ := style.Decompose()
	if bg != tview.Styles.ContrastBackgroundColor {
		t.Errorf("prompt background = %v, want %v", bg, tview.Styles.ContrastBackgroundColor)
	}

	ex, ey, _, _ := pf.Editor().GetRect()
	_, style, _ = screen.Get(ex+2, ey+2)
	_, bg, _ = style.Decompose()
	if bg != tview.Styles.PrimitiveBackgroundColor {
		t.Errorf("form background = %v, want %v", bg, tview.Styles.PrimitiveBackgroundColor)
	}
}

func TestPromptForm_EditStartsAtTop(t *testing.T) {
	t.Parallel()
	text := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8"
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt(text)})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	drawPromptForm(t, pf)

	row, column := pf.TextArea().GetOffset()
	if row != 0 || column != 0 {
		t.Errorf("initial offset = (%d, %d), want (0, 0)", row, column)
	}
}

func TestPromptForm_Focus(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})

	pf.FocusEditor()
	if !pf.TextArea().HasFocus() {
		t.Fatal("prompt field does not have focus")
	}

	step := func(want tview.Primitive) {
		t.Helper()
		current := app.Focused
		if current == nil {
			current = pf.TextArea()
		}
		var capture func(*tcell.EventKey) *tcell.EventKey
		switch v := current.(type) {
		case *tview.TextArea:
			capture = v.GetInputCapture()
		case *tview.Button:
			capture = v.GetInputCapture()
		case *tview.DropDown:
			capture = v.GetInputCapture()
		case *tview.List:
			capture = v.GetInputCapture()
		}
		if capture == nil {
			t.Fatalf("no input capture on %T", current)
		}
		capture(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		if app.Focused != want {
			t.Errorf("Tab focus = %T, want %T", app.Focused, want)
		}
	}

	step(pf.SavePauseButton())
	step(pf.SaveButton())
	step(pf.CancelButton())
	step(pf.DeleteButton())
	step(pf.Status())
	step(pf.SessionButton())
	step(pf.PromptList())
	step(pf.PasteButton())
	step(pf.ClearButton())
	step(pf.CopyButton())
	step(pf.TextArea())
}

func TestPromptForm_Copy(t *testing.T) {
	t.Parallel()

	t.Run("success copies the exact text", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		pf.TextArea().SetText("unsaved text", true)
		pressButton(t, pf.CopyButton())
		if app.CopiedText != "unsaved text" {
			t.Errorf("copied %q, want unsaved text", app.CopiedText)
		}
		if app.Status != "copied" {
			t.Errorf("status = %q, want copied", app.Status)
		}
	})

	t.Run("failure shows the error and stays open", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		app.CopyErr = errors.New("no clipboard")
		closed := false
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() { closed = true })
		pressButton(t, pf.CopyButton())
		if app.ErrorStatus != "copy failed: no clipboard" {
			t.Errorf("error = %q, want copy failed: no clipboard", app.ErrorStatus)
		}
		if closed {
			t.Error("form closed after a copy failure")
		}
	})
}

func TestPromptForm_Clear(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	pf.TextArea().SetText("unsaved text", true)
	pressButton(t, pf.ClearButton())
	if got := pf.TextArea().GetText(); got != "" {
		t.Errorf("prompt = %q, want empty", got)
	}
	if app.Focused != pf.TextArea() {
		t.Errorf("focus = %T, want prompt field", app.Focused)
	}
}

func TestPromptForm_Paste(t *testing.T) {
	t.Parallel()

	t.Run("replaces the selection", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		app.ClipboardText = "second"
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		pf.TextArea().Select(0, 5)
		pressButton(t, pf.PasteButton())
		if got := pf.TextArea().GetText(); got != "second" {
			t.Errorf("prompt = %q, want second", got)
		}
		if app.Status != "pasted" {
			t.Errorf("status = %q, want pasted", app.Status)
		}
	})

	t.Run("failure shows an error", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		app.ReadErr = errors.New("no clipboard")
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		pressButton(t, pf.PasteButton())
		if app.ErrorStatus != "paste failed: no clipboard" {
			t.Errorf("error = %q, want paste failed: no clipboard", app.ErrorStatus)
		}
	})

	t.Run("empty clipboard leaves the text unchanged", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		pf.TextArea().Select(0, 5)
		pressButton(t, pf.PasteButton())
		if got := pf.TextArea().GetText(); got != "first" {
			t.Errorf("prompt = %q, want first", got)
		}
		if app.Status != "" {
			t.Errorf("status = %q, want empty", app.Status)
		}
	})
}

func TestPromptForm_ClipboardHotkeys(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	textArea := pf.TextArea()
	textArea.SetText("first", false)
	textArea.Select(0, 5)
	textArea.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone), func(tview.Primitive) {})
	if app.CopiedText != "first" {
		t.Errorf("copied = %q, want first", app.CopiedText)
	}

	app.ClipboardText = "second"
	textArea.Select(0, 5)
	textArea.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModNone), func(tview.Primitive) {})
	if got := textArea.GetText(); got != "second" {
		t.Errorf("text = %q, want second", got)
	}
}

func TestPromptForm_ClipboardHotkeyErrors(t *testing.T) {
	t.Parallel()

	t.Run("copy failure sets an error", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		app.CopyErr = errors.New("no clipboard")
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		pf.TextArea().Select(0, 5)
		pf.TextArea().InputHandler()(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone), func(tview.Primitive) {})
		if app.ErrorStatus != "copy: no clipboard" {
			t.Errorf("error = %q, want copy: no clipboard", app.ErrorStatus)
		}
	})

	t.Run("paste failure sets an error", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		app.ReadErr = errors.New("no clipboard")
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		pf.TextArea().Select(0, 5)
		pf.TextArea().InputHandler()(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModNone), func(tview.Primitive) {})
		if app.ErrorStatus != "paste: no clipboard" {
			t.Errorf("error = %q, want paste: no clipboard", app.ErrorStatus)
		}
	})
}

func TestNormalizeClipboardKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		event   *tcell.EventKey
		wantKey tcell.Key
		wantNil bool
	}{
		{"nil event", nil, 0, true},
		{"non-rune event", tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), tcell.KeyEnter, false},
		{"rune without control", tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone), tcell.KeyRune, false},
		{"control q", tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModCtrl|tcell.ModAlt), tcell.KeyCtrlQ, false},
		{"control x", tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModCtrl|tcell.ModAlt), tcell.KeyCtrlX, false},
		{"control v", tcell.NewEventKey(tcell.KeyRune, 'v', tcell.ModCtrl|tcell.ModAlt), tcell.KeyCtrlV, false},
		{"uppercase control q", tcell.NewEventKey(tcell.KeyRune, 'Q', tcell.ModCtrl|tcell.ModAlt), tcell.KeyCtrlQ, false},
		{"unrelated control rune", tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModCtrl|tcell.ModAlt), tcell.KeyRune, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := forms.NormalizeClipboardKey(tc.event)
			if tc.wantNil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil, want an event")
			}
			if got.Key() != tc.wantKey {
				t.Errorf("key = %v, want %v", got.Key(), tc.wantKey)
			}
		})
	}
}

func TestPromptForm_FirstPromptSave(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, nil)
	closed := false
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() { closed = true })
	pf.TextArea().SetText("  first  ", true)
	pressButton(t, pf.SaveButton())
	if !closed {
		t.Error("form did not close after a successful save")
	}
	got := app.Svc.Session(sess.ID()).Queue()
	if len(got) != 1 || got[0].Text() != "first" {
		t.Errorf("queue = %v, want first", got)
	}
}

func TestPromptForm_EditSave(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 1, nil, func() {})
	pf.TextArea().SetText("changed", true)
	pf.Status().SetCurrentOption(int(model.SessionStatusClosed))
	pressButton(t, pf.SaveButton())
	got := app.Svc.Session(sess.ID()).Queue()
	if len(got) != 2 || got[0].Text() != "first" || got[1].Text() != "changed" {
		t.Errorf("queue = %v, want first, changed", got)
	}
	if gotStatus := app.Svc.Session(sess.ID()).Status(); gotStatus != model.SessionStatusClosed {
		t.Errorf("status = %v, want Closed", gotStatus)
	}
}

func TestPromptForm_StatusOnlySave(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		queue []model.Prompt
		want  []string
	}{
		{name: "empty queue", want: nil},
		{name: "unchanged prompt", queue: []model.Prompt{model.NewPrompt("first")}, want: []string{"first"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app, sess := newPromptFormApp(t, tt.queue)
			closed := false
			pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() { closed = true })
			pf.Status().SetCurrentOption(int(model.SessionStatusClosed))
			pressButton(t, pf.SaveButton())

			if !closed {
				t.Error("form did not close after saving the status")
			}
			live := app.Svc.Session(sess.ID())
			if got := live.Status(); got != model.SessionStatusClosed {
				t.Errorf("status = %v, want Closed", got)
			}
			got := live.Queue()
			if len(got) != len(tt.want) {
				t.Fatalf("queue = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i].Text() != tt.want[i] {
					t.Errorf("prompt %d = %q, want %q", i, got[i].Text(), tt.want[i])
				}
			}
		})
	}
}

func TestPromptForm_ListSelectSync(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})

	pf.TextArea().SetText("changed first", true)
	pf.PromptList().SetCurrentItem(1)
	if got := pf.TextArea().GetText(); got != "second" {
		t.Errorf("prompt after selection = %q, want second", got)
	}
	pf.TextArea().SetText("changed second", true)
	pressButton(t, pf.SaveButton())

	got := app.Svc.Session(sess.ID()).Queue()
	if len(got) != 2 || got[0].Text() != "changed first" || got[1].Text() != "changed second" {
		t.Errorf("queue = %v, want changed first, changed second", got)
	}
}

func TestPromptForm_ListAdd(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})

	pf.AddPrompt()
	if got := pf.TextArea().GetText(); got != "" {
		t.Errorf("new prompt = %q, want empty", got)
	}
	if got := pf.PromptList().GetItemCount(); got != 3 {
		t.Errorf("list items = %d, want 3", got)
	}
	pf.TextArea().SetText("inserted", true)
	pressButton(t, pf.SaveButton())

	got := app.Svc.Session(sess.ID()).Queue()
	if len(got) != 3 || got[0].Text() != "first" || got[1].Text() != "inserted" || got[2].Text() != "second" {
		t.Errorf("queue = %v, want first, inserted, second", got)
	}
}

func TestPromptForm_ListDelete(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})

	pf.DeletePrompt()
	if got := pf.PromptList().GetItemCount(); got != 2 {
		t.Fatalf("list items before confirmation = %d, want 2", got)
	}
	modal := confirmModalFrom(t, app)
	pressConfirm(t, modal)

	list := forms.PromptListFromPrimitive(app.Modal)
	if list == nil {
		t.Fatal("prompt form not restored after confirmation")
	}
	if got := list.GetItemCount(); got != 1 {
		t.Errorf("list items = %d, want 1", got)
	}
	if text, _, ok, err := forms.PromptStateFromPrimitive(app.Modal); err != nil || !ok || text != "second" {
		t.Errorf("prompt after delete = %q, want second (err %v)", text, err)
	}
	if app.Focused != list {
		t.Errorf("focus after delete = %T, want prompt list", app.Focused)
	}
	forms.SavePromptFromPrimitive(app.Modal)

	got := app.Svc.Session(sess.ID()).Queue()
	if len(got) != 1 || got[0].Text() != "second" {
		t.Errorf("queue = %v, want second", got)
	}
}

func TestPromptForm_ListDeleteCancel(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	app.ShowModal(pf.Primitive()) //nolint:errcheck // Test setup.

	pf.DeletePrompt()
	modal := confirmModalFrom(t, app)
	pressCancel(t, modal)

	list := forms.PromptListFromPrimitive(app.Modal)
	if list == nil {
		t.Fatal("prompt form not restored after cancellation")
	}
	if got := list.GetItemCount(); got != 2 {
		t.Errorf("list items = %d, want 2", got)
	}
	if text, _, ok, err := forms.PromptStateFromPrimitive(app.Modal); err != nil || !ok || text != "first" {
		t.Errorf("prompt after cancel = %q, want first (err %v)", text, err)
	}
}

func TestPromptForm_ListDeleteLast(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})

	pf.DeletePrompt()
	modal := confirmModalFrom(t, app)
	pressConfirm(t, modal)

	list := forms.PromptListFromPrimitive(app.Modal)
	if list == nil {
		t.Fatal("prompt form not restored after confirmation")
	}
	if got := list.GetItemCount(); got != 0 {
		t.Errorf("list items = %d, want 0", got)
	}
	if text, _, ok, err := forms.PromptStateFromPrimitive(app.Modal); err != nil || !ok || text != "" {
		t.Errorf("prompt after delete = %q, want empty (err %v)", text, err)
	}
	forms.SavePromptFromPrimitive(app.Modal)

	if got := app.Svc.Session(sess.ID()).Queue(); len(got) != 0 {
		t.Errorf("queue = %v, want empty", got)
	}
}

func TestPromptForm_SaveAndPause(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	closed, paused := false, false
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {
		closed = true
	}, func() {
		paused = true
	})

	pf.TextArea().SetText("changed", true)
	pressButton(t, pf.SavePauseButton())

	if got := app.Svc.Session(sess.ID()).Queue()[0].Text(); got != "changed" {
		t.Errorf("prompt = %q, want changed", got)
	}
	if !closed {
		t.Error("prompt form was not closed")
	}
	if !paused {
		t.Error("pause callback was not called")
	}
}

func TestPromptForm_SaveAndPauseFailureDoesNotPause(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	closed, paused := false, false
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {
		closed = true
	}, func() {
		paused = true
	})

	pf.TextArea().SetText("   ", true)
	pressButton(t, pf.SavePauseButton())

	if app.ErrorStatus == "" {
		t.Error("expected a validation error")
	}
	if closed {
		t.Error("prompt form closed after a failed save")
	}
	if paused {
		t.Error("pause callback was called after a failed save")
	}
}

func TestPromptForm_DeleteButtonSavesImmediately(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	app.ShowModal(pf.Primitive()) //nolint:errcheck // Test setup.

	pf.TextArea().SetText("changed first", true)
	pf.PromptList().SetCurrentItem(1)
	pf.TextArea().SetText("changed second", true)
	pf.PromptList().SetCurrentItem(0)

	pressButton(t, pf.DeleteButton())
	modal := confirmModalFrom(t, app)
	pressConfirm(t, modal)

	got := app.Svc.Session(sess.ID()).Queue()
	if len(got) != 1 || got[0].Text() != "changed second" {
		t.Errorf("queue = %v, want changed second", got)
	}
	list := forms.PromptListFromPrimitive(app.Modal)
	if list == nil {
		t.Fatal("prompt form not restored after delete")
	}
	if got := list.GetItemCount(); got != 1 {
		t.Errorf("list items = %d, want 1", got)
	}
	if text, _, ok, err := forms.PromptStateFromPrimitive(app.Modal); err != nil || !ok || text != "changed second" {
		t.Errorf("prompt after delete = %q, want changed second (err %v)", text, err)
	}
	if app.Focused != list {
		t.Errorf("focus after delete = %T, want prompt list", app.Focused)
	}
}

func TestPromptForm_DeleteButtonLastPromptCloses(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	closed := false
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {
		closed = true
		app.HideModal()
	})
	app.ShowModal(pf.Primitive()) //nolint:errcheck // Test setup.

	pressButton(t, pf.DeleteButton())
	modal := confirmModalFrom(t, app)
	pressConfirm(t, modal)

	if !closed {
		t.Error("form did not close after deleting the last prompt")
	}
	if app.Modal != nil {
		t.Errorf("modal = %T, want nil", app.Modal)
	}
	if got := app.Svc.Session(sess.ID()).Queue(); len(got) != 0 {
		t.Errorf("queue = %v, want empty", got)
	}
}

func TestPromptForm_DeleteButtonCancel(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	app.ShowModal(pf.Primitive()) //nolint:errcheck // Test setup.

	pressButton(t, pf.DeleteButton())
	modal := confirmModalFrom(t, app)
	pressCancel(t, modal)

	got := app.Svc.Session(sess.ID()).Queue()
	if len(got) != 2 || got[0].Text() != "first" || got[1].Text() != "second" {
		t.Errorf("queue = %v, want first, second", got)
	}
	list := forms.PromptListFromPrimitive(app.Modal)
	if list == nil {
		t.Fatal("prompt form not restored after cancel")
	}
	if got := list.GetItemCount(); got != 2 {
		t.Errorf("list items = %d, want 2", got)
	}
}

func TestPromptForm_ListFocusHints(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})

	app.SetFocus(pf.PromptList())
	if got := app.HintsSpecific; got != views.HintsPromptListSpecific {
		t.Errorf("specific hints = %q, want prompt-list hints", got)
	}
	if got := app.HintsCommon; got != views.HintsFormCommon {
		t.Errorf("common hints = %q, want form hints", got)
	}
}

func TestPromptForm_ListHotkeys(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	capture := pf.PromptList().GetInputCapture()
	if capture == nil {
		t.Fatal("prompt list has no input capture")
	}

	if got := capture(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)); got == nil {
		t.Error("list navigation key was consumed")
	}
	capture(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.Focused != pf.TextArea() {
		t.Errorf("Enter focus = %T, want prompt field", app.Focused)
	}
	capture(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if got := pf.PromptList().GetItemCount(); got != 3 {
		t.Errorf("list items after Insert = %d, want 3", got)
	}
	capture(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
	modal := confirmModalFrom(t, app)
	pressConfirm(t, modal)
	list := forms.PromptListFromPrimitive(app.Modal)
	if list == nil {
		t.Fatal("prompt form not restored after Delete")
	}
	if got := list.GetItemCount(); got != 2 {
		t.Errorf("list items after Delete = %d, want 2", got)
	}
}

func TestPromptForm_ListClickFocusesAndInsertWorks(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	root := pf.Primitive()
	root.SetRect(0, 0, 80, 24)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	root.Draw(screen)
	list := pf.PromptList()
	x, y, _, _ := list.GetRect()
	event := tcell.NewEventMouse(x+2, y+2, tcell.ButtonPrimary, tcell.ModNone)
	consumed, _ := root.MouseHandler()(tview.MouseLeftClick, event, app.SetFocus)
	if !consumed {
		t.Fatal("click was not consumed")
	}
	if app.Focused != list {
		t.Fatalf("focus = %T, want prompt list", app.Focused)
	}
	list.InputHandler()(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone), app.SetFocus)
	if got := list.GetItemCount(); got != 3 {
		t.Errorf("list items after Insert = %d, want 3", got)
	}
}

func TestPromptForm_ListInputHandlerInsert(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	list := pf.PromptList()
	list.SetRect(0, 0, 40, 10)
	list.InputHandler()(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone), app.SetFocus)
	if got := list.GetItemCount(); got != 3 {
		t.Errorf("list items after Insert = %d, want 3", got)
	}
}

func TestPromptForm_ListDoubleClickFocusesPrompt(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	list := pf.PromptList()
	list.SetRect(0, 0, 40, 10)
	capture := list.GetMouseCapture()
	if capture == nil {
		t.Fatal("prompt list has no mouse capture")
	}

	event := tcell.NewEventMouse(2, 2, tcell.ButtonPrimary, tcell.ModNone)
	action, gotEvent := capture(tview.MouseLeftDoubleClick, event)
	if action != tview.MouseConsumed || gotEvent != nil {
		t.Fatalf("mouse capture = (%v, %v), want (%v, nil)", action, gotEvent, tview.MouseConsumed)
	}
	if app.Focused != pf.TextArea() {
		t.Errorf("double-click focus = %T, want prompt field", app.Focused)
	}
}

func TestPromptForm_EmptyText(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	pf.TextArea().SetText("   ", true)
	pressButton(t, pf.SaveButton())
	if app.ErrorStatus == "" {
		t.Error("expected a validation error for empty text")
	}
	if got := app.Svc.Session(sess.ID()).Queue(); len(got) != 1 {
		t.Errorf("queue changed after a validation error: %v", got)
	}
}

func TestPromptForm_SessionModal(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	app.ShowModal(pf.Primitive()) //nolint:errcheck // Test setup.
	pf.TextArea().SetText("unsaved prompt", true)
	pf.Status().SetCurrentOption(int(model.SessionStatusNeedsClosing))

	pressButton(t, pf.SessionButton())
	form := formFromCenteredModal(t, app.Modal)
	if got := form.GetTitle(); got != " Edit Session " {
		t.Errorf("title = %q, want Edit Session", got)
	}
	if form.GetFormItemByLabel("Status") != nil {
		t.Error("session form must not contain Status")
	}

	pressButton(t, form.GetButton(1)) // Cancel
	text, status, ok, err := forms.PromptStateFromPrimitive(app.Modal)
	must(t, err)
	if !ok {
		t.Fatal("session form did not return to the prompt form")
	}
	if text != "unsaved prompt" {
		t.Errorf("prompt = %q, want unsaved prompt", text)
	}
	if status != model.SessionStatusNeedsClosing {
		t.Errorf("status = %v, want Needs closing", status)
	}
}

func TestNewPromptForm_Guards(t *testing.T) {
	t.Parallel()

	t.Run("nil session", func(t *testing.T) {
		t.Parallel()
		app, _ := newPromptFormApp(t, nil)
		prim := forms.NewPromptForm(app, nil, forms.PromptFormOptions{})
		if _, ok := views.UnwrapModal(prim).(*tview.Modal); !ok {
			t.Errorf("got %T, want error modal", views.UnwrapModal(prim))
		}
	})

	t.Run("missing session", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, nil)
		must(t, app.Svc.DeleteSession("alice@example.com", "claude", sess.ID()))
		prim := forms.NewPromptForm(app, sess, forms.PromptFormOptions{})
		if _, ok := views.UnwrapModal(prim).(*tview.Modal); !ok {
			t.Errorf("got %T, want error modal", views.UnwrapModal(prim))
		}
	})

	t.Run("out of range index leaves no selection", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		prim := forms.NewPromptForm(app, sess, forms.PromptFormOptions{Index: 5})
		text, _, ok, err := forms.PromptStateFromPrimitive(prim)
		must(t, err)
		if !ok {
			t.Fatal("got an error modal, want the session editor")
		}
		if text != "" {
			t.Errorf("prompt = %q, want no selection", text)
		}
		list := forms.PromptListFromPrimitive(prim)
		if list == nil || list.GetItemCount() != 1 {
			t.Errorf("list = %v, want one prompt", list)
		}
	})
}

func TestPromptForm_EmptyQueue(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, nil)
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})

	list := pf.PromptList()
	if list == nil {
		t.Fatal("session editor has no prompt list")
	}
	if got := list.GetItemCount(); got != 0 {
		t.Errorf("list items = %d, want 0", got)
	}
	if !pf.DeleteButton().IsDisabled() {
		t.Error("Delete must be disabled with no prompts")
	}

	pf.AddPrompt()
	if got := list.GetItemCount(); got != 1 {
		t.Errorf("list items after add = %d, want 1", got)
	}
	if got := pf.TextArea().GetText(); got != "" {
		t.Errorf("new prompt = %q, want empty", got)
	}
	if app.Focused != pf.TextArea() {
		t.Errorf("focus = %T, want the prompt field", app.Focused)
	}

	pf.DeletePrompt()
	if app.Modal == nil {
		t.Error("Delete on a prompt must open a confirmation modal")
	}
}

func TestPromptForm_SaveFailures(t *testing.T) {
	t.Parallel()

	t.Run("empty text saves nothing", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, nil)
		closed := false
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() { closed = true })
		pf.TextArea().SetText("   ", true)
		pressButton(t, pf.SaveButton())
		if app.ErrorStatus != "" {
			t.Errorf("error = %q, want none", app.ErrorStatus)
		}
		if !closed {
			t.Error("form did not close")
		}
		if got := app.Svc.Session(sess.ID()).Queue(); len(got) != 0 {
			t.Errorf("queue = %v, want empty", got)
		}
	})

	t.Run("add mutation failure", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, nil)
		app.MutateErr = errors.New("save failed")
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		pf.TextArea().SetText("first", true)
		pressButton(t, pf.SaveButton())
		if app.ErrorStatus == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})

	t.Run("edit mutation failure", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		app.MutateErr = errors.New("save failed")
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		pressButton(t, pf.SaveButton())
		if app.ErrorStatus == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})

	t.Run("session deleted before save", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		must(t, app.Svc.DeleteSession("alice@example.com", "claude", sess.ID()))
		pf.TextArea().SetText("changed", true)
		pressButton(t, pf.SaveButton())
		if app.ErrorStatus == "" {
			t.Error("expected a status message for the deleted session")
		}
	})
}

func TestPromptForm_DeleteMutationFailure(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	app.ShowModal(pf.Primitive()) //nolint:errcheck // Test setup.

	pressButton(t, pf.DeleteButton())
	modal := confirmModalFrom(t, app)
	app.MutateErr = errors.New("delete failed")
	pressConfirm(t, modal)

	if app.ErrorStatus == "" {
		t.Error("expected a status message reporting the delete failure")
	}
	if app.Modal == nil {
		t.Error("prompt form closed after the delete failure")
	}
}

func TestPromptForm_StatusOutOfRange(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	pf.Status().SetCurrentOption(-1)

	forms.SavePromptFromPrimitive(pf.Primitive())

	if got := app.Svc.Session(sess.ID()).Status(); got != model.SessionStatusActive {
		t.Errorf("status = %v, want Active", got)
	}
}

func TestPromptForm_OpenSessionErrors(t *testing.T) {
	t.Parallel()

	t.Run("missing session", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		app.ShowModal(pf.Primitive()) //nolint:errcheck // Test setup.
		must(t, app.Svc.DeleteSession("alice@example.com", "claude", sess.ID()))

		pressButton(t, pf.SessionButton())

		if app.ErrorStatus != "session no longer exists" {
			t.Errorf("error = %q, want session no longer exists", app.ErrorStatus)
		}
	})

	t.Run("session save failure", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		app.ShowModal(pf.Primitive()) //nolint:errcheck // Test setup.

		pressButton(t, pf.SessionButton())
		form := formFromCenteredModal(t, app.Modal)
		app.MutateErr = errors.New("save failed")
		triggerSave(t, form)

		if app.ErrorStatus == "" {
			t.Error("expected a status message reporting the session save failure")
		}
	})
}

func TestPromptForm_ListCaptureKeys(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first"), model.NewPrompt("second")})
	closed := false
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() { closed = true })
	capture := pf.PromptList().GetInputCapture()

	if got := capture(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)); got != nil {
		t.Error("Backtab was not consumed")
	}
	capture(tcell.NewEventKey(views.KeySave, 0, tcell.ModNone))
	capture(tcell.NewEventKey(views.KeyCancel, 0, tcell.ModNone))

	if !closed {
		t.Error("Cancel did not close the form")
	}
}

func TestPromptForm_TextAreaCaptureKeys(t *testing.T) {
	t.Parallel()

	t.Run("backtab moves focus", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
		pf.FocusEditor()
		capture := pf.TextArea().GetInputCapture()
		if got := capture(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)); got != nil {
			t.Error("Backtab was not consumed")
		}
		if app.Focused != pf.CopyButton() {
			t.Errorf("focus = %T, want Copy button", app.Focused)
		}
	})

	t.Run("save closes", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		closed := false
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() { closed = true })
		pf.TextArea().GetInputCapture()(tcell.NewEventKey(views.KeySave, 0, tcell.ModNone))
		if !closed {
			t.Error("Save did not close the form")
		}
	})

	t.Run("cancel closes", func(t *testing.T) {
		t.Parallel()
		app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt("first")})
		closed := false
		pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() { closed = true })
		pf.TextArea().GetInputCapture()(tcell.NewEventKey(views.KeyCancel, 0, tcell.ModNone))
		if !closed {
			t.Error("Cancel did not close the form")
		}
	})
}

func TestPromptPreview(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		want string
	}{
		{"single line", "hello", "hello"},
		{"multiline", "first\nsecond", "first"},
		{"blank", "   ", "(empty)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := forms.PromptPreview(tc.text); got != tc.want {
				t.Errorf("Preview(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestPromptForm_LongListItemEllipsis(t *testing.T) {
	t.Parallel()
	app, sess := newPromptFormApp(t, []model.Prompt{model.NewPrompt(strings.Repeat("x", 100))})
	pf := forms.NewPromptFormForTest(app, sess, 0, nil, func() {})
	drawPromptForm(t, pf)

	list := pf.PromptList()
	if list == nil {
		t.Fatal("edit form has no prompt list")
	}
	got, _ := list.GetItemText(0)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("long list item = %q, want an ellipsis", got)
	}
	_, _, width, _ := list.GetInnerRect()
	if gotWidth := tview.TaggedStringWidth(got); gotWidth > width {
		t.Errorf("list item width = %d, want at most %d", gotWidth, width)
	}
}
