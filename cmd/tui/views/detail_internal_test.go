package views

import (
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/internal/model"
)

func TestSessionItemText_PendingLabelWidth(t *testing.T) {
	t.Parallel()
	sess := model.NewSession(model.AccountKey{Email: "a@example.com", Vendor: "claude"}, 1, "", "Topic", nil)

	narrow := sessionItemText(sess, 50)
	if !strings.Contains(narrow, "pending: 0") || strings.Contains(narrow, "pending messages: 0") {
		t.Errorf("narrow row = %q, want short pending label", narrow)
	}

	wide := sessionItemText(sess, 70)
	if !strings.Contains(wide, "pending messages: 0") {
		t.Errorf("wide row = %q, want long pending label", wide)
	}
}

func TestWriteInfoRows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rows []infoRow
		want string
	}{
		{
			name: "account labels",
			rows: []infoRow{
				{label: "User", value: "alice"},
				{label: "Vendor", value: "claude"},
				{label: "Projects", value: "Alpha, Beta"},
			},
			want: "[::b]User[-::-]     alice\n" +
				"[::b]Vendor[-::-]   claude\n" +
				"[::b]Projects[-::-] Alpha, Beta\n",
		},
		{
			name: "vendor labels",
			rows: []infoRow{
				{label: "Name", value: "claude"},
				{label: "Projects", value: "yes"},
				{label: "Deep research", value: "yes"},
			},
			want: "[::b]Name[-::-]          claude\n" +
				"[::b]Projects[-::-]      yes\n" +
				"[::b]Deep research[-::-] yes\n",
		},
		{
			name: "project labels",
			rows: []infoRow{
				{label: "Name", value: "Alpha"},
				{label: "Description", value: "Backend rewrite"},
				{label: "Instructions", value: "Use Go interfaces"},
			},
			want: "[::b]Name[-::-]         Alpha\n" +
				"[::b]Description[-::-]  Backend rewrite\n" +
				"[::b]Instructions[-::-] Use Go interfaces\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var sb strings.Builder
			writeInfoRows(&sb, tc.rows)
			if got := sb.String(); got != tc.want {
				t.Errorf("writeInfoRows() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewInfoViewTitle(t *testing.T) {
	t.Parallel()
	info := newInfoView("Account Details")
	if got := info.GetTitle(); got != " Account Details " {
		t.Errorf("GetTitle() = %q, want %q", got, " Account Details ")
	}
}

func TestInfoTextHeight(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		text  string
		width int
		want  int
	}{
		{"empty", "", 40, 3},
		{"one line", "hello", 40, 3},
		{"trailing newline", "hello\n", 40, 3},
		{"wrapped", strings.Repeat("x", 20), 12, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := infoTextHeight(tc.text, tc.width); got != tc.want {
				t.Errorf("infoTextHeight(%q, %d) = %d, want %d", tc.text, tc.width, got, tc.want)
			}
		})
	}
}

func TestFitInfoHeight(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		required  int
		available int
		minOther  int
		want      int
	}{
		{"fits", 5, 24, 3, 5},
		{"clamped", 30, 24, 3, 21},
		{"no room", 5, 3, 3, 0},
		{"zero available", 5, 0, 3, 0},
		{"negative available", 5, -1, 3, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := fitInfoHeight(tc.required, tc.available, tc.minOther); got != tc.want {
				t.Errorf("fitInfoHeight(%d, %d, %d) = %d, want %d", tc.required, tc.available, tc.minOther, got, tc.want)
			}
		})
	}
}
