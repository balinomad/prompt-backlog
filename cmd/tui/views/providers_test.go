package views

import (
	"testing"

	"github.com/balinomad/llm-desk/internal/model"
)

func TestNewProvidersPanel(t *testing.T) {
	t.Parallel()
	if p := NewProvidersPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil ProvidersPanel")
	}
}

func TestProvidersPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*mockApp)
		wantCount int
	}{
		{"two providers", func(_ *mockApp) {}, 2},
		{"empty registry", func(app *mockApp) {
			app.registry = model.NewRegistry()
		}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			tc.setup(app)
			p := NewProvidersPanel(app)
			p.Refresh()
			if got := p.list.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestProvidersPanel_Refresh_SortedByName(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewProvidersPanel(app)
	p.Refresh()

	var names []string
	itemCount := p.list.GetItemCount()
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

func TestProvidersPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewProvidersPanel(app)
	p.Refresh()
	p.list.SetCurrentItem(1)
	want := p.selectedProvider().Name()

	p.Refresh()
	if got := p.selectedProvider(); got == nil || got.Name() != want {
		t.Errorf("selection changed: got %v, want %q", got, want)
	}
}

func TestProvidersPanel_Keys_SwitchToMain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fire func(*ProvidersPanel)
	}{
		{"esc key", func(p *ProvidersPanel) { p.handleKey(escKey()) }},
		{"r key", func(p *ProvidersPanel) { p.handleKey(runeKey(KeyProviders)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewProvidersPanel(app)
			p.Refresh()
			tc.fire(p)
			if app.switchedTo != "main" {
				t.Errorf("expected switchedTo=main, got %q", app.switchedTo)
			}
		})
	}
}

func TestProvidersPanel_Keys_ShowModal(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewProvidersPanel(app)
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

func TestProvidersPanel_EmptyList_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  rune
	}{
		{"edit", KeyEdit},
		{"delete", KeyDelete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			app.registry = model.NewRegistry()
			p := NewProvidersPanel(app)
			p.Refresh()
			p.handleKey(runeKey(tc.key))
			if app.modal != nil {
				t.Errorf("key %q with empty list should not open a modal", string(tc.key))
			}
		})
	}
}

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
			p := model.NewProvider("test", tc.hasProjects, tc.hasDeepResearch)
			got := providerItemText(p)
			if tc.wantEqualName && got != "test" {
				t.Errorf("got %q, want %q", got, "test")
			}
			if !tc.wantEqualName && got == "test" {
				t.Error("expected capability tags to be appended")
			}
		})
	}
}

func TestBoolTag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input bool
	}{
		{"true produces non-empty tag", true},
		{"false produces non-empty tag", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := boolTag(tc.input); got == "" {
				t.Errorf("boolTag(%v): expected non-empty string", tc.input)
			}
		})
	}
}
