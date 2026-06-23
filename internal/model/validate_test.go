package model_test

import (
	"errors"
	"testing"

	"github.com/balinomad/llm-desk/internal/model"
)

func TestValidateProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *model.Provider
		want error
	}{
		{
			name: "nil",
			in:   nil,
			want: model.ErrProviderNil,
		},
		{
			name: "empty name",
			in:   model.NewProvider("", false, false),
			want: model.ErrNameRequired,
		},
		{
			name: "whitespace name normalized to empty",
			in:   model.NewProvider("   ", false, false),
			want: model.ErrNameRequired,
		},
		{
			name: "valid",
			in:   model.NewProvider("claude", true, false),
		},
		{
			name: "trimmed valid",
			in:   model.NewProvider("  claude  ", true, false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := model.ValidateProvider(tt.in)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

func TestValidateUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *model.User
		want []error
	}{
		{
			name: "nil",
			in:   nil,
			want: []error{model.ErrUserNil},
		},
		{
			name: "email required",
			in:   model.NewUser("", "Alice"),
			want: []error{model.ErrEmailRequired},
		},
		{
			name: "name required",
			in:   model.NewUser("alice@example.com", ""),
			want: []error{model.ErrNameRequired},
		},
		{
			name: "email and name required",
			in:   model.NewUser("", ""),
			want: []error{
				model.ErrEmailRequired,
				model.ErrNameRequired,
			},
		},
		{
			name: "invalid email missing at",
			in:   model.NewUser("alice.example.com", "Alice"),
			want: []error{model.ErrEmailInvalid},
		},
		{
			name: "invalid email missing local",
			in:   model.NewUser("@example.com", "Alice"),
			want: []error{model.ErrEmailInvalid},
		},
		{
			name: "invalid email missing domain",
			in:   model.NewUser("alice@", "Alice"),
			want: []error{model.ErrEmailInvalid},
		},
		{
			name: "invalid email double at",
			in:   model.NewUser("alice@@example.com", "Alice"),
			want: []error{model.ErrEmailInvalid},
		},
		{
			name: "invalid email missing dot in domain",
			in:   model.NewUser("alice@example", "Alice"),
			want: []error{model.ErrEmailInvalid},
		},
		{
			name: "normalized email",
			in:   model.NewUser("  ALICE@EXAMPLE.COM  ", "Alice"),
		},
		{
			name: "valid",
			in:   model.NewUser("alice@example.com", "Alice"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := model.ValidateUser(tt.in)
			if len(tt.want) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			for _, target := range tt.want {
				if !errors.Is(err, target) {
					t.Fatalf("expected %v in %v", target, err)
				}
			}
		})
	}
}

func TestValidateProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *model.Project
		want error
	}{
		{
			name: "nil",
			in:   nil,
			want: model.ErrProjectNil,
		},
		{
			name: "empty name",
			in:   model.NewProject("", "", ""),
			want: model.ErrNameRequired,
		},
		{
			name: "whitespace name normalized to empty",
			in:   model.NewProject("   ", "", ""),
			want: model.ErrNameRequired,
		},
		{
			name: "valid with empty fields",
			in:   model.NewProject("Alpha", "", ""),
		},
		{
			name: "trimmed valid",
			in:   model.NewProject("  Alpha  ", "", ""),
		},
		{
			name: "valid",
			in:   model.NewProject("Alpha", "desc", "inst"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := model.ValidateProject(tt.in)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

func TestValidateAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *model.Account
		want []error
	}{
		{
			name: "nil",
			in:   nil,
			want: []error{model.ErrAccountNil},
		},
		{
			name: "zero key",
			in:   &model.Account{},
			want: []error{model.ErrKeyRequired},
		},
		{
			name: "missing email",
			in:   model.NewAccount("", "claude"),
			want: []error{model.ErrEmailRequired},
		},
		{
			name: "missing provider",
			in:   model.NewAccount("alice@example.com", ""),
			want: []error{model.ErrNameRequired},
		},
		{
			name: "valid",
			in: model.NewAccount(
				"alice@example.com",
				"claude",
			),
		},
		{
			name: "normalized values",
			in: model.NewAccount(
				"  ALICE@EXAMPLE.COM ",
				"  claude  ",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := model.ValidateAccount(tt.in)
			if len(tt.want) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			for _, target := range tt.want {
				if !errors.Is(err, target) {
					t.Fatalf("expected %v in %v", target, err)
				}
			}
		})
	}
}

func TestValidatePrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   model.Prompt
		want error
	}{
		{
			name: "empty",
			in:   model.Prompt{},
			want: model.ErrTextRequired,
		},
		{
			name: "whitespace normalized to empty",
			in:   model.NewPrompt("   ", false),
			want: model.ErrTextRequired,
		},
		{
			name: "empty text",
			in:   model.NewPrompt("", true),
			want: model.ErrTextRequired,
		},
		{
			name: "valid no spawn",
			in:   model.NewPrompt("continue", false),
			want: nil,
		},
		{
			name: "valid spawn session",
			in:   model.NewPrompt("continue", true),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := model.ValidatePrompt(tt.in)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

// TestErrorCategories confirms every entity-specific sentinel wraps
// its shared category, since registry.go and service.go callers dispatch on
// the category (e.g. errors.Is(err, model.ErrNotFound)) without knowing the
// concrete entity type.
func TestErrorCategories(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		target error
	}{
		{"provider not found", model.ErrProviderNotFound, model.ErrNotFound},
		{"provider exists", model.ErrProviderExists, model.ErrExists},
		{"provider stale", model.ErrProviderStale, model.ErrStale},
		{"provider nil", model.ErrProviderNil, model.ErrNil},

		{"user not found", model.ErrUserNotFound, model.ErrNotFound},
		{"user exists", model.ErrUserExists, model.ErrExists},
		{"user stale", model.ErrUserStale, model.ErrStale},
		{"user nil", model.ErrUserNil, model.ErrNil},

		{"project not found", model.ErrProjectNotFound, model.ErrNotFound},
		{"project exists", model.ErrProjectExists, model.ErrExists},
		{"project stale", model.ErrProjectStale, model.ErrStale},
		{"project nil", model.ErrProjectNil, model.ErrNil},
		{"projects unsupported", model.ErrProjectsUnsupported, model.ErrUnsupported},

		{"account not found", model.ErrAccountNotFound, model.ErrNotFound},
		{"account exists", model.ErrAccountExists, model.ErrExists},
		{"account stale", model.ErrAccountStale, model.ErrStale},
		{"account nil", model.ErrAccountNil, model.ErrNil},

		{"session not found", model.ErrSessionNotFound, model.ErrNotFound},
		{"session stale", model.ErrSessionStale, model.ErrStale},
		{"session nil", model.ErrSessionNil, model.ErrNil},

		{"email required", model.ErrEmailRequired, model.ErrRequired},
		{"key required", model.ErrKeyRequired, model.ErrRequired},
		{"name required", model.ErrNameRequired, model.ErrRequired},
		{"text required", model.ErrTextRequired, model.ErrRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tt.err, tt.target) {
				t.Fatalf("%v should match %v", tt.err, tt.target)
			}
		})
	}
}
