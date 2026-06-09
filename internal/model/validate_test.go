package model_test

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/internal/model"
)

// ── FieldError ────────────────────────────────────────────────────────────────

func TestFieldError_Format(t *testing.T) {
	e := &model.FieldError{Field: "email", Message: "required"}
	if got, want := e.Error(), "email: required"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ── ValidateProvider ──────────────────────────────────────────────────────────

func TestValidateProvider_Valid(t *testing.T) {
	p := model.NewProvider("claude", true, false)
	if errs := model.ValidateProvider(p); len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateProvider_Nil(t *testing.T) {
	if errs := model.ValidateProvider(nil); len(errs) == 0 {
		t.Error("expected error for nil provider")
	}
}

func TestValidateProvider_EmptyName(t *testing.T) {
	p := model.NewProvider("", false, false)
	if errs := model.ValidateProvider(p); len(errs) == 0 {
		t.Error("expected error for empty name")
	}
}

func TestValidateProvider_WhitespaceName(t *testing.T) {
	p := model.NewProvider("   ", false, false)
	if errs := model.ValidateProvider(p); len(errs) == 0 {
		t.Error("expected error for whitespace-only name")
	}
}

// ── ValidateUser ──────────────────────────────────────────────────────────────

func TestValidateUser_Valid(t *testing.T) {
	u := model.NewUser("alice@example.com", "Alice")
	if errs := model.ValidateUser(u); len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateUser_Nil(t *testing.T) {
	if errs := model.ValidateUser(nil); len(errs) == 0 {
		t.Error("expected error for nil user")
	}
}

func TestValidateUser_EmptyEmail(t *testing.T) {
	u := model.NewUser("", "Alice")
	if errs := model.ValidateUser(u); len(errs) == 0 {
		t.Error("expected error for empty email")
	}
}

func TestValidateUser_InvalidEmail(t *testing.T) {
	cases := []string{
		"notanemail",
		"missing@nodot",
		"@nodomain",
		"nodotcom",
		"double@@example.com",
	}
	for _, email := range cases {
		t.Run(email, func(t *testing.T) {
			u := model.NewUser(email, "Alice")
			if errs := model.ValidateUser(u); len(errs) == 0 {
				t.Errorf("expected error for email %q", email)
			}
		})
	}
}

func TestValidateUser_EmptyName(t *testing.T) {
	u := model.NewUser("alice@example.com", "")
	if errs := model.ValidateUser(u); len(errs) == 0 {
		t.Error("expected error for empty name")
	}
}

func TestValidateUser_WhitespaceName(t *testing.T) {
	u := model.NewUser("alice@example.com", "   ")
	if errs := model.ValidateUser(u); len(errs) == 0 {
		t.Error("expected error for whitespace-only name")
	}
}

func TestValidateUser_BothInvalid(t *testing.T) {
	u := model.NewUser("", "")
	if errs := model.ValidateUser(u); len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errs))
	}
}

// ── ValidateProject ───────────────────────────────────────────────────────────

func TestValidateProject_Valid(t *testing.T) {
	p := model.NewProject("Alpha")
	if errs := model.ValidateProject(p); len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateProject_Nil(t *testing.T) {
	if errs := model.ValidateProject(nil); len(errs) == 0 {
		t.Error("expected error for nil project")
	}
}

func TestValidateProject_EmptyName(t *testing.T) {
	p := model.NewProject("")
	if errs := model.ValidateProject(p); len(errs) == 0 {
		t.Error("expected error for empty name")
	}
}

func TestValidateProject_WhitespaceName(t *testing.T) {
	p := model.NewProject("   ")
	if errs := model.ValidateProject(p); len(errs) == 0 {
		t.Error("expected error for whitespace-only name")
	}
}

// ── ValidateLastQuestion ──────────────────────────────────────────────────────

func TestValidateLastQuestion_AllCombinationsValid(t *testing.T) {
	proj := model.NewProject("Alpha")
	cases := []*model.LastQuestion{
		model.NewLastQuestion(nil),
		func() *model.LastQuestion {
			lq := model.NewLastQuestion(nil)
			lq.Topic = "Design discussion"
			return lq
		}(),
		func() *model.LastQuestion {
			lq := model.NewLastQuestion(nil)
			lq.MidAnswer = true
			return lq
		}(),
		func() *model.LastQuestion {
			lq := model.NewLastQuestion(nil)
			lq.MidAnswer = true
			lq.NextPrompt = "continue"
			return lq
		}(),
		func() *model.LastQuestion {
			lq := model.NewLastQuestion(proj)
			lq.Topic = "Project discussion"
			lq.MidAnswer = true
			lq.NextPrompt = "continue with the diagram"
			return lq
		}(),
	}
	for _, lq := range cases {
		if errs := model.ValidateLastQuestion(lq); len(errs) != 0 {
			t.Errorf("unexpected errors for %+v: %v", lq, errs)
		}
	}
}

// ── ValidateProjectAssignment ─────────────────────────────────────────────────

func TestValidateProjectAssignment_NilSession(t *testing.T) {
	// nil session — no session association to validate, always valid.
	if errs := model.ValidateProjectAssignment(nil); len(errs) != 0 {
		t.Errorf("expected no errors for nil session, got: %v", errs)
	}
}

func TestValidateProjectAssignment_ProviderSupportsProjects(t *testing.T) {
	u := model.NewUser("alice@example.com", "Alice")
	p := model.NewProvider("claude", true, false)
	s := model.NewSession(u, p)
	if errs := model.ValidateProjectAssignment(s); len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateProjectAssignment_ProviderDoesNotSupportProjects(t *testing.T) {
	u := model.NewUser("bob@example.com", "Bob")
	p := model.NewProvider("chatgpt", false, false)
	s := model.NewSession(u, p)
	if errs := model.ValidateProjectAssignment(s); len(errs) == 0 {
		t.Error("expected error: provider does not support projects")
	}
}

func TestValidateProjectAssignment_NilProvider(t *testing.T) {
	u := model.NewUser("alice@example.com", "Alice")
	s := model.NewSession(u, nil)
	if errs := model.ValidateProjectAssignment(s); len(errs) == 0 {
		t.Error("expected error for nil provider")
	}
}

// ── FormatErrors ──────────────────────────────────────────────────────────────

func TestFormatErrors_Nil(t *testing.T) {
	if s := model.FormatErrors(nil); s != "" {
		t.Errorf("expected empty string, got %q", s)
	}
}

func TestFormatErrors_Empty(t *testing.T) {
	if s := model.FormatErrors([]error{}); s != "" {
		t.Errorf("expected empty string, got %q", s)
	}
}

func TestFormatErrors_Single(t *testing.T) {
	errs := []error{&model.FieldError{Field: "name", Message: "required"}}
	if got := model.FormatErrors(errs); got != "name: required" {
		t.Errorf("got %q, want %q", got, "name: required")
	}
}

func TestFormatErrors_Multiple(t *testing.T) {
	errs := model.ValidateUser(model.NewUser("", ""))
	s := model.FormatErrors(errs)
	if !strings.Contains(s, "; ") {
		t.Errorf("expected semicolon separator in %q", s)
	}
	if strings.Count(s, "; ") != 1 {
		t.Errorf("expected exactly 1 separator for 2 errors in %q", s)
	}
}
