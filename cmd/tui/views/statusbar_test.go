package views

import "testing"

func TestNewStatusBar(t *testing.T) {
	t.Parallel()
	sb := NewStatusBar()
	if sb == nil {
		t.Fatal("expected non-nil StatusBar")
	}
}

func TestStatusBar_DefaultHints(t *testing.T) {
	t.Parallel()
	sb := NewStatusBar()
	if got := sb.hintsView.GetText(false); got != HintsAccountsPanel {
		t.Errorf("default hints: got %q, want HintsAccountsPanel", got)
	}
}

func TestStatusBar_DefaultMessageEmpty(t *testing.T) {
	t.Parallel()
	sb := NewStatusBar()
	if got := sb.messageView.GetText(false); got != "" {
		t.Errorf("expected empty message, got %q", got)
	}
}

func TestStatusBar_SetHints(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		hints string
	}{
		{"accounts panel", HintsAccountsPanel},
		{"detail panel", HintsDetailPanel},
		{"projects panel", HintsProjectsPanel},
		{"providers panel", HintsProvidersPanel},
		{"queue editor", HintsQueueEditor},
		{"form", HintsForm},
		{"confirm modal", HintsConfirmModal},
		{"help overlay", HintsHelpOverlay},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sb := NewStatusBar()
			sb.SetHints(tc.hints)
			if got := sb.hintsView.GetText(false); got != tc.hints {
				t.Errorf("SetHints(%q): got %q", tc.name, got)
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
		{"HintsAccountsPanel", HintsAccountsPanel},
		{"HintsDetailPanel", HintsDetailPanel},
		{"HintsProjectsPanel", HintsProjectsPanel},
		{"HintsProvidersPanel", HintsProvidersPanel},
		{"HintsQueueEditor", HintsQueueEditor},
		{"HintsForm", HintsForm},
		{"HintsConfirmModal", HintsConfirmModal},
		{"HintsHelpOverlay", HintsHelpOverlay},
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
			sb := NewStatusBar()
			sb.SetMessage(tc.msg)
			if got := sb.messageView.GetText(false); got != tc.msg {
				t.Errorf("SetMessage(%q): got %q", tc.msg, got)
			}
		})
	}
}
