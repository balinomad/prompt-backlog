package service_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/apperr"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"
	"github.com/balinomad/llm-desk/internal/store"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// must fails the test immediately when err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// errGeneric is a sentinel placeholder for table-driven cases whose
// underlying error carries no specific model/apperr sentinel — only its
// presence can be asserted, not its identity. Used as a table value, never
// returned by production code.
var errGeneric = errors.New("table test: generic error expected")

// mockStore is a minimal store.Store implementation used to exercise
// Service.New and Service.Save without depending on a real persistence
// backend.
type mockStore struct {
	reg     *model.Registry
	loadErr error
	saveErr error
}

var _ store.Store = (*mockStore)(nil)

func (m *mockStore) Load() (*model.Registry, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.reg, nil
}

func (m *mockStore) Save(_ *model.Registry) error {
	return m.saveErr
}

// newService returns a Service backed by a fresh, empty Registry via a
// mockStore that never fails.
func newService(t *testing.T) *service.Service {
	t.Helper()
	svc, err := service.New(&mockStore{reg: model.NewRegistry()})
	must(t, err)
	return svc
}

// seedService returns a Service whose registry is pre-populated:
//
//	Vendors: "claude" (HasProjects=true), "chatgpt" (HasProjects=false)
//	Users:     alice@example.com / Alice, bob@example.com / Bob
//	Projects:  "Alpha", "Beta", "Orphan"
//	Accounts:  alice@example.com|claude (granted Alpha + Beta)
//	           bob@example.com|chatgpt  (no projects granted)
//	Sessions:
//	  ID 1: alice@example.com|claude, project=Alpha, name="Architecture review"
//	  ID 2: bob@example.com|chatgpt, no project
//
// The fixture supports every resolveAccount/resolveSession edge case without
// per-case setup.
func seedService(t *testing.T) *service.Service {
	t.Helper()
	svc := newService(t)
	must(t, svc.AddVendor("claude", true, false))
	must(t, svc.AddVendor("chatgpt", false, false))
	must(t, svc.AddUser("alice@example.com", "Alice"))
	must(t, svc.AddUser("bob@example.com", "Bob"))
	must(t, svc.AddProject("Alpha", "", ""))
	must(t, svc.AddProject("Beta", "", ""))
	must(t, svc.AddProject("Orphan", "", ""))
	must(t, svc.AddAccount("alice@example.com", "claude"))
	must(t, svc.AddAccount("bob@example.com", "chatgpt"))
	must(t, svc.GrantProject("alice@example.com", "claude", "Alpha"))
	must(t, svc.GrantProject("alice@example.com", "claude", "Beta"))
	_, err := svc.StartSession("alice@example.com", "claude", "Alpha", "Architecture review", nil)
	must(t, err)
	_, err = svc.StartSession("bob@example.com", "chatgpt", "", "", nil)
	must(t, err)
	return svc
}

// ── Service ───────────────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Parallel()
	loadErr := errors.New("disk error")

	cases := []struct {
		name          string
		store         store.Store
		wantErrIs     error  // checked via errors.Is when non-nil
		wantErrSubstr string // checked via strings.Contains when non-empty
	}{
		{"nil store", nil, apperr.ErrNil, ""},
		{"load error", &mockStore{loadErr: loadErr}, loadErr, ""},
		{"store returns nil registry", &mockStore{reg: nil}, nil, "nil registry"},
		{"success", &mockStore{reg: model.NewRegistry()}, nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc, err := service.New(tc.store)
			wantErr := tc.wantErrIs != nil || tc.wantErrSubstr != ""
			if !wantErr {
				must(t, err)
				if svc == nil {
					t.Fatal("got nil service")
				}
				return
			}
			if err == nil {
				t.Fatal("got nil, want error")
			}
			if svc != nil {
				t.Errorf("got %v, want nil service", svc)
			}
			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErrIs)
			}
			if tc.wantErrSubstr != "" && !strings.Contains(err.Error(), tc.wantErrSubstr) {
				t.Errorf("got %q, want substring %q", err.Error(), tc.wantErrSubstr)
			}
		})
	}
}

func TestService_Save(t *testing.T) {
	t.Parallel()
	saveErr := errors.New("disk full")

	cases := []struct {
		name    string
		saveErr error
	}{
		{"success", nil},
		{"store error", saveErr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc, err := service.New(&mockStore{reg: model.NewRegistry(), saveErr: tc.saveErr})
			must(t, err)
			err = svc.Save()
			if tc.saveErr == nil {
				must(t, err)
				return
			}
			if !errors.Is(err, tc.saveErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.saveErr)
			}
		})
	}
}

// ── Vendor ──────────────────────────────────────────────────────────────────

func TestService_Vendor(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	cases := []struct {
		name    string
		query   string
		wantNil bool
	}{
		{"found", "claude", false},
		{"not found", "ghost", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := svc.Vendor(tc.query)
			if tc.wantNil && p != nil {
				t.Errorf("got %v, want nil", p)
			}
			if !tc.wantNil && p == nil {
				t.Error("got nil, want non-nil vendor")
			}
		})
	}
}

func TestService_Vendors(t *testing.T) {
	t.Parallel()
	empty := newService(t)
	seeded := seedService(t)

	cases := []struct {
		name      string
		svc       *service.Service
		wantNames []string
	}{
		{"empty", empty, nil},
		{"sorted by name", seeded, []string{"chatgpt", "claude"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.svc.Vendors()
			if len(got) != len(tc.wantNames) {
				t.Fatalf("got %d vendors, want %d", len(got), len(tc.wantNames))
			}
			for i, want := range tc.wantNames {
				if got[i].Name() != want {
					t.Errorf("got[%d] = %q, want %q", i, got[i].Name(), want)
				}
			}
		})
	}
}

func TestService_ValidateVendor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		vendorName      string
		hasProjects     bool
		hasDeepResearch bool
		wantErr         error
	}{
		{"valid", "claude", true, true, nil},
		{"empty name", "", false, false, model.ErrNameRequired},
		{"whitespace-only name", "   ", false, false, model.ErrNameRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := newService(t).ValidateVendor(tc.vendorName, tc.hasProjects, tc.hasDeepResearch)
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

func TestService_AddVendor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		vendorName string
		wantErr    error
	}{
		{"success", "gemini", nil}, // not in the seed
		{"empty name", "", model.ErrNameRequired},
		{"duplicate", "claude", model.ErrVendorExists},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := seedService(t).AddVendor(tc.vendorName, true, false)
			if tc.wantErr == nil {
				must(t, err)
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestService_UpdateVendor(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddVendor("claude", false, false))
		must(t, svc.UpdateVendor("claude", true, true))
		got := svc.Vendor("claude")
		if !got.HasProjects() || !got.HasDeepResearch() {
			t.Errorf("got HasProjects=%v HasDeepResearch=%v, want both true", got.HasProjects(), got.HasDeepResearch())
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := newService(t).UpdateVendor("ghost", true, false)
		if !errors.Is(err, model.ErrVendorNotFound) {
			t.Errorf("got %v, want ErrVendorNotFound", err)
		}
	})
}

func TestService_DeleteVendor(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddVendor("claude", true, false))
		must(t, svc.DeleteVendor("claude"))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := newService(t).DeleteVendor("ghost")
		if !errors.Is(err, model.ErrVendorNotFound) {
			t.Errorf("got %v, want ErrVendorNotFound", err)
		}
	})

	t.Run("has associated accounts", func(t *testing.T) {
		t.Parallel()
		err := seedService(t).DeleteVendor("claude")
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

// ── User ──────────────────────────────────────────────────────────────────────

func TestService_User(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	cases := []struct {
		name    string
		query   string
		wantNil bool
	}{
		{"found", "alice@example.com", false},
		{"not found", "ghost@example.com", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := svc.User(tc.query)
			if tc.wantNil && u != nil {
				t.Errorf("got %v, want nil", u)
			}
			if !tc.wantNil && u == nil {
				t.Error("got nil, want non-nil user")
			}
		})
	}
}

func TestService_Users(t *testing.T) {
	t.Parallel()
	empty := newService(t)
	seeded := seedService(t)
	nameSorted := newService(t)
	must(t, nameSorted.AddUser("zoe@example.com", "Alice"))
	must(t, nameSorted.AddUser("alice@example.com", "Zoe"))
	must(t, nameSorted.AddUser("bob@example.com", "alice"))

	cases := []struct {
		name       string
		svc        *service.Service
		wantEmails []string
	}{
		{"empty", empty, nil},
		{"sorted by name", seeded, []string{"alice@example.com", "bob@example.com"}},
		{"name order with email tie-break", nameSorted, []string{"bob@example.com", "zoe@example.com", "alice@example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.svc.Users()
			if len(got) != len(tc.wantEmails) {
				t.Fatalf("got %d users, want %d", len(got), len(tc.wantEmails))
			}
			for i, want := range tc.wantEmails {
				if got[i].Email() != want {
					t.Errorf("got[%d] = %q, want %q", i, got[i].Email(), want)
				}
			}
		})
	}
}

func TestService_ValidateUser(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		email    string
		userName string
		wantErrs []error
	}{
		{"valid", "alice@example.com", "Alice", nil},
		{"empty email", "", "Alice", []error{model.ErrEmailRequired}},
		{"invalid email", "not-an-email", "Alice", []error{apperr.ErrEmailInvalid}},
		{"empty name", "alice@example.com", "", []error{model.ErrNameRequired}},
		{"email and name missing", "", "", []error{model.ErrEmailRequired, model.ErrNameRequired}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := newService(t).ValidateUser(tc.email, tc.userName)
			if len(tc.wantErrs) == 0 {
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}
				return
			}
			for _, want := range tc.wantErrs {
				if !errors.Is(err, want) {
					t.Errorf("got %v, want errors.Is(_, %v)", err, want)
				}
			}
		})
	}
}

func TestService_AddUser(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		email    string
		userName string
		wantErr  error
	}{
		{"success", "carol@example.com", "Carol", nil}, // not in the seed
		{"invalid email", "not-an-email", "Nobody", apperr.ErrEmailInvalid},
		{"duplicate", "alice@example.com", "Alice2", model.ErrUserExists},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := seedService(t).AddUser(tc.email, tc.userName)
			if tc.wantErr == nil {
				must(t, err)
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestService_UpdateUser(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddUser("alice@example.com", "Alice"))
		must(t, svc.UpdateUser("alice@example.com", "Alice Smith"))
		if got := svc.User("alice@example.com").Name(); got != "Alice Smith" {
			t.Errorf("got %q, want Alice Smith", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := newService(t).UpdateUser("ghost@example.com", "Ghost")
		if !errors.Is(err, model.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})
}

func TestService_DeleteUser(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddUser("alice@example.com", "Alice"))
		must(t, svc.DeleteUser("alice@example.com"))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := newService(t).DeleteUser("ghost@example.com")
		if !errors.Is(err, model.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})

	t.Run("has associated accounts", func(t *testing.T) {
		t.Parallel()
		err := seedService(t).DeleteUser("alice@example.com")
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

// ── Project ───────────────────────────────────────────────────────────────────

func TestService_Project(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	cases := []struct {
		name    string
		query   string
		wantNil bool
	}{
		{"found", "Alpha", false},
		{"not found", "Ghost", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := svc.Project(tc.query)
			if tc.wantNil && p != nil {
				t.Errorf("got %v, want nil", p)
			}
			if !tc.wantNil && p == nil {
				t.Error("got nil, want non-nil project")
			}
		})
	}
}

func TestService_Projects(t *testing.T) {
	t.Parallel()
	empty := newService(t)
	seeded := seedService(t)

	cases := []struct {
		name      string
		svc       *service.Service
		wantNames []string
	}{
		{"empty", empty, nil},
		{"sorted by name", seeded, []string{"Alpha", "Beta", "Orphan"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.svc.Projects()
			if len(got) != len(tc.wantNames) {
				t.Fatalf("got %d projects, want %d", len(got), len(tc.wantNames))
			}
			for i, want := range tc.wantNames {
				if got[i].Name() != want {
					t.Errorf("got[%d] = %q, want %q", i, got[i].Name(), want)
				}
			}
		})
	}
}

func TestService_ProjectAccounts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		projectName string
		wantErr     error
		wantEmails  []string
	}{
		{"not found", "Ghost", model.ErrProjectNotFound, nil},
		{"none assigned", "Orphan", nil, nil},
		{"assigned", "Alpha", nil, []string{"alice@example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := seedService(t).ProjectAccounts(tc.projectName)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
				}
				return
			}
			must(t, err)
			if len(got) != len(tc.wantEmails) {
				t.Fatalf("got %d accounts, want %d", len(got), len(tc.wantEmails))
			}
			for i, want := range tc.wantEmails {
				if got[i].UserEmail() != want {
					t.Errorf("got[%d] = %q, want %q", i, got[i].UserEmail(), want)
				}
			}
		})
	}
}

func TestService_ProjectSessions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		projectName string
		wantErr     error
		wantIDs     []uint64
	}{
		{"not found", "Ghost", model.ErrProjectNotFound, nil},
		{"none", "Beta", nil, nil},
		{"has sessions", "Alpha", nil, []uint64{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := seedService(t).ProjectSessions(tc.projectName)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
				}
				return
			}
			must(t, err)
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("got %d sessions, want %d", len(got), len(tc.wantIDs))
			}
			for i, want := range tc.wantIDs {
				if got[i].ID() != want {
					t.Errorf("got[%d].ID() = %d, want %d", i, got[i].ID(), want)
				}
			}
		})
	}
}

func TestService_ValidateProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		projectName  string
		description  string
		instructions string
		wantErr      error
	}{
		{"valid", "Alpha", "desc", "inst", nil},
		{"empty name", "", "", "", model.ErrNameRequired},
		{"whitespace-only name", "   ", "", "", model.ErrNameRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := newService(t).ValidateProject(tc.projectName, tc.description, tc.instructions)
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

func TestService_AddProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		projectName string
		wantErr     error
	}{
		{"success", "Gamma", nil}, // not in the seed
		{"empty name", "", model.ErrNameRequired},
		{"duplicate", "Alpha", model.ErrProjectExists},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := seedService(t).AddProject(tc.projectName, "", "")
			if tc.wantErr == nil {
				must(t, err)
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestService_UpdateProject(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddProject("Alpha", "old", "old"))
		must(t, svc.UpdateProject("Alpha", "new", "new"))
		got := svc.Project("Alpha")
		if got.Description() != "new" || got.Instructions() != "new" {
			t.Errorf("got desc %q inst %q, want both %q", got.Description(), got.Instructions(), "new")
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := newService(t).UpdateProject("Ghost", "x", "")
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})
}

func TestService_DeleteProject(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddProject("Alpha", "", ""))
		must(t, svc.DeleteProject("Alpha"))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := newService(t).DeleteProject("Ghost")
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("assigned to an account", func(t *testing.T) {
		t.Parallel()
		// Beta is granted to alice|claude but has no sessions in the seed,
		// isolating the "assigned" branch from the "active sessions" one.
		err := seedService(t).DeleteProject("Beta")
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

// ── Account ───────────────────────────────────────────────────────────────────

func TestService_Account(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		email   string
		vendor  string
		wantErr error
	}{
		{"success", "alice@example.com", "claude", nil},
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound},
		{"vendor not found", "alice@example.com", "ghost", model.ErrVendorNotFound},
		{"account not found", "bob@example.com", "claude", model.ErrAccountNotFound}, // bob exists, claude exists, no bob|claude account
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			acc, err := seedService(t).Account(tc.email, tc.vendor)
			if tc.wantErr == nil {
				must(t, err)
				if acc.UserEmail() != tc.email || acc.VendorName() != tc.vendor {
					t.Errorf("got %+v, want %s|%s", acc.Key(), tc.email, tc.vendor)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestService_Accounts(t *testing.T) {
	t.Parallel()
	empty := newService(t)
	seeded := seedService(t)

	cases := []struct {
		name       string
		svc        *service.Service
		wantEmails []string
	}{
		{"empty", empty, nil},
		{"sorted by key", seeded, []string{"alice@example.com", "bob@example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.svc.Accounts()
			if len(got) != len(tc.wantEmails) {
				t.Fatalf("got %d accounts, want %d", len(got), len(tc.wantEmails))
			}
			for i, want := range tc.wantEmails {
				if got[i].UserEmail() != want {
					t.Errorf("got[%d] = %q, want %q", i, got[i].UserEmail(), want)
				}
			}
		})
	}
}

func TestService_AccountProjects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		email     string
		vendor    string
		wantErr   error
		wantNames []string
	}{
		{"success", "alice@example.com", "claude", nil, []string{"Alpha", "Beta"}},
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound, nil},
		{"vendor not found", "alice@example.com", "ghost", model.ErrVendorNotFound, nil},
		{"account not found", "bob@example.com", "claude", model.ErrAccountNotFound, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := seedService(t).AccountProjects(tc.email, tc.vendor)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
				}
				return
			}
			must(t, err)
			if len(got) != len(tc.wantNames) {
				t.Fatalf("got %d projects, want %d", len(got), len(tc.wantNames))
			}
			for i, want := range tc.wantNames {
				if got[i].Name() != want {
					t.Errorf("got[%d] = %q, want %q", i, got[i].Name(), want)
				}
			}
		})
	}
}

func TestService_AccountSessions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		email   string
		vendor  string
		wantErr error
		wantIDs []uint64
	}{
		{"success", "alice@example.com", "claude", nil, []uint64{1}},
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound, nil},
		{"vendor not found", "alice@example.com", "ghost", model.ErrVendorNotFound, nil},
		{"account not found", "bob@example.com", "claude", model.ErrAccountNotFound, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := seedService(t).AccountSessions(tc.email, tc.vendor)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
				}
				return
			}
			must(t, err)
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("got %d sessions, want %d", len(got), len(tc.wantIDs))
			}
			for i, want := range tc.wantIDs {
				if got[i].ID() != want {
					t.Errorf("got[%d].ID() = %d, want %d", i, got[i].ID(), want)
				}
			}
		})
	}
}

func TestService_AddAccount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		email   string
		vendor  string
		wantErr error
	}{
		{"success", "alice@example.com", "chatgpt", nil}, // alice and chatgpt both exist, never paired
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound},
		{"vendor not found", "alice@example.com", "ghost", model.ErrVendorNotFound},
		{"duplicate", "alice@example.com", "claude", model.ErrAccountExists},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := seedService(t).AddAccount(tc.email, tc.vendor)
			if tc.wantErr == nil {
				must(t, err)
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

// TestService_DeleteAccount covers the resolveAccount branches plus the
// dependency checks for sessions and projects.
func TestService_DeleteAccount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		seeded  bool
		email   string
		vendor  string
		wantErr error
	}{
		{"success", false, "alice@example.com", "claude", nil},
		{"user not found", true, "ghost@example.com", "claude", model.ErrUserNotFound},
		{"vendor not found", true, "alice@example.com", "ghost", model.ErrVendorNotFound},
		{"account not found", true, "bob@example.com", "claude", model.ErrAccountNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var svc *service.Service
			if tc.seeded {
				svc = seedService(t)
			} else {
				svc = newService(t)
				must(t, svc.AddVendor(tc.vendor, true, false))
				must(t, svc.AddUser(tc.email, "Test"))
				must(t, svc.AddAccount(tc.email, tc.vendor))
			}
			err := svc.DeleteAccount(tc.email, tc.vendor)
			if tc.wantErr == nil {
				must(t, err)
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}

	t.Run("has active sessions", func(t *testing.T) {
		t.Parallel()
		// bob|chatgpt has a session and no granted projects in the seed,
		// isolating the "sessions" branch from the "assigned projects" one.
		err := seedService(t).DeleteAccount("bob@example.com", "chatgpt")
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("has assigned projects", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddVendor("claude", true, false))
		must(t, svc.AddUser("alice@example.com", "Alice"))
		must(t, svc.AddAccount("alice@example.com", "claude"))
		must(t, svc.AddProject("Alpha", "", ""))
		must(t, svc.GrantProject("alice@example.com", "claude", "Alpha"))
		// No sessions on this account, isolating the "assigned projects"
		// branch from the "active sessions" one.
		err := svc.DeleteAccount("alice@example.com", "claude")
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

// TestService_PauseAccount covers the resolveAccount branches and verifies
// the pause time round-trips through Account.
func TestService_PauseAccount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		email   string
		vendor  string
		wantErr error
	}{
		{"success", "alice@example.com", "claude", nil},
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound},
		{"vendor not found", "alice@example.com", "ghost", model.ErrVendorNotFound},
		{"account not found", "bob@example.com", "claude", model.ErrAccountNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			until := time.Now().Add(time.Hour).Truncate(time.Second)
			err := svc.PauseAccount(tc.email, tc.vendor, until)
			if tc.wantErr == nil {
				must(t, err)
				acc, aErr := svc.Account(tc.email, tc.vendor)
				must(t, aErr)
				if !acc.PauseUntil().Equal(until) {
					t.Errorf("got %v, want %v", acc.PauseUntil(), until)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

// TestService_GrantProject covers resolveAccount branches plus project
// not found, unsupported vendor, and already-assigned errors.
func TestService_GrantProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		email   string
		vendor  string
		project string
		wantErr error
	}{
		{"success", "alice@example.com", "claude", "Orphan", nil},
		{"user not found", "ghost@example.com", "claude", "Alpha", model.ErrUserNotFound},
		{"vendor not found", "alice@example.com", "ghost", "Alpha", model.ErrVendorNotFound},
		{"account not found", "bob@example.com", "claude", "Alpha", model.ErrAccountNotFound},
		{"project not found", "alice@example.com", "claude", "Ghost", model.ErrProjectNotFound},
		{"vendor does not support projects", "bob@example.com", "chatgpt", "Orphan", model.ErrProjectsUnsupported},
		{"already assigned", "alice@example.com", "claude", "Alpha", errGeneric},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := seedService(t).GrantProject(tc.email, tc.vendor, tc.project)
			switch {
			case tc.wantErr == nil:
				must(t, err)
			case errors.Is(tc.wantErr, errGeneric):
				if err == nil {
					t.Fatal("got nil, want error")
				}
			default:
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
				}
			}
		})
	}
}

// TestService_RevokeProject mirrors TestService_GrantProject and verifies
// that a successful revoke removes the project from AccountProjects.
func TestService_RevokeProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		email   string
		vendor  string
		project string
		wantErr error
	}{
		{"success", "alice@example.com", "claude", "Beta", nil},
		{"user not found", "ghost@example.com", "claude", "Alpha", model.ErrUserNotFound},
		{"vendor not found", "alice@example.com", "ghost", "Alpha", model.ErrVendorNotFound},
		{"account not found", "bob@example.com", "claude", "Alpha", model.ErrAccountNotFound},
		{"project not found", "alice@example.com", "claude", "Ghost", model.ErrProjectNotFound},
		{"not assigned", "alice@example.com", "claude", "Orphan", errGeneric},
		{"has active sessions", "alice@example.com", "claude", "Alpha", errGeneric}, // session 1 uses Alpha
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.RevokeProject(tc.email, tc.vendor, tc.project)
			switch {
			case tc.wantErr == nil:
				must(t, err)
				got, gErr := svc.AccountProjects(tc.email, tc.vendor)
				must(t, gErr)
				for _, p := range got {
					if p.Name() == tc.project {
						t.Errorf("got %v, want %q removed", got, tc.project)
					}
				}
			case errors.Is(tc.wantErr, errGeneric):
				if err == nil {
					t.Fatal("got nil, want error")
				}
			default:
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
				}
			}
		})
	}
}

func TestService_ValidateAccount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		email   string
		vendor  string
		wantErr error
	}{
		{"valid", "alice@example.com", "claude", nil},
		{"missing email", "", "claude", model.ErrEmailRequired},
		{"missing vendor", "alice@example.com", "", model.ErrNameRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := newService(t).ValidateAccount(tc.email, tc.vendor)
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

// ── Session ───────────────────────────────────────────────────────────────────

func TestService_Session(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	cases := []struct {
		name    string
		id      uint64
		wantNil bool
	}{
		{"found", 1, false},
		{"not found", 99, true},
		{"zero id never assigned", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := svc.Session(tc.id)
			if tc.wantNil && s != nil {
				t.Errorf("got %v, want nil", s)
			}
			if !tc.wantNil && s == nil {
				t.Error("got nil, want non-nil session")
			}
		})
	}
}

func TestService_Sessions(t *testing.T) {
	t.Parallel()
	empty := newService(t)
	seeded := seedService(t)

	cases := []struct {
		name    string
		svc     *service.Service
		wantIDs []uint64
	}{
		{"empty", empty, nil},
		{"sorted by ID", seeded, []uint64{1, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.svc.Sessions()
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("got %d sessions, want %d", len(got), len(tc.wantIDs))
			}
			for i, want := range tc.wantIDs {
				if got[i].ID() != want {
					t.Errorf("got[%d].ID() = %d, want %d", i, got[i].ID(), want)
				}
			}
		})
	}
}

// TestService_UpdateSessionName covers resolveSession branches and the
// cross-account guard.
func TestService_UpdateSessionName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		vendor       string
		id           uint64
		sessionName  string
		wantErr      error
		wantErrIsNot error
	}{
		{"success", "alice@example.com", "claude", 1, "New Name", nil, nil},
		{"user not found", "ghost@example.com", "claude", 1, "x", model.ErrUserNotFound, nil},
		{"vendor not found", "alice@example.com", "ghost", 1, "x", model.ErrVendorNotFound, nil},
		{"account not found", "alice@example.com", "chatgpt", 1, "x", model.ErrAccountNotFound, nil},
		{"session not found", "alice@example.com", "claude", 9999, "x", model.ErrSessionNotFound, nil},
		{"cross-account session", "alice@example.com", "claude", 2, "x", apperr.ErrNotFound, model.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.UpdateSessionName(tc.email, tc.vendor, tc.id, tc.sessionName)
			if tc.wantErr == nil {
				must(t, err)
				if got := svc.Session(tc.id).Name(); got != tc.sessionName {
					t.Errorf("got %q, want %q", got, tc.sessionName)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
			if tc.wantErrIsNot != nil && errors.Is(err, tc.wantErrIsNot) {
				t.Errorf("got errors.Is(_, %v) = true, want false", tc.wantErrIsNot)
			}
		})
	}
}

// TestService_UpdateSessionProject covers resolveSession branches plus
// project-not-found and project-not-assigned errors.
func TestService_UpdateSessionProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		vendor       string
		id           uint64
		projectName  string
		wantErr      error
		wantErrIsNot error
	}{
		{"assign", "alice@example.com", "claude", 1, "Beta", nil, nil},
		{"unassign via empty project name", "alice@example.com", "claude", 1, "", nil, nil},
		{"user not found", "ghost@example.com", "claude", 1, "Alpha", model.ErrUserNotFound, nil},
		{"vendor not found", "alice@example.com", "ghost", 1, "Alpha", model.ErrVendorNotFound, nil},
		{"account not found", "alice@example.com", "chatgpt", 1, "Alpha", model.ErrAccountNotFound, nil},
		{"session not found", "alice@example.com", "claude", 9999, "Alpha", model.ErrSessionNotFound, nil},
		{"cross-account session", "alice@example.com", "claude", 2, "Alpha", apperr.ErrNotFound, model.ErrSessionNotFound},
		{"project not found", "alice@example.com", "claude", 1, "Ghost", model.ErrProjectNotFound, nil},
		{"project not assigned to account", "alice@example.com", "claude", 1, "Orphan", errGeneric, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.UpdateSessionProject(tc.email, tc.vendor, tc.id, tc.projectName)
			switch {
			case tc.wantErr == nil:
				must(t, err)
				if got := svc.Session(tc.id).ProjectName(); got != tc.projectName {
					t.Errorf("got %q, want %q", got, tc.projectName)
				}
			case errors.Is(tc.wantErr, errGeneric):
				if err == nil {
					t.Fatal("got nil, want error")
				}
			default:
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
				}
				if tc.wantErrIsNot != nil && errors.Is(err, tc.wantErrIsNot) {
					t.Errorf("got errors.Is(_, %v) = true, want false", tc.wantErrIsNot)
				}
			}
		})
	}
}

// TestService_UpdateSessionPrompts covers resolveSession branches and
// invalid prompt text.
func TestService_UpdateSessionPrompts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		vendor       string
		id           uint64
		queue        []model.Prompt
		wantErr      error
		wantErrIsNot error
	}{
		{"success", "alice@example.com", "claude", 1, []model.Prompt{model.NewPrompt("step one"), model.NewPrompt("step two")}, nil, nil},
		{"invalid prompt", "alice@example.com", "claude", 1, []model.Prompt{model.NewPrompt("")}, model.ErrTextRequired, nil},
		{"user not found", "ghost@example.com", "claude", 1, nil, model.ErrUserNotFound, nil},
		{"vendor not found", "alice@example.com", "ghost", 1, nil, model.ErrVendorNotFound, nil},
		{"account not found", "alice@example.com", "chatgpt", 1, nil, model.ErrAccountNotFound, nil},
		{"session not found", "alice@example.com", "claude", 9999, nil, model.ErrSessionNotFound, nil},
		{"cross-account session", "alice@example.com", "claude", 2, nil, apperr.ErrNotFound, model.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.UpdateSessionPrompts(tc.email, tc.vendor, tc.id, tc.queue)
			if tc.wantErr == nil {
				must(t, err)
				got := svc.Session(tc.id).Queue()
				if len(got) != len(tc.queue) {
					t.Fatalf("got %d prompts, want %d", len(got), len(tc.queue))
				}
				for i, p := range tc.queue {
					if got[i].Text() != p.Text() {
						t.Errorf("got[%d] = %q, want %q", i, got[i].Text(), p.Text())
					}
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
			if tc.wantErrIsNot != nil && errors.Is(err, tc.wantErrIsNot) {
				t.Errorf("got errors.Is(_, %v) = true, want false", tc.wantErrIsNot)
			}
		})
	}
}

// TestService_EndSession covers the full resolveSession branch set.
func TestService_EndSession(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		vendor       string
		id           uint64
		wantErr      error
		wantErrIsNot error
	}{
		{"success", "alice@example.com", "claude", 1, nil, nil},
		{"user not found", "ghost@example.com", "claude", 1, model.ErrUserNotFound, nil},
		{"vendor not found", "alice@example.com", "ghost", 1, model.ErrVendorNotFound, nil},
		{"account not found", "alice@example.com", "chatgpt", 1, model.ErrAccountNotFound, nil},
		{"session not found", "alice@example.com", "claude", 9999, model.ErrSessionNotFound, nil},
		{"cross-account session", "alice@example.com", "claude", 2, apperr.ErrNotFound, model.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.EndSession(tc.email, tc.vendor, tc.id)
			if tc.wantErr == nil {
				must(t, err)
				if svc.Session(tc.id) != nil {
					t.Error("session should no longer be retrievable")
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
			if tc.wantErrIsNot != nil && errors.Is(err, tc.wantErrIsNot) {
				t.Errorf("got errors.Is(_, %v) = true, want false", tc.wantErrIsNot)
			}
		})
	}
}

// ── Workflows ─────────────────────────────────────────────────────────────────

// TestService_StartSession covers account creation for an existing user,
// project and queue passthrough, and the error branches.
func TestService_StartSession(t *testing.T) {
	t.Parallel()

	t.Run("creates an account for an existing user", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddVendor("claude", true, false))
		must(t, svc.AddUser("alice@example.com", "Alice"))
		sess, err := svc.StartSession("alice@example.com", "claude", "", "Name", nil)
		must(t, err)
		if sess.ProjectName() != "" {
			t.Errorf("got project %q, want empty", sess.ProjectName())
		}
		if _, err := svc.Account("alice@example.com", "claude"); err != nil {
			t.Errorf("expected account to be created: %v", err)
		}
	})

	t.Run("reuses an existing account without creating a duplicate", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		_, err := svc.StartSession("alice@example.com", "claude", "", "second", nil)
		must(t, err)
		got, err := svc.AccountSessions("alice@example.com", "claude")
		must(t, err)
		if len(got) != 2 {
			t.Errorf("got %d sessions on the account, want 2", len(got))
		}
		// Only one account can exist for this key; a second AddAccount must
		// still fail, confirming StartSession did not create a duplicate.
		err = svc.AddAccount("alice@example.com", "claude")
		if !errors.Is(err, model.ErrAccountExists) {
			t.Errorf("got %v, want ErrAccountExists", err)
		}
	})

	t.Run("with project and queue", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		queue := []model.Prompt{model.NewPrompt("continue")}
		sess, err := svc.StartSession("alice@example.com", "claude", "Alpha", "Name", queue)
		must(t, err)
		if sess.ProjectName() != "Alpha" {
			t.Errorf("got project %q, want Alpha", sess.ProjectName())
		}
		if len(sess.Queue()) != 1 {
			t.Errorf("got %d queue items, want 1", len(sess.Queue()))
		}
	})

	t.Run("project not granted to account", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).StartSession("alice@example.com", "claude", "Orphan", "", nil)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	errorCases := []struct {
		name        string
		email       string
		vendor      string
		project     string
		sessionName string
		queue       []model.Prompt
		wantErr     error
	}{
		{"user not found", "ghost@example.com", "claude", "", "", nil, model.ErrUserNotFound},
		{"vendor not found", "alice@example.com", "ghost", "", "", nil, model.ErrVendorNotFound},
		{"project not found", "alice@example.com", "claude", "Ghost", "", nil, model.ErrProjectNotFound},
		{
			"invalid message in queue", "alice@example.com", "claude", "", "",
			[]model.Prompt{model.NewPrompt("ok"), model.NewPrompt("")},
			model.ErrTextRequired,
		},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := seedService(t).StartSession(tc.email, tc.vendor, tc.project, tc.sessionName, tc.queue)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

func TestService_UpdateSessionURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		vendor       string
		id           uint64
		url          string
		wantErr      error
		wantErrIsNot error
	}{
		{"success", "alice@example.com", "claude", 1, " https://example.com/chat/1 ", nil, nil},
		{"user not found", "ghost@example.com", "claude", 1, "x", model.ErrUserNotFound, nil},
		{"vendor not found", "alice@example.com", "ghost", 1, "x", model.ErrVendorNotFound, nil},
		{"account not found", "alice@example.com", "chatgpt", 1, "x", model.ErrAccountNotFound, nil},
		{"session not found", "alice@example.com", "claude", 9999, "x", model.ErrSessionNotFound, nil},
		{"cross-account session", "alice@example.com", "claude", 2, "x", apperr.ErrNotFound, model.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.UpdateSessionURL(tc.email, tc.vendor, tc.id, tc.url)
			if tc.wantErr == nil {
				must(t, err)
				if got := svc.Session(tc.id).URL(); got != "https://example.com/chat/1" {
					t.Errorf("URL() = %q, want trimmed URL", got)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
			if tc.wantErrIsNot != nil && errors.Is(err, tc.wantErrIsNot) {
				t.Errorf("got errors.Is(_, %v) = true, want false", tc.wantErrIsNot)
			}
		})
	}
}

func TestService_UpdateSessionFinish(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		vendor       string
		id           uint64
		finish       bool
		wantErr      error
		wantErrIsNot error
	}{
		{"success", "alice@example.com", "claude", 1, true, nil, nil},
		{"user not found", "ghost@example.com", "claude", 1, true, model.ErrUserNotFound, nil},
		{"vendor not found", "alice@example.com", "ghost", 1, true, model.ErrVendorNotFound, nil},
		{"account not found", "alice@example.com", "chatgpt", 1, true, model.ErrAccountNotFound, nil},
		{"session not found", "alice@example.com", "claude", 9999, true, model.ErrSessionNotFound, nil},
		{"cross-account session", "alice@example.com", "claude", 2, true, apperr.ErrNotFound, model.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.UpdateSessionFinish(tc.email, tc.vendor, tc.id, tc.finish)
			if tc.wantErr == nil {
				must(t, err)
				if !svc.Session(tc.id).Finish() {
					t.Error("Finish() = false, want true")
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
			if tc.wantErrIsNot != nil && errors.Is(err, tc.wantErrIsNot) {
				t.Errorf("got errors.Is(_, %v) = true, want false", tc.wantErrIsNot)
			}
		})
	}
}

func TestService_LatestSession(t *testing.T) {
	t.Parallel()

	t.Run("returns the touched session", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		must(t, svc.UpdateSessionPrompts("alice@example.com", "claude", 1, []model.Prompt{model.NewPrompt("next")}))

		sess, ok, err := svc.LatestSession("alice@example.com", "claude")
		must(t, err)
		if !ok || sess.ID() != 1 {
			t.Errorf("LatestSession() = %v, %v; want session 1, true", sess, ok)
		}
	})

	t.Run("no touched session", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		sess, ok, err := svc.LatestSession("bob@example.com", "chatgpt")
		must(t, err)
		if ok || sess != nil {
			t.Errorf("LatestSession() = %v, %v; want nil, false", sess, ok)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		_, _, err := seedService(t).LatestSession("alice@example.com", "chatgpt")
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()
		_, _, err := seedService(t).LatestSession("ghost@example.com", "claude")
		if !errors.Is(err, model.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})

	t.Run("vendor not found", func(t *testing.T) {
		t.Parallel()
		_, _, err := seedService(t).LatestSession("alice@example.com", "ghost")
		if !errors.Is(err, model.ErrVendorNotFound) {
			t.Errorf("got %v, want ErrVendorNotFound", err)
		}
	})
}

func TestService_AccountSessionsActivityOrder(t *testing.T) {
	t.Parallel()

	t.Run("zero updated-at sessions are ordered by ID descending", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		got, err := svc.AccountSessionsActivityOrder("alice@example.com", "claude")
		must(t, err)
		if len(got) != 1 || got[0].ID() != 1 {
			t.Errorf("got %v, want session 1", got)
		}

		got, err = svc.AccountSessionsActivityOrder("bob@example.com", "chatgpt")
		must(t, err)
		if len(got) != 1 || got[0].ID() != 2 {
			t.Errorf("got %v, want session 2", got)
		}
	})

	t.Run("touched session comes first", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		_, err := svc.StartSession("alice@example.com", "claude", "", "", nil)
		must(t, err)
		must(t, svc.UpdateSessionPrompts("alice@example.com", "claude", 1, []model.Prompt{model.NewPrompt("next")}))

		got, err := svc.AccountSessionsActivityOrder("alice@example.com", "claude")
		must(t, err)
		if len(got) != 2 || got[0].ID() != 1 {
			t.Errorf("got %v, want session 1 first", got)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).AccountSessionsActivityOrder("ghost@example.com", "claude")
		if !errors.Is(err, model.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).AccountSessionsActivityOrder("alice@example.com", "chatgpt")
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestService_AddNote(t *testing.T) {
	t.Parallel()

	t.Run("no project and no account", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("  text  ", "", "", "")
		must(t, err)
		if n.Text() != "text" || n.ProjectName() != "" {
			t.Errorf("note = text %q project %q", n.Text(), n.ProjectName())
		}
		if _, ok := n.AccountKey(); ok {
			t.Error("note has an account, want none")
		}
	})

	t.Run("project only", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("text", "Alpha", "", "")
		must(t, err)
		if n.ProjectName() != "Alpha" {
			t.Errorf("ProjectName() = %q, want Alpha", n.ProjectName())
		}
		if _, ok := n.AccountKey(); ok {
			t.Error("note has an account, want none")
		}
	})

	t.Run("account only", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("text", "", "alice@example.com", "claude")
		must(t, err)
		if key, ok := n.AccountKey(); !ok || key.Email != "alice@example.com" {
			t.Errorf("AccountKey() = %v, %v; want alice, true", key, ok)
		}
	})

	t.Run("project and account", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("text", "Alpha", "alice@example.com", "claude")
		must(t, err)
		if n.ProjectName() != "Alpha" {
			t.Errorf("ProjectName() = %q, want Alpha", n.ProjectName())
		}
		if key, ok := n.AccountKey(); !ok || key.Email != "alice@example.com" {
			t.Errorf("AccountKey() = %v, %v; want alice, true", key, ok)
		}
	})

	t.Run("empty text", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).AddNote("  ", "", "", "")
		if !errors.Is(err, model.ErrTextRequired) {
			t.Errorf("got %v, want ErrTextRequired", err)
		}
	})

	t.Run("project not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).AddNote("text", "Ghost", "", "")
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).AddNote("text", "", "alice@example.com", "chatgpt")
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("exactly one account field empty", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name   string
			email  string
			vendor string
		}{
			{"email empty", "", "claude"},
			{"vendor empty", "alice@example.com", ""},
			{"email whitespace", "   ", "claude"},
			{"vendor whitespace", "alice@example.com", "   "},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				_, err := seedService(t).AddNote("text", "", tc.email, tc.vendor)
				if !errors.Is(err, model.ErrKeyRequired) {
					t.Errorf("got %v, want ErrKeyRequired", err)
				}
			})
		}
	})

	t.Run("whitespace only account fields mean no account", func(t *testing.T) {
		t.Parallel()
		n, err := seedService(t).AddNote("text", "", "   ", "   ")
		must(t, err)
		if _, ok := n.AccountKey(); ok {
			t.Error("note has an account, want none")
		}
	})

	t.Run("project not granted to account", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).AddNote("text", "Orphan", "alice@example.com", "claude")
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestService_UpdateNote(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("old", "", "", "")
		must(t, err)
		must(t, svc.UpdateNote(n.ID(), "new", "Alpha", "alice@example.com", "claude"))
		got := svc.Note(n.ID())
		if got.Text() != "new" || got.ProjectName() != "Alpha" {
			t.Errorf("note = text %q project %q", got.Text(), got.ProjectName())
		}
		if key, ok := got.AccountKey(); !ok || key.Email != "alice@example.com" {
			t.Errorf("AccountKey() = %v, %v; want alice, true", key, ok)
		}
	})

	t.Run("no-op keeps the same pointer", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("same", "Alpha", "alice@example.com", "claude")
		must(t, err)
		must(t, svc.UpdateNote(n.ID(), "  same  ", "Alpha", "alice@example.com", "claude"))
		if got := svc.Note(n.ID()); got != n {
			t.Error("expected the same pointer for an unchanged note")
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := seedService(t).UpdateNote(99, "text", "", "", "")
		if !errors.Is(err, model.ErrNoteNotFound) {
			t.Errorf("got %v, want ErrNoteNotFound", err)
		}
	})

	t.Run("empty text", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("text", "", "", "")
		must(t, err)
		err = svc.UpdateNote(n.ID(), "  ", "", "", "")
		if !errors.Is(err, model.ErrTextRequired) {
			t.Errorf("got %v, want ErrTextRequired", err)
		}
	})

	t.Run("exactly one account field empty", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("text", "", "", "")
		must(t, err)
		err = svc.UpdateNote(n.ID(), "text", "", "alice@example.com", "")
		if !errors.Is(err, model.ErrKeyRequired) {
			t.Errorf("got %v, want ErrKeyRequired", err)
		}
	})
}

func TestService_DeleteNote(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("text", "", "", "")
		must(t, err)
		must(t, svc.DeleteNote(n.ID()))
		if got := svc.Note(n.ID()); got != nil {
			t.Errorf("Note(%d) = %v, want nil", n.ID(), got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := seedService(t).DeleteNote(99)
		if !errors.Is(err, model.ErrNoteNotFound) {
			t.Errorf("got %v, want ErrNoteNotFound", err)
		}
	})
}

func TestService_NoteAndNotes(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		if got := svc.Notes(); len(got) != 0 {
			t.Errorf("Notes() = %v, want empty", got)
		}
		if got := svc.Note(1); got != nil {
			t.Errorf("Note(1) = %v, want nil", got)
		}
	})

	t.Run("sorted by ID", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n1, err := svc.AddNote("one", "", "", "")
		must(t, err)
		n2, err := svc.AddNote("two", "", "", "")
		must(t, err)
		got := svc.Notes()
		if len(got) != 2 || got[0].ID() != n1.ID() || got[1].ID() != n2.ID() {
			t.Errorf("Notes() = %v, want IDs %d, %d", got, n1.ID(), n2.ID())
		}
		if svc.Note(n1.ID()) != n1 {
			t.Error("Note() did not return the stored pointer")
		}
	})
}

func TestService_VisibleNotes(t *testing.T) {
	t.Parallel()

	t.Run("filters and visibility", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		aliceNote, err := svc.AddNote("alice", "", "alice@example.com", "claude")
		must(t, err)
		bobNote, err := svc.AddNote("bob", "", "bob@example.com", "chatgpt")
		must(t, err)

		got, err := svc.VisibleNotes("alice@example.com", "claude", model.AllProjectFilter())
		must(t, err)
		if len(got) != 1 || got[0].ID() != aliceNote.ID() {
			t.Errorf("VisibleNotes(alice) = %v, want alice's note", got)
		}
		got, err = svc.VisibleNotes("bob@example.com", "chatgpt", model.AllProjectFilter())
		must(t, err)
		if len(got) != 1 || got[0].ID() != bobNote.ID() {
			t.Errorf("VisibleNotes(bob) = %v, want bob's note", got)
		}
	})

	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).VisibleNotes("alice@example.com", "chatgpt", model.AllProjectFilter())
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})
}

func TestService_ProjectNotes(t *testing.T) {
	t.Parallel()

	t.Run("returns the project's notes", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("text", "Alpha", "", "")
		must(t, err)
		_, err = svc.AddNote("other", "Beta", "", "")
		must(t, err)

		got, err := svc.ProjectNotes("Alpha")
		must(t, err)
		if len(got) != 1 || got[0].ID() != n.ID() {
			t.Errorf("ProjectNotes(Alpha) = %v, want %d", got, n.ID())
		}
	})

	t.Run("project not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).ProjectNotes("Ghost")
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})
}

//nolint:cyclop // Table-driven test with independent cases; splitting would obscure coverage.
func TestService_StartSessionAtomic(t *testing.T) {
	t.Parallel()

	t.Run("blank start", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		sess, err := svc.StartSessionAtomic("alice@example.com", "claude", "", "", "name", "", 0)
		must(t, err)
		if sess.ProjectName() != "" || len(sess.Queue()) != 0 {
			t.Errorf("session = project %q queue %v, want empty", sess.ProjectName(), sess.Queue())
		}
	})

	t.Run("existing project and prompt", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		sess, err := svc.StartSessionAtomic("alice@example.com", "claude", "Alpha", "", "name", " hello ", 0)
		must(t, err)
		if sess.ProjectName() != "Alpha" {
			t.Errorf("ProjectName() = %q, want Alpha", sess.ProjectName())
		}
		if got := sess.Queue(); len(got) != 1 || got[0].Text() != "hello" {
			t.Errorf("Queue() = %v, want one trimmed prompt", got)
		}
	})

	t.Run("new project", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		sess, err := svc.StartSessionAtomic("alice@example.com", "claude", "", " Gamma ", "name", "hello", 0)
		must(t, err)
		if sess.ProjectName() != "Gamma" {
			t.Errorf("ProjectName() = %q, want Gamma", sess.ProjectName())
		}
		if svc.Project("Gamma") == nil {
			t.Error("new project was not created")
		}
	})

	t.Run("from a note", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("note text", "", "alice@example.com", "claude")
		must(t, err)

		sess, err := svc.StartSessionAtomic("alice@example.com", "claude", "", "", "name", "note text", n.ID())
		must(t, err)
		if got := sess.Queue(); len(got) != 1 || got[0].Text() != "note text" {
			t.Errorf("Queue() = %v, want the note text", got)
		}
		if svc.Note(n.ID()) != nil {
			t.Error("note still exists after a successful start")
		}
	})

	t.Run("account not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).StartSessionAtomic("alice@example.com", "chatgpt", "", "", "", "", 0)
		if !errors.Is(err, model.ErrAccountNotFound) {
			t.Errorf("got %v, want ErrAccountNotFound", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).StartSessionAtomic("ghost@example.com", "claude", "", "", "", "", 0)
		if !errors.Is(err, model.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})

	t.Run("vendor not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).StartSessionAtomic("alice@example.com", "ghost", "", "", "", "", 0)
		if !errors.Is(err, model.ErrVendorNotFound) {
			t.Errorf("got %v, want ErrVendorNotFound", err)
		}
	})

	t.Run("project not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).StartSessionAtomic("alice@example.com", "claude", "Ghost", "", "", "", 0)
		if !errors.Is(err, model.ErrProjectNotFound) {
			t.Errorf("got %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("note not found", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).StartSessionAtomic("alice@example.com", "claude", "", "", "", "text", 99)
		if !errors.Is(err, model.ErrNoteNotFound) {
			t.Errorf("got %v, want ErrNoteNotFound", err)
		}
	})

	t.Run("note requires prompt text", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("note text", "", "alice@example.com", "claude")
		must(t, err)
		_, err = svc.StartSessionAtomic("alice@example.com", "claude", "", "", "", "", n.ID())
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("duplicate new project", func(t *testing.T) {
		t.Parallel()
		_, err := seedService(t).StartSessionAtomic("alice@example.com", "claude", "", "Alpha", "", "", 0)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	t.Run("note project mismatch", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		n, err := svc.AddNote("note text", "Alpha", "", "")
		must(t, err)
		_, err = svc.StartSessionAtomic("alice@example.com", "claude", "Beta", "", "", "note text", n.ID())
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})
}

func TestService_RenameProject(t *testing.T) {
	t.Parallel()
	svc := seedService(t)
	project := svc.Project("Alpha")
	projectID := project.ID()

	renamed, err := svc.RenameProject("Alpha", "Renamed")
	must(t, err)
	if renamed.ID() != projectID || renamed.Name() != "Renamed" {
		t.Errorf("renamed = id %d name %q, want id %d name Renamed", renamed.ID(), renamed.Name(), projectID)
	}
	if svc.Project("Alpha") != nil {
		t.Error("old name still resolves")
	}
	if got := svc.Session(1); got == nil || got.ProjectName() != "Renamed" || got.ProjectID() != projectID {
		t.Errorf("session after rename = %v", got)
	}

	if _, err := svc.RenameProject("Alpha", "Other"); !errors.Is(err, model.ErrProjectNotFound) {
		t.Errorf("rename missing project = %v, want ErrProjectNotFound", err)
	}
	if _, err := svc.RenameProject("Renamed", "Beta"); !errors.Is(err, model.ErrProjectExists) {
		t.Errorf("rename to existing name = %v, want ErrProjectExists", err)
	}
	if _, err := svc.RenameProject("Renamed", "  "); !errors.Is(err, model.ErrNameRequired) {
		t.Errorf("rename to empty name = %v, want ErrNameRequired", err)
	}
}
