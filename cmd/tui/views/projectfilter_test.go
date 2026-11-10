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

func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func titleSuffix(t *testing.T, f *views.ProjectFilterState) string {
	t.Helper()
	suffix, err := f.TitleSuffix()
	if err != nil {
		t.Fatalf("TitleSuffix: %v", err)
	}
	return suffix
}

func TestProjectFilterState_Initial(t *testing.T) {
	t.Parallel()
	f := views.NewProjectFilterState()
	if !f.Value().IsAll() {
		t.Error("initial filter is not All")
	}
	if got := titleSuffix(t, f); got != "" {
		t.Errorf("TitleSuffix() = %q, want empty", got)
	}
}

func TestProjectFilterState_Cycle(t *testing.T) {
	t.Parallel()

	t.Run("with granted projects", func(t *testing.T) {
		t.Parallel()
		f := views.NewProjectFilterState()
		mustNoError(t, f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, filterProjects(t, "Beta", "Alpha")))

		want := []string{"", " (Alpha)", " (Beta)", " (no project)", ""}
		for i, suffix := range want {
			if got := titleSuffix(t, f); got != suffix {
				t.Errorf("step %d suffix = %q, want %q", i, got, suffix)
			}
			mustNoError(t, f.Cycle())
		}
	})

	t.Run("without an account", func(t *testing.T) {
		t.Parallel()
		f := views.NewProjectFilterState()
		mustNoError(t, f.SetAccount(model.AccountKey{}, false, nil))

		want := []string{"", " (no project)", ""}
		for i, suffix := range want {
			if got := titleSuffix(t, f); got != suffix {
				t.Errorf("step %d suffix = %q, want %q", i, got, suffix)
			}
			mustNoError(t, f.Cycle())
		}
	})

	t.Run("deduplicates by ID", func(t *testing.T) {
		t.Parallel()
		projects := filterProjects(t, "Alpha", "Beta")
		f := views.NewProjectFilterState()
		mustNoError(t, f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, append(projects, projects[0])))
		mustNoError(t, f.Cycle()) // Alpha
		if got := titleSuffix(t, f); got != " (Alpha)" {
			t.Errorf("first project suffix = %q, want (Alpha)", got)
		}
		mustNoError(t, f.Cycle()) // Beta
		if got := titleSuffix(t, f); got != " (Beta)" {
			t.Errorf("second project suffix = %q, want (Beta)", got)
		}
	})
}

func TestProjectFilterState_Resets(t *testing.T) {
	t.Parallel()

	t.Run("account change resets to All", func(t *testing.T) {
		t.Parallel()
		f := views.NewProjectFilterState()
		mustNoError(t, f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, filterProjects(t, "Alpha")))
		mustNoError(t, f.Cycle()) // Alpha
		mustNoError(t, f.SetAccount(model.AccountKey{Email: "bob@example.com", Vendor: "chatgpt"}, true, filterProjects(t, "Alpha")))
		if !f.Value().IsAll() {
			t.Error("filter did not reset to All after an account change")
		}
	})

	t.Run("lost grant resets to All", func(t *testing.T) {
		t.Parallel()
		projects := filterProjects(t, "Alpha", "Beta")
		f := views.NewProjectFilterState()
		key := model.AccountKey{Email: "alice@example.com", Vendor: "claude"}
		mustNoError(t, f.SetAccount(key, true, projects))
		mustNoError(t, f.Cycle()) // Alpha
		mustNoError(t, f.SetAccount(key, true, projects[1:]))
		if !f.Value().IsAll() {
			t.Error("filter did not reset to All after the project grant was lost")
		}
	})

	t.Run("kept grant keeps the value", func(t *testing.T) {
		t.Parallel()
		projects := filterProjects(t, "Alpha", "Beta", "Gamma")
		f := views.NewProjectFilterState()
		key := model.AccountKey{Email: "alice@example.com", Vendor: "claude"}
		mustNoError(t, f.SetAccount(key, true, projects[:2]))
		mustNoError(t, f.Cycle()) // Alpha
		mustNoError(t, f.SetAccount(key, true, projects))
		if id, ok := f.Value().ProjectID(); !ok || id != projects[0].ID() {
			t.Errorf("filter = %v, want project %d", f.Value(), projects[0].ID())
		}
	})

	t.Run("no account resets to All", func(t *testing.T) {
		t.Parallel()
		f := views.NewProjectFilterState()
		mustNoError(t, f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, filterProjects(t, "Alpha")))
		mustNoError(t, f.Cycle())
		mustNoError(t, f.SetAccount(model.AccountKey{}, false, nil))
		if !f.Value().IsAll() {
			t.Error("filter did not reset to All without an account")
		}
	})
}

func TestProjectFilterState_TitleSuffix(t *testing.T) {
	t.Parallel()
	f := views.NewProjectFilterState()
	mustNoError(t, f.SetAccount(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true, filterProjects(t, "[Alpha]")))
	mustNoError(t, f.Cycle())
	if got := titleSuffix(t, f); got != " ("+tview.Escape("[Alpha]")+")" {
		t.Errorf("TitleSuffix() = %q, want escaped brackets", got)
	}
	mustNoError(t, f.Cycle()) // None
	if got := titleSuffix(t, f); got != " (no project)" {
		t.Errorf("TitleSuffix() = %q, want (no project)", got)
	}
}
