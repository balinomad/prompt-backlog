package forms

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// InputField is a single-line form field with application clipboard support.
type InputField struct {
	*tview.InputField
}

// newInputField returns a single-line input field with label and initial text.
func newInputField(label, text string, width int) *InputField {
	field := tview.NewInputField().
		SetLabel(label).
		SetFieldWidth(width).
		SetText(text)
	field.SetFieldStyle(tcell.StyleDefault.
		Background(tview.Styles.ContrastBackgroundColor).
		Foreground(tview.Styles.PrimaryTextColor))
	field.SetPlaceholderStyle(tcell.StyleDefault.
		Background(tview.Styles.ContrastBackgroundColor).
		Foreground(tview.Styles.ContrastSecondaryTextColor))
	return &InputField{InputField: field}
}

// SetText sets the field text. The unused argument preserves the TextArea
// signature used by existing callers.
func (f *InputField) SetText(text string, _ bool) {
	f.InputField.SetText(text)
}

// addInputField appends a single-line field to form.
func addInputField(form *tview.Form, label, text string, width int) {
	form.AddFormItem(newInputField(label, text, width))
}
