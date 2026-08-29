package views

import (
	"testing"

	"github.com/rivo/tview"
)

// ── NewConfirmModal ───────────────────────────────────────────────────────────

func TestNewConfirmModal_EscCallsCancel(t *testing.T) {
	t.Parallel()
	var cancelled bool
	prim := NewConfirmModal("test?", func() {}, func() { cancelled = true })
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
	prim := NewConfirmModal("test?", func() {}, func() {})
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
	prim := NewConfirmModal("message", func() {}, func() {})
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
	overlay := NewHelpOverlay(func() { closed = true })
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

func TestShowHelp_EscRestoresHintsAndHidesModal(t *testing.T) {
	t.Parallel()
	app := &mockApp{}
	showHelp(app, "return hints")
	if app.modal == nil {
		t.Fatal("expected a modal to be shown")
	}
	if app.hints != HintsHelpOverlay {
		t.Errorf("hints while open: got %q, want %q", app.hints, HintsHelpOverlay)
	}

	helpOverlayTextView(t, app.modal).GetInputCapture()(escKey())

	if app.modal != nil {
		t.Error("expected modal to be hidden after Esc")
	}
	if app.hints != "return hints" {
		t.Errorf("hints after close: got %q, want %q", app.hints, "return hints")
	}
}
