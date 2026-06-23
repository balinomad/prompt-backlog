package model_test

import (
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		providerName    string
		hasProjects     bool
		hasDeepResearch bool
		wantName        string
	}{
		{
			name:            "all capabilities",
			providerName:    " claude ",
			hasProjects:     true,
			hasDeepResearch: true,
			wantName:        "claude",
		},
		{
			name:            "no capabilities",
			providerName:    "basic",
			hasProjects:     false,
			hasDeepResearch: false,
			wantName:        "basic",
		},
		{
			name:         "whitespace only",
			providerName: "   ",
			wantName:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := model.NewProvider(tt.providerName, tt.hasProjects, tt.hasDeepResearch)
			if got := p.Name(); got != tt.wantName {
				t.Fatalf("Name() = %q, want %q", got, tt.wantName)
			}
			if got := p.HasProjects(); got != tt.hasProjects {
				t.Fatalf("HasProjects() = %v, want %v", got, tt.hasProjects)
			}
			if got := p.HasDeepResearch(); got != tt.hasDeepResearch {
				t.Fatalf("HasDeepResearch() = %v, want %v", got, tt.hasDeepResearch)
			}
		})
	}
}

func TestNewUser(t *testing.T) {
	t.Parallel()
	u := model.NewUser(
		" ALICE@example.com ",
		" Alice ",
	)
	if got, want := u.Email(), "alice@example.com"; got != want {
		t.Fatalf("Email() = %q, want %q", got, want)
	}
	if got, want := u.Name(), "Alice"; got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}
}

func TestNewProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		inName   string
		inDesc   string
		inInst   string
		wantName string
		wantDesc string
		wantInst string
	}{
		{
			name:     "normal values",
			inName:   "Alpha",
			inDesc:   "Description",
			inInst:   "Instructions",
			wantName: "Alpha",
			wantDesc: "Description",
			wantInst: "Instructions",
		},
		{
			name:     "whitespace trimming",
			inName:   " Alpha ",
			inDesc:   " Desc ",
			inInst:   " Inst ",
			wantName: "Alpha",
			wantDesc: "Desc",
			wantInst: "Inst",
		},
		{
			name:     "internal whitespace preserved",
			inName:   " Alpha Beta ",
			inDesc:   " Desc  1 ",
			inInst:   " Inst  2 ",
			wantName: "Alpha Beta",
			wantDesc: "Desc  1",
			wantInst: "Inst  2",
		},
		{
			name:     "empty strings",
			inName:   "",
			inDesc:   "",
			inInst:   "",
			wantName: "",
			wantDesc: "",
			wantInst: "",
		},
		{
			name:     "only whitespace",
			inName:   "   ",
			inDesc:   "\t\n",
			inInst:   "  ",
			wantName: "",
			wantDesc: "",
			wantInst: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := model.NewProject(tt.inName, tt.inDesc, tt.inInst)
			if got := p.Name(); got != tt.wantName {
				t.Fatalf("Name() = %q, want %q", got, tt.wantName)
			}
			if got := p.Description(); got != tt.wantDesc {
				t.Fatalf("Description() = %q, want %q", got, tt.wantDesc)
			}
			if got := p.Instructions(); got != tt.wantInst {
				t.Fatalf("Instructions() = %q, want %q", got, tt.wantInst)
			}
		})
	}
}

func TestAccountKeyString(t *testing.T) {
	t.Parallel()
	k := model.AccountKey{
		Email:    "alice@example.com",
		Provider: "claude",
	}
	if got, want := k.String(), "alice@example.com|claude"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestNewAccount(t *testing.T) {
	t.Parallel()
	acc := model.NewAccount(
		" ALICE@EXAMPLE.COM ",
		" claude ",
	)
	key := acc.Key()
	if got, want := key.Email, "alice@example.com"; got != want {
		t.Fatalf("Email = %q, want %q", got, want)
	}
	if got, want := key.Provider, "claude"; got != want {
		t.Fatalf("Provider = %q, want %q", got, want)
	}
	if got, want := acc.UserEmail(), "alice@example.com"; got != want {
		t.Fatalf("UserEmail() = %q, want %q", got, want)
	}
	if got, want := acc.ProviderName(), "claude"; got != want {
		t.Fatalf("ProviderName() = %q, want %q", got, want)
	}
	if !acc.PauseUntil().IsZero() {
		t.Fatalf("PauseUntil() expected zero time")
	}
}

func TestNewSession(t *testing.T) {
	t.Parallel()

	accKey := model.AccountKey{
		Email:    "alice@example.com",
		Provider: "claude",
	}
	queue := []model.Prompt{
		model.NewPrompt("continue", false),
		model.NewPrompt("step 2", true),
	}
	sess := model.NewSession(accKey, 42, " Project ", " Topic ", queue)

	if got := sess.ID(); got != 42 {
		t.Fatalf("ID() = %d, want 42", got)
	}
	if got := sess.AccountKey(); got != accKey {
		t.Fatalf("AccountKey() mismatch")
	}
	if got := sess.ProjectName(); got != "Project" {
		t.Fatalf("ProjectName() = %q", got)
	}
	if got := sess.Topic(); got != "Topic" {
		t.Fatalf("Topic() = %q", got)
	}

	q := sess.Queue()
	if len(q) != 2 {
		t.Fatalf("Queue length = %d, want 2", len(q))
	}

	q[0] = model.NewPrompt("mutated", false)

	again := sess.Queue()
	if again[0].Text() != "continue" {
		t.Fatal("Queue() did not return defensive copy")
	}
}

func TestNewPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		text          string
		spawn         bool
		wantText      string
		wantSpawnFlag bool
	}{
		{
			name:          "trimmed",
			text:          " continue ",
			spawn:         true,
			wantText:      "continue",
			wantSpawnFlag: true,
		},
		{
			name:          "empty",
			text:          "   ",
			spawn:         false,
			wantText:      "",
			wantSpawnFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := model.NewPrompt(tt.text, tt.spawn)
			if got := p.Text(); got != tt.wantText {
				t.Fatalf("Text() = %q, want %q", got, tt.wantText)
			}
			if got := p.SpawnsSession(); got != tt.wantSpawnFlag {
				t.Fatalf("SpawnsSession() = %v, want %v", got, tt.wantSpawnFlag)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid morning", "2026-05-18 09:00", false},
		{"valid midnight", "2026-01-01 00:00", false},
		{"empty", "", true},
		{"garbage", "not-a-date", true},
		{"wrong separator", "2026/05/18 09:00", true},
		{"reversed", "18-05-2026 09:00", true},
		{"missing time", "2026-05-18", true},
		{"missing date", "09:00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := model.ParseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTime(%q) error=%v wantErr=%v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()
		const original = "2026-05-20 14:30"
		parsed, err := model.ParseTime(original)
		if err != nil {
			t.Fatalf("ParseTime() error = %v", err)
		}
		if got := model.FormatTime(parsed); got != original {
			t.Fatalf("FormatTime() = %q, want %q", got, original)
		}
	})

	t.Run("utc preservation", func(t *testing.T) {
		t.Parallel()
		ts := time.Date(2026, time.May, 20, 14, 30, 0, 0, time.UTC)
		formatted := model.FormatTime(ts)
		parsed, err := model.ParseTime(formatted)
		if err != nil {
			t.Fatalf("ParseTime() error = %v", err)
		}
		if !parsed.Equal(ts) {
			t.Fatalf("parsed=%v want=%v", parsed, ts)
		}
	})
}

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		email string
		want  string
	}{
		{
			name:  "trim and lowercase",
			email: " ALICE@EXAMPLE.COM ",
			want:  "alice@example.com",
		},
		{
			name:  "already normalized",
			email: "alice@example.com",
			want:  "alice@example.com",
		},
		{
			name:  "empty",
			email: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := model.NewUser(tt.email, "Alice")
			if got := u.Email(); got != tt.want {
				t.Fatalf("Email() = %q, want %q", got, tt.want)
			}
		})
	}
}
