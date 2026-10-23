package views_test

import (
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"

	"github.com/rivo/tview"
)

// statusBarParts returns sb's three text views: the upper, context-specific
// hints, the lower common hints, and the message, reached via the exported
// Flex.GetItem — StatusBar puts the specific-hints line at index 0 and a
// lower row (itself a Flex) at index 1, with common hints at its index 0
// and the message at its index 1 (see statusbar.go).
//
//nolint:nonamedreturns // multiple returns of the same type, hard to differentiate
func statusBarParts(t *testing.T, sb *views.StatusBar) (specificHints, commonHints, message *tview.TextView) {
	t.Helper()
	spec, ok := sb.GetItem(0).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 0, got %T", sb.GetItem(0))
	}
	lowerRow, ok := sb.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected inner *tview.Flex at index 1, got %T", sb.GetItem(1))
	}
	common, ok := lowerRow.GetItem(0).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at lower row index 0, got %T", lowerRow.GetItem(0))
	}
	msg, ok := lowerRow.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at lower row index 1, got %T", lowerRow.GetItem(1))
	}
	return spec, common, msg
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
	spec, common, _ := statusBarParts(t, sb)
	if spec.GetText(false) != views.HintsAccountsPanelSpecific {
		t.Errorf("default specific hints: got %q, want HintsAccountsPanelSpecific", spec.GetText(false))
	}
	if common.GetText(false) != views.HintsAccountsPanelCommon {
		t.Errorf("default common hints: got %q, want HintsAccountsPanelCommon", common.GetText(false))
	}
}

func TestStatusBar_DefaultMessageEmpty(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	_, _, message := statusBarParts(t, sb)
	if message.GetText(false) != "" {
		t.Errorf("expected empty message, got %q", message.GetText(false))
	}
}

func TestStatusBar_SetHints(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		specific string
		common   string
	}{
		{"accounts panel", views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon},
		{"detail panel", views.HintsDetailPanelSpecific, views.HintsDetailPanelCommon},
		{"projects panel", views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon},
		{"providers panel", views.HintsProvidersPanelSpecific, views.HintsProvidersPanelCommon},
		{"queue editor", views.HintsQueueEditorSpecific, views.HintsQueueEditorCommon},
		{"form", views.HintsFormSpecific, views.HintsFormCommon},
		{"confirm modal", views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
		{"help overlay", views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sb := views.NewStatusBar()
			sb.SetHints(tc.specific, tc.common)
			spec, common, _ := statusBarParts(t, sb)
			if spec.GetText(false) != tc.specific {
				t.Errorf("SetHints(%q): specific got %q", tc.name, spec.GetText(false))
			}
			if common.GetText(false) != tc.common {
				t.Errorf("SetHints(%q): common got %q", tc.name, common.GetText(false))
			}
		})
	}
}

// TestStatusBar_CommonHintsConstantsNonEmpty replaces
// TestStatusBar_AllHintsConstantsNonEmpty: every context has at least one
// common hint (help/quit at minimum, confirm/cancel, or close), but a
// context-specific line can legitimately be empty (confirm modal and help
// overlay have none), so only the common half is checked.
func TestStatusBar_CommonHintsConstantsNonEmpty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value string
	}{
		{"HintsAccountsPanelCommon", views.HintsAccountsPanelCommon},
		{"HintsDetailPanelCommon", views.HintsDetailPanelCommon},
		{"HintsProjectsPanelCommon", views.HintsProjectsPanelCommon},
		{"HintsProvidersPanelCommon", views.HintsProvidersPanelCommon},
		{"HintsQueueEditorCommon", views.HintsQueueEditorCommon},
		{"HintsFormCommon", views.HintsFormCommon},
		{"HintsConfirmModalCommon", views.HintsConfirmModalCommon},
		{"HintsHelpOverlayCommon", views.HintsHelpOverlayCommon},
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
			_, _, message := statusBarParts(t, sb)
			if message.GetText(false) != tc.msg {
				t.Errorf("SetMessage(%q): got %q", tc.msg, message.GetText(false))
			}
		})
	}
}
