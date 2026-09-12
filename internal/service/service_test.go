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
//	Providers: "claude" (HasProjects=true), "chatgpt" (HasProjects=false)
//	Users:     alice@example.com / Alice, bob@example.com / Bob
//	Projects:  "Alpha", "Beta", "Orphan"
//	Accounts:  alice@example.com|claude (granted Alpha + Beta)
//	           bob@example.com|chatgpt  (no projects granted)
//	Sessions:
//	  ID 1: alice@example.com|claude, project=Alpha, topic="Architecture review"
//	  ID 2: bob@example.com|chatgpt, no project
//
// The fixture is deliberately shaped so every resolveAccount/resolveSession
// edge case can be reached by picking an existing or non-existing
// email/provider pair against this single fixture, with no per-case setup:
// alice+chatgpt and bob+claude each name two entities that individually
// exist but were never paired into an account.
func seedService(t *testing.T) *service.Service {
	t.Helper()
	svc := newService(t)
	must(t, svc.AddProvider("claude", true, false))
	must(t, svc.AddProvider("chatgpt", false, false))
	must(t, svc.AddUser("alice@example.com", "Alice"))
	must(t, svc.AddUser("bob@example.com", "Bob"))
	must(t, svc.AddProject("Alpha", "", ""))
	must(t, svc.AddProject("Beta", "", ""))
	must(t, svc.AddProject("Orphan", "", ""))
	must(t, svc.AddAccount("alice@example.com", "claude"))
	must(t, svc.AddAccount("bob@example.com", "chatgpt"))
	must(t, svc.GrantProject("alice@example.com", "claude", "Alpha"))
	must(t, svc.GrantProject("alice@example.com", "claude", "Beta"))
	_, err := svc.StartSession("alice@example.com", "Alice", "claude", "Alpha", "Architecture review", nil)
	must(t, err)
	_, err = svc.StartSession("bob@example.com", "Bob", "chatgpt", "", "", nil)
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

// ── Provider ──────────────────────────────────────────────────────────────────

func TestService_Provider(t *testing.T) {
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
			p := svc.Provider(tc.query)
			if tc.wantNil && p != nil {
				t.Errorf("got %v, want nil", p)
			}
			if !tc.wantNil && p == nil {
				t.Error("got nil, want non-nil provider")
			}
		})
	}
}

func TestService_Providers(t *testing.T) {
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
			got := tc.svc.Providers()
			if len(got) != len(tc.wantNames) {
				t.Fatalf("got %d providers, want %d", len(got), len(tc.wantNames))
			}
			for i, want := range tc.wantNames {
				if got[i].Name() != want {
					t.Errorf("got[%d] = %q, want %q", i, got[i].Name(), want)
				}
			}
		})
	}
}

func TestService_ValidateProvider(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		providerName    string
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
			err := newService(t).ValidateProvider(tc.providerName, tc.hasProjects, tc.hasDeepResearch)
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

func TestService_AddProvider(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		providerName string
		wantErr      error
	}{
		{"success", "gemini", nil}, // not in the seed
		{"empty name", "", model.ErrNameRequired},
		{"duplicate", "claude", model.ErrProviderExists},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := seedService(t).AddProvider(tc.providerName, true, false)
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

func TestService_UpdateProvider(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddProvider("claude", false, false))
		must(t, svc.UpdateProvider("claude", true, true))
		got := svc.Provider("claude")
		if !got.HasProjects() || !got.HasDeepResearch() {
			t.Errorf("got HasProjects=%v HasDeepResearch=%v, want both true", got.HasProjects(), got.HasDeepResearch())
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := newService(t).UpdateProvider("ghost", true, false)
		if !errors.Is(err, model.ErrProviderNotFound) {
			t.Errorf("got %v, want ErrProviderNotFound", err)
		}
	})
}

func TestService_DeleteProvider(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddProvider("claude", true, false))
		must(t, svc.DeleteProvider("claude"))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		err := newService(t).DeleteProvider("ghost")
		if !errors.Is(err, model.ErrProviderNotFound) {
			t.Errorf("got %v, want ErrProviderNotFound", err)
		}
	})

	t.Run("has associated accounts", func(t *testing.T) {
		t.Parallel()
		err := seedService(t).DeleteProvider("claude")
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

	cases := []struct {
		name       string
		svc        *service.Service
		wantEmails []string
	}{
		{"empty", empty, nil},
		{"sorted by email", seeded, []string{"alice@example.com", "bob@example.com"}},
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
		name     string
		email    string
		provider string
		wantErr  error
	}{
		{"success", "alice@example.com", "claude", nil},
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound},
		{"provider not found", "alice@example.com", "ghost", model.ErrProviderNotFound},
		{"account not found", "bob@example.com", "claude", model.ErrAccountNotFound}, // bob exists, claude exists, no bob|claude account
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			acc, err := seedService(t).Account(tc.email, tc.provider)
			if tc.wantErr == nil {
				must(t, err)
				if acc.UserEmail() != tc.email || acc.ProviderName() != tc.provider {
					t.Errorf("got %+v, want %s|%s", acc.Key(), tc.email, tc.provider)
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
		provider  string
		wantErr   error
		wantNames []string
	}{
		{"success", "alice@example.com", "claude", nil, []string{"Alpha", "Beta"}},
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound, nil},
		{"provider not found", "alice@example.com", "ghost", model.ErrProviderNotFound, nil},
		{"account not found", "bob@example.com", "claude", model.ErrAccountNotFound, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := seedService(t).AccountProjects(tc.email, tc.provider)
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
		name     string
		email    string
		provider string
		wantErr  error
		wantIDs  []uint64
	}{
		{"success", "alice@example.com", "claude", nil, []uint64{1}},
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound, nil},
		{"provider not found", "alice@example.com", "ghost", model.ErrProviderNotFound, nil},
		{"account not found", "bob@example.com", "claude", model.ErrAccountNotFound, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := seedService(t).AccountSessions(tc.email, tc.provider)
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
		name     string
		email    string
		provider string
		wantErr  error
	}{
		{"success", "alice@example.com", "chatgpt", nil}, // alice and chatgpt both exist, never paired
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound},
		{"provider not found", "alice@example.com", "ghost", model.ErrProviderNotFound},
		{"duplicate", "alice@example.com", "claude", model.ErrAccountExists},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := seedService(t).AddAccount(tc.email, tc.provider)
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

// TestService_CreateAccount keeps the success/behavioural cases as
// individual subtests for the same reason TestService_StartSession does:
// each verifies a different aspect of the on-demand user creation
// (idempotency, name preservation) or the account itself (duplicate
// rejection), which a single table would obscure behind a per-row
// verification function. The pure error-sentinel cases share one action
// and one assertion shape, so those are tabled.
func TestService_CreateAccount(t *testing.T) {
	t.Parallel()

	t.Run("creates user and account on demand", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddProvider("claude", true, false))
		acc, err := svc.CreateAccount("alice@example.com", "Alice", "claude")
		must(t, err)
		if acc.UserEmail() != "alice@example.com" || acc.ProviderName() != "claude" {
			t.Errorf("got %+v, want alice@example.com|claude", acc.Key())
		}
		if svc.User("alice@example.com") == nil {
			t.Error("expected user to be created")
		}
	})

	t.Run("does not change an existing user's name", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddProvider("claude", true, false))
		must(t, svc.AddUser("alice@example.com", "Alice"))
		_, err := svc.CreateAccount("alice@example.com", "ignored name", "claude")
		must(t, err)
		if got := svc.User("alice@example.com").Name(); got != "Alice" {
			t.Errorf("got name %q, want existing name preserved (Alice)", got)
		}
	})

	t.Run("duplicate account", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t) // alice|claude already exists
		_, err := svc.CreateAccount("alice@example.com", "Alice", "claude")
		if !errors.Is(err, model.ErrAccountExists) {
			t.Errorf("got %v, want ErrAccountExists", err)
		}
	})

	errorCases := []struct {
		name     string
		email    string
		userName string
		provider string
		wantErr  error
	}{
		{"provider not found", "alice@example.com", "Alice", "ghost", model.ErrProviderNotFound},
		{"new user creation fails on invalid email", "not-an-email", "Nobody", "claude", apperr.ErrEmailInvalid},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := seedService(t).CreateAccount(tc.email, tc.userName, tc.provider)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}

// TestService_DeleteAccount tables the four resolveAccount-shaped cases
// (success plus its three not-found branches) and keeps the two
// dependent-check cases as separate subtests, since those two errors carry
// no sentinel and so don't fit the same errors.Is-shaped assertion as the
// other four. "success" needs a dependency-free account that the seeded
// fixture's two accounts don't have (both have a session), hence `seeded`.
func TestService_DeleteAccount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		seeded   bool
		email    string
		provider string
		wantErr  error
	}{
		{"success", false, "alice@example.com", "claude", nil},
		{"user not found", true, "ghost@example.com", "claude", model.ErrUserNotFound},
		{"provider not found", true, "alice@example.com", "ghost", model.ErrProviderNotFound},
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
				must(t, svc.AddProvider(tc.provider, true, false))
				must(t, svc.AddUser(tc.email, "Test"))
				must(t, svc.AddAccount(tc.email, tc.provider))
			}
			err := svc.DeleteAccount(tc.email, tc.provider)
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
		must(t, svc.AddProvider("claude", true, false))
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

// TestService_PauseAccount tables the full resolveAccount branch set;
// "success" verifies the input pause time round-trips through Account().
func TestService_PauseAccount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		email    string
		provider string
		wantErr  error
	}{
		{"success", "alice@example.com", "claude", nil},
		{"user not found", "ghost@example.com", "claude", model.ErrUserNotFound},
		{"provider not found", "alice@example.com", "ghost", model.ErrProviderNotFound},
		{"account not found", "bob@example.com", "claude", model.ErrAccountNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			until := time.Now().Add(time.Hour).Truncate(time.Second)
			err := svc.PauseAccount(tc.email, tc.provider, until)
			if tc.wantErr == nil {
				must(t, err)
				acc, aErr := svc.Account(tc.email, tc.provider)
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

// TestService_GrantProject tables resolveAccount's full branch set plus
// this method's own branches (project-not-found, provider-unsupported,
// already-assigned). "already assigned" carries no sentinel, so it uses
// errGeneric instead of a specific error value.
func TestService_GrantProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		email    string
		provider string
		project  string
		wantErr  error
	}{
		{"success", "alice@example.com", "claude", "Orphan", nil},
		{"user not found", "ghost@example.com", "claude", "Alpha", model.ErrUserNotFound},
		{"provider not found", "alice@example.com", "ghost", "Alpha", model.ErrProviderNotFound},
		{"account not found", "bob@example.com", "claude", "Alpha", model.ErrAccountNotFound},
		{"project not found", "alice@example.com", "claude", "Ghost", model.ErrProjectNotFound},
		{"provider does not support projects", "bob@example.com", "chatgpt", "Orphan", model.ErrProjectsUnsupported},
		{"already assigned", "alice@example.com", "claude", "Alpha", errGeneric},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := seedService(t).GrantProject(tc.email, tc.provider, tc.project)
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

// TestService_RevokeProject mirrors TestService_GrantProject's shape;
// "not assigned" and "has active sessions" also use errGeneric. "success"
// additionally verifies the project no longer appears in AccountProjects.
func TestService_RevokeProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		email    string
		provider string
		project  string
		wantErr  error
	}{
		{"success", "alice@example.com", "claude", "Beta", nil},
		{"user not found", "ghost@example.com", "claude", "Alpha", model.ErrUserNotFound},
		{"provider not found", "alice@example.com", "ghost", "Alpha", model.ErrProviderNotFound},
		{"account not found", "bob@example.com", "claude", "Alpha", model.ErrAccountNotFound},
		{"project not found", "alice@example.com", "claude", "Ghost", model.ErrProjectNotFound},
		{"not assigned", "alice@example.com", "claude", "Orphan", errGeneric},
		{"has active sessions", "alice@example.com", "claude", "Alpha", errGeneric}, // session 1 uses Alpha
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.RevokeProject(tc.email, tc.provider, tc.project)
			switch {
			case tc.wantErr == nil:
				must(t, err)
				got, gErr := svc.AccountProjects(tc.email, tc.provider)
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
		name     string
		email    string
		provider string
		wantErr  error
	}{
		{"valid", "alice@example.com", "claude", nil},
		{"missing email", "", "claude", model.ErrEmailRequired},
		{"missing provider", "alice@example.com", "", model.ErrNameRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := newService(t).ValidateAccount(tc.email, tc.provider)
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

// TestService_UpdateSessionTopic tables every resolveSession-shaped case:
// success, the three resolveAccount not-found branches, session-not-found,
// and the cross-account guard. The guard must match apperr.ErrNotFound but
// must NOT match the specific model.ErrSessionNotFound; wantErrIsNot
// asserts that negative for the one row that needs it.
func TestService_UpdateSessionTopic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		provider     string
		id           uint64
		topic        string
		wantErr      error
		wantErrIsNot error
	}{
		{"success", "alice@example.com", "claude", 1, "New Topic", nil, nil},
		{"user not found", "ghost@example.com", "claude", 1, "x", model.ErrUserNotFound, nil},
		{"provider not found", "alice@example.com", "ghost", 1, "x", model.ErrProviderNotFound, nil},
		{"account not found", "alice@example.com", "chatgpt", 1, "x", model.ErrAccountNotFound, nil},
		{"session not found", "alice@example.com", "claude", 9999, "x", model.ErrSessionNotFound, nil},
		{"cross-account session", "alice@example.com", "claude", 2, "x", apperr.ErrNotFound, model.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.UpdateSessionTopic(tc.email, tc.provider, tc.id, tc.topic)
			if tc.wantErr == nil {
				must(t, err)
				if got := svc.Session(tc.id).Topic(); got != tc.topic {
					t.Errorf("got %q, want %q", got, tc.topic)
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

// TestService_UpdateSessionProject extends the same resolveSession-shaped
// set with this method's own branches: project-not-found (sentinel) and
// project-not-assigned-to-account (errGeneric, no sentinel available). On
// success the expected ProjectName() always equals the input projectName
// itself (assign or unassign-via-empty-string), so no separate
// want-field is needed for it.
func TestService_UpdateSessionProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		provider     string
		id           uint64
		projectName  string
		wantErr      error
		wantErrIsNot error
	}{
		{"assign", "alice@example.com", "claude", 1, "Beta", nil, nil},
		{"unassign via empty project name", "alice@example.com", "claude", 1, "", nil, nil},
		{"user not found", "ghost@example.com", "claude", 1, "Alpha", model.ErrUserNotFound, nil},
		{"provider not found", "alice@example.com", "ghost", 1, "Alpha", model.ErrProviderNotFound, nil},
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
			err := svc.UpdateSessionProject(tc.email, tc.provider, tc.id, tc.projectName)
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

// TestService_UpdateSessionPrompts tables the same resolveSession-shaped
// set plus this method's own validation branch (invalid prompt text).
func TestService_UpdateSessionPrompts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		provider     string
		id           uint64
		queue        []model.Prompt
		wantErr      error
		wantErrIsNot error
	}{
		{"success", "alice@example.com", "claude", 1, []model.Prompt{model.NewPrompt("step one", false), model.NewPrompt("step two", true)}, nil, nil},
		{"invalid prompt", "alice@example.com", "claude", 1, []model.Prompt{model.NewPrompt("", false)}, model.ErrTextRequired, nil},
		{"user not found", "ghost@example.com", "claude", 1, nil, model.ErrUserNotFound, nil},
		{"provider not found", "alice@example.com", "ghost", 1, nil, model.ErrProviderNotFound, nil},
		{"account not found", "alice@example.com", "chatgpt", 1, nil, model.ErrAccountNotFound, nil},
		{"session not found", "alice@example.com", "claude", 9999, nil, model.ErrSessionNotFound, nil},
		{"cross-account session", "alice@example.com", "claude", 2, nil, apperr.ErrNotFound, model.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.UpdateSessionPrompts(tc.email, tc.provider, tc.id, tc.queue)
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

// TestService_EndSession tables the full resolveSession-shaped set; this
// method has no branches of its own beyond that.
func TestService_EndSession(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		email        string
		provider     string
		id           uint64
		wantErr      error
		wantErrIsNot error
	}{
		{"success", "alice@example.com", "claude", 1, nil, nil},
		{"user not found", "ghost@example.com", "claude", 1, model.ErrUserNotFound, nil},
		{"provider not found", "alice@example.com", "ghost", 1, model.ErrProviderNotFound, nil},
		{"account not found", "alice@example.com", "chatgpt", 1, model.ErrAccountNotFound, nil},
		{"session not found", "alice@example.com", "claude", 9999, model.ErrSessionNotFound, nil},
		{"cross-account session", "alice@example.com", "claude", 2, apperr.ErrNotFound, model.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := seedService(t)
			err := svc.EndSession(tc.email, tc.provider, tc.id)
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

// TestService_StartSession keeps the success/behavioural cases as
// individual subtests, since each verifies a different aspect of the
// on-demand user/account creation (idempotency, name preservation,
// no-duplicate-account guarantee, project+queue passthrough, the
// project-not-granted check) — folding these into one table would need a
// per-row verification function, replacing readable assertions with
// indirection rather than removing duplication. The pure error-sentinel
// cases below share one action and one assertion shape, so those are tabled.
func TestService_StartSession(t *testing.T) {
	t.Parallel()

	t.Run("creates user and account on demand", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddProvider("claude", true, false))
		sess, err := svc.StartSession("alice@example.com", "Alice", "claude", "", "Topic", nil)
		must(t, err)
		if sess.ProjectName() != "" {
			t.Errorf("got project %q, want empty", sess.ProjectName())
		}
		if svc.User("alice@example.com") == nil {
			t.Error("expected user to be created")
		}
		if _, err := svc.Account("alice@example.com", "claude"); err != nil {
			t.Errorf("expected account to be created: %v", err)
		}
	})

	t.Run("creates an account for an existing user without changing its name", func(t *testing.T) {
		t.Parallel()
		svc := newService(t)
		must(t, svc.AddProvider("claude", true, false))
		must(t, svc.AddUser("alice@example.com", "Alice"))
		_, err := svc.StartSession("alice@example.com", "ignored name", "claude", "", "", nil)
		must(t, err)
		if got := svc.User("alice@example.com").Name(); got != "Alice" {
			t.Errorf("got name %q, want existing name preserved (Alice)", got)
		}
		if _, err := svc.Account("alice@example.com", "claude"); err != nil {
			t.Errorf("expected account to be created: %v", err)
		}
	})

	t.Run("reuses an existing account without creating a duplicate", func(t *testing.T) {
		t.Parallel()
		svc := seedService(t)
		_, err := svc.StartSession("alice@example.com", "Alice", "claude", "", "second", nil)
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
		queue := []model.Prompt{model.NewPrompt("continue", false)}
		sess, err := svc.StartSession("alice@example.com", "Alice", "claude", "Alpha", "Topic", queue)
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
		_, err := seedService(t).StartSession("alice@example.com", "Alice", "claude", "Orphan", "", nil)
		if err == nil {
			t.Fatal("got nil, want error")
		}
	})

	errorCases := []struct {
		name     string
		email    string
		userName string
		provider string
		project  string
		topic    string
		queue    []model.Prompt
		wantErr  error
	}{
		{"provider not found", "alice@example.com", "Alice", "ghost", "", "", nil, model.ErrProviderNotFound},
		{"new user creation fails on invalid email", "not-an-email", "Nobody", "claude", "", "", nil, apperr.ErrEmailInvalid},
		{"project not found", "alice@example.com", "Alice", "claude", "Ghost", "", nil, model.ErrProjectNotFound},
		{
			"invalid prompt in queue", "alice@example.com", "Alice", "claude", "", "",
			[]model.Prompt{model.NewPrompt("ok", false), model.NewPrompt("", false)},
			model.ErrTextRequired,
		},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := seedService(t).StartSession(tc.email, tc.userName, tc.provider, tc.project, tc.topic, tc.queue)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, tc.wantErr)
			}
		})
	}
}
