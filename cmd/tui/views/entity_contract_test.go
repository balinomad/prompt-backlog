package views_test

import (
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type entityPanel interface {
	tview.Primitive
	Refresh()
	RequiredInfoHeight(int) int
	ResizeInfo(int, int)
	GetItem(int) tview.Primitive
}

func entityPanelParts(t *testing.T, p entityPanel) (*tview.List, *tview.TextView) {
	t.Helper()
	list, ok := p.GetItem(0).(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List at index 0, got %T", p.GetItem(0))
	}
	info, ok := p.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 1, got %T", p.GetItem(1))
	}
	return list, info
}

func assertKeyOpensModal(t *testing.T, newPanel func(*mockApp) entityPanel, key *tcell.EventKey, wantSpecific, wantCommon string) {
	t.Helper()
	app := newMockApp(t)
	p := newPanel(app)
	p.Refresh()
	list, _ := entityPanelParts(t, p)
	list.GetInputCapture()(key)
	if app.modal == nil {
		t.Fatal("expected a modal")
	}
	assertHints(t, app, wantSpecific, wantCommon)
}

func runEntityScreenContract(t *testing.T, newPanel func(*mockApp) entityPanel, wantItems int, wantInfoTitle string) {
	t.Helper()

	t.Run("refresh populates list", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		list, info := entityPanelParts(t, p)
		if got := list.GetItemCount(); got != wantItems {
			t.Errorf("item count: got %d, want %d", got, wantItems)
		}
		if got := info.GetTitle(); got != wantInfoTitle {
			t.Errorf("info title = %q, want %q", got, wantInfoTitle)
		}
	})

	t.Run("selection restored by key", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		list, _ := entityPanelParts(t, p)
		if list.GetItemCount() < 2 {
			t.Skip("needs at least two items")
		}
		list.SetCurrentItem(1)
		want, _ := list.GetItemText(1)
		p.Refresh()
		got, _ := list.GetItemText(list.GetCurrentItem())
		if got != want {
			t.Errorf("selection changed: got %q, want %q", got, want)
		}
	})

	t.Run("esc switches to main", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		list, _ := entityPanelParts(t, p)
		list.GetInputCapture()(escKey())
		if app.switchedTo != "main" {
			t.Errorf("switchedTo = %q, want main", app.switchedTo)
		}
	})

	t.Run("tab consumed", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		list, _ := entityPanelParts(t, p)
		if got := list.GetInputCapture()(specialKey(views.KeyFocusNext)); got != nil {
			t.Error("Tab must be consumed")
		}
	})

	t.Run("insert opens form", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, specialKey(views.KeyInsert), views.HintsFormSpecific, views.HintsFormCommon)
	})

	t.Run("enter opens form", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsFormCommon)
	})

	t.Run("delete opens confirm", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	})

	t.Run("help opens overlay", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, runeKey(views.KeyHelp), views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon)
	})

	t.Run("quit opens confirm", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, runeKey(views.KeyQuit), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	})

	t.Run("empty list guards", func(t *testing.T) {
		t.Parallel()
		app := &mockApp{svc: newTestService(t, model.NewRegistry())}
		p := newPanel(app)
		p.Refresh()
		list, _ := entityPanelParts(t, p)
		if got := list.GetItemCount(); got != 0 {
			t.Fatalf("item count: got %d, want 0", got)
		}
		list.GetInputCapture()(specialKey(views.KeyDelete))
		if app.modal != nil {
			t.Error("Del on an empty list must not open a modal")
		}
		list.GetInputCapture()(specialKey(views.KeyEdit))
		if app.modal != nil {
			t.Error("Enter on an empty list must not open a modal")
		}
	})

	t.Run("info resize", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := newPanel(app)
		p.Refresh()
		if got := p.RequiredInfoHeight(80); got <= 0 {
			t.Errorf("RequiredInfoHeight(80) = %d, want positive", got)
		}
		p.ResizeInfo(80, 24)
	})
}

func TestVendorsPanel_Contract(t *testing.T) {
	t.Parallel()
	runEntityScreenContract(t, func(app *mockApp) entityPanel {
		return views.NewVendorsPanel(app)
	}, 2, " Vendor Details ")
}

func TestUsersPanel_Contract(t *testing.T) {
	t.Parallel()
	runEntityScreenContract(t, func(app *mockApp) entityPanel {
		return views.NewUsersPanel(app)
	}, 2, " User Details ")
}
