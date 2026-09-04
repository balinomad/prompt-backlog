package views_test

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

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
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
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
				must(t, app.svc.EndSession(acc.UserEmail(), acc.ProviderName(), sess.ID()))
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

// ── Key handling: opens the right modal ────────────────────────────────────────

func TestQueueEditor_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       rune
		wantHints string
	}{
		{"new", views.KeyNew, views.HintsForm},
		{"edit", views.KeyEdit, views.HintsForm},
		{"delete", views.KeyDelete, views.HintsConfirmModal},
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
			capture(runeKey(tc.key))
			if app.modal == nil {
				t.Errorf("key %q: expected modal to be shown", string(tc.key))
			}
			if app.hints != tc.wantHints {
				t.Errorf("key %q: hints got %q, want %q", string(tc.key), app.hints, tc.wantHints)
			}
		})
	}
}

// TestQueueEditor_EmptyQueue_NoModal builds its own minimal fixture rather
// than reusing newDetailTestApp (detail_test.go, not yet converted — see
// the standing note on cross-file dependencies during this conversion).
// Only a single account with one empty-queue session is needed here.
func TestQueueEditor_EmptyQueue_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  rune
	}{
		{"edit", views.KeyEdit},
		{"delete", views.KeyDelete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
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
			capture(runeKey(tc.key))
			if app.modal != nil {
				t.Errorf("key %q with empty queue should not open a modal", string(tc.key))
			}
		})
	}
}

// ── Reorder: commits immediately, no modal involved ───────────────────────────

func TestQueueEditor_Reorder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       rune
		startIdx  int
		wantOrder []string
		wantIdx   int
	}{
		{
			name:      "move up from first item is a no-op",
			key:       views.KeyMoveUp,
			startIdx:  0,
			wantOrder: []string{"first", "second", "third"},
			wantIdx:   0,
		},
		{
			name:      "move up swaps with previous",
			key:       views.KeyMoveUp,
			startIdx:  1,
			wantOrder: []string{"second", "first", "third"},
			wantIdx:   0,
		},
		{
			name:      "move down from last item is a no-op",
			key:       views.KeyMoveDown,
			startIdx:  2,
			wantOrder: []string{"first", "second", "third"},
			wantIdx:   2,
		},
		{
			name:      "move down swaps with next",
			key:       views.KeyMoveDown,
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
			capture(runeKey(tc.key))

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
	must(t, app.svc.EndSession(acc.UserEmail(), acc.ProviderName(), sess.ID()))

	list.SetCurrentItem(1)
	capture := list.GetInputCapture()
	capture(runeKey(views.KeyMoveUp))

	if app.status == "" {
		t.Error("expected a status message reporting the commit failure")
	}
}

// ── New / edit prompt: opens the form ──────────────────────────────────────────

// TestQueueEditor_NewAndEditPrompt_OpensForm replaces the old direct call
// to the unexported newPromptForm: it drives the same two code paths
// (KeyNew, KeyEdit) through the queue editor's own reachable key capture
// and checks the resulting modal is a centred form (*tview.Flex), not
// e.g. an error modal.
func TestQueueEditor_NewAndEditPrompt_OpensForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  rune
	}{
		{"new prompt", views.KeyNew},
		{"edit existing prompt", views.KeyEdit},
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
			list.GetInputCapture()(runeKey(tc.key))
			if _, ok := app.modal.(*tview.Flex); !ok {
				t.Errorf("key %q: expected centred form (*tview.Flex), got %T", string(tc.key), app.modal)
			}
		})
	}
}

// ── promptItemText ──────────────────────────────────────────────────────────────

// TestPromptItemText builds a real session per case and reads the rendered
// row back via the list's exported GetItemText, rather than calling the
// unexported promptItemText directly.
func TestPromptItemText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		text         string
		spawns       bool
		wantContains string
		wantEqual    string
	}{
		{"no spawn returns plain text", "continue", false, "", "continue"},
		{"spawns appends marker", "start fresh", true, "new session", ""},
		{"bracket in text is escaped", "do [x]", false, "", tview.Escape("do [x]")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
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
		if app.hints != views.HintsQueueEditor {
			t.Errorf("hints after reopen: got %q, want %q", app.hints, views.HintsQueueEditor)
		}
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

		confirmModal, ok := app.modal.(*tview.Modal)
		if !ok {
			t.Fatalf("expected *tview.Modal, got %T", app.modal)
		}
		confirmModal.GetInputCapture()(escKey())

		if _, ok := app.modal.(*tview.List); !ok {
			t.Fatalf("expected cancelling quit to reopen the queue editor (*tview.List), got %T", app.modal)
		}
		if app.hints != views.HintsQueueEditor {
			t.Errorf("hints after reopen: got %q, want %q", app.hints, views.HintsQueueEditor)
		}
		if app.quitted {
			t.Error("cancelling quit must not actually quit")
		}
	})
}
