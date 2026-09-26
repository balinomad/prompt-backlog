package views_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── construction ──────────────────────────────────────────────────────────────

func TestNewDetailPanel(t *testing.T) {
	t.Parallel()
	if p := views.NewDetailPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil DetailPanel")
	}
}

// TestDetailPanel_FocusFunc_SetsHints covers NewDetailPanel's
// SetFocusFunc registration. tview.Box.Focus invokes the registered
// handler synchronously (confirmed against rivo/tview v0.42.0's box.go),
// so this is reachable without a running tview.Application.
func TestDetailPanel_FocusFunc_SetsHints(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewDetailPanel(app)
	list, _ := detailPanelParts(t, p)

	list.Focus(func(tview.Primitive) {})

	if app.hints != views.HintsDetailPanel {
		t.Errorf("hints after focus: got %q, want %q", app.hints, views.HintsDetailPanel)
	}
}

// ── SetAccount / Refresh ──────────────────────────────────────────────────────

// newDetailTestApp returns a mock wired with a single account that has two
// sessions attached, plus one granted project. Defined locally rather than
// added to newMockApp in helpers_test.go: that shared fixture deliberately
// carries no sessions, and this file is the only current caller that needs
// them.
//
//	Account:  alice|claude, project "Alpha" granted
//	Sessions: #1 "Architecture review" / Alpha / queue=[continue] (pending)
//	          #2 untitled / no project / empty queue
func newDetailTestApp(t *testing.T) (*mockApp, *model.Account) {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
	_, err = reg.AddSession(acc, reg.Project("Alpha"), "Architecture review",
		[]model.Prompt{model.NewPrompt("continue", false)})
	must(t, err)
	_, err = reg.AddSession(acc, nil, "", nil)
	must(t, err)

	app := &mockApp{svc: newTestService(t, reg)}
	return app, acc
}

// detailPanelParts returns p's sessions list and info view, reached via
// the exported Flex.GetItem — DetailPanel puts the list at index 0, the
// info view at index 1 (see detail.go).
func detailPanelParts(t *testing.T, p *views.DetailPanel) (*tview.List, *tview.TextView) {
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

func TestDetailPanel_SetAccount_Nil(t *testing.T) {
	t.Parallel()
	p := views.NewDetailPanel(newMockApp(t))
	p.SetAccount(nil)
	_, info := detailPanelParts(t, p)
	if !strings.Contains(info.GetText(false), "no account selected") {
		t.Errorf("expected info panel to show \"no account selected\", got %q", info.GetText(false))
	}
}

func TestDetailPanel_SetAccount_ItemCount(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list, _ := detailPanelParts(t, p)
	if got := list.GetItemCount(); got != 2 {
		t.Errorf("expected 2 list items, got %d", got)
	}
}

func TestDetailPanel_Refresh_ClearsOnDeletedAccount(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	app := &mockApp{svc: newTestService(t, reg)}

	p := views.NewDetailPanel(app)
	p.SetAccount(acc)

	must(t, reg.DeleteAccount(acc))
	p.Refresh()

	list, info := detailPanelParts(t, p)
	if !strings.Contains(info.GetText(false), "no account selected") {
		t.Error("expected info panel to show \"no account selected\" after deletion from registry")
	}
	if list.GetItemCount() != 0 {
		t.Error("list should be empty after account deleted")
	}
}

// TestDetailPanel_Refresh_PreservesSelectionByID mirrors
// TestAccountsPanel_Refresh_PreservesSelection and
// TestProjectsPanel_Refresh_PreservesSelection: Refresh must restore the
// previously selected session by ID, not just re-select index 0.
func TestDetailPanel_Refresh_PreservesSelectionByID(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list, _ := detailPanelParts(t, p)
	list.SetCurrentItem(1) // the untitled session, per newDetailTestApp's doc comment

	want := p.Selected()
	if want == nil {
		t.Fatal("second item should not be nil")
	}
	wantID := want.ID()

	p.Refresh()

	got := p.Selected()
	if got == nil {
		t.Fatal("selection should survive Refresh")
	}
	if got.ID() != wantID {
		t.Errorf("selection changed: got session #%d, want #%d", got.ID(), wantID)
	}
}

// TestDetailPanel_RenderInfo_PausedAccount covers renderInfo's paused
// branch, unreached by every other DetailPanel test.
func TestDetailPanel_RenderInfo_PausedAccount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		pauseAt      time.Time
		wantContains string
	}{
		{"future pause shown red, still unavailable", time.Now().Add(time.Hour), "[red]"},
		{"past pause shown green, available again", time.Now().Add(-time.Hour), "[green]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
			must(t, err)
			must(t, reg.PauseAccount(acc, tc.pauseAt))
			acc = reg.Account(reg.User("alice@example.com"), reg.Provider("claude")) // re-read: PauseAccount replaces via Copy-on-Write

			app := &mockApp{svc: newTestService(t, reg)}
			p := views.NewDetailPanel(app)
			p.SetAccount(acc)
			_, info := detailPanelParts(t, p)

			if got := info.GetText(false); !strings.Contains(got, tc.wantContains) {
				t.Errorf("got %q, want it to contain %q", got, tc.wantContains)
			}
		})
	}
}

// TestDetailPanel_RenderInfo_NeverPaused covers renderInfo's remaining
// pause-status branch: an account with no PauseUntil at all shows the
// "Paused:" label with nothing after it, matching pauseStatusText's ""
// return for a zero value, the same way the accounts list omits a pause
// column entirely for such an account, now that both share one function.
func TestDetailPanel_RenderInfo_NeverPaused(t *testing.T) {
	t.Parallel()
	app := newMockApp(t) // alice|claude, never paused
	acc, err := app.svc.Account("alice@example.com", "claude")
	must(t, err)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	_, info := detailPanelParts(t, p)

	got := info.GetText(false)
	if strings.Contains(got, "[red]") || strings.Contains(got, "[green]") {
		t.Errorf("got %q, want no pause colour tag for a never-paused account", got)
	}
	if !strings.Contains(got, "Paused") {
		t.Errorf("got %q, want it to still contain the Paused label", got)
	}
}

// ── keyboard handlers ─────────────────────────────────────────────────────────

// TestDetailPanel_Keys_ShowModal checks not just that a modal opens, but
// which concrete tview type it is: NewSessionForm and NewHelpOverlay
// return a centred *tview.Flex, NewConfirmModal returns *tview.Modal,
// NewQueueEditor returns its *tview.List directly. A wrong-modal
// regression (e.g. help wired to the wrong handler) would still pass a
// bare non-nil check; it won't pass this.
func TestDetailPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       *tcell.EventKey
		wantHints string
		wantType  string // "flex" | "modal" | "list"
	}{
		{"help", runeKey(views.KeyHelp), views.HintsHelpOverlay, "flex"},
		{"quit", runeKey(views.KeyQuit), views.HintsConfirmModal, "modal"},
		{"new session", runeKey(views.KeyNew), views.HintsForm, "flex"},
		{"edit session", runeKey(views.KeySessionFields), views.HintsForm, "flex"},
		{"delete session", specialKey(views.KeyDelete), views.HintsConfirmModal, "modal"},
		{"queue editor", runeKey(views.KeyQueue), views.HintsQueueEditor, "list"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc := newDetailTestApp(t)
			p := views.NewDetailPanel(app)
			p.SetAccount(acc)
			list, _ := detailPanelParts(t, p)
			list.SetCurrentItem(0)
			capture := list.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.modal == nil {
				t.Fatalf("key %q: expected modal", tc.name)
			}
			switch tc.wantType {
			case "flex":
				if _, ok := app.modal.(*tview.Flex); !ok {
					t.Errorf("key %q: expected *tview.Flex, got %T", tc.name, app.modal)
				}
			case "modal":
				if _, ok := app.modal.(*tview.Modal); !ok {
					t.Errorf("key %q: expected *tview.Modal, got %T", tc.name, app.modal)
				}
			case "list":
				if _, ok := app.modal.(*tview.List); !ok {
					t.Errorf("key %q: expected *tview.List, got %T", tc.name, app.modal)
				}
			}
			if app.hints != tc.wantHints {
				t.Errorf("key %q: hints got %q, want %q", tc.name, app.hints, tc.wantHints)
			}
		})
	}
}

// TestDetailPanel_ShowModalError covers the ShowModal-error branch shared
// by handleNew, handleEdit, handleDelete, and handleQueue.
func TestDetailPanel_ShowModalError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  *tcell.EventKey
	}{
		{"new session", runeKey(views.KeyNew)},
		{"edit session", runeKey(views.KeySessionFields)},
		{"delete session", specialKey(views.KeyDelete)},
		{"queue editor", runeKey(views.KeyQueue)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc := newDetailTestApp(t)
			app.showModalErr = errors.New("boom")
			p := views.NewDetailPanel(app)
			p.SetAccount(acc)
			list, _ := detailPanelParts(t, p)
			list.SetCurrentItem(0)

			list.GetInputCapture()(tc.key)

			if app.status == "" {
				t.Errorf("key %q: expected a status message reporting the ShowModal failure", tc.name)
			}
			if app.modal != nil {
				t.Errorf("key %q: expected no modal recorded when ShowModal itself fails", tc.name)
			}
		})
	}
}

func TestDetailPanel_NoSessions_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  *tcell.EventKey
	}{
		{"edit", runeKey(views.KeySessionFields)},
		{"delete", specialKey(views.KeyDelete)},
		{"queue", runeKey(views.KeyQueue)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
			must(t, err)
			app := &mockApp{svc: newTestService(t, reg)}

			p := views.NewDetailPanel(app)
			p.SetAccount(acc)
			list, _ := detailPanelParts(t, p)
			capture := list.GetInputCapture()
			capture(tc.key)
			if app.modal != nil {
				t.Errorf("key %q with no sessions should not open a modal", tc.name)
			}
		})
	}
}

// TestDetailPanel_NewSession_NilAccount_NoModal covers handleNew's own
// guard: unlike handleEdit/handleDelete/handleQueue/handleToggle, it does
// not depend on a selected session, only on p.account being non-nil, so
// it needs its own case rather than folding into
// TestDetailPanel_NoSessions_NoModal.
func TestDetailPanel_NewSession_NilAccount_NoModal(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewDetailPanel(app) // account is nil; SetAccount is never called
	list, _ := detailPanelParts(t, p)
	list.GetInputCapture()(runeKey(views.KeyNew))
	if app.modal != nil {
		t.Error("KeyNew with no account selected should not open a modal")
	}
}

func TestDetailPanel_HandleKey_PassesThroughUnrecognizedKey(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list, _ := detailPanelParts(t, p)

	event := runeKey('z') // not bound to anything in DetailPanel
	if got := list.GetInputCapture()(event); got != event {
		t.Error("unrecognized key should pass through unchanged")
	}
}

// TestDetailPanel_EditSession_OpensPrepopulatedForm strengthens
// TestDetailPanel_Keys_ShowModal's "edit session" case, which only
// checks that *a* form opens: this checks it's a form for the *selected*
// session specifically, by reading its Topic field back.
func TestDetailPanel_EditSession_OpensPrepopulatedForm(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list, _ := detailPanelParts(t, p)
	list.SetCurrentItem(0) // "Architecture review", per newDetailTestApp's doc comment

	list.GetInputCapture()(runeKey(views.KeySessionFields))

	form := formFromCenteredModal(t, app.modal)
	topicField, ok := form.GetFormItemByLabel("Topic").(*tview.InputField)
	if !ok {
		t.Fatal("expected Topic *tview.InputField")
	}
	if got := topicField.GetText(); got != "Architecture review" {
		t.Errorf("topic: got %q, want %q (the selected session's topic)", got, "Architecture review")
	}
}

// TestDetailPanel_Queue_OpensForSelectedSession strengthens
// TestDetailPanel_Keys_ShowModal's "queue editor" case the same way:
// checks the opened editor holds the *selected* session's queue, not
// just that some *tview.List opened.
func TestDetailPanel_Queue_OpensForSelectedSession(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list, _ := detailPanelParts(t, p)
	list.SetCurrentItem(0) // "Architecture review", queue=[continue]

	list.GetInputCapture()(runeKey(views.KeyQueue))

	queueList, ok := app.modal.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List (queue editor), got %T", app.modal)
	}
	if got := queueList.GetItemCount(); got != 1 {
		t.Errorf("queue item count: got %d, want 1 (the selected session's queue)", got)
	}
}

// TestDetailPanel_DeleteSession_Cancel is as far as handleDelete's
// confirm modal can be driven black-box. The Confirm button's click
// path is not reachable from outside tview: it routes through
// (*tview.Modal).InputHandler, which is gated behind
// (*tview.Frame).HasFocus → (*tview.Form).HasFocus → a per-button
// hasFocus flag that only a running tview.Application's real focus
// delegation ever sets — nothing in this test suite spins one up, and
// the Form/Button holding that state is an unexported field on Modal,
// unreachable even if it weren't focus-gated. Esc is different: Modal's
// SetInputCapture intercepts it directly (see modal.go's comment on
// why), with no focus dependency, so onCancel is reachable the same way
// every other capture in this suite is. Confirming an end-session
// therefore stays unverified black-box; this covers the reachable half —
// cancelling leaves the session untouched and state restored.
func TestDetailPanel_DeleteSession_Cancel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		idx  int // 0 = has a topic, 1 = untitled (exercises handleDelete's label=="" fallback)
	}{
		{"session with topic", 0},
		{"untitled session", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc := newDetailTestApp(t)
			p := views.NewDetailPanel(app)
			p.SetAccount(acc)
			list, _ := detailPanelParts(t, p)
			list.SetCurrentItem(tc.idx)

			sessionsBefore, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
			must(t, err)
			wantCount := len(sessionsBefore)

			list.GetInputCapture()(specialKey(views.KeyDelete))
			confirmModal, ok := app.modal.(*tview.Modal)
			if !ok {
				t.Fatalf("expected *tview.Modal, got %T", app.modal)
			}
			capture := confirmModal.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(escKey())

			if app.modal != nil {
				t.Error("expected modal to be hidden after cancelling")
			}
			if app.hints != views.HintsDetailPanel {
				t.Errorf("hints after cancel: got %q, want %q", app.hints, views.HintsDetailPanel)
			}
			sessionsAfter, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
			must(t, err)
			if len(sessionsAfter) != wantCount {
				t.Errorf("session count changed after cancel: got %d, want %d (unchanged)", len(sessionsAfter), wantCount)
			}
		})
	}
}

// TestDetailPanel_Toggle_Continue also subsumes the old, unexported
// toggleContinuePrompt's direct-call table: the three cases below are the
// same three transformations (empty->append, pending->remove,
// non-continue-tail->append), verified through the committed session
// queue instead of the pure function's return value.
func TestDetailPanel_Toggle_Continue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		initQueue   []model.Prompt
		wantLen     int
		wantPending bool
	}{
		{"empty queue gets continue appended", nil, 1, true},
		{"pending continue gets removed", []model.Prompt{model.NewPrompt("continue", false)}, 0, false},
		{"non-continue tail is kept, continue appended", []model.Prompt{model.NewPrompt("do X", false)}, 2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
			must(t, err)
			sess, err := reg.AddSession(acc, nil, "topic", tc.initQueue)
			must(t, err)
			app := &mockApp{svc: newTestService(t, reg)}

			p := views.NewDetailPanel(app)
			p.SetAccount(acc)
			list, _ := detailPanelParts(t, p)
			list.SetCurrentItem(0)
			list.GetInputCapture()(runeKey(views.KeyToggle))

			updated := app.svc.Session(sess.ID())
			if updated == nil {
				t.Fatal("session not found after toggle")
			}
			q := updated.Queue()
			if len(q) != tc.wantLen {
				t.Fatalf("queue length: got %d, want %d", len(q), tc.wantLen)
			}
			gotPending := len(q) > 0 && q[len(q)-1] == model.NewPrompt("continue", false)
			if gotPending != tc.wantPending {
				t.Errorf("pending continue: got %v, want %v", gotPending, tc.wantPending)
			}
		})
	}
}

// TestDetailPanel_Toggle_NoSession_NoOp covers handleToggle's own guard.
// It is not folded into TestDetailPanel_NoSessions_NoModal because
// handleToggle never opens a modal on any path — the meaningful
// assertion here is that nothing panics (an unguarded handleToggle would
// dereference a nil *model.Session) and no status is set.
func TestDetailPanel_Toggle_NoSession_NoOp(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	app := &mockApp{svc: newTestService(t, reg)}

	p := views.NewDetailPanel(app)
	p.SetAccount(acc) // no sessions
	list, _ := detailPanelParts(t, p)
	list.GetInputCapture()(runeKey(views.KeyToggle))

	if app.status != "" {
		t.Errorf("expected no status message from a no-op toggle, got %q", app.status)
	}
}

// TestDetailPanel_Toggle_MutateError covers handleToggle's own
// SetStatus-on-error branch.
func TestDetailPanel_Toggle_MutateError(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	_, err = reg.AddSession(acc, nil, "topic", nil)
	must(t, err)
	app := &mockApp{svc: newTestService(t, reg), mutateErr: errors.New("boom")}

	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list, _ := detailPanelParts(t, p)
	list.SetCurrentItem(0)
	list.GetInputCapture()(runeKey(views.KeyToggle))

	if app.status == "" {
		t.Error("expected a status message reporting the mutate failure")
	}
}

// TestDetailPanel_NewSession_SaveReturnsToPanel closes the loop that
// TestDetailPanel_Keys_ShowModal leaves open: that test only checks the
// modal type and hints when the form opens, not that completing it hands
// control back to the panel correctly.
func TestDetailPanel_NewSession_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list, _ := detailPanelParts(t, p)

	list.GetInputCapture()(runeKey(views.KeyNew))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Topic", "Fresh session")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hints != views.HintsDetailPanel {
		t.Errorf("hints after save: got %q, want %q", app.hints, views.HintsDetailPanel)
	}
	sessions, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
	must(t, err)
	var found bool
	for _, s := range sessions {
		if s.Topic() == "Fresh session" {
			found = true
		}
	}
	if !found {
		t.Error("new session not created")
	}
}

// TestDetailPanel_EditSession_SaveReturnsToPanel is the same round trip
// as TestDetailPanel_NewSession_SaveReturnsToPanel for handleEdit's own
// onClose closure.
func TestDetailPanel_EditSession_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list, _ := detailPanelParts(t, p)
	list.SetCurrentItem(0) // "Architecture review"

	list.GetInputCapture()(runeKey(views.KeySessionFields))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Topic", "Renamed topic")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hints != views.HintsDetailPanel {
		t.Errorf("hints after save: got %q, want %q", app.hints, views.HintsDetailPanel)
	}
	sessions, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
	must(t, err)
	if sessions[0].Topic() != "Renamed topic" {
		t.Errorf("Topic() = %q, want %q", sessions[0].Topic(), "Renamed topic")
	}
}

// TestDetailPanel_Queue_CloseReturnsToPanel closes the loop for
// handleQueue's own onClose closure: TestDetailPanel_Queue_OpensForSelectedSession
// only checks the editor opens with the right queue, not that closing it
// (Esc, the reachable half per queue.go's own handleKey) hands control
// back to DetailPanel correctly.
func TestDetailPanel_Queue_CloseReturnsToPanel(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list, _ := detailPanelParts(t, p)
	list.SetCurrentItem(0)

	list.GetInputCapture()(runeKey(views.KeyQueue))

	queueList, ok := app.modal.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List (queue editor), got %T", app.modal)
	}
	queueList.GetInputCapture()(escKey())

	if app.modal != nil {
		t.Error("expected modal to be hidden after closing the queue editor")
	}
	if app.hints != views.HintsDetailPanel {
		t.Errorf("hints after close: got %q, want %q", app.hints, views.HintsDetailPanel)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// TestDetailPanel_SessionRowText replaces the old direct calls to the
// unexported sessionItemText and isContinuePending: it builds a real
// session per case through a registry, sets it on a DetailPanel, and
// reads the rendered row back via the sessions list's exported
// GetItemText. Folds in the old isContinuePending cases (marker
// presence) alongside the old sessionItemText cases (topic, project,
// queue count, escaping) — both concerns render into the same row, and
// building a session per case already covers what either would need
// separately. TestTruncate has no equivalent here: sessionItemText pads
// its result into a fixed-width column before this test ever sees it, so
// the exact rune-boundary case isn't recoverable from the rendered
// string — dropped rather than tested imprecisely.
func TestDetailPanel_SessionRowText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		topic        string
		projectName  string // empty = no project
		queue        []model.Prompt
		wantContains []string
		wantMarker   bool
	}{
		{
			name:         "untitled, no project, empty queue",
			wantContains: []string{"(untitled)", "queue: 0"},
		},
		{
			name:         "topic and project shown",
			topic:        "Architecture review",
			projectName:  "Alpha",
			wantContains: []string{"Architecture review", "Alpha", "queue: 0"},
		},
		{
			name:         "bracket in topic and project is escaped",
			topic:        "plan [x]",
			projectName:  "proj]y",
			wantContains: []string{tview.Escape("plan [x]"), tview.Escape("proj]y")},
		},
		{
			name:         "pending continue shows marker",
			topic:        "topic",
			queue:        []model.Prompt{model.NewPrompt("continue", false)},
			wantContains: []string{"queue: 1"},
			wantMarker:   true,
		},
		{
			name:         "continue not last: no marker",
			queue:        []model.Prompt{model.NewPrompt("continue", false), model.NewPrompt("do X", false)},
			wantContains: []string{"queue: 2"},
		},
		{
			name:         "continue with spawnsSession true: no marker",
			queue:        []model.Prompt{model.NewPrompt("continue", true)},
			wantContains: []string{"queue: 1"},
		},
		{
			name:         "non-continue tail: no marker",
			queue:        []model.Prompt{model.NewPrompt("do X", false)},
			wantContains: []string{"queue: 1"},
		},
		{
			name:         "long topic is truncated",
			topic:        strings.Repeat("A", 40),
			wantContains: []string{strings.Repeat("A", 25) + "…"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
			must(t, err)

			var proj *model.Project
			if tc.projectName != "" {
				must(t, reg.AddProject(model.NewProject(tc.projectName, "", "")))
				proj = reg.Project(tc.projectName)
				must(t, reg.AddProjectToAccount(acc, proj))
			}
			_, err = reg.AddSession(acc, proj, tc.topic, tc.queue)
			must(t, err)

			app := &mockApp{svc: newTestService(t, reg)}
			p := views.NewDetailPanel(app)
			p.SetAccount(acc)
			list, _ := detailPanelParts(t, p)
			got, _ := list.GetItemText(0)

			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("got %q, want it to contain %q", got, want)
				}
			}
			if hasMarker := strings.Contains(got, "↻"); hasMarker != tc.wantMarker {
				t.Errorf("marker present: got %v, want %v (row: %q)", hasMarker, tc.wantMarker, got)
			}
		})
	}
}
