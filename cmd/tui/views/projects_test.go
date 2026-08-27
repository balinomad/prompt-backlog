package views

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/internal/model"

	"github.com/rivo/tview"
)

func TestNewProjectsPanel(t *testing.T) {
	t.Parallel()
	if p := NewProjectsPanel(newMockApp(t)); p == nil {
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
			p := NewProjectsPanel(app)
			p.Refresh()
			if got := p.list.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestProjectsPanel_Refresh_SortedByName(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewProjectsPanel(app)
	p.Refresh()

	itemCount := p.list.GetItemCount()
	names := make([]string, 0, itemCount)
	for i := range itemCount {
		main, _ := p.list.GetItemText(i)
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
	p := NewProjectsPanel(app)
	p.Refresh()
	p.list.SetCurrentItem(1)
	want := p.selectedProject().Name()

	p.Refresh()
	if got := p.selectedProject(); got == nil || got.Name() != want {
		t.Errorf("selection changed: got %v, want %q", got, want)
	}
}

func TestProjectsPanel_Keys_SwitchToMain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fire func(*ProjectsPanel)
	}{
		{"esc key", func(p *ProjectsPanel) { p.handleKey(escKey()) }},
		{"p key", func(p *ProjectsPanel) { p.handleKey(runeKey(KeyProjects)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewProjectsPanel(app)
			p.Refresh()
			tc.fire(p)
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
		{"new", KeyNew, HintsForm},
		{"edit", KeyEdit, HintsForm},
		{"delete", KeyDelete, HintsConfirmModal},
		{"grant", KeyGrant, HintsForm},
		{"revoke", KeyRevoke, HintsForm},
		{"help", KeyHelp, HintsHelpOverlay},
		{"quit", KeyQuit, HintsConfirmModal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewProjectsPanel(app)
			p.Refresh()
			p.handleKey(runeKey(tc.key))
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
		{"edit", KeyEdit},
		{"delete", KeyDelete},
		{"grant", KeyGrant},
		{"revoke", KeyRevoke},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			app.svc = newTestService(t, model.NewRegistry())
			p := NewProjectsPanel(app)
			p.Refresh()
			p.handleKey(runeKey(tc.key))
			if app.modal != nil {
				t.Errorf("key %q with empty list should not open a modal", string(tc.key))
			}
		})
	}
}

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
			p := model.NewProject("Alpha", "", "")
			got := projectItemText(p, tc.accountCount)
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
	p := model.NewProject("Alpha[red]", "", "")
	got := projectItemText(p, 1)
	want := tview.Escape("Alpha[red]")
	if !strings.Contains(got, want) {
		t.Errorf("got %q, want it to contain %q", got, want)
	}
}
