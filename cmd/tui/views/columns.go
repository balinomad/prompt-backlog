package views

import (
	"strings"

	"github.com/rivo/tview"
)

// columnSeparator separates padded cells in list rows.
const columnSeparator = " │ "

// rowInset is the single-cell inset shared by list and detail rows.
const rowInset = " "

// formatColumns aligns rows into fixed-width columns separated by
// columnSeparator. Every cell, including the last, is padded to its column
// width so blank cells still show separators and keep later columns aligned.
// Rows may have different lengths; missing cells are rendered blank. Each
// row has a one-cell inset so text does not touch the panel frame. The
// returned width is the rendered width of every row, including both insets.
func formatColumns(rows [][]string) ([]string, int) {
	if len(rows) == 0 {
		return nil, 0
	}

	columns := 0
	for _, row := range rows {
		columns = max(columns, len(row))
	}
	if columns == 0 {
		return make([]string, len(rows)), 0
	}

	widths := make([]int, columns)
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], tview.TaggedStringWidth(cell))
		}
	}

	separatorWidth := tview.TaggedStringWidth(columnSeparator)
	width := 2 * tview.TaggedStringWidth(rowInset)
	for i, columnWidth := range widths {
		width += columnWidth
		if i > 0 {
			width += separatorWidth
		}
	}

	lines := make([]string, len(rows))
	for rowIndex, row := range rows {
		var b strings.Builder
		b.Grow(width)
		b.WriteString(rowInset)
		for i := range columns {
			if i > 0 {
				b.WriteString(columnSeparator)
			}
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			b.WriteString(padWidth(cell, widths[i]))
		}
		b.WriteString(rowInset)
		lines[rowIndex] = b.String()
	}

	return lines, width
}

// insetListRow prefixes a list row with the shared one-cell inset.
func insetListRow(s string) string {
	return rowInset + s
}

// truncateWidth shortens plain text to maxWidth display cells, appending an
// ellipsis when truncated. A maxWidth of zero or less means no limit. Brackets
// are measured as literal characters; callers escape the result afterwards.
func truncateWidth(s string, maxWidth int) string {
	if maxWidth <= 0 || plainWidth(s) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return "…"
	}

	target := maxWidth - 1
	var b strings.Builder
	width := 0
	for _, r := range s {
		runeWidth := plainWidth(string(r))
		if width+runeWidth > target {
			break
		}
		b.WriteRune(r)
		width += runeWidth
	}
	return b.String() + "…"
}

// plainWidth returns the display width of s when tview tags are treated as
// literal text.
func plainWidth(s string) int {
	return tview.TaggedStringWidth(tview.Escape(s))
}

// padWidth appends spaces until s occupies width display cells. Style tags
// count as zero width. Strings already at or above width are returned as-is.
func padWidth(s string, width int) string {
	if current := tview.TaggedStringWidth(s); current < width {
		return s + strings.Repeat(" ", width-current)
	}
	return s
}
