package panels_test

import (
	"strings"
	"testing"

	"github.com/balinomad/prompt-backlog/cmd/tui/views/panels"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/internal/viewtest"
	"github.com/balinomad/prompt-backlog/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type fakeApp = viewtest.FakeApp

var (
	must             = viewtest.Must
	newService       = viewtest.NewService
	newFakeApp       = viewtest.NewFakeApp
	runeKey          = viewtest.RuneKey
	escKey           = viewtest.EscKey
	specialKey       = viewtest.SpecialKey
	assertHints      = viewtest.AssertHints
	confirmModalFrom = viewtest.ConfirmModalFrom
)

type panelContract interface {
	tview.Primitive
	Refresh()
	RequiredInfoHeight(int) int
	ResizeInfo(int, int)
	GetItem(int) tview.Primitive
}

func panelParts(t *testing.T, p panelContract) (*tview.List, *tview.TextView) {
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

func assertKeyOpensModal(t *testing.T, newPanel func(*fakeApp) panelContract, key *tcell.EventKey, wantSpecific, wantCommon string) {
	t.Helper()
	app := newFakeApp(t)
	p := newPanel(app)
	p.Refresh()
	list, _ := panelParts(t, p)
	list.GetInputCapture()(key)
	if app.Modal == nil {
		t.Fatal("expected a modal")
	}
	assertHints(t, app, wantSpecific, wantCommon)
}

func runPanelContract(t *testing.T, newPanel func(*fakeApp) panelContract, wantItems int, wantInfoTitle string) {
	t.Helper()

	t.Run("refresh populates list", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		p := newPanel(app)
		p.Refresh()
		list, info := panelParts(t, p)
		if got := list.GetItemCount(); got != wantItems {
			t.Errorf("item count: got %d, want %d", got, wantItems)
		}
		if got := info.GetTitle(); got != wantInfoTitle {
			t.Errorf("info title = %q, want %q", got, wantInfoTitle)
		}
	})

	t.Run("selection restored by key", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		p := newPanel(app)
		p.Refresh()
		list, _ := panelParts(t, p)
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
		app := newFakeApp(t)
		p := newPanel(app)
		p.Refresh()
		list, _ := panelParts(t, p)
		list.GetInputCapture()(escKey())
		if app.SwitchedTo != "main" {
			t.Errorf("switchedTo = %q, want main", app.SwitchedTo)
		}
	})

	t.Run("tab consumed", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		p := newPanel(app)
		p.Refresh()
		list, _ := panelParts(t, p)
		if got := list.GetInputCapture()(specialKey(views.KeyFocusNext)); got != nil {
			t.Error("Tab must be consumed")
		}
	})

	t.Run("insert opens form", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, specialKey(views.KeyInsert), views.HintsFormSpecific, views.HintsTextFormCommon)
	})

	t.Run("enter opens form", func(t *testing.T) {
		t.Parallel()
		assertKeyOpensModal(t, newPanel, specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsTextFormCommon)
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
		app := &fakeApp{Svc: newService(t, model.NewRegistry())}
		p := newPanel(app)
		p.Refresh()
		list, _ := panelParts(t, p)
		if got := list.GetItemCount(); got != 0 {
			t.Fatalf("item count: got %d, want 0", got)
		}
		list.GetInputCapture()(specialKey(views.KeyDelete))
		if app.Modal != nil {
			t.Error("Del on an empty list must not open a modal")
		}
		list.GetInputCapture()(specialKey(views.KeyEdit))
		if app.Modal != nil {
			t.Error("Enter on an empty list must not open a modal")
		}
	})

	t.Run("info resize", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
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
	runPanelContract(t, func(app *fakeApp) panelContract {
		return panels.NewVendorsPanel(app)
	}, 2, " Vendor Details ")
}

func TestUsersPanel_Contract(t *testing.T) {
	t.Parallel()
	runPanelContract(t, func(app *fakeApp) panelContract {
		return panels.NewUsersPanel(app)
	}, 2, " User Details ")
}

func TestPanels_DoubleClickOpensEditForm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		newPanel  func(*fakeApp) panelContract
		wantTitle string
	}{
		{
			name: "vendors",
			newPanel: func(app *fakeApp) panelContract {
				return panels.NewVendorsPanel(app)
			},
			wantTitle: "Edit Vendor",
		},
		{
			name: "users",
			newPanel: func(app *fakeApp) panelContract {
				return panels.NewUsersPanel(app)
			},
			wantTitle: "Edit User",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := newFakeApp(t)
			p := tc.newPanel(app)
			p.Refresh()
			list, _ := panelParts(t, p)

			doubleClickList(t, list)

			form := formFromCenteredModal(t, app.Modal)
			if got := form.GetTitle(); !strings.Contains(got, tc.wantTitle) {
				t.Errorf("title = %q, want it to contain %q", got, tc.wantTitle)
			}
		})
	}
}

func TestListTitles_Panels(t *testing.T) {
	t.Parallel()
	t.Run("vendors", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		p := panels.NewVendorsPanel(app)
		p.Refresh()
		list, _ := vendorsPanelParts(t, p)
		if got, want := list.GetTitle(), " Vendors 1/2 "; got != want {
			t.Errorf("initial title = %q, want %q", got, want)
		}
		list.SetCurrentItem(1)
		if got, want := list.GetTitle(), " Vendors 2/2 "; got != want {
			t.Errorf("selected title = %q, want %q", got, want)
		}
		app.Svc = newService(t, model.NewRegistry())
		p.Refresh()
		if got, want := list.GetTitle(), " Vendors 0/0 "; got != want {
			t.Errorf("empty title = %q, want %q", got, want)
		}
	})

	t.Run("users", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		p := panels.NewUsersPanel(app)
		p.Refresh()
		list, _ := usersPanelParts(t, p)
		if got, want := list.GetTitle(), " Users 1/2 "; got != want {
			t.Errorf("initial title = %q, want %q", got, want)
		}
		list.SetCurrentItem(1)
		if got, want := list.GetTitle(), " Users 2/2 "; got != want {
			t.Errorf("selected title = %q, want %q", got, want)
		}
		app.Svc = newService(t, model.NewRegistry())
		p.Refresh()
		if got, want := list.GetTitle(), " Users 0/0 "; got != want {
			t.Errorf("empty title = %q, want %q", got, want)
		}
	})
}

// centeredModalContent returns the content inside a centered modal wrapper.
func centeredModalContent(t *testing.T, p tview.Primitive) tview.Primitive {
	t.Helper()
	p = views.UnwrapModal(p)
	outer, ok := p.(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex, got %T", p)
	}
	column, ok := outer.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected column *tview.Flex, got %T", outer.GetItem(1))
	}
	row, ok := column.GetItem(2).(*tview.Flex)
	if !ok {
		t.Fatalf("expected row *tview.Flex, got %T", column.GetItem(2))
	}
	content := row.GetItem(2)
	if frame, ok := content.(*tview.Frame); ok {
		content = frame.GetPrimitive()
	}
	return content
}

// centeredModalTitle returns the title of a centered modal's content.
func centeredModalTitle(t *testing.T, p tview.Primitive) string {
	t.Helper()
	titled, ok := centeredModalContent(t, p).(interface{ GetTitle() string })
	if !ok {
		t.Fatalf("expected titled content, got %T", centeredModalContent(t, p))
	}
	return titled.GetTitle()
}

// findDropDownByLabel returns the dropdown with label inside p, or nil.
func findDropDownByLabel(p tview.Primitive, label string) *tview.DropDown {
	if dropdown, ok := p.(*tview.DropDown); ok {
		if dropdown.GetLabel() == label {
			return dropdown
		}
		return nil
	}
	container, ok := p.(interface {
		GetItemCount() int
		GetItem(int) tview.Primitive
	})
	if !ok {
		return nil
	}
	for i := range container.GetItemCount() {
		if dropdown := findDropDownByLabel(container.GetItem(i), label); dropdown != nil {
			return dropdown
		}
	}
	return nil
}

// formFromCenteredModal returns the form inside a centered modal wrapper.
func formFromCenteredModal(t *testing.T, p tview.Primitive) *tview.Form {
	t.Helper()
	form, ok := centeredModalContent(t, p).(*tview.Form)
	if !ok {
		t.Fatalf("expected *tview.Form, got %T", centeredModalContent(t, p))
	}
	return form
}

// setInputField sets an input field's text.
func setInputField(t *testing.T, form *tview.Form, label, text string) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	field, ok := item.(*forms.InputField)
	if !ok {
		t.Fatalf("expected %q to be *forms.InputField, got %T", label, item)
	}
	field.SetText(text, false)
}

// setTextArea sets a text area's text.
func setTextArea(t *testing.T, form *tview.Form, label, text string) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	field, ok := item.(*tview.TextArea)
	if !ok {
		t.Fatalf("expected %q to be *tview.TextArea, got %T", label, item)
	}
	field.SetText(text, true)
}

// setCheckbox checks a checkbox.
func setCheckbox(t *testing.T, form *tview.Form, label string) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	cb, ok := item.(*tview.Checkbox)
	if !ok {
		t.Fatalf("expected %q to be *tview.Checkbox, got %T", label, item)
	}
	cb.SetChecked(true)
}

// setDropDown selects a dropdown option.
func setDropDown(t *testing.T, form *tview.Form, label string, index int) {
	t.Helper()
	item := form.GetFormItemByLabel(label)
	dd, ok := item.(*tview.DropDown)
	if !ok {
		t.Fatalf("expected %q to be *tview.DropDown, got %T", label, item)
	}
	dd.SetCurrentOption(index)
}

// triggerSave invokes the form's Ctrl+S capture.
func triggerSave(t *testing.T, form *tview.Form) {
	t.Helper()
	capture := form.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil form input capture (addCtrlS)")
	}
	capture(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone))
}

// dropdownOption returns the display text of one dropdown option.
func dropdownOption(t *testing.T, dropdown *tview.DropDown, index int) string {
	t.Helper()
	dropdown.SetCurrentOption(index)
	_, option := dropdown.GetCurrentOption()
	return option
}

// doubleClickList invokes list's mouse capture with a double click.
func doubleClickList(t *testing.T, list *tview.List) {
	t.Helper()
	list.SetRect(0, 0, 40, 10)
	capture := list.GetMouseCapture()
	if capture == nil {
		t.Fatal("list has no mouse capture")
	}
	event := tcell.NewEventMouse(2, 2, tcell.ButtonPrimary, tcell.ModNone)
	action, gotEvent := capture(tview.MouseLeftDoubleClick, event)
	if action != tview.MouseConsumed || gotEvent != nil {
		t.Fatalf("mouse capture = (%v, %v), want (%v, nil)", action, gotEvent, tview.MouseConsumed)
	}
}
