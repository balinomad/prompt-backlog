package views_test

import (
	"strings"
	"testing"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"

	"github.com/rivo/tview"
)

func TestFormatColumns_PadsBlankCells(t *testing.T) {
	t.Parallel()
	lines, width := views.FormatColumns([][]string{
		{"chatgpt", "Bob", "", ""},
		{"claude", "Alice", "09:00", "Alpha"},
	})

	want := []string{
		" chatgpt │ Bob   │       │       ",
		" claude  │ Alice │ 09:00 │ Alpha ",
	}
	if len(lines) != len(want) {
		t.Fatalf("line count: got %d, want %d", len(lines), len(want))
	}
	for i, expected := range want {
		if lines[i] != expected {
			t.Errorf("line %d = %q, want %q", i, lines[i], expected)
		}
	}
	if wantWidth := tview.TaggedStringWidth(" chatgpt │ Bob   │       │       "); width != wantWidth {
		t.Errorf("width = %d, want %d", width, wantWidth)
	}
}

func TestTruncateWidth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{"no limit", "abcdef", 0, "abcdef"},
		{"fits", "abc", 3, "abc"},
		{"ascii truncation", "abcdef", 4, "abc…"},
		{"wide rune boundary", "a界b", 3, "a…"},
		{"brackets are literal", "Alice[red]", 8, "Alice[r…"},
		{"single cell", "abcdef", 1, "…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := views.TruncateWidth(tc.input, tc.maxWidth); got != tc.want {
				t.Errorf("views.TruncateWidth(%q, %d) = %q, want %q", tc.input, tc.maxWidth, got, tc.want)
			}
		})
	}
}

func TestPadWidth_IgnoresStyleTags(t *testing.T) {
	t.Parallel()
	if got, want := views.PadWidth("[red]x[-]", 3), "[red]x[-]  "; got != want {
		t.Errorf("PadWidth = %q, want %q", got, want)
	}
}

func TestCountTitle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		base  string
		index int
		total int
		want  string
	}{
		{
			name:  "first of three",
			base:  " Vendors ",
			index: 0,
			total: 3,
			want:  " Vendors 1/3 ",
		},
		{
			name:  "second of three",
			base:  " Accounts ",
			index: 1,
			total: 3,
			want:  " Accounts 2/3 ",
		},
		{
			name:  "index clamped to total",
			base:  " Vendors ",
			index: 5,
			total: 3,
			want:  " Vendors 3/3 ",
		},
		{
			name:  "empty list",
			base:  " Vendors ",
			index: 0,
			total: 0,
			want:  " Vendors 0/0 ",
		},
		{
			name:  "dynamic base",
			base:  " Messages: session #1 ",
			index: 1,
			total: 3,
			want:  " Messages: session #1 2/3 ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := views.CountTitle(tc.base, tc.index, tc.total); got != tc.want {
				t.Errorf("views.CountTitle(%q, %d, %d) = %q, want %q", tc.base, tc.index, tc.total, got, tc.want)
			}
		})
	}
}

func TestSetListTitle(t *testing.T) {
	t.Parallel()
	list := tview.NewList()
	views.SetListTitle(list, " Accounts ", 1, 3)
	if got, want := list.GetTitle(), " Accounts 2/3 "; got != want {
		t.Errorf("GetTitle() = %q, want %q", got, want)
	}
}

func TestWriteInfoRows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rows []views.InfoRow
		want string
	}{
		{
			name: "account labels",
			rows: []views.InfoRow{
				{Label: "User", Value: "alice"},
				{Label: "Vendor", Value: "claude"},
				{Label: "Projects", Value: "Alpha, Beta"},
			},
			want: "[::b]User[-::-]     alice\n" +
				"[::b]Vendor[-::-]   claude\n" +
				"[::b]Projects[-::-] Alpha, Beta\n",
		},
		{
			name: "vendor labels",
			rows: []views.InfoRow{
				{Label: "Name", Value: "claude"},
				{Label: "Projects", Value: "yes"},
				{Label: "Deep research", Value: "yes"},
			},
			want: "[::b]Name[-::-]          claude\n" +
				"[::b]Projects[-::-]      yes\n" +
				"[::b]Deep research[-::-] yes\n",
		},
		{
			name: "project labels",
			rows: []views.InfoRow{
				{Label: "Name", Value: "Alpha"},
				{Label: "Description", Value: "Backend rewrite"},
				{Label: "Instructions", Value: "Use Go interfaces"},
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
			views.WriteInfoRows(&sb, tc.rows)
			if got := sb.String(); got != tc.want {
				t.Errorf("views.WriteInfoRows() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewInfoViewTitle(t *testing.T) {
	t.Parallel()
	info := views.NewInfoView("Account Details")
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
			if got := views.InfoTextHeight(tc.text, tc.width); got != tc.want {
				t.Errorf("views.InfoTextHeight(%q, %d) = %d, want %d", tc.text, tc.width, got, tc.want)
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
			if got := views.FitInfoHeight(tc.required, tc.available, tc.minOther); got != tc.want {
				t.Errorf("views.FitInfoHeight(%d, %d, %d) = %d, want %d", tc.required, tc.available, tc.minOther, got, tc.want)
			}
		})
	}
}
