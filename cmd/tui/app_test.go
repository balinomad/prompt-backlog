package main_test

import (
	"errors"
	"testing"

	tui "github.com/balinomad/llm-desk/cmd/tui"
	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/apperr"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"
	"github.com/balinomad/llm-desk/internal/store"

	"github.com/rivo/tview"
)

// ── mockStore ─────────────────────────────────────────────────────────────────

// mockStore is a minimal in-memory store.Store, for tests that need a
// working *service.Service without touching the filesystem.
var _ store.Store = (*mockStore)(nil)

type mockStore struct {
	reg     *model.Registry
	loadErr error
	saveErr error
	saved   *model.Registry // last value passed to Save, for assertions
}

func (s *mockStore) Load() (*model.Registry, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	if s.reg == nil {
		return model.NewRegistry(), nil
	}
	return s.reg, nil
}

func (s *mockStore) Save(reg *model.Registry) error {
	s.saved = reg
	return s.saveErr
}

// newTestService returns a *service.Service backed by a fresh in-memory
// registry, for tests that don't need to inspect the underlying store.
func newTestService(t *testing.T) *service.Service {
	t.Helper()
	svc, err := service.New(&mockStore{reg: model.NewRegistry()})
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	return svc
}

// ── New ───────────────────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("nil service", func(t *testing.T) {
		t.Parallel()
		app, err := tui.New(nil)
		if app != nil {
			t.Error("expected nil App on error")
		}
		if !errors.Is(err, apperr.ErrNil) {
			t.Errorf("got %v, want errors.Is(_, apperr.ErrNil)", err)
		}
	})

	t.Run("valid service", func(t *testing.T) {
		t.Parallel()
		svc := newTestService(t)
		app, err := tui.New(svc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if app == nil {
			t.Fatal("expected non-nil App")
		}
		if app.Service() != svc {
			t.Error("Service() does not return the svc passed to New")
		}
	})
}

// ── Mutate ────────────────────────────────────────────────────────────────────

func TestApp_Mutate_NilFn(t *testing.T) {
	t.Parallel()
	// Zero-value construction is legal from outside the package — no
	// unexported field is named, just the type. The nil-fn guard returns
	// before any field is touched, so this needs neither New nor a real svc.
	app := &tui.App{}
	gotErr := app.Mutate(nil)
	if !errors.Is(gotErr, apperr.ErrNil) {
		t.Errorf("got %v, want errors.Is(_, apperr.ErrNil)", gotErr)
	}
}

func TestApp_Mutate_Success(t *testing.T) {
	t.Parallel()
	ms := &mockStore{reg: model.NewRegistry()}
	svc, err := service.New(ms)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	app, err := tui.New(svc)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	called := false
	mutateErr := app.Mutate(func(s *service.Service) error {
		called = true
		return s.AddProvider("claude", true, false)
	})
	if mutateErr != nil {
		t.Fatalf("Mutate: %v", mutateErr)
	}
	if !called {
		t.Error("fn was not called")
	}
	if ms.saved == nil {
		t.Error("expected Save to be called with the mutated registry")
	}
	if app.Service().Provider("claude") == nil {
		t.Error("provider not visible through Service after Mutate")
	}
}

func TestApp_Mutate_FnError(t *testing.T) {
	t.Parallel()
	ms := &mockStore{reg: model.NewRegistry()}
	svc, err := service.New(ms)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	app, err := tui.New(svc)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	wantErr := errors.New("boom")
	gotErr := app.Mutate(func(*service.Service) error { return wantErr })
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("got %v, want %v", gotErr, wantErr)
	}
	if ms.saved != nil {
		t.Error("Save should not be called when fn returns an error")
	}
}

func TestApp_Mutate_SaveError(t *testing.T) {
	t.Parallel()
	saveErr := errors.New("disk full")
	ms := &mockStore{reg: model.NewRegistry(), saveErr: saveErr}
	svc, err := service.New(ms)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	app, err := tui.New(svc)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	gotErr := app.Mutate(func(*service.Service) error { return nil })
	if !errors.Is(gotErr, saveErr) {
		t.Errorf("got %v, want %v", gotErr, saveErr)
	}
	if !errors.Is(gotErr, views.ErrSaveFailed) {
		t.Errorf("got %v, want errors.Is(_, views.ErrSaveFailed)", gotErr)
	}
}

// ── ShowModal ─────────────────────────────────────────────────────────────────

func TestApp_ShowModal_NilPrimitive(t *testing.T) {
	t.Parallel()
	var nilFlex *tview.Flex // typed nil, deliberately not a nil interface

	cases := []struct {
		name string
		p    tview.Primitive
	}{
		{"literal nil interface", nil},
		{"typed nil pointer wrapped in interface", nilFlex},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Zero-value App: the nil check returns before a.tv/a.pages are
			// touched, so this needs neither New nor a built application.
			app := &tui.App{}
			gotErr := app.ShowModal(tc.p)
			if !errors.Is(gotErr, apperr.ErrNil) {
				t.Errorf("got %v, want errors.Is(_, apperr.ErrNil)", gotErr)
			}
		})
	}
}

func TestApp_ShowModal_Success(t *testing.T) {
	t.Parallel()
	app, err := tui.New(newTestService(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := app.ShowModal(tview.NewFlex()); err != nil {
		t.Errorf("ShowModal: unexpected error: %v", err)
	}
}
