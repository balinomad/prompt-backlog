package views_test

import (
	"errors"
	"strings"
	"testing"

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
	list := detailPanelList(t, p)

	list.Focus(func(tview.Primitive) {})

	assertHints(t, app, views.HintsDetailPanelSpecific, views.HintsDetailPanelCommon)
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

// detailPanelList returns p's embedded sessions list.
func detailPanelList(t *testing.T, p *views.DetailPanel) *tview.List {
	t.Helper()
	if p.List == nil {
		t.Fatal("expected non-nil sessions list")
	}
	return p.List
}

func TestDetailPanel_SetAccount_Nil(t *testing.T) {
	t.Parallel()
	p := views.NewDetailPanel(newMockApp(t))
	p.SetAccount(nil)
	if got := detailPanelList(t, p).GetItemCount(); got != 0 {
		t.Errorf("expected no list items, got %d", got)
	}
}

func TestDetailPanel_SetAccount_ItemCount(t *testing.T) {
	t.Parallel()
	app, acc := newDetailTestApp(t)
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	list := detailPanelList(t, p)
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

	list := detailPanelList(t, p)
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
	list := detailPanelList(t, p)
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
		name         string
		key          *tcell.EventKey
		wantSpecific string
		wantCommon   string
		wantType     string // "flex" | "modal" | "list"
	}{
		{"help", runeKey(views.KeyHelp), views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon, "flex"},
		{"quit", runeKey(views.KeyQuit), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon, "modal"},
		{"new session", specialKey(views.KeyInsert), views.HintsFormSpecific, views.HintsFormCommon, "flex"},
		{"edit session", specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsFormCommon, "flex"},
		{"delete session", specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon, "modal"},
		{"queue editor", runeKey(views.KeyQueue), views.HintsQueueEditorSpecific, views.HintsQueueEditorCommon, "list"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc := newDetailTestApp(t)
			p := views.NewDetailPanel(app)
			p.SetAccount(acc)
			list := detailPanelList(t, p)
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
			assertHints(t, app, tc.wantSpecific, tc.wantCommon)
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
		{"new session", specialKey(views.KeyInsert)},
		{"edit session", specialKey(views.KeyEdit)},
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
			list := detailPanelList(t, p)
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
		{"edit", specialKey(views.KeyEdit)},
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
			list := detailPanelList(t, p)
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
	list := detailPanelList(t, p)
	list.GetInputCapture()(specialKey(views.KeyInsert))
	if app.modal != nil {
		t.Error("KeyInsert with no account selected should not open a modal")
	}
}

func TestDetailPanel_HandleKey_PassesThroughUnrecognizedKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		event *tcell.EventKey
	}{
		{"unrecognized rune", runeKey('z')},
		{"removed edit alias", runeKey('e')},
		{"removed help alias", runeKey('h')},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc := newDetailTestApp(t)
			p := views.NewDetailPanel(app)
			p.SetAccount(acc)
			list := detailPanelList(t, p)

			if got := list.GetInputCapture()(tc.event); got != tc.event {
				t.Error("key should pass through unchanged")
			}
			if app.modal != nil {
				t.Error("key must not open a modal")
			}
		})
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
	list := detailPanelList(t, p)
	list.SetCurrentItem(0) // "Architecture review", per newDetailTestApp's doc comment

	list.GetInputCapture()(specialKey(views.KeyEdit))

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
	list := detailPanelList(t, p)
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
			list := detailPanelList(t, p)
			list.SetCurrentItem(tc.idx)

			sessionsBefore, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
			must(t, err)
			wantCount := len(sessionsBefore)

			list.GetInputCapture()(specialKey(views.KeyDelete))
			confirmModal := confirmModalFrom(t, app.modal)
			capture := confirmModal.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(escKey())

			if app.modal != nil {
				t.Error("expected modal to be hidden after cancelling")
			}
			assertHints(t, app, views.HintsDetailPanelSpecific, views.HintsDetailPanelCommon)
			sessionsAfter, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
			must(t, err)
			if len(sessionsAfter) != wantCount {
				t.Errorf("session count changed after cancel: got %d, want %d (unchanged)", len(sessionsAfter), wantCount)
			}
		})
	}
}

// TestDetailPanel_Toggle_Continue covers the three queue transformations
// through the committed session queue.
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
			list := detailPanelList(t, p)
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
	list := detailPanelList(t, p)
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
	list := detailPanelList(t, p)
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
	list := detailPanelList(t, p)

	list.GetInputCapture()(specialKey(views.KeyInsert))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Topic", "Fresh session")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsDetailPanelSpecific, views.HintsDetailPanelCommon)
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
	list := detailPanelList(t, p)
	list.SetCurrentItem(0) // "Architecture review"

	list.GetInputCapture()(specialKey(views.KeyEdit))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Topic", "Renamed topic")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsDetailPanelSpecific, views.HintsDetailPanelCommon)
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
	list := detailPanelList(t, p)
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
	assertHints(t, app, views.HintsDetailPanelSpecific, views.HintsDetailPanelCommon)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// TestDetailPanel_SessionRowText verifies the rendered session row: topic,
// project, queue count, escaping, and the pending marker.
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
			list := detailPanelList(t, p)
			got, _ := list.GetItemText(0)
			if !strings.HasPrefix(got, " ") {
				t.Errorf("got %q, want a leading one-cell inset", got)
			}

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

// ── AccountInfoPanel ──────────────────────────────────────────────────────────

func TestNewAccountInfoPanel(t *testing.T) {
	t.Parallel()
	p := views.NewAccountInfoPanel(newMockApp(t))
	if p == nil {
		t.Fatal("expected non-nil AccountInfoPanel")
	}
	if got := p.GetTitle(); got != " Account Details " {
		t.Errorf("GetTitle() = %q, want %q", got, " Account Details ")
	}
}

func TestAccountInfoPanel_SetAccount_Nil(t *testing.T) {
	t.Parallel()
	p := views.NewAccountInfoPanel(newMockApp(t))
	p.SetAccount(nil)

	if got := p.GetText(false); !strings.Contains(got, "no account selected") {
		t.Errorf("got %q, want it to contain %q", got, "no account selected")
	}
	if got := p.RequiredHeight(40); got != 3 {
		t.Errorf("RequiredHeight(40) = %d, want 3", got)
	}
}

func TestAccountInfoPanel_RendersDetailsWithoutPaused(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc, err := app.svc.Account("alice@example.com", "claude")
	must(t, err)

	p := views.NewAccountInfoPanel(app)
	p.SetAccount(acc)

	got := p.GetText(false)
	for _, want := range []string{"User", "Alice", "alice@example.com", "Provider", "claude", "Projects", "Alpha", "Beta"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "Paused") {
		t.Errorf("got %q, want no Paused line", got)
	}
	if gotHeight := p.RequiredHeight(80); gotHeight != 5 {
		t.Errorf("RequiredHeight(80) = %d, want 5 for three content lines", gotHeight)
	}
}

func TestAccountInfoPanel_AlignedDetailRows(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc, err := app.svc.Account("alice@example.com", "claude")
	must(t, err)

	p := views.NewAccountInfoPanel(app)
	p.SetAccount(acc)

	lines := strings.Split(p.GetText(true), "\n")
	wantValues := []string{"Alice", "claude", "Alpha"}
	if len(lines) != len(wantValues) {
		t.Fatalf("got %d detail lines, want %d: %q", len(lines), len(wantValues), lines)
	}
	for i, line := range lines {
		if got := strings.Index(line, wantValues[i]); got != 9 {
			t.Errorf("line %d value starts at %d, want 9: %q", i, got, line)
		}
	}
}

func TestAccountInfoPanel_WrappedLinesKeepInsets(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	project := model.NewProject(strings.Repeat("long-project-name-", 8), "", "")
	must(t, reg.AddProject(project))
	must(t, reg.AddProjectToAccount(acc, project))

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewAccountInfoPanel(app)
	p.SetAccount(reg.Account(reg.User("alice@example.com"), reg.Provider("claude")))

	screen := tcell.NewSimulationScreen("UTF-8")
	must(t, screen.Init())
	t.Cleanup(screen.Fini)
	screen.SetSize(40, 10)
	p.SetRect(0, 0, 40, 10)
	p.Draw(screen)

	x, y, width, height := p.GetInnerRect()
	nonBlank := 0
	for i := range height {
		mainc, _, _ := screen.Get(x, y+i)
		if strings.TrimSpace(mainc) == "" {
			continue
		}
		nonBlank++
		if pad, _, _ := screen.Get(x-1, y+i); pad != " " {
			t.Errorf("line %d starts with %q at column %d, want left padding", i, pad, x-1)
		}
		if pad, _, _ := screen.Get(x+width, y+i); pad != " " {
			t.Errorf("line %d reaches %q at column %d, want right padding", i, pad, x+width)
		}
	}
	if nonBlank < 4 {
		t.Fatalf("got %d non-blank lines, want at least 4 to exercise wrapping", nonBlank)
	}
}

func TestAccountInfoPanel_NoProjects(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc, err := app.svc.Account("bob@example.com", "chatgpt")
	must(t, err)

	p := views.NewAccountInfoPanel(app)
	p.SetAccount(acc)

	if got := p.GetText(false); !strings.Contains(got, "none") {
		t.Errorf("got %q, want it to contain %q", got, "none")
	}
}

func TestAccountInfoPanel_RequiredHeight_Wraps(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	project := model.NewProject(strings.Repeat("long-project-name-", 8), "", "")
	must(t, reg.AddProject(project))
	must(t, reg.AddProjectToAccount(acc, project))

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewAccountInfoPanel(app)
	p.SetAccount(reg.Account(reg.User("alice@example.com"), reg.Provider("claude")))

	narrow := p.RequiredHeight(20)
	wide := p.RequiredHeight(200)
	if narrow <= wide {
		t.Errorf("RequiredHeight(20) = %d, want more than RequiredHeight(200) = %d", narrow, wide)
	}
	if wide != 5 {
		t.Errorf("RequiredHeight(200) = %d, want 5", wide)
	}
}
