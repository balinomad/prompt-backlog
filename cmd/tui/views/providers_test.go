package views_test

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// providersPanelParts returns p's provider list and info view, reached
// via the exported Flex.GetItem — ProvidersPanel puts the list at index
// 0, the info view at index 1 (see providers.go).
func providersPanelParts(t *testing.T, p *views.ProvidersPanel) (*tview.List, *tview.TextView) {
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

func TestNewProvidersPanel(t *testing.T) {
	t.Parallel()
	if p := views.NewProvidersPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil ProvidersPanel")
	}
}

func TestProvidersPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*testing.T, *mockApp)
		wantCount int
	}{
		{"two providers", func(_ *testing.T, _ *mockApp) {}, 2},
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
			p := views.NewProvidersPanel(app)
			p.Refresh()
			list, _ := providersPanelParts(t, p)
			if got := list.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestProvidersPanel_Refresh_SortedByName(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProvidersPanel(app)
	p.Refresh()
	list, _ := providersPanelParts(t, p)

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

func TestProvidersPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProvidersPanel(app)
	p.Refresh()
	list, _ := providersPanelParts(t, p)
	list.SetCurrentItem(1)
	want, _ := list.GetItemText(1)

	p.Refresh()

	got, _ := list.GetItemText(list.GetCurrentItem())
	if got != want {
		t.Errorf("selection changed: got %q, want %q", got, want)
	}
}

func TestProvidersPanel_Keys_SwitchToMain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		event *tcell.EventKey
	}{
		{"esc key", escKey()},
		{"r key", runeKey(views.KeyProviders)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := views.NewProvidersPanel(app)
			p.Refresh()
			list, _ := providersPanelParts(t, p)
			list.GetInputCapture()(tc.event)
			if app.switchedTo != "main" {
				t.Errorf("expected switchedTo=main, got %q", app.switchedTo)
			}
		})
	}
}

func TestProvidersPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		key          *tcell.EventKey
		wantSpecific string
		wantCommon   string
	}{
		{"new", runeKey(views.KeyNew), views.HintsFormSpecific, views.HintsFormCommon},
		{"edit", specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsFormCommon},
		{"delete", specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
		{"help", runeKey(views.KeyHelp), views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon},
		{"quit", runeKey(views.KeyQuit), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := views.NewProvidersPanel(app)
			p.Refresh()
			list, _ := providersPanelParts(t, p)
			capture := list.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.modal == nil {
				t.Errorf("key %q: expected modal", tc.name)
			}
			if app.hintsSpecific != tc.wantSpecific || app.hintsCommon != tc.wantCommon {
				t.Errorf("key %q: hints got (%q, %q), want (%q, %q)",
					tc.name, app.hintsSpecific, app.hintsCommon, tc.wantSpecific, tc.wantCommon)
			}
		})
	}
}

func TestProvidersPanel_EmptyList_NoModal(t *testing.T) {
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
			p := views.NewProvidersPanel(app)
			p.Refresh()
			list, _ := providersPanelParts(t, p)
			list.GetInputCapture()(tc.key)
			if app.modal != nil {
				t.Errorf("key %q with empty list should not open a modal", tc.name)
			}
		})
	}
}

// TestProvidersPanel_New_SaveReturnsToPanel closes the loop that
// TestProvidersPanel_Keys_ShowModal leaves open: that test only checks
// that pressing 'n' opens a modal with the right hints, not that
// completing the form hands control back to the panel correctly.
func TestProvidersPanel_New_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProvidersPanel(app)
	p.Refresh()
	list, _ := providersPanelParts(t, p)

	list.GetInputCapture()(runeKey(views.KeyNew))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Name", "gemini")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hintsSpecific != views.HintsProvidersPanelSpecific || app.hintsCommon != views.HintsProvidersPanelCommon {
		t.Errorf("hints after save: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProvidersPanelSpecific, views.HintsProvidersPanelCommon)
	}
	if app.svc.Provider("gemini") == nil {
		t.Error("provider not created")
	}
}

// TestProvidersPanel_Edit_SaveReturnsToPanel is the same round trip as
// TestProvidersPanel_New_SaveReturnsToPanel for handleEdit's own onClose
// closure.
func TestProvidersPanel_Edit_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProvidersPanel(app)
	p.Refresh() // selects "chatgpt" first (sorted by name)
	list, _ := providersPanelParts(t, p)

	list.GetInputCapture()(specialKey(views.KeyEdit))

	form := formFromCenteredModal(t, app.modal)
	setCheckbox(t, form, "Supports projects")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hintsSpecific != views.HintsProvidersPanelSpecific || app.hintsCommon != views.HintsProvidersPanelCommon {
		t.Errorf("hints after save: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProvidersPanelSpecific, views.HintsProvidersPanelCommon)
	}
	if got := app.svc.Provider("chatgpt"); !got.HasProjects() {
		t.Error("HasProjects() = false, want true")
	}
}

// TestProvidersPanel_Delete_Cancel is the providers.go counterpart to
// TestDetailPanel_DeleteSession_Cancel and TestAccountsPanel_Delete_Cancel:
// confirming the delete is button-click-gated and unreachable black box,
// so this covers the reachable half.
func TestProvidersPanel_Delete_Cancel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProvidersPanel(app)
	p.Refresh()
	list, _ := providersPanelParts(t, p)

	before := len(app.svc.Providers())

	list.GetInputCapture()(specialKey(views.KeyDelete))

	confirmModal, ok := app.modal.(*tview.Modal)
	if !ok {
		t.Fatalf("expected *tview.Modal, got %T", app.modal)
	}
	confirmModal.GetInputCapture()(escKey())

	if app.modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	if app.hintsSpecific != views.HintsProvidersPanelSpecific || app.hintsCommon != views.HintsProvidersPanelCommon {
		t.Errorf("hints after cancel: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProvidersPanelSpecific, views.HintsProvidersPanelCommon)
	}
	if got := len(app.svc.Providers()); got != before {
		t.Errorf("provider count changed after cancel: got %d, want %d (unchanged)", got, before)
	}
}

// TestProviderItemText replaces the old direct calls to the unexported
// providerItemText: seeds a single provider through a real registry,
// refreshes, and reads the rendered row back via the list's exported
// GetItemText. Same wantEqualName shape as the original — a provider
// with no capabilities renders as exactly its name; any capability
// appends a tag, so the row no longer equals the bare name.
func TestProviderItemText(t *testing.T) {
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
			must(t, reg.AddProvider(model.NewProvider("test", tc.hasProjects, tc.hasDeepResearch)))
			app := &mockApp{svc: newTestService(t, reg)}
			p := views.NewProvidersPanel(app)
			p.Refresh()
			list, _ := providersPanelParts(t, p)
			got, _ := list.GetItemText(0)

			if tc.wantEqualName && got != "test" {
				t.Errorf("got %q, want %q", got, "test")
			}
			if !tc.wantEqualName && got == "test" {
				t.Error("expected capability tags to be appended")
			}
		})
	}
}

func TestProviderItemText_EscapesName(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude[red]", true, false)))
	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewProvidersPanel(app)
	p.Refresh()
	list, _ := providersPanelParts(t, p)
	got, _ := list.GetItemText(0)
	want := tview.Escape("claude[red]")
	if !strings.Contains(got, want) {
		t.Errorf("got %q, want it to contain %q", got, want)
	}
}

// TestProvidersPanel_InfoPanel_CapabilityDisplay replaces TestBoolTag,
// which called the unexported boolTag directly and only checked for a
// non-empty result. This checks the actual rendered Projects/Deep
// research lines in the info panel show "yes" or "no" per the
// provider's capabilities — a more precise check than the original's
// non-emptiness assertion, and the only kind reachable now that boolTag
// itself isn't callable directly.
func TestProvidersPanel_InfoPanel_CapabilityDisplay(t *testing.T) {
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
			must(t, reg.AddProvider(model.NewProvider("test", tc.hasProjects, tc.hasDeepResearch)))
			app := &mockApp{svc: newTestService(t, reg)}
			p := views.NewProvidersPanel(app)
			p.Refresh()
			_, info := providersPanelParts(t, p)
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
