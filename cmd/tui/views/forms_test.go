package views

import (
	"testing"

	"github.com/balinomad/llm-desk/internal/model"

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

// ── NewSessionForm ────────────────────────────────────────────────────────────

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
				app, acc := newDetailTestApp(t)
				return app, acc, nil
			},
			wantModal: false,
		},
		{
			name: "edit mode",
			setup: func(t *testing.T) (*mockApp, *model.Account, *model.Session) {
				t.Helper()
				app, acc := newDetailTestApp(t)
				sessions, err := app.svc.AccountSessions(acc.UserEmail(), acc.ProviderName())
				must(t, err)
				return app, acc, sessions[0]
			},
			wantModal: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc, sess := tc.setup(t)
			result := NewSessionForm(app, acc, sess, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

func TestSessionFormProjectNames(t *testing.T) {
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

	app := &mockApp{svc: newTestService(t, reg)}
	got := sessionFormProjectNames(app, "alice@example.com", "claude")

	// AccountProjects sorts by name: "Alpha" before "none". Index 0 is
	// always the sentinel, so the real "none" project must land at
	// index 2, distinct from the sentinel at index 0.
	want := []string{"none", "Alpha", "none"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestSessionFormInitial(t *testing.T) {
	t.Parallel()

	// Sentinel, a normal granted project, and a second granted project
	// that happens to be named identically to the sentinel — the exact
	// shape sessionFormProjectNames produces when a project is granted
	// under the name "none".
	projectNames := []string{"none", "Alpha", "none"}

	cases := []struct {
		name      string
		session   *model.Session
		wantTopic string
		wantIdx   int
	}{
		{
			name:      "nil session",
			session:   nil,
			wantTopic: "",
			wantIdx:   0,
		},
		{
			name:      "no project assigned",
			session:   model.NewSession(model.AccountKey{Email: "a@x.com", Provider: "p"}, 1, "", "Topic", nil),
			wantTopic: "Topic",
			wantIdx:   0,
		},
		{
			name:      "normal project matches by name",
			session:   model.NewSession(model.AccountKey{Email: "a@x.com", Provider: "p"}, 1, "Alpha", "Topic", nil),
			wantTopic: "Topic",
			wantIdx:   1,
		},
		{
			name:      "project literally named none resolves to its own index, not the sentinel",
			session:   model.NewSession(model.AccountKey{Email: "a@x.com", Provider: "p"}, 1, "none", "Topic", nil),
			wantTopic: "Topic",
			wantIdx:   2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotTopic, gotIdx := sessionFormInitial(tc.session, projectNames)
			if gotTopic != tc.wantTopic {
				t.Errorf("topic: got %q, want %q", gotTopic, tc.wantTopic)
			}
			if gotIdx != tc.wantIdx {
				t.Errorf("index: got %d, want %d", gotIdx, tc.wantIdx)
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
			result := NewProjectForm(app, proj, func() {})
			assertFormOrModal(t, result, false)
		})
	}
}

func TestNewProjectForm_NameFieldDisabledState(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)

	t.Run("create mode: enabled", func(t *testing.T) {
		t.Parallel()
		form := formFromCenteredModal(t, NewProjectForm(app, nil, func() {}))
		nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
		if !ok {
			t.Fatal("expected Name *tview.InputField")
		}
		if !inputFieldAcceptsText(nameField) {
			t.Error("Name field should accept text in create mode")
		}
	})

	t.Run("edit mode: disabled", func(t *testing.T) {
		t.Parallel()
		form := formFromCenteredModal(t, NewProjectForm(app, app.svc.Project("Alpha"), func() {}))
		nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
		if !ok {
			t.Fatal("expected Name *tview.InputField")
		}
		if inputFieldAcceptsText(nameField) {
			t.Error("Name field should not accept text in edit mode")
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
			result := NewProviderForm(app, prov, func() {})
			assertFormOrModal(t, result, false)
		})
	}
}

func TestNewProviderForm_NameFieldDisabledState(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)

	t.Run("create mode: enabled", func(t *testing.T) {
		t.Parallel()
		form := formFromCenteredModal(t, NewProviderForm(app, nil, func() {}))
		nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
		if !ok {
			t.Fatal("expected Name *tview.InputField")
		}
		if !inputFieldAcceptsText(nameField) {
			t.Error("Name field should accept text in create mode")
		}
	})

	t.Run("edit mode: disabled", func(t *testing.T) {
		t.Parallel()
		form := formFromCenteredModal(t, NewProviderForm(app, app.svc.Provider("claude"), func() {}))
		nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
		if !ok {
			t.Fatal("expected Name *tview.InputField")
		}
		if inputFieldAcceptsText(nameField) {
			t.Error("Name field should not accept text in edit mode")
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
			result := NewAccountForm(app, acc, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

func TestAccountEditForm_EmailProviderDisabled(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	acc, err := app.svc.Account("alice@example.com", "claude")
	must(t, err)

	form := formFromCenteredModal(t, NewAccountForm(app, acc, func() {}))

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
			result := NewUserForm(app, acc, func() {})
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
			result := NewGrantForm(app, proj, func() {})
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
			result := NewRevokeForm(app, proj, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

func TestEscapeAll(t *testing.T) {
	t.Parallel()
	in := []string{"Alpha", "Be[ta]", ""}
	got := escapeAll(in)
	if len(got) != len(in) {
		t.Fatalf("got %d entries, want %d", len(got), len(in))
	}
	for i, want := range in {
		if got[i] != tview.Escape(want) {
			t.Errorf("got[%d] = %q, want %q", i, got[i], tview.Escape(want))
		}
	}
}
