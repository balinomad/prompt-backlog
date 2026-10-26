package views_test

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── Test fixture ──────────────────────────────────────────────────────────────

// newQueueTestApp returns a mock wired with a single account holding one
// session whose queue has three prompts, in order: "first" (no spawn),
// "second" (spawns), "third" (no spawn). Defined locally for the same
// reason newDetailTestApp is: this file is the only current caller that
// needs a session with a specific, multi-entry queue shape.
func newQueueTestApp(t *testing.T) (*mockApp, *model.Account, *model.Session) {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	sess, err := reg.AddSession(acc, nil, "topic", []model.Prompt{
		model.NewPrompt("first", false),
		model.NewPrompt("second", true),
		model.NewPrompt("third", false),
	})
	must(t, err)

	app := &mockApp{svc: newTestService(t, reg)}
	return app, acc, sess
}

// ── NewQueueEditor: preconditions and list population ─────────────────────────

func TestNewQueueEditor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(t *testing.T) (app *mockApp, account *model.Account, session *model.Session)
		wantModal bool
		wantItems int
	}{
		{
			name: "nil account",
			setup: func(t *testing.T) (*mockApp, *model.Account, *model.Session) {
				t.Helper()
				app, _, sess := newQueueTestApp(t)
				return app, nil, sess
			},
			wantModal: true,
		},
		{
			name: "nil session",
			setup: func(t *testing.T) (*mockApp, *model.Account, *model.Session) {
				t.Helper()
				app, acc, _ := newQueueTestApp(t)
				return app, acc, nil
			},
			wantModal: true,
		},
		{
			name: "session no longer exists",
			setup: func(t *testing.T) (*mockApp, *model.Account, *model.Session) {
				t.Helper()
				app, acc, sess := newQueueTestApp(t)
				must(t, app.svc.EndSession(acc.UserEmail(), acc.VendorName(), sess.ID()))
				return app, acc, sess // sess is now a stale pointer to a deleted session
			},
			wantModal: true,
		},
		{
			name: "returns list",
			setup: func(t *testing.T) (*mockApp, *model.Account, *model.Session) {
				t.Helper()
				return newQueueTestApp(t)
			},
			wantModal: false,
			wantItems: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc, sess := tc.setup(t)
			result := views.NewQueueEditor(app, acc, sess, func() {})
			if tc.wantModal {
				if _, ok := result.(*tview.Modal); !ok {
					t.Errorf("expected error modal, got %T", result)
				}
				return
			}
			list, ok := result.(*tview.List)
			if !ok {
				t.Fatalf("expected *tview.List, got %T", result)
			}
			if got := list.GetItemCount(); got != tc.wantItems {
				t.Errorf("item count: got %d, want %d", got, tc.wantItems)
			}
		})
	}
}

// ── handleKey: Esc and passthrough ──────────────────────────────────────────

// TestQueueEditor_HandleKey_Esc covers handleKey's own KeyCancel branch,
// distinct from Esc on any of the modals it opens (help overlay, confirm
// modals), which are covered separately below.
func TestQueueEditor_HandleKey_Esc(t *testing.T) {
	t.Parallel()
	app, acc, sess := newQueueTestApp(t)
	closed := false
	result := views.NewQueueEditor(app, acc, sess, func() { closed = true })
	list, ok := result.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List, got %T", result)
	}

	capture := list.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	if got := capture(escKey()); got != nil {
		t.Error("Esc must consume the event (return nil)")
	}
	if !closed {
		t.Error("Esc must call onClose")
	}
}

// TestQueueEditor_HandleKey_PassesThroughUnhandledKeys covers the two
// remaining unhandled paths through handleKey: a non-rune key other than
// Esc, and a rune that matches none of the KeyX cases.
func TestQueueEditor_HandleKey_PassesThroughUnhandledKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		event        *tcell.EventKey
		wantConsumed bool
	}{
		{"unhandled special key", specialKey(tcell.KeyF1), false},
		{"unrecognized rune", runeKey('z'), false},
		{"tab", specialKey(views.KeyFocusNext), true},
		{"backtab", specialKey(views.KeyFocusPrev), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc, sess := newQueueTestApp(t)
			result := views.NewQueueEditor(app, acc, sess, func() {})
			list, ok := result.(*tview.List)
			if !ok {
				t.Fatalf("expected *tview.List, got %T", result)
			}
			got := list.GetInputCapture()(tc.event)
			if tc.wantConsumed {
				if got != nil {
					t.Error("key must be consumed (return nil)")
				}
				return
			}
			if got != tc.event {
				t.Error("key must pass through unchanged")
			}
		})
	}
}

// ── Key handling: opens the right modal ────────────────────────────────────────

func TestQueueEditor_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		key          *tcell.EventKey
		wantSpecific string
		wantCommon   string
	}{
		{"new", specialKey(views.KeyInsert), views.HintsFormSpecific, views.HintsFormCommon},
		{"edit", specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsFormCommon},
		{"delete", specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc, sess := newQueueTestApp(t)
			result := views.NewQueueEditor(app, acc, sess, func() {})
			list, ok := result.(*tview.List)
			if !ok {
				t.Fatalf("expected *tview.List, got %T", result)
			}
			capture := list.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.modal == nil {
				t.Errorf("key %q: expected modal to be shown", tc.name)
			}
			assertHints(t, app, tc.wantSpecific, tc.wantCommon)
		})
	}
}

// TestQueueEditor_EmptyQueue_NoModal builds its own minimal fixture because
// only a single account with one empty-queue session is needed here.
func TestQueueEditor_EmptyQueue_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  *tcell.EventKey
	}{
		{"edit", specialKey(views.KeyEdit)},
		{"delete", specialKey(views.KeyDelete)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
			must(t, err)
			emptySess, err := reg.AddSession(acc, nil, "", nil)
			must(t, err)
			app := &mockApp{svc: newTestService(t, reg)}

			result := views.NewQueueEditor(app, acc, emptySess, func() {})
			list, ok := result.(*tview.List)
			if !ok {
				t.Fatalf("expected *tview.List, got %T", result)
			}
			capture := list.GetInputCapture()
			capture(tc.key)
			if app.modal != nil {
				t.Errorf("key %q with empty queue should not open a modal", tc.name)
			}
		})
	}
}

// ── Reorder: commits immediately, no modal involved ───────────────────────────

func TestQueueEditor_Reorder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       *tcell.EventKey
		startIdx  int
		wantOrder []string
		wantIdx   int
	}{
		{
			name:      "move up from first item is a no-op",
			key:       ctrlKey(tcell.KeyUp),
			startIdx:  0,
			wantOrder: []string{"first", "second", "third"},
			wantIdx:   0,
		},
		{
			name:      "move up swaps with previous",
			key:       ctrlKey(tcell.KeyUp),
			startIdx:  1,
			wantOrder: []string{"second", "first", "third"},
			wantIdx:   0,
		},
		{
			name:      "move down from last item is a no-op",
			key:       ctrlKey(tcell.KeyDown),
			startIdx:  2,
			wantOrder: []string{"first", "second", "third"},
			wantIdx:   2,
		},
		{
			name:      "move down swaps with next",
			key:       ctrlKey(tcell.KeyDown),
			startIdx:  0,
			wantOrder: []string{"second", "first", "third"},
			wantIdx:   1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc, sess := newQueueTestApp(t)
			result := views.NewQueueEditor(app, acc, sess, func() {})
			list, ok := result.(*tview.List)
			if !ok {
				t.Fatalf("expected *tview.List, got %T", result)
			}
			list.SetCurrentItem(tc.startIdx)

			capture := list.GetInputCapture()
			capture(tc.key)

			if got := list.GetCurrentItem(); got != tc.wantIdx {
				t.Errorf("selected index after reorder: got %d, want %d", got, tc.wantIdx)
			}

			updated := app.svc.Session(sess.ID())
			if updated == nil {
				t.Fatal("session not found after reorder")
			}
			queue := updated.Queue()
			if len(queue) != len(tc.wantOrder) {
				t.Fatalf("queue length: got %d, want %d", len(queue), len(tc.wantOrder))
			}
			for i, want := range tc.wantOrder {
				if queue[i].Text() != want {
					t.Errorf("queue[%d] = %q, want %q", i, queue[i].Text(), want)
				}
			}
		})
	}
}

func TestQueueEditor_MoveAliasesRemoved(t *testing.T) {
	t.Parallel()
	for _, key := range []rune{'K', 'J'} {
		t.Run(string(key), func(t *testing.T) {
			t.Parallel()
			app, acc, sess := newQueueTestApp(t)
			result := views.NewQueueEditor(app, acc, sess, func() {})
			list, ok := result.(*tview.List)
			if !ok {
				t.Fatalf("expected *tview.List, got %T", result)
			}
			list.SetCurrentItem(1)

			event := runeKey(key)
			if got := list.GetInputCapture()(event); got != event {
				t.Error("removed alias must pass through unchanged")
			}
			if got := list.GetCurrentItem(); got != 1 {
				t.Errorf("selection changed: got %d, want 1", got)
			}

			updated := app.svc.Session(sess.ID())
			if updated == nil {
				t.Fatal("session not found")
			}
			queue := updated.Queue()
			want := []string{"first", "second", "third"}
			for i, text := range want {
				if queue[i].Text() != text {
					t.Errorf("queue[%d] = %q, want %q", i, queue[i].Text(), text)
				}
			}
		})
	}
}

func TestQueueEditor_Reorder_CommitFailure(t *testing.T) {
	t.Parallel()
	app, acc, sess := newQueueTestApp(t)
	result := views.NewQueueEditor(app, acc, sess, func() {})
	list, ok := result.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List, got %T", result)
	}

	// Remove the session out from under the editor so the next commit's
	// UpdateSessionPrompts call fails at resolveSession.
	must(t, app.svc.EndSession(acc.UserEmail(), acc.VendorName(), sess.ID()))

	list.SetCurrentItem(1)
	capture := list.GetInputCapture()
	capture(ctrlKey(tcell.KeyUp))

	if app.status == "" {
		t.Error("expected a status message reporting the commit failure")
	}
}

// ── New / edit prompt: opens the form ──────────────────────────────────────────

// TestQueueEditor_NewAndEditPrompt_OpensForm verifies both keys open a
// centred prompt form.
func TestQueueEditor_NewAndEditPrompt_OpensForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  *tcell.EventKey
	}{
		{"new prompt", specialKey(views.KeyInsert)},
		{"edit existing prompt", specialKey(views.KeyEdit)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc, sess := newQueueTestApp(t)
			result := views.NewQueueEditor(app, acc, sess, func() {})
			list, ok := result.(*tview.List)
			if !ok {
				t.Fatalf("expected *tview.List, got %T", result)
			}
			list.SetCurrentItem(0)
			list.GetInputCapture()(tc.key)
			if _, ok := app.modal.(*tview.Flex); !ok {
				t.Errorf("key %q: expected centred form (*tview.Flex), got %T", tc.name, app.modal)
			}
		})
	}
}

// TestQueueEditor_EditPrompt_PrefillsExistingValues strengthens the
// "edit existing prompt" case above, which only checks that *a* form
// opens: this checks the form is pre-filled with the *selected* prompt's
// own text and spawn state, not just any form.
func TestQueueEditor_EditPrompt_PrefillsExistingValues(t *testing.T) {
	t.Parallel()
	app, acc, sess := newQueueTestApp(t)
	result := views.NewQueueEditor(app, acc, sess, func() {})
	list, ok := result.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List, got %T", result)
	}
	list.SetCurrentItem(1) // "second", spawns: true

	list.GetInputCapture()(specialKey(views.KeyEdit))
	form := formFromCenteredModal(t, app.modal)

	textField, ok := form.GetFormItemByLabel("Text").(*tview.InputField)
	if !ok {
		t.Fatal("expected Text *tview.InputField")
	}
	if got := textField.GetText(); got != "second" {
		t.Errorf("Text = %q, want %q", got, "second")
	}

	spawnCheckbox, ok := form.GetFormItemByLabel("Spawns new session").(*tview.Checkbox)
	if !ok {
		t.Fatal("expected Spawns new session *tview.Checkbox")
	}
	if !spawnCheckbox.IsChecked() {
		t.Error("Spawns new session should be checked for a prompt that spawns a session")
	}
}

// TestQueueEditor_NewPrompt_Save covers newPromptForm's isNew branch:
// the new prompt is appended, committed, and the editor reopens showing
// the updated queue.
func TestQueueEditor_NewPrompt_Save(t *testing.T) {
	t.Parallel()
	app, acc, sess := newQueueTestApp(t)
	result := views.NewQueueEditor(app, acc, sess, func() {})
	list, ok := result.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List, got %T", result)
	}

	list.GetInputCapture()(specialKey(views.KeyInsert))
	form := formFromCenteredModal(t, app.modal)

	setInputField(t, form, "Text", "fourth")
	setCheckbox(t, form, "Spawns new session")
	triggerSave(t, form)

	updated := app.svc.Session(sess.ID())
	if updated == nil {
		t.Fatal("session no longer exists")
	}
	queue := updated.Queue()
	if len(queue) != 4 {
		t.Fatalf("queue length: got %d, want 4", len(queue))
	}
	last := queue[3]
	if last.Text() != "fourth" || !last.SpawnsSession() {
		t.Errorf("queue[3] = %+v, want text %q spawns=true", last, "fourth")
	}

	// commit() succeeded, so save() calls onDone (qe.reopen): the modal
	// becomes a fresh queue editor, not the prompt form staying open.
	newList, ok := app.modal.(*tview.List)
	if !ok {
		t.Fatalf("expected reopen to show the queue editor (*tview.List), got %T", app.modal)
	}
	if got := newList.GetItemCount(); got != 4 {
		t.Errorf("reopened list item count: got %d, want 4", got)
	}
}

// TestQueueEditor_EditPrompt_Save covers newPromptForm's !isNew branch:
// the edited prompt replaces its entry in place, the queue length is
// unchanged, and the other entries are untouched.
func TestQueueEditor_EditPrompt_Save(t *testing.T) {
	t.Parallel()
	app, acc, sess := newQueueTestApp(t)
	result := views.NewQueueEditor(app, acc, sess, func() {})
	list, ok := result.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List, got %T", result)
	}
	list.SetCurrentItem(0) // "first"

	list.GetInputCapture()(specialKey(views.KeyEdit))
	form := formFromCenteredModal(t, app.modal)

	setInputField(t, form, "Text", "first, edited")
	triggerSave(t, form)

	updated := app.svc.Session(sess.ID())
	if updated == nil {
		t.Fatal("session no longer exists")
	}
	queue := updated.Queue()
	if len(queue) != 3 {
		t.Fatalf("queue length: got %d, want 3 (edit in place, not append)", len(queue))
	}
	if queue[0].Text() != "first, edited" {
		t.Errorf("queue[0].Text() = %q, want %q", queue[0].Text(), "first, edited")
	}
	if queue[1].Text() != "second" || queue[2].Text() != "third" {
		t.Errorf("queue[1:] changed unexpectedly: %+v", queue[1:])
	}
}

// TestQueueEditor_PromptSave_Failure covers save()'s own guard: when
// commit fails, save returns before calling onDone, so the prompt form
// stays open for a retry rather than the editor reopening underneath a
// change that was never actually persisted.
func TestQueueEditor_PromptSave_Failure(t *testing.T) {
	t.Parallel()
	app, acc, sess := newQueueTestApp(t)
	result := views.NewQueueEditor(app, acc, sess, func() {})
	list, ok := result.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List, got %T", result)
	}

	list.GetInputCapture()(specialKey(views.KeyInsert))
	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Text", "will not be saved")

	must(t, app.svc.EndSession(acc.UserEmail(), acc.VendorName(), sess.ID()))

	triggerSave(t, form)

	if app.status == "" {
		t.Error("expected a status message reporting the save failure")
	}
	if _, ok := app.modal.(*tview.Flex); !ok {
		t.Errorf("expected the form to stay open (*tview.Flex) on failure, got %T", app.modal)
	}
}

// ── promptItemText ──────────────────────────────────────────────────────────────

// TestQueueEditor_RowText verifies the rendered prompt row.
func TestQueueEditor_RowText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		text         string
		spawns       bool
		wantContains string
		wantEqual    string
	}{
		{"no spawn returns plain text", "continue", false, "", " continue"},
		{"spawns appends marker", "start fresh", true, "new session", ""},
		{"bracket in text is escaped", "do [x]", false, "", " " + tview.Escape("do [x]")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
			must(t, err)
			sess, err := reg.AddSession(acc, nil, "", []model.Prompt{model.NewPrompt(tc.text, tc.spawns)})
			must(t, err)
			app := &mockApp{svc: newTestService(t, reg)}

			result := views.NewQueueEditor(app, acc, sess, func() {})
			list, ok := result.(*tview.List)
			if !ok {
				t.Fatalf("expected *tview.List, got %T", result)
			}
			got, _ := list.GetItemText(0)
			if !strings.HasPrefix(got, " ") {
				t.Errorf("got %q, want a leading one-cell inset", got)
			}

			if tc.wantEqual != "" && got != tc.wantEqual {
				t.Errorf("got %q, want %q", got, tc.wantEqual)
			}
			if tc.wantContains != "" && !strings.Contains(got, tc.wantContains) {
				t.Errorf("got %q, want it to contain %q", got, tc.wantContains)
			}
		})
	}
}

// ── Help / Quit: cancel reopens the editor, doesn't just hide it ──────────────

func TestQueueEditor_HelpAndQuit_CancelReopensEditor(t *testing.T) {
	t.Parallel()

	t.Run("help", func(t *testing.T) {
		t.Parallel()
		app, acc, sess := newQueueTestApp(t)
		result := views.NewQueueEditor(app, acc, sess, func() {})
		list, ok := result.(*tview.List)
		if !ok {
			t.Fatalf("expected *tview.List, got %T", result)
		}
		list.GetInputCapture()(runeKey(views.KeyHelp))

		helpOverlayTextView(t, app.modal).GetInputCapture()(escKey())

		if _, ok := app.modal.(*tview.List); !ok {
			t.Fatalf("expected Esc on help to reopen the queue editor (*tview.List), got %T", app.modal)
		}
		assertHints(t, app, views.HintsQueueEditorSpecific, views.HintsQueueEditorCommon)
	})

	t.Run("quit", func(t *testing.T) {
		t.Parallel()
		app, acc, sess := newQueueTestApp(t)
		result := views.NewQueueEditor(app, acc, sess, func() {})
		list, ok := result.(*tview.List)
		if !ok {
			t.Fatalf("expected *tview.List, got %T", result)
		}
		list.GetInputCapture()(runeKey(views.KeyQuit))

		confirmModal := confirmModalFrom(t, app.modal)
		confirmModal.GetInputCapture()(escKey())

		if _, ok := app.modal.(*tview.List); !ok {
			t.Fatalf("expected cancelling quit to reopen the queue editor (*tview.List), got %T", app.modal)
		}
		assertHints(t, app, views.HintsQueueEditorSpecific, views.HintsQueueEditorCommon)
		if app.quitted {
			t.Error("cancelling quit must not actually quit")
		}
	})
}

// ── reopen: session vanished before it re-fires ────────────────────────────────

// TestQueueEditor_Reopen_SessionGone covers reopen()'s own dependency on
// NewQueueEditor's nil-session check: if the session disappears while
// some other modal (here, the help overlay) is open on top of the
// editor, reopen must show the resulting error modal rather than assume
// a fresh queue editor is always what NewQueueEditor hands back.
func TestQueueEditor_Reopen_SessionGone(t *testing.T) {
	t.Parallel()
	app, acc, sess := newQueueTestApp(t)
	result := views.NewQueueEditor(app, acc, sess, func() {})
	list, ok := result.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List, got %T", result)
	}

	list.GetInputCapture()(runeKey(views.KeyHelp))
	must(t, app.svc.EndSession(acc.UserEmail(), acc.VendorName(), sess.ID()))

	helpOverlayTextView(t, app.modal).GetInputCapture()(escKey())

	if _, ok := app.modal.(*tview.Modal); !ok {
		t.Errorf("expected reopen to show an error modal (*tview.Modal) once the session is gone, got %T", app.modal)
	}
}
