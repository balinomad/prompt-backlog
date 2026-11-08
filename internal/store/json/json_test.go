package json_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/balinomad/prompt-backlog/internal/apperr"
	"github.com/balinomad/prompt-backlog/internal/model"
	storejson "github.com/balinomad/prompt-backlog/internal/store/json"
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
//	Vendors: claude (HasProjects+HasDeepResearch), chatgpt (neither)
//	Users:     alice@example.com / Alice, bob@example.com / Bob
//	Projects:  Alpha (description+instructions), Beta (blank), Orphan (unattached)
//	Accounts:  alice/claude — not paused, assigned Alpha+Beta
//	           bob/chatgpt  — paused, no projects
//	Sessions:  alice/claude — project Alpha, name, two queued prompts
//	           bob/chatgpt  — no project, no name, no queue
//	Notes:     project Alpha, no account; account alice/claude, no project;
//	           project Alpha and account alice/claude; neither
func seedRegistry(t *testing.T) *model.Registry {
	t.Helper()
	reg := model.NewRegistry()

	must(t, reg.AddVendor(model.NewVendor("claude", true, true)))
	must(t, reg.AddVendor(model.NewVendor("chatgpt", false, false)))

	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))

	must(t, reg.AddProject(model.NewProject("Alpha", "Backend rewrite", "Use Go interfaces")))
	must(t, reg.AddProject(model.NewProject("Beta", "", "")))
	must(t, reg.AddProject(model.NewProject("Orphan", "", "")))

	aliceClaude, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Beta")))
	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Alpha")))

	bobChatgpt, err := reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
	must(t, err)
	must(t, reg.PauseAccount(bobChatgpt, time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)))
	// PauseAccount replaces the registry's Account via Copy-on-Write; the
	// pointer held above is now stale and must be re-read before further use.
	bobChatgpt = reg.Account(reg.User("bob@example.com"), reg.Vendor("chatgpt"))

	_, err = reg.AddSession(aliceClaude, reg.Project("Alpha"), "Architecture review", []model.Prompt{
		model.NewPrompt("continue"),
		model.NewPrompt("proceed to step 2"),
	})
	must(t, err)

	_, err = reg.AddSession(bobChatgpt, nil, "", nil)
	must(t, err)

	_, err = reg.AddNote("Implement export", reg.Project("Alpha"), nil)
	must(t, err)
	_, err = reg.AddNote("Review billing", nil, aliceClaude)
	must(t, err)
	_, err = reg.AddNote("Project and account", reg.Project("Alpha"), aliceClaude)
	must(t, err)
	_, err = reg.AddNote("Global note", nil, nil)
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
	if len(reg.Vendors()) != 0 {
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
				if _, ok := errors.AsType[*json.SyntaxError](err); !ok {
					t.Errorf("error %v is not a *json.SyntaxError", err)
				}
			},
		},
		{
			name:    "wrong top-level field type",
			content: `{"vendors": "not-an-array"}`,
			check: func(t *testing.T, err error) {
				t.Helper()
				if _, ok := errors.AsType[*json.UnmarshalTypeError](err); !ok {
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
			name: "vendor empty name",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "",
      "has_projects": false,
      "has_deep_research": false
    }
  ]
}`,
			wantErrIs: []error{model.ErrNameRequired},
		},
		{
			name: "duplicate vendor",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    },
    {
      "name": "claude",
      "has_projects": false,
      "has_deep_research": false
    }
  ]
}`,
			wantErrIs: []error{model.ErrVendorExists},
		},
		{
			name: "user empty email",
			json: `{
  "version": 1,
  "users": [
    {
      "email": "",
      "name": "Alice"
    }
  ]
}`,
			wantErrIs: []error{model.ErrEmailRequired},
		},
		{
			name: "user invalid email",
			json: `{
  "version": 1,
  "users": [
    {
      "email": "alice.example.com",
      "name": "Alice"
    }
  ]
}`,
			wantErrIs: []error{apperr.ErrEmailInvalid},
		},
		{
			name: "duplicate user",
			json: `{
  "version": 1,
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    },
    {
      "email": "alice@example.com",
      "name": "Alice2"
    }
  ]
}`,
			wantErrIs: []error{model.ErrUserExists},
		},
		{
			name: "project empty name",
			json: `{
  "version": 1,
  "projects": [
    {
      "name": "",
      "id": 1
    }
  ]
}`,
			wantErrIs: []error{model.ErrNameRequired},
		},
		{
			name: "duplicate project",
			json: `{
  "version": 1,
  "projects": [
    {
      "name": "Alpha",
      "id": 1
    },
    {
      "name": "Alpha",
      "id": 2
    }
  ]
}`,
			wantErrIs: []error{model.ErrProjectExists},
		},
		{
			name: "account references unknown vendor",
			json: `{
  "version": 1,
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "ghost",
      "id": 1
    }
  ]
}`,
			wantErrIs: []error{model.ErrVendorNil},
		},
		{
			name: "account references unknown user",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "accounts": [
    {
      "user_email": "ghost@example.com",
      "vendor_name": "claude",
      "id": 1
    }
  ]
}`,
			wantErrIs: []error{model.ErrUserNil},
		},
		{
			name: "duplicate account",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 1
    },
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 2
    }
  ]
}`,
			wantErrIs: []error{model.ErrAccountExists},
		},
		{
			name: "account references unknown project",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 1,
      "project_ids": [
        999
      ]
    }
  ]
}`,
			wantErrIs: []error{model.ErrProjectNotFound},
		},
		{
			name: "account project on vendor without project support",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "chatgpt",
      "has_projects": false,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "projects": [
    {
      "name": "Alpha",
      "id": 1
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "chatgpt",
      "id": 1,
      "project_ids": [
        1
      ]
    }
  ]
}`,
			wantErrIs: []error{model.ErrProjectsUnsupported},
		},
		{
			name: "account project listed twice",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "projects": [
    {
      "name": "Alpha",
      "id": 1
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 1,
      "project_ids": [
        1,
        1
      ]
    }
  ]
}`,
			wantErrSubstrs: []string{"already assigned"},
		},
		{
			name: "session references unknown account",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "sessions": [
    {
      "account_id": 1
    }
  ]
}`,
			wantErrSubstrs: []string{"account", "not found"},
		},
		{
			name: "session references unknown project",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 1
    }
  ],
  "sessions": [
    {
      "account_id": 1,
      "project_id": 999
    }
  ]
}`,
			wantErrSubstrs: []string{"project", "not found"},
		},
		{
			name: "session project not assigned to account",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "projects": [
    {
      "name": "Alpha",
      "id": 1
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 1
    }
  ],
  "sessions": [
    {
      "id": 1,
      "account_id": 1,
      "project_id": 1
    }
  ]
}`,
			wantErrSubstrs: []string{"not assigned"},
		},
		{
			name: "session queue has an invalid prompt",
			json: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 1
    }
  ],
  "sessions": [
    {
      "id": 1,
      "queue": [
        {
          "text": ""
        }
      ],
      "account_id": 1
    }
  ]
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

func TestLoad_EmptyJSONObject_Rejected(t *testing.T) {
	t.Parallel()
	path := writeTempFile(t, "{}")
	_, err := storejson.New(path).Load()
	if err == nil {
		t.Fatal("expected an error for an object without version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error %q does not mention version", err.Error())
	}
}

func TestLoad_AccountPauseUntilZero_Skipped(t *testing.T) {
	t.Parallel()
	// A present-but-zero pause_until must not be applied: PauseAccount is
	// only called when the timestamp is both non-nil and non-zero.
	path := writeTempFile(t, `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "pause_until": "0001-01-01T00:00:00Z",
      "id": 1
    }
  ]
}`)
	reg, err := storejson.New(path).Load()
	must(t, err)

	acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
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
// These tests exercise Save's filesystem error paths by arranging for the
// underlying os call to fail.

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

func TestSave_WriteFails(t *testing.T) {
	t.Parallel()
	// /dev/full accepts an open but fails every write. A symlink at the temp
	// path makes the store's write hit ENOSPC without needing write access to
	// the device node itself.
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full is not available")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")
	must(t, os.Symlink("/dev/full", path+".tmp"))

	err := storejson.New(path).Save(model.NewRegistry())
	if err == nil {
		t.Fatal("expected an error writing to /dev/full")
	}
	if !strings.Contains(err.Error(), "write") {
		t.Errorf("error = %v, want a write error", err)
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
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
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

func TestRoundTrip_Vendors(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	loaded, err := st.Load()
	must(t, err)

	if got := len(loaded.Vendors()); got != 2 {
		t.Fatalf("vendors: got %d, want 2", got)
	}
	claude := loaded.Vendor("claude")
	if claude == nil {
		t.Fatal("vendor claude not found")
	}
	if !claude.HasProjects() || !claude.HasDeepResearch() {
		t.Error("claude capabilities not preserved")
	}
	chatgpt := loaded.Vendor("chatgpt")
	if chatgpt == nil {
		t.Fatal("vendor chatgpt not found")
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

	aliceClaude := loaded.Account(loaded.User("alice@example.com"), loaded.Vendor("claude"))
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

	bobChatgpt := loaded.Account(loaded.User("bob@example.com"), loaded.Vendor("chatgpt"))
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

func TestRoundTrip_PausedAccountWithProjects(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
	until := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	must(t, reg.PauseAccount(acc, until))

	st, _ := newStore(t)
	must(t, st.Save(reg))
	loaded, err := st.Load()
	must(t, err)

	loadedAcc := loaded.Account(loaded.User("alice@example.com"), loaded.Vendor("claude"))
	if loadedAcc == nil {
		t.Fatal("account not found")
	}
	if !loadedAcc.PauseUntil().Equal(until) {
		t.Errorf("PauseUntil() = %v, want %v", loadedAcc.PauseUntil(), until)
	}
	projects, err := loaded.AccountProjects(loadedAcc)
	must(t, err)
	if len(projects) != 1 || projects[0].Name() != "Alpha" {
		t.Errorf("projects = %v, want [Alpha]", projects)
	}
}

func TestRoundTrip_Sessions(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	loaded, err := st.Load()
	must(t, err)

	aliceClaude := loaded.Account(loaded.User("alice@example.com"), loaded.Vendor("claude"))
	sessions, err := loaded.AccountSessions(aliceClaude)
	must(t, err)
	if len(sessions) != 1 {
		t.Fatalf("alice/claude sessions: got %d, want 1", len(sessions))
	}
	s := sessions[0]
	if s.ProjectName() != "Alpha" {
		t.Errorf("ProjectName() = %q, want Alpha", s.ProjectName())
	}
	if s.Name() != "Architecture review" {
		t.Errorf("Name() = %q", s.Name())
	}
	queue := s.Queue()
	if len(queue) != 2 {
		t.Fatalf("queue: got %d items, want 2", len(queue))
	}
	if queue[0].Text() != "continue" {
		t.Errorf("queue[0] = %+v", queue[0])
	}
	if queue[1].Text() != "proceed to step 2" {
		t.Errorf("queue[1] = %+v", queue[1])
	}

	bobChatgpt := loaded.Account(loaded.User("bob@example.com"), loaded.Vendor("chatgpt"))
	bobSessions, err := loaded.AccountSessions(bobChatgpt)
	must(t, err)
	if len(bobSessions) != 1 {
		t.Fatalf("bob/chatgpt sessions: got %d, want 1", len(bobSessions))
	}
	bs := bobSessions[0]
	if bs.ProjectName() != "" || bs.Name() != "" || len(bs.Queue()) != 0 {
		t.Errorf("bob/chatgpt session should have no project/name/queue, got %+v", bs)
	}
}

func TestRoundTrip_SessionIDsPreserved(t *testing.T) {
	t.Parallel()
	// Session IDs are persisted. Deleting the first of three sessions before
	// saving must produce a reloaded registry whose remaining sessions keep
	// IDs 2 and 3, and whose counter stays at 3.
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
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
	if sessions[0].ID() != 2 || sessions[0].Name() != "second" {
		t.Errorf("sessions[0] = id %d name %q, want id 2 name %q",
			sessions[0].ID(), sessions[0].Name(), "second")
	}
	if sessions[1].ID() != 3 || sessions[1].Name() != "third" {
		t.Errorf("sessions[1] = id %d name %q, want id 3 name %q",
			sessions[1].ID(), sessions[1].Name(), "third")
	}
	loadedAcc := loaded.Account(loaded.User("alice@example.com"), loaded.Vendor("claude"))
	next, err := loaded.AddSession(loadedAcc, nil, "", nil)
	must(t, err)
	if next.ID() != 4 {
		t.Errorf("next session ID = %d, want 4", next.ID())
	}
}

func TestRoundTrip_EmptyRegistry(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(model.NewRegistry()))
	loaded, err := st.Load()
	must(t, err)

	if len(loaded.Vendors()) != 0 || len(loaded.Users()) != 0 || len(loaded.Projects()) != 0 ||
		len(loaded.Accounts()) != 0 || len(loaded.Sessions()) != 0 {
		t.Error("expected a fully empty registry after round-tripping an empty one")
	}
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestRoundTrip_Notes(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	loaded, err := st.Load()
	must(t, err)

	notes := loaded.Notes()
	if len(notes) != 4 {
		t.Fatalf("notes: got %d, want 4", len(notes))
	}
	for i, wantID := range []uint64{1, 2, 3, 4} {
		if notes[i].ID() != wantID {
			t.Errorf("notes[%d].ID() = %d, want %d", i, notes[i].ID(), wantID)
		}
	}

	if notes[0].ProjectName() != "Alpha" {
		t.Errorf("notes[0].ProjectName() = %q, want Alpha", notes[0].ProjectName())
	}
	if _, ok := notes[0].AccountKey(); ok {
		t.Error("notes[0] has an account, want none")
	}

	if notes[1].ProjectName() != "" {
		t.Errorf("notes[1].ProjectName() = %q, want empty", notes[1].ProjectName())
	}
	if key, ok := notes[1].AccountKey(); !ok || key.Email != "alice@example.com" || key.Vendor != "claude" {
		t.Errorf("notes[1].AccountKey() = %v, %v; want alice/claude, true", key, ok)
	}

	if notes[2].ProjectName() != "Alpha" {
		t.Errorf("notes[2].ProjectName() = %q, want Alpha", notes[2].ProjectName())
	}
	if key, ok := notes[2].AccountKey(); !ok || key.Email != "alice@example.com" {
		t.Errorf("notes[2].AccountKey() = %v, %v; want alice, true", key, ok)
	}

	if notes[3].ProjectName() != "" {
		t.Errorf("notes[3].ProjectName() = %q, want empty", notes[3].ProjectName())
	}
	if _, ok := notes[3].AccountKey(); ok {
		t.Error("notes[3] has an account, want none")
	}

	next, err := loaded.AddNote("next", nil, nil)
	must(t, err)
	if next.ID() != 5 {
		t.Errorf("next note ID = %d, want 5", next.ID())
	}
}

func TestLoad_SessionStatus(t *testing.T) {
	t.Parallel()

	base := `{
  "version": 1,
  "vendors": [{"name": "claude"}],
  "users": [{"email": "alice@example.com", "name": "Alice"}],
  "accounts": [{"id": 1, "user_email": "alice@example.com", "vendor_name": "claude"}],
  "sessions": [{"id": 1, "account_id": 1%s}]
}`

	cases := []struct {
		name   string
		suffix string
		want   model.SessionStatus
	}{
		{"active", "", model.SessionStatusActive},
		{"needs closing", `, "status": "needs_closing"`, model.SessionStatusNeedsClosing},
		{"closed", `, "status": "closed"`, model.SessionStatusClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeTempFile(t, fmt.Sprintf(base, tc.suffix))
			reg, err := storejson.New(path).Load()
			must(t, err)
			if got := reg.Session(1).Status(); got != tc.want {
				t.Errorf("Status() = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("invalid status", func(t *testing.T) {
		t.Parallel()
		path := writeTempFile(t, fmt.Sprintf(base, `, "status": "bogus"`))
		_, err := storejson.New(path).Load()
		if err == nil || !strings.Contains(err.Error(), "invalid session status") {
			t.Errorf("error = %v, want invalid session status", err)
		}
	})
}

func TestRoundTrip_SessionFields(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	reg.SetClock(func() time.Time { return time.Date(2026, 10, 30, 9, 15, 0, 0, time.UTC) })
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))

	sess, err := reg.AddSession(acc, reg.Project("Alpha"), "Architecture review", []model.Prompt{
		model.NewPrompt("continue"),
		model.NewPrompt("start handover"),
	})
	must(t, err)
	must(t, reg.UpdateSessionURL(sess, " https://example.com/chat/1 "))
	must(t, reg.UpdateSessionStatus(reg.Session(sess.ID()), model.SessionStatusNeedsClosing))

	st, _ := newStore(t)
	must(t, st.Save(reg))
	loaded, err := st.Load()
	must(t, err)

	got := loaded.Session(sess.ID())
	if got == nil {
		t.Fatal("session not found after round trip")
	}
	if got.URL() != "https://example.com/chat/1" {
		t.Errorf("URL() = %q", got.URL())
	}
	if got.Status() != model.SessionStatusNeedsClosing {
		t.Errorf("Status() = %v, want Needs closing", got.Status())
	}
	wantAt := time.Date(2026, 10, 30, 9, 15, 0, 0, time.UTC)
	if !got.UpdatedAt().Equal(wantAt) {
		t.Errorf("UpdatedAt() = %v, want %v", got.UpdatedAt(), wantAt)
	}
	queue := got.Queue()
	if len(queue) != 2 || queue[0].Text() != "continue" || queue[1].Text() != "start handover" {
		t.Errorf("Queue() = %v, want the two prompts", queue)
	}
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestSave_WritesVersionAndFields(t *testing.T) {
	t.Parallel()
	st, path := newStore(t)
	must(t, st.Save(seedRegistry(t)))

	data, err := os.ReadFile(filepath.Clean(path))
	must(t, err)

	var raw struct {
		Version  int `json:"version"`
		Projects []struct {
			ID   uint64 `json:"id"`
			Name string `json:"name"`
		} `json:"projects"`
		Accounts []struct {
			ID         uint64   `json:"id"`
			UserEmail  string   `json:"user_email"`
			VendorName string   `json:"vendor_name"`
			ProjectIDs []uint64 `json:"project_ids"`
		} `json:"accounts"`
		Sessions []struct {
			ID        uint64 `json:"id"`
			AccountID uint64 `json:"account_id"`
			ProjectID uint64 `json:"project_id"`
			Status    string `json:"status"`
			Queue     []struct {
				Text string `json:"text"`
			} `json:"queue"`
		} `json:"sessions"`
		Notes []struct {
			ID         uint64 `json:"id"`
			Text       string `json:"text"`
			ProjectID  uint64 `json:"project_id"`
			UserEmail  string `json:"user_email"`
			VendorName string `json:"vendor_name"`
		} `json:"notes"`
	}
	must(t, json.Unmarshal(data, &raw))

	if raw.Version != 1 {
		t.Errorf("version = %d, want 1", raw.Version)
	}
	if strings.Contains(string(data), "last_session_id") || strings.Contains(string(data), "last_note_id") {
		t.Errorf("saved JSON still contains a stored counter")
	}
	if len(raw.Projects) != 3 {
		t.Fatalf("projects = %d, want 3", len(raw.Projects))
	}
	for _, project := range raw.Projects {
		if project.ID == 0 || project.Name == "" {
			t.Errorf("project = %+v, want id and name", project)
		}
	}
	if len(raw.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(raw.Sessions))
	}
	if len(raw.Accounts) != 2 {
		t.Fatalf("accounts = %d, want 2", len(raw.Accounts))
	}
	for _, acc := range raw.Accounts {
		if acc.ID == 0 || acc.UserEmail == "" || acc.VendorName == "" {
			t.Errorf("account = %+v, want id and natural key", acc)
		}
	}
	if len(raw.Accounts[0].ProjectIDs) != 2 {
		t.Errorf("account project_ids = %v, want 2 IDs", raw.Accounts[0].ProjectIDs)
	}
	if raw.Sessions[0].ID != 1 || raw.Sessions[0].AccountID == 0 || raw.Sessions[0].ProjectID == 0 {
		t.Errorf("sessions[0] = %+v, want id, account_id and project_id", raw.Sessions[0])
	}
	for i, session := range raw.Sessions {
		if session.Status == "" {
			t.Errorf("sessions[%d].status is empty", i)
		}
	}
	if len(raw.Sessions[0].Queue) != 2 || raw.Sessions[0].Queue[0].Text != "continue" {
		t.Errorf("sessions[0].queue = %+v", raw.Sessions[0].Queue)
	}
	if len(raw.Notes) != 4 {
		t.Fatalf("notes = %d, want 4", len(raw.Notes))
	}
	if raw.Notes[0].ID != 1 || raw.Notes[0].Text == "" {
		t.Errorf("notes[0] = %+v, want id and text", raw.Notes[0])
	}
	if raw.Notes[0].UserEmail != "" || raw.Notes[0].VendorName != "" {
		t.Errorf("notes[0] = %+v, want omitted account fields", raw.Notes[0])
	}
	if raw.Notes[1].UserEmail == "" || raw.Notes[1].VendorName == "" {
		t.Errorf("notes[1] = %+v, want account fields", raw.Notes[1])
	}
}

func TestLoad_VersionRejection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		wantSub string
	}{
		{"missing version", `{"vendors": []}`, "version"},
		{"version zero", `{
  "version": 0
}`, "version"},
		{"version two", `{
  "version": 2
}`, "version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeTempFile(t, tc.content)
			_, err := storejson.New(path).Load()
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}

	t.Run("version one is accepted", func(t *testing.T) {
		t.Parallel()
		path := writeTempFile(t, `{
  "version": 1
}`)
		reg, err := storejson.New(path).Load()
		must(t, err)
		if len(reg.Vendors()) != 0 || len(reg.Sessions()) != 0 || len(reg.Notes()) != 0 {
			t.Error("expected an empty registry")
		}
	})
}

func TestLoad_SessionAndNoteIDErrors(t *testing.T) {
	t.Parallel()

	sessionBase := `{
		"version": 1,
		"vendors": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
		"users": [{"email": "alice@example.com", "name": "Alice"}],
		"accounts": [{"id": 1, "user_email": "alice@example.com", "vendor_name": "claude"}],
		"sessions": [
			{"id": %d, "account_id": 1}%s
		]
	}`

	cases := []struct {
		name    string
		content string
		wantSub string
	}{
		{
			name: "zero session ID",
			content: `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 1
    }
  ],
  "sessions": [
    {
      "id": 0,
      "account_id": 1
    }
  ]
}`,
			wantSub: "id 0",
		},
		{
			name: "duplicate session ID",
			content: fmt.Sprintf(sessionBase, 1, `,
				{"id": 1, "account_id": 1}`),
			wantSub: "already exists",
		},
		{
			name: "zero note ID",
			content: `{
  "version": 1,
  "notes": [
    {
      "id": 0,
      "text": "one"
    }
  ]
}`,
			wantSub: "id 0",
		},
		{
			name: "duplicate note ID",
			content: `{
  "version": 1,
  "notes": [
    {
      "id": 1,
      "text": "one"
    },
    {
      "id": 1,
      "text": "two"
    }
  ]
}`,
			wantSub: "already exists",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeTempFile(t, tc.content)
			_, err := storejson.New(path).Load()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("got %v, want error containing %q", err, tc.wantSub)
			}
		})
	}
}

func TestLoad_NoteErrors(t *testing.T) {
	t.Parallel()

	base := `{
		"version": 1,
		"vendors": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
		"users": [{"email": "alice@example.com", "name": "Alice"}],
		"projects": [{"id": 1, "name": "Alpha"}],
		"accounts": [{"id": 1, "user_email": "alice@example.com", "vendor_name": "claude"}],
		"notes": [%s]
	}`

	cases := []struct {
		name    string
		note    string
		wantIs  error
		wantSub string
	}{
		{
			name:    "unknown project",
			note:    `{"id": 1, "text": "one", "project_id": 999}`,
			wantSub: "project",
		},
		{
			name:    "unknown account",
			note:    `{"id": 1, "text": "one", "user_email": "ghost@example.com", "vendor_name": "claude"}`,
			wantSub: "account",
		},
		{
			name:    "one account field only",
			note:    `{"id": 1, "text": "one", "user_email": "alice@example.com"}`,
			wantSub: "account",
		},
		{
			name:   "empty text",
			note:   `{"id": 1, "text": ""}`,
			wantIs: model.ErrTextRequired,
		},
		{
			name:    "project not granted to account",
			note:    `{"id": 1, "text": "one", "project_id": 1, "user_email": "alice@example.com", "vendor_name": "claude"}`,
			wantSub: "not assigned",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeTempFile(t, fmt.Sprintf(base, tc.note))
			_, err := storejson.New(path).Load()
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Errorf("errors.Is(err, %v) = false; err = %v", tc.wantIs, err)
			}
			if tc.wantSub != "" && !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestLoad_NextIDs(t *testing.T) {
	t.Parallel()

	t.Run("greatest loaded IDs", func(t *testing.T) {
		t.Parallel()
		path := writeTempFile(t, `{
  "version": 1,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 1
    }
  ],
  "sessions": [
    {
      "id": 3,
      "account_id": 1
    },
    {
      "id": 7,
      "account_id": 1
    }
  ],
  "notes": [
    {
      "id": 2,
      "text": "two"
    },
    {
      "id": 5,
      "text": "five"
    }
  ]
}`)
		reg, err := storejson.New(path).Load()
		must(t, err)

		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		sess, err := reg.AddSession(acc, nil, "", nil)
		must(t, err)
		if sess.ID() != 8 {
			t.Errorf("next session ID = %d, want 8", sess.ID())
		}
		note, err := reg.AddNote("next", nil, nil)
		must(t, err)
		if note.ID() != 6 {
			t.Errorf("next note ID = %d, want 6", note.ID())
		}
	})

	t.Run("empty file", func(t *testing.T) {
		t.Parallel()
		path := writeTempFile(t, `{
  "version": 1
}`)
		reg, err := storejson.New(path).Load()
		must(t, err)

		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)

		sess, err := reg.AddSession(acc, nil, "", nil)
		must(t, err)
		if sess.ID() != 1 {
			t.Errorf("next session ID = %d, want 1", sess.ID())
		}
		note, err := reg.AddNote("first", nil, nil)
		must(t, err)
		if note.ID() != 1 {
			t.Errorf("next note ID = %d, want 1", note.ID())
		}
	})
}

func TestLoad_IgnoresStoredCounters(t *testing.T) {
	t.Parallel()
	path := writeTempFile(t, `{
  "version": 1,
  "last_session_id": 99,
  "last_note_id": 99,
  "vendors": [
    {
      "name": "claude",
      "has_projects": true,
      "has_deep_research": false
    }
  ],
  "users": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "accounts": [
    {
      "user_email": "alice@example.com",
      "vendor_name": "claude",
      "id": 1
    }
  ],
  "sessions": [
    {
      "id": 2,
      "account_id": 1
    }
  ],
  "notes": [
    {
      "id": 2,
      "text": "two"
    }
  ]
}`)
	reg, err := storejson.New(path).Load()
	must(t, err)

	acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
	sess, err := reg.AddSession(acc, nil, "", nil)
	must(t, err)
	if sess.ID() != 3 {
		t.Errorf("next session ID = %d, want 3", sess.ID())
	}
	note, err := reg.AddNote("next", nil, nil)
	must(t, err)
	if note.ID() != 3 {
		t.Errorf("next note ID = %d, want 3", note.ID())
	}
}

func TestRoundTrip_ProjectIDs(t *testing.T) {
	t.Parallel()
	st, _ := newStore(t)
	must(t, st.Save(seedRegistry(t)))
	loaded, err := st.Load()
	must(t, err)

	for _, project := range loaded.Projects() {
		if project.ID() == 0 {
			t.Errorf("project %q has ID 0", project.Name())
		}
		if got := loaded.ProjectByID(project.ID()); got != project {
			t.Errorf("ProjectByID(%d) = %v, want %v", project.ID(), got, project)
		}
	}

	acc := loaded.Account(loaded.User("alice@example.com"), loaded.Vendor("claude"))
	projects, err := loaded.AccountProjects(acc)
	must(t, err)
	for _, project := range projects {
		if project.ID() == 0 {
			t.Errorf("account project %q has ID 0", project.Name())
		}
	}

	session := loaded.Session(1)
	if session == nil || session.ProjectID() == 0 {
		t.Errorf("session project ID = %v, want non-zero", session)
	}
	for _, note := range loaded.Notes() {
		if note.ProjectName() != "" && note.ProjectID() == 0 {
			t.Errorf("note %d has project name %q but ID 0", note.ID(), note.ProjectName())
		}
	}
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestRoundTrip_ProjectRename(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddProject(model.NewProject("Alpha", "desc", "inst")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	project := reg.Project("Alpha")
	must(t, reg.AddProjectToAccount(acc, project))
	session, err := reg.AddSession(acc, project, "Name", []model.Prompt{model.NewPrompt("next")})
	must(t, err)
	note, err := reg.AddNote("note", project, acc)
	must(t, err)
	projectID := project.ID()

	st, _ := newStore(t)
	must(t, st.Save(reg))
	loaded, err := st.Load()
	must(t, err)

	loadedProject := loaded.Project("Alpha")
	if loadedProject == nil {
		t.Fatal("Alpha not found after load")
	}
	renamed, err := loaded.RenameProject(loadedProject, "Beta")
	must(t, err)
	if renamed.ID() != projectID || renamed.Name() != "Beta" {
		t.Errorf("renamed project = id %d name %q, want id %d name Beta", renamed.ID(), renamed.Name(), projectID)
	}
	if got := loaded.Session(session.ID()); got == nil || got.ProjectName() != "Beta" || got.ProjectID() != projectID {
		t.Errorf("session after rename = %v", got)
	}
	if got := loaded.Note(note.ID()); got == nil || got.ProjectName() != "Beta" || got.ProjectID() != projectID {
		t.Errorf("note after rename = %v", got)
	}

	must(t, st.Save(loaded))
	reloaded, err := st.Load()
	must(t, err)
	reloadedProject := reloaded.Project("Beta")
	if reloadedProject == nil || reloadedProject.ID() != projectID {
		t.Fatalf("Beta after reload = %v, want id %d", reloadedProject, projectID)
	}
	if got := reloaded.Session(session.ID()); got == nil || got.ProjectName() != "Beta" || got.ProjectID() != projectID {
		t.Errorf("session after reload = %v", got)
	}
	if got := reloaded.Note(note.ID()); got == nil || got.ProjectName() != "Beta" || got.ProjectID() != projectID {
		t.Errorf("note after reload = %v", got)
	}
}

func TestLoad_ProjectIDErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		wantSub string
	}{
		{
			name:    "zero project ID",
			content: `{"version": 1, "projects": [{"id": 0, "name": "Alpha"}]}`,
			wantSub: "id 0",
		},
		{
			name:    "duplicate project ID",
			content: `{"version": 1, "projects": [{"id": 1, "name": "Alpha"}, {"id": 1, "name": "Beta"}]}`,
			wantSub: "already exists",
		},
		{
			name: "unknown project ID in account",
			content: `{
				"version": 1,
				"vendors": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"accounts": [{"id": 1, "user_email": "alice@example.com", "vendor_name": "claude", "project_ids": [999]}]
			}`,
			wantSub: "project id 999",
		},
		{
			name: "unknown project ID in session",
			content: `{
				"version": 1,
				"vendors": [{"name": "claude", "has_projects": true, "has_deep_research": false}],
				"users": [{"email": "alice@example.com", "name": "Alice"}],
				"accounts": [{"id": 1, "user_email": "alice@example.com", "vendor_name": "claude"}],
				"sessions": [{"id": 1, "account_id": 1, "project_id": 999}]
			}`,
			wantSub: "project id 999",
		},
		{
			name: "unknown project ID in note",
			content: `{
				"version": 1,
				"notes": [{"id": 1, "text": "one", "project_id": 999}]
			}`,
			wantSub: "project id 999",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeTempFile(t, tc.content)
			_, err := storejson.New(path).Load()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("got %v, want error containing %q", err, tc.wantSub)
			}
		})
	}
}
