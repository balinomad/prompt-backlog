package views

import (
	"testing"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/rivo/tview"
)

// ── NewSessionForm ────────────────────────────────────────────────────────────

func TestNewSessionForm_NoProviders_ReturnsErrorModal(t *testing.T) {
	t.Parallel()
	// An empty registry has no providers; the form cannot be constructed.
	app := &mockApp{registry: model.NewRegistry()}
	result := NewSessionForm(app, nil, func() {})
	if _, ok := result.(*tview.Modal); !ok {
		t.Error("expected error modal when no providers configured")
	}
}

func TestNewSessionForm_WithProviders_ReturnsForm(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	result := NewSessionForm(app, nil, func() {})
	if _, ok := result.(*tview.Flex); !ok {
		t.Error("expected centred form when providers are available")
	}
}

// ── NewProjectForm ────────────────────────────────────────────────────────────

func TestNewProjectForm_IncompatibleProvider_ReturnsErrorModal(t *testing.T) {
	t.Parallel()
	// bob/chatgpt has HasProjects=false — pre-flight must fire.
	app := newMockApp(t)
	s := app.registry.SessionAt("bob@example.com", "chatgpt")
	result := NewProjectForm(app, nil, s, func() {})
	if _, ok := result.(*tview.Modal); !ok {
		t.Error("expected error modal for provider without project support")
	}
}

func TestNewProjectForm_ReturnsForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		userEmail    string // empty = nil session
		providerName string
		projectName  string // empty = nil (new mode), non-empty = edit mode
	}{
		{"compatible provider", "alice@example.com", "claude", ""},
		{"nil session", "", "", ""},
		{"edit mode bypasses provider check", "bob@example.com", "chatgpt", "Orphan"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			var s *model.Session
			if tc.userEmail != "" {
				s = app.registry.SessionAt(tc.userEmail, tc.providerName)
			}
			var proj *model.Project
			if tc.projectName != "" {
				proj = app.registry.ProjectByName(tc.projectName)
			}
			result := NewProjectForm(app, proj, s, func() {})
			if _, ok := result.(*tview.Flex); !ok {
				t.Errorf("expected centred form for case %q", tc.name)
			}
		})
	}
}

// ── NewLastQuestionForm ───────────────────────────────────────────────────────

func TestNewLastQuestionForm_NilSession_ReturnsErrorModal(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	result := NewLastQuestionForm(app, nil, func() {})
	if _, ok := result.(*tview.Modal); !ok {
		t.Error("expected error modal for nil session")
	}
}

func TestNewLastQuestionForm_WithSession_ReturnsForm(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	s := app.registry.SessionAt("alice@example.com", "claude")
	result := NewLastQuestionForm(app, s, func() {})
	if _, ok := result.(*tview.Flex); !ok {
		t.Error("expected centred form when session is valid")
	}
}
