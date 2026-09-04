package views_test

import (
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"

	"github.com/rivo/tview"
)

// ── NewConfirmModal ───────────────────────────────────────────────────────────

func TestNewConfirmModal_EscCallsCancel(t *testing.T) {
	t.Parallel()
	var cancelled bool
	prim := views.NewConfirmModal("test?", func() {}, func() { cancelled = true })
	modal, ok := prim.(*tview.Modal)
	if !ok {
		t.Fatal("NewConfirmModal must return *tview.Modal")
	}
	capture := modal.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	result := capture(escKey())
	if result != nil {
		t.Error("Esc must consume the event (return nil)")
	}
	if !cancelled {
		t.Error("Esc must call onCancel")
	}
}

func TestNewConfirmModal_NonEscPassesThrough(t *testing.T) {
	t.Parallel()
	prim := views.NewConfirmModal("test?", func() {}, func() {})
	modal, ok := prim.(*tview.Modal)
	if !ok {
		t.Fatal("NewConfirmModal must return *tview.Modal")
	}
	capture := modal.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	event := runeKey('n')
	if got := capture(event); got != event {
		t.Error("non-Esc key must pass through unchanged")
	}
}

func TestNewConfirmModal_ReturnsModal(t *testing.T) {
	t.Parallel()
	prim := views.NewConfirmModal("message", func() {}, func() {})
	if _, ok := prim.(*tview.Modal); !ok {
		t.Error("expected *tview.Modal")
	}
}

// ── NewHelpOverlay / showHelp ───────────────────────────────────────────────────

// helpOverlayTextView drills into the Flex/Flex/Frame structure
// NewHelpOverlay returns to reach the inner TextView, where its Esc
// handling is actually registered.
func helpOverlayTextView(t *testing.T, overlay tview.Primitive) *tview.TextView {
	t.Helper()
	outer, ok := overlay.(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex, got %T", overlay)
	}
	inner, ok := outer.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected inner *tview.Flex, got %T", outer.GetItem(1))
	}
	frame, ok := inner.GetItem(1).(*tview.Frame)
	if !ok {
		t.Fatalf("expected *tview.Frame, got %T", inner.GetItem(1))
	}
	text, ok := frame.GetPrimitive().(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView, got %T", frame.GetPrimitive())
	}
	return text
}

func TestNewHelpOverlay_EscCallsOnClose(t *testing.T) {
	t.Parallel()
	var closed bool
	overlay := views.NewHelpOverlay(func() { closed = true })
	capture := helpOverlayTextView(t, overlay).GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	result := capture(escKey())
	if result != nil {
		t.Error("Esc must consume the event (return nil)")
	}
	if !closed {
		t.Error("Esc must call onClose")
	}
}

// TestShowHelp_EscRestoresHintsAndHidesModal exercises the unexported
// showHelp indirectly through AccountsPanel's KeyHelp handling —
// AccountsPanel embeds *tview.List directly, so its GetInputCapture()
// reaches handleKey from outside the package. The restored-hints value is
// necessarily HintsAccountsPanel here, since that's what
// AccountsPanel.handleKey passes to showHelp; the round trip itself
// (open, HintsHelpOverlay, Esc, hints restored, modal hidden) is what's
// under test, not the specific string.
func TestShowHelp_EscRestoresHintsAndHidesModal(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)

	capture := p.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(runeKey(views.KeyHelp))

	if app.modal == nil {
		t.Fatal("expected a modal to be shown")
	}
	if app.hints != views.HintsHelpOverlay {
		t.Errorf("hints while open: got %q, want %q", app.hints, views.HintsHelpOverlay)
	}

	helpOverlayTextView(t, app.modal).GetInputCapture()(escKey())

	if app.modal != nil {
		t.Error("expected modal to be hidden after Esc")
	}
	if app.hints != views.HintsAccountsPanel {
		t.Errorf("hints after close: got %q, want %q", app.hints, views.HintsAccountsPanel)
	}
}
