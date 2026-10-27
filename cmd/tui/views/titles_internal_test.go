package views

import (
	"testing"

	"github.com/rivo/tview"
)

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
			if got := countTitle(tc.base, tc.index, tc.total); got != tc.want {
				t.Errorf("countTitle(%q, %d, %d) = %q, want %q", tc.base, tc.index, tc.total, got, tc.want)
			}
		})
	}
}

func TestSetListTitle(t *testing.T) {
	t.Parallel()
	list := tview.NewList()
	setListTitle(list, " Accounts ", 1, 3)
	if got, want := list.GetTitle(), " Accounts 2/3 "; got != want {
		t.Errorf("GetTitle() = %q, want %q", got, want)
	}
}
