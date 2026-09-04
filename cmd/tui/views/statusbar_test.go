package views_test

import (
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"

	"github.com/rivo/tview"
)

// statusBarTexts returns the hints (left) and message (right) text
// currently displayed by sb, reached via the exported Flex.GetItem —
// StatusBar puts hints at index 0, message at index 1 (see statusbar.go).
func statusBarTexts(t *testing.T, sb *views.StatusBar) (string, string) {
	t.Helper()
	hints, ok := sb.GetItem(0).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 0, got %T", sb.GetItem(0))
	}
	message, ok := sb.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 1, got %T", sb.GetItem(1))
	}
	return hints.GetText(false), message.GetText(false)
}

func TestNewStatusBar(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	if sb == nil {
		t.Fatal("expected non-nil StatusBar")
	}
}

func TestStatusBar_DefaultHints(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	hints, _ := statusBarTexts(t, sb)
	if hints != views.HintsAccountsPanel {
		t.Errorf("default hints: got %q, want HintsAccountsPanel", hints)
	}
}

func TestStatusBar_DefaultMessageEmpty(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	_, message := statusBarTexts(t, sb)
	if message != "" {
		t.Errorf("expected empty message, got %q", message)
	}
}

func TestStatusBar_SetHints(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		hints string
	}{
		{"accounts panel", views.HintsAccountsPanel},
		{"detail panel", views.HintsDetailPanel},
		{"projects panel", views.HintsProjectsPanel},
		{"providers panel", views.HintsProvidersPanel},
		{"queue editor", views.HintsQueueEditor},
		{"form", views.HintsForm},
		{"confirm modal", views.HintsConfirmModal},
		{"help overlay", views.HintsHelpOverlay},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sb := views.NewStatusBar()
			sb.SetHints(tc.hints)
			hints, _ := statusBarTexts(t, sb)
			if hints != tc.hints {
				t.Errorf("SetHints(%q): got %q", tc.name, hints)
			}
		})
	}
}

func TestStatusBar_AllHintsConstantsNonEmpty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value string
	}{
		{"HintsAccountsPanel", views.HintsAccountsPanel},
		{"HintsDetailPanel", views.HintsDetailPanel},
		{"HintsProjectsPanel", views.HintsProjectsPanel},
		{"HintsProvidersPanel", views.HintsProvidersPanel},
		{"HintsQueueEditor", views.HintsQueueEditor},
		{"HintsForm", views.HintsForm},
		{"HintsConfirmModal", views.HintsConfirmModal},
		{"HintsHelpOverlay", views.HintsHelpOverlay},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.value == "" {
				t.Errorf("%s must not be empty", tc.name)
			}
		})
	}
}

func TestStatusBar_SetMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		msg  string
	}{
		{"non-empty message", "save failed"},
		{"empty clears message", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sb := views.NewStatusBar()
			sb.SetMessage(tc.msg)
			_, message := statusBarTexts(t, sb)
			if message != tc.msg {
				t.Errorf("SetMessage(%q): got %q", tc.msg, message)
			}
		})
	}
}
