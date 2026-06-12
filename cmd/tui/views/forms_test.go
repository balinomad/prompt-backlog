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

func TestNewProjectForm_CompatibleProvider_ReturnsForm(t *testing.T) {
	t.Parallel()
	// alice/claude has HasProjects=true — form should open normally.
	app := newMockApp(t)
	s := app.registry.SessionAt("alice@example.com", "claude")
	result := NewProjectForm(app, nil, s, func() {})
	if _, ok := result.(*tview.Flex); !ok {
		t.Error("expected centred form for provider with project support")
	}
}

func TestNewProjectForm_NilSession_ReturnsForm(t *testing.T) {
	t.Parallel()
	// nil session means unattached project — no provider check needed.
	app := newMockApp(t)
	result := NewProjectForm(app, nil, nil, func() {})
	if _, ok := result.(*tview.Flex); !ok {
		t.Error("expected form when session is nil")
	}
}

func TestNewProjectForm_EditMode_ReturnsForm(t *testing.T) {
	t.Parallel()
	// Edit mode skips the pre-flight check entirely.
	app := newMockApp(t)
	// Use the chatgpt session (incompatible), but edit an existing project —
	// the pre-flight guard must not fire.
	s := app.registry.SessionAt("bob@example.com", "chatgpt")
	proj := app.registry.ProjectByName("Orphan")
	result := NewProjectForm(app, proj, s, func() {})
	if _, ok := result.(*tview.Flex); !ok {
		t.Error("expected form in edit mode regardless of provider capability")
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
