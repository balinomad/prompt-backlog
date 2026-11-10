package forms

import (
	"unicode"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// installTextAreaClipboard connects ta's copy and paste actions to the system
// clipboard. It also normalises Ctrl+Q, Ctrl+X, and Ctrl+V delivered as runes
// with the Ctrl modifier by terminals that report modified keys. Any input
// capture already installed on ta runs after capture.
func installTextAreaClipboard(app views.Host, ta *tview.TextArea, capture func(*tcell.EventKey) *tcell.EventKey) {
	ta.SetClipboard(
		func(text string) {
			if err := app.CopyToClipboard(text); err != nil {
				app.SetError("copy: " + err.Error())
			}
		},
		func() string {
			text, err := app.ReadFromClipboard()
			if err != nil {
				app.SetError("paste: " + err.Error())
				return ""
			}
			return text
		},
	)
	existing := ta.GetInputCapture()
	ta.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		event = normalizeClipboardKey(event)
		if capture != nil {
			event = capture(event)
		}
		if event == nil {
			return nil
		}
		if existing != nil {
			return existing(event)
		}
		return event
	})
}

// installFormClipboard wires the system clipboard to every text field in form.
func installFormClipboard(app views.Host, form *tview.Form) {
	for i := range form.GetFormItemCount() {
		switch item := form.GetFormItem(i).(type) {
		case *InputField:
			installInputFieldClipboard(app, item)
		case *tview.TextArea:
			installTextAreaClipboard(app, item, nil)
		}
	}
}

// installInputFieldClipboard wires a single-line field to the system clipboard.
// Copy and cut take the whole value because InputField exposes no selection.
func installInputFieldClipboard(app views.Host, field *InputField) {
	field.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		event = normalizeClipboardKey(event)
		switch event.Key() {
		case tcell.KeyCtrlQ:
			copyInputField(app, field)
			return nil
		case tcell.KeyCtrlX:
			if copyInputField(app, field) {
				field.SetText("")
			}
			return nil
		case tcell.KeyCtrlV:
			pasteInputField(app, field)
			return nil
		default:
			return event
		}
	})
}

// copyInputField copies the field value to the system clipboard.
func copyInputField(app views.Host, field *InputField) bool {
	if err := app.CopyToClipboard(field.GetText()); err != nil {
		app.SetError("copy: " + err.Error())
		return false
	}
	return true
}

// pasteInputField inserts the system clipboard text at the cursor.
func pasteInputField(app views.Host, field *InputField) {
	text, err := app.ReadFromClipboard()
	if err != nil {
		app.SetError("paste: " + err.Error())
		return
	}
	if text == "" {
		return
	}
	field.PasteHandler()(text, func(tview.Primitive) {})
}

// copyTextArea copies the full text area contents to the system clipboard.
func copyTextArea(app views.Host, ta *tview.TextArea) {
	if err := app.CopyToClipboard(ta.GetText()); err != nil {
		app.SetError("copy failed: " + err.Error())
		return
	}
	app.SetStatus("copied")
}

// pasteFromClipboard replaces the current selection with the system
// clipboard text, or inserts it at the cursor when there is no selection.
func pasteFromClipboard(app views.Host, ta *tview.TextArea) {
	text, err := app.ReadFromClipboard()
	if err != nil {
		app.SetError("paste failed: " + err.Error())
		return
	}
	if text == "" {
		return
	}
	_, start, end := ta.GetSelection()
	ta.Replace(start, end, text)
	app.SetStatus("pasted")
}

// normalizeClipboardKey converts rune events carrying Ctrl into the key
// constants that TextArea's clipboard actions handle.
func normalizeClipboardKey(event *tcell.EventKey) *tcell.EventKey {
	if event == nil || event.Key() != tcell.KeyRune || event.Modifiers()&tcell.ModCtrl == 0 {
		return event
	}
	switch unicode.ToLower(event.Rune()) {
	case 'q':
		return tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone)
	case 'x':
		return tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModNone)
	case 'v':
		return tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModNone)
	default:
		return event
	}
}
