package views

import (
	"testing"

	"github.com/rivo/tview"
)

func TestNewFormTitle(t *testing.T) {
	t.Parallel()
	form := newForm("Edit Account")
	if got := form.GetTitle(); got != " Edit Account " {
		t.Errorf("GetTitle() = %q, want %q", got, " Edit Account ")
	}
}

func TestFormHeight(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		heights []int
		buttons int
		want    int
	}{
		{"no items", nil, 0, 4},
		{"one item", []int{1}, 0, 5},
		{"one item and button", []int{1}, 1, 7},
		{"two items and buttons", []int{1, 1}, 2, 9},
		{"three items and buttons", []int{1, 1, 1}, 3, 11},
		{"four items and buttons", []int{1, 1, 1, 1}, 2, 13},
		{"two-line items", []int{2, 2}, 0, 9},
		{"mixed item heights and buttons", []int{1, 2, 2}, 2, 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			form := tview.NewForm()
			for _, height := range tc.heights {
				if height == 1 {
					form.AddInputField("field", "", formFieldWidth, nil, nil)
					continue
				}
				form.AddTextArea("field", "", formFieldWidth, height, 0, nil)
			}
			for range tc.buttons {
				form.AddButton("button", nil)
			}
			if got := formHeight(form); got != tc.want {
				t.Errorf("formHeight() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFormWidth(t *testing.T) {
	t.Parallel()
	labelText := "claude · Alice"
	cases := []struct {
		name string
		form func() *tview.Form
		want int
	}{
		{
			name: "label and field",
			form: func() *tview.Form {
				form := tview.NewForm()
				form.AddInputField("Name", "", formFieldWidth, nil, nil)
				return form
			},
			want: 49,
		},
		{
			name: "pause field",
			form: func() *tview.Form {
				form := tview.NewForm()
				form.AddInputField("Pause until", "", formPauseWidth, nil, nil)
				return form
			},
			want: 36,
		},
		{
			name: "buttons",
			form: func() *tview.Form {
				form := tview.NewForm()
				form.AddButton("Save", nil)
				form.AddButton("Cancel", nil)
				return form
			},
			want: 23,
		},
		{
			name: "centered label",
			form: func() *tview.Form {
				form := tview.NewForm()
				form.AddFormItem(newCenteredFormLabel(labelText))
				return form
			},
			want: tview.TaggedStringWidth(labelText) + 1 + 4,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formWidth(tc.form()); got != tc.want {
				t.Errorf("formWidth() = %d, want %d", got, tc.want)
			}
		})
	}
}
