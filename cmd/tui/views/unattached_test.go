package views

import (
	"testing"
)

func TestNewUnattachedPanel(t *testing.T) {
	t.Parallel()
	if p := NewUnattachedPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil UnattachedPanel")
	}
}

func TestUnattachedPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*mockApp)
		wantCount int
	}{
		{"one unattached project", func(_ *mockApp) {}, 1},
		{"three unattached after detach", func(app *mockApp) {
			must(t, app.registry.DetachProject("Alpha"))
			must(t, app.registry.DetachProject("Beta"))
		}, 3},
		{"none after remove", func(app *mockApp) {
			must(t, app.registry.RemoveProject("Orphan"))
		}, 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			tc.setup(app)
			p := NewUnattachedPanel(app)
			p.Refresh()
			if got := p.list.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestUnattachedPanel_Refresh_SortedByName(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	must(t, app.registry.DetachProject("Alpha"))
	must(t, app.registry.DetachProject("Beta"))

	p := NewUnattachedPanel(app)
	p.Refresh()

	var names []string
	for i := 0; i < p.list.GetItemCount(); i++ {
		main, _ := p.list.GetItemText(i)
		names = append(names, main)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("list not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

func TestUnattachedPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	must(t, app.registry.DetachProject("Alpha"))
	must(t, app.registry.DetachProject("Beta"))

	p := NewUnattachedPanel(app)
	p.Refresh()
	p.list.SetCurrentItem(1)
	want := p.selectedProject().Name()

	p.Refresh()
	if got := p.selectedProject(); got == nil || got.Name() != want {
		t.Errorf("selection changed: got %v, want %q", got, want)
	}
}

func TestUnattachedPanel_Keys_SwitchToMain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fire func(*UnattachedPanel)
	}{
		{"esc key", func(p *UnattachedPanel) { p.handleKey(escKey()) }},
		{"s key", func(p *UnattachedPanel) { p.handleKey(runeKey(KeyUnattached)) }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewUnattachedPanel(app)
			p.Refresh()
			tc.fire(p)
			if app.switchedTo != "main" {
				t.Errorf("expected switchedTo=main, got %q", app.switchedTo)
			}
		})
	}
}

func TestUnattachedPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       rune
		wantHints string
	}{
		{"new", KeyNew, HintsForm},
		{"edit", KeyEdit, HintsForm},
		{"delete", KeyDelete, HintsConfirmModal},
		{"help", KeyHelp, HintsHelpOverlay},
		{"quit", KeyQuit, HintsConfirmModal},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewUnattachedPanel(app)
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

func TestUnattachedPanel_Edit_EmptyList_NoModal(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	must(t, app.registry.RemoveProject("Orphan"))
	p := NewUnattachedPanel(app)
	p.Refresh()
	p.handleKey(runeKey(KeyEdit))
	if app.modal != nil {
		t.Error("edit with empty list should not open modal")
	}
}

func TestUnattachedItemText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		description   string
		wantEqualName bool
	}{
		{"no description returns name only", "", true},
		{"description is appended", "Some notes", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			proj := app.registry.ProjectByName("Orphan")
			proj.Description = tc.description
			got := unattachedItemText(proj)
			if tc.wantEqualName && got != "Orphan" {
				t.Errorf("got %q, want %q", got, "Orphan")
			}
			if !tc.wantEqualName && got == "Orphan" {
				t.Error("expected description to appear in text")
			}
		})
	}
}
