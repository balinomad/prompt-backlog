package model_test

import (
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
)

// ── NewRegistry ───────────────────────────────────────────────────────────────

func TestNewRegistry_Empty(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	if n := len(reg.AllProviders()); n != 0 {
		t.Errorf("AllProviders: expected 0, got %d", n)
	}
	if n := len(reg.AllUsers()); n != 0 {
		t.Errorf("AllUsers: expected 0, got %d", n)
	}
	if n := len(reg.AllSessions()); n != 0 {
		t.Errorf("AllSessions: expected 0, got %d", n)
	}
	if n := len(reg.AllProjects()); n != 0 {
		t.Errorf("AllProjects: expected 0, got %d", n)
	}
}

func TestNewRegistry_AcceptsFirstProvider(t *testing.T) {
	t.Parallel()
	// Verifies that NewRegistry initialises internal maps so the first write
	// does not panic.
	reg := model.NewRegistry()
	if err := reg.AddProvider(model.NewProvider("claude", true, false)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Provider ──────────────────────────────────────────────────────────────────

func TestNewProvider(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		providerName    string
		hasProjects     bool
		hasDeepResearch bool
	}{
		{"all capabilities set", "claude", true, false},
		{"no capabilities", "basic", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := model.NewProvider(tc.providerName, tc.hasProjects, tc.hasDeepResearch)
			if p.Name() != tc.providerName {
				t.Errorf("Name: got %q, want %q", p.Name(), tc.providerName)
			}
			if p.HasProjects != tc.hasProjects {
				t.Errorf("HasProjects: got %v, want %v", p.HasProjects, tc.hasProjects)
			}
			if p.HasDeepResearch != tc.hasDeepResearch {
				t.Errorf("HasDeepResearch: got %v, want %v", p.HasDeepResearch, tc.hasDeepResearch)
			}
		})
	}
}

// ── User ──────────────────────────────────────────────────────────────────────

func TestNewUser_Fields(t *testing.T) {
	t.Parallel()
	u := model.NewUser("alice@example.com", "Alice")
	if u.Email() != "alice@example.com" {
		t.Errorf("Email: got %q, want %q", u.Email(), "alice@example.com")
	}
	if u.Name != "Alice" {
		t.Errorf("Name: got %q, want %q", u.Name, "Alice")
	}
}

func TestUser_Email_Immutable(t *testing.T) {
	t.Parallel()
	u := model.NewUser("alice@example.com", "Alice")
	if u.Email() != "alice@example.com" {
		t.Errorf("Email: got %q, want %q", u.Email(), "alice@example.com")
	}
}

func TestUser_Name_Mutable(t *testing.T) {
	t.Parallel()
	u := model.NewUser("alice@example.com", "Alice")
	u.Name = "Alice Smith"
	if u.Name != "Alice Smith" {
		t.Errorf("Name: got %q, want %q", u.Name, "Alice Smith")
	}
}

// ── Session ───────────────────────────────────────────────────────────────────

func TestNewSession_Fields(t *testing.T) {
	t.Parallel()
	u := model.NewUser("alice@example.com", "Alice")
	p := model.NewProvider("claude", true, false)
	s := model.NewSession(u, p)
	if s.User() != u {
		t.Error("User: pointer mismatch")
	}
	if s.Provider() != p {
		t.Error("Provider: pointer mismatch")
	}
	if s.LastQuestion != nil {
		t.Error("LastQuestion: expected nil")
	}
	if s.NextPromptAt != nil {
		t.Error("NextPromptAt: expected nil")
	}
}

func TestSession_NextPromptAt_Mutable(t *testing.T) {
	t.Parallel()
	u := model.NewUser("alice@example.com", "Alice")
	p := model.NewProvider("claude", true, false)
	s := model.NewSession(u, p)
	ts, err := model.ParseTime("2026-06-01 09:00")
	if err != nil {
		t.Fatalf("ParseTime: %v", err)
	}
	s.NextPromptAt = &ts
	if s.NextPromptAt == nil || !s.NextPromptAt.Equal(ts) {
		t.Error("NextPromptAt not stored correctly")
	}
}

func TestSession_LastQuestion_Mutable(t *testing.T) {
	t.Parallel()
	u := model.NewUser("alice@example.com", "Alice")
	p := model.NewProvider("claude", true, false)
	s := model.NewSession(u, p)
	lq := model.NewLastQuestion(nil)
	lq.Topic = "API design"
	s.LastQuestion = lq
	if s.LastQuestion == nil || s.LastQuestion.Topic != "API design" {
		t.Error("LastQuestion not stored correctly")
	}
	s.LastQuestion = nil
	if s.LastQuestion != nil {
		t.Error("LastQuestion should be nil after clearing")
	}
}

// ── Project ───────────────────────────────────────────────────────────────────

func TestNewProject_Fields(t *testing.T) {
	t.Parallel()
	p := model.NewProject("Alpha")
	if p.Name() != "Alpha" {
		t.Errorf("Name: got %q, want %q", p.Name(), "Alpha")
	}
}

func TestProject_Fields_Mutable(t *testing.T) {
	t.Parallel()
	p := model.NewProject("Alpha")
	p.Description = "Backend rewrite"
	p.Instructions = "Use Go interfaces"
	if p.Description != "Backend rewrite" {
		t.Errorf("Description: got %q", p.Description)
	}
	if p.Instructions != "Use Go interfaces" {
		t.Errorf("Instructions: got %q", p.Instructions)
	}
}

// ── LastQuestion ──────────────────────────────────────────────────────────────

func TestNewLastQuestion(t *testing.T) {
	t.Parallel()
	proj := model.NewProject("Alpha")
	cases := []struct {
		name        string
		project     *model.Project
		wantProject *model.Project
	}{
		{"nil project", nil, nil},
		{"with project", proj, proj},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lq := model.NewLastQuestion(tc.project)
			if lq.Project() != tc.wantProject {
				t.Errorf("Project: got %v, want %v", lq.Project(), tc.wantProject)
			}
			if lq.MidAnswer {
				t.Error("MidAnswer: expected false by default")
			}
			if lq.Topic != "" {
				t.Errorf("Topic: expected empty, got %q", lq.Topic)
			}
			if lq.NextPrompt != "" {
				t.Errorf("NextPrompt: expected empty, got %q", lq.NextPrompt)
			}
		})
	}
}

func TestLastQuestion_Fields_Mutable(t *testing.T) {
	t.Parallel()
	lq := model.NewLastQuestion(nil)
	lq.Topic = "Architecture"
	lq.MidAnswer = true
	lq.NextPrompt = "continue"
	if lq.Topic != "Architecture" {
		t.Errorf("Topic: got %q", lq.Topic)
	}
	if !lq.MidAnswer {
		t.Error("MidAnswer: expected true")
	}
	if lq.NextPrompt != "continue" {
		t.Errorf("NextPrompt: got %q", lq.NextPrompt)
	}
}

// ── ParseTime / FormatTime ────────────────────────────────────────────────────

func TestParseTime_ValidInputs(t *testing.T) {
	t.Parallel()
	cases := []string{
		"2026-05-18 09:00",
		"2026-12-31 23:59",
		"2026-01-01 00:00",
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			if _, err := model.ParseTime(s); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseTime_InvalidInputs(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"not-a-date",
		"2026/05/18 09:00",
		"18-05-2026 09:00",
		"2026-05-18",
		"09:00",
		"2026-13-01 00:00",
	}
	for _, s := range cases {
		t.Run(`"`+s+`"`, func(t *testing.T) {
			t.Parallel()
			if _, err := model.ParseTime(s); err == nil {
				t.Errorf("expected error for %q, got none", s)
			}
		})
	}
}

func TestFormatTime_RoundTrip(t *testing.T) {
	t.Parallel()
	original := "2026-05-20 14:30"
	parsed, err := model.ParseTime(original)
	if err != nil {
		t.Fatalf("ParseTime: %v", err)
	}
	if got := model.FormatTime(parsed); got != original {
		t.Errorf("round-trip: got %q, want %q", got, original)
	}
}

func TestFormatTime_ParseTime_Symmetric(t *testing.T) {
	t.Parallel()
	now := time.Now().Truncate(time.Minute)
	formatted := model.FormatTime(now)
	parsed, err := model.ParseTime(formatted)
	if err != nil {
		t.Fatalf("ParseTime failed on FormatTime output %q: %v", formatted, err)
	}
	if !parsed.Equal(now) {
		t.Errorf("time not preserved: got %v, want %v", parsed, now)
	}
}
