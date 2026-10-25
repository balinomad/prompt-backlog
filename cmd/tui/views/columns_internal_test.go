package views

import (
	"testing"

	"github.com/rivo/tview"
)

func TestFormatColumns_PadsBlankCells(t *testing.T) {
	t.Parallel()
	lines, width := formatColumns([][]string{
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
			if got := truncateWidth(tc.input, tc.maxWidth); got != tc.want {
				t.Errorf("truncateWidth(%q, %d) = %q, want %q", tc.input, tc.maxWidth, got, tc.want)
			}
		})
	}
}

func TestPadWidth_IgnoresStyleTags(t *testing.T) {
	t.Parallel()
	if got, want := padWidth("[red]x[-]", 3), "[red]x[-]  "; got != want {
		t.Errorf("padWidth = %q, want %q", got, want)
	}
}
