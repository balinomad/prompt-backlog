package views_test

import (
	"testing"
	"time"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/gdamore/tcell/v2"

	"github.com/rivo/tview"
)

// ── Shared assertion helper ────────────────────────────────────────────────────

// assertFormOrModal checks that result is a *tview.Modal when wantModal is
// true, or a *tview.Flex (a centred form) when false. Shared by every form
// constructor's precondition-shape test in this file.
func assertFormOrModal(t *testing.T, result tview.Primitive, wantModal bool) {
	t.Helper()
	_, isModal := result.(*tview.Modal)
	if wantModal {
		if !isModal {
			t.Errorf("expected error modal, got %T", result)
		}
		return
	}
	if _, ok := result.(*tview.Flex); !ok {
		t.Errorf("expected centred form, got %T", result)
	}
}

// formFromCenteredModal drills into the Flex/Box/content/Box/Flex
// structure centeredModal wraps content in, returning content itself.
func formFromCenteredModal(t *testing.T, p tview.Primitive) *tview.Form {
	t.Helper()
	outer, ok := p.(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex, got %T", p)
	}
	inner, ok := outer.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected inner *tview.Flex, got %T", outer.GetItem(1))
	}
	form, ok := inner.GetItem(2).(*tview.Form)
	if !ok {
		t.Fatalf("expected *tview.Form, got %T", inner.GetItem(2))
	}
	return form
}

// inputFieldAcceptsText reports whether typing a rune into field changes
// its text — see this section's doc comment for why this, not a disabled
// getter, is what's actually testable here.
func inputFieldAcceptsText(field *tview.InputField) bool {
	before := field.GetText()
	field.InputHandler()(runeKey('z'), func(tview.Primitive) {})
	return field.GetText() != before
}

// dropDownOptions returns every currently configured option's display
// text for d, walked via SetCurrentOption/GetCurrentOption since
// tview.DropDown exposes no direct "list all options" accessor. Restores
// the dropdown's original selection before returning.
func dropDownOptions(t *testing.T, d *tview.DropDown) []string {
	t.Helper()
	restore, _ := d.GetCurrentOption()
	defer d.SetCurrentOption(restore)

	n := d.GetOptionCount()
	opts := make([]string, n)
	for i := range n {
		d.SetCurrentOption(i)
		_, opts[i] = d.GetCurrentOption()
	}
	return opts
}

// ── Shared form-manipulation helpers ───────────────────────────────────────────
//
// save() tests need to set a value and read the result back through the
// service/registry, not just probe whether a field accepts input.

// setInputField sets the text of the *tview.InputField labelled label on
// form, failing the test if the label does not resolve to an InputField.
func setInputField(t *testing.T, form *tview.Form, label, text string) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	field, ok := item.(*tview.InputField)
	if !ok {
		t.Fatalf("expected %q to be *tview.InputField, got %T", label, item)
	}
	field.SetText(text)
}

// setCheckbox sets the checked state of the *tview.Checkbox labelled
// label on form, failing the test if the label does not resolve to a
// Checkbox.
func setCheckbox(t *testing.T, form *tview.Form, label string) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	cb, ok := item.(*tview.Checkbox)
	if !ok {
		t.Fatalf("expected %q to be *tview.Checkbox, got %T", label, item)
	}
	cb.SetChecked(true)
}

// setDropDown selects option index on the *tview.DropDown labelled label
// on form, failing the test if the label does not resolve to a DropDown.
func setDropDown(t *testing.T, form *tview.Form, label string, index int) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	dd, ok := item.(*tview.DropDown)
	if !ok {
		t.Fatalf("expected %q to be *tview.DropDown, got %T", label, item)
	}
	dd.SetCurrentOption(index)
}

// triggerSave drives form's Ctrl+S capture (see addCtrlS), failing the
// test if form has no input capture registered.
func triggerSave(t *testing.T, form *tview.Form) {
	t.Helper()
	capture := form.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil form input capture (addCtrlS)")
	}
	capture(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone))
}

// ── NewSessionForm ────────────────────────────────────────────────────────────

// newSessionFormTestAccount returns a mock wired with a single account
// holding one session, for NewSessionForm's edit-mode precondition.
// Defined locally rather than shared with detail_test.go's
// newDetailTestApp: that file is still package views until it converts —
// see the standing cross-file-fixture note in queueeditor_test.go.
func newSessionFormTestAccount(t *testing.T) (*mockApp, *model.Account, *model.Session) {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	sess, err := reg.AddSession(acc, nil, "", nil)
	must(t, err)
	app := &mockApp{svc: newTestService(t, reg)}
	return app, acc, sess
}

// sessionSaveFixture returns a mockApp with an account granted "Alpha",
// holding one session (topic "old topic", no project). Separate from
// newSessionFormTestAccount because the save() tests below need a
// granted project that form's callers don't; kept distinct rather than
// widening an already-passing fixture used elsewhere in this file.
func sessionSaveFixture(t *testing.T) (*mockApp, *model.Account, *model.Session) {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
	sess, err := reg.AddSession(acc, nil, "old topic", nil)
	must(t, err)
	app := &mockApp{svc: newTestService(t, reg)}
	return app, acc, sess
}

func TestNewSessionForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(t *testing.T) (app *mockApp, account *model.Account, session *model.Session)
		wantModal bool
	}{
		{
			name: "nil account",
			setup: func(t *testing.T) (*mockApp, *model.Account, *model.Session) {
				t.Helper()
				return newMockApp(t), nil, nil
			},
			wantModal: true,
		},
		{
			name: "create mode",
			setup: func(t *testing.T) (*mockApp, *model.Account, *model.Session) {
				t.Helper()
				app, acc, _ := newSessionFormTestAccount(t)
				return app, acc, nil
			},
			wantModal: false,
		},
		{
			name: "edit mode",
			setup: func(t *testing.T) (*mockApp, *model.Account, *model.Session) {
				t.Helper()
				return newSessionFormTestAccount(t)
			},
			wantModal: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc, sess := tc.setup(t)
			result := views.NewSessionForm(app, acc, sess, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

// TestSessionForm_ProjectNames replaces the old direct call to the
// unexported sessionFormProjectNames: it builds the real form and reads
// the Project dropdown's options back via dropDownOptions. Expanded from
// the original single hard-coded scenario into a table — the no-projects
// and normal-projects cases weren't covered before.
func TestSessionForm_ProjectNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, reg *model.Registry, acc *model.Account)
		want  []string
	}{
		{
			name:  "no granted projects",
			setup: func(t *testing.T, _ *model.Registry, _ *model.Account) { t.Helper() },
			want:  []string{"none"},
		},
		{
			name: "normal projects",
			setup: func(t *testing.T, reg *model.Registry, acc *model.Account) {
				t.Helper()
				must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
				must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
				must(t, reg.AddProject(model.NewProject("Beta", "", "")))
				must(t, reg.AddProjectToAccount(acc, reg.Project("Beta")))
			},
			want: []string{"none", "Alpha", "Beta"},
		},
		{
			name: "project literally named none collides with the sentinel",
			setup: func(t *testing.T, reg *model.Registry, acc *model.Account) {
				t.Helper()
				must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
				must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
				must(t, reg.AddProject(model.NewProject("none", "", "")))
				must(t, reg.AddProjectToAccount(acc, reg.Project("none")))
			},
			// AccountProjects sorts by name: "Alpha" before "none". Index 0
			// is always the sentinel, so the real "none" project must land
			// at index 2, distinct from the sentinel at index 0.
			want: []string{"none", "Alpha", "none"},
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
			tc.setup(t, reg, acc)

			app := &mockApp{svc: newTestService(t, reg)}
			form := formFromCenteredModal(t, views.NewSessionForm(app, acc, nil, func() {}))
			dropDown, ok := form.GetFormItemByLabel("Project").(*tview.DropDown)
			if !ok {
				t.Fatal("expected Project *tview.DropDown")
			}
			got := dropDownOptions(t, dropDown)

			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("got[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

func TestNewSessionForm_ProjectDropdown_BracketNameSurvivesSave(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Weird[Name]", "", "")))
	must(t, reg.AddProjectToAccount(acc, reg.Project("Weird[Name]")))

	app := &mockApp{svc: newTestService(t, reg)}
	form := formFromCenteredModal(t, views.NewSessionForm(app, acc, nil, func() {}))

	projectDropDown, ok := form.GetFormItemByLabel("Project").(*tview.DropDown)
	if !ok {
		t.Fatal("expected Project *tview.DropDown")
	}
	// Index 0 is always the "none" sentinel; the sole granted project is
	// index 1 (see TestSessionForm_ProjectNames).
	projectDropDown.SetCurrentOption(1)

	capture := form.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil form input capture (addCtrlS)")
	}
	capture(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone))

	sessions := app.svc.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions after save, want 1", len(sessions))
	}
	if got := sessions[0].ProjectName(); got != "Weird[Name]" {
		t.Errorf("ProjectName() = %q, want %q (unescaped)", got, "Weird[Name]")
	}
}

// TestSessionFormInitial replaces the old direct call to the unexported
// sessionFormInitial: it builds the real form per case and reads the
// Topic field and Project dropdown's initial state back.
func TestSessionFormInitial(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		nilSession  bool
		project     string // project granted+assigned to the session; ignored if nilSession
		wantTopic   string
		wantProjIdx int
	}{
		{
			name:        "nil session",
			nilSession:  true,
			wantTopic:   "",
			wantProjIdx: 0,
		},
		{
			name:        "no project assigned",
			wantTopic:   "Topic",
			wantProjIdx: 0,
		},
		{
			name:        "normal project matches by name",
			project:     "Alpha",
			wantTopic:   "Topic",
			wantProjIdx: 1,
		},
		{
			name: "project literally named none resolves to its own index, not the sentinel",
			// AccountProjects sorts by name: "Alpha" before "none", so the
			// sentinel at index 0 and the real "none" project at index 2
			// stay distinct.
			project:     "none",
			wantTopic:   "Topic",
			wantProjIdx: 2,
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
			must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
			must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
			must(t, reg.AddProject(model.NewProject("none", "", "")))
			must(t, reg.AddProjectToAccount(acc, reg.Project("none")))

			var sess *model.Session
			if !tc.nilSession {
				var proj *model.Project
				if tc.project != "" {
					proj = reg.Project(tc.project)
				}
				sess, err = reg.AddSession(acc, proj, "Topic", nil)
				must(t, err)
			}

			app := &mockApp{svc: newTestService(t, reg)}
			form := formFromCenteredModal(t, views.NewSessionForm(app, acc, sess, func() {}))

			topicField, ok := form.GetFormItemByLabel("Topic").(*tview.InputField)
			if !ok {
				t.Fatal("expected Topic *tview.InputField")
			}
			if got := topicField.GetText(); got != tc.wantTopic {
				t.Errorf("topic: got %q, want %q", got, tc.wantTopic)
			}

			dropDown, ok := form.GetFormItemByLabel("Project").(*tview.DropDown)
			if !ok {
				t.Fatal("expected Project *tview.DropDown")
			}
			if got, _ := dropDown.GetCurrentOption(); got != tc.wantProjIdx {
				t.Errorf("project index: got %d, want %d", got, tc.wantProjIdx)
			}
		})
	}
}

// TestNewSessionForm_Save_Edit covers save()'s isEdit branch: topic and
// project both update through UpdateSessionTopic/UpdateSessionProject,
// batched as one Mutate call per the doc comment on that call site in
// forms.go.
func TestNewSessionForm_Save_Edit(t *testing.T) {
	t.Parallel()

	t.Run("updates topic and assigns project", func(t *testing.T) {
		t.Parallel()
		app, acc, sess := sessionSaveFixture(t)
		form := formFromCenteredModal(t, views.NewSessionForm(app, acc, sess, func() {}))

		setInputField(t, form, "Topic", "New Topic")
		setDropDown(t, form, "Project", 1) // 0 = none, 1 = Alpha
		triggerSave(t, form)

		got := app.svc.Session(sess.ID())
		if got == nil {
			t.Fatal("session no longer exists")
		}
		if got.Topic() != "New Topic" {
			t.Errorf("Topic() = %q, want %q", got.Topic(), "New Topic")
		}
		if got.ProjectName() != "Alpha" {
			t.Errorf("ProjectName() = %q, want %q", got.ProjectName(), "Alpha")
		}
	})

	t.Run("selecting none clears an assigned project", func(t *testing.T) {
		t.Parallel()
		app, acc, sess := sessionSaveFixture(t)
		must(t, app.svc.UpdateSessionProject(acc.UserEmail(), acc.ProviderName(), sess.ID(), "Alpha"))
		sess = app.svc.Session(sess.ID()) // re-read: UpdateSessionProject replaced it via Copy-on-Write
		form := formFromCenteredModal(t, views.NewSessionForm(app, acc, sess, func() {}))

		setDropDown(t, form, "Project", 0) // none
		triggerSave(t, form)

		got := app.svc.Session(sess.ID())
		if got == nil {
			t.Fatal("session no longer exists")
		}
		if got.ProjectName() != "" {
			t.Errorf("ProjectName() = %q, want empty", got.ProjectName())
		}
	})

	t.Run("failure: session ended before save", func(t *testing.T) {
		t.Parallel()
		app, acc, sess := sessionSaveFixture(t)
		form := formFromCenteredModal(t, views.NewSessionForm(app, acc, sess, func() {}))
		must(t, app.svc.EndSession(acc.UserEmail(), acc.ProviderName(), sess.ID()))

		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// TestNewSessionForm_Save_Create covers save()'s !isEdit branch via
// StartSession. The project-selected success path is already covered by
// TestNewSessionForm_ProjectDropdown_BracketNameSurvivesSave; this adds
// the topic-only path and the failure path.
func TestNewSessionForm_Save_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app, acc, _ := sessionSaveFixture(t)
		form := formFromCenteredModal(t, views.NewSessionForm(app, acc, nil, func() {}))

		setInputField(t, form, "Topic", "Fresh topic")
		triggerSave(t, form)

		sessions, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
		must(t, err)
		var found bool
		for _, s := range sessions {
			if s.Topic() == "Fresh topic" {
				found = true
			}
		}
		if !found {
			t.Errorf("no session with topic %q found among %d sessions", "Fresh topic", len(sessions))
		}
	})

	t.Run("failure: project revoked before save", func(t *testing.T) {
		t.Parallel()
		app, acc, _ := sessionSaveFixture(t)
		form := formFromCenteredModal(t, views.NewSessionForm(app, acc, nil, func() {}))
		setDropDown(t, form, "Project", 1) // Alpha

		must(t, app.svc.RevokeProject(acc.UserEmail(), acc.ProviderName(), "Alpha"))

		before, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
		must(t, err)

		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
		after, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
		must(t, err)
		if len(after) != len(before) {
			t.Errorf("session count changed: got %d, want %d (unchanged)", len(after), len(before))
		}
	})
}

// ── NewProjectForm ────────────────────────────────────────────────────────────

// NewProjectForm no longer takes a session parameter (Project has no
// structural prerequisites), so the old provider-incompatibility pre-flight
// rejection — tied to attach-on-create, which no longer exists — has no
// analog here. project=nil is a valid input meaning "create mode", not a
// missing precondition, so there is no error-modal path to test.
func TestNewProjectForm_ReturnsForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		projectName string // empty = nil project = create mode
	}{
		{"create mode", ""},
		{"edit mode", "Alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			var proj *model.Project
			if tc.projectName != "" {
				proj = app.svc.Project(tc.projectName)
			}
			result := views.NewProjectForm(app, proj, func() {})
			assertFormOrModal(t, result, false)
		})
	}
}

func TestNewProjectForm_NameFieldDisabledState(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)

	cases := []struct {
		name        string
		projectName string // empty = nil project = create mode
		wantAccepts bool
	}{
		{"create mode: enabled", "", true},
		{"edit mode: disabled", "Alpha", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var proj *model.Project
			if tc.projectName != "" {
				proj = app.svc.Project(tc.projectName)
			}
			form := formFromCenteredModal(t, views.NewProjectForm(app, proj, func() {}))
			nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
			if !ok {
				t.Fatal("expected Name *tview.InputField")
			}
			if got := inputFieldAcceptsText(nameField); got != tc.wantAccepts {
				t.Errorf("accepts text: got %v, want %v", got, tc.wantAccepts)
			}
		})
	}
}

// TestNewProjectForm_Save_Edit covers save()'s isEdit branch:
// UpdateProject's success and not-found paths.
func TestNewProjectForm_Save_Edit(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "old desc", "old inst")))
		app := &mockApp{svc: newTestService(t, reg)}
		form := formFromCenteredModal(t, views.NewProjectForm(app, app.svc.Project("Alpha"), func() {}))

		setInputField(t, form, "Description", "new desc")
		setInputField(t, form, "Instructions", "new inst")
		triggerSave(t, form)

		got := app.svc.Project("Alpha")
		if got.Description() != "new desc" || got.Instructions() != "new inst" {
			t.Errorf("got desc %q inst %q, want both updated", got.Description(), got.Instructions())
		}
	})

	t.Run("failure: project deleted before save", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		app := &mockApp{svc: newTestService(t, reg)}
		proj := app.svc.Project("Alpha")
		form := formFromCenteredModal(t, views.NewProjectForm(app, proj, func() {}))

		must(t, app.svc.DeleteProject("Alpha"))

		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// TestNewProjectForm_Save_Create covers save()'s !isEdit branch: the
// pre-Mutate ValidateProject check, AddProject's success path, and
// AddProject's own failure path (duplicate name), which is a distinct
// code path from the pre-flight validation failure.
func TestNewProjectForm_Save_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		form := formFromCenteredModal(t, views.NewProjectForm(app, nil, func() {}))

		setInputField(t, form, "Name", "Gamma")
		setInputField(t, form, "Description", "desc")
		setInputField(t, form, "Instructions", "inst")
		triggerSave(t, form)

		got := app.svc.Project("Gamma")
		if got == nil {
			t.Fatal("project not created")
		}
		if got.Description() != "desc" || got.Instructions() != "inst" {
			t.Errorf("got desc %q inst %q", got.Description(), got.Instructions())
		}
	})

	t.Run("validation failure: empty name never mutates", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		before := len(app.svc.Projects())
		form := formFromCenteredModal(t, views.NewProjectForm(app, nil, func() {}))

		setInputField(t, form, "Name", "")
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the validation failure")
		}
		if got := len(app.svc.Projects()); got != before {
			t.Errorf("project count changed: got %d, want %d (unchanged)", got, before)
		}
	})

	t.Run("failure: duplicate name", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t) // already has "Alpha"
		form := formFromCenteredModal(t, views.NewProjectForm(app, nil, func() {}))

		setInputField(t, form, "Name", "Alpha")
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── NewProviderForm ───────────────────────────────────────────────────────────

// NewProviderForm's body is unchanged from the prior session; it had no
// dedicated test coverage before this rewrite.
func TestNewProviderForm_ReturnsForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		providerName string // empty = nil provider = create mode
	}{
		{"create mode", ""},
		{"edit mode", "claude"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			var prov *model.Provider
			if tc.providerName != "" {
				prov = app.svc.Provider(tc.providerName)
			}
			result := views.NewProviderForm(app, prov, func() {})
			assertFormOrModal(t, result, false)
		})
	}
}

func TestNewProviderForm_NameFieldDisabledState(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)

	cases := []struct {
		name         string
		providerName string // empty = nil provider = create mode
		wantAccepts  bool
	}{
		{"create mode: enabled", "", true},
		{"edit mode: disabled", "claude", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var prov *model.Provider
			if tc.providerName != "" {
				prov = app.svc.Provider(tc.providerName)
			}
			form := formFromCenteredModal(t, views.NewProviderForm(app, prov, func() {}))
			nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
			if !ok {
				t.Fatal("expected Name *tview.InputField")
			}
			if got := inputFieldAcceptsText(nameField); got != tc.wantAccepts {
				t.Errorf("accepts text: got %v, want %v", got, tc.wantAccepts)
			}
		})
	}
}

// TestNewProviderForm_Save_Edit covers save()'s isEdit branch:
// UpdateProvider's success and not-found paths.
func TestNewProviderForm_Save_Edit(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProvider(model.NewProvider("claude", false, false)))
		app := &mockApp{svc: newTestService(t, reg)}
		form := formFromCenteredModal(t, views.NewProviderForm(app, app.svc.Provider("claude"), func() {}))

		setCheckbox(t, form, "Supports projects")
		setCheckbox(t, form, "Supports deep research")
		triggerSave(t, form)

		got := app.svc.Provider("claude")
		if !got.HasProjects() || !got.HasDeepResearch() {
			t.Errorf("HasProjects=%v HasDeepResearch=%v, want both true", got.HasProjects(), got.HasDeepResearch())
		}
	})

	t.Run("failure: provider deleted before save", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProvider(model.NewProvider("claude", false, false)))
		app := &mockApp{svc: newTestService(t, reg)}
		prov := app.svc.Provider("claude")
		form := formFromCenteredModal(t, views.NewProviderForm(app, prov, func() {}))

		must(t, app.svc.DeleteProvider("claude"))

		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// TestNewProviderForm_Save_Create covers save()'s !isEdit branch: the
// pre-Mutate ValidateProvider check, AddProvider's success path, and
// AddProvider's own failure path (duplicate name).
func TestNewProviderForm_Save_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		form := formFromCenteredModal(t, views.NewProviderForm(app, nil, func() {}))

		setInputField(t, form, "Name", "gemini")
		setCheckbox(t, form, "Supports projects")
		triggerSave(t, form)

		got := app.svc.Provider("gemini")
		if got == nil {
			t.Fatal("provider not created")
		}
		if !got.HasProjects() {
			t.Error("HasProjects() = false, want true")
		}
	})

	t.Run("validation failure: empty name never mutates", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		before := len(app.svc.Providers())
		form := formFromCenteredModal(t, views.NewProviderForm(app, nil, func() {}))

		setInputField(t, form, "Name", "")
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the validation failure")
		}
		if got := len(app.svc.Providers()); got != before {
			t.Errorf("provider count changed: got %d, want %d (unchanged)", got, before)
		}
	})

	t.Run("failure: duplicate name", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t) // already has "claude"
		form := formFromCenteredModal(t, views.NewProviderForm(app, nil, func() {}))

		setInputField(t, form, "Name", "claude")
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── NewAccountForm ────────────────────────────────────────────────────────────

func TestNewAccountForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(t *testing.T) (*mockApp, *model.Account)
		wantModal bool
	}{
		{
			name: "no providers",
			setup: func(t *testing.T) (*mockApp, *model.Account) {
				t.Helper()
				app := newMockApp(t)
				app.svc = newTestService(t, model.NewRegistry())
				return app, nil
			},
			wantModal: true,
		},
		{
			name: "create mode",
			setup: func(t *testing.T) (*mockApp, *model.Account) {
				t.Helper()
				return newMockApp(t), nil
			},
			wantModal: false,
		},
		{
			name: "edit mode",
			setup: func(t *testing.T) (*mockApp, *model.Account) {
				t.Helper()
				app := newMockApp(t)
				acc, err := app.svc.Account("alice@example.com", "claude")
				must(t, err)
				return app, acc
			},
			wantModal: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc := tc.setup(t)
			result := views.NewAccountForm(app, acc, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

func TestAccountEditForm_EmailProviderDisabled(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc, err := app.svc.Account("alice@example.com", "claude")
	must(t, err)

	form := formFromCenteredModal(t, views.NewAccountForm(app, acc, func() {}))

	emailField, ok := form.GetFormItemByLabel("Email").(*tview.InputField)
	if !ok {
		t.Fatal("expected Email *tview.InputField")
	}
	if inputFieldAcceptsText(emailField) {
		t.Error("Email field should not accept text in edit mode")
	}

	providerField, ok := form.GetFormItemByLabel("Provider").(*tview.InputField)
	if !ok {
		t.Fatal("expected Provider *tview.InputField")
	}
	if inputFieldAcceptsText(providerField) {
		t.Error("Provider field should not accept text in edit mode")
	}
}

// TestAccountEditForm_Save covers accountEditForm's save(): the parsed
// pause-time success path, the empty-clears-pause path, the invalid
// format path (fails before any Mutate), and PauseAccount's own failure
// path (account removed before save).
func TestAccountEditForm_Save(t *testing.T) {
	t.Parallel()

	t.Run("sets pause time", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		acc, err := app.svc.Account("alice@example.com", "claude")
		must(t, err)
		form := formFromCenteredModal(t, views.NewAccountForm(app, acc, func() {}))

		setInputField(t, form, "Pause until", "2026-12-31 23:59")
		triggerSave(t, form)

		got, err := app.svc.Account("alice@example.com", "claude")
		must(t, err)
		want, err := model.ParseTime("2026-12-31 23:59")
		must(t, err)
		if !got.PauseUntil().Equal(want) {
			t.Errorf("PauseUntil() = %v, want %v", got.PauseUntil(), want)
		}
	})

	t.Run("empty clears an existing pause", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		must(t, app.svc.PauseAccount("alice@example.com", "claude", time.Now().Add(time.Hour)))
		acc, err := app.svc.Account("alice@example.com", "claude") // read after pausing: PauseAccount replaces the account via Copy-on-Write
		must(t, err)
		form := formFromCenteredModal(t, views.NewAccountForm(app, acc, func() {}))

		setInputField(t, form, "Pause until", "")
		triggerSave(t, form)

		got, err := app.svc.Account("alice@example.com", "claude")
		must(t, err)
		if !got.PauseUntil().IsZero() {
			t.Errorf("PauseUntil() = %v, want zero", got.PauseUntil())
		}
	})

	t.Run("invalid format never mutates", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		acc, err := app.svc.Account("alice@example.com", "claude")
		must(t, err)
		before := acc.PauseUntil()
		form := formFromCenteredModal(t, views.NewAccountForm(app, acc, func() {}))

		setInputField(t, form, "Pause until", "not-a-time")
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the format error")
		}
		got, err := app.svc.Account("alice@example.com", "claude")
		must(t, err)
		if !got.PauseUntil().Equal(before) {
			t.Errorf("PauseUntil() changed: got %v, want unchanged %v", got.PauseUntil(), before)
		}
	})

	t.Run("failure: account removed before save", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
		must(t, err)
		app := &mockApp{svc: newTestService(t, reg)}
		form := formFromCenteredModal(t, views.NewAccountForm(app, acc, func() {}))

		must(t, app.svc.DeleteAccount("alice@example.com", "claude"))

		setInputField(t, form, "Pause until", "2026-12-31 23:59")
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// TestAccountCreateForm_Save covers accountCreateForm's save(): a
// brand-new user, an existing user selected from the dropdown, the
// pre-Mutate ValidateAccount failure (invalid email), and CreateAccount's
// own failure path (the account already exists).
func TestAccountCreateForm_Save(t *testing.T) {
	t.Parallel()

	t.Run("success: new user", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		form := formFromCenteredModal(t, views.NewAccountForm(app, nil, func() {}))

		setInputField(t, form, "Email", "carol@example.com")
		setInputField(t, form, "Name", "Carol")
		setDropDown(t, form, "Provider", 1) // 0 = chatgpt, 1 = claude (sorted by name)
		triggerSave(t, form)

		gotUser := app.svc.User("carol@example.com")
		if gotUser == nil {
			t.Fatal("user not created")
		}
		if gotUser.Name() != "Carol" {
			t.Errorf("Name() = %q, want %q", gotUser.Name(), "Carol")
		}
		if _, err := app.svc.Account("carol@example.com", "claude"); err != nil {
			t.Errorf("account not created: %v", err)
		}
	})

	t.Run("success: existing user selected from dropdown", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t) // bob@example.com exists with a chatgpt account, no claude one
		form := formFromCenteredModal(t, views.NewAccountForm(app, nil, func() {}))

		setDropDown(t, form, "User", 2)     // 0 = "+ New user", 1 = alice, 2 = bob (sorted by email)
		setDropDown(t, form, "Provider", 1) // 0 = chatgpt, 1 = claude (sorted by name)
		triggerSave(t, form)

		if _, err := app.svc.Account("bob@example.com", "claude"); err != nil {
			t.Errorf("account not created: %v", err)
		}
	})

	t.Run("validation failure: invalid email never mutates", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		form := formFromCenteredModal(t, views.NewAccountForm(app, nil, func() {}))

		setInputField(t, form, "Email", "not-an-email")
		setInputField(t, form, "Name", "Nobody")
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the validation failure")
		}
		if app.svc.User("not-an-email") != nil {
			t.Error("user should not have been created")
		}
	})

	t.Run("failure: account already exists", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t) // alice|claude already exists
		form := formFromCenteredModal(t, views.NewAccountForm(app, nil, func() {}))

		setDropDown(t, form, "User", 1)     // alice
		setDropDown(t, form, "Provider", 1) // claude (0 = chatgpt, 1 = claude)
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── NewUserForm ───────────────────────────────────────────────────────────────

func TestNewUserForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(t *testing.T) (*mockApp, *model.Account)
		wantModal bool
	}{
		{
			name: "nil account",
			setup: func(t *testing.T) (*mockApp, *model.Account) {
				t.Helper()
				return newMockApp(t), nil
			},
			wantModal: true,
		},
		{
			name: "user no longer exists",
			setup: func(t *testing.T) (*mockApp, *model.Account) {
				t.Helper()
				app := newMockApp(t)
				// Constructed directly rather than resolved from the
				// registry: a real Account can't outlive its User through
				// normal service calls (DeleteUser refuses while any
				// account still references it), so this is the only way
				// to reach NewUserForm's defensive "user no longer exists"
				// check.
				ghost := model.NewAccount("ghost@example.com", "claude")
				return app, ghost
			},
			wantModal: true,
		},
		{
			name: "success",
			setup: func(t *testing.T) (*mockApp, *model.Account) {
				t.Helper()
				app := newMockApp(t)
				acc, err := app.svc.Account("alice@example.com", "claude")
				must(t, err)
				return app, acc
			},
			wantModal: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc := tc.setup(t)
			result := views.NewUserForm(app, acc, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

// TestNewUserForm_Save covers save()'s UpdateUser call: the success path,
// and the failure path where the underlying user is gone by the time
// save runs.
func TestNewUserForm_Save(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		acc, err := app.svc.Account("alice@example.com", "claude")
		must(t, err)
		form := formFromCenteredModal(t, views.NewUserForm(app, acc, func() {}))

		setInputField(t, form, "Name", "Alice Smith")
		triggerSave(t, form)

		got := app.svc.User("alice@example.com")
		if got.Name() != "Alice Smith" {
			t.Errorf("Name() = %q, want %q", got.Name(), "Alice Smith")
		}
	})

	t.Run("failure: user removed before save", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
		must(t, err)
		app := &mockApp{svc: newTestService(t, reg)}
		form := formFromCenteredModal(t, views.NewUserForm(app, acc, func() {}))

		// Both the account and the user must go: DeleteUser refuses while
		// any account still references it, same constraint noted in the
		// "user no longer exists" precondition test above.
		must(t, app.svc.DeleteAccount("alice@example.com", "claude"))
		must(t, app.svc.DeleteUser("alice@example.com"))

		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── NewGrantForm / NewRevokeForm ──────────────────────────────────────────────

func TestNewGrantForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(t *testing.T) (*mockApp, *model.Project)
		wantModal bool
	}{
		{
			name: "nil project",
			setup: func(t *testing.T) (*mockApp, *model.Project) {
				t.Helper()
				return newMockApp(t), nil
			},
			wantModal: true,
		},
		{
			name: "no accounts exist",
			setup: func(t *testing.T) (*mockApp, *model.Project) {
				t.Helper()
				reg := model.NewRegistry()
				must(t, reg.AddProject(model.NewProject("Solo", "", "")))
				app := newMockApp(t)
				app.svc = newTestService(t, reg)
				return app, app.svc.Project("Solo")
			},
			wantModal: true,
		},
		{
			name: "accounts available",
			setup: func(t *testing.T) (*mockApp, *model.Project) {
				t.Helper()
				app := newMockApp(t)
				return app, app.svc.Project("Orphan")
			},
			wantModal: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, proj := tc.setup(t)
			result := views.NewGrantForm(app, proj, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

// TestNewGrantForm_Save covers save()'s GrantProject call: the success
// path, and AddProjectToAccount's own failure path (already assigned).
func TestNewGrantForm_Save(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t) // Orphan is granted to no one
		proj := app.svc.Project("Orphan")
		form := formFromCenteredModal(t, views.NewGrantForm(app, proj, func() {}))

		setDropDown(t, form, "Account", 0) // accounts sorted by key: alice@example.com|claude first
		triggerSave(t, form)

		projects, err := app.svc.AccountProjects("alice@example.com", "claude")
		must(t, err)
		var found bool
		for _, p := range projects {
			if p.Name() == "Orphan" {
				found = true
			}
		}
		if !found {
			t.Error("Orphan not granted to alice|claude")
		}
	})

	t.Run("failure: already granted", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t) // Alpha is already granted to alice|claude
		proj := app.svc.Project("Alpha")
		form := formFromCenteredModal(t, views.NewGrantForm(app, proj, func() {}))

		setDropDown(t, form, "Account", 0) // alice|claude
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

func TestNewRevokeForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(t *testing.T) (*mockApp, *model.Project)
		wantModal bool
	}{
		{
			name: "nil project",
			setup: func(t *testing.T) (*mockApp, *model.Project) {
				t.Helper()
				return newMockApp(t), nil
			},
			wantModal: true,
		},
		{
			name: "project not granted to any account",
			setup: func(t *testing.T) (*mockApp, *model.Project) {
				t.Helper()
				app := newMockApp(t)
				return app, app.svc.Project("Orphan")
			},
			wantModal: true,
		},
		{
			name: "project granted",
			setup: func(t *testing.T) (*mockApp, *model.Project) {
				t.Helper()
				app := newMockApp(t)
				return app, app.svc.Project("Alpha")
			},
			wantModal: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, proj := tc.setup(t)
			result := views.NewRevokeForm(app, proj, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

// TestNewRevokeForm_Save covers save()'s RevokeProject call: the success
// path, and RemoveProjectFromAccount's own failure path (the project is
// in active use by a session).
func TestNewRevokeForm_Save(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t) // Beta is granted to alice|claude with no sessions using it
		proj := app.svc.Project("Beta")
		form := formFromCenteredModal(t, views.NewRevokeForm(app, proj, func() {}))

		setDropDown(t, form, "Account", 0) // only alice|claude has Beta
		triggerSave(t, form)

		projects, err := app.svc.AccountProjects("alice@example.com", "claude")
		must(t, err)
		for _, p := range projects {
			if p.Name() == "Beta" {
				t.Error("Beta should have been revoked")
			}
		}
	})

	t.Run("failure: project in active use", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
		must(t, err)
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
		_, err = reg.AddSession(acc, reg.Project("Alpha"), "", nil)
		must(t, err)
		app := &mockApp{svc: newTestService(t, reg)}
		proj := app.svc.Project("Alpha")
		form := formFromCenteredModal(t, views.NewRevokeForm(app, proj, func() {}))

		setDropDown(t, form, "Account", 0)
		triggerSave(t, form)

		if app.status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── addCtrlS ──────────────────────────────────────────────────────────────────

// TestAddCtrlS_NonSaveKeyPassesThrough covers the return event branch:
// addCtrlS is shared across all eight forms, so exercising it through any
// one of them (NewProjectForm, arbitrarily) covers every caller.
func TestAddCtrlS_NonSaveKeyPassesThrough(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	before := len(app.svc.Projects())
	form := formFromCenteredModal(t, views.NewProjectForm(app, nil, func() {}))

	capture := form.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil form input capture (addCtrlS)")
	}
	event := runeKey('z')
	if got := capture(event); got != event {
		t.Error("non-Ctrl+S key must pass through unchanged")
	}
	if got := len(app.svc.Projects()); got != before {
		t.Errorf("project count changed: got %d, want %d (save must not have run)", got, before)
	}
}
