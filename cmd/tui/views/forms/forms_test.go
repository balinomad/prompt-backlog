package forms_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/internal/viewtest"
	"github.com/gdamore/tcell/v2"

	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/rivo/tview"
)

type fakeApp = viewtest.FakeApp

var (
	must       = viewtest.Must
	newService = viewtest.NewService
	newFakeApp = viewtest.NewFakeApp
	runeKey    = viewtest.RuneKey
)

// ── Dropdown helper ───────────────────────────────────────────────────────────

func TestNewDropDown_ArrowAndWidth(t *testing.T) {
	t.Parallel()
	dropDown := forms.NewDropDown("Project", []string{"none", "Alpha"}, 1)
	dropDown.SetRect(0, 0, 40, 1)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 1)
	dropDown.Draw(screen)

	var row strings.Builder
	for x := range 40 {
		mainc, _, _ := screen.Get(x, 0)
		row.WriteString(mainc)
	}
	if !strings.Contains(row.String(), forms.DropDownArrow[1:]) {
		t.Errorf("dropdown row = %q, want arrow", row.String())
	}

	wantWidth := tview.TaggedStringWidth("Alpha") + tview.TaggedStringWidth(forms.DropDownArrow)
	if got := dropDown.GetFieldWidth(); got != wantWidth {
		t.Errorf("field width = %d, want %d", got, wantWidth)
	}

	long := forms.NewDropDown("Project", []string{strings.Repeat("x", 100)}, 0)
	if got := long.GetFieldWidth(); got != forms.FormFieldWidth {
		t.Errorf("capped field width = %d, want %d", got, forms.FormFieldWidth)
	}
}

// ── NewSessionForm ────────────────────────────────────────────────────────────

// TestSessionForm_ProjectNames verifies the Project dropdown options.
func TestEditSessionForm_ProjectNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, reg *model.Registry, acc *model.Account)
		want  []string
	}{
		{
			name:  "no granted projects",
			setup: func(t *testing.T, _ *model.Registry, _ *model.Account) { t.Helper() },
			want:  []string{"none"},
		},
		{
			name: "normal projects",
			setup: func(t *testing.T, reg *model.Registry, acc *model.Account) {
				t.Helper()
				must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
				must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
				must(t, reg.AddProject(model.NewProject("Beta", "", "")))
				must(t, reg.AddProjectToAccount(acc, reg.Project("Beta")))
			},
			want: []string{"none", "Alpha", "Beta"},
		},
		{
			name: "project literally named none collides with the sentinel",
			setup: func(t *testing.T, reg *model.Registry, acc *model.Account) {
				t.Helper()
				must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
				must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
				must(t, reg.AddProject(model.NewProject("none", "", "")))
				must(t, reg.AddProjectToAccount(acc, reg.Project("none")))
			},
			// AccountProjects sorts by name: "Alpha" before "none". Index 0
			// is always the sentinel, so the real "none" project must land
			// at index 2, distinct from the sentinel at index 0.
			want: []string{"none", "Alpha", "none"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
			must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
			acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
			must(t, err)
			tc.setup(t, reg, acc)

			app := &fakeApp{Svc: newService(t, reg)}
			form := formFromCenteredModal(t, forms.NewSessionForm(app, &forms.SessionFormOptions{
				Title:       "Edit Session",
				SubmitLabel: "Save",
				Account:     acc,
				OnSubmit:    func(service.SessionFields) error { return nil },
				OnCancel:    func() {},
			}))
			dropDown, ok := form.GetFormItemByLabel("Project").(*tview.DropDown)
			if !ok {
				t.Fatal("expected Project *tview.DropDown")
			}
			got := dropDownOptions(t, dropDown)

			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("got[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

// ── NewProjectForm ────────────────────────────────────────────────────────────

// TestNewProjectForm_ReturnsForm verifies create and edit modes both return a
// form.
func TestNewProjectForm_ReturnsForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		projectName string // empty = nil project = create mode
	}{
		{"create mode", ""},
		{"edit mode", "Alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			var proj *model.Project
			if tc.projectName != "" {
				proj = app.Svc.Project(tc.projectName)
			}
			result := forms.NewProjectForm(app, proj, func() {})
			assertFormOrModal(t, result, false)
		})
	}
}

func TestNewProjectForm_NameField(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)

	cases := []struct {
		name        string
		projectName string // empty = nil project = create mode
		wantName    bool
	}{
		{"create mode: editable name", "", true},
		{"edit mode: editable name", "Alpha", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var proj *model.Project
			if tc.projectName != "" {
				proj = app.Svc.Project(tc.projectName)
			}
			form := formFromCenteredModal(t, forms.NewProjectForm(app, proj, func() {}))
			item := form.GetFormItemByLabel("Name")
			if !tc.wantName {
				if item != nil {
					t.Error("edit mode should not repeat the project name as a field")
				}
				return
			}
			nameField, ok := item.(*forms.InputField)
			if !ok {
				t.Fatal("expected Name *forms.InputField")
			}
			if tc.projectName != "" && nameField.GetText() != tc.projectName {
				t.Errorf("Name = %q, want %q", nameField.GetText(), tc.projectName)
			}
			if !inputFieldAcceptsText(nameField) {
				t.Error("Name field should accept text")
			}
		})
	}
}

func TestNewProjectForm_Rename(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	project := app.Svc.Project("Alpha")
	form := formFromCenteredModal(t, forms.NewProjectForm(app, project, func() {}))

	form.GetFormItemByLabel("Name").(*forms.InputField).SetText("Renamed", false) //nolint:errcheck // The item type is known.
	pressButton(t, form.GetButton(0))

	if app.Svc.Project("Alpha") != nil {
		t.Error("old project name still resolves")
	}
	renamed := app.Svc.Project("Renamed")
	if renamed == nil {
		t.Fatal("renamed project not found")
	}
	if renamed.ID() != project.ID() {
		t.Errorf("ID = %d, want %d", renamed.ID(), project.ID())
	}
}

func TestNewProjectForm_MultilineFields(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)

	cases := []struct {
		name        string
		projectName string // empty = nil project = create mode
	}{
		{"create mode", ""},
		{"edit mode", "Alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var proj *model.Project
			if tc.projectName != "" {
				proj = app.Svc.Project(tc.projectName)
			}
			form := formFromCenteredModal(t, forms.NewProjectForm(app, proj, func() {}))
			for _, label := range []string{"Description", "Instructions"} {
				item := form.GetFormItemByLabel(label)
				area, ok := item.(*tview.TextArea)
				if !ok {
					t.Fatalf("expected %q to be *tview.TextArea, got %T", label, item)
				}
				if got := area.GetFieldHeight(); got != 2 {
					t.Errorf("%s field height = %d, want 2", label, got)
				}
			}
		})
	}
}

// TestNewProjectForm_Save_Edit covers save()'s isEdit branch:
// UpdateProject's success and not-found paths.
func TestNewProjectForm_Save_Edit(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "old desc", "old inst")))
		app := &fakeApp{Svc: newService(t, reg)}
		form := formFromCenteredModal(t, forms.NewProjectForm(app, app.Svc.Project("Alpha"), func() {}))

		setTextArea(t, form, "Description", "new desc")
		setTextArea(t, form, "Instructions", "new inst")
		triggerSave(t, form)

		got := app.Svc.Project("Alpha")
		if got.Description() != "new desc" || got.Instructions() != "new inst" {
			t.Errorf("got desc %q inst %q, want both updated", got.Description(), got.Instructions())
		}
	})

	t.Run("failure: project deleted before save", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		app := &fakeApp{Svc: newService(t, reg)}
		proj := app.Svc.Project("Alpha")
		form := formFromCenteredModal(t, forms.NewProjectForm(app, proj, func() {}))

		must(t, app.Svc.DeleteProject("Alpha"))

		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// TestNewProjectForm_Save_Create covers save()'s !isEdit branch: the
// pre-Mutate ValidateProject check, AddProject's success path, and
// AddProject's own failure path (duplicate name), which is a distinct
// code path from the pre-flight validation failure.
func TestNewProjectForm_Save_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		form := formFromCenteredModal(t, forms.NewProjectForm(app, nil, func() {}))

		setInputField(t, form, "Name", "Gamma")
		setTextArea(t, form, "Description", "desc")
		setTextArea(t, form, "Instructions", "inst")
		triggerSave(t, form)

		got := app.Svc.Project("Gamma")
		if got == nil {
			t.Fatal("project not created")
		}
		if got.Description() != "desc" || got.Instructions() != "inst" {
			t.Errorf("got desc %q inst %q", got.Description(), got.Instructions())
		}
	})

	t.Run("validation failure: empty name never mutates", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		before := len(app.Svc.Projects())
		form := formFromCenteredModal(t, forms.NewProjectForm(app, nil, func() {}))

		setInputField(t, form, "Name", "")
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the validation failure")
		}
		if got := len(app.Svc.Projects()); got != before {
			t.Errorf("project count changed: got %d, want %d (unchanged)", got, before)
		}
	})

	t.Run("failure: duplicate name", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t) // already has "Alpha"
		form := formFromCenteredModal(t, forms.NewProjectForm(app, nil, func() {}))

		setInputField(t, form, "Name", "Alpha")
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── NewVendorForm ───────────────────────────────────────────────────────────

func TestNewVendorForm_ReturnsForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		vendorName string // empty = nil vendor = create mode
	}{
		{"create mode", ""},
		{"edit mode", "claude"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			var prov *model.Vendor
			if tc.vendorName != "" {
				prov = app.Svc.Vendor(tc.vendorName)
			}
			result := forms.NewVendorForm(app, prov, func() {})
			assertFormOrModal(t, result, false)
		})
	}
}

func TestNewVendorForm_NameFieldDisabledState(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)

	cases := []struct {
		name        string
		vendorName  string // empty = nil vendor = create mode
		wantAccepts bool
	}{
		{"create mode: enabled", "", true},
		{"edit mode: disabled", "claude", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var prov *model.Vendor
			if tc.vendorName != "" {
				prov = app.Svc.Vendor(tc.vendorName)
			}
			form := formFromCenteredModal(t, forms.NewVendorForm(app, prov, func() {}))
			nameField, ok := form.GetFormItemByLabel("Name").(*forms.InputField)
			if !ok {
				t.Fatal("expected Name *forms.InputField")
			}
			if got := inputFieldAcceptsText(nameField); got != tc.wantAccepts {
				t.Errorf("accepts text: got %v, want %v", got, tc.wantAccepts)
			}
		})
	}
}

// TestNewVendorForm_Save_Edit covers save()'s isEdit branch:
// UpdateVendor's success and not-found paths.
func TestNewVendorForm_Save_Edit(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", false, false)))
		app := &fakeApp{Svc: newService(t, reg)}
		form := formFromCenteredModal(t, forms.NewVendorForm(app, app.Svc.Vendor("claude"), func() {}))

		setCheckbox(t, form, "Supports projects")
		setCheckbox(t, form, "Supports deep research")
		triggerSave(t, form)

		got := app.Svc.Vendor("claude")
		if !got.HasProjects() || !got.HasDeepResearch() {
			t.Errorf("HasProjects=%v HasDeepResearch=%v, want both true", got.HasProjects(), got.HasDeepResearch())
		}
	})

	t.Run("failure: vendor deleted before save", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", false, false)))
		app := &fakeApp{Svc: newService(t, reg)}
		prov := app.Svc.Vendor("claude")
		form := formFromCenteredModal(t, forms.NewVendorForm(app, prov, func() {}))

		must(t, app.Svc.DeleteVendor("claude"))

		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// TestNewVendorForm_Save_Create covers save()'s !isEdit branch: the
// pre-Mutate ValidateVendor check, AddVendor's success path, and
// AddVendor's own failure path (duplicate name).
func TestNewVendorForm_Save_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		form := formFromCenteredModal(t, forms.NewVendorForm(app, nil, func() {}))

		setInputField(t, form, "Name", "gemini")
		setCheckbox(t, form, "Supports projects")
		triggerSave(t, form)

		got := app.Svc.Vendor("gemini")
		if got == nil {
			t.Fatal("vendor not created")
		}
		if !got.HasProjects() {
			t.Error("HasProjects() = false, want true")
		}
	})

	t.Run("validation failure: empty name never mutates", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		before := len(app.Svc.Vendors())
		form := formFromCenteredModal(t, forms.NewVendorForm(app, nil, func() {}))

		setInputField(t, form, "Name", "")
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the validation failure")
		}
		if got := len(app.Svc.Vendors()); got != before {
			t.Errorf("vendor count changed: got %d, want %d (unchanged)", got, before)
		}
	})

	t.Run("failure: duplicate name", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t) // already has "claude"
		form := formFromCenteredModal(t, forms.NewVendorForm(app, nil, func() {}))

		setInputField(t, form, "Name", "claude")
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── NewAccountForm ────────────────────────────────────────────────────────────

func TestNewAccountForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(t *testing.T) (*fakeApp, *model.Account)
		wantModal bool
	}{
		{
			name: "no vendors",
			setup: func(t *testing.T) (*fakeApp, *model.Account) {
				t.Helper()
				app := newFakeApp(t)
				app.Svc = newService(t, model.NewRegistry())
				return app, nil
			},
			wantModal: true,
		},
		{
			name: "create mode",
			setup: func(t *testing.T) (*fakeApp, *model.Account) {
				t.Helper()
				return newFakeApp(t), nil
			},
			wantModal: false,
		},
		{
			name: "edit mode",
			setup: func(t *testing.T) (*fakeApp, *model.Account) {
				t.Helper()
				app := newFakeApp(t)
				acc, err := app.Svc.Account("alice@example.com", "claude")
				must(t, err)
				return app, acc
			},
			wantModal: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, acc := tc.setup(t)
			result := forms.NewAccountForm(app, acc, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

func TestAccountEditForm_Layout(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc, err := app.Svc.Account("alice@example.com", "claude")
	must(t, err)

	form := formFromCenteredModal(t, forms.NewAccountForm(app, acc, func() {}))

	if got := strings.TrimSpace(form.GetTitle()); got != "Pause Account" {
		t.Errorf("title = %q, want %q", got, "Pause Account")
	}

	if got := form.GetFormItemCount(); got != 2 {
		t.Fatalf("form item count = %d, want 2", got)
	}
	label, ok := form.GetFormItem(0).(interface{ GetText(bool) string })
	if !ok {
		t.Fatal("expected centered label form item")
	}
	if got := label.GetText(false); got != "claude · Alice" {
		t.Errorf("centered label = %q, want %q", got, "claude · Alice")
	}
	if got := form.GetFormItem(1).GetLabel(); got != "Pause until" {
		t.Errorf("form item 1 label = %q, want %q", got, "Pause until")
	}
	if item := form.GetFormItemByLabel("Vendor"); item != nil {
		t.Error("Vendor should not be a separate form field")
	}
	if item := form.GetFormItemByLabel("User"); item != nil {
		t.Error("User should not be a separate form field")
	}
	if idx := form.GetButtonIndex("Account…"); idx >= 0 {
		t.Error("Account… button must be removed")
	}
}

// TestAccountEditForm_Save covers accountEditForm's save(): the parsed
// pause-time success path, the empty-clears-pause path, the invalid
// format path (fails before any Mutate), and PauseAccount's own failure
// path (account removed before save).
func TestAccountEditForm_Save(t *testing.T) {
	t.Parallel()

	t.Run("sets pause time", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		acc, err := app.Svc.Account("alice@example.com", "claude")
		must(t, err)
		form := formFromCenteredModal(t, forms.NewAccountForm(app, acc, func() {}))

		setInputField(t, form, "Pause until", "2026-12-31 23:59")
		triggerSave(t, form)

		got, err := app.Svc.Account("alice@example.com", "claude")
		must(t, err)
		want, err := model.ParseTime("2026-12-31 23:59")
		must(t, err)
		if !got.PauseUntil().Equal(want) {
			t.Errorf("PauseUntil() = %v, want %v", got.PauseUntil(), want)
		}
	})

	t.Run("empty clears an existing pause", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		must(t, app.Svc.PauseAccount("alice@example.com", "claude", time.Now().Add(time.Hour)))
		acc, err := app.Svc.Account("alice@example.com", "claude") // read after pausing: PauseAccount replaces the account via Copy-on-Write
		must(t, err)
		form := formFromCenteredModal(t, forms.NewAccountForm(app, acc, func() {}))

		setInputField(t, form, "Pause until", "")
		triggerSave(t, form)

		got, err := app.Svc.Account("alice@example.com", "claude")
		must(t, err)
		if !got.PauseUntil().IsZero() {
			t.Errorf("PauseUntil() = %v, want zero", got.PauseUntil())
		}
	})

	t.Run("invalid format never mutates", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		acc, err := app.Svc.Account("alice@example.com", "claude")
		must(t, err)
		before := acc.PauseUntil()
		form := formFromCenteredModal(t, forms.NewAccountForm(app, acc, func() {}))

		setInputField(t, form, "Pause until", "not-a-time")
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the format error")
		}
		got, err := app.Svc.Account("alice@example.com", "claude")
		must(t, err)
		if !got.PauseUntil().Equal(before) {
			t.Errorf("PauseUntil() changed: got %v, want unchanged %v", got.PauseUntil(), before)
		}
	})

	t.Run("failure: account removed before save", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		app := &fakeApp{Svc: newService(t, reg)}
		form := formFromCenteredModal(t, forms.NewAccountForm(app, acc, func() {}))

		must(t, app.Svc.DeleteAccount("alice@example.com", "claude"))

		setInputField(t, form, "Pause until", "2026-12-31 23:59")
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// TestAccountCreateForm_Save covers accountCreateForm's save: selecting an
// existing user, the zero-user guard, and the duplicate-account failure.
func TestAccountCreateForm_Save(t *testing.T) {
	t.Parallel()

	t.Run("success: existing user selected from dropdown", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		form := formFromCenteredModal(t, forms.NewAccountForm(app, nil, func() {}))

		setDropDown(t, form, "User", 0)   // alice
		setDropDown(t, form, "Vendor", 0) // chatgpt
		triggerSave(t, form)

		if _, err := app.Svc.Account("alice@example.com", "chatgpt"); err != nil {
			t.Errorf("account not created: %v", err)
		}
	})

	t.Run("zero users shows an error modal", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		app := &fakeApp{Svc: newService(t, reg)}
		prim := forms.NewAccountForm(app, nil, func() {})
		if _, ok := views.UnwrapModal(prim).(*tview.Modal); !ok {
			t.Errorf("got %T, want error modal", prim)
		}
	})

	t.Run("failure: account already exists", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t) // alice|claude already exists
		form := formFromCenteredModal(t, forms.NewAccountForm(app, nil, func() {}))

		setDropDown(t, form, "User", 0)   // alice
		setDropDown(t, form, "Vendor", 1) // claude
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── NewUserForm ───────────────────────────────────────────────────────────────

func TestNewUserForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(t *testing.T) (*fakeApp, *model.User)
		wantModal bool
	}{
		{
			name: "nil user creates form",
			setup: func(t *testing.T) (*fakeApp, *model.User) {
				t.Helper()
				return newFakeApp(t), nil
			},
			wantModal: false,
		},
		{
			name: "user no longer exists",
			setup: func(t *testing.T) (*fakeApp, *model.User) {
				t.Helper()
				app := newFakeApp(t)
				return app, model.NewUser("ghost@example.com", "Ghost")
			},
			wantModal: true,
		},
		{
			name: "edit mode",
			setup: func(t *testing.T) (*fakeApp, *model.User) {
				t.Helper()
				app := newFakeApp(t)
				return app, app.Svc.User("alice@example.com")
			},
			wantModal: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, user := tc.setup(t)
			result := forms.NewUserForm(app, user, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

func TestNewUserForm_FieldState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		user        *model.User
		wantEmailOK bool
	}{
		{"create mode email editable", nil, true},
		{"edit mode email disabled", model.NewUser("alice@example.com", "Alice"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			form := formFromCenteredModal(t, forms.NewUserForm(app, tc.user, func() {}))
			emailField, ok := form.GetFormItemByLabel("Email").(*forms.InputField)
			if !ok {
				t.Fatal("expected Email *forms.InputField")
			}
			if got := inputFieldAcceptsText(emailField); got != tc.wantEmailOK {
				t.Errorf("email accepts text: got %v, want %v", got, tc.wantEmailOK)
			}
		})
	}
}

// TestNewUserForm_Save covers create and edit saves plus the failure paths.
func TestNewUserForm_Save(t *testing.T) {
	t.Parallel()

	t.Run("create success", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		form := formFromCenteredModal(t, forms.NewUserForm(app, nil, func() {}))

		setInputField(t, form, "Email", "carol@example.com")
		setInputField(t, form, "Name", "Carol")
		triggerSave(t, form)

		got := app.Svc.User("carol@example.com")
		if got == nil {
			t.Fatal("user not created")
		}
		if got.Name() != "Carol" {
			t.Errorf("Name() = %q, want %q", got.Name(), "Carol")
		}
	})

	t.Run("edit success", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		form := formFromCenteredModal(t, forms.NewUserForm(app, app.Svc.User("alice@example.com"), func() {}))

		setInputField(t, form, "Name", "Alice Smith")
		triggerSave(t, form)

		got := app.Svc.User("alice@example.com")
		if got.Name() != "Alice Smith" {
			t.Errorf("Name() = %q, want %q", got.Name(), "Alice Smith")
		}
	})

	t.Run("create failure: invalid email", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		form := formFromCenteredModal(t, forms.NewUserForm(app, nil, func() {}))

		setInputField(t, form, "Email", "not-an-email")
		setInputField(t, form, "Name", "Nobody")
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the validation failure")
		}
		if app.Svc.User("not-an-email") != nil {
			t.Error("user should not have been created")
		}
	})

	t.Run("create failure: duplicate email", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t)
		form := formFromCenteredModal(t, forms.NewUserForm(app, nil, func() {}))

		setInputField(t, form, "Email", "alice@example.com")
		setInputField(t, form, "Name", "Alice2")
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})

	t.Run("edit failure: user removed before save", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("solo@example.com", "Solo")))
		app := &fakeApp{Svc: newService(t, reg)}
		form := formFromCenteredModal(t, forms.NewUserForm(app, app.Svc.User("solo@example.com"), func() {}))

		must(t, app.Svc.DeleteUser("solo@example.com"))

		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── NewGrantForm ──────────────────────────────────────────────────────────────

func TestNewGrantForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(t *testing.T) (*fakeApp, *model.Project)
		wantModal bool
	}{
		{
			name: "nil project",
			setup: func(t *testing.T) (*fakeApp, *model.Project) {
				t.Helper()
				return newFakeApp(t), nil
			},
			wantModal: true,
		},
		{
			name: "no accounts exist",
			setup: func(t *testing.T) (*fakeApp, *model.Project) {
				t.Helper()
				reg := model.NewRegistry()
				must(t, reg.AddProject(model.NewProject("Solo", "", "")))
				app := newFakeApp(t)
				app.Svc = newService(t, reg)
				return app, app.Svc.Project("Solo")
			},
			wantModal: true,
		},
		{
			name: "accounts available",
			setup: func(t *testing.T) (*fakeApp, *model.Project) {
				t.Helper()
				app := newFakeApp(t)
				return app, app.Svc.Project("Orphan")
			},
			wantModal: false,
		},
		{
			name: "no eligible accounts",
			setup: func(t *testing.T) (*fakeApp, *model.Project) {
				t.Helper()
				app := newFakeApp(t)
				return app, app.Svc.Project("Alpha")
			},
			wantModal: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, proj := tc.setup(t)
			result := forms.NewGrantForm(app, proj, func() {})
			assertFormOrModal(t, result, tc.wantModal)
		})
	}
}

// TestNewGrantForm_FiltersCandidates verifies the picker excludes accounts
// that already have access and accounts whose vendor does not support
// projects.
func TestNewGrantForm_FiltersCandidates(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	proj := app.Svc.Project("Orphan")
	form := formFromCenteredModal(t, forms.NewGrantForm(app, proj, func() {}))

	dropDown, ok := form.GetFormItemByLabel("Account").(*tview.DropDown)
	if !ok {
		t.Fatal("expected *tview.DropDown for Account")
	}
	if got := dropDown.GetOptionCount(); got != 1 {
		t.Fatalf("option count: got %d, want 1", got)
	}
	_, text := dropDown.GetCurrentOption()
	if !strings.Contains(text, "claude") || !strings.Contains(text, "Alice") {
		t.Errorf("option = %q, want claude and Alice", text)
	}
	if strings.Contains(text, "chatgpt") || strings.Contains(text, "Bob") {
		t.Errorf("option = %q, must not include the project-ineligible account", text)
	}
}

// TestNewGrantForm_Save covers save()'s GrantProject call: the success
// path and the Mutate failure path.
func TestNewGrantForm_Save(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t) // Orphan is granted to no one
		proj := app.Svc.Project("Orphan")
		form := formFromCenteredModal(t, forms.NewGrantForm(app, proj, func() {}))

		setDropDown(t, form, "Account", 0) // only eligible candidate: alice|claude
		triggerSave(t, form)

		projects, err := app.Svc.AccountProjects("alice@example.com", "claude")
		must(t, err)
		var found bool
		for _, p := range projects {
			if p.Name() == "Orphan" {
				found = true
			}
		}
		if !found {
			t.Error("Orphan not granted to alice|claude")
		}
	})

	t.Run("failure: mutate error", func(t *testing.T) {
		t.Parallel()
		app := newFakeApp(t) // Orphan is granted to no one
		app.MutateErr = errors.New("save failed")
		proj := app.Svc.Project("Orphan")
		form := formFromCenteredModal(t, forms.NewGrantForm(app, proj, func() {}))

		setDropDown(t, form, "Account", 0)
		triggerSave(t, form)

		if app.Status == "" {
			t.Error("expected a status message reporting the save failure")
		}
	})
}

// ── addCtrlS ──────────────────────────────────────────────────────────────────

// TestAddCtrlS_NonSaveKeyPassesThrough covers the return event branch:
// addCtrlS is shared across all eight forms, so exercising it through any
// one of them (NewProjectForm, arbitrarily) covers every caller.
func TestAddCtrlS_NonSaveKeyPassesThrough(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	before := len(app.Svc.Projects())
	form := formFromCenteredModal(t, forms.NewProjectForm(app, nil, func() {}))

	capture := form.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil form input capture (addCtrlS)")
	}
	event := runeKey('z')
	if got := capture(event); got != event {
		t.Error("non-Ctrl+S key must pass through unchanged")
	}
	if got := len(app.Svc.Projects()); got != before {
		t.Errorf("project count changed: got %d, want %d (save must not have run)", got, before)
	}
}

func TestNewFormTitle(t *testing.T) {
	t.Parallel()
	form := forms.NewForm("Edit Account")
	if got := form.GetTitle(); got != " Edit Account " {
		t.Errorf("GetTitle() = %q, want %q", got, " Edit Account ")
	}
}

func TestFormHeight(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		heights []int
		buttons int
		want    int
	}{
		{"no items", nil, 0, 4},
		{"one item", []int{1}, 0, 5},
		{"one item and button", []int{1}, 1, 7},
		{"two items and buttons", []int{1, 1}, 2, 9},
		{"three items and buttons", []int{1, 1, 1}, 3, 11},
		{"four items and buttons", []int{1, 1, 1, 1}, 2, 13},
		{"two-line items", []int{2, 2}, 0, 9},
		{"mixed item heights and buttons", []int{1, 2, 2}, 2, 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			form := tview.NewForm()
			for _, height := range tc.heights {
				if height == 1 {
					form.AddInputField("field", "", forms.FormFieldWidth, nil, nil)
					continue
				}
				form.AddTextArea("field", "", forms.FormFieldWidth, height, 0, nil)
			}
			for range tc.buttons {
				form.AddButton("button", nil)
			}
			if got := forms.FormHeight(form); got != tc.want {
				t.Errorf("forms.FormHeight() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFormWidth(t *testing.T) {
	t.Parallel()
	labelText := "claude · Alice"
	cases := []struct {
		name string
		form func() *tview.Form
		want int
	}{
		{
			name: "label and field",
			form: func() *tview.Form {
				form := tview.NewForm()
				form.AddInputField("Name", "", forms.FormFieldWidth, nil, nil)
				return form
			},
			want: 49,
		},
		{
			name: "pause field",
			form: func() *tview.Form {
				form := tview.NewForm()
				form.AddInputField("Pause until", "", forms.FormPauseWidth, nil, nil)
				return form
			},
			want: 33,
		},
		{
			name: "buttons",
			form: func() *tview.Form {
				form := tview.NewForm()
				form.AddButton("Save", nil)
				form.AddButton("Cancel", nil)
				return form
			},
			want: 23,
		},
		{
			name: "centered label",
			form: func() *tview.Form {
				form := tview.NewForm()
				form.AddFormItem(forms.NewCenteredFormLabel(labelText))
				return form
			},
			want: tview.TaggedStringWidth(labelText) + 1 + 4,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := forms.FormWidth(tc.form()); got != tc.want {
				t.Errorf("forms.FormWidth() = %d, want %d", got, tc.want)
			}
		})
	}
}

// assertFormOrModal checks that result is a modal or a centered form.
func assertFormOrModal(t *testing.T, result tview.Primitive, wantModal bool) {
	t.Helper()
	result = views.UnwrapModal(result)
	_, isModal := result.(*tview.Modal)
	if wantModal {
		if !isModal {
			t.Errorf("expected error modal, got %T", result)
		}
		return
	}
	if _, ok := result.(*tview.Flex); !ok {
		t.Errorf("expected centred form, got %T", result)
	}
}

// formFromCenteredModal returns the form inside a centered modal wrapper.
func formFromCenteredModal(t *testing.T, p tview.Primitive) *tview.Form {
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
	form, ok := row.GetItem(2).(*tview.Form)
	if !ok {
		t.Fatalf("expected *tview.Form, got %T", row.GetItem(2))
	}
	return form
}

// inputFieldAcceptsText reports whether typing changes field's text.
func inputFieldAcceptsText(field *forms.InputField) bool {
	before := field.GetText()
	field.InputHandler()(runeKey('z'), func(tview.Primitive) {})
	return field.GetText() != before
}

// dropDownOptions returns every dropdown option and restores the selection.
func dropDownOptions(t *testing.T, d *tview.DropDown) []string {
	t.Helper()
	restore, _ := d.GetCurrentOption()
	defer d.SetCurrentOption(restore)

	n := d.GetOptionCount()
	opts := make([]string, n)
	for i := range n {
		d.SetCurrentOption(i)
		_, opts[i] = d.GetCurrentOption()
	}
	return opts
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

// pressButton activates a button with Enter.
func pressButton(t *testing.T, button *tview.Button) {
	t.Helper()
	button.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
}

// dropdownOption returns the display text of one dropdown option.
func dropdownOption(t *testing.T, dropdown *tview.DropDown, index int) string {
	t.Helper()
	dropdown.SetCurrentOption(index)
	_, option := dropdown.GetCurrentOption()
	return option
}
