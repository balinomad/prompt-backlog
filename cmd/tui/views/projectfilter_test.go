package views_test

import (
	"testing"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/internal/model"

	"github.com/rivo/tview"
)

func filterProjects(t *testing.T, names ...string) []*model.Project {
	t.Helper()
	reg := model.NewRegistry()
	for _, name := range names {
		if err := reg.AddProject(model.NewProject(name, "", "")); err != nil {
			t.Fatalf("AddProject(%q): %v", name, err)
		}
	}
	return reg.Projects()
}

func TestProjectFilterState_Initial(t *testing.T) {
	t.Parallel()
	f := views.NewProjectFilterState()
	if !f.Value().IsAll() {
		t.Error("initial filter is not All")
	}
	if got := f.TitleSuffix(); got != "" {
		t.Errorf("TitleSuffix() = %q, want empty", got)
	}
}

func TestProjectFilterState_Cycle(t *testing.T) {
	t.Parallel()

	t.Run("with granted projects", func(t *testing.T) {
		t.Parallel()
		f := views.NewProjectFilterState()
		f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, filterProjects(t, "Beta", "Alpha"))

		want := []string{"", " (Alpha)", " (Beta)", " (no project)", ""}
		for i, suffix := range want {
			if got := f.TitleSuffix(); got != suffix {
				t.Errorf("step %d suffix = %q, want %q", i, got, suffix)
			}
			f.Cycle()
		}
	})

	t.Run("without an account", func(t *testing.T) {
		t.Parallel()
		f := views.NewProjectFilterState()
		f.SetAccount(model.AccountKey{}, false, nil)

		want := []string{"", " (no project)", ""}
		for i, suffix := range want {
			if got := f.TitleSuffix(); got != suffix {
				t.Errorf("step %d suffix = %q, want %q", i, got, suffix)
			}
			f.Cycle()
		}
	})

	t.Run("deduplicates by ID", func(t *testing.T) {
		t.Parallel()
		projects := filterProjects(t, "Alpha", "Beta")
		f := views.NewProjectFilterState()
		f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, append(projects, projects[0]))
		f.Cycle() // Alpha
		if got := f.TitleSuffix(); got != " (Alpha)" {
			t.Errorf("first project suffix = %q, want (Alpha)", got)
		}
		f.Cycle() // Beta
		if got := f.TitleSuffix(); got != " (Beta)" {
			t.Errorf("second project suffix = %q, want (Beta)", got)
		}
	})
}

func TestProjectFilterState_Resets(t *testing.T) {
	t.Parallel()

	t.Run("account change resets to All", func(t *testing.T) {
		t.Parallel()
		f := views.NewProjectFilterState()
		f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, filterProjects(t, "Alpha"))
		f.Cycle() // Alpha
		f.SetAccount(model.AccountKey{Email: "bob@example.com", Vendor: "chatgpt"}, true, filterProjects(t, "Alpha"))
		if !f.Value().IsAll() {
			t.Error("filter did not reset to All after an account change")
		}
	})

	t.Run("lost grant resets to All", func(t *testing.T) {
		t.Parallel()
		projects := filterProjects(t, "Alpha", "Beta")
		f := views.NewProjectFilterState()
		key := model.AccountKey{Email: "alice@example.com", Vendor: "claude"}
		f.SetAccount(key, true, projects)
		f.Cycle() // Alpha
		f.SetAccount(key, true, projects[1:])
		if !f.Value().IsAll() {
			t.Error("filter did not reset to All after the project grant was lost")
		}
	})

	t.Run("kept grant keeps the value", func(t *testing.T) {
		t.Parallel()
		projects := filterProjects(t, "Alpha", "Beta", "Gamma")
		f := views.NewProjectFilterState()
		key := model.AccountKey{Email: "alice@example.com", Vendor: "claude"}
		f.SetAccount(key, true, projects[:2])
		f.Cycle() // Alpha
		f.SetAccount(key, true, projects)
		if id, ok := f.Value().ProjectID(); !ok || id != projects[0].ID() {
			t.Errorf("filter = %v, want project %d", f.Value(), projects[0].ID())
		}
	})

	t.Run("no account resets to All", func(t *testing.T) {
		t.Parallel()
		f := views.NewProjectFilterState()
		f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, filterProjects(t, "Alpha"))
		f.Cycle()
		f.SetAccount(model.AccountKey{}, false, nil)
		if !f.Value().IsAll() {
			t.Error("filter did not reset to All without an account")
		}
	})
}

func TestProjectFilterState_TitleSuffix(t *testing.T) {
	t.Parallel()
	f := views.NewProjectFilterState()
	f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, filterProjects(t, "[Alpha]"))
	f.Cycle()
	if got := f.TitleSuffix(); got != " ("+tview.Escape("[Alpha]")+")" {
		t.Errorf("TitleSuffix() = %q, want escaped brackets", got)
	}
	f.Cycle() // None
	if got := f.TitleSuffix(); got != " (no project)" {
		t.Errorf("TitleSuffix() = %q, want (no project)", got)
	}
}
