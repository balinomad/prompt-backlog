package views

import (
	"testing"

	"github.com/balinomad/llm-desk/internal/model"
)

// ── construction ──────────────────────────────────────────────────────────────

func TestNewDetailPanel(t *testing.T) {
	t.Parallel()
	if p := NewDetailPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil DetailPanel")
	}
}

// ── SetSession / Refresh ──────────────────────────────────────────────────────

func TestDetailPanel_SetSession_Nil(t *testing.T) {
	t.Parallel()
	p := NewDetailPanel(newMockApp(t))
	p.SetSession(nil)
	if p.session != nil {
		t.Error("session should be nil after SetSession(nil)")
	}
}

func TestDetailPanel_SetSession_ItemCount(t *testing.T) {
	t.Parallel()
	// Alice/claude has Alpha + Beta (2 projects) + separator + LastQuestion = 4 rows.
	app := newMockApp(t)
	p := NewDetailPanel(app)
	p.SetSession(app.registry.SessionAt("alice@example.com", "claude"))
	if got := p.projectsList.GetItemCount(); got != 4 {
		t.Errorf("expected 4 list items, got %d", got)
	}
}

func TestDetailPanel_Refresh_ClearsOnDeletedSession(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewDetailPanel(app)
	p.SetSession(app.registry.SessionAt("alice@example.com", "claude"))
	must(t, app.registry.RemoveSession("alice@example.com", "claude"))
	p.Refresh()
	if p.session != nil {
		t.Error("session should be nil after deletion from registry")
	}
	if p.projectsList.GetItemCount() != 0 {
		t.Error("list should be empty after session deleted")
	}
}

// ── selectedEntry ─────────────────────────────────────────────────────────────

func TestDetailPanel_SelectedEntry(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewDetailPanel(app)
	p.SetSession(app.registry.SessionAt("alice@example.com", "claude"))
	last := p.projectsList.GetItemCount() - 1

	cases := []struct {
		name      string
		index     int
		wantProj  bool
		wantLastQ bool
	}{
		{"first row is project", 0, true, false},
		{"last row is last question", last, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := NewDetailPanel(app)
			p.SetSession(app.registry.SessionAt("alice@example.com", "claude"))
			p.projectsList.SetCurrentItem(tc.index)
			e := p.selectedEntry()
			if (e.project != nil) != tc.wantProj {
				t.Errorf("wantProj=%v, got project=%v", tc.wantProj, e.project)
			}
			if e.isLastQ != tc.wantLastQ {
				t.Errorf("wantLastQ=%v, got %v", tc.wantLastQ, e.isLastQ)
			}
		})
	}
}

// ── keyboard handlers ─────────────────────────────────────────────────────────

func TestDetailPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		key       rune
		wantHints string
	}{
		{"help", KeyHelp, HintsHelpOverlay},
		{"quit", KeyQuit, HintsConfirmModal},
		{"new project", KeyNew, HintsForm},
		{"delete project", KeyDelete, HintsConfirmModal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := NewDetailPanel(app)
			p.SetSession(app.registry.SessionAt("alice@example.com", "claude"))
			p.projectsList.SetCurrentItem(0) // first project row
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

func TestDetailPanel_Delete_NilLastQuestion_NoModal(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := NewDetailPanel(app)
	s := app.registry.SessionAt("alice@example.com", "claude")
	s.LastQuestion = nil
	p.SetSession(s)

	last := p.projectsList.GetItemCount() - 1
	p.projectsList.SetCurrentItem(last)
	p.handleKey(runeKey(KeyDelete))
	if app.modal != nil {
		t.Error("delete on nil LastQuestion should not open modal")
	}
}

func TestDetailPanel_Toggle_MidAnswer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		initialMid     bool
		initialLQIsNil bool
		wantMid        bool
	}{
		{"nil lq creates with mid=true", false, true, true},
		{"true flips to false", true, false, false},
		{"false flips to true", false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			s := app.registry.SessionAt("alice@example.com", "claude")
			if tc.initialLQIsNil {
				s.LastQuestion = nil
			} else {
				lq := model.NewLastQuestion(nil)
				lq.MidAnswer = tc.initialMid
				s.LastQuestion = lq
			}

			p := NewDetailPanel(app)
			p.SetSession(s)
			last := p.projectsList.GetItemCount() - 1
			p.projectsList.SetCurrentItem(last)
			p.handleKey(runeKey(KeyToggle))

			updated := app.registry.SessionAt("alice@example.com", "claude")
			if updated.LastQuestion == nil {
				t.Fatal("LastQuestion should exist after toggle")
			}
			if updated.LastQuestion.MidAnswer != tc.wantMid {
				t.Errorf("MidAnswer: got %v, want %v", updated.LastQuestion.MidAnswer, tc.wantMid)
			}
		})
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func TestProjectItemText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		description string
		wantEqual   string // exact match expected when non-empty
		wantNotEq   string // must differ from this when non-empty
	}{
		{"no description returns name", "", "Alpha", ""},
		{"description appended", "Backend rewrite", "", "Alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := model.NewProject("Alpha")
			p.Description = tc.description
			got := projectItemText(p)
			if tc.wantEqual != "" && got != tc.wantEqual {
				t.Errorf("got %q, want %q", got, tc.wantEqual)
			}
			if tc.wantNotEq != "" && got == tc.wantNotEq {
				t.Errorf("expected text to differ from %q", tc.wantNotEq)
			}
		})
	}
}

func TestLastQuestionItemText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		lq        *model.LastQuestion
		wantEmpty bool
	}{
		{"nil produces placeholder", nil, false},
		{"topic only", func() *model.LastQuestion {
			lq := model.NewLastQuestion(nil)
			lq.Topic = "API design"
			return lq
		}(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := lastQuestionItemText(tc.lq)
			if tc.wantEmpty && got != "" {
				t.Errorf("expected empty, got %q", got)
			}
			if !tc.wantEmpty && got == "" {
				t.Error("expected non-empty text")
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"shorter than max unchanged", "hi", 10, "hi"},
		{"exactly max unchanged", "hello", 5, "hello"},
		{"longer truncated to max runes", "hello world", 6, "hello…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncate(tc.input, tc.max)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if len([]rune(got)) > tc.max {
				t.Errorf("result longer than max: %d > %d", len([]rune(got)), tc.max)
			}
		})
	}
}
