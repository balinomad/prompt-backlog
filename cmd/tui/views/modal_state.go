package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// BorderBox is a primitive whose border colour can be read and changed.
type BorderBox interface {
	SetBorderColor(tcell.Color) *tview.Box
	GetBorderColor() tcell.Color
}

// ModalState marks a modal as active or inactive.
type ModalState interface {
	SetModalActive(active bool)
}

type modalState struct {
	tview.Primitive
	border      BorderBox
	activeColor tcell.Color
}

// NewModalState wraps a modal and the border that identifies its active state.
func NewModalState(modal tview.Primitive, border BorderBox) tview.Primitive {
	return &modalState{
		Primitive:   modal,
		border:      border,
		activeColor: border.GetBorderColor(),
	}
}

// SetModalActive restores the original border colour when active and uses gray
// when inactive.
func (m *modalState) SetModalActive(active bool) {
	if active {
		m.border.SetBorderColor(m.activeColor)
		return
	}
	m.border.SetBorderColor(tcell.ColorGray)
}

// UnwrapModal returns the primitive inside a modal-state wrapper.
func UnwrapModal(p tview.Primitive) tview.Primitive {
	if state, ok := p.(*modalState); ok {
		return state.Primitive
	}
	return p
}
