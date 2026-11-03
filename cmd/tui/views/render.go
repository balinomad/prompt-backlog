package views

import (
	"slices"
	"strconv"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/rivo/tview"
)

// Column widths shared by list rows.
const (
	ProjectColumnWidth     = 14
	SessionNameColumnWidth = 26
	VendorColumnWidthCap   = 12
	NameColumnWidthCap     = 20
)

// ColumnSeparator separates padded cells in list rows.
const ColumnSeparator = " │ "

// RowInset is the single-cell inset shared by list and detail rows.
const RowInset = " "

// FormatColumns aligns rows into fixed-width columns separated by
// ColumnSeparator. Every cell, including the last, is padded to its column
// width so blank cells still show separators and keep later columns aligned.
// Rows may have different lengths; missing cells are rendered blank. Each
// row has a one-cell inset so text does not touch the panel frame. The
// returned width is the rendered width of every row, including both insets.
func FormatColumns(rows [][]string) ([]string, int) {
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

	separatorWidth := tview.TaggedStringWidth(ColumnSeparator)
	width := 2 * tview.TaggedStringWidth(RowInset)
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
		b.WriteString(RowInset)
		for i := range columns {
			if i > 0 {
				b.WriteString(ColumnSeparator)
			}
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			b.WriteString(PadWidth(cell, widths[i]))
		}
		b.WriteString(RowInset)
		lines[rowIndex] = b.String()
	}

	return lines, width
}

// DropEmptyColumns removes columns that are empty in every row.
func DropEmptyColumns(rows [][]string) [][]string {
	if len(rows) == 0 {
		return rows
	}
	columns := 0
	for _, row := range rows {
		columns = max(columns, len(row))
	}
	keep := make([]bool, columns)
	for _, row := range rows {
		for i, cell := range row {
			if cell != "" {
				keep[i] = true
			}
		}
	}
	result := make([][]string, len(rows))
	for r, row := range rows {
		out := make([]string, 0, columns)
		for i := range columns {
			if !keep[i] {
				continue
			}
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			out = append(out, cell)
		}
		result[r] = out
	}
	return result
}

// InsetListRow prefixes a list row with the shared one-cell inset.
func InsetListRow(s string) string {
	return RowInset + s
}

// TruncateWidth shortens plain text to maxWidth display cells, appending an
// ellipsis when truncated. A maxWidth of zero or less means no limit. Brackets
// are measured as literal characters; callers escape the result afterwards.
func TruncateWidth(s string, maxWidth int) string {
	if maxWidth <= 0 || PlainWidth(s) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return "…"
	}

	target := maxWidth - 1
	var b strings.Builder
	width := 0
	for _, r := range s {
		runeWidth := PlainWidth(string(r))
		if width+runeWidth > target {
			break
		}
		b.WriteRune(r)
		width += runeWidth
	}
	return b.String() + "…"
}

// PlainWidth returns the display width of s when tview tags are treated as
// literal text.
func PlainWidth(s string) int {
	return tview.TaggedStringWidth(tview.Escape(s))
}

// PadWidth appends spaces until s occupies width display cells. Style tags
// count as zero width. Strings already at or above width are returned as-is.
func PadWidth(s string, width int) string {
	if current := tview.TaggedStringWidth(s); current < width {
		return s + strings.Repeat(" ", width-current)
	}
	return s
}

// CountTitle appends the 1-based selection position and item count to base.
// base keeps its own surrounding spaces. An empty list reports 0/0.
func CountTitle(base string, index, total int) string {
	n := 0
	if total > 0 {
		n = min(index+1, total)
	}
	return base + strconv.Itoa(n) + "/" + strconv.Itoa(total) + " "
}

// SetListTitle updates list's title with its selection position.
func SetListTitle(list *tview.List, base string, index, total int) {
	list.SetTitle(CountTitle(base, index, total))
}

// ── Info view helpers ─────────────────────────────────────────────────────────

const (
	// infoPadding keeps every detail line, including wrapped lines, clear of
	// the panel frame.
	infoPadding = 1
	// MinListPanelHeight keeps the list usable above a detail panel.
	MinListPanelHeight = 3
)

// InfoRow is one label and value pair in a detail panel.
type InfoRow struct {
	Label string
	Value string
}

// NewInfoView returns a detail text view with the shared border, inset, and
// title spacing.
func NewInfoView(title string) *tview.TextView {
	info := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true)
	info.SetBorder(true)
	info.SetBorderPadding(0, 0, infoPadding, infoPadding)
	info.SetTitle(" " + title + " ")
	return info
}

// WriteInfoRows appends aligned detail rows to sb. The label column is sized
// from the longest label in rows.
func WriteInfoRows(sb *strings.Builder, rows []InfoRow) {
	width := 0
	for _, row := range rows {
		width = max(width, len(row.Label))
	}
	for _, row := range rows {
		sb.WriteString("[::b]")
		sb.WriteString(row.Label)
		sb.WriteString("[-::-]")
		sb.WriteString(strings.Repeat(" ", max(1, width-len(row.Label)+1)))
		sb.WriteString(row.Value)
		sb.WriteByte('\n')
	}
}

// InfoTextHeight returns the outer height needed to render text at outerWidth,
// including the border. Left and right padding reduce the wrapping width.
func InfoTextHeight(text string, outerWidth int) int {
	innerWidth := max(1, outerWidth-2-2*infoPadding)
	text = strings.TrimRight(text, "\n")
	lines := tview.WordWrap(text, innerWidth)
	if len(lines) == 0 {
		lines = []string{""}
	}
	return len(lines) + 2
}

// FitInfoHeight clamps required so the other panel keeps minOtherHeight rows.
func FitInfoHeight(required, available, minOtherHeight int) int {
	if available <= 0 {
		return 0
	}
	maxHeight := available - minOtherHeight
	if maxHeight < 1 {
		return 0
	}
	return min(required, maxHeight)
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// DimText wraps s in tview's dim colour tag.
func DimText(s string) string {
	return "[::d]" + s + "[-::-]"
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// ProjectAccountUserName returns the account's user display name, falling
// back to the email when the user record is missing.
func ProjectAccountUserName(svc *service.Service, acc *model.Account) string {
	if u := svc.User(acc.UserEmail()); u != nil {
		return u.Name()
	}
	return acc.UserEmail()
}

// ProjectAccountText formats one granted account as Vendor │ User for the
// grant form. The vendor and user name are truncated to vendorWidth and
// nameWidth; both widths are capped by the shared project-screen limits.
func ProjectAccountText(svc *service.Service, acc *model.Account, vendorWidth, nameWidth int) string {
	vendor := TruncateWidth(acc.VendorName(), vendorWidth)
	name := TruncateWidth(ProjectAccountUserName(svc, acc), nameWidth)
	return vendor + ColumnSeparator + name
}

// ProjectAccountWidths returns the vendor and user-name widths used by the
// Projects screen's granted-accounts list and grant form. They use store-wide
// values, including the email fallback for accounts without a user record,
// and the shared project-screen caps.
func ProjectAccountWidths(svc *service.Service) (int, int) {
	var vendorWidth, nameWidth int
	for _, vendor := range svc.Vendors() {
		vendorWidth = max(vendorWidth, PlainWidth(vendor.Name()))
	}
	vendorWidth = min(vendorWidth, VendorColumnWidthCap)

	for _, u := range svc.Users() {
		nameWidth = max(nameWidth, PlainWidth(u.Name()))
	}
	for _, acc := range svc.Accounts() {
		if svc.User(acc.UserEmail()) == nil {
			nameWidth = max(nameWidth, PlainWidth(acc.UserEmail()))
		}
	}
	nameWidth = min(nameWidth, NameColumnWidthCap)
	return vendorWidth, nameWidth
}

// SortProjectAccounts orders accounts alphabetically by vendor, then by
// user name, using a case-insensitive comparison. Equal pairs keep their
// incoming order.
func SortProjectAccounts(svc *service.Service, accounts []*model.Account) {
	type key struct {
		vendor string
		user   string
	}
	keys := make(map[model.AccountKey]key, len(accounts))
	for _, acc := range accounts {
		keys[acc.Key()] = key{
			vendor: strings.ToLower(acc.VendorName()),
			user:   strings.ToLower(ProjectAccountUserName(svc, acc)),
		}
	}
	slices.SortStableFunc(accounts, func(a, b *model.Account) int {
		ka, kb := keys[a.Key()], keys[b.Key()]
		if c := strings.Compare(ka.vendor, kb.vendor); c != 0 {
			return c
		}
		return strings.Compare(ka.user, kb.user)
	})
}

// ProjectAccountUserNameForKey returns the user display name for key, falling
// back to the email when the user record is missing.
func ProjectAccountUserNameForKey(svc *service.Service, key model.AccountKey) string {
	if u := svc.User(key.Email); u != nil {
		return u.Name()
	}
	return key.Email
}
