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
