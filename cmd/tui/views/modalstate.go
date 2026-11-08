package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// BorderBox is a primitive whose border and title colours can be changed.
type BorderBox interface {
	SetBorderColor(tcell.Color) *tview.Box
	SetTitleColor(tcell.Color) *tview.Box
}

// ModalState marks a modal as active or inactive.
type ModalState interface {
	SetModalActive(active bool)
}

type modalState struct {
	tview.Primitive
	border BorderBox
}

// NewModalState wraps a modal and the border that identifies its active state.
func NewModalState(modal tview.Primitive, border BorderBox) tview.Primitive {
	return &modalState{
		Primitive: modal,
		border:    border,
	}
}

// SetModalActive uses the accent colour when active and gray when inactive.
func (m *modalState) SetModalActive(active bool) {
	if active {
		m.border.SetBorderColor(ColorAccent)
		m.border.SetTitleColor(ColorAccent)
		return
	}
	m.border.SetBorderColor(ColorInactive)
	m.border.SetTitleColor(ColorInactive)
}

// UnwrapModal returns the primitive inside a modal-state wrapper.
func UnwrapModal(p tview.Primitive) tview.Primitive {
	if state, ok := p.(*modalState); ok {
		return state.Primitive
	}
	return p
}
