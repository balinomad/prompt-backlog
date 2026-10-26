package model_test

import (
	"slices"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
)

// ── Time helpers ──────────────────────────────────────────────────────────────

func TestParseTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid morning", "2026-05-18 09:00", false},
		{"valid midnight", "2026-01-01 00:00", false},
		{"valid end of year", "2026-12-31 23:59", false},
		{"empty", "", true},
		{"garbage", "not-a-date", true},
		{"wrong date separator", "2026/05/18 09:00", true},
		{"reversed layout", "18-05-2026 09:00", true},
		{"missing time", "2026-05-18", true},
		{"missing date", "09:00", true},
		{"trailing seconds not in layout", "2026-05-18 09:00:00", true},
		{"month not zero-padded", "2026-5-18 09:00", true},
		{"day not zero-padded", "2026-05-8 09:00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := model.ParseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got.Location() != time.UTC {
				t.Errorf("ParseTime(%q) location = %v, want UTC", tt.input, got.Location())
			}
		})
	}
}

// TestParseTime_InterpretsLocal verifies s is parsed as the operator's
// local time zone, not UTC: matching the instant time.Date produces for
// the same wall-clock fields under time.Local is the property ParseTime
// exists to get right (see its doc comment).
func TestParseTime_InterpretsLocal(t *testing.T) {
	t.Parallel()
	got, err := model.ParseTime("2026-05-20 14:30")
	if err != nil {
		t.Fatalf("ParseTime: %v", err)
	}
	want := time.Date(2026, 5, 20, 14, 30, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v (2026-05-20 14:30 in local time)", got, want)
	}
}

func TestFormatTime(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()
		const original = "2026-05-20 14:30"
		parsed, err := model.ParseTime(original)
		if err != nil {
			t.Fatalf("ParseTime: %v", err)
		}
		if got := model.FormatTime(parsed); got != original {
			t.Errorf("round-trip: got %q, want %q", got, original)
		}
	})

	t.Run("explicit UTC input round-trips", func(t *testing.T) {
		t.Parallel()
		// now is explicitly UTC-tagged rather than the Local tag time.Now()
		// normally carries, to verify FormatTime's own Local conversion
		// handles an arbitrarily-zoned input correctly, not just an
		// already-local one; the "round trip" subtest above covers that case.
		now := time.Now().UTC().Truncate(time.Minute)
		formatted := model.FormatTime(now)
		parsed, err := model.ParseTime(formatted)
		if err != nil {
			t.Fatalf("ParseTime failed on FormatTime output %q: %v", formatted, err)
		}
		if !parsed.Equal(now) {
			t.Errorf("time not preserved: got %v, want %v", parsed, now)
		}
	})
}

// ── Normalization helpers ─────────────────────────────────────────────────────

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already normalized",
			input: "alice@example.com",
			want:  "alice@example.com",
		},
		{
			name:  "whitespace with uppercase",
			input: "  ALICE@EXAMPLE.COM  ",
			want:  "alice@example.com",
		},
		{
			name:  "tabs and newlines",
			input: "\talice@example.com\n",
			want:  "alice@example.com",
		},
		{
			name:  "mixed case and whitespace",
			input: "  Alice@Example.Com  ",
			want:  "alice@example.com",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := model.NormalizeEmail(tt.input); got != tt.want {
				t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already normalized",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "leading and trailing whitespace",
			input: "  hello  ",
			want:  "hello",
		},
		{
			name:  "internal whitespace preserved",
			input: "  hello   world  ",
			want:  "hello   world",
		},
		{
			name:  "tabs and newlines",
			input: "\thello\n",
			want:  "hello",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "  \t\n ",
			want:  "",
		},
		{
			name:  "case preserved",
			input: "  HeLLo  ",
			want:  "HeLLo",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := model.NormalizeText(tt.input); got != tt.want {
				t.Errorf("NormalizeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewVendor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		inName          string
		hasProjects     bool
		hasDeepResearch bool
		wantName        string
	}{
		{
			name:            "all capabilities",
			inName:          " claude ",
			hasProjects:     true,
			hasDeepResearch: true,
			wantName:        "claude",
		},
		{
			name:            "no capabilities",
			inName:          "basic",
			hasProjects:     false,
			hasDeepResearch: false,
			wantName:        "basic",
		},
		{
			name:            "whitespace only",
			inName:          "   ",
			hasProjects:     false,
			hasDeepResearch: false,
			wantName:        "",
		},
		{
			name:            "empty name",
			inName:          "",
			hasProjects:     false,
			hasDeepResearch: false,
			wantName:        "",
		},
		{
			name:            "projects only",
			inName:          "mid",
			hasProjects:     true,
			hasDeepResearch: false,
			wantName:        "mid",
		},
		{
			name:            "deep research only",
			inName:          "mid",
			hasProjects:     false,
			hasDeepResearch: true,
			wantName:        "mid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := model.NewVendor(tt.inName, tt.hasProjects, tt.hasDeepResearch)
			if got := p.Name(); got != tt.wantName {
				t.Errorf("Name() = %q, want %q", got, tt.wantName)
			}
			if got := p.HasProjects(); got != tt.hasProjects {
				t.Errorf("HasProjects() = %v, want %v", got, tt.hasProjects)
			}
			if got := p.HasDeepResearch(); got != tt.hasDeepResearch {
				t.Errorf("HasDeepResearch() = %v, want %v", got, tt.hasDeepResearch)
			}
		})
	}
}

// ── User ──────────────────────────────────────────────────────────────────────

func TestNewUser(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		inEmail   string
		inName    string
		wantEmail string
		wantName  string
	}{
		{
			name:      "trims and lowercases email, trims name",
			inEmail:   " ALICE@Example.com ",
			inName:    " Alice ",
			wantEmail: "alice@example.com",
			wantName:  "Alice",
		},
		{
			name:      "already normalized",
			inEmail:   "bob@example.com",
			inName:    "Bob",
			wantEmail: "bob@example.com",
			wantName:  "Bob",
		},
		{
			name:      "empty values",
			inEmail:   "",
			inName:    "",
			wantEmail: "",
			wantName:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := model.NewUser(tt.inEmail, tt.inName)
			if got := u.Email(); got != tt.wantEmail {
				t.Errorf("Email() = %q, want %q", got, tt.wantEmail)
			}
			if got := u.Name(); got != tt.wantName {
				t.Errorf("Name() = %q, want %q", got, tt.wantName)
			}
		})
	}
}

// ── Project ───────────────────────────────────────────────────────────────────

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
				t.Errorf("Name() = %q, want %q", got, tt.wantName)
			}
			if got := p.Description(); got != tt.wantDesc {
				t.Errorf("Description() = %q, want %q", got, tt.wantDesc)
			}
			if got := p.Instructions(); got != tt.wantInst {
				t.Errorf("Instructions() = %q, want %q", got, tt.wantInst)
			}
		})
	}
}

// ── AccountKey ────────────────────────────────────────────────────────────────

func TestAccountKey_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  model.AccountKey
		want string
	}{
		{"normal", model.AccountKey{Email: "alice@example.com", Vendor: "claude"}, "alice@example.com|claude"},
		{"zero value", model.AccountKey{}, "|"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.key.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAccountKey_Equality(t *testing.T) {
	t.Parallel()
	a := model.AccountKey{Email: "alice@example.com", Vendor: "claude"}
	b := model.AccountKey{Email: "alice@example.com", Vendor: "claude"}
	c := model.AccountKey{Email: "bob@example.com", Vendor: "claude"}

	if a != b {
		t.Error("identical field values should compare equal")
	}
	if a == c {
		t.Error("differing emails should not compare equal")
	}
}

// ── Account ───────────────────────────────────────────────────────────────────

func TestNewAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		inEmail   string
		inProv    string
		wantKey   model.AccountKey
		wantEmail string
		wantProv  string
	}{
		{
			name:      "trims and lowercases email, trims vendor",
			inEmail:   " ALICE@Example.com ",
			inProv:    " claude ",
			wantKey:   model.AccountKey{Email: "alice@example.com", Vendor: "claude"},
			wantEmail: "alice@example.com",
			wantProv:  "claude",
		},
		{
			name:      "already normalized",
			inEmail:   "bob@example.com",
			inProv:    "chatgpt",
			wantKey:   model.AccountKey{Email: "bob@example.com", Vendor: "chatgpt"},
			wantEmail: "bob@example.com",
			wantProv:  "chatgpt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := model.NewAccount(tt.inEmail, tt.inProv)
			if got := a.Key(); got != tt.wantKey {
				t.Errorf("Key() = %+v, want %+v", got, tt.wantKey)
			}
			if got := a.UserEmail(); got != tt.wantEmail {
				t.Errorf("UserEmail() = %q, want %q", got, tt.wantEmail)
			}
			if got := a.VendorName(); got != tt.wantProv {
				t.Errorf("VendorName() = %q, want %q", got, tt.wantProv)
			}
			if !a.PauseUntil().IsZero() {
				t.Errorf("PauseUntil() = %v, want zero value", a.PauseUntil())
			}
		})
	}
}

// ── Session ───────────────────────────────────────────────────────────────────

func TestNewSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         model.AccountKey
		id          uint64
		inProject   string
		inTopic     string
		inQueue     []model.Prompt
		wantProject string
		wantTopic   string
		wantQueue   []string
	}{
		{
			name:        "basic fields",
			key:         model.AccountKey{Email: "alice@example.com", Vendor: "claude"},
			id:          7,
			inProject:   " Alpha ",
			inTopic:     " Architecture review ",
			inQueue:     []model.Prompt{model.NewPrompt("continue", false)},
			wantProject: "Alpha",
			wantTopic:   "Architecture review",
			wantQueue:   []string{"continue"},
		},
		{
			name:        "no project empty topic nil queue",
			key:         model.AccountKey{Email: "alice@example.com", Vendor: "claude"},
			id:          1,
			wantProject: "",
			wantTopic:   "",
			wantQueue:   nil,
		},
		{
			name: "queue order preserved",
			key:  model.AccountKey{Email: "alice@example.com", Vendor: "claude"},
			id:   1,
			inQueue: []model.Prompt{
				model.NewPrompt("first", false),
				model.NewPrompt("second", true),
				model.NewPrompt("third", false),
			},
			wantProject: "",
			wantTopic:   "",
			wantQueue:   []string{"first", "second", "third"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := model.NewSession(tt.key, tt.id, tt.inProject, tt.inTopic, tt.inQueue)

			if got := s.ID(); got != tt.id {
				t.Errorf("ID() = %d, want %d", got, tt.id)
			}
			if got := s.AccountKey(); got != tt.key {
				t.Errorf("AccountKey() = %+v, want %+v", got, tt.key)
			}
			if got := s.ProjectName(); got != tt.wantProject {
				t.Errorf("ProjectName() = %q, want %q", got, tt.wantProject)
			}
			if got := s.Topic(); got != tt.wantTopic {
				t.Errorf("Topic() = %q, want %q", got, tt.wantTopic)
			}

			gotQ := s.Queue()
			if len(gotQ) != len(tt.wantQueue) {
				t.Fatalf("Queue() length = %d, want %d", len(gotQ), len(tt.wantQueue))
			}
			for i, w := range tt.wantQueue {
				if gotQ[i].Text() != w {
					t.Errorf("Queue()[%d].Text() = %q, want %q", i, gotQ[i].Text(), w)
				}
			}
		})
	}
}

func TestNewSession_Behavioral(t *testing.T) {
	t.Parallel()

	t.Run("input queue is defensively cloned", func(t *testing.T) {
		t.Parallel()
		key := model.AccountKey{Email: "alice@example.com", Vendor: "claude"}
		queue := []model.Prompt{model.NewPrompt("first", false)}
		s := model.NewSession(key, 1, "", "", queue)

		queue[0] = model.NewPrompt("mutated", true)

		got := s.Queue()
		if len(got) != 1 || got[0].Text() != "first" {
			t.Errorf("Queue() = %+v, want unaffected by post-construction mutation", got)
		}
	})

	t.Run("returned queue is a defensive copy", func(t *testing.T) {
		t.Parallel()
		key := model.AccountKey{Email: "alice@example.com", Vendor: "claude"}
		queue := []model.Prompt{model.NewPrompt("first", false)}
		s := model.NewSession(key, 1, "", "", queue)

		got := s.Queue()
		got[0] = model.NewPrompt("mutated", true)

		again := s.Queue()
		if len(again) != 1 || again[0].Text() != "first" {
			t.Errorf("Queue() = %+v, want unaffected by mutation of returned slice", again)
		}
	})
}

// ── Prompt ────────────────────────────────────────────────────────────────────

func TestNewPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		spawn     bool
		wantText  string
		wantSpawn bool
	}{
		{
			name:      "trimmed",
			text:      " continue ",
			spawn:     true,
			wantText:  "continue",
			wantSpawn: true,
		},
		{
			name:      "empty",
			text:      "   ",
			spawn:     false,
			wantText:  "",
			wantSpawn: false,
		},
		{
			name:      "internal whitespace preserved",
			text:      "  Proceed   to step 2  ",
			spawn:     false,
			wantText:  "Proceed   to step 2",
			wantSpawn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := model.NewPrompt(tt.text, tt.spawn)
			if got := p.Text(); got != tt.wantText {
				t.Errorf("Text() = %q, want %q", got, tt.wantText)
			}
			if got := p.SpawnsSession(); got != tt.wantSpawn {
				t.Errorf("SpawnsSession() = %v, want %v", got, tt.wantSpawn)
			}
		})
	}
}

func TestPrompt_Comparable(t *testing.T) {
	t.Parallel()
	// Registry.UpdateSessionPrompts relies on Prompt being comparable via ==
	// (through slices.Equal) to short-circuit no-op updates.
	a := model.NewPrompt("continue", false)
	b := model.NewPrompt("continue", false)
	c := model.NewPrompt("continue", true)

	if a != b {
		t.Error("identical prompts should compare equal")
	}
	if a == c {
		t.Error("prompts differing only in SpawnsSession should not compare equal")
	}
	if !slices.Equal([]model.Prompt{a}, []model.Prompt{b}) {
		t.Error("slices.Equal should treat identical prompt slices as equal")
	}
}
