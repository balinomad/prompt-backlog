package json_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/store/json"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// must fails the test immediately when err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// tempStore returns a Store and its data file path. The file lives in a
// per-test temp directory that is removed automatically when the test ends.
func tempStore(t *testing.T) (*json.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.json")
	return json.New(path), path
}

// seedRegistry builds a populated registry for round-trip tests.
//
//	Providers: "claude" (HasProjects=true), "chatgpt" (HasProjects=false)
//	Users:     alice@example.com / Alice, bob@example.com / Bob
//	Sessions:  alice → claude (with LastQuestion + NextPromptAt)
//	           bob   → chatgpt
//	Projects:  "Alpha" (attached: alice/claude, with Description+Instructions)
//	           "Beta"  (attached: alice/claude)
//	           "Orphan" (no sessions)
func seedRegistry(t *testing.T) model.Registry {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddProvider(model.NewProvider("chatgpt", false, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("alice@example.com"),
		reg.ProviderByName("claude"),
	)))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("bob@example.com"),
		reg.ProviderByName("chatgpt"),
	)))
	must(t, reg.AddProject(model.NewProject("Alpha")))
	must(t, reg.AttachProject("Alpha", reg.SessionAt("alice@example.com", "claude")))
	must(t, reg.AddProject(model.NewProject("Beta")))
	must(t, reg.AttachProject("Beta", reg.SessionAt("alice@example.com", "claude")))
	must(t, reg.AddProject(model.NewProject("Orphan")))

	// Enrich alice's session with a LastQuestion and a NextPromptAt.
	ts, err := time.Parse(time.RFC3339, "2026-06-01T09:00:00Z")
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	s := reg.SessionAt("alice@example.com", "claude")
	s.NextPromptAt = &ts
	lq := model.NewLastQuestion(reg.ProjectByName("Alpha"))
	lq.Topic = "Architecture review"
	lq.MidAnswer = true
	lq.NextPrompt = "continue with the diagram"
	s.LastQuestion = lq

	// Enrich Alpha with description and instructions.
	alpha := reg.ProjectByName("Alpha")
	alpha.Description = "Backend rewrite"
	alpha.Instructions = "Use Go interfaces"

	return reg
}

// ── Load ──────────────────────────────────────────────────────────────────────

func TestLoad_FileNotExist_ReturnsEmptyRegistry(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	reg, err := s.Load()
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if n := len(reg.AllProviders()); n != 0 {
		t.Errorf("expected empty registry, got %d providers", n)
	}
}

func TestLoad_InvalidData_ReturnsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content []byte
	}{
		{"empty file", []byte{}},
		{"invalid json", []byte("not json {{{")},
		{"session references unknown user", []byte(`{
			"providers": {"claude": {"has_projects": true, "has_deep_research": false}},
			"users": {},
			"sessions": {"ghost@example.com": {"claude": {}}},
			"projects": {}
		}`)},
		{"session references unknown provider", []byte(`{
			"providers": {},
			"users": {"alice@example.com": {"name": "Alice"}},
			"sessions": {"alice@example.com": {"ghost": {}}},
			"projects": {}
		}`)},
		{"project references unknown session", []byte(`{
			"providers": {"claude": {"has_projects": true, "has_deep_research": false}},
			"users": {"alice@example.com": {"name": "Alice"}},
			"sessions": {},
			"projects": {
				"Alpha": {
					"sessions": [{"user_email": "alice@example.com", "provider": "claude"}]
				}
			}
		}`)},
		{"project has partial session reference", []byte(`{
			"providers": {"claude": {"has_projects": true, "has_deep_research": false}},
			"users": {"alice@example.com": {"name": "Alice"}},
			"sessions": {"alice@example.com": {"claude": {}}},
			"projects": {
				"Alpha": {
					"sessions": [{"user_email": "alice@example.com"}]
				}
			}
		}`)},
		{"last question references unknown project", []byte(`{
			"providers": {"claude": {"has_projects": true, "has_deep_research": false}},
			"users": {"alice@example.com": {"name": "Alice"}},
			"sessions": {
				"alice@example.com": {
					"claude": {
						"last_question": {"project_name": "ghost", "mid_answer": false}
					}
				}
			},
			"projects": {}
		}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "registry.json")
			if err := os.WriteFile(path, tc.content, 0600); err != nil {
				t.Fatalf("write file: %v", err)
			}
			if _, err := json.New(path).Load(); err == nil {
				t.Errorf("expected error for %q, got none", tc.name)
			}
		})
	}
}

// ── Save ──────────────────────────────────────────────────────────────────────

func TestSave_CreatesFile(t *testing.T) {
	t.Parallel()
	s, path := tempStore(t)
	must(t, s.Save(model.NewRegistry()))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created after Save: %v", err)
	}
}

func TestSave_CreatesParentDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	s := json.New(filepath.Join(dir, "registry.json"))
	must(t, s.Save(model.NewRegistry()))
	if _, err := os.Stat(filepath.Join(dir, "registry.json")); err != nil {
		t.Fatalf("file not created in new directory: %v", err)
	}
}

func TestSave_NoTempFileLeft(t *testing.T) {
	t.Parallel()
	s, path := tempStore(t)
	must(t, s.Save(model.NewRegistry()))
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful Save")
	}
}

func TestSave_FileIsValidJSON(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	// A successful Load is the definitive JSON validity check.
	if _, err := s.Load(); err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}
}

// ── Round-trip ────────────────────────────────────────────────────────────────

func TestRoundTrip_Providers(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if n := len(loaded.AllProviders()); n != 2 {
		t.Errorf("providers: expected 2, got %d", n)
	}
	claude := loaded.ProviderByName("claude")
	if claude == nil {
		t.Fatal("provider 'claude' not found after round-trip")
	}
	if !claude.HasProjects {
		t.Error("claude.HasProjects should be true")
	}
	chatgpt := loaded.ProviderByName("chatgpt")
	if chatgpt == nil {
		t.Fatal("provider 'chatgpt' not found after round-trip")
	}
	if chatgpt.HasProjects {
		t.Error("chatgpt.HasProjects should be false")
	}
}

func TestRoundTrip_Users(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	alice := loaded.UserByEmail("alice@example.com")
	if alice == nil {
		t.Fatal("user alice not found after round-trip")
	}
	if alice.Name != "Alice" {
		t.Errorf("alice.Name: got %q, want %q", alice.Name, "Alice")
	}
}

func TestRoundTrip_Sessions(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sess := loaded.SessionAt("alice@example.com", "claude")
	if sess == nil {
		t.Fatal("session alice/claude not found after round-trip")
	}
	if sess.User() == nil || sess.User().Email() != "alice@example.com" {
		t.Error("session.User pointer not resolved correctly")
	}
	if sess.Provider() == nil || sess.Provider().Name() != "claude" {
		t.Error("session.Provider pointer not resolved correctly")
	}
}

func TestRoundTrip_NextPromptAt(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sess := loaded.SessionAt("alice@example.com", "claude")
	if sess.NextPromptAt == nil {
		t.Fatal("NextPromptAt should not be nil")
	}
	want := "2026-06-01T09:00:00Z"
	got := sess.NextPromptAt.UTC().Format(time.RFC3339)
	if got != want {
		t.Errorf("NextPromptAt: got %q, want %q", got, want)
	}
}

func TestRoundTrip_LastQuestion(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sess := loaded.SessionAt("alice@example.com", "claude")
	lq := sess.LastQuestion
	if lq == nil {
		t.Fatal("LastQuestion should not be nil")
	}
	if lq.Topic != "Architecture review" {
		t.Errorf("Topic: got %q, want %q", lq.Topic, "Architecture review")
	}
	if !lq.MidAnswer {
		t.Error("MidAnswer: expected true")
	}
	if lq.NextPrompt != "continue with the diagram" {
		t.Errorf("NextPrompt: got %q", lq.NextPrompt)
	}
}

func TestRoundTrip_LastQuestion_ProjectPointer(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sess := loaded.SessionAt("alice@example.com", "claude")
	lq := sess.LastQuestion
	if lq == nil {
		t.Fatal("LastQuestion should not be nil")
	}
	if lq.Project() == nil {
		t.Fatal("LastQuestion.Project should not be nil")
	}
	if lq.Project().Name() != "Alpha" {
		t.Errorf("LastQuestion.Project.Name: got %q, want Alpha", lq.Project().Name())
	}
	// Pointer must be the same object as the one in the projects map.
	if lq.Project() != loaded.ProjectByName("Alpha") {
		t.Error("LastQuestion.Project is not the same pointer as the registry project")
	}
}

func TestRoundTrip_Projects(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if n := len(loaded.AllProjects()); n != 3 {
		t.Errorf("projects: expected 3, got %d", n)
	}
	alpha := loaded.ProjectByName("Alpha")
	if alpha == nil {
		t.Fatal("project Alpha not found after round-trip")
	}
	if alpha.Description != "Backend rewrite" {
		t.Errorf("Alpha.Description: got %q", alpha.Description)
	}
	if alpha.Instructions != "Use Go interfaces" {
		t.Errorf("Alpha.Instructions: got %q", alpha.Instructions)
	}
}

func TestRoundTrip_Project_SessionPointers(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sessions := loaded.SessionsByProject("Alpha")
	if len(sessions) != 1 {
		t.Fatalf("Alpha should have 1 session after round-trip, got %d", len(sessions))
	}
	if sessions[0] != loaded.SessionAt("alice@example.com", "claude") {
		t.Error("Alpha session is not the same pointer as the registry session")
	}
}

func TestRoundTrip_Project_MultiSession(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddProvider(model.NewProvider("gemini", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("alice@example.com"),
		reg.ProviderByName("claude"),
	)))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("alice@example.com"),
		reg.ProviderByName("gemini"),
	)))
	must(t, reg.AddProject(model.NewProject("Alpha")))
	must(t, reg.AttachProject("Alpha", reg.SessionAt("alice@example.com", "claude")))
	must(t, reg.AttachProject("Alpha", reg.SessionAt("alice@example.com", "gemini")))

	s, _ := tempStore(t)
	must(t, s.Save(reg))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if n := len(loaded.SessionsByProject("Alpha")); n != 2 {
		t.Errorf("Alpha should have 2 sessions after round-trip, got %d", n)
	}
	for _, providerName := range []string{"claude", "gemini"} {
		sess := loaded.SessionAt("alice@example.com", providerName)
		projects := loaded.ProjectsBySession(sess)
		if len(projects) != 1 || projects[0].Name() != "Alpha" {
			t.Errorf("Alpha not found in ProjectsBySession for alice/%s after round-trip",
				providerName)
		}
	}
}

func TestRoundTrip_UnattachedProject(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	orphan := loaded.ProjectByName("Orphan")
	if orphan == nil {
		t.Fatal("project Orphan not found after round-trip")
	}
	if n := len(loaded.SessionsByProject("Orphan")); n != 0 {
		t.Errorf("Orphan should have no sessions after round-trip, got %d", n)
	}
}

func TestRoundTrip_SessionWithNoLastQuestion(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// bob/chatgpt has no LastQuestion in seedRegistry.
	sess := loaded.SessionAt("bob@example.com", "chatgpt")
	if sess == nil {
		t.Fatal("session bob/chatgpt not found")
	}
	if sess.LastQuestion != nil {
		t.Error("bob/chatgpt LastQuestion should be nil")
	}
}

func TestRoundTrip_EmptyRegistry(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(model.NewRegistry()))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if n := len(loaded.AllProviders()); n != 0 {
		t.Errorf("expected 0 providers, got %d", n)
	}
}

func TestRoundTrip_DualIndexIntact(t *testing.T) {
	t.Parallel()
	s, _ := tempStore(t)
	must(t, s.Save(seedRegistry(t)))
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Both indexes must return the same session pointer after Load.
	byUser := loaded.SessionsByUser("alice@example.com")
	byProvider := loaded.SessionsByProvider("claude")
	if len(byUser) != 1 || len(byProvider) != 1 {
		t.Fatalf("expected 1 session in each index, got byUser=%d byProvider=%d",
			len(byUser), len(byProvider))
	}
	if byUser[0] != byProvider[0] {
		t.Error("dual-index pointers diverge after round-trip")
	}
}

// ── Determinism ───────────────────────────────────────────────────────────────

func TestSave_ProjectSessionsOrderDeterministic(t *testing.T) {
	t.Parallel()
	// A project attached to two sessions must produce identical JSON on
	// repeated saves regardless of map iteration order.
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddProvider(model.NewProvider("gemini", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("alice@example.com"),
		reg.ProviderByName("claude"),
	)))
	must(t, reg.AddSession(model.NewSession(
		reg.UserByEmail("alice@example.com"),
		reg.ProviderByName("gemini"),
	)))
	must(t, reg.AddProject(model.NewProject("Alpha")))
	must(t, reg.AttachProject("Alpha", reg.SessionAt("alice@example.com", "claude")))
	must(t, reg.AttachProject("Alpha", reg.SessionAt("alice@example.com", "gemini")))

	s, path := tempStore(t)
	must(t, s.Save(reg))
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read first save: %v", err)
	}
	must(t, s.Save(reg))
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read second save: %v", err)
	}
	if string(first) != string(second) {
		t.Error("repeated saves of identical registry produced different JSON")
	}
}
