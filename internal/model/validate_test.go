package model_test

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/internal/model"
)

// ── FieldError ────────────────────────────────────────────────────────────────

func TestFieldError_Format(t *testing.T) {
	t.Parallel()
	e := &model.FieldError{Field: "email", Message: "required"}
	if got, want := e.Error(), "email: required"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ── ValidateProvider ──────────────────────────────────────────────────────────

func TestValidateProvider(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		provider *model.Provider
		wantErrs bool
	}{
		{"valid", model.NewProvider("claude", true, false), false},
		{"nil", nil, true},
		{"empty name", model.NewProvider("", false, false), true},
		{"whitespace name", model.NewProvider("   ", false, false), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := model.ValidateProvider(tc.provider)
			if tc.wantErrs && len(errs) == 0 {
				t.Error("expected errors, got none")
			}
			if !tc.wantErrs && len(errs) != 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}

// ── ValidateUser ──────────────────────────────────────────────────────────────

func TestValidateUser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		user     *model.User
		wantErrs bool
	}{
		{"valid", model.NewUser("alice@example.com", "Alice"), false},
		{"nil", nil, true},
		{"empty email", model.NewUser("", "Alice"), true},
		{"whitespace email", model.NewUser("   ", "Alice"), true},
		{"empty name", model.NewUser("alice@example.com", ""), true},
		{"whitespace name", model.NewUser("alice@example.com", "   "), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := model.ValidateUser(tc.user)
			if tc.wantErrs && len(errs) == 0 {
				t.Error("expected errors, got none")
			}
			if !tc.wantErrs && len(errs) != 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}

func TestValidateUser_InvalidEmail(t *testing.T) {
	t.Parallel()
	cases := []string{
		"notanemail",
		"missing@nodot",
		"@nodomain",
		"nodotcom",
		"double@@example.com",
	}
	for _, email := range cases {
		t.Run(email, func(t *testing.T) {
			t.Parallel()
			u := model.NewUser(email, "Alice")
			if errs := model.ValidateUser(u); len(errs) == 0 {
				t.Errorf("expected error for email %q", email)
			}
		})
	}
}

func TestValidateUser_BothInvalid(t *testing.T) {
	t.Parallel()
	u := model.NewUser("", "")
	if errs := model.ValidateUser(u); len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errs))
	}
}

// ── ValidateProject ───────────────────────────────────────────────────────────

func TestValidateProject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		project  *model.Project
		wantErrs bool
	}{
		{"valid", model.NewProject("Alpha"), false},
		{"nil", nil, true},
		{"empty name", model.NewProject(""), true},
		{"whitespace name", model.NewProject("   "), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := model.ValidateProject(tc.project)
			if tc.wantErrs && len(errs) == 0 {
				t.Error("expected errors, got none")
			}
			if !tc.wantErrs && len(errs) != 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}

// ── ValidateLastQuestion ──────────────────────────────────────────────────────

func TestValidateLastQuestion_AllCombinationsValid(t *testing.T) {
	t.Parallel()
	proj := model.NewProject("Alpha")
	cases := []struct {
		name string
		lq   *model.LastQuestion
	}{
		{"empty", model.NewLastQuestion(nil)},
		{"topic only", func() *model.LastQuestion {
			lq := model.NewLastQuestion(nil)
			lq.Topic = "Design discussion"
			return lq
		}()},
		{"mid answer", func() *model.LastQuestion {
			lq := model.NewLastQuestion(nil)
			lq.MidAnswer = true
			return lq
		}()},
		{"mid answer with next prompt", func() *model.LastQuestion {
			lq := model.NewLastQuestion(nil)
			lq.MidAnswer = true
			lq.NextPrompt = "continue"
			return lq
		}()},
		{"full with project", func() *model.LastQuestion {
			lq := model.NewLastQuestion(proj)
			lq.Topic = "Project discussion"
			lq.MidAnswer = true
			lq.NextPrompt = "continue with the diagram"
			return lq
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if errs := model.ValidateLastQuestion(tc.lq); len(errs) != 0 {
				t.Errorf("unexpected errors for %q: %v", tc.name, errs)
			}
		})
	}
}

// ── ValidateProjectAssignment ─────────────────────────────────────────────────

func TestValidateProjectAssignment(t *testing.T) {
	t.Parallel()
	alice := model.NewUser("alice@example.com", "Alice")
	bob := model.NewUser("bob@example.com", "Bob")
	cases := []struct {
		name     string
		session  *model.Session
		wantErrs bool
	}{
		{"nil session", nil, false},
		{"provider supports projects",
			model.NewSession(alice, model.NewProvider("claude", true, false)), false},
		{"provider does not support projects",
			model.NewSession(bob, model.NewProvider("chatgpt", false, false)), true},
		{"nil provider",
			model.NewSession(alice, nil), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := model.ValidateProjectAssignment(tc.session)
			if tc.wantErrs && len(errs) == 0 {
				t.Error("expected errors, got none")
			}
			if !tc.wantErrs && len(errs) != 0 {
				t.Errorf("expected no errors, got: %v", errs)
			}
		})
	}
}

// ── FormatErrors ──────────────────────────────────────────────────────────────

func TestFormatErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		errs []error
		want string
	}{
		{"nil input", nil, ""},
		{"empty slice", []error{}, ""},
		{"single error", []error{&model.FieldError{Field: "name", Message: "required"}}, "name: required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := model.FormatErrors(tc.errs); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatErrors_Multiple(t *testing.T) {
	t.Parallel()
	errs := model.ValidateUser(model.NewUser("", ""))
	s := model.FormatErrors(errs)
	if !strings.Contains(s, "; ") {
		t.Errorf("expected semicolon separator in %q", s)
	}
	if strings.Count(s, "; ") != 1 {
		t.Errorf("expected exactly 1 separator for 2 errors in %q", s)
	}
}
