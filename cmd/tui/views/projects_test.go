package views_test

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// projectsPanelParts returns p's project list, granted-accounts list, and
// info view, reached via the exported Flex.GetItem. ProjectsPanel puts the
// body Flex at index 0 and the info view at index 1; the body holds the
// project list at index 0 and the account list at index 1.
func projectsPanelParts(t *testing.T, p *views.ProjectsPanel) (*tview.List, *tview.List, *tview.TextView) {
	t.Helper()
	body, ok := p.GetItem(0).(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex at index 0, got %T", p.GetItem(0))
	}
	projectList, ok := body.GetItem(0).(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List at body index 0, got %T", body.GetItem(0))
	}
	accountList, ok := body.GetItem(1).(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List at body index 1, got %T", body.GetItem(1))
	}
	info, ok := p.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 1, got %T", p.GetItem(1))
	}
	return projectList, accountList, info
}

func TestNewProjectsPanel(t *testing.T) {
	t.Parallel()
	p := views.NewProjectsPanel(newMockApp(t))
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
			projectList, _, _ := projectsPanelParts(t, p)
			if got := projectList.GetItemCount(); got != tc.wantCount {
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
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
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
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
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
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	alice, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	bob, err := reg.AddAccount(reg.User("bob@example.com"), reg.Provider("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(alice, reg.Project("Alpha")))
	must(t, reg.AddProjectToAccount(bob, reg.Project("Alpha")))

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewProjectsPanel(app)
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

func TestProjectsPanel_AccountList_OrderByProviderThenUser(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("qwen", true, false)))
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
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
		email    string
		provider string
	}{
		{"zoe@example.com", "qwen"},
		{"alice@example.com", "claude"},
		{"zoe@example.com", "claude"},
		{"alice@example.com", "qwen"},
	} {
		acc, err := reg.AddAccount(reg.User(pair.email), reg.Provider(pair.provider))
		must(t, err)
		must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
	}

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewProjectsPanel(app)
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
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	alice, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	bob, err := reg.AddAccount(reg.User("bob@example.com"), reg.Provider("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(alice, reg.Project("Alpha")))
	must(t, reg.AddProjectToAccount(bob, reg.Project("Alpha")))

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewProjectsPanel(app)
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
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	_, accountList, _ := projectsPanelParts(t, p)

	capture := accountList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil account input capture")
	}
	capture(specialKey(views.KeyDelete))

	confirmModal, ok := app.modal.(*tview.Modal)
	if !ok {
		t.Fatalf("expected *tview.Modal, got %T", app.modal)
	}
	if app.hintsSpecific != views.HintsConfirmModalSpecific || app.hintsCommon != views.HintsConfirmModalCommon {
		t.Errorf("hints after Del: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	}

	confirmCapture := confirmModal.GetInputCapture()
	if confirmCapture == nil {
		t.Fatal("expected non-nil input capture")
	}
	confirmCapture(escKey())

	if app.modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	if app.hintsSpecific != views.HintsProjectAccountsSpecific || app.hintsCommon != views.HintsProjectAccountsCommon {
		t.Errorf("hints after cancel: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)
	}

	projects, err := app.svc.AccountProjects("alice@example.com", "claude")
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
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)

	projectList.GetInputCapture()(escKey())
	if app.switchedTo != "main" {
		t.Errorf("expected switchedTo=main, got %q", app.switchedTo)
	}
}

func TestProjectsPanel_AccountEsc_SwitchesToProjectList(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	projectList, accountList, _ := projectsPanelParts(t, p)

	accountList.GetInputCapture()(escKey())
	if app.switchedTo != "" {
		t.Errorf("expected no screen switch, got %q", app.switchedTo)
	}
	if app.focused != projectList {
		t.Errorf("expected focus on project list, got %T", app.focused)
	}
}

func TestProjectsPanel_Keys_NoHiddenAliases(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	projectList, accountList, _ := projectsPanelParts(t, p)

	for _, list := range []*tview.List{projectList, accountList} {
		event := runeKey(views.KeyProjects)
		if got := list.GetInputCapture()(event); got != event {
			t.Error("p must pass through unchanged")
		}
		if app.switchedTo != "" {
			t.Errorf("p must not switch screens, got %q", app.switchedTo)
		}
	}
}

func TestProjectsPanel_Keys_TabSwitchesFocus(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	projectList, accountList, _ := projectsPanelParts(t, p)

	projectCapture := projectList.GetInputCapture()
	accountCapture := accountList.GetInputCapture()
	if projectCapture == nil || accountCapture == nil {
		t.Fatal("expected non-nil input captures")
	}

	projectCapture(specialKey(views.KeyFocusNext))
	if app.focused != accountList {
		t.Errorf("project Tab focused %T, want account list", app.focused)
	}

	accountCapture(specialKey(views.KeyFocusPrev))
	if app.focused != projectList {
		t.Errorf("account Backtab focused %T, want project list", app.focused)
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
			p := views.NewProjectsPanel(app)
			p.Refresh()
			projectList, _, _ := projectsPanelParts(t, p)
			capture := projectList.GetInputCapture()
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
			app := newMockApp(t)
			p := views.NewProjectsPanel(app)
			p.Refresh()
			_, accountList, _ := projectsPanelParts(t, p)
			capture := accountList.GetInputCapture()
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
			app := newMockApp(t)
			app.svc = newTestService(t, model.NewRegistry())
			p := views.NewProjectsPanel(app)
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
			if app.modal != nil {
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
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	projectList, _, _ := projectsPanelParts(t, p)

	capture := projectList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(specialKey(views.KeyInsert))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Name", "Gamma")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hintsSpecific != views.HintsProjectsPanelSpecific || app.hintsCommon != views.HintsProjectsPanelCommon {
		t.Errorf("hints after save: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
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
	projectList, _, _ := projectsPanelParts(t, p)

	capture := projectList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(specialKey(views.KeyEdit))

	form := formFromCenteredModal(t, app.modal)
	setTextArea(t, form, "Description", "new desc")
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	if app.hintsSpecific != views.HintsProjectsPanelSpecific || app.hintsCommon != views.HintsProjectsPanelCommon {
		t.Errorf("hints after save: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
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
	projectList, _, _ := projectsPanelParts(t, p)

	before := len(app.svc.Projects())

	capture := projectList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(specialKey(views.KeyDelete))

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
	if app.hintsSpecific != views.HintsProjectsPanelSpecific || app.hintsCommon != views.HintsProjectsPanelCommon {
		t.Errorf("hints after cancel: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
	}
	if got := len(app.svc.Projects()); got != before {
		t.Errorf("project count changed after cancel: got %d, want %d (unchanged)", got, before)
	}
}

func TestProjectsPanel_GrantFromAccounts_SaveReturnsToAccountHints(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	alice, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	_, err = reg.AddAccount(reg.User("bob@example.com"), reg.Provider("claude"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProjectToAccount(alice, reg.Project("Alpha")))

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewProjectsPanel(app)
	p.Refresh()
	_, accountList, _ := projectsPanelParts(t, p)

	capture := accountList.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil account input capture")
	}
	capture(specialKey(views.KeyInsert))

	form := formFromCenteredModal(t, app.modal)
	setDropDown(t, form, "Account", 0) // only eligible candidate: bob|claude
	triggerSave(t, form)

	if app.hintsSpecific != views.HintsProjectAccountsSpecific || app.hintsCommon != views.HintsProjectAccountsCommon {
		t.Errorf("hints after save: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)
	}
	projects, err := app.svc.AccountProjects("bob@example.com", "claude")
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
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	projectList, accountList, _ := projectsPanelParts(t, p)

	projectList.Focus(func(tview.Primitive) {})
	if app.hintsSpecific != views.HintsProjectsPanelSpecific || app.hintsCommon != views.HintsProjectsPanelCommon {
		t.Errorf("project focus hints: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon)
	}

	accountList.Focus(func(tview.Primitive) {})
	if app.hintsSpecific != views.HintsProjectAccountsSpecific || app.hintsCommon != views.HintsProjectAccountsCommon {
		t.Errorf("account focus hints: got (%q, %q), want (%q, %q)",
			app.hintsSpecific, app.hintsCommon, views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon)
	}
}

// TestProjectItemText verifies a project row is its name with the shared
// one-cell inset, including when the project has no granted accounts.
func TestProjectItemText(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewProjectsPanel(app)
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

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewProjectsPanel(app)
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
	projectList, _, _ := projectsPanelParts(t, p)
	got, _ := projectList.GetItemText(0)
	want := tview.Escape("Alpha[red]")
	if !strings.Contains(got, want) {
		t.Errorf("got %q, want it to contain %q", got, want)
	}
}
