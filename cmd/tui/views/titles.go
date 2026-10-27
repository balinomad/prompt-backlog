package views

import (
	"strconv"

	"github.com/rivo/tview"
)

// countTitle appends the 1-based selection position and item count to base.
// base keeps its own surrounding spaces. An empty list reports 0/0.
func countTitle(base string, index, total int) string {
	n := 0
	if total > 0 {
		n = min(index+1, total)
	}
	return base + strconv.Itoa(n) + "/" + strconv.Itoa(total) + " "
}

// setListTitle updates list's title with its selection position.
func setListTitle(list *tview.List, base string, index, total int) {
	list.SetTitle(countTitle(base, index, total))
}
