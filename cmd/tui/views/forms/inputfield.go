package forms

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// InputField is a single-line form field backed by tview.TextArea. It supports
// the same clipboard actions as multiline fields and therefore uses the
// application clipboard rather than tview's field-local buffer.
type InputField struct {
	*tview.TextArea
	finished func(tcell.Key)
}

// newInputField returns a single-line input field with label and initial text.
func newInputField(label, text string, width int) *InputField {
	textArea := tview.NewTextArea().SetWrap(false)
	textArea.SetLabel(label)
	textArea.SetSize(1, width)
	textArea.SetText(text, true)
	textArea.SetTextStyle(tcell.StyleDefault.
		Background(tview.Styles.ContrastBackgroundColor).
		Foreground(tview.Styles.PrimaryTextColor))
	textArea.SetPlaceholderStyle(tcell.StyleDefault.
		Background(tview.Styles.ContrastBackgroundColor).
		Foreground(tview.Styles.ContrastSecondaryTextColor))

	field := &InputField{TextArea: textArea}
	textArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			if field.finished != nil {
				field.finished(tcell.KeyEnter)
			}
			return nil
		}
		return event
	})
	return field
}

// addInputField appends a single-line field to form.
func addInputField(form *tview.Form, label, text string, width int) {
	form.AddFormItem(newInputField(label, text, width))
}

// SetFinishedFunc sets the handler called when the field is left.
func (f *InputField) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	f.finished = handler
	f.TextArea.SetFinishedFunc(handler)
	return f
}
