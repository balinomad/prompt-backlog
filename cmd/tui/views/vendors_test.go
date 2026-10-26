package views_test

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// vendorsPanelParts returns p's vendor list and info view, reached
// via the exported Flex.GetItem — VendorsPanel puts the list at index
// 0, the info view at index 1 (see vendors.go).
func vendorsPanelParts(t *testing.T, p *views.VendorsPanel) (*tview.List, *tview.TextView) {
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

func TestNewVendorsPanel(t *testing.T) {
	t.Parallel()
	p := views.NewVendorsPanel(newMockApp(t))
	if p == nil {
		t.Fatal("expected non-nil VendorsPanel")
	}
	info, ok := p.GetItem(1).(interface{ GetTitle() string })
	if !ok {
		t.Fatal("detail panel does not expose a title")
	}
	if got := info.GetTitle(); got != " Vendor Details " {
		t.Errorf("GetTitle() = %q, want %q", got, " Vendor Details ")
	}
}

func TestVendorsPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*testing.T, *mockApp)
		wantCount int
	}{
		{"two vendors", func(_ *testing.T, _ *mockApp) {}, 2},
		{"empty registry", func(t *testing.T, app *mockApp) {
			t.Helper()
			app.svc = newTestService(t, model.NewRegistry())
		}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			tc.setup(t, app)
			p := views.NewVendorsPanel(app)
			p.Refresh()
			list, _ := vendorsPanelParts(t, p)
			if got := list.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestVendorsPanel_Refresh_SortedByName(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewVendorsPanel(app)
	p.Refresh()
	list, _ := vendorsPanelParts(t, p)

	itemCount := list.GetItemCount()
	names := make([]string, 0, itemCount)
	for i := range itemCount {
		main, _ := list.GetItemText(i)
		names = append(names, main)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("list not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

func TestVendorsPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewVendorsPanel(app)
	p.Refresh()
	list, _ := vendorsPanelParts(t, p)
	list.SetCurrentItem(1)
	want, _ := list.GetItemText(1)

	p.Refresh()

	got, _ := list.GetItemText(list.GetCurrentItem())
	if got != want {
		t.Errorf("selection changed: got %q, want %q", got, want)
	}
}

func TestVendorsPanel_Keys_SwitchToMain(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewVendorsPanel(app)
	p.Refresh()
	list, _ := vendorsPanelParts(t, p)
	list.GetInputCapture()(escKey())
	if app.switchedTo != "main" {
		t.Errorf("expected switchedTo=main, got %q", app.switchedTo)
	}
}

func TestVendorsPanel_Keys_NoHiddenAliases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		event *tcell.EventKey
	}{
		{"v does not switch screens", runeKey(views.KeyVendors)},
		{"tab is consumed", specialKey(views.KeyFocusNext)},
		{"backtab is consumed", specialKey(views.KeyFocusPrev)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := views.NewVendorsPanel(app)
			p.Refresh()
			list, _ := vendorsPanelParts(t, p)
			got := list.GetInputCapture()(tc.event)
			if app.switchedTo != "" {
				t.Errorf("expected no screen switch, got %q", app.switchedTo)
			}
			if tc.event.Key() == views.KeyFocusNext || tc.event.Key() == views.KeyFocusPrev {
				if got != nil {
					t.Error("Tab must be consumed")
				}
			}
		})
	}
}

func TestVendorsPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		key          *tcell.EventKey
		wantSpecific string
		wantCommon   string
	}{
		{"new", specialKey(views.KeyInsert), views.HintsFormSpecific, views.HintsFormCommon},
		{"edit", specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsFormCommon},
		{"delete", specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
		{"help", runeKey(views.KeyHelp), views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon},
		{"quit", runeKey(views.KeyQuit), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := views.NewVendorsPanel(app)
			p.Refresh()
			list, _ := vendorsPanelParts(t, p)
			capture := list.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.modal == nil {
				t.Errorf("key %q: expected modal", tc.name)
			}
			assertHints(t, app, tc.wantSpecific, tc.wantCommon)
		})
	}
}

func TestVendorsPanel_EmptyList_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  *tcell.EventKey
	}{
		{"edit", specialKey(views.KeyEdit)},
		{"delete", specialKey(views.KeyDelete)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			app.svc = newTestService(t, model.NewRegistry())
			p := views.NewVendorsPanel(app)
			p.Refresh()
			list, _ := vendorsPanelParts(t, p)
			list.GetInputCapture()(tc.key)
			if app.modal != nil {
				t.Errorf("key %q with empty list should not open a modal", tc.name)
			}
		})
	}
}

// TestVendorsPanel_New_SaveReturnsToPanel closes the loop that
// TestVendorsPanel_Keys_ShowModal leaves open: that test only checks
// that pressing Ins opens a modal with the right hints, not that
// completing the form hands control back to the panel correctly.
func TestVendorsPanel_New_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewVendorsPanel(app)
	p.Refresh()
	list, _ := vendorsPanelParts(t, p)

	list.GetInputCapture()(specialKey(views.KeyInsert))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Name", "gemini")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsVendorsPanelSpecific, views.HintsVendorsPanelCommon)
	if app.svc.Vendor("gemini") == nil {
		t.Error("vendor not created")
	}
}

// TestVendorsPanel_Edit_SaveReturnsToPanel is the same round trip as
// TestVendorsPanel_New_SaveReturnsToPanel for handleEdit's own onClose
// closure.
func TestVendorsPanel_Edit_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewVendorsPanel(app)
	p.Refresh() // selects "chatgpt" first (sorted by name)
	list, _ := vendorsPanelParts(t, p)

	list.GetInputCapture()(specialKey(views.KeyEdit))

	form := formFromCenteredModal(t, app.modal)
	setCheckbox(t, form, "Supports projects")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsVendorsPanelSpecific, views.HintsVendorsPanelCommon)
	if got := app.svc.Vendor("chatgpt"); !got.HasProjects() {
		t.Error("HasProjects() = false, want true")
	}
}

// TestVendorsPanel_Delete_Cancel is the vendors.go counterpart to
// TestDetailPanel_DeleteSession_Cancel and TestAccountsPanel_Delete_Cancel:
// confirming the delete is button-click-gated and unreachable black box,
// so this covers the reachable half.
func TestVendorsPanel_Delete_Cancel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewVendorsPanel(app)
	p.Refresh()
	list, _ := vendorsPanelParts(t, p)

	before := len(app.svc.Vendors())

	list.GetInputCapture()(specialKey(views.KeyDelete))

	confirmModal := confirmModalFrom(t, app.modal)
	confirmModal.GetInputCapture()(escKey())

	if app.modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	assertHints(t, app, views.HintsVendorsPanelSpecific, views.HintsVendorsPanelCommon)
	if got := len(app.svc.Vendors()); got != before {
		t.Errorf("vendor count changed after cancel: got %d, want %d (unchanged)", got, before)
	}
}

// TestVendorsPanel_RowText verifies the rendered vendor row. A vendor
// with no capabilities renders as its name; any capability adds a second
// column.
func TestVendorsPanel_RowText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		hasProjects     bool
		hasDeepResearch bool
		wantEqualName   bool
	}{
		{"no capabilities returns name only", false, false, true},
		{"has projects appends tag", true, false, false},
		{"has deep research appends tag", false, true, false},
		{"all capabilities appends tags", true, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddVendor(model.NewVendor("test", tc.hasProjects, tc.hasDeepResearch)))
			app := &mockApp{svc: newTestService(t, reg)}
			p := views.NewVendorsPanel(app)
			p.Refresh()
			list, _ := vendorsPanelParts(t, p)
			got, _ := list.GetItemText(0)
			if !strings.HasPrefix(got, " ") {
				t.Errorf("got %q, want a leading one-cell inset", got)
			}
			trimmed := strings.TrimSpace(got)

			if tc.wantEqualName && trimmed != "test" {
				t.Errorf("got %q, want %q", got, "test")
			}
			if !tc.wantEqualName && trimmed == "test" {
				t.Error("expected capability tags to be appended")
			}
		})
	}
}

func TestVendorsPanel_RowText_EscapesName(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude[red]", true, false)))
	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewVendorsPanel(app)
	p.Refresh()
	list, _ := vendorsPanelParts(t, p)
	got, _ := list.GetItemText(0)
	want := tview.Escape("claude[red]")
	if !strings.Contains(got, want) {
		t.Errorf("got %q, want it to contain %q", got, want)
	}
}

// TestVendorsPanel_InfoPanel_CapabilityDisplay verifies the info panel
// renders yes or no for each capability.
func TestVendorsPanel_InfoPanel_CapabilityDisplay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		hasProjects     bool
		hasDeepResearch bool
	}{
		{"both capabilities true", true, true},
		{"both capabilities false", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddVendor(model.NewVendor("test", tc.hasProjects, tc.hasDeepResearch)))
			app := &mockApp{svc: newTestService(t, reg)}
			p := views.NewVendorsPanel(app)
			p.Refresh()
			_, info := vendorsPanelParts(t, p)
			lines := strings.Split(info.GetText(false), "\n")
			if len(lines) < 3 {
				t.Fatalf("expected at least 3 info lines, got %d: %q", len(lines), lines)
			}

			wantProjects := "no"
			if tc.hasProjects {
				wantProjects = "yes"
			}
			if !strings.Contains(lines[1], wantProjects) {
				t.Errorf("Projects line %q, want it to contain %q", lines[1], wantProjects)
			}

			wantResearch := "no"
			if tc.hasDeepResearch {
				wantResearch = "yes"
			}
			if !strings.Contains(lines[2], wantResearch) {
				t.Errorf("Deep research line %q, want it to contain %q", lines[2], wantResearch)
			}
		})
	}
}

func TestVendorsPanel_InfoPanel_AlignedDetailRows(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("test", true, true)))
	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewVendorsPanel(app)
	p.Refresh()
	_, info := vendorsPanelParts(t, p)

	lines := strings.Split(info.GetText(true), "\n")
	wantValues := []string{"test", "yes", "yes"}
	if len(lines) != len(wantValues) {
		t.Fatalf("got %d detail lines, want %d: %q", len(lines), len(wantValues), lines)
	}
	for i, line := range lines {
		if got := strings.Index(line, wantValues[i]); got != 14 {
			t.Errorf("line %d value starts at %d, want 14: %q", i, got, line)
		}
	}
}
