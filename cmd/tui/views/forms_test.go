package views_test

import (
	"testing"

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
