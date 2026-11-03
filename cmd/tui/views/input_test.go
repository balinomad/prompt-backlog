package views_test

import (
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestWireListDoubleClick(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		action     tview.MouseAction
		x, y       int
		wantCalled bool
		wantAction tview.MouseAction
		wantEvent  bool
	}{
		{
			name:       "double click inside",
			action:     tview.MouseLeftDoubleClick,
			x:          2,
			y:          2,
			wantCalled: true,
			wantAction: tview.MouseConsumed,
		},
		{
			name:       "single click inside",
			action:     tview.MouseLeftClick,
			x:          2,
			y:          2,
			wantAction: tview.MouseLeftClick,
			wantEvent:  true,
		},
		{
			name:       "double click border",
			action:     tview.MouseLeftDoubleClick,
			x:          0,
			y:          2,
			wantAction: tview.MouseLeftDoubleClick,
			wantEvent:  true,
		},
		{
			name:       "double click outside",
			action:     tview.MouseLeftDoubleClick,
			x:          20,
			y:          2,
			wantAction: tview.MouseLeftDoubleClick,
			wantEvent:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			list := tview.NewList()
			list.SetBorder(true)
			list.SetRect(0, 0, 20, 10)

			called := false
			views.WireListDoubleClick(list, func() { called = true })

			event := tcell.NewEventMouse(tc.x, tc.y, tcell.ButtonPrimary, tcell.ModNone)
			action, gotEvent := list.GetMouseCapture()(tc.action, event)

			if called != tc.wantCalled {
				t.Errorf("action called = %v, want %v", called, tc.wantCalled)
			}
			if action != tc.wantAction {
				t.Errorf("action = %v, want %v", action, tc.wantAction)
			}
			if (gotEvent != nil) != tc.wantEvent {
				t.Errorf("event present = %v, want %v", gotEvent != nil, tc.wantEvent)
			}
		})
	}
}
