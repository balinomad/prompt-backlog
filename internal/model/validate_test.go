package model_test

import (
	"errors"
	"testing"

	"github.com/balinomad/prompt-backlog/internal/apperr"
	"github.com/balinomad/prompt-backlog/internal/model"
)

func TestValidateVendor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *model.Vendor
		want error
	}{
		{
			name: "nil",
			in:   nil,
			want: model.ErrVendorNil,
		},
		{
			name: "empty name",
			in:   model.NewVendor("", false, false),
			want: model.ErrNameRequired,
		},
		{
			name: "whitespace name normalized to empty",
			in:   model.NewVendor("   ", false, false),
			want: model.ErrNameRequired,
		},
		{
			name: "valid with no capabilities",
			in:   model.NewVendor("claude", false, false),
			want: nil,
		},
		{
			name: "valid with all capabilities",
			in:   model.NewVendor("claude", true, true),
			want: nil,
		},
		{
			name: "trimmed valid",
			in:   model.NewVendor("  claude  ", true, false),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := model.ValidateVendor(tt.in)
			if tt.want == nil {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, err)
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
			want: []error{apperr.ErrEmailInvalid},
		},
		{
			name: "invalid email missing local",
			in:   model.NewUser("@example.com", "Alice"),
			want: []error{apperr.ErrEmailInvalid},
		},
		{
			name: "invalid email missing domain",
			in:   model.NewUser("alice@", "Alice"),
			want: []error{apperr.ErrEmailInvalid},
		},
		{
			name: "invalid email double at",
			in:   model.NewUser("alice@@example.com", "Alice"),
			want: []error{apperr.ErrEmailInvalid},
		},
		{
			name: "invalid email missing dot in domain",
			in:   model.NewUser("alice@example", "Alice"),
			want: []error{apperr.ErrEmailInvalid},
		},
		{
			name: "normalized email",
			in:   model.NewUser("  ALICE@EXAMPLE.COM  ", "Alice"),
			want: nil,
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
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			for _, target := range tt.want {
				if !errors.Is(err, target) {
					t.Errorf("expected %v in %v", target, err)
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
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, err)
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
			name: "missing vendor",
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
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			for _, target := range tt.want {
				if !errors.Is(err, target) {
					t.Errorf("expected %v in %v", target, err)
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
			in:   model.NewPrompt("   "),
			want: model.ErrTextRequired,
		},
		{
			name: "empty text",
			in:   model.NewPrompt(""),
			want: model.ErrTextRequired,
		},
		{
			name: "valid",
			in:   model.NewPrompt("continue"),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := model.ValidatePrompt(tt.in)
			if tt.want == nil {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, err)
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
		name     string
		specific error
		category error
	}{
		{"vendor not found", model.ErrVendorNotFound, apperr.ErrNotFound},
		{"vendor exists", model.ErrVendorExists, apperr.ErrExists},
		{"vendor stale", model.ErrVendorStale, apperr.ErrStale},
		{"vendor nil", model.ErrVendorNil, apperr.ErrNil},

		{"user not found", model.ErrUserNotFound, apperr.ErrNotFound},
		{"user exists", model.ErrUserExists, apperr.ErrExists},
		{"user stale", model.ErrUserStale, apperr.ErrStale},
		{"user nil", model.ErrUserNil, apperr.ErrNil},

		{"project not found", model.ErrProjectNotFound, apperr.ErrNotFound},
		{"project exists", model.ErrProjectExists, apperr.ErrExists},
		{"project stale", model.ErrProjectStale, apperr.ErrStale},
		{"project nil", model.ErrProjectNil, apperr.ErrNil},
		{"projects unsupported", model.ErrProjectsUnsupported, apperr.ErrUnsupported},

		{"account not found", model.ErrAccountNotFound, apperr.ErrNotFound},
		{"account exists", model.ErrAccountExists, apperr.ErrExists},
		{"account stale", model.ErrAccountStale, apperr.ErrStale},
		{"account nil", model.ErrAccountNil, apperr.ErrNil},

		{"session not found", model.ErrSessionNotFound, apperr.ErrNotFound},
		{"session stale", model.ErrSessionStale, apperr.ErrStale},
		{"session nil", model.ErrSessionNil, apperr.ErrNil},

		{"note not found", model.ErrNoteNotFound, apperr.ErrNotFound},
		{"note stale", model.ErrNoteStale, apperr.ErrStale},
		{"note nil", model.ErrNoteNil, apperr.ErrNil},

		{"email required", model.ErrEmailRequired, apperr.ErrRequired},
		{"key required", model.ErrKeyRequired, apperr.ErrRequired},
		{"name required", model.ErrNameRequired, apperr.ErrRequired},
		{"text required", model.ErrTextRequired, apperr.ErrRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tt.specific, tt.category) {
				t.Errorf("%v does not wrap %v", tt.specific, tt.category)
			}
		})
	}
}

func TestValidateNote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *model.Note
		want error
	}{
		{
			name: "nil",
			in:   nil,
			want: model.ErrNoteNil,
		},
		{
			name: "empty text",
			in:   model.NewNote(1, "", 0, "", model.AccountKey{}, false),
			want: model.ErrTextRequired,
		},
		{
			name: "whitespace text",
			in:   model.NewNote(1, "   ", 0, "", model.AccountKey{}, false),
			want: model.ErrTextRequired,
		},
		{
			name: "account without key",
			in:   model.NewNote(1, "text", 0, "", model.AccountKey{}, true),
			want: model.ErrKeyRequired,
		},
		{
			name: "valid without account",
			in:   model.NewNote(1, "text", 1, "Alpha", model.AccountKey{}, false),
			want: nil,
		},
		{
			name: "valid with account",
			in:   model.NewNote(1, "text", 1, "Alpha", model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, true),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := model.ValidateNote(tt.in)
			if tt.want == nil {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, err)
			}
		})
	}
}
