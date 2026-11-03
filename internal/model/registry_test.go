package model_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/apperr"
	"github.com/balinomad/llm-desk/internal/model"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// must fails the test immediately when err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// seedRegistry returns a populated registry for use across multiple tests.
//
//	Vendors:   "claude" (HasProjects=true), "chatgpt" (HasProjects=false)
//	Users:       alice@example.com / Alice, bob@example.com / Bob
//	Accounts:    alice@example.com|claude, bob@example.com|chatgpt
//	Projects:    "Alpha", "Beta", "Orphan"
//	Assignments: Alpha & Beta -> alice@example.com|claude
//	Sessions:
//	  ID 1: alice@example.com|claude, project=Alpha
//	  ID 2: bob@example.com|chatgpt, project=nil
func seedRegistry(t *testing.T) *model.Registry {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddVendor(model.NewVendor("chatgpt", false, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))

	must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
	must(t, reg.AddProject(model.NewProject("Beta", "", "")))
	must(t, reg.AddProject(model.NewProject("Orphan", "", "")))

	aliceClaude, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	bobChatgpt, err := reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
	must(t, err)

	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Alpha")))
	must(t, reg.AddProjectToAccount(aliceClaude, reg.Project("Beta")))

	_, err = reg.AddSession(aliceClaude, reg.Project("Alpha"), "Architecture review", nil)
	must(t, err)
	_, err = reg.AddSession(bobChatgpt, nil, "", nil)
	must(t, err)

	return reg
}

// ── Registry ──────────────────────────────────────────────────────────────────

func TestNewRegistry(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if got := reg.Vendors(); len(got) != 0 {
		t.Errorf("Vendors() = %v, want empty", got)
	}
	if got := reg.Users(); len(got) != 0 {
		t.Errorf("Users() = %v, want empty", got)
	}
	if got := reg.Projects(); len(got) != 0 {
		t.Errorf("Projects() = %v, want empty", got)
	}
	if got := reg.Accounts(); len(got) != 0 {
		t.Errorf("Accounts() = %v, want empty", got)
	}
	if got := reg.Sessions(); len(got) != 0 {
		t.Errorf("Sessions() = %v, want empty", got)
	}
}

// ── Vendor CRUD ─────────────────────────────────────────────────────────────

func TestRegistry_AddVendor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		setup   func(*testing.T, *model.Registry)
		vendor  *model.Vendor
		wantErr error
	}{
		{"success", func(*testing.T, *model.Registry) {}, model.NewVendor("gemini", true, true), nil},
		{"nil vendor", func(*testing.T, *model.Registry) {}, nil, model.ErrVendorNil},
		{"empty name", func(*testing.T, *model.Registry) {}, model.NewVendor("", false, false), model.ErrNameRequired},
		{"whitespace-only name", func(*testing.T, *model.Registry) {}, model.NewVendor("   ", false, false), model.ErrNameRequired},
		{
			"duplicate name",
			func(t *testing.T, r *model.Registry) {
				t.Helper()
				must(t, r.AddVendor(model.NewVendor("claude", true, false)))
			},
			model.NewVendor("claude", false, false),
			model.ErrVendorExists,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			tc.setup(t, reg)
			err := reg.AddVendor(tc.vendor)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestRegistry_Vendor(t *testing.T) {
	t.Parallel()
	reg := seedRegistry(t)
	cases := []struct {
		name     string
		query    string
		wantName string // empty means want nil
	}{
		{"found exact", "claude", "claude"},
		{"found with surrounding whitespace", "  claude  ", "claude"},
		{"not found", "ghost", ""},
		{"case sensitive, not found", "CLAUDE", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := reg.Vendor(tc.query)
			if tc.wantName == "" {
				if p != nil {
					t.Errorf("got %v, want nil", p)
				}
				return
			}
			if p == nil || p.Name() != tc.wantName {
				t.Errorf("got %v, want name %q", p, tc.wantName)
			}
		})
	}
}

func TestRegistry_Vendors(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if got := model.NewRegistry().Vendors(); len(got) != 0 {
			t.Errorf("got %d vendors, want 0", len(got))
		}
	})
	t.Run("sorted by name", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		got := reg.Vendors()
		if len(got) != 2 {
			t.Fatalf("got %d vendors, want 2", len(got))
		}
		if got[0].Name() != "chatgpt" || got[1].Name() != "claude" {
			t.Errorf("got %q, %q; want chatgpt, claude in order", got[0].Name(), got[1].Name())
		}
	})
}

func TestRegistry_DeleteVendor(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("gemini", true, false)))
		must(t, reg.DeleteVendor(reg.Vendor("gemini")))
		if reg.Vendor("gemini") != nil {
			t.Error("vendor still present after delete")
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().DeleteVendor(nil)
		if !errors.Is(err, model.ErrVendorNil) {
			t.Errorf("got %v, want ErrVendorNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewVendor("ghost", false, false)
		err := model.NewRegistry().DeleteVendor(ghost)
		if !errors.Is(err, model.ErrVendorNotFound) {
			t.Errorf("got %v, want ErrVendorNotFound", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", false, false)))
		stale := reg.Vendor("claude")
		must(t, reg.UpdateVendor(stale, model.NewVendor("claude", true, false)))
		err := reg.DeleteVendor(stale)
		if !errors.Is(err, model.ErrVendorStale) {
			t.Errorf("got %v, want ErrVendorStale", err)
		}
	})
	t.Run("has associated accounts", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		err := reg.DeleteVendor(reg.Vendor("claude"))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestRegistry_UpdateVendor(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", false, false)))
		current := reg.Vendor("claude")
		must(t, reg.UpdateVendor(current, model.NewVendor("claude", true, true)))
		got := reg.Vendor("claude")
		if !got.HasProjects() || !got.HasDeepResearch() {
			t.Errorf("got HasProjects=%v HasDeepResearch=%v, want both true", got.HasProjects(), got.HasDeepResearch())
		}
	})
	t.Run("current nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().UpdateVendor(nil, model.NewVendor("claude", true, false))
		if !errors.Is(err, model.ErrVendorNil) {
			t.Errorf("got %v, want ErrVendorNil", err)
		}
	})
	t.Run("current not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewVendor("ghost", false, false)
		err := model.NewRegistry().UpdateVendor(ghost, model.NewVendor("ghost", true, false))
		if !errors.Is(err, model.ErrVendorNotFound) {
			t.Errorf("got %v, want ErrVendorNotFound", err)
		}
	})
	t.Run("current stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", false, false)))
		stale := reg.Vendor("claude")
		must(t, reg.UpdateVendor(stale, model.NewVendor("claude", true, false)))
		err := reg.UpdateVendor(stale, model.NewVendor("claude", true, true))
		if !errors.Is(err, model.ErrVendorStale) {
			t.Errorf("got %v, want ErrVendorStale", err)
		}
	})
	t.Run("next nil", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", false, false)))
		err := reg.UpdateVendor(reg.Vendor("claude"), nil)
		if !errors.Is(err, model.ErrVendorNil) {
			t.Errorf("got %v, want ErrVendorNil", err)
		}
	})
	t.Run("next has empty name", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", false, false)))
		err := reg.UpdateVendor(reg.Vendor("claude"), model.NewVendor("", true, false))
		if !errors.Is(err, model.ErrNameRequired) {
			t.Errorf("got %v, want ErrNameRequired", err)
		}
	})
	t.Run("name mismatch", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", false, false)))
		err := reg.UpdateVendor(reg.Vendor("claude"), model.NewVendor("gemini", true, false))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

// ── User CRUD ─────────────────────────────────────────────────────────────────

func TestRegistry_AddUser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		setup   func(*testing.T, *model.Registry)
		user    *model.User
		wantErr error
	}{
		{"success", func(*testing.T, *model.Registry) {}, model.NewUser("new@example.com", "New"), nil},
		{"nil user", func(*testing.T, *model.Registry) {}, nil, model.ErrUserNil},
		{"empty email", func(*testing.T, *model.Registry) {}, model.NewUser("", "Name"), model.ErrEmailRequired},
		{"invalid email", func(*testing.T, *model.Registry) {}, model.NewUser("not-an-email", "Name"), apperr.ErrEmailInvalid},
		{"empty name", func(*testing.T, *model.Registry) {}, model.NewUser("alice@example.com", ""), model.ErrNameRequired},
		{
			"duplicate email",
			func(t *testing.T, r *model.Registry) {
				t.Helper()
				must(t, r.AddUser(model.NewUser("alice@example.com", "Alice")))
			},
			model.NewUser("alice@example.com", "Alice2"),
			model.ErrUserExists,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			tc.setup(t, reg)
			err := reg.AddUser(tc.user)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestRegistry_User(t *testing.T) {
	t.Parallel()
	reg := seedRegistry(t)
	cases := []struct {
		name      string
		query     string
		wantEmail string // empty means want nil
	}{
		{"found exact", "alice@example.com", "alice@example.com"},
		{"found case-insensitive", "ALICE@EXAMPLE.COM", "alice@example.com"},
		{"found with whitespace", "  alice@example.com  ", "alice@example.com"},
		{"not found", "ghost@example.com", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := reg.User(tc.query)
			if tc.wantEmail == "" {
				if u != nil {
					t.Errorf("got %v, want nil", u)
				}
				return
			}
			if u == nil || u.Email() != tc.wantEmail {
				t.Errorf("got %v, want email %q", u, tc.wantEmail)
			}
		})
	}
}

func TestRegistry_Users(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if got := model.NewRegistry().Users(); len(got) != 0 {
			t.Errorf("got %d users, want 0", len(got))
		}
	})
	t.Run("sorted by name", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		got := reg.Users()
		if len(got) != 2 {
			t.Fatalf("got %d users, want 2", len(got))
		}
		if got[0].Email() != "alice@example.com" || got[1].Email() != "bob@example.com" {
			t.Errorf("got %q, %q; want alice, bob in order", got[0].Email(), got[1].Email())
		}
	})
	t.Run("sorted by name case-insensitively with email tie-break", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("zoe@example.com", "Alice")))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Zoe")))
		must(t, reg.AddUser(model.NewUser("bob@example.com", "alice")))
		got := reg.Users()
		wantEmails := []string{"bob@example.com", "zoe@example.com", "alice@example.com"}
		if len(got) != len(wantEmails) {
			t.Fatalf("got %d users, want %d", len(got), len(wantEmails))
		}
		for i, want := range wantEmails {
			if got[i].Email() != want {
				t.Errorf("got[%d] = %q, want %q", i, got[i].Email(), want)
			}
		}
	})
}

func TestRegistry_DeleteUser(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("new@example.com", "New")))
		must(t, reg.DeleteUser(reg.User("new@example.com")))
		if reg.User("new@example.com") != nil {
			t.Error("user still present after delete")
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().DeleteUser(nil)
		if !errors.Is(err, model.ErrUserNil) {
			t.Errorf("got %v, want ErrUserNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewUser("ghost@example.com", "Ghost")
		err := model.NewRegistry().DeleteUser(ghost)
		if !errors.Is(err, model.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		stale := reg.User("alice@example.com")
		must(t, reg.UpdateUser(stale, model.NewUser("alice@example.com", "Alice Smith")))
		err := reg.DeleteUser(stale)
		if !errors.Is(err, model.ErrUserStale) {
			t.Errorf("got %v, want ErrUserStale", err)
		}
	})
	t.Run("has associated accounts", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		err := reg.DeleteUser(reg.User("alice@example.com"))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestRegistry_UpdateUser(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		current := reg.User("alice@example.com")
		must(t, reg.UpdateUser(current, model.NewUser("alice@example.com", "Alice Smith")))
		if got := reg.User("alice@example.com").Name(); got != "Alice Smith" {
			t.Errorf("got %q, want %q", got, "Alice Smith")
		}
	})
	t.Run("current nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().UpdateUser(nil, model.NewUser("alice@example.com", "Alice"))
		if !errors.Is(err, model.ErrUserNil) {
			t.Errorf("got %v, want ErrUserNil", err)
		}
	})
	t.Run("current not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewUser("ghost@example.com", "Ghost")
		err := model.NewRegistry().UpdateUser(ghost, model.NewUser("ghost@example.com", "Ghost2"))
		if !errors.Is(err, model.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})
	t.Run("current stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		stale := reg.User("alice@example.com")
		must(t, reg.UpdateUser(stale, model.NewUser("alice@example.com", "Alice Smith")))
		err := reg.UpdateUser(stale, model.NewUser("alice@example.com", "Alice Jones"))
		if !errors.Is(err, model.ErrUserStale) {
			t.Errorf("got %v, want ErrUserStale", err)
		}
	})
	t.Run("next invalid", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		err := reg.UpdateUser(reg.User("alice@example.com"), model.NewUser("alice@example.com", ""))
		if !errors.Is(err, model.ErrNameRequired) {
			t.Errorf("got %v, want ErrNameRequired", err)
		}
	})
	t.Run("email mismatch", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		err := reg.UpdateUser(reg.User("alice@example.com"), model.NewUser("bob@example.com", "Bob"))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

// ── Project CRUD ──────────────────────────────────────────────────────────────

func TestRegistry_AddProject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		setup   func(*testing.T, *model.Registry)
		project *model.Project
		wantErr error
	}{
		{"success", func(*testing.T, *model.Registry) {}, model.NewProject("Gamma", "", ""), nil},
		{"nil project", func(*testing.T, *model.Registry) {}, nil, model.ErrProjectNil},
		{"empty name", func(*testing.T, *model.Registry) {}, model.NewProject("", "", ""), model.ErrNameRequired},
		{"whitespace-only name", func(*testing.T, *model.Registry) {}, model.NewProject("   ", "", ""), model.ErrNameRequired},
		{
			"duplicate name",
			func(t *testing.T, r *model.Registry) {
				t.Helper()
				must(t, r.AddProject(model.NewProject("Alpha", "", "")))
			},
			model.NewProject("Alpha", "", ""),
			model.ErrProjectExists,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			tc.setup(t, reg)
			err := reg.AddProject(tc.project)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestRegistry_Project(t *testing.T) {
	t.Parallel()
	reg := seedRegistry(t)
	if p := reg.Project("Alpha"); p == nil {
		t.Error("expected to find Alpha")
	}
	if p := reg.Project("  Alpha  "); p == nil {
		t.Error("expected lookup with surrounding whitespace to find Alpha")
	}
	if p := reg.Project("Ghost"); p != nil {
		t.Error("expected nil for unknown project")
	}
}

func TestRegistry_Projects(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if got := model.NewRegistry().Projects(); len(got) != 0 {
			t.Errorf("got %d projects, want 0", len(got))
		}
	})
	t.Run("sorted by name", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		got := reg.Projects()
		if len(got) != 3 {
			t.Fatalf("got %d projects, want 3", len(got))
		}
		want := []string{"Alpha", "Beta", "Orphan"}
		for i, w := range want {
			if got[i].Name() != w {
				t.Errorf("got[%d] = %q, want %q", i, got[i].Name(), w)
			}
		}
	})
}

func TestRegistry_DeleteProject(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Gamma", "", "")))
		must(t, reg.DeleteProject(reg.Project("Gamma")))
		if reg.Project("Gamma") != nil {
			t.Error("project still present after delete")
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().DeleteProject(nil)
		if !errors.Is(err, model.ErrProjectNil) {
			t.Errorf("got %v, want ErrProjectNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewProject("Ghost", "", "")
		err := model.NewRegistry().DeleteProject(ghost)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		stale := reg.Project("Alpha")
		must(t, reg.UpdateProject(stale, model.NewProject("Alpha", "new desc", "")))
		err := reg.DeleteProject(stale)
		if !errors.Is(err, model.ErrProjectStale) {
			t.Errorf("got %v, want ErrProjectStale", err)
		}
	})
	t.Run("assigned to accounts", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		// Beta is assigned to alice|claude but has no sessions, isolating
		// the "assigned to accounts" branch from the "active sessions" one.
		err := reg.DeleteProject(reg.Project("Beta"))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
	t.Run("success when unassigned and unused", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		must(t, reg.DeleteProject(reg.Project("Orphan")))
	})
}

func TestRegistry_UpdateProject(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "old desc", "old inst")))
		current := reg.Project("Alpha")
		must(t, reg.UpdateProject(current, model.NewProject("Alpha", "new desc", "new inst")))
		got := reg.Project("Alpha")
		if got.Description() != "new desc" || got.Instructions() != "new inst" {
			t.Errorf("got desc %q inst %q, want %q %q", got.Description(), got.Instructions(), "new desc", "new inst")
		}
	})
	t.Run("current nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().UpdateProject(nil, model.NewProject("Alpha", "", ""))
		if !errors.Is(err, model.ErrProjectNil) {
			t.Errorf("got %v, want ErrProjectNil", err)
		}
	})
	t.Run("current not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewProject("Ghost", "", "")
		err := model.NewRegistry().UpdateProject(ghost, model.NewProject("Ghost", "x", ""))
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})
	t.Run("current stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		stale := reg.Project("Alpha")
		must(t, reg.UpdateProject(stale, model.NewProject("Alpha", "new", "")))
		err := reg.UpdateProject(stale, model.NewProject("Alpha", "newer", ""))
		if !errors.Is(err, model.ErrProjectStale) {
			t.Errorf("got %v, want ErrProjectStale", err)
		}
	})
	t.Run("next invalid", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		err := reg.UpdateProject(reg.Project("Alpha"), model.NewProject("", "x", ""))
		if !errors.Is(err, model.ErrNameRequired) {
			t.Errorf("got %v, want ErrNameRequired", err)
		}
	})
	t.Run("name mismatch", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		err := reg.UpdateProject(reg.Project("Alpha"), model.NewProject("Beta", "x", ""))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestRegistry_ProjectAccounts(t *testing.T) {
	t.Parallel()
	t.Run("nil project", func(t *testing.T) {
		t.Parallel()
		_, err := model.NewRegistry().ProjectAccounts(nil)
		if !errors.Is(err, model.ErrProjectNil) {
			t.Errorf("got %v, want ErrProjectNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewProject("Ghost", "", "")
		_, err := model.NewRegistry().ProjectAccounts(ghost)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})
	t.Run("none assigned", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		got, err := reg.ProjectAccounts(reg.Project("Orphan"))
		must(t, err)
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
	t.Run("assigned, sorted by key", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		got, err := reg.ProjectAccounts(reg.Project("Alpha"))
		must(t, err)
		if len(got) != 1 {
			t.Fatalf("got %d accounts, want 1", len(got))
		}
		if got[0].UserEmail() != "alice@example.com" {
			t.Errorf("got %q, want alice@example.com", got[0].UserEmail())
		}
	})
}

func TestRegistry_ProjectSessions(t *testing.T) {
	t.Parallel()
	t.Run("nil project", func(t *testing.T) {
		t.Parallel()
		_, err := model.NewRegistry().ProjectSessions(nil)
		if !errors.Is(err, model.ErrProjectNil) {
			t.Errorf("got %v, want ErrProjectNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewProject("Ghost", "", "")
		_, err := model.NewRegistry().ProjectSessions(ghost)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		got, err := reg.ProjectSessions(reg.Project("Beta"))
		must(t, err)
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
	t.Run("has sessions, sorted by ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		got, err := reg.ProjectSessions(reg.Project("Alpha"))
		must(t, err)
		if len(got) != 1 || got[0].ID() != 1 {
			t.Errorf("got %v, want one session with ID 1", got)
		}
	})
}

// ── Account CRUD ──────────────────────────────────────────────────────────────

func TestRegistry_AddAccount(t *testing.T) {
	t.Parallel()
	t.Run("success returns the new account", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		if acc == nil {
			t.Fatal("got nil account")
		}
		if acc.Key() != (model.AccountKey{Email: "alice@example.com", Vendor: "claude"}) {
			t.Errorf("got key %+v, want alice@example.com|claude", acc.Key())
		}
	})
	t.Run("vendor nil", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		_, err := reg.AddAccount(reg.User("alice@example.com"), nil)
		if !errors.Is(err, model.ErrVendorNil) {
			t.Errorf("got %v, want ErrVendorNil", err)
		}
	})
	t.Run("vendor not found", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		ghost := model.NewVendor("ghost", false, false)
		_, err := reg.AddAccount(reg.User("alice@example.com"), ghost)
		if !errors.Is(err, model.ErrVendorNotFound) {
			t.Errorf("got %v, want ErrVendorNotFound", err)
		}
	})
	t.Run("vendor stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		must(t, reg.AddVendor(model.NewVendor("claude", false, false)))
		stale := reg.Vendor("claude")
		must(t, reg.UpdateVendor(stale, model.NewVendor("claude", true, false)))
		_, err := reg.AddAccount(reg.User("alice@example.com"), stale)
		if !errors.Is(err, model.ErrVendorStale) {
			t.Errorf("got %v, want ErrVendorStale", err)
		}
	})
	t.Run("user nil", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		_, err := reg.AddAccount(nil, reg.Vendor("claude"))
		if !errors.Is(err, model.ErrUserNil) {
			t.Errorf("got %v, want ErrUserNil", err)
		}
	})
	t.Run("user not found", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		ghost := model.NewUser("ghost@example.com", "Ghost")
		_, err := reg.AddAccount(ghost, reg.Vendor("claude"))
		if !errors.Is(err, model.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})
	t.Run("user stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		stale := reg.User("alice@example.com")
		must(t, reg.UpdateUser(stale, model.NewUser("alice@example.com", "Alice Smith")))
		_, err := reg.AddAccount(stale, reg.Vendor("claude"))
		if !errors.Is(err, model.ErrUserStale) {
			t.Errorf("got %v, want ErrUserStale", err)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		_, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		if !errors.Is(err, model.ErrAccountExists) {
			t.Errorf("got %v, want ErrAccountExists", err)
		}
	})
}

func TestRegistry_Account(t *testing.T) {
	t.Parallel()
	t.Run("found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		if acc == nil {
			t.Fatal("got nil")
		}
		if acc.UserEmail() != "alice@example.com" || acc.VendorName() != "claude" {
			t.Errorf("got %+v, want alice@example.com|claude", acc.Key())
		}
	})
	t.Run("vendor verify-fail returns nil", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewVendor("ghost", false, false)
		if acc := reg.Account(reg.User("alice@example.com"), ghost); acc != nil {
			t.Errorf("got %v, want nil", acc)
		}
	})
	t.Run("user verify-fail returns nil", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewUser("ghost@example.com", "Ghost")
		if acc := reg.Account(ghost, reg.Vendor("claude")); acc != nil {
			t.Errorf("got %v, want nil", acc)
		}
	})
	t.Run("valid pointers but no account exists", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		// bob exists and chatgpt exists, but alice never has a chatgpt account.
		if acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("chatgpt")); acc != nil {
			t.Errorf("got %v, want nil", acc)
		}
	})
}

func TestRegistry_Accounts(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if got := model.NewRegistry().Accounts(); len(got) != 0 {
			t.Errorf("got %d accounts, want 0", len(got))
		}
	})
	t.Run("sorted by key", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		got := reg.Accounts()
		if len(got) != 2 {
			t.Fatalf("got %d accounts, want 2", len(got))
		}
		if got[0].UserEmail() != "alice@example.com" || got[1].UserEmail() != "bob@example.com" {
			t.Errorf("got %q, %q; want alice, bob in order", got[0].UserEmail(), got[1].UserEmail())
		}
	})
}

func TestRegistry_DeleteAccount(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		must(t, reg.DeleteAccount(acc))
		if reg.Account(reg.User("alice@example.com"), reg.Vendor("claude")) != nil {
			t.Error("account still present after delete")
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().DeleteAccount(nil)
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewAccount("ghost@example.com", "ghost")
		err := model.NewRegistry().DeleteAccount(ghost)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		must(t, reg.PauseAccount(acc, time.Now().Add(time.Hour)))
		err = reg.DeleteAccount(acc)
		if !errors.Is(err, model.ErrAccountStale) {
			t.Errorf("got %v, want ErrAccountStale", err)
		}
	})
	t.Run("has active sessions", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
		err := reg.DeleteAccount(acc)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
	t.Run("has assigned projects but no sessions", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))

		err = reg.DeleteAccount(acc)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestRegistry_PauseAccount(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)

		until := time.Now().Add(time.Hour).Truncate(time.Second)
		must(t, reg.PauseAccount(acc, until))

		got := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		if !got.PauseUntil().Equal(until) {
			t.Errorf("got %v, want %v", got.PauseUntil(), until)
		}
		// The original pointer must now be stale: CoW replaced it in the map.
		if got == acc {
			t.Error("expected a new pointer after PauseAccount, got the same one")
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().PauseAccount(nil, time.Now())
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewAccount("ghost@example.com", "ghost")
		err := model.NewRegistry().PauseAccount(ghost, time.Now())
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		must(t, reg.PauseAccount(acc, time.Now().Add(time.Hour)))
		err = reg.PauseAccount(acc, time.Now().Add(2*time.Hour))
		if !errors.Is(err, model.ErrAccountStale) {
			t.Errorf("got %v, want ErrAccountStale", err)
		}
	})
}

func TestRegistry_AddProjectToAccount(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, reg.AddProjectToAccount(acc, reg.Project("Orphan")))
		got, err := reg.AccountProjects(acc)
		must(t, err)
		if len(got) != 3 {
			t.Errorf("got %d projects, want 3", len(got))
		}
	})
	t.Run("account nil", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		err := reg.AddProjectToAccount(nil, reg.Project("Alpha"))
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})
	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewAccount("ghost@example.com", "claude")
		err := reg.AddProjectToAccount(ghost, reg.Project("Alpha"))
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
	t.Run("account stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, reg.PauseAccount(acc, time.Now().Add(time.Hour)))
		err := reg.AddProjectToAccount(acc, reg.Project("Orphan"))
		if !errors.Is(err, model.ErrAccountStale) {
			t.Errorf("got %v, want ErrAccountStale", err)
		}
	})
	t.Run("project nil", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		err := reg.AddProjectToAccount(acc, nil)
		if !errors.Is(err, model.ErrProjectNil) {
			t.Errorf("got %v, want ErrProjectNil", err)
		}
	})
	t.Run("project not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		ghost := model.NewProject("Ghost", "", "")
		err := reg.AddProjectToAccount(acc, ghost)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})
	t.Run("project stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		stale := reg.Project("Orphan")
		must(t, reg.UpdateProject(stale, model.NewProject("Orphan", "new", "")))
		err := reg.AddProjectToAccount(acc, stale)
		if !errors.Is(err, model.ErrProjectStale) {
			t.Errorf("got %v, want ErrProjectStale", err)
		}
	})
	t.Run("vendor does not support projects", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
		err := reg.AddProjectToAccount(acc, reg.Project("Orphan"))
		if !errors.Is(err, model.ErrProjectsUnsupported) {
			t.Errorf("got %v, want ErrProjectsUnsupported", err)
		}
	})
	t.Run("already assigned", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		err := reg.AddProjectToAccount(acc, reg.Project("Alpha"))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestRegistry_RemoveProjectFromAccount(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, reg.RemoveProjectFromAccount(acc, reg.Project("Beta")))
		got, err := reg.AccountProjects(acc)
		must(t, err)
		if len(got) != 1 || got[0].Name() != "Alpha" {
			t.Errorf("got %v, want only Alpha remaining", got)
		}
	})
	t.Run("account nil", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		err := reg.RemoveProjectFromAccount(nil, reg.Project("Alpha"))
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})
	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewAccount("ghost@example.com", "claude")
		err := reg.RemoveProjectFromAccount(ghost, reg.Project("Alpha"))
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
	t.Run("project nil", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		err := reg.RemoveProjectFromAccount(acc, nil)
		if !errors.Is(err, model.ErrProjectNil) {
			t.Errorf("got %v, want ErrProjectNil", err)
		}
	})
	t.Run("project not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		ghost := model.NewProject("Ghost", "", "")
		err := reg.RemoveProjectFromAccount(acc, ghost)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})
	t.Run("not assigned", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		err := reg.RemoveProjectFromAccount(acc, reg.Project("Orphan"))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
	t.Run("has active sessions", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		// Session 1 uses Alpha for this account.
		err := reg.RemoveProjectFromAccount(acc, reg.Project("Alpha"))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestRegistry_AccountProjects(t *testing.T) {
	t.Parallel()
	t.Run("nil account", func(t *testing.T) {
		t.Parallel()
		_, err := model.NewRegistry().AccountProjects(nil)
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewAccount("ghost@example.com", "claude")
		_, err := model.NewRegistry().AccountProjects(ghost)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
	t.Run("none assigned", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
		got, err := reg.AccountProjects(acc)
		must(t, err)
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
	t.Run("assigned, sorted by name", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		got, err := reg.AccountProjects(acc)
		must(t, err)
		if len(got) != 2 || got[0].Name() != "Alpha" || got[1].Name() != "Beta" {
			t.Errorf("got %v, want [Alpha Beta]", got)
		}
	})
}

func TestRegistry_AccountSessions(t *testing.T) {
	t.Parallel()
	t.Run("nil account", func(t *testing.T) {
		t.Parallel()
		_, err := model.NewRegistry().AccountSessions(nil)
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewAccount("ghost@example.com", "claude")
		_, err := model.NewRegistry().AccountSessions(ghost)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
	t.Run("has sessions, sorted by ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		got, err := reg.AccountSessions(acc)
		must(t, err)
		if len(got) != 1 || got[0].ID() != 1 {
			t.Errorf("got %v, want one session with ID 1", got)
		}
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		got, err := reg.AccountSessions(acc)
		must(t, err)
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
}

// ── Session CRUD ──────────────────────────────────────────────────────────────

func TestRegistry_AddSession_Success(t *testing.T) {
	t.Parallel()
	t.Run("success without project", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		sess, err := reg.AddSession(acc, nil, "Name", nil)
		must(t, err)
		if sess.ProjectName() != "" {
			t.Errorf("got project %q, want empty", sess.ProjectName())
		}
		if sess.ID() == 0 {
			t.Error("expected a non-zero ID")
		}
		if sess.Status() != model.SessionStatusActive {
			t.Errorf("Status() = %v, want Active", sess.Status())
		}
	})
	t.Run("success with project", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		queue := []model.Prompt{model.NewPrompt("continue")}
		sess, err := reg.AddSession(acc, reg.Project("Beta"), "Name", queue)
		must(t, err)
		if sess.ProjectName() != "Beta" {
			t.Errorf("got project %q, want Beta", sess.ProjectName())
		}
		if sess.AccountID() != acc.ID() {
			t.Errorf("AccountID() = %d, want %d", sess.AccountID(), acc.ID())
		}
		if sess.ProjectID() != reg.Project("Beta").ID() {
			t.Errorf("ProjectID() = %d, want %d", sess.ProjectID(), reg.Project("Beta").ID())
		}
		if len(sess.Queue()) != 1 {
			t.Errorf("got %d queue items, want 1", len(sess.Queue()))
		}
		if sess.Status() != model.SessionStatusActive {
			t.Errorf("Status() = %v, want Active", sess.Status())
		}
		got, err := reg.ProjectSessions(reg.Project("Beta"))
		must(t, err)
		if len(got) != 1 {
			t.Errorf("expected the new session to be indexed under Beta, got %v", got)
		}
	})
}

func TestRegistry_AddSession_Validation(t *testing.T) {
	t.Parallel()
	t.Run("account nil", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		_, err := reg.AddSession(nil, nil, "", nil)
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})
	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewAccount("ghost@example.com", "claude")
		_, err := reg.AddSession(ghost, nil, "", nil)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
	t.Run("account stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, reg.PauseAccount(acc, time.Now().Add(time.Hour)))
		_, err := reg.AddSession(acc, nil, "", nil)
		if !errors.Is(err, model.ErrAccountStale) {
			t.Errorf("got %v, want ErrAccountStale", err)
		}
	})
	t.Run("invalid prompt in queue", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		queue := []model.Prompt{
			model.NewPrompt("ok"),
			model.NewPrompt(""),
		}
		_, err := reg.AddSession(acc, nil, "", queue)
		if !errors.Is(err, model.ErrTextRequired) {
			t.Errorf("got %v, want ErrTextRequired", err)
		}
	})
	t.Run("project not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		ghost := model.NewProject("Ghost", "", "")
		_, err := reg.AddSession(acc, ghost, "", nil)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})
	t.Run("project stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		stale := reg.Project("Alpha")
		must(t, reg.UpdateProject(stale, model.NewProject("Alpha", "new", "")))
		_, err := reg.AddSession(acc, stale, "", nil)
		if !errors.Is(err, model.ErrProjectStale) {
			t.Errorf("got %v, want ErrProjectStale", err)
		}
	})
	t.Run("project not assigned to account", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.AddSession(acc, reg.Project("Orphan"), "", nil)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestRegistry_AddSession_GlobalSequence(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
	aliceAcc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	bobAcc, err := reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("claude"))
	must(t, err)

	s1, err := reg.AddSession(aliceAcc, nil, "", nil)
	must(t, err)
	s2, err := reg.AddSession(bobAcc, nil, "", nil)
	must(t, err)
	s3, err := reg.AddSession(aliceAcc, nil, "", nil)
	must(t, err)

	if s1.ID() != 1 || s2.ID() != 2 || s3.ID() != 3 {
		t.Errorf("got IDs %d, %d, %d across accounts; want 1, 2, 3 (a single global sequence)", s1.ID(), s2.ID(), s3.ID())
	}
}

func TestRegistry_Session(t *testing.T) {
	t.Parallel()
	reg := seedRegistry(t)
	if s := reg.Session(1); s == nil {
		t.Error("expected to find session 1")
	}
	if s := reg.Session(99); s != nil {
		t.Error("expected nil for unknown session")
	}
	if s := reg.Session(0); s != nil {
		t.Error("expected nil for ID 0, which is never assigned")
	}
}

func TestRegistry_Sessions(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if got := model.NewRegistry().Sessions(); len(got) != 0 {
			t.Errorf("got %d sessions, want 0", len(got))
		}
	})
	t.Run("sorted by ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		got := reg.Sessions()
		if len(got) != 2 || got[0].ID() != 1 || got[1].ID() != 2 {
			t.Errorf("got %v, want IDs 1, 2 in order", got)
		}
	})
}

func TestRegistry_DeleteSession(t *testing.T) {
	t.Parallel()
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().DeleteSession(nil)
		if !errors.Is(err, model.ErrSessionNil) {
			t.Errorf("got %v, want ErrSessionNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewSession(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, 999, "", "", nil)
		err := reg.DeleteSession(ghost)
		if !errors.Is(err, model.ErrSessionNotFound) {
			t.Errorf("got %v, want ErrSessionNotFound", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		stale := reg.Session(1)
		must(t, reg.UpdateSessionName(stale, "changed"))
		err := reg.DeleteSession(stale)
		if !errors.Is(err, model.ErrSessionStale) {
			t.Errorf("got %v, want ErrSessionStale", err)
		}
	})
	t.Run("success cleans account and project indexes", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))

		must(t, reg.DeleteSession(sess))

		if reg.Session(1) != nil {
			t.Error("session 1 should no longer be retrievable")
		}
		pSessions, err := reg.ProjectSessions(reg.Project("Alpha"))
		must(t, err)
		if len(pSessions) != 0 {
			t.Errorf("ProjectSessions(Alpha) = %v, want empty", pSessions)
		}
		aSessions, err := reg.AccountSessions(acc)
		must(t, err)
		if len(aSessions) != 0 {
			t.Errorf("AccountSessions(alice|claude) = %v, want empty", aSessions)
		}
	})
	t.Run("success without a project leaves project index untouched", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(2) // bob|chatgpt, no project
		must(t, reg.DeleteSession(sess))
		if reg.Session(2) != nil {
			t.Error("session 2 should no longer be retrievable")
		}
	})
}

func TestRegistry_UpdateSessionName(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		must(t, reg.UpdateSessionName(sess, "New Name"))
		got := reg.Session(1)
		if got.Name() != "New Name" {
			t.Errorf("got %q, want %q", got.Name(), "New Name")
		}
		if got == sess {
			t.Error("expected a new pointer after a real change")
		}
	})
	t.Run("no-op when name is unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		before := reg.Session(1)
		must(t, reg.UpdateSessionName(before, before.Name()))
		after := reg.Session(1)
		if before != after {
			t.Error("expected the same pointer when the name does not change (no-op CoW)")
		}
	})
	t.Run("trims and normalizes before comparing", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		before := reg.Session(1) // name is "Architecture review"
		must(t, reg.UpdateSessionName(before, "  Architecture review  "))
		after := reg.Session(1)
		if before != after {
			t.Error("expected the same pointer: normalized name is unchanged")
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().UpdateSessionName(nil, "x")
		if !errors.Is(err, model.ErrSessionNil) {
			t.Errorf("got %v, want ErrSessionNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewSession(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, 999, "", "", nil)
		err := reg.UpdateSessionName(ghost, "x")
		if !errors.Is(err, model.ErrSessionNotFound) {
			t.Errorf("got %v, want ErrSessionNotFound", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		stale := reg.Session(1)
		must(t, reg.UpdateSessionName(stale, "first change"))
		err := reg.UpdateSessionName(stale, "second change")
		if !errors.Is(err, model.ErrSessionStale) {
			t.Errorf("got %v, want ErrSessionStale", err)
		}
	})
}
func TestRegistry_UpdateSessionProject(t *testing.T) {
	t.Parallel()

	t.Run("assign", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		newSess, err := reg.AddSession(acc, nil, "", nil)
		must(t, err)
		must(t, reg.UpdateSessionProject(newSess, reg.Project("Beta")))
		got := reg.Session(newSess.ID())
		if got.ProjectName() != "Beta" {
			t.Errorf("got %q, want Beta", got.ProjectName())
		}
		if got.ProjectID() != reg.Project("Beta").ID() {
			t.Errorf("ProjectID() = %d, want %d", got.ProjectID(), reg.Project("Beta").ID())
		}
		pSessions, err := reg.ProjectSessions(reg.Project("Beta"))
		must(t, err)
		if len(pSessions) != 1 {
			t.Errorf("ProjectSessions(Beta) = %v, want the new session indexed", pSessions)
		}
	})

	t.Run("unassign", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		must(t, reg.UpdateSessionProject(sess, nil))
		got := reg.Session(1)
		if got.ProjectName() != "" {
			t.Errorf("got %q, want empty", got.ProjectName())
		}
		pSessions, err := reg.ProjectSessions(reg.Project("Alpha"))
		must(t, err)
		if len(pSessions) != 0 {
			t.Errorf("ProjectSessions(Alpha) = %v, want empty after unassign", pSessions)
		}
	})

	t.Run("switch project", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		must(t, reg.UpdateSessionProject(sess, reg.Project("Beta")))
		got := reg.Session(1)
		if got.ProjectName() != "Beta" {
			t.Errorf("got %q, want Beta", got.ProjectName())
		}
		pSessionsAlpha, err := reg.ProjectSessions(reg.Project("Alpha"))
		must(t, err)
		if len(pSessionsAlpha) != 0 {
			t.Errorf("ProjectSessions(Alpha) = %v, want empty after switching away", pSessionsAlpha)
		}
		pSessionsBeta, err := reg.ProjectSessions(reg.Project("Beta"))
		must(t, err)
		if len(pSessionsBeta) != 1 {
			t.Errorf("ProjectSessions(Beta) = %v, want the session indexed after switching to it", pSessionsBeta)
		}
	})

	t.Run("no-op when project is unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		before := reg.Session(1) // project=Alpha
		must(t, reg.UpdateSessionProject(before, reg.Project("Alpha")))
		after := reg.Session(1)
		if before != after {
			t.Error("expected the same pointer when the project does not change (no-op CoW)")
		}
	})

	t.Run("no-op unassigning an already-unassigned session", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		before := reg.Session(2) // bob|chatgpt, no project
		must(t, reg.UpdateSessionProject(before, nil))
		after := reg.Session(2)
		if before != after {
			t.Error("expected the same pointer: already unassigned")
		}
	})
}

func TestRegistry_UpdateSessionProject_Errors(t *testing.T) {
	t.Parallel()

	t.Run("nil session", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		err := reg.UpdateSessionProject(nil, reg.Project("Alpha"))
		if !errors.Is(err, model.ErrSessionNil) {
			t.Errorf("got %v, want ErrSessionNil", err)
		}
	})

	t.Run("session not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewSession(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, 999, "", "", nil)
		err := reg.UpdateSessionProject(ghost, reg.Project("Alpha"))
		if !errors.Is(err, model.ErrSessionNotFound) {
			t.Errorf("got %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("session stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		stale := reg.Session(1)
		must(t, reg.UpdateSessionProject(stale, reg.Project("Beta")))
		err := reg.UpdateSessionProject(stale, nil)
		if !errors.Is(err, model.ErrSessionStale) {
			t.Errorf("got %v, want ErrSessionStale", err)
		}
	})

	t.Run("project not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		ghost := model.NewProject("Ghost", "", "")
		err := reg.UpdateSessionProject(sess, ghost)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("project stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		stale := reg.Project("Beta")
		must(t, reg.UpdateProject(stale, model.NewProject("Beta", "new", "")))
		err := reg.UpdateSessionProject(sess, stale)
		if !errors.Is(err, model.ErrProjectStale) {
			t.Errorf("got %v, want ErrProjectStale", err)
		}
	})

	t.Run("project not assigned to account", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1) // alice|claude
		err := reg.UpdateSessionProject(sess, reg.Project("Orphan"))
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestRegistry_UpdateSessionPrompts(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		newQueue := []model.Prompt{model.NewPrompt("step one"), model.NewPrompt("step two")}
		must(t, reg.UpdateSessionPrompts(sess, newQueue))
		got := reg.Session(1).Queue()
		if len(got) != 2 || got[0].Text() != "step one" || got[1].Text() != "step two" {
			t.Errorf("got %+v, want two prompts in order", got)
		}
	})
	t.Run("no-op when queue is unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		queue := []model.Prompt{model.NewPrompt("same")}
		sess, err := reg.AddSession(reg.Account(reg.User("alice@example.com"), reg.Vendor("claude")), nil, "", queue)
		must(t, err)
		before := reg.Session(sess.ID())
		must(t, reg.UpdateSessionPrompts(before, queue))
		after := reg.Session(sess.ID())
		if before != after {
			t.Error("expected the same pointer when the queue does not change (no-op CoW)")
		}
	})
	t.Run("invalid prompt", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		err := reg.UpdateSessionPrompts(sess, []model.Prompt{model.NewPrompt("")})
		if !errors.Is(err, model.ErrTextRequired) {
			t.Errorf("got %v, want ErrTextRequired", err)
		}
	})
	t.Run("nil session", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().UpdateSessionPrompts(nil, nil)
		if !errors.Is(err, model.ErrSessionNil) {
			t.Errorf("got %v, want ErrSessionNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewSession(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, 999, "", "", nil)
		err := reg.UpdateSessionPrompts(ghost, nil)
		if !errors.Is(err, model.ErrSessionNotFound) {
			t.Errorf("got %v, want ErrSessionNotFound", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		stale := reg.Session(1)
		must(t, reg.UpdateSessionPrompts(stale, []model.Prompt{model.NewPrompt("first update")}))
		err := reg.UpdateSessionPrompts(stale, []model.Prompt{model.NewPrompt("second update")})
		if !errors.Is(err, model.ErrSessionStale) {
			t.Errorf("got %v, want ErrSessionStale", err)
		}
	})
}

func TestRegistry_AddSession_UpdatedAt(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, 10, 30, 9, 15, 0, 0, time.UTC)

	t.Run("non-empty queue sets updated-at from the clock", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		reg.SetClock(func() time.Time { return fixed })
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		sess, err := reg.AddSession(acc, nil, "Name", []model.Prompt{model.NewPrompt("continue")})
		must(t, err)
		if !sess.UpdatedAt().Equal(fixed) {
			t.Errorf("UpdatedAt() = %v, want %v", sess.UpdatedAt(), fixed)
		}
	})

	t.Run("empty queue leaves updated-at zero", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		reg.SetClock(func() time.Time { return fixed })
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		sess, err := reg.AddSession(acc, nil, "Name", nil)
		must(t, err)
		if !sess.UpdatedAt().IsZero() {
			t.Errorf("UpdatedAt() = %v, want zero", sess.UpdatedAt())
		}
	})

	t.Run("clock is per registry instance", func(t *testing.T) {
		t.Parallel()
		other := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
		regA := seedRegistry(t)
		regA.SetClock(func() time.Time { return fixed })
		regB := seedRegistry(t)
		regB.SetClock(func() time.Time { return other })

		accA := regA.Account(regA.User("alice@example.com"), regA.Vendor("claude"))
		accB := regB.Account(regB.User("alice@example.com"), regB.Vendor("claude"))
		queue := []model.Prompt{model.NewPrompt("continue")}
		sessA, err := regA.AddSession(accA, nil, "", queue)
		must(t, err)
		sessB, err := regB.AddSession(accB, nil, "", queue)
		must(t, err)

		if !sessA.UpdatedAt().Equal(fixed) {
			t.Errorf("registry A UpdatedAt() = %v, want %v", sessA.UpdatedAt(), fixed)
		}
		if !sessB.UpdatedAt().Equal(other) {
			t.Errorf("registry B UpdatedAt() = %v, want %v", sessB.UpdatedAt(), other)
		}
	})

	t.Run("clock result is converted to UTC", func(t *testing.T) {
		t.Parallel()
		loc := time.FixedZone("test", 2*60*60)
		local := time.Date(2026, 10, 30, 11, 15, 0, 0, loc)
		reg := seedRegistry(t)
		reg.SetClock(func() time.Time { return local })
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		sess, err := reg.AddSession(acc, nil, "", []model.Prompt{model.NewPrompt("continue")})
		must(t, err)
		if got := sess.UpdatedAt().Location(); got != time.UTC {
			t.Errorf("UpdatedAt().Location() = %v, want UTC", got)
		}
		if !sess.UpdatedAt().Equal(local) {
			t.Errorf("UpdatedAt() = %v, want instant %v", sess.UpdatedAt(), local)
		}
	})
}

func TestRegistry_UpdateSessionPrompts_UpdatedAt(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 10, 30, 9, 15, 0, 0, time.UTC)
	second := first.Add(time.Hour)

	t.Run("changed queue sets updated-at", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		reg.SetClock(func() time.Time { return first })
		sess := reg.Session(1)
		must(t, reg.UpdateSessionPrompts(sess, []model.Prompt{model.NewPrompt("next")}))
		if got := reg.Session(1).UpdatedAt(); !got.Equal(first) {
			t.Errorf("UpdatedAt() = %v, want %v", got, first)
		}
	})

	t.Run("no-op queue leaves pointer and updated-at unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		reg.SetClock(func() time.Time { return first })
		sess := reg.Session(1)
		before := sess.UpdatedAt()
		reg.SetClock(func() time.Time { return second })
		must(t, reg.UpdateSessionPrompts(sess, sess.Queue()))
		after := reg.Session(1)
		if after != sess {
			t.Error("expected the same pointer for an unchanged queue")
		}
		if !after.UpdatedAt().Equal(before) {
			t.Errorf("UpdatedAt() = %v, want unchanged %v", after.UpdatedAt(), before)
		}
	})

	t.Run("empty queue is a change when the previous queue was non-empty", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		reg.SetClock(func() time.Time { return first })
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		sess, err := reg.AddSession(acc, nil, "", []model.Prompt{model.NewPrompt("one")})
		must(t, err)
		reg.SetClock(func() time.Time { return second })
		must(t, reg.UpdateSessionPrompts(sess, nil))
		got := reg.Session(sess.ID())
		if len(got.Queue()) != 0 {
			t.Errorf("Queue() = %v, want empty", got.Queue())
		}
		if !got.UpdatedAt().Equal(second) {
			t.Errorf("UpdatedAt() = %v, want %v", got.UpdatedAt(), second)
		}
	})
}

func TestRegistry_UpdateSessionURL(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		must(t, reg.UpdateSessionURL(sess, " https://example.com/chat/1 "))
		got := reg.Session(1)
		if got.URL() != "https://example.com/chat/1" {
			t.Errorf("URL() = %q, want trimmed URL", got.URL())
		}
		if got == sess {
			t.Error("expected a new pointer after a real change")
		}
		if !got.UpdatedAt().Equal(sess.UpdatedAt()) {
			t.Error("URL update must not change updated-at")
		}
	})

	t.Run("no-op when unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		must(t, reg.UpdateSessionURL(sess, "https://example.com/chat/1"))
		before := reg.Session(1)
		must(t, reg.UpdateSessionURL(before, "  https://example.com/chat/1  "))
		if got := reg.Session(1); got != before {
			t.Error("expected the same pointer for an unchanged URL")
		}
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().UpdateSessionURL(nil, "x")
		if !errors.Is(err, model.ErrSessionNil) {
			t.Errorf("got %v, want ErrSessionNil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewSession(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, 999, "", "", nil)
		err := reg.UpdateSessionURL(ghost, "x")
		if !errors.Is(err, model.ErrSessionNotFound) {
			t.Errorf("got %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		stale := reg.Session(1)
		must(t, reg.UpdateSessionURL(stale, "first"))
		err := reg.UpdateSessionURL(stale, "second")
		if !errors.Is(err, model.ErrSessionStale) {
			t.Errorf("got %v, want ErrSessionStale", err)
		}
	})
}

func TestRegistry_UpdateSessionStatus(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		must(t, reg.UpdateSessionStatus(sess, model.SessionStatusNeedsClosing))
		got := reg.Session(1)
		if got.Status() != model.SessionStatusNeedsClosing {
			t.Errorf("Status() = %v, want Needs closing", got.Status())
		}
		if got == sess {
			t.Error("expected a new pointer after a real change")
		}
		if !got.UpdatedAt().Equal(sess.UpdatedAt()) {
			t.Error("status update must not change updated-at")
		}
	})

	t.Run("any status to any status", func(t *testing.T) {
		t.Parallel()
		statuses := []model.SessionStatus{
			model.SessionStatusActive,
			model.SessionStatusNeedsClosing,
			model.SessionStatusClosed,
		}
		for _, from := range statuses {
			for _, to := range statuses {
				t.Run(from.String()+" to "+to.String(), func(t *testing.T) {
					t.Parallel()
					reg := seedRegistry(t)
					sess := reg.Session(1)
					must(t, reg.UpdateSessionStatus(sess, from))
					sess = reg.Session(1)
					must(t, reg.UpdateSessionStatus(sess, to))
					if got := reg.Session(1).Status(); got != to {
						t.Errorf("Status() = %v, want %v", got, to)
					}
				})
			}
		}
	})

	t.Run("no-op when unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		must(t, reg.UpdateSessionStatus(sess, model.SessionStatusActive))
		if got := reg.Session(1); got != sess {
			t.Error("expected the same pointer for an unchanged status")
		}
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().UpdateSessionStatus(nil, model.SessionStatusNeedsClosing)
		if !errors.Is(err, model.ErrSessionNil) {
			t.Errorf("got %v, want ErrSessionNil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewSession(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, 999, "", "", nil)
		err := reg.UpdateSessionStatus(ghost, model.SessionStatusNeedsClosing)
		if !errors.Is(err, model.ErrSessionNotFound) {
			t.Errorf("got %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		stale := reg.Session(1)
		must(t, reg.UpdateSessionStatus(stale, model.SessionStatusNeedsClosing))
		err := reg.UpdateSessionStatus(stale, model.SessionStatusClosed)
		if !errors.Is(err, model.ErrSessionStale) {
			t.Errorf("got %v, want ErrSessionStale", err)
		}
	})
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestRegistry_LatestSession(t *testing.T) {
	t.Parallel()

	t.Run("greatest non-zero updated-at wins", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		early := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
		late := early.Add(time.Hour)

		reg.SetClock(func() time.Time { return early })
		s1, err := reg.AddSession(acc, nil, "early", []model.Prompt{model.NewPrompt("a")})
		must(t, err)
		reg.SetClock(func() time.Time { return late })
		s2, err := reg.AddSession(acc, nil, "late", []model.Prompt{model.NewPrompt("b")})
		must(t, err)

		got, ok, err := reg.LatestSession(acc)
		must(t, err)
		if !ok || got != s2 {
			t.Errorf("LatestSession() = %v, %v; want session %d", got, ok, s2.ID())
		}
		if got.ID() == s1.ID() {
			t.Error("expected the later session, got the earlier one")
		}
	})

	t.Run("equal updated-at uses the greater ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		at := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
		reg.SetClock(func() time.Time { return at })

		s1, err := reg.AddSession(acc, nil, "first", []model.Prompt{model.NewPrompt("a")})
		must(t, err)
		s2, err := reg.AddSession(acc, nil, "second", []model.Prompt{model.NewPrompt("b")})
		must(t, err)

		got, ok, err := reg.LatestSession(acc)
		must(t, err)
		if !ok || got.ID() != s2.ID() || got.ID() <= s1.ID() {
			t.Errorf("LatestSession() = %v, want session %d", got, s2.ID())
		}
	})

	t.Run("never-touched sessions are ignored", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		at := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
		reg.SetClock(func() time.Time { return at })

		touched, err := reg.AddSession(acc, nil, "touched", []model.Prompt{model.NewPrompt("a")})
		must(t, err)
		_, err = reg.AddSession(acc, nil, "never", nil)
		must(t, err)

		got, ok, err := reg.LatestSession(acc)
		must(t, err)
		if !ok || got.ID() != touched.ID() {
			t.Errorf("LatestSession() = %v, want the touched session", got)
		}
	})

	t.Run("no touched session", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.AddSession(acc, nil, "never", nil)
		must(t, err)

		got, ok, err := reg.LatestSession(acc)
		must(t, err)
		if ok || got != nil {
			t.Errorf("LatestSession() = %v, %v; want nil, false", got, ok)
		}
	})

	t.Run("account with no sessions", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)

		got, ok, err := reg.LatestSession(acc)
		must(t, err)
		if ok || got != nil {
			t.Errorf("LatestSession() = %v, %v; want nil, false", got, ok)
		}
	})

	t.Run("nil account", func(t *testing.T) {
		t.Parallel()
		_, _, err := model.NewRegistry().LatestSession(nil)
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewAccount("ghost@example.com", "claude")
		_, _, err := model.NewRegistry().LatestSession(ghost)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, reg.PauseAccount(acc, time.Now().Add(time.Hour)))
		_, _, err := reg.LatestSession(acc)
		if !errors.Is(err, model.ErrAccountStale) {
			t.Errorf("got %v, want ErrAccountStale", err)
		}
	})
}

func TestRegistry_AccountSessionsActivityOrder(t *testing.T) {
	t.Parallel()

	t.Run("updated-at descending then ID descending", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		at1 := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
		at2 := at1.Add(time.Hour)

		reg.SetClock(func() time.Time { return at1 })
		s1, err := reg.AddSession(acc, nil, "one", []model.Prompt{model.NewPrompt("a")})
		must(t, err)
		s2, err := reg.AddSession(acc, nil, "two", []model.Prompt{model.NewPrompt("b")})
		must(t, err)
		reg.SetClock(func() time.Time { return at2 })
		s3, err := reg.AddSession(acc, nil, "three", []model.Prompt{model.NewPrompt("c")})
		must(t, err)
		s4, err := reg.AddSession(acc, nil, "four", nil)
		must(t, err)

		got, err := reg.AccountSessionsActivityOrder(acc)
		must(t, err)
		want := []uint64{s3.ID(), s2.ID(), s1.ID(), s4.ID()}
		if len(got) != len(want) {
			t.Fatalf("got %d sessions, want %d", len(got), len(want))
		}
		for i, id := range want {
			if got[i].ID() != id {
				t.Errorf("got[%d] = %d, want %d", i, got[i].ID(), id)
			}
		}
	})

	t.Run("nil account", func(t *testing.T) {
		t.Parallel()
		_, err := model.NewRegistry().AccountSessionsActivityOrder(nil)
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewAccount("ghost@example.com", "claude")
		_, err := model.NewRegistry().AccountSessionsActivityOrder(ghost)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestRegistry_AddNote(t *testing.T) {
	t.Parallel()

	t.Run("no project and no account", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		n, err := reg.AddNote("  implement export  ", nil, nil)
		must(t, err)
		if n.Text() != "implement export" {
			t.Errorf("Text() = %q, want trimmed text", n.Text())
		}
		if n.ProjectName() != "" {
			t.Errorf("ProjectName() = %q, want empty", n.ProjectName())
		}
		if _, ok := n.AccountKey(); ok {
			t.Error("AccountKey() reports an account, want none")
		}
		if n.ID() == 0 {
			t.Error("expected a non-zero ID")
		}
	})

	t.Run("project only", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		n, err := reg.AddNote("review", reg.Project("Alpha"), nil)
		must(t, err)
		if n.ProjectName() != "Alpha" {
			t.Errorf("ProjectName() = %q, want Alpha", n.ProjectName())
		}
		if n.ProjectID() != reg.Project("Alpha").ID() {
			t.Errorf("ProjectID() = %d, want %d", n.ProjectID(), reg.Project("Alpha").ID())
		}
		if _, ok := n.AccountKey(); ok {
			t.Error("AccountKey() reports an account, want none")
		}
	})

	t.Run("account only", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		n, err := reg.AddNote("review", nil, acc)
		must(t, err)
		got, ok := n.AccountKey()
		if !ok || got != acc.Key() {
			t.Errorf("AccountKey() = %v, %v; want %v, true", got, ok, acc.Key())
		}
	})

	t.Run("project and account", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		n, err := reg.AddNote("review", reg.Project("Alpha"), acc)
		must(t, err)
		if n.ProjectName() != "Alpha" {
			t.Errorf("ProjectName() = %q, want Alpha", n.ProjectName())
		}
		if got, ok := n.AccountKey(); !ok || got != acc.Key() {
			t.Errorf("AccountKey() = %v, %v; want %v, true", got, ok, acc.Key())
		}
	})

	t.Run("IDs are sequential", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		n1, err := reg.AddNote("one", nil, nil)
		must(t, err)
		n2, err := reg.AddNote("two", nil, nil)
		must(t, err)
		if n1.ID() != 1 || n2.ID() != 2 {
			t.Errorf("got IDs %d, %d; want 1, 2", n1.ID(), n2.ID())
		}
	})

	t.Run("empty text", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		_, err := reg.AddNote("   ", nil, nil)
		if !errors.Is(err, model.ErrTextRequired) {
			t.Errorf("got %v, want ErrTextRequired", err)
		}
	})

	t.Run("project not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		_, err := reg.AddNote("text", model.NewProject("Ghost", "", ""), nil)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("project stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		stale := reg.Project("Alpha")
		must(t, reg.UpdateProject(stale, model.NewProject("Alpha", "new", "")))
		_, err := reg.AddNote("text", stale, nil)
		if !errors.Is(err, model.ErrProjectStale) {
			t.Errorf("got %v, want ErrProjectStale", err)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		_, err := reg.AddNote("text", nil, model.NewAccount("ghost@example.com", "claude"))
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("account stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, reg.PauseAccount(acc, time.Now().Add(time.Hour)))
		_, err := reg.AddNote("text", nil, acc)
		if !errors.Is(err, model.ErrAccountStale) {
			t.Errorf("got %v, want ErrAccountStale", err)
		}
	})

	t.Run("project not assigned to account", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.AddNote("text", reg.Project("Orphan"), acc)
		if err == nil || !strings.Contains(err.Error(), `project "Orphan" is not assigned to this account`) {
			t.Errorf("got %v, want the assignment error", err)
		}
	})
}

func TestRegistry_UpdateNote(t *testing.T) {
	t.Parallel()

	t.Run("success replaces every value", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		current, err := reg.AddNote("old", nil, nil)
		must(t, err)

		must(t, reg.UpdateNote(current, "new", reg.Project("Alpha"), acc))
		got := reg.Note(current.ID())
		if got.Text() != "new" || got.ProjectName() != "Alpha" {
			t.Errorf("got text %q project %q, want new/Alpha", got.Text(), got.ProjectName())
		}
		if got.ProjectID() != reg.Project("Alpha").ID() {
			t.Errorf("ProjectID() = %d, want %d", got.ProjectID(), reg.Project("Alpha").ID())
		}
		if key, ok := got.AccountKey(); !ok || key != acc.Key() {
			t.Errorf("AccountKey() = %v, %v; want %v, true", key, ok, acc.Key())
		}
		if got == current {
			t.Error("expected a new pointer after a real change")
		}
	})

	t.Run("no-op when every value is unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		current, err := reg.AddNote("same", reg.Project("Alpha"), acc)
		must(t, err)
		must(t, reg.UpdateNote(current, "  same  ", reg.Project("Alpha"), acc))
		if got := reg.Note(current.ID()); got != current {
			t.Error("expected the same pointer for an unchanged note")
		}
	})

	t.Run("clear project and account", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		current, err := reg.AddNote("text", reg.Project("Alpha"), acc)
		must(t, err)
		must(t, reg.UpdateNote(current, "text", nil, nil))
		got := reg.Note(current.ID())
		if got.ProjectName() != "" {
			t.Errorf("ProjectName() = %q, want empty", got.ProjectName())
		}
		if _, ok := got.AccountKey(); ok {
			t.Error("AccountKey() reports an account, want none")
		}
	})

	t.Run("nil current", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().UpdateNote(nil, "text", nil, nil)
		if !errors.Is(err, model.ErrNoteNil) {
			t.Errorf("got %v, want ErrNoteNil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewNote(99, "text", 0, "", model.AccountKey{}, false)
		err := reg.UpdateNote(ghost, "new", nil, nil)
		if !errors.Is(err, model.ErrNoteNotFound) {
			t.Errorf("got %v, want ErrNoteNotFound", err)
		}
	})

	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		current, err := reg.AddNote("text", nil, nil)
		must(t, err)
		must(t, reg.UpdateNote(current, "first", nil, nil))
		err = reg.UpdateNote(current, "second", nil, nil)
		if !errors.Is(err, model.ErrNoteStale) {
			t.Errorf("got %v, want ErrNoteStale", err)
		}
	})

	t.Run("invalid text leaves the note unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		current, err := reg.AddNote("text", nil, nil)
		must(t, err)
		err = reg.UpdateNote(current, "  ", nil, nil)
		if !errors.Is(err, model.ErrTextRequired) {
			t.Errorf("got %v, want ErrTextRequired", err)
		}
		if got := reg.Note(current.ID()); got != current {
			t.Error("expected the note to be unchanged after a validation failure")
		}
	})
}

func TestRegistry_DeleteNote(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		n, err := reg.AddNote("text", nil, nil)
		must(t, err)
		must(t, reg.DeleteNote(n))
		if got := reg.Note(n.ID()); got != nil {
			t.Errorf("Note(%d) = %v, want nil", n.ID(), got)
		}
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().DeleteNote(nil)
		if !errors.Is(err, model.ErrNoteNil) {
			t.Errorf("got %v, want ErrNoteNil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewNote(99, "text", 0, "", model.AccountKey{}, false)
		err := reg.DeleteNote(ghost)
		if !errors.Is(err, model.ErrNoteNotFound) {
			t.Errorf("got %v, want ErrNoteNotFound", err)
		}
	})

	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		n, err := reg.AddNote("text", nil, nil)
		must(t, err)
		must(t, reg.UpdateNote(n, "new", nil, nil))
		err = reg.DeleteNote(n)
		if !errors.Is(err, model.ErrNoteStale) {
			t.Errorf("got %v, want ErrNoteStale", err)
		}
	})
}

func TestRegistry_NoteAndNotes(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if got := model.NewRegistry().Notes(); len(got) != 0 {
			t.Errorf("got %d notes, want 0", len(got))
		}
	})

	t.Run("sorted by ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		n1, err := reg.AddNote("one", nil, nil)
		must(t, err)
		n2, err := reg.AddNote("two", nil, nil)
		must(t, err)
		n3, err := reg.AddNote("three", nil, nil)
		must(t, err)

		got := reg.Notes()
		want := []uint64{n1.ID(), n2.ID(), n3.ID()}
		if len(got) != len(want) {
			t.Fatalf("got %d notes, want %d", len(got), len(want))
		}
		for i, id := range want {
			if got[i].ID() != id {
				t.Errorf("got[%d] = %d, want %d", i, got[i].ID(), id)
			}
		}
	})

	t.Run("lookup by ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		n, err := reg.AddNote("text", nil, nil)
		must(t, err)
		if got := reg.Note(n.ID()); got != n {
			t.Errorf("Note(%d) = %v, want %v", n.ID(), got, n)
		}
		if got := reg.Note(99); got != nil {
			t.Errorf("Note(99) = %v, want nil", got)
		}
	})
}

func TestProjectFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		filter model.ProjectFilter
		value  uint64
		want   bool
	}{
		{"all matches zero", model.AllProjectFilter(), 0, true},
		{"all matches named", model.AllProjectFilter(), 1, true},
		{"none matches zero", model.NoneProjectFilter(), 0, true},
		{"none rejects named", model.NoneProjectFilter(), 1, false},
		{"named matches same", model.NamedProjectFilter(1), 1, true},
		{"named rejects other", model.NamedProjectFilter(1), 2, false},
		{"named rejects zero", model.NamedProjectFilter(1), 0, false},
		{"zero value is all", model.ProjectFilter{}, 1, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.filter.Matches(tc.value); got != tc.want {
				t.Errorf("Matches(%d) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}

	t.Run("accessors", func(t *testing.T) {
		t.Parallel()
		if !model.AllProjectFilter().IsAll() || model.AllProjectFilter().IsNone() {
			t.Error("AllProjectFilter accessors are inconsistent")
		}
		if !model.NoneProjectFilter().IsNone() || model.NoneProjectFilter().IsAll() {
			t.Error("NoneProjectFilter accessors are inconsistent")
		}
		id, ok := model.NamedProjectFilter(7).ProjectID()
		if !ok || id != 7 {
			t.Errorf("ProjectID() = %d, %v; want 7, true", id, ok)
		}
		if _, ok := model.AllProjectFilter().ProjectID(); ok {
			t.Error("AllProjectFilter.ProjectID() reports an ID")
		}
	})
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestRegistry_VisibleNotes(t *testing.T) {
	t.Parallel()

	// Four notes, one for each row of the visibility table.
	type fixture struct {
		reg          *model.Registry
		alice, bob   *model.Account
		alpha        *model.Project
		projectNone  *model.Note // project Alpha, no account
		projectOwned *model.Note // project Alpha, alice
		plainOwned   *model.Note // no project, alice
		plainGlobal  *model.Note // no project, no account
	}
	newFixture := func(t *testing.T) fixture {
		t.Helper()
		reg := seedRegistry(t)
		alice := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		bob := reg.Account(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
		alpha := reg.Project("Alpha")
		projectNone, err := reg.AddNote("project", alpha, nil)
		must(t, err)
		projectOwned, err := reg.AddNote("owned project", alpha, alice)
		must(t, err)
		plainOwned, err := reg.AddNote("owned plain", nil, alice)
		must(t, err)
		plainGlobal, err := reg.AddNote("global plain", nil, nil)
		must(t, err)
		return fixture{reg, alice, bob, alpha, projectNone, projectOwned, plainOwned, plainGlobal}
	}

	t.Run("project set, account none", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		got, err := f.reg.VisibleNotes(f.alice, model.AllProjectFilter())
		must(t, err)
		if !containsNote(got, f.projectNone) {
			t.Error("granted account should see the project note")
		}
		got, err = f.reg.VisibleNotes(f.bob, model.AllProjectFilter())
		must(t, err)
		if containsNote(got, f.projectNone) {
			t.Error("account without the grant should not see the project note")
		}
	})

	t.Run("project set, no accounts granted", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		orphan := f.reg.Project("Orphan")
		note, err := f.reg.AddNote("orphan project", orphan, nil)
		must(t, err)

		for _, acc := range []*model.Account{f.alice, f.bob} {
			got, err := f.reg.VisibleNotes(acc, model.AllProjectFilter())
			must(t, err)
			if !containsNote(got, note) {
				t.Errorf("account %v should see the note for a project with no grants", acc.Key())
			}
		}
	})

	t.Run("project set, account set", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		got, err := f.reg.VisibleNotes(f.alice, model.AllProjectFilter())
		must(t, err)
		if !containsNote(got, f.projectOwned) {
			t.Error("the note's account should see it")
		}
		got, err = f.reg.VisibleNotes(f.bob, model.AllProjectFilter())
		must(t, err)
		if containsNote(got, f.projectOwned) {
			t.Error("another account should not see it")
		}
	})

	t.Run("project none, account set", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		got, err := f.reg.VisibleNotes(f.alice, model.AllProjectFilter())
		must(t, err)
		if !containsNote(got, f.plainOwned) {
			t.Error("the note's account should see it")
		}
		got, err = f.reg.VisibleNotes(f.bob, model.AllProjectFilter())
		must(t, err)
		if containsNote(got, f.plainOwned) {
			t.Error("another account should not see it")
		}
	})

	t.Run("project none, account none", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		for _, acc := range []*model.Account{f.alice, f.bob} {
			got, err := f.reg.VisibleNotes(acc, model.AllProjectFilter())
			must(t, err)
			if !containsNote(got, f.plainGlobal) {
				t.Errorf("account %v should see the global note", acc.Key())
			}
		}
	})

	t.Run("filters apply to the note project", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		got, err := f.reg.VisibleNotes(f.alice, model.NoneProjectFilter())
		must(t, err)
		if containsNote(got, f.projectNone) || containsNote(got, f.projectOwned) {
			t.Error("None filter should reject notes with a project")
		}
		if !containsNote(got, f.plainOwned) || !containsNote(got, f.plainGlobal) {
			t.Error("None filter should keep notes without a project")
		}

		got, err = f.reg.VisibleNotes(f.alice, model.NamedProjectFilter(f.alpha.ID()))
		must(t, err)
		if !containsNote(got, f.projectNone) || !containsNote(got, f.projectOwned) {
			t.Error("Alpha filter should keep Alpha notes")
		}
		if containsNote(got, f.plainOwned) || containsNote(got, f.plainGlobal) {
			t.Error("Alpha filter should reject notes without Alpha")
		}

		got, err = f.reg.VisibleNotes(f.alice, model.NamedProjectFilter(f.reg.Project("Beta").ID()))
		must(t, err)
		if len(got) != 0 {
			t.Errorf("Beta filter = %v, want empty", got)
		}
	})

	t.Run("ordered by ID ascending", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		got, err := f.reg.VisibleNotes(f.alice, model.AllProjectFilter())
		must(t, err)
		for i := 1; i < len(got); i++ {
			if got[i-1].ID() >= got[i].ID() {
				t.Errorf("notes are not ordered by ID: %d then %d", got[i-1].ID(), got[i].ID())
			}
		}
	})

	t.Run("nil account", func(t *testing.T) {
		t.Parallel()
		_, err := model.NewRegistry().VisibleNotes(nil, model.AllProjectFilter())
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewAccount("ghost@example.com", "claude")
		_, err := model.NewRegistry().VisibleNotes(ghost, model.AllProjectFilter())
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, reg.PauseAccount(acc, time.Now().Add(time.Hour)))
		_, err := reg.VisibleNotes(acc, model.AllProjectFilter())
		if !errors.Is(err, model.ErrAccountStale) {
			t.Errorf("got %v, want ErrAccountStale", err)
		}
	})
}

func TestRegistry_ProjectNotes(t *testing.T) {
	t.Parallel()

	t.Run("returns every note for the project regardless of account", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		alice := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		n1, err := reg.AddNote("project only", reg.Project("Alpha"), nil)
		must(t, err)
		n2, err := reg.AddNote("project and account", reg.Project("Alpha"), alice)
		must(t, err)
		_, err = reg.AddNote("other project", reg.Project("Beta"), alice)
		must(t, err)
		_, err = reg.AddNote("no project", nil, nil)
		must(t, err)

		got, err := reg.ProjectNotes(reg.Project("Alpha"))
		must(t, err)
		if len(got) != 2 || got[0].ID() != n1.ID() || got[1].ID() != n2.ID() {
			t.Errorf("ProjectNotes(Alpha) = %v, want IDs %d, %d", got, n1.ID(), n2.ID())
		}
	})

	t.Run("nil project", func(t *testing.T) {
		t.Parallel()
		_, err := model.NewRegistry().ProjectNotes(nil)
		if !errors.Is(err, model.ErrProjectNil) {
			t.Errorf("got %v, want ErrProjectNil", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		ghost := model.NewProject("Ghost", "", "")
		_, err := model.NewRegistry().ProjectNotes(ghost)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		stale := reg.Project("Alpha")
		must(t, reg.UpdateProject(stale, model.NewProject("Alpha", "new", "")))
		_, err := reg.ProjectNotes(stale)
		if !errors.Is(err, model.ErrProjectStale) {
			t.Errorf("got %v, want ErrProjectStale", err)
		}
	})
}

func containsNote(notes []*model.Note, want *model.Note) bool {
	return slices.Contains(notes, want)
}

func TestRegistry_NoteRefusals(t *testing.T) {
	t.Parallel()

	t.Run("delete account refuses while a note has the account", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		_, err = reg.AddNote("text", nil, acc)
		must(t, err)

		err = reg.DeleteAccount(acc)
		if err == nil || !strings.Contains(err.Error(), "has 1 notes") {
			t.Errorf("got %v, want the note refusal", err)
		}
		if reg.Account(reg.User("alice@example.com"), reg.Vendor("claude")) == nil {
			t.Error("account was deleted despite the note")
		}
	})

	t.Run("delete project refuses while a note has the project", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		_, err := reg.AddNote("text", reg.Project("Alpha"), nil)
		must(t, err)

		err = reg.DeleteProject(reg.Project("Alpha"))
		if err == nil || !strings.Contains(err.Error(), "has 1 notes") {
			t.Errorf("got %v, want the note refusal", err)
		}
		if reg.Project("Alpha") == nil {
			t.Error("project was deleted despite the note")
		}
	})

	t.Run("remove project grant refuses while a note has both", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
		_, err = reg.AddNote("text", reg.Project("Alpha"), acc)
		must(t, err)

		err = reg.RemoveProjectFromAccount(acc, reg.Project("Alpha"))
		if err == nil || !strings.Contains(err.Error(), "has 1 notes for project") {
			t.Errorf("got %v, want the note refusal", err)
		}
		if projects, err := reg.AccountProjects(acc); err != nil || len(projects) != 1 {
			t.Errorf("grant was removed despite the note: projects=%v err=%v", projects, err)
		}
	})

	t.Run("notes for other accounts or projects do not block", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		must(t, reg.AddProjectToAccount(acc, reg.Project("Alpha")))
		_, err = reg.AddNote("text", reg.Project("Alpha"), nil)
		must(t, err)

		// The note has the project but no account, so revoking the grant and
		// deleting the account are both allowed.
		must(t, reg.RemoveProjectFromAccount(acc, reg.Project("Alpha")))
		must(t, reg.DeleteAccount(acc))
	})
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestRegistry_RestoreSession(t *testing.T) {
	t.Parallel()

	t.Run("restores every field and raises the next ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		at := time.Date(2026, 10, 30, 9, 15, 0, 0, time.UTC)
		queue := []model.Prompt{model.NewPrompt("one"), model.NewPrompt("two")}

		sess, err := reg.RestoreSession(acc, 10, reg.Project("Alpha"), "Name", " https://example.com ", model.SessionStatusNeedsClosing, at, queue)
		must(t, err)
		if sess.ID() != 10 {
			t.Errorf("ID() = %d, want 10", sess.ID())
		}
		if sess.Name() != "Name" || sess.URL() != "https://example.com" || sess.Status() != model.SessionStatusNeedsClosing {
			t.Errorf("fields = name %q url %q status %v", sess.Name(), sess.URL(), sess.Status())
		}
		if sess.AccountID() != acc.ID() {
			t.Errorf("AccountID() = %d, want %d", sess.AccountID(), acc.ID())
		}
		if !sess.UpdatedAt().Equal(at) {
			t.Errorf("UpdatedAt() = %v, want %v", sess.UpdatedAt(), at)
		}
		if got := sess.Queue(); len(got) != 2 || got[0].Text() != "one" || got[1].Text() != "two" {
			t.Errorf("Queue() = %v, want two prompts", got)
		}
		next, err := reg.AddSession(acc, nil, "", nil)
		must(t, err)
		if next.ID() != 11 {
			t.Errorf("next session ID = %d, want 11", next.ID())
		}
		if got := reg.Session(10); got != sess {
			t.Error("restored session is not retrievable by ID")
		}
	})

	t.Run("zero ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.RestoreSession(acc, 0, nil, "", "", model.SessionStatusActive, time.Time{}, nil)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("duplicate ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.RestoreSession(acc, 1, nil, "", "", model.SessionStatusActive, time.Time{}, nil)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("invalid prompt", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.RestoreSession(acc, 10, nil, "", "", model.SessionStatusActive, time.Time{}, []model.Prompt{model.NewPrompt("")})
		if !errors.Is(err, model.ErrTextRequired) {
			t.Errorf("got %v, want ErrTextRequired", err)
		}
	})

	t.Run("project not assigned", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.RestoreSession(acc, 10, reg.Project("Orphan"), "", "", model.SessionStatusActive, time.Time{}, nil)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewAccount("ghost@example.com", "claude")
		_, err := reg.RestoreSession(ghost, 10, nil, "", "", model.SessionStatusActive, time.Time{}, nil)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("zero updated-at stays zero", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		sess, err := reg.RestoreSession(acc, 10, nil, "", "", model.SessionStatusActive, time.Time{}, nil)
		must(t, err)
		if !sess.UpdatedAt().IsZero() {
			t.Errorf("UpdatedAt() = %v, want zero", sess.UpdatedAt())
		}
	})
}

func TestRegistry_RestoreNote(t *testing.T) {
	t.Parallel()

	t.Run("restores every field and raises the next ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		n, err := reg.RestoreNote(10, "  text  ", reg.Project("Alpha"), acc)
		must(t, err)
		if n.ID() != 10 {
			t.Errorf("ID() = %d, want 10", n.ID())
		}
		if n.Text() != "text" || n.ProjectName() != "Alpha" {
			t.Errorf("fields = text %q project %q", n.Text(), n.ProjectName())
		}
		if n.ProjectID() != reg.Project("Alpha").ID() {
			t.Errorf("ProjectID() = %d, want %d", n.ProjectID(), reg.Project("Alpha").ID())
		}
		if key, ok := n.AccountKey(); !ok || key != acc.Key() {
			t.Errorf("AccountKey() = %v, %v; want %v, true", key, ok, acc.Key())
		}
		next, err := reg.AddNote("next", nil, nil)
		must(t, err)
		if next.ID() != 11 {
			t.Errorf("next note ID = %d, want 11", next.ID())
		}
		if got := reg.Note(10); got != n {
			t.Error("restored note is not retrievable by ID")
		}
	})

	t.Run("zero ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		_, err := reg.RestoreNote(0, "text", nil, nil)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("duplicate ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		_, err := reg.RestoreNote(1, "text", nil, nil)
		must(t, err)
		_, err = reg.RestoreNote(1, "other", nil, nil)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("empty text", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		_, err := reg.RestoreNote(10, "  ", nil, nil)
		if !errors.Is(err, model.ErrTextRequired) {
			t.Errorf("got %v, want ErrTextRequired", err)
		}
	})

	t.Run("project not assigned", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.RestoreNote(10, "text", reg.Project("Orphan"), acc)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewAccount("ghost@example.com", "claude")
		_, err := reg.RestoreNote(10, "text", nil, ghost)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestRegistry_StartSessionAtomic(t *testing.T) {
	t.Parallel()

	t.Run("blank start creates an empty session", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		beforeID := uint64(0)
		for _, session := range reg.Sessions() {
			beforeID = max(beforeID, session.ID())
		}

		sess, err := reg.StartSessionAtomic(acc, nil, "name", "", "", nil)
		must(t, err)
		if sess.ID() != beforeID+1 {
			t.Errorf("ID() = %d, want %d", sess.ID(), beforeID+1)
		}
		if sess.AccountID() != acc.ID() {
			t.Errorf("AccountID() = %d, want %d", sess.AccountID(), acc.ID())
		}
		if sess.ProjectName() != "" || sess.Name() != "name" || sess.URL() != "" {
			t.Errorf("session = project %q name %q url %q", sess.ProjectName(), sess.Name(), sess.URL())
		}
		if sess.Status() != model.SessionStatusActive {
			t.Errorf("Status() = %v, want Active", sess.Status())
		}
		if len(sess.Queue()) != 0 {
			t.Errorf("Queue() = %v, want empty", sess.Queue())
		}
		if !sess.UpdatedAt().IsZero() {
			t.Errorf("UpdatedAt() = %v, want zero", sess.UpdatedAt())
		}
	})

	t.Run("existing project, URL, and prompt", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		at := time.Date(2026, 10, 30, 9, 15, 0, 0, time.UTC)
		reg.SetClock(func() time.Time { return at })

		sess, err := reg.StartSessionAtomic(acc, reg.Project("Alpha"), "name", "  https://example.com/chat  ", "  hello  ", nil)
		must(t, err)
		if sess.ProjectName() != "Alpha" {
			t.Errorf("ProjectName() = %q, want Alpha", sess.ProjectName())
		}
		if sess.ProjectID() != reg.Project("Alpha").ID() {
			t.Errorf("ProjectID() = %d, want %d", sess.ProjectID(), reg.Project("Alpha").ID())
		}
		if sess.URL() != "https://example.com/chat" {
			t.Errorf("URL() = %q, want trimmed URL", sess.URL())
		}
		if got := sess.Queue(); len(got) != 1 || got[0].Text() != "hello" {
			t.Errorf("Queue() = %v, want one trimmed prompt", got)
		}
		if !sess.UpdatedAt().Equal(at) {
			t.Errorf("UpdatedAt() = %v, want %v", sess.UpdatedAt(), at)
		}
	})

	t.Run("note is deleted after a successful start", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		note, err := reg.AddNote("note text", nil, acc)
		must(t, err)

		sess, err := reg.StartSessionAtomic(acc, nil, "name", "", "note text", note)
		must(t, err)
		if reg.Note(note.ID()) != nil {
			t.Error("note still exists after a successful start")
		}
		if len(sess.Queue()) != 1 || sess.Queue()[0].Text() != "note text" {
			t.Errorf("Queue() = %v, want the note text", sess.Queue())
		}
	})

	t.Run("note with project requires the same existing project", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		note, err := reg.AddNote("note text", reg.Project("Alpha"), nil)
		must(t, err)

		sess, err := reg.StartSessionAtomic(acc, reg.Project("Alpha"), "", "", "note text", note)
		must(t, err)
		if sess.ProjectName() != "Alpha" {
			t.Errorf("ProjectName() = %q, want Alpha", sess.ProjectName())
		}
	})

	t.Run("account nil", func(t *testing.T) {
		t.Parallel()
		_, err := model.NewRegistry().StartSessionAtomic(nil, nil, "", "", "", nil)
		if !errors.Is(err, model.ErrAccountNil) {
			t.Errorf("got %v, want ErrAccountNil", err)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewAccount("ghost@example.com", "claude")
		_, err := reg.StartSessionAtomic(ghost, nil, "", "", "", nil)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("account stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, reg.PauseAccount(acc, time.Now().Add(time.Hour)))
		_, err := reg.StartSessionAtomic(acc, nil, "", "", "", nil)
		if !errors.Is(err, model.ErrAccountStale) {
			t.Errorf("got %v, want ErrAccountStale", err)
		}
	})

	t.Run("existing project not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.StartSessionAtomic(acc, model.NewProject("Ghost", "", ""), "", "", "", nil)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("existing project stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		stale := reg.Project("Alpha")
		must(t, reg.UpdateProject(stale, model.NewProject("Alpha", "new", "")))
		_, err := reg.StartSessionAtomic(acc, stale, "", "", "", nil)
		if !errors.Is(err, model.ErrProjectStale) {
			t.Errorf("got %v, want ErrProjectStale", err)
		}
	})

	t.Run("existing project not granted", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		_, err := reg.StartSessionAtomic(acc, reg.Project("Orphan"), "", "", "", nil)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("note not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		ghost := model.NewNote(99, "text", 0, "", model.AccountKey{}, false)
		_, err := reg.StartSessionAtomic(acc, nil, "", "", "text", ghost)
		if !errors.Is(err, model.ErrNoteNotFound) {
			t.Errorf("got %v, want ErrNoteNotFound", err)
		}
	})

	t.Run("note stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		note, err := reg.AddNote("text", nil, acc)
		must(t, err)
		must(t, reg.UpdateNote(note, "new", nil, acc))
		_, err = reg.StartSessionAtomic(acc, nil, "", "", "new", note)
		if !errors.Is(err, model.ErrNoteStale) {
			t.Errorf("got %v, want ErrNoteStale", err)
		}
	})

	t.Run("note not visible for the account", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		bob := reg.Account(reg.User("bob@example.com"), reg.Vendor("chatgpt"))
		note, err := reg.AddNote("text", reg.Project("Alpha"), nil)
		must(t, err)
		_, err = reg.StartSessionAtomic(bob, nil, "", "", "text", note)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("note project must match the existing project", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		note, err := reg.AddNote("text", reg.Project("Alpha"), nil)
		must(t, err)
		_, err = reg.StartSessionAtomic(acc, reg.Project("Beta"), "", "", "text", note)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("note requires prompt text", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		note, err := reg.AddNote("text", nil, acc)
		must(t, err)
		_, err = reg.StartSessionAtomic(acc, nil, "", "", "   ", note)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("failure leaves the registry unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		note, err := reg.AddNote("text", reg.Project("Alpha"), nil)
		must(t, err)

		sessionsBefore := len(reg.Sessions())
		notesBefore := len(reg.Notes())

		_, err = reg.StartSessionAtomic(acc, reg.Project("Beta"), "", "", "text", note)
		if err == nil {
			t.Fatal("got nil, want error")
		}
		if got := len(reg.Sessions()); got != sessionsBefore {
			t.Errorf("sessions = %d, want %d", got, sessionsBefore)
		}
		if got := len(reg.Notes()); got != notesBefore {
			t.Errorf("notes = %d, want %d", got, notesBefore)
		}
		if reg.Note(note.ID()) != note {
			t.Error("note was changed by a failed start")
		}
	})
}

func TestRegistry_DeleteDoesNotReuseIDs(t *testing.T) {
	t.Parallel()

	t.Run("sessions", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, reg.DeleteSession(reg.Session(2)))
		next, err := reg.AddSession(acc, nil, "", nil)
		must(t, err)
		if next.ID() != 3 {
			t.Errorf("next session ID = %d, want 3", next.ID())
		}
	})

	t.Run("notes", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		first, err := reg.AddNote("one", nil, nil)
		must(t, err)
		second, err := reg.AddNote("two", nil, nil)
		must(t, err)
		must(t, reg.DeleteNote(second))
		next, err := reg.AddNote("three", nil, nil)
		must(t, err)
		if first.ID() != 1 || next.ID() != 3 {
			t.Errorf("note IDs = %d, %d; want 1, 3", first.ID(), next.ID())
		}
	})
}

func TestRegistry_AccountIDs(t *testing.T) {
	t.Parallel()

	t.Run("AddAccount assigns increasing IDs", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
		first, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		second, err := reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("claude"))
		must(t, err)
		if first.ID() != 1 || second.ID() != 2 {
			t.Errorf("account IDs = %d, %d; want 1, 2", first.ID(), second.ID())
		}
	})

	t.Run("duplicate AddAccount does not consume an ID", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
		_, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		_, err = reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		if !errors.Is(err, model.ErrAccountExists) {
			t.Fatalf("duplicate AddAccount error = %v, want ErrAccountExists", err)
		}
		next, err := reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("claude"))
		must(t, err)
		if next.ID() != 2 {
			t.Errorf("next account ID = %d, want 2", next.ID())
		}
	})

	t.Run("AccountByID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		if got := reg.AccountByID(acc.ID()); got != acc {
			t.Errorf("AccountByID(%d) = %v, want %v", acc.ID(), got, acc)
		}
		if got := reg.AccountByID(99); got != nil {
			t.Errorf("AccountByID(99) = %v, want nil", got)
		}
	})

	t.Run("DeleteAccount does not reuse an ID", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		must(t, reg.DeleteAccount(acc))
		if got := reg.AccountByID(acc.ID()); got != nil {
			t.Errorf("AccountByID(%d) after delete = %v, want nil", acc.ID(), got)
		}
		next, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
		must(t, err)
		if next.ID() != 2 {
			t.Errorf("next account ID = %d, want 2", next.ID())
		}
	})

	t.Run("PauseAccount keeps the ID and updates the ID lookup", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		until := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
		must(t, reg.PauseAccount(acc, until))
		live := reg.AccountByID(acc.ID())
		if live == nil || live.ID() != acc.ID() {
			t.Fatalf("AccountByID(%d) = %v, want the paused account", acc.ID(), live)
		}
		if !live.PauseUntil().Equal(until) {
			t.Errorf("PauseUntil() = %v, want %v", live.PauseUntil(), until)
		}
	})
}

func TestRegistry_RestoreAccount(t *testing.T) {
	t.Parallel()

	t.Run("restores every field and raises the next ID", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		acc, err := reg.RestoreAccount(reg.User("alice@example.com"), reg.Vendor("claude"), 10)
		must(t, err)
		if acc.ID() != 10 || acc.UserEmail() != "alice@example.com" || acc.VendorName() != "claude" {
			t.Errorf("account = id %d user %q vendor %q", acc.ID(), acc.UserEmail(), acc.VendorName())
		}
		if got := reg.AccountByID(10); got != acc {
			t.Errorf("AccountByID(10) = %v, want %v", got, acc)
		}
	})

	t.Run("next ID after a restored high ID", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
		_, err := reg.RestoreAccount(reg.User("alice@example.com"), reg.Vendor("claude"), 10)
		must(t, err)
		next, err := reg.AddAccount(reg.User("bob@example.com"), reg.Vendor("claude"))
		must(t, err)
		if next.ID() != 11 {
			t.Errorf("next account ID = %d, want 11", next.ID())
		}
	})

	t.Run("zero ID", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		_, err := reg.RestoreAccount(reg.User("alice@example.com"), reg.Vendor("claude"), 0)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("duplicate ID", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		must(t, reg.AddUser(model.NewUser("bob@example.com", "Bob")))
		_, err := reg.RestoreAccount(reg.User("alice@example.com"), reg.Vendor("claude"), 10)
		must(t, err)
		_, err = reg.RestoreAccount(reg.User("bob@example.com"), reg.Vendor("claude"), 10)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("duplicate natural key", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
		must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
		_, err := reg.RestoreAccount(reg.User("alice@example.com"), reg.Vendor("claude"), 10)
		must(t, err)
		_, err = reg.RestoreAccount(reg.User("alice@example.com"), reg.Vendor("claude"), 11)
		if !errors.Is(err, model.ErrAccountExists) {
			t.Errorf("got %v, want ErrAccountExists", err)
		}
	})
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestRegistry_ProjectIDs(t *testing.T) {
	t.Parallel()

	t.Run("AddProject assigns increasing IDs", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		first := model.NewProject("Alpha", "", "")
		second := model.NewProject("Beta", "", "")
		must(t, reg.AddProject(first))
		must(t, reg.AddProject(second))
		if first.ID() != 1 || second.ID() != 2 {
			t.Errorf("project IDs = %d, %d; want 1, 2", first.ID(), second.ID())
		}
	})

	t.Run("duplicate AddProject does not consume an ID", func(t *testing.T) {
		t.Parallel()
		reg := model.NewRegistry()
		must(t, reg.AddProject(model.NewProject("Alpha", "", "")))
		err := reg.AddProject(model.NewProject("Alpha", "", ""))
		if !errors.Is(err, model.ErrProjectExists) {
			t.Fatalf("duplicate AddProject error = %v, want ErrProjectExists", err)
		}
		next := model.NewProject("Beta", "", "")
		must(t, reg.AddProject(next))
		if next.ID() != 2 {
			t.Errorf("next project ID = %d, want 2", next.ID())
		}
	})

	t.Run("ProjectByID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		project := reg.Project("Alpha")
		if got := reg.ProjectByID(project.ID()); got != project {
			t.Errorf("ProjectByID(%d) = %v, want %v", project.ID(), got, project)
		}
		if got := reg.ProjectByID(999); got != nil {
			t.Errorf("ProjectByID(999) = %v, want nil", got)
		}
	})

	t.Run("DeleteProject does not reuse an ID", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		project := reg.Project("Orphan")
		must(t, reg.DeleteProject(project))
		next := model.NewProject("Gamma", "", "")
		must(t, reg.AddProject(next))
		if next.ID() <= project.ID() {
			t.Errorf("next project ID = %d, want greater than %d", next.ID(), project.ID())
		}
	})

	t.Run("RestoreProject", func(t *testing.T) {
		t.Parallel()

		t.Run("restores every field and raises the next ID", func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			project, err := reg.RestoreProject("Alpha", "desc", "inst", 10)
			must(t, err)
			if project.ID() != 10 || project.Name() != "Alpha" || project.Description() != "desc" || project.Instructions() != "inst" {
				t.Errorf("project = %+v", project)
			}
			if got := reg.ProjectByID(10); got != project {
				t.Errorf("ProjectByID(10) = %v, want %v", got, project)
			}
			next := model.NewProject("Beta", "", "")
			must(t, reg.AddProject(next))
			if next.ID() != 11 {
				t.Errorf("next project ID = %d, want 11", next.ID())
			}
		})

		t.Run("zero ID", func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			_, err := reg.RestoreProject("Alpha", "", "", 0)
			if err == nil {
				t.Fatal("got nil, want error")
			}
		})

		t.Run("duplicate ID", func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			_, err := reg.RestoreProject("Alpha", "", "", 10)
			must(t, err)
			_, err = reg.RestoreProject("Beta", "", "", 10)
			if err == nil {
				t.Fatal("got nil, want error")
			}
		})

		t.Run("duplicate name", func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			_, err := reg.RestoreProject("Alpha", "", "", 10)
			must(t, err)
			_, err = reg.RestoreProject("Alpha", "", "", 11)
			if !errors.Is(err, model.ErrProjectExists) {
				t.Errorf("got %v, want ErrProjectExists", err)
			}
		})
	})
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestRegistry_RenameProject(t *testing.T) {
	t.Parallel()
	reg := seedRegistry(t)
	acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
	project := reg.Project("Alpha")
	projectID := project.ID()
	session := reg.Session(1)
	note, err := reg.AddNote("note", project, acc)
	must(t, err)

	renamed, err := reg.RenameProject(project, "Renamed")
	must(t, err)
	if renamed.ID() != projectID || renamed.Name() != "Renamed" {
		t.Errorf("renamed project = id %d name %q, want id %d name Renamed", renamed.ID(), renamed.Name(), projectID)
	}
	if reg.Project("Alpha") != nil {
		t.Error("old project name still resolves")
	}
	if got := reg.ProjectByID(projectID); got != renamed {
		t.Errorf("ProjectByID(%d) = %v, want %v", projectID, got, renamed)
	}
	if got := reg.Session(session.ID()); got == nil || got.ProjectName() != "Renamed" || got.ProjectID() != projectID {
		t.Errorf("session after rename = %v", got)
	}
	if got := reg.Note(note.ID()); got == nil || got.ProjectName() != "Renamed" || got.ProjectID() != projectID {
		t.Errorf("note after rename = %v", got)
	}
	projects, err := reg.AccountProjects(acc)
	must(t, err)
	found := false
	for _, project := range projects {
		if project.ID() == projectID {
			found = true
			if project.Name() != "Renamed" {
				t.Errorf("renamed project name = %q, want Renamed", project.Name())
			}
		}
	}
	if !found {
		t.Errorf("renamed project not in account projects: %v", projects)
	}
	sessions, err := reg.ProjectSessions(renamed)
	must(t, err)
	if len(sessions) != 1 || sessions[0].ID() != session.ID() {
		t.Errorf("project sessions after rename = %v", sessions)
	}
	notes, err := reg.ProjectNotes(renamed)
	must(t, err)
	if len(notes) != 1 || notes[0].ID() != note.ID() {
		t.Errorf("project notes after rename = %v", notes)
	}

	if _, renameErr := reg.RenameProject(project, "Other"); !errors.Is(renameErr, model.ErrProjectStale) {
		t.Errorf("rename with stale pointer = %v, want ErrProjectStale", renameErr)
	}
	if _, renameErr := reg.RenameProject(renamed, "Beta"); !errors.Is(renameErr, model.ErrProjectExists) {
		t.Errorf("rename to existing name = %v, want ErrProjectExists", renameErr)
	}
	if _, renameErr := reg.RenameProject(renamed, "  "); !errors.Is(renameErr, model.ErrNameRequired) {
		t.Errorf("rename to empty name = %v, want ErrNameRequired", renameErr)
	}
	same, err := reg.RenameProject(renamed, "Renamed")
	must(t, err)
	if same != renamed {
		t.Error("no-op rename returned a different pointer")
	}
}
