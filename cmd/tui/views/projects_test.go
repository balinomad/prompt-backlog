package views_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// projectsPanelParts returns p's project list and info view, reached via
// the exported Flex.GetItem — ProjectsPanel puts the list at index 0, the
// info view at index 1 (see projects.go).
func projectsPanelParts(t *testing.T, p *views.ProjectsPanel) (*tview.List, *tview.TextView) {
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

func TestNewProjectsPanel(t *testing.T) {
	t.Parallel()
	if p := views.NewProjectsPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil ProjectsPanel")
	}
}

func TestProjectsPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*testing.T, *mockApp)
		wantCount int
	}{
		{"three projects", func(_ *testing.T, _ *mockApp) {}, 3},
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
			p := views.NewProjectsPanel(app)
			p.Refresh()
			list, _ := projectsPanelParts(t, p)
			if got := list.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestProjectsPanel_Refresh_SortedByName(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	list, _ := projectsPanelParts(t, p)

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

func TestProjectsPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	list, _ := projectsPanelParts(t, p)
	list.SetCurrentItem(1)
	want, _ := list.GetItemText(1)

	p.Refresh()

	got, _ := list.GetItemText(list.GetCurrentItem())
	if got != want {
		t.Errorf("selection changed: got %q, want %q", got, want)
	}
}

func TestProjectsPanel_Keys_SwitchToMain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		event *tcell.EventKey
	}{
		{"esc key", escKey()},
		{"p key", runeKey(views.KeyProjects)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := views.NewProjectsPanel(app)
			p.Refresh()
			list, _ := projectsPanelParts(t, p)
			list.GetInputCapture()(tc.event)
			if app.switchedTo != "main" {
				t.Errorf("expected switchedTo=main, got %q", app.switchedTo)
			}
		})
	}
}

func TestProjectsPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       rune
		wantHints string
	}{
		{"new", views.KeyNew, views.HintsForm},
		{"edit", views.KeyEdit, views.HintsForm},
		{"delete", views.KeyDelete, views.HintsConfirmModal},
		{"grant", views.KeyGrant, views.HintsForm},
		{"revoke", views.KeyRevoke, views.HintsForm},
		{"help", views.KeyHelp, views.HintsHelpOverlay},
		{"quit", views.KeyQuit, views.HintsConfirmModal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := views.NewProjectsPanel(app)
			p.Refresh()
			list, _ := projectsPanelParts(t, p)
			capture := list.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(runeKey(tc.key))
			if app.modal == nil {
				t.Errorf("key %q: expected modal", string(tc.key))
			}
			if app.hints != tc.wantHints {
				t.Errorf("key %q: hints got %q, want %q", string(tc.key), app.hints, tc.wantHints)
			}
		})
	}
}

func TestProjectsPanel_EmptyList_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  rune
	}{
		{"edit", views.KeyEdit},
		{"delete", views.KeyDelete},
		{"grant", views.KeyGrant},
		{"revoke", views.KeyRevoke},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			app.svc = newTestService(t, model.NewRegistry())
			p := views.NewProjectsPanel(app)
			p.Refresh()
			list, _ := projectsPanelParts(t, p)
			list.GetInputCapture()(runeKey(tc.key))
			if app.modal != nil {
				t.Errorf("key %q with empty list should not open a modal", string(tc.key))
			}
		})
	}
}

// TestProjectsPanel_New_SaveReturnsToPanel closes the loop that
// TestProjectsPanel_Keys_ShowModal leaves open: that test only checks
// that pressing 'n' opens a modal with the right hints, not that
// completing the form hands control back to the panel correctly.
func TestProjectsPanel_New_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	list, _ := projectsPanelParts(t, p)

	capture := list.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(runeKey(views.KeyNew))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Name", "Gamma")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hints != views.HintsProjectsPanel {
		t.Errorf("hints after save: got %q, want %q", app.hints, views.HintsProjectsPanel)
	}
	if app.svc.Project("Gamma") == nil {
		t.Error("project not created")
	}
}

// TestProjectsPanel_Edit_SaveReturnsToPanel is the same round trip as
// TestProjectsPanel_New_SaveReturnsToPanel for handleEdit's own onClose
// closure.
func TestProjectsPanel_Edit_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh() // selects "Alpha" first (sorted by name)
	list, _ := projectsPanelParts(t, p)

	capture := list.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(runeKey(views.KeyEdit))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Description", "new desc")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hints != views.HintsProjectsPanel {
		t.Errorf("hints after save: got %q, want %q", app.hints, views.HintsProjectsPanel)
	}
	if got := app.svc.Project("Alpha").Description(); got != "new desc" {
		t.Errorf("Description() = %q, want %q", got, "new desc")
	}
}

// TestProjectsPanel_Delete_Cancel is the projects.go counterpart to
// TestDetailPanel_DeleteSession_Cancel: confirming the delete is
// button-click-gated and unreachable black box, so this covers the
// reachable half.
func TestProjectsPanel_Delete_Cancel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh() // selects "Alpha" first; Alpha is granted, cancelling must leave it untouched
	list, _ := projectsPanelParts(t, p)

	before := len(app.svc.Projects())

	capture := list.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(runeKey(views.KeyDelete))

	confirmModal, ok := app.modal.(*tview.Modal)
	if !ok {
		t.Fatalf("expected *tview.Modal, got %T", app.modal)
	}
	confirmCapture := confirmModal.GetInputCapture()
	if confirmCapture == nil {
		t.Fatal("expected non-nil input capture")
	}
	confirmCapture(escKey())

	if app.modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	if app.hints != views.HintsProjectsPanel {
		t.Errorf("hints after cancel: got %q, want %q", app.hints, views.HintsProjectsPanel)
	}
	if got := len(app.svc.Projects()); got != before {
		t.Errorf("project count changed after cancel: got %d, want %d (unchanged)", got, before)
	}
}

// TestProjectsPanel_Grant_SaveReturnsToPanel closes the loop for
// handleGrant's onClose closure, granting Orphan (unattached in the
// fixture) to alice|claude.
func TestProjectsPanel_Grant_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	list, _ := projectsPanelParts(t, p)
	list.SetCurrentItem(2) // "Orphan": sorted order is Alpha, Beta, Orphan

	capture := list.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(runeKey(views.KeyGrant))

	form := formFromCenteredModal(t, app.modal)
	setDropDown(t, form, "Account", 0) // 0 = alice|claude, 1 = bob|chatgpt (sorted by key); chatgpt has HasProjects=false
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hints != views.HintsProjectsPanel {
		t.Errorf("hints after save: got %q, want %q", app.hints, views.HintsProjectsPanel)
	}
	projects, err := app.svc.AccountProjects("alice@example.com", "claude")
	must(t, err)
	var found bool
	for _, proj := range projects {
		if proj.Name() == "Orphan" {
			found = true
		}
	}
	if !found {
		t.Error("Orphan not granted to alice|claude")
	}
}

// TestProjectsPanel_Revoke_SaveReturnsToPanel closes the loop for
// handleRevoke's onClose closure, revoking Alpha from alice|claude.
func TestProjectsPanel_Revoke_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh() // selects "Alpha" first
	list, _ := projectsPanelParts(t, p)

	capture := list.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(runeKey(views.KeyRevoke))

	form := formFromCenteredModal(t, app.modal)
	setDropDown(t, form, "Account", 0) // only alice|claude has Alpha
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hints != views.HintsProjectsPanel {
		t.Errorf("hints after save: got %q, want %q", app.hints, views.HintsProjectsPanel)
	}
	projects, err := app.svc.AccountProjects("alice@example.com", "claude")
	must(t, err)
	for _, proj := range projects {
		if proj.Name() == "Alpha" {
			t.Error("Alpha should have been revoked")
		}
	}
}

// TestProjectItemText replaces the old direct calls to the unexported
// projectItemText: grants the project to accountCount real accounts
// through a registry, refreshes, and reads the rendered row back via the
// list's exported GetItemText.
func TestProjectItemText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		accountCount int
		wantContains string
		wantEqual    string // exact match expected when non-empty
	}{
		{"zero accounts flagged unattached", 0, "unattached", ""},
		{"one account returns plain name", 1, "", "Alpha"},
		{"multiple accounts returns plain name", 3, "", "Alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
			must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
			for i := range tc.accountCount {
				email := fmt.Sprintf("user%d@example.com", i)
				must(t, reg.AddUser(model.NewUser(email, "User")))
				acc, err := reg.AddAccount(reg.User(email), reg.Provider("claude"))
				must(t, err)
				must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
			}

			app := &mockApp{svc: newTestService(t, reg)}
			p := views.NewProjectsPanel(app)
			p.Refresh()
			list, _ := projectsPanelParts(t, p)
			got, _ := list.GetItemText(0)

			if tc.wantContains != "" && !strings.Contains(got, tc.wantContains) {
				t.Errorf("got %q, want it to contain %q", got, tc.wantContains)
			}
			if tc.wantEqual != "" && got != tc.wantEqual {
				t.Errorf("got %q, want %q", got, tc.wantEqual)
			}
		})
	}
}

func TestProjectItemText_EscapesName(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha[red]", "", "")))
	must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha[red]")))

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewProjectsPanel(app)
	p.Refresh()
	list, _ := projectsPanelParts(t, p)
	got, _ := list.GetItemText(0)
	want := tview.Escape("Alpha[red]")
	if !strings.Contains(got, want) {
		t.Errorf("got %q, want it to contain %q", got, want)
	}
}
