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
