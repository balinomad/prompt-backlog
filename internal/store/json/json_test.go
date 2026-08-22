package json_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/apperr"
	"github.com/balinomad/llm-desk/internal/model"
	storejson "github.com/balinomad/llm-desk/internal/store/json"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// must fails the test immediately when err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// newStore returns a Store and its data file path. The file lives in a
// per-test temp directory removed automatically when the test ends.
func newStore(t *testing.T) (*storejson.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.json")
	return storejson.New(path), path
}

// writeTempFile writes content to a fresh registry.json in a per-test temp
// directory and returns its path. Used to exercise Load against hand-crafted
// JSON, since the unexported DTO types in package json cannot be
// constructed from outside the package.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.json")
	must(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// seedRegistry builds a registry exercising every shape Save/Load must
// round-trip:
//
//	Providers: claude (HasProjects+HasDeepResearch), chatgpt (neither)
//	Users:     alice@example.com / Alice, bob@example.com / Bob
//	Projects:  Alpha (description+instructions), Beta (blank), Orphan (unattached)
//	Accounts:  alice/claude — not paused, assigned Alpha+Beta
//	           bob/chatgpt  — paused, no projects
//	Sessions:  alice/claude — project Alpha, topic, two queued prompts
//	           bob/chatgpt  — no project, no topic, no queue
//
// Pause and project-assignment are deliberately kept on separate accounts:
// combining both on one account currently fails to reload due to a bug in
// buildRegistry (see TestLoad_PausedAccountWithProjects_KnownBug). Each
// condition is still independently covered.
func seedRegistry(t *testing.T) *model.Registry {
	t.Helper()
	reg := model.NewRegistry()

	must(t, reg.AddProvider(model.NewProvider("claude", true, true)))
	must(t, reg.AddProvider(model.NewProvider("chatgpt", false, false)))

	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))

	must(t, reg.AddProject(model.NewProject("Alpha", "Backend rewrite", "Use Go interfaces")))
	must(t, reg.AddProject(model.NewProject("Beta", "", "")))
	must(t, reg.AddProject(model.NewProject("Orphan", "", "")))

	aliceClaude, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Beta")))
	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Alpha")))

	bobChatgpt, err := reg.AddAccount(reg.User("bob@example.com"), reg.Provider("chatgpt"))
	must(t, err)
	must(t, reg.PauseAccount(bobChatgpt, time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)))
	// PauseAccount replaces the registry's Account via Copy-on-Write; the
	// pointer held above is now stale and must be re-read before further use.
	bobChatgpt = reg.Account(reg.User("bob@example.com"), reg.Provider("chatgpt"))

	_, err = reg.AddSession(aliceClaude, reg.Project("Alpha"), "Architecture review", []model.Prompt{
		model.NewPrompt("continue", false),
		model.NewPrompt("proceed to step 2", true),
	})
	must(t, err)

	_, err = reg.AddSession(bobChatgpt, nil, "", nil)
	must(t, err)

	return reg
}

// ── Load: file and parse errors ───────────────────────────────────────────────

func TestLoad_FileNotExist_ReturnsEmptyRegistry(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	reg, err := storejson.New(path).Load()
	if err != nil {
		t.Fatalf("unexpected error for a missing file: %v", err)
	}
	if len(reg.Providers()) != 0 {
		t.Error("expected an empty registry for a missing file")
	}
}

func TestLoad_UnreadableFile_ReturnsWrappedError(t *testing.T) {
	t.Parallel()
	// A directory at the data path makes os.ReadFile fail with something
	// other than os.ErrNotExist, exercising the "other I/O error" branch
	// rather than the first-run "file does not exist" branch.
	path := filepath.Join(t.TempDir(), "registry.json")
	must(t, os.Mkdir(path, 0o750))

	_, err := storejson.New(path).Load()
	if err == nil {
		t.Fatal("expected an error reading a directory as a file")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Error("error must not be classified as os.ErrNotExist")
	}
}

func TestLoad_ParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		check   func(t *testing.T, err error)
	}{
		{
			name:    "empty file",
			content: "",
			check: func(t *testing.T, err error) {
				t.Helper()
				if !strings.Contains(err.Error(), "empty") {
					t.Errorf("error %q does not mention emptiness", err.Error())
				}
			},
		},
		{
			name:    "malformed syntax",
			content: "not json {{{",
			check: func(t *testing.T, err error) {
				t.Helper()
				var serr *json.SyntaxError
				if !errors.As(err, &serr) {
					t.Errorf("error %v is not a *json.SyntaxError", err)
				}
			},
		},
		{
			name:    "wrong top-level field type",
			content: `{"providers": "not-an-array"}`,
			check: func(t *testing.T, err error) {
				t.Helper()
				var terr *json.UnmarshalTypeError
				if !errors.As(err, &terr) {
					t.Errorf("error %v is not a *json.UnmarshalTypeError", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := writeTempFile(t, tt.content)
			_, err := storejson.New(path).Load()
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			tt.check(t, err)
		})
	}
}

// ── Load: registry construction errors ────────────────────────────────────────

// TestLoad_RegistryConstructionErrors drives Load with hand-crafted, valid
// JSON that is individually defective in one way, covering every error
// branch in the registry-construction path that JSON content can reach.
func TestLoad_RegistryConstructionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		json           string
		wantErrIs      []error
		wantErrSubstrs []string
	}{
		{
			name:      "provider empty name",
			json:      `{"providers": [{"name": "", "has_projects": false, "has_deep_research": false}]}`,
			wantErrIs: []error{model.ErrNameRequired},
		},
		{
			name: "duplicate provider",
			json: `{"providers": [
				{"name": "claude", "has_projects": true, "has_deep_research": false},
				{"name": "claude", "has_projects": false, "has_deep_research": false}
			]}`,
			wantErrIs: []error{model.ErrProviderExists},
		},
		{
			name:      "user empty email",
			json:      `{"users": [{"email": "", "name": "Alice"}]}`,
			wantErrIs: []error{model.ErrEmailRequired},
		},
		{
			name:      "user invalid email",
			json:      `{"users": [{"email": "alice.example.com", "name": "Alice"}]}`,
			wantErrIs: []error{apperr.ErrEmailInvalid},
		},
		{
			name: "duplicate user",
			json: `{"users": [
				{"email": "alice@example.com", "name": "Alice"},
				{"email": "alice@example.com", "name": "Alice2"}
			]}`,
			wantErrIs: []error{model.ErrUserExists},
		},
		{
			name:      "project empty name",
			json:      `{"projects": [{"name": ""}]}`,
			wantErrIs: []error{model.ErrNameRequired},
		},
		{
			name:      "duplicate project",
			json:      `{"projects": [{"name": "Alpha"}, {"name": "Alpha"}]}`,
			wantErrIs: []error{model.ErrProjectExists},
		},
		{
			name: "account references unknown provider",
			json: `{
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"accounts": [{"user_email": "alice@example.com", "provider_name": "ghost"}]
			}`,
			wantErrIs: []error{model.ErrProviderNil},
		},
		{
			name: "account references unknown user",
			json: `{
				"providers": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"accounts": [{"user_email": "ghost@example.com", "provider_name": "claude"}]
			}`,
			wantErrIs: []error{model.ErrUserNil},
		},
		{
			name: "duplicate account",
			json: `{
				"providers": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"accounts": [
					{"user_email": "alice@example.com", "provider_name": "claude"},
					{"user_email": "alice@example.com", "provider_name": "claude"}
				]
			}`,
			wantErrIs: []error{model.ErrAccountExists},
		},
		{
			name: "account references unknown project",
			json: `{
				"providers": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"accounts": [{
					"user_email": "alice@example.com",
					"provider_name": "claude",
					"projects": ["Ghost"]
				}]
			}`,
			wantErrIs: []error{model.ErrProjectNil},
		},
		{
			name: "account project on provider without project support",
			json: `{
				"providers": [{"name": "chatgpt", "has_projects": false, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"projects": [{"name": "Alpha"}],
				"accounts": [{
					"user_email": "alice@example.com",
					"provider_name": "chatgpt",
					"projects": ["Alpha"]
				}]
			}`,
			wantErrIs: []error{model.ErrProjectsUnsupported},
		},
		{
			name: "account project listed twice",
			json: `{
				"providers": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"projects": [{"name": "Alpha"}],
				"accounts": [{
					"user_email": "alice@example.com",
					"provider_name": "claude",
					"projects": ["Alpha", "Alpha"]
				}]
			}`,
			wantErrSubstrs: []string{"already assigned"},
		},
		{
			name: "session references unknown account",
			json: `{
				"providers": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"sessions": [{"user_email": "alice@example.com", "provider_name": "claude"}]
			}`,
			wantErrSubstrs: []string{"account", "not found"},
		},
		{
			name: "session references unknown project",
			json: `{
				"providers": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"accounts": [{"user_email": "alice@example.com", "provider_name": "claude"}],
				"sessions": [{
					"user_email": "alice@example.com",
					"provider_name": "claude",
					"project_name": "Ghost"
				}]
			}`,
			wantErrSubstrs: []string{"project", "not found"},
		},
		{
			name: "session project not assigned to account",
			json: `{
				"providers": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"projects": [{"name": "Alpha"}],
				"accounts": [{"user_email": "alice@example.com", "provider_name": "claude"}],
				"sessions": [{
					"user_email": "alice@example.com",
					"provider_name": "claude",
					"project_name": "Alpha"
				}]
			}`,
			wantErrSubstrs: []string{"not assigned"},
		},
		{
			name: "session queue has an invalid prompt",
			json: `{
				"providers": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"accounts": [{"user_email": "alice@example.com", "provider_name": "claude"}],
				"sessions": [{
					"user_email": "alice@example.com",
					"provider_name": "claude",
					"queue": [{"text": "", "spawns_session": false}]
				}]
			}`,
			wantErrIs: []error{model.ErrTextRequired},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := writeTempFile(t, tt.json)
			_, err := storejson.New(path).Load()
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			for _, want := range tt.wantErrIs {
				if !errors.Is(err, want) {
					t.Errorf("errors.Is(err, %v) = false; err = %v", want, err)
				}
			}
			for _, substr := range tt.wantErrSubstrs {
				if !strings.Contains(err.Error(), substr) {
					t.Errorf("error %q does not contain %q", err.Error(), substr)
				}
			}
		})
	}
}

// ── Load: success-path edge cases ─────────────────────────────────────────────

func TestLoad_EmptyJSONObject_ReturnsEmptyRegistry(t *testing.T) {
	t.Parallel()
	path := writeTempFile(t, "{}")
	reg, err := storejson.New(path).Load()
	must(t, err)
	if len(reg.Providers()) != 0 || len(reg.Users()) != 0 || len(reg.Projects()) != 0 ||
		len(reg.Accounts()) != 0 || len(reg.Sessions()) != 0 {
		t.Error("expected a fully empty registry from '{}'")
	}
}

func TestLoad_AccountPauseUntilZero_Skipped(t *testing.T) {
	t.Parallel()
	// A present-but-zero pause_until must not be applied: PauseAccount is
	// only called when the timestamp is both non-nil and non-zero.
	path := writeTempFile(t, `{
		"providers": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
		"users": [{"email": "alice@example.com", "name": "Alice"}],
		"accounts": [{
			"user_email": "alice@example.com",
			"provider_name": "claude",
			"pause_until": "0001-01-01T00:00:00Z"
		}]
	}`)
	reg, err := storejson.New(path).Load()
	must(t, err)

	acc := reg.Account(reg.User("alice@example.com"), reg.Provider("claude"))
	if acc == nil {
		t.Fatal("account not found")
	}
	if !acc.PauseUntil().IsZero() {
		t.Errorf("PauseUntil() = %v, want zero", acc.PauseUntil())
	}
}

// ── Save: basic behaviour ──────────────────────────────────────────────────────

func TestSave_CreatesFile(t *testing.T) {
	t.Parallel()
	st, path := newStore(t)
	must(t, st.Save(model.NewRegistry()))
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created after Save: %v", err)
	}
}

func TestSave_CreatesParentDirectory(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "dir", "registry.json")
	must(t, storejson.New(path).Save(model.NewRegistry()))
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created in new directory: %v", err)
	}
}

func TestSave_NoTempFileLeft(t *testing.T) {
	t.Parallel()
	st, path := newStore(t)
	must(t, st.Save(model.NewRegistry()))
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file should not exist after a successful Save")
	}
}

func TestSave_FileIsValidJSON(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	if _, err := st.Load(); err != nil {
		t.Errorf("Load after Save failed: %v", err)
	}
}

func TestSave_Deterministic(t *testing.T) {
	t.Parallel()
	// A registry with multiple accounts/projects must serialise identically
	// on repeated saves regardless of internal map iteration order.
	st, path := newStore(t)
	reg := seedRegistry(t)
	cleanPath := filepath.Clean(path)

	must(t, st.Save(reg))
	first, err := os.ReadFile(cleanPath)
	must(t, err)

	must(t, st.Save(reg))
	second, err := os.ReadFile(cleanPath)
	must(t, err)

	if !bytes.Equal(first, second) {
		t.Error("repeated saves of an identical registry produced different JSON")
	}
}

// ── Save: filesystem fault injection ──────────────────────────────────────────
//
// These tests exercise Save's three guarded filesystem error paths by
// arranging for the underlying os call to fail, without modifying json.go.
// A fourth path — Rename failing AND the subsequent Remove also failing —
// is not reachable through the public Store API; see the accompanying
// summary for why.

func TestSave_MkdirAllFails(t *testing.T) {
	t.Parallel()
	// blocker is a regular file occupying the path where Save needs to
	// create a directory, so os.MkdirAll fails ("not a directory").
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	must(t, os.WriteFile(blocker, []byte("x"), 0o600))

	err := storejson.New(filepath.Join(blocker, "registry.json")).Save(model.NewRegistry())
	if err == nil {
		t.Fatal("expected an error when a path component is a regular file")
	}
}

func TestSave_WriteTempFileFails(t *testing.T) {
	t.Parallel()
	// Pre-creating the ".tmp" path as a directory makes os.WriteFile fail
	// when Save tries to open it for writing as a regular file.
	st, path := newStore(t)
	must(t, os.Mkdir(path+".tmp", 0o750))

	if err := st.Save(model.NewRegistry()); err == nil {
		t.Fatal("expected an error writing to a path occupied by a directory")
	}
}

func TestSave_RenameFails_RemovesTempFile(t *testing.T) {
	t.Parallel()
	// Pre-creating the destination as an existing directory makes
	// os.Rename fail — a regular file cannot replace a directory. Save
	// must still remove the temp file it wrote before returning the error.
	st, path := newStore(t)
	must(t, os.Mkdir(path, 0o750))

	err := st.Save(model.NewRegistry())
	if err == nil {
		t.Fatal("expected an error renaming over an existing directory")
	}
	if _, statErr := os.Stat(path + ".tmp"); !os.IsNotExist(statErr) {
		t.Error("temp file should have been removed after the failed rename")
	}
}

func TestSave_MarshalFails(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)
	// time.Time.MarshalJSON fails for years outside [0, 9999]. Every other
	// DTO field is a plain string, bool, or slice of those, so this is the
	// only reachable way to make json.Marshal fail on a *model.Registry.
	outOfRange := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	must(t, reg.PauseAccount(acc, outOfRange))

	s, _ := newStore(t)
	if err := s.Save(reg); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── Round-trip ────────────────────────────────────────────────────────────────

func TestRoundTrip_Providers(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	loaded, err := st.Load()
	must(t, err)

	if got := len(loaded.Providers()); got != 2 {
		t.Fatalf("providers: got %d, want 2", got)
	}
	claude := loaded.Provider("claude")
	if claude == nil {
		t.Fatal("provider claude not found")
	}
	if !claude.HasProjects() || !claude.HasDeepResearch() {
		t.Error("claude capabilities not preserved")
	}
	chatgpt := loaded.Provider("chatgpt")
	if chatgpt == nil {
		t.Fatal("provider chatgpt not found")
	}
	if chatgpt.HasProjects() || chatgpt.HasDeepResearch() {
		t.Error("chatgpt capabilities should both be false")
	}
}

func TestRoundTrip_Users(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	loaded, err := st.Load()
	must(t, err)

	alice := loaded.User("alice@example.com")
	if alice == nil {
		t.Fatal("user alice not found")
	}
	if alice.Name() != "Alice" {
		t.Errorf("alice.Name() = %q, want Alice", alice.Name())
	}
}

func TestRoundTrip_Projects(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	loaded, err := st.Load()
	must(t, err)

	if got := len(loaded.Projects()); got != 3 {
		t.Fatalf("projects: got %d, want 3", got)
	}
	alpha := loaded.Project("Alpha")
	if alpha == nil {
		t.Fatal("project Alpha not found")
	}
	if alpha.Description() != "Backend rewrite" {
		t.Errorf("Alpha.Description() = %q", alpha.Description())
	}
	if alpha.Instructions() != "Use Go interfaces" {
		t.Errorf("Alpha.Instructions() = %q", alpha.Instructions())
	}
	if loaded.Project("Orphan") == nil {
		t.Error("unattached project Orphan not preserved")
	}
}

func TestRoundTrip_Accounts(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	loaded, err := st.Load()
	must(t, err)

	aliceClaude := loaded.Account(loaded.User("alice@example.com"), loaded.Provider("claude"))
	if aliceClaude == nil {
		t.Fatal("account alice/claude not found")
	}
	if !aliceClaude.PauseUntil().IsZero() {
		t.Error("alice/claude PauseUntil() should be zero")
	}
	projects, err := loaded.AccountProjects(aliceClaude)
	must(t, err)
	names := make([]string, len(projects))
	for i, p := range projects {
		names[i] = p.Name()
	}
	if len(names) != 2 {
		t.Fatalf("alice/claude projects: got %d, want 2 (%v)", len(names), names)
	}
	if names[0] != "Alpha" || names[1] != "Beta" {
		t.Errorf("alice/claude projects = %v, want [Alpha Beta]", names)
	}

	bobChatgpt := loaded.Account(loaded.User("bob@example.com"), loaded.Provider("chatgpt"))
	if bobChatgpt == nil {
		t.Fatal("account bob/chatgpt not found")
	}
	wantPause := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	if !bobChatgpt.PauseUntil().Equal(wantPause) {
		t.Errorf("PauseUntil() = %v, want %v", bobChatgpt.PauseUntil(), wantPause)
	}
	bobProjects, err := loaded.AccountProjects(bobChatgpt)
	must(t, err)
	if len(bobProjects) != 0 {
		t.Error("bob/chatgpt should have no assigned projects")
	}
}

func TestRoundTrip_Sessions(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	loaded, err := st.Load()
	must(t, err)

	aliceClaude := loaded.Account(loaded.User("alice@example.com"), loaded.Provider("claude"))
	sessions, err := loaded.AccountSessions(aliceClaude)
	must(t, err)
	if len(sessions) != 1 {
		t.Fatalf("alice/claude sessions: got %d, want 1", len(sessions))
	}
	s := sessions[0]
	if s.ProjectName() != "Alpha" {
		t.Errorf("ProjectName() = %q, want Alpha", s.ProjectName())
	}
	if s.Topic() != "Architecture review" {
		t.Errorf("Topic() = %q", s.Topic())
	}
	queue := s.Queue()
	if len(queue) != 2 {
		t.Fatalf("queue: got %d items, want 2", len(queue))
	}
	if queue[0].Text() != "continue" || queue[0].SpawnsSession() {
		t.Errorf("queue[0] = %+v", queue[0])
	}
	if queue[1].Text() != "proceed to step 2" || !queue[1].SpawnsSession() {
		t.Errorf("queue[1] = %+v", queue[1])
	}

	bobChatgpt := loaded.Account(loaded.User("bob@example.com"), loaded.Provider("chatgpt"))
	bobSessions, err := loaded.AccountSessions(bobChatgpt)
	must(t, err)
	if len(bobSessions) != 1 {
		t.Fatalf("bob/chatgpt sessions: got %d, want 1", len(bobSessions))
	}
	bs := bobSessions[0]
	if bs.ProjectName() != "" || bs.Topic() != "" || len(bs.Queue()) != 0 {
		t.Errorf("bob/chatgpt session should have no project/topic/queue, got %+v", bs)
	}
}

func TestRoundTrip_SessionIDsRenumbered(t *testing.T) {
	t.Parallel()
	// Session IDs are ephemeral surrogate keys, not persisted. Deleting the
	// first of three sessions before saving must produce a reloaded
	// registry whose two remaining sessions are renumbered 1, 2 — not a
	// reload that preserves gaps from the original numbering.
	reg := model.NewRegistry()
	must(t, reg.AddProvider(model.NewProvider("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Provider("claude"))
	must(t, err)

	first, err := reg.AddSession(acc, nil, "first", nil)
	must(t, err)
	_, err = reg.AddSession(acc, nil, "second", nil)
	must(t, err)
	_, err = reg.AddSession(acc, nil, "third", nil)
	must(t, err)
	must(t, reg.DeleteSession(first))

	st, _ := newStore(t)
	must(t, st.Save(reg))
	loaded, err := st.Load()
	must(t, err)

	sessions := loaded.Sessions()
	if len(sessions) != 2 {
		t.Fatalf("sessions: got %d, want 2", len(sessions))
	}
	if sessions[0].ID() != 1 || sessions[0].Topic() != "second" {
		t.Errorf("sessions[0] = id %d topic %q, want id 1 topic %q",
			sessions[0].ID(), sessions[0].Topic(), "second")
	}
	if sessions[1].ID() != 2 || sessions[1].Topic() != "third" {
		t.Errorf("sessions[1] = id %d topic %q, want id 2 topic %q",
			sessions[1].ID(), sessions[1].Topic(), "third")
	}
}

func TestRoundTrip_EmptyRegistry(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(model.NewRegistry()))
	loaded, err := st.Load()
	must(t, err)

	if len(loaded.Providers()) != 0 || len(loaded.Users()) != 0 || len(loaded.Projects()) != 0 ||
		len(loaded.Accounts()) != 0 || len(loaded.Sessions()) != 0 {
		t.Error("expected a fully empty registry after round-tripping an empty one")
	}
}
