package model_test

import (
	"errors"
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
		sess, err := reg.AddSession(acc, nil, "Topic", nil)
		must(t, err)
		if sess.ProjectName() != "" {
			t.Errorf("got project %q, want empty", sess.ProjectName())
		}
		if sess.ID() == 0 {
			t.Error("expected a non-zero ID")
		}
	})
	t.Run("success with project", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		acc := reg.Account(reg.User("alice@example.com"), reg.Vendor("claude"))
		queue := []model.Prompt{model.NewPrompt("continue", false)}
		sess, err := reg.AddSession(acc, reg.Project("Beta"), "Topic", queue)
		must(t, err)
		if sess.ProjectName() != "Beta" {
			t.Errorf("got project %q, want Beta", sess.ProjectName())
		}
		if len(sess.Queue()) != 1 {
			t.Errorf("got %d queue items, want 1", len(sess.Queue()))
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
			model.NewPrompt("ok", false),
			model.NewPrompt("", false),
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
		must(t, reg.UpdateSessionTopic(stale, "changed"))
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

func TestRegistry_UpdateSessionTopic(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		sess := reg.Session(1)
		must(t, reg.UpdateSessionTopic(sess, "New Topic"))
		got := reg.Session(1)
		if got.Topic() != "New Topic" {
			t.Errorf("got %q, want %q", got.Topic(), "New Topic")
		}
		if got == sess {
			t.Error("expected a new pointer after a real change")
		}
	})
	t.Run("no-op when topic is unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		before := reg.Session(1)
		must(t, reg.UpdateSessionTopic(before, before.Topic()))
		after := reg.Session(1)
		if before != after {
			t.Error("expected the same pointer when the topic does not change (no-op CoW)")
		}
	})
	t.Run("trims and normalizes before comparing", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		before := reg.Session(1) // topic is "Architecture review"
		must(t, reg.UpdateSessionTopic(before, "  Architecture review  "))
		after := reg.Session(1)
		if before != after {
			t.Error("expected the same pointer: normalized topic is unchanged")
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		err := model.NewRegistry().UpdateSessionTopic(nil, "x")
		if !errors.Is(err, model.ErrSessionNil) {
			t.Errorf("got %v, want ErrSessionNil", err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		ghost := model.NewSession(model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, 999, "", "", nil)
		err := reg.UpdateSessionTopic(ghost, "x")
		if !errors.Is(err, model.ErrSessionNotFound) {
			t.Errorf("got %v, want ErrSessionNotFound", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		stale := reg.Session(1)
		must(t, reg.UpdateSessionTopic(stale, "first change"))
		err := reg.UpdateSessionTopic(stale, "second change")
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
		newQueue := []model.Prompt{model.NewPrompt("step one", false), model.NewPrompt("step two", true)}
		must(t, reg.UpdateSessionPrompts(sess, newQueue))
		got := reg.Session(1).Queue()
		if len(got) != 2 || got[0].Text() != "step one" || got[1].Text() != "step two" {
			t.Errorf("got %+v, want two prompts in order", got)
		}
	})
	t.Run("no-op when queue is unchanged", func(t *testing.T) {
		t.Parallel()
		reg := seedRegistry(t)
		queue := []model.Prompt{model.NewPrompt("same", false)}
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
		err := reg.UpdateSessionPrompts(sess, []model.Prompt{model.NewPrompt("", false)})
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
		must(t, reg.UpdateSessionPrompts(stale, []model.Prompt{model.NewPrompt("first update", false)}))
		err := reg.UpdateSessionPrompts(stale, []model.Prompt{model.NewPrompt("second update", false)})
		if !errors.Is(err, model.ErrSessionStale) {
			t.Errorf("got %v, want ErrSessionStale", err)
		}
	})
}
