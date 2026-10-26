package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// layoutStore is an in-memory store for layout tests.
type layoutStore struct{ reg *model.Registry }

func (s layoutStore) Load() (*model.Registry, error) { return s.reg, nil }
func (s layoutStore) Save(*model.Registry) error     { return nil }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}

func TestRun(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
	}{
		{
			name:       "missing data flag",
			args:       nil,
			wantCode:   exitUsage,
			wantStderr: "-data flag is required",
		},
		{
			name:       "unsupported format",
			args:       []string{"-data", "registry.txt"},
			wantCode:   exitFailure,
			wantStderr: "unsupported data file format",
		},
		{
			name:       "unknown flag",
			args:       []string{"-unknown"},
			wantCode:   exitUsage,
			wantStderr: "flag provided but not defined",
		},
		{
			name:       "help",
			args:       []string{"-h"},
			wantCode:   exitSuccess,
			wantStderr: "Usage of llm-desk",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			if got := run(tc.args, &stderr); got != tc.wantCode {
				t.Fatalf("run() = %d, want %d; stderr: %s", got, tc.wantCode, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}

func TestOpenStore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"json extension", "registry.json", false},
		{"unsupported extension", "registry.txt", true},
		{"no extension", "registry", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, err := openStore(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s == nil {
				t.Fatal("expected non-nil Store")
			}
		})
	}
}

// TestNew_SizesAccountsPanelToWidestRow verifies the main layout allocates and
// updates the accounts panel width to fit the widest row, including its border
// and the full pause status.
func TestNew_SizesAccountsPanelToWidestRow(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	proj := model.NewProject("abcdefghijklmnopqrst", "", "")
	must(t, reg.AddProject(proj))
	must(t, reg.AddProjectToAccount(acc, proj))
	_, err = reg.AddSession(acc, proj, "", nil)
	must(t, err)

	svc, err := service.New(layoutStore{reg: reg})
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	app, err := New(svc)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(200, 20)

	app.mainFlex.SetRect(0, 0, 200, 20)
	app.mainFlex.Draw(screen)

	_, _, width, _ := app.accounts.GetRect()
	if want := app.accounts.RequiredWidth(); width != want {
		t.Errorf("accounts panel width = %d, want %d", width, want)
	}
	row, _ := app.accounts.GetItemText(0)
	if want := tview.TaggedStringWidth(row) + 2; width < want {
		t.Errorf("accounts panel width = %d, want at least %d to fit row %q", width, want, row)
	}

	initialWidth := width
	must(t, reg.PauseAccount(acc, time.Now().AddDate(0, 0, -5)))
	app.refreshAccounts()
	app.mainFlex.Draw(screen)

	_, _, width, _ = app.accounts.GetRect()
	if want := app.accounts.RequiredWidth(); width != want {
		t.Errorf("accounts panel width after pause = %d, want %d", width, want)
	}
	if width <= initialWidth {
		t.Errorf("accounts panel width after pause = %d, want greater than %d", width, initialWidth)
	}
	row, _ = app.accounts.GetItemText(0)
	if want := tview.TaggedStringWidth(row) + 2; width < want {
		t.Errorf("accounts panel width after pause = %d, want at least %d to fit row %q", width, want, row)
	}
}

// TestNew_AccountInfoFullWidthAndWrapHeight verifies the account panel spans
// the content width and grows with wrapped project text, while the top row
// keeps its minimum height on very short terminals.
func TestNew_AccountInfoFullWidthAndWrapHeight(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	proj := model.NewProject(strings.Repeat("long-project-", 12), "", "")
	must(t, reg.AddProject(proj))
	must(t, reg.AddProjectToAccount(acc, proj))
	_, err = reg.AddSession(acc, proj, "", nil)
	must(t, err)

	svc, err := service.New(layoutStore{reg: reg})
	must(t, err)
	app, err := New(svc)
	must(t, err)

	screen := tcell.NewSimulationScreen("UTF-8")
	must(t, screen.Init())
	t.Cleanup(screen.Fini)

	app.resizeAccountInfo(80, 24)
	app.mainFlex.SetRect(0, 0, 80, 24-statusBarHeight)
	app.mainFlex.Draw(screen)

	_, _, infoWidth, infoHeight := app.accountInfo.GetRect()
	if infoWidth != 80 {
		t.Errorf("account info width = %d, want 80", infoWidth)
	}
	wantHeight := app.accountInfo.RequiredHeight(80)
	if infoHeight != wantHeight {
		t.Errorf("account info height = %d, want %d", infoHeight, wantHeight)
	}
	if wantHeight <= 5 {
		t.Errorf("account info height = %d, want more than 5 for wrapped project text", wantHeight)
	}
	_, _, _, topHeight := app.mainTop.GetRect()
	if topHeight < minMainTopHeight {
		t.Errorf("main top height = %d, want at least %d", topHeight, minMainTopHeight)
	}

	app.resizeAccountInfo(80, statusBarHeight+minMainTopHeight)
	app.mainFlex.SetRect(0, 0, 80, minMainTopHeight)
	app.mainFlex.Draw(screen)

	_, _, _, infoHeight = app.accountInfo.GetRect()
	if infoHeight != 0 {
		t.Errorf("account info height = %d, want 0 when only the top minimum fits", infoHeight)
	}
	_, _, _, topHeight = app.mainTop.GetRect()
	if topHeight != minMainTopHeight {
		t.Errorf("main top height = %d, want %d", topHeight, minMainTopHeight)
	}
}

// TestNew_DetailPanelsFitContent verifies the vendor, project, and user
// detail panels are sized to their wrapped content rather than a fixed height.
func TestNew_DetailPanelsFitContent(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	vendorName := strings.Repeat("vendor-", 12)
	must(t, reg.AddVendor(model.NewVendor(vendorName, true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser(strings.Repeat("long-", 20)+"@example.com", "Aa"+strings.Repeat("long ", 20))))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor(vendorName))
	must(t, err)
	proj := model.NewProject("Alpha", strings.Repeat("description ", 12), "instructions")
	must(t, reg.AddProject(proj))
	must(t, reg.AddProjectToAccount(acc, proj))

	svc, err := service.New(layoutStore{reg: reg})
	must(t, err)
	app, err := New(svc)
	must(t, err)

	screen := tcell.NewSimulationScreen("UTF-8")
	must(t, screen.Init())
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	app.resizeDetailPanels(80, 24)

	app.vendors.SetRect(0, 0, 80, 24-statusBarHeight)
	app.vendors.Draw(screen)
	vendorInfo, ok := app.vendors.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected vendor info *tview.TextView, got %T", app.vendors.GetItem(1))
	}
	_, _, _, vendorHeight := vendorInfo.GetRect()
	if want := app.vendors.RequiredInfoHeight(80); vendorHeight != want {
		t.Errorf("vendor info height = %d, want %d", vendorHeight, want)
	}
	if want := app.vendors.RequiredInfoHeight(80); want <= 5 {
		t.Errorf("vendor info height = %d, want more than 5 for wrapped name", want)
	}

	app.projects.SetRect(0, 0, 80, 24-statusBarHeight)
	app.projects.Draw(screen)
	projectInfo, ok := app.projects.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected project info *tview.TextView, got %T", app.projects.GetItem(1))
	}
	_, _, _, projectHeight := projectInfo.GetRect()
	if want := app.projects.RequiredInfoHeight(80); projectHeight != want {
		t.Errorf("project info height = %d, want %d", projectHeight, want)
	}
	if want := app.projects.RequiredInfoHeight(80); want <= 5 {
		t.Errorf("project info height = %d, want more than 5 for wrapped description", want)
	}

	app.users.SetRect(0, 0, 80, 24-statusBarHeight)
	app.users.Draw(screen)
	userInfo, ok := app.users.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected user info *tview.TextView, got %T", app.users.GetItem(1))
	}
	_, _, _, userHeight := userInfo.GetRect()
	if want := app.users.RequiredInfoHeight(80); userHeight != want {
		t.Errorf("user info height = %d, want %d", userHeight, want)
	}
	if want := app.users.RequiredInfoHeight(80); want <= 5 {
		t.Errorf("user info height = %d, want more than 5 for wrapped name/email", want)
	}
}
