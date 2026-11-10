package apperr_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/balinomad/prompt-backlog/internal/apperr"
)

func TestInvariantf(t *testing.T) {
	t.Parallel()

	err := apperr.Invariantf("bad value %d", 7)
	if !errors.Is(err, apperr.ErrInvariant) {
		t.Fatalf("errors.Is(err, ErrInvariant) = false")
	}
	if !apperr.IsInvariant(err) {
		t.Fatal("IsInvariant(err) = false")
	}

	var invariant *apperr.InvariantError
	if !errors.As(err, &invariant) {
		t.Fatalf("errors.As(err, *InvariantError) = false")
	}
	if got := invariant.Error(); !strings.Contains(got, "bad value 7") {
		t.Errorf("Error() = %q, want the formatted message", got)
	}
	if len(invariant.Stack()) == 0 {
		t.Error("Stack() is empty")
	}
}

func TestIsInvariant(t *testing.T) {
	t.Parallel()

	if apperr.IsInvariant(nil) {
		t.Error("IsInvariant(nil) = true")
	}
	if apperr.IsInvariant(errors.New("ordinary")) {
		t.Error("IsInvariant(ordinary) = true")
	}
}
