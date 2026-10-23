package main

import (
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	must(reg.AddProvider(model.NewProvider("claude", true, false)))
	must(reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(err)
	proj := model.NewProject("abcdefghijklmnopqrst", "", "")
	must(reg.AddProject(proj))
	must(reg.AddProjectToAccount(acc, proj))
	_, err = reg.AddSession(acc, proj, "", nil)
	must(err)

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
	must(reg.PauseAccount(acc, time.Now().AddDate(0, 0, -5)))
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
