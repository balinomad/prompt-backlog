package panels_test

import (
	"strings"
	"testing"

	"github.com/balinomad/prompt-backlog/cmd/tui/views/panels"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// projectsPanelParts returns p's project list, granted-accounts list, and
// info view, reached via the exported Flex.GetItem. ProjectsPanel puts the
// body Flex at index 0 and the info view at index 1; the body holds the
// project list at index 0 and the account list at index 1.
func projectsPanelParts(t *testing.T, p *panels.ProjectsPanel) (*tview.List, *tview.List, *tview.TextView) {
	t.Helper()
	body, ok := p.GetItem(0).(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex at index 0, got %T", p.GetItem(0))
	}
	projectList, ok := body.GetItem(0).(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List at body index 0, got %T", body.GetItem(0))
	}
	rightColumn, ok := body.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex at body index 1, got %T", body.GetItem(1))
	}
	accountList, ok := rightColumn.GetItem(0).(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List at right column index 0, got %T", rightColumn.GetItem(0))
	}
	info, ok := p.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 1, got %T", p.GetItem(1))
	}
	return projectList, accountList, info
}

// projectsNoteList returns p's project notes list.
func projectsNoteList(t *testing.T, p *panels.ProjectsPanel) *tview.List {
	t.Helper()
	body, ok := p.GetItem(0).(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex at index 0, got %T", p.GetItem(0))
	}
	rightColumn, ok := body.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex at body index 1, got %T", body.GetItem(1))
	}
	noteList, ok := rightColumn.GetItem(1).(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List at right column index 1, got %T", rightColumn.GetItem(1))
	}
	return noteList
}

func TestNewProjectsPanel(t *testing.T) {
	t.Parallel()
	p := panels.NewProjectsPanel(newFakeApp(t))
	if p == nil {
		t.Fatal("expected non-nil ProjectsPanel")
	}
	projectsPanelParts(t, p)
	info, ok := p.GetItem(1).(interface{ GetTitle() string })
	if !ok {
		t.Fatal("detail panel does not expose a title")
	}
	if got := info.GetTitle(); got != " Project Details " {
		t.Errorf("GetTitle() = %q, want %q", got, " Project Details ")
	}
}

func TestProjectsPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*testing.T, *fakeApp)
		wantCount int
	}{
		{"three projects", func(_ *testing.T, _ *fakeApp) {}, 3},
		{"empty registry", func(t *testing.T, app *fakeApp) {
			t.Helper()
			app.Svc = newService(t, model.NewRegistry())
		}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			tc.setup(t, app)
			p := panels.NewProjectsPanel(app)
			p.Refresh()
			projectList, _, _ := projectsPanelParts(t, p)
			if got := projectList.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestProjectsPanel_Refresh_SortedByName(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)

	want := []string{"Alpha", "Beta", "Orphan"}
	if got := projectList.GetItemCount(); got != len(want) {
		t.Fatalf("item count: got %d, want %d", got, len(want))
	}
	for i, name := range want {
		text, _ := projectList.GetItemText(i)
		if !strings.HasPrefix(strings.TrimSpace(text), name) {
			t.Errorf("row %d = %q, want prefix %q", i, text, name)
		}
	}
}

func TestProjectsPanel_Refresh_PreservesProjectSelection(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)
	projectList.SetCurrentItem(1)
	want, _ := projectList.GetItemText(1)

	p.Refresh()

	got, _ := projectList.GetItemText(projectList.GetCurrentItem())
	if got != want {
		t.Errorf("selection changed: got %q, want %q", got, want)
	}
}

func TestProjectsPanel_AccountList_GrantedOnly(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, accountList, _ := projectsPanelParts(t, p)

	if got := accountList.GetItemCount(); got != 1 {
		t.Fatalf("Alpha account count: got %d, want 1", got)
	}
	text, _ := accountList.GetItemText(0)
	if !strings.Contains(text, "claude") || !strings.Contains(text, "Alice") {
		t.Errorf("Alpha account row = %q, want claude and Alice", text)
	}

	projectList.SetCurrentItem(2) // Orphan
	if got := accountList.GetItemCount(); got != 0 {
		t.Errorf("Orphan account count: got %d, want 0", got)
	}

	projectList.SetCurrentItem(1) // Beta
	if got := accountList.GetItemCount(); got != 1 {
		t.Errorf("Beta account count: got %d, want 1", got)
	}
}

func TestProjectsPanel_AccountList_Format(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	alice, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	bob, err := reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(alice, reg.Project("Alpha")))
	must(t, reg.AddProjectToAccount(bob, reg.Project("Alpha")))

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	_, accountList, _ := projectsPanelParts(t, p)

	want := []string{" claude │ Alice ", " claude │ Bob   "}
	if got := accountList.GetItemCount(); got != len(want) {
		t.Fatalf("account count: got %d, want %d", got, len(want))
	}
	for i, expected := range want {
		got, _ := accountList.GetItemText(i)
		if got != expected {
			t.Errorf("row %d = %q, want %q", i, got, expected)
		}
	}
}

func TestProjectsPanel_AccountList_OrderByVendorThenUser(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("qwen", true, false)))
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	for _, u := range []struct {
		email string
		name  string
	}{
		{"alice@example.com", "Alice"},
		{"zoe@example.com", "Zoe"},
	} {
		must(t, reg.AddUser(model.NewUser(u.email, u.name)))
	}
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	for _, pair := range []struct {
		email  string
		vendor string
	}{
		{"zoe@example.com", "qwen"},
		{"alice@example.com", "claude"},
		{"zoe@example.com", "claude"},
		{"alice@example.com", "qwen"},
	} {
		acc, err := reg.AddAccount(reg.User(pair.email), reg.Vendor(pair.vendor))
		must(t, err)
		must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
	}

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	_, accountList, _ := projectsPanelParts(t, p)

	want := []string{
		" claude │ Alice ",
		" claude │ Zoe   ",
		" qwen   │ Alice ",
		" qwen   │ Zoe   ",
	}
	if got := accountList.GetItemCount(); got != len(want) {
		t.Fatalf("account count: got %d, want %d", got, len(want))
	}
	for i, expected := range want {
		got, _ := accountList.GetItemText(i)
		if got != expected {
			t.Errorf("row %d = %q, want %q", i, got, expected)
		}
	}
}

func TestProjectsPanel_Refresh_PreservesAccountSelection(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	alice, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	bob, err := reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(alice, reg.Project("Alpha")))
	must(t, reg.AddProjectToAccount(bob, reg.Project("Alpha")))

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	_, accountList, _ := projectsPanelParts(t, p)
	accountList.SetCurrentItem(1)
	want, _ := accountList.GetItemText(1)

	p.Refresh()

	got, _ := accountList.GetItemText(accountList.GetCurrentItem())
	if got != want {
		t.Errorf("account selection changed: got %q, want %q", got, want)
	}
}

// TestProjectsPanel_Revoke_Cancel verifies Del opens a confirmation modal
// and that cancelling leaves the grant and account list untouched.
func TestProjectsPanel_Revoke_Cancel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	_, accountList, _ := projectsPanelParts(t, p)

	capture := accountList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil account input capture")
	}
	capture(specialKey(views.KeyDelete))

	confirmModal := confirmModalFrom(t, app.Modal)
	assertHints(t, app, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)

	confirmCapture := confirmModal.GetInputCapture()
	if confirmCapture == nil {
		t.Fatal("expected non-nil input capture")
	}
	confirmCapture(escKey())

	if app.Modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	assertHints(t, app, views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)

	projects, err := app.Svc.AccountProjects("alice@example.com", "claude")
	must(t, err)
	var found bool
	for _, proj := range projects {
		if proj.Name() == "Alpha" {
			found = true
		}
	}
	if !found {
		t.Error("Alpha should still be granted after cancel")
	}
	if got := accountList.GetItemCount(); got != 1 {
		t.Errorf("account count after cancel: got %d, want 1", got)
	}
}

func TestProjectsPanel_ProjectEsc_SwitchesToMain(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)

	projectList.GetInputCapture()(escKey())
	if app.SwitchedTo != "main" {
		t.Errorf("expected switchedTo=main, got %q", app.SwitchedTo)
	}
}

func TestProjectsPanel_AccountEsc_SwitchesToMain(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	_, accountList, _ := projectsPanelParts(t, p)

	accountList.GetInputCapture()(escKey())
	if app.SwitchedTo != "main" {
		t.Errorf("expected switchedTo=main, got %q", app.SwitchedTo)
	}
}

func TestProjectsPanel_Keys_NoHiddenAliases(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, accountList, _ := projectsPanelParts(t, p)

	for _, list := range []*tview.List{projectList, accountList} {
		event := runeKey(views.KeyProjects)
		if got := list.GetInputCapture()(event); got != event {
			t.Error("p must pass through unchanged")
		}
		if app.SwitchedTo != "" {
			t.Errorf("p must not switch screens, got %q", app.SwitchedTo)
		}
	}
}

func TestProjectsPanel_Keys_TabSwitchesFocus(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, accountList, _ := projectsPanelParts(t, p)
	noteList := projectsNoteList(t, p)

	projectCapture := projectList.GetInputCapture()
	accountCapture := accountList.GetInputCapture()
	noteCapture := noteList.GetInputCapture()
	if projectCapture == nil || accountCapture == nil || noteCapture == nil {
		t.Fatal("expected non-nil input captures")
	}

	projectCapture(specialKey(views.KeyFocusNext))
	if app.Focused != accountList {
		t.Errorf("project Tab focused %T, want account list", app.Focused)
	}
	accountCapture(specialKey(views.KeyFocusNext))
	if app.Focused != noteList {
		t.Errorf("account Tab focused %T, want note list", app.Focused)
	}
	noteCapture(specialKey(views.KeyFocusNext))
	if app.Focused != projectList {
		t.Errorf("note Tab focused %T, want project list", app.Focused)
	}

	projectCapture(specialKey(views.KeyFocusPrev))
	if app.Focused != noteList {
		t.Errorf("project Backtab focused %T, want note list", app.Focused)
	}
	noteCapture(specialKey(views.KeyFocusPrev))
	if app.Focused != accountList {
		t.Errorf("note Backtab focused %T, want account list", app.Focused)
	}
	accountCapture(specialKey(views.KeyFocusPrev))
	if app.Focused != projectList {
		t.Errorf("account Backtab focused %T, want project list", app.Focused)
	}
}

func TestProjectsPanel_ProjectKeys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		key          *tcell.EventKey
		wantSpecific string
		wantCommon   string
	}{
		{"new", specialKey(views.KeyInsert), views.HintsFormSpecific, views.HintsTextFormCommon},
		{"edit", specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsTextFormCommon},
		{"delete", specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
		{"help", runeKey(views.KeyHelp), views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon},
		{"quit", runeKey(views.KeyQuit), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			p := panels.NewProjectsPanel(app)
			p.Refresh()
			projectList, _, _ := projectsPanelParts(t, p)
			capture := projectList.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.Modal == nil {
				t.Errorf("key %q: expected modal", tc.name)
			}
			assertHints(t, app, tc.wantSpecific, tc.wantCommon)
		})
	}
}

func TestProjectsPanel_AccountKeys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		key          *tcell.EventKey
		wantSpecific string
		wantCommon   string
	}{
		{"grant", specialKey(views.KeyInsert), views.HintsFormSpecific, views.HintsFormCommon},
		{"revoke", specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
		{"help", runeKey(views.KeyHelp), views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon},
		{"quit", runeKey(views.KeyQuit), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			p := panels.NewProjectsPanel(app)
			p.Refresh()
			_, accountList, _ := projectsPanelParts(t, p)
			capture := accountList.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.Modal == nil {
				t.Errorf("key %q: expected modal", tc.name)
			}
			assertHints(t, app, tc.wantSpecific, tc.wantCommon)
		})
	}
}

func TestProjectsPanel_EmptyList_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		focus string
		key   *tcell.EventKey
	}{
		{"project edit", "project", specialKey(views.KeyEdit)},
		{"project delete", "project", specialKey(views.KeyDelete)},
		{"account revoke", "account", specialKey(views.KeyDelete)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			app.Svc = newService(t, model.NewRegistry())
			p := panels.NewProjectsPanel(app)
			p.Refresh()
			projectList, accountList, _ := projectsPanelParts(t, p)

			capture := projectList.GetInputCapture()
			if tc.focus == "account" {
				capture = accountList.GetInputCapture()
			}
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.Modal != nil {
				t.Errorf("key %q with empty list should not open a modal", tc.name)
			}
		})
	}
}

// TestProjectsPanel_New_SaveReturnsToPanel closes the loop that
// TestProjectsPanel_ProjectKeys_ShowModal leaves open: that test only checks
// that pressing Ins opens a modal with the right hints, not that completing
// the form hands control back to the panel correctly.
func TestProjectsPanel_New_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)

	capture := projectList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(specialKey(views.KeyInsert))

	form := formFromCenteredModal(t, app.Modal)
	setInputField(t, form, "Name", "Gamma")
	triggerSave(t, form)

	if app.Modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
	if app.Svc.Project("Gamma") == nil {
		t.Error("project not created")
	}
}

// TestProjectsPanel_Edit_SaveReturnsToPanel is the same round trip as
// TestProjectsPanel_New_SaveReturnsToPanel for handleEdit's own onClose
// closure.
func TestProjectsPanel_Edit_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh() // selects "Alpha" first (sorted by name)
	projectList, _, _ := projectsPanelParts(t, p)

	capture := projectList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(specialKey(views.KeyEdit))

	form := formFromCenteredModal(t, app.Modal)
	setTextArea(t, form, "Description", "new desc")
	triggerSave(t, form)

	if app.Modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
	if got := app.Svc.Project("Alpha").Description(); got != "new desc" {
		t.Errorf("Description() = %q, want %q", got, "new desc")
	}
}

// TestProjectsPanel_Delete_Cancel is the projects.go counterpart to
// TestDetailPanel_DeleteSession_Cancel: confirming the delete is
// button-click-gated and unreachable black box, so this covers the
// reachable half.
func TestProjectsPanel_Delete_Cancel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh() // selects "Alpha" first; Alpha is granted, cancelling must leave it untouched
	projectList, _, _ := projectsPanelParts(t, p)

	before := len(app.Svc.Projects())

	capture := projectList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(specialKey(views.KeyDelete))

	confirmModal := confirmModalFrom(t, app.Modal)
	confirmCapture := confirmModal.GetInputCapture()
	if confirmCapture == nil {
		t.Fatal("expected non-nil input capture")
	}
	confirmCapture(escKey())

	if app.Modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	assertHints(t, app, views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
	if got := len(app.Svc.Projects()); got != before {
		t.Errorf("project count changed after cancel: got %d, want %d (unchanged)", got, before)
	}
}

func TestProjectsPanel_GrantFromAccounts_SaveReturnsToAccountHints(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	alice, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	_, err = reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(alice, reg.Project("Alpha")))

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	_, accountList, _ := projectsPanelParts(t, p)

	capture := accountList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil account input capture")
	}
	capture(specialKey(views.KeyInsert))

	form := formFromCenteredModal(t, app.Modal)
	setDropDown(t, form, "Account", 0) // only eligible candidate: bob|claude
	triggerSave(t, form)

	assertHints(t, app, views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)
	projects, err := app.Svc.AccountProjects("bob@example.com", "claude")
	must(t, err)
	var found bool
	for _, proj := range projects {
		if proj.Name() == "Alpha" {
			found = true
		}
	}
	if !found {
		t.Error("Alpha not granted to bob|claude")
	}
}

func TestProjectsPanel_FocusFuncs_SetHints(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	projectList, accountList, _ := projectsPanelParts(t, p)

	projectList.Focus(func(tview.Primitive) {})
	assertHints(t, app, views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)

	accountList.Focus(func(tview.Primitive) {})
	assertHints(t, app, views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)
}

// TestProjectsPanel_RowText verifies a project row is its name with the shared
// one-cell inset, including when the project has no granted accounts.
func TestProjectsPanel_RowText(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)
	got, _ := projectList.GetItemText(0)
	if want := " Alpha"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProjectsPanel_InfoPanel_AlignedDetailRows(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProject(model.NewProject("Alpha", "Backend rewrite", "Use Go interfaces")))

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	_, _, info := projectsPanelParts(t, p)

	lines := strings.Split(info.GetText(true), "\n")
	wantValues := []string{"Alpha", "Backend rewrite", "Use Go interfaces"}
	if len(lines) != len(wantValues) {
		t.Fatalf("got %d detail lines, want %d: %q", len(lines), len(wantValues), lines)
	}
	for i, line := range lines {
		if got := strings.Index(line, wantValues[i]); got != 13 {
			t.Errorf("line %d value starts at %d, want 13: %q", i, got, line)
		}
	}
}

func TestProjectsPanel_RowText_EscapesName(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha[red]", "", "")))
	must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha[red]")))

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)
	got, _ := projectList.GetItemText(0)
	want := tview.Escape("Alpha[red]")
	if !strings.Contains(got, want) {
		t.Errorf("got %q, want it to contain %q", got, want)
	}
}

func TestProjectsPanel_ProjectNotes(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	n1, err := app.Svc.AddNote("alpha one", "Alpha", "alice@example.com", "claude")
	must(t, err)
	n2, err := app.Svc.AddNote("alpha two", "Alpha", "", "")
	must(t, err)
	_, err = app.Svc.AddNote("beta note", "Beta", "alice@example.com", "claude")
	must(t, err)

	p := panels.NewProjectsPanel(app)
	p.Refresh()
	noteList := projectsNoteList(t, p)

	if got := noteList.GetItemCount(); got != 2 {
		t.Fatalf("project notes = %d, want 2", got)
	}
	row0, _ := noteList.GetItemText(0)
	if !strings.Contains(row0, "claude") || !strings.Contains(row0, "Alice") || !strings.Contains(row0, "alpha one") {
		t.Errorf("row 0 = %q", row0)
	}
	row1, _ := noteList.GetItemText(1)
	if !strings.Contains(row1, "alpha two") {
		t.Errorf("row 1 = %q", row1)
	}
	if got := noteList.GetTitle(); !strings.Contains(got, "Project Notes") || !strings.Contains(got, "1/2") {
		t.Errorf("title = %q, want Project Notes 1/2", got)
	}
	if noteList.GetCurrentItem() != 0 {
		t.Errorf("selection = %d, want 0", noteList.GetCurrentItem())
	}
	if pNote := p.SelectedNote(); pNote == nil || pNote.ID() != n1.ID() {
		t.Errorf("SelectedNote() = %v, want note %d", pNote, n1.ID())
	}
	_ = n2
}

func TestProjectsPanel_ProjectNotes_FollowProjectSelection(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	_, err := app.Svc.AddNote("alpha note", "Alpha", "alice@example.com", "claude")
	must(t, err)
	_, err = app.Svc.AddNote("beta note", "Beta", "alice@example.com", "claude")
	must(t, err)

	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)
	noteList := projectsNoteList(t, p)

	projectList.SetCurrentItem(1) // Beta
	if got := noteList.GetItemCount(); got != 1 {
		t.Fatalf("Beta notes = %d, want 1", got)
	}
	row, _ := noteList.GetItemText(0)
	if !strings.Contains(row, "beta note") {
		t.Errorf("Beta row = %q", row)
	}

	projectList.SetCurrentItem(2) // Orphan
	if got := noteList.GetItemCount(); got != 0 {
		t.Errorf("Orphan notes = %d, want 0", got)
	}
}

func TestProjectsPanel_ProjectNotes_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	_, err := app.Svc.AddNote("one", "Alpha", "", "")
	must(t, err)
	n2, err := app.Svc.AddNote("two", "Alpha", "", "")
	must(t, err)

	p := panels.NewProjectsPanel(app)
	p.Refresh()
	noteList := projectsNoteList(t, p)
	noteList.SetCurrentItem(1)
	p.Refresh()

	if got := p.SelectedNote(); got == nil || got.ID() != n2.ID() {
		t.Errorf("SelectedNote() = %v, want note %d", got, n2.ID())
	}
}

func TestProjectsPanel_ProjectNotes_Keys(t *testing.T) {
	t.Parallel()

	t.Run("Ins opens the add form with the project fixed", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		p := panels.NewProjectsPanel(app)
		p.Refresh()
		noteList := projectsNoteList(t, p)
		noteList.GetInputCapture()(specialKey(views.KeyInsert))
		if got := centeredModalTitle(t, app.Modal); got != " New Note " {
			t.Errorf("title = %q, want New Note", got)
		}
		project := findDropDownByLabel(centeredModalContent(t, app.Modal), "Project")
		if project == nil {
			t.Fatal("Project dropdown not found")
		}
		if got := dropdownOption(t, project, 0); got != "Alpha" {
			t.Errorf("project option = %q, want Alpha", got)
		}
	})

	t.Run("Enter opens the edit form", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		_, err := app.Svc.AddNote("note", "Alpha", "alice@example.com", "claude")
		must(t, err)
		p := panels.NewProjectsPanel(app)
		p.Refresh()
		noteList := projectsNoteList(t, p)
		noteList.SetCurrentItem(0)
		noteList.GetInputCapture()(specialKey(views.KeyEdit))
		if got := centeredModalTitle(t, app.Modal); got != " Edit Note " {
			t.Errorf("title = %q, want Edit Note", got)
		}
	})

	t.Run("Del confirms and deletes", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		note, err := app.Svc.AddNote("note", "Alpha", "alice@example.com", "claude")
		must(t, err)
		p := panels.NewProjectsPanel(app)
		p.Refresh()
		noteList := projectsNoteList(t, p)
		noteList.SetCurrentItem(0)
		noteList.GetInputCapture()(specialKey(views.KeyDelete))
		confirmModal(t, app)
		p.Refresh()
		if app.Svc.Note(note.ID()) != nil {
			t.Error("note still exists after confirming delete")
		}
	})
}

func TestProjectsPanel_Refresh_PreservesSelectionByIDAfterRename(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)

	projectList.SetCurrentItem(1) // Beta
	must(t, func() error {
		_, err := app.Svc.RenameProject("Beta", "Aardvark")
		return err
	}())
	p.Refresh()

	if got := projectList.GetCurrentItem(); got != 0 {
		t.Errorf("selection = %d, want 0 after the renamed project sorted first", got)
	}
	row, _ := projectList.GetItemText(0)
	if !strings.Contains(row, "Aardvark") {
		t.Errorf("selected row = %q, want the renamed project", row)
	}
}

func TestProjectsPanel_DoubleClickProjectOpensEditForm(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)

	doubleClickList(t, projectList)

	form := formFromCenteredModal(t, app.Modal)
	if got := form.GetTitle(); !strings.Contains(got, "Edit Project") {
		t.Errorf("title = %q, want it to contain Edit Project", got)
	}
}

func TestProjectsPanel_DoubleClickNoteOpensEditForm(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	_, err := app.Svc.AddNote("alpha note", "Alpha", "alice@example.com", "claude")
	must(t, err)

	p := panels.NewProjectsPanel(app)
	p.Refresh()
	noteList := projectsNoteList(t, p)
	if noteList.GetItemCount() == 0 {
		t.Fatal("expected at least one project note")
	}
	noteList.SetCurrentItem(0)

	doubleClickList(t, noteList)

	if got := centeredModalTitle(t, app.Modal); got != " Edit Note " {
		t.Errorf("title = %q, want Edit Note", got)
	}
}

func TestListTitles_ProjectsPanel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewProjectsPanel(app)
	p.Refresh()
	projectList, accountList, _ := projectsPanelParts(t, p)

	if got, want := projectList.GetTitle(), " Projects 1/3 "; got != want {
		t.Errorf("initial project title = %q, want %q", got, want)
	}
	if got, want := accountList.GetTitle(), " Project Accounts 1/1 "; got != want {
		t.Errorf("initial account title = %q, want %q", got, want)
	}

	projectList.SetCurrentItem(2)
	if got, want := projectList.GetTitle(), " Projects 3/3 "; got != want {
		t.Errorf("selected project title = %q, want %q", got, want)
	}
	if got, want := accountList.GetTitle(), " Project Accounts 0/0 "; got != want {
		t.Errorf("empty account title = %q, want %q", got, want)
	}
}
