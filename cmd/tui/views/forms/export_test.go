package forms

import (
	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/internal/model"

	"github.com/rivo/tview"
)

// NewForm exposes newForm for black-box tests.
var NewForm = newForm

// FormHeight exposes formHeight for black-box tests.
var FormHeight = formHeight

// FormWidth exposes formWidth for black-box tests.
var FormWidth = formWidth

// FormFieldWidth is the exported formFieldWidth constant for black-box tests.
const FormFieldWidth = formFieldWidth

// PromptSideWidth is the exported promptSideWidth constant for black-box tests.
const PromptSideWidth = promptSideWidth

// NormalizeClipboardKey exposes normalizeClipboardKey for black-box tests.
var NormalizeClipboardKey = normalizeClipboardKey

// PromptPreview exposes promptPreview for black-box tests.
var PromptPreview = promptPreview

// DropDownArrow is the exported dropDownArrow constant for black-box tests.
const DropDownArrow = dropDownArrow

// FormPauseWidth is the exported formPauseWidth constant for black-box tests.
const FormPauseWidth = formPauseWidth

// NewCenteredFormLabel exposes newCenteredFormLabel for black-box tests.
func NewCenteredFormLabel(text string) tview.FormItem {
	return newCenteredFormLabel(text)
}

// NewDropDown exposes newDropDown for black-box tests.
var NewDropDown = newDropDown

// PromptForm wraps the Add/Edit editor for black-box tests.
type PromptForm struct {
	editor *promptEditor
}

// NewPromptFormForTest constructs a prompt form for black-box tests.
func NewPromptFormForTest(app views.Host, session *model.Session, mode PromptFormMode, index int, onSaved, onClose func(), onSaveAndPause ...func()) *PromptForm {
	var pause func()
	if len(onSaveAndPause) > 0 {
		pause = onSaveAndPause[0]
	}
	return &PromptForm{editor: newPromptEditor(app, session, PromptFormOptions{
		Mode:           mode,
		Index:          index,
		OnSaved:        onSaved,
		OnClose:        onClose,
		OnSaveAndPause: pause,
	})}
}

// Primitive returns the form's modal primitive.
func (p *PromptForm) Primitive() tview.Primitive {
	return p.editor.primitive()
}

// Editor returns the Add/Edit root Flex.
func (p *PromptForm) Editor() *tview.Flex {
	return p.editor.Flex
}

// Label returns the Prompt label.
func (p *PromptForm) Label() *tview.TextView {
	return p.editor.label
}

// TextArea returns the prompt text area.
func (p *PromptForm) TextArea() *tview.TextArea {
	return p.editor.textArea
}

// Status returns the status dropdown.
func (p *PromptForm) Status() *tview.DropDown {
	return p.editor.status
}

// PromptList returns the prompt list, or nil for Add mode.
func (p *PromptForm) PromptList() *tview.List {
	return p.editor.list
}

// AddPrompt inserts a blank prompt after the selected one.
func (p *PromptForm) AddPrompt() {
	p.editor.addPrompt()
}

// DeletePrompt removes the selected prompt from the open editor.
func (p *PromptForm) DeletePrompt() {
	p.editor.deletePrompt()
}

// ClearButton returns the Clear button.
func (p *PromptForm) ClearButton() *tview.Button {
	return p.editor.clearButton
}

// CopyButton returns the Copy button.
func (p *PromptForm) CopyButton() *tview.Button {
	return p.editor.copyButton
}

// PasteButton returns the Paste button.
func (p *PromptForm) PasteButton() *tview.Button {
	return p.editor.pasteButton
}

// SessionButton returns the Session button.
func (p *PromptForm) SessionButton() *tview.Button {
	return p.editor.sessionButton
}

// SaveButton returns the Save button.
func (p *PromptForm) SaveButton() *tview.Button {
	return p.editor.saveButton
}

// SavePauseButton returns the Save & Pause button.
func (p *PromptForm) SavePauseButton() *tview.Button {
	return p.editor.savePauseButton
}

// CancelButton returns the Cancel button.
func (p *PromptForm) CancelButton() *tview.Button {
	return p.editor.cancelButton
}

// DeleteButton returns the Delete button, or nil for Add mode.
func (p *PromptForm) DeleteButton() *tview.Button {
	return p.editor.deleteButton
}

// FocusEditor focuses the prompt editor as the application would.
func (p *PromptForm) FocusEditor() {
	p.editor.app.SetFocus(p.editor.textArea)
}

// NoteForm wraps the note editor for black-box tests.
type NoteForm struct {
	editor *noteEditor
}

// NoteFormFromPrimitive returns the note editor inside a modal primitive.
func NoteFormFromPrimitive(p tview.Primitive) (*NoteForm, bool) {
	p = views.UnwrapModal(p)
	outer, ok := p.(*tview.Flex)
	if !ok {
		return nil, false
	}
	column, ok := outer.GetItem(1).(*tview.Flex)
	if !ok {
		return nil, false
	}
	row, ok := column.GetItem(2).(*tview.Flex)
	if !ok {
		return nil, false
	}
	content := row.GetItem(2)
	if frame, isFrame := content.(*tview.Frame); isFrame {
		content = frame.GetPrimitive()
	}
	editor, ok := content.(*noteEditor)
	if !ok {
		return nil, false
	}
	return &NoteForm{editor: editor}, true
}

// Title returns the editor title.
func (n *NoteForm) Title() string {
	return n.editor.GetTitle()
}

// TextArea returns the note text area.
func (n *NoteForm) TextArea() *tview.TextArea {
	return n.editor.textArea
}

// Project returns the Project dropdown.
func (n *NoteForm) Project() *tview.DropDown {
	return n.editor.project
}

// Account returns the Account dropdown.
func (n *NoteForm) Account() *tview.DropDown {
	return n.editor.account
}

// Button returns the button with label, or nil.
func (n *NoteForm) Button(label string) *tview.Button {
	switch label {
	case "Clear":
		return n.editor.clearButton
	case "Copy":
		return n.editor.copyButton
	case "Paste":
		return n.editor.pasteButton
	case "Save":
		return n.editor.saveButton
	case "Cancel":
		return n.editor.cancelButton
	default:
		return nil
	}
}

func promptEditorFromPrimitive(p tview.Primitive) (*promptEditor, bool) {
	p = views.UnwrapModal(p)
	outer, ok := p.(*tview.Flex)
	if !ok {
		return nil, false
	}
	column, ok := outer.GetItem(1).(*tview.Flex)
	if !ok {
		return nil, false
	}
	row, ok := column.GetItem(2).(*tview.Flex)
	if !ok {
		return nil, false
	}
	content := row.GetItem(2)
	if frame, isFrame := content.(*tview.Frame); isFrame {
		content = frame.GetPrimitive()
	}
	editor, ok := content.(*promptEditor)
	return editor, ok
}

// PromptStateFromPrimitive returns the prompt text and status from a prompt
// editor modal primitive.
func PromptStateFromPrimitive(p tview.Primitive) (string, model.SessionStatus, bool, error) {
	editor, ok := promptEditorFromPrimitive(p)
	if !ok {
		return "", model.SessionStatusActive, false, nil
	}
	status, err := editor.statusValue()
	if err != nil {
		return "", 0, false, err
	}
	return editor.textArea.GetText(), status, true, nil
}

// PromptListFromPrimitive returns the prompt list from an Edit modal primitive.
func PromptListFromPrimitive(p tview.Primitive) *tview.List {
	editor, ok := promptEditorFromPrimitive(p)
	if !ok {
		return nil
	}
	return editor.list
}

// SavePromptFromPrimitive runs the save action on an Edit modal primitive.
func SavePromptFromPrimitive(p tview.Primitive) {
	if editor, ok := promptEditorFromPrimitive(p); ok {
		editor.save()
	}
}
