package forms

import (
	"github.com/balinomad/prompt-backlog/cmd/tui/views"

	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	noteSideWidth   = 9
	noteModalWidth  = noteSideWidth + 1 + noteFieldWidth + 4
	noteModalHeight = noteTextHeight + 1 + 1 + 1 + 1 + 1 + 1 + 4
)

// noteEditor is the note modal. It mirrors the prompt editor's side column of
// clipboard actions next to the text area.
type noteEditor struct {
	*tview.Flex
	app          views.Host
	textArea     *tview.TextArea
	project      *tview.DropDown
	account      *tview.DropDown
	clearButton  *tview.Button
	copyButton   *tview.Button
	pasteButton  *tview.Button
	saveButton   *tview.Button
	cancelButton *tview.Button
	projectNames []string
	accounts     []*model.Account
	fixedProject *model.Project
	noteID       uint64
	isEdit       bool
	onClose      func()
	focusables   []tview.Primitive
}

// NewNoteForm returns a note form. On the main screen account is the account
// in view and fixedProject is nil. On the Projects screen fixedProject is the
// selected project, the Project dropdown is disabled, and account is ignored.
func NewNoteForm(app views.Host, note *model.Note, account *model.Account, fixedProject *model.Project, onClose func()) tview.Primitive {
	svc := app.Service()
	if note != nil {
		live := svc.Note(note.ID())
		if live == nil {
			return views.ErrorModal("Note no longer exists.", onClose)
		}
		note = live
	}

	data := newNoteFormData(svc, note, account, fixedProject)
	return newNoteEditor(app, data, fixedProject, onClose).primitive()
}

// noteFormData is the initial state shared by the note editor's fields.
type noteFormData struct {
	title         string
	initialText   string
	isEdit        bool
	noteID        uint64
	projectNames  []string
	projectIndex  int
	accounts      []*model.Account
	accountLabels []string
	accountIndex  int
}

// newNoteFormData resolves the note editor's initial field values.
func newNoteFormData(svc *service.Service, note *model.Note, account *model.Account, fixedProject *model.Project) *noteFormData {
	data := noteFormData{title: "New Note"}
	if note != nil {
		data.title = "Edit Note"
		data.initialText = note.Text()
		data.isEdit = true
		data.noteID = note.ID()
	}

	data.projectNames = []string{"none"}
	if fixedProject != nil {
		data.projectNames = []string{fixedProject.Name()}
	} else {
		projects := svc.Projects()
		for _, p := range projects {
			data.projectNames = append(data.projectNames, p.Name())
		}
		if data.isEdit && note.ProjectID() != 0 {
			for i, p := range projects {
				if p.ID() == note.ProjectID() {
					data.projectIndex = i + 1
					break
				}
			}
		}
	}

	data.accounts = noteAccountOptions(svc, account, fixedProject)
	data.accountLabels = make([]string, 0, len(data.accounts)+1)
	data.accountLabels = append(data.accountLabels, "none")
	for _, acc := range data.accounts {
		data.accountLabels = append(data.accountLabels, accountDisplayName(svc, acc))
	}
	if data.isEdit {
		if key, ok := note.AccountKey(); ok {
			for i, acc := range data.accounts {
				if acc.Key() == key {
					data.accountIndex = i + 1
					break
				}
			}
		}
	} else if fixedProject == nil && account != nil {
		data.accountIndex = 1
	}
	return &data
}

// newNoteEditor builds the note editor from resolved form data.
func newNoteEditor(app views.Host, data *noteFormData, fixedProject *model.Project, onClose func()) *noteEditor {
	e := &noteEditor{
		app:          app,
		projectNames: data.projectNames,
		accounts:     data.accounts,
		fixedProject: fixedProject,
		noteID:       data.noteID,
		isEdit:       data.isEdit,
		onClose:      onClose,
	}
	e.build(data)
	return e
}

// build constructs the note editor's fields and layout.
func (e *noteEditor) build(data *noteFormData) {
	e.textArea = tview.NewTextArea()
	e.textArea.SetTextStyle(tcell.StyleDefault.
		Background(tview.Styles.ContrastBackgroundColor).
		Foreground(tview.Styles.PrimaryTextColor))
	e.textArea.SetText(data.initialText, false)

	e.project = newDropDown("Project", escapeAll(data.projectNames), data.projectIndex)
	e.project.SetLabelWidth(noteSideWidth + 1)
	if e.fixedProject != nil {
		e.project.SetDisabled(true)
	}
	e.account = newDropDown("Account", escapeAll(data.accountLabels), data.accountIndex)
	e.account.SetLabelWidth(noteSideWidth + 1)

	e.clearButton = tview.NewButton("Clear")
	e.copyButton = tview.NewButton("Copy")
	e.pasteButton = tview.NewButton("Paste")
	e.saveButton = tview.NewButton("Save")
	e.cancelButton = tview.NewButton("Cancel")

	e.Flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(e.editorRow(), noteTextHeight, 0, true).
		AddItem(nil, 1, 0, false).
		AddItem(e.projectRow(), 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(e.accountRow(), 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(e.buttonRow(), 1, 0, false)
	e.SetBorder(true).SetBorderPadding(1, 1, 1, 1).SetTitle(" " + data.title + " ")
	e.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	e.wire()
}

// editorRow returns the text label, clipboard side column, and text area.
func (e *noteEditor) editorRow() tview.Primitive {
	label := tview.NewTextView().SetText("Text").SetWrap(false)
	label.SetTextColor(views.ColorLabel)
	label.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	buttonWidth := max(tview.TaggedStringWidth("Clear"), tview.TaggedStringWidth("Copy"), tview.TaggedStringWidth("Paste")) + 4
	side := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(label, 1, 0, false).
		AddItem(nil, noteTextHeight-6, 0, false).
		AddItem(e.pasteButton, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(e.clearButton, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(e.copyButton, 1, 0, false)

	return tview.NewFlex().
		AddItem(side, buttonWidth, 0, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(e.textArea, 0, 1, true)
}

// projectRow returns the project dropdown row.
func (e *noteEditor) projectRow() tview.Primitive {
	return tview.NewFlex().
		AddItem(e.project, noteSideWidth+1+e.project.GetFieldWidth(), 0, false).
		AddItem(nil, 0, 1, false)
}

// accountRow returns the account dropdown row.
func (e *noteEditor) accountRow() tview.Primitive {
	return tview.NewFlex().
		AddItem(e.account, noteSideWidth+1+e.account.GetFieldWidth(), 0, false).
		AddItem(nil, 0, 1, false)
}

// buttonRow returns the save and cancel button row.
func (e *noteEditor) buttonRow() tview.Primitive {
	saveWidth := tview.TaggedStringWidth("Save") + 4
	cancelWidth := tview.TaggedStringWidth("Cancel") + 4
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(e.saveButton, saveWidth, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(e.cancelButton, cancelWidth, 0, false).
		AddItem(nil, 0, 1, false)
}

// wire installs focus order, button handlers, and shared input capture.
func (e *noteEditor) wire() {
	if e.fixedProject == nil {
		e.focusables = append(e.focusables, e.project)
	}
	e.focusables = append(e.focusables, e.account, e.pasteButton, e.clearButton, e.copyButton, e.textArea, e.saveButton, e.cancelButton)

	e.clearButton.SetSelectedFunc(e.clear)
	e.copyButton.SetSelectedFunc(e.copy)
	e.pasteButton.SetSelectedFunc(e.paste)
	e.saveButton.SetSelectedFunc(e.save)
	e.cancelButton.SetSelectedFunc(e.onClose)
	e.installInputCapture()
}

// Focus delegates focus to the note text area.
func (e *noteEditor) Focus(delegate func(p tview.Primitive)) {
	delegate(e.textArea)
}

// HasFocus reports whether any note editor child has focus.
func (e *noteEditor) HasFocus() bool {
	for _, p := range e.focusables {
		if p.HasFocus() {
			return true
		}
	}
	return false
}

func (e *noteEditor) installInputCapture() {
	capture := func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			e.moveFocus(1)
			return nil
		case tcell.KeyBacktab:
			e.moveFocus(-1)
			return nil
		case views.KeySave:
			e.save()
			return nil
		case views.KeyCancel:
			e.onClose()
			return nil
		default:
			return event
		}
	}
	for _, p := range e.focusables {
		switch v := p.(type) {
		case *tview.TextArea:
			installTextAreaClipboard(e.app, v, capture)
		case *tview.Button:
			v.SetInputCapture(capture)
		case *tview.DropDown:
			v.SetInputCapture(capture)
		}
	}
}

func (e *noteEditor) moveFocus(delta int) {
	index := 0
	for i, p := range e.focusables {
		if p.HasFocus() {
			index = i
			break
		}
	}
	index = (index + delta + len(e.focusables)) % len(e.focusables)
	e.app.SetFocus(e.focusables[index])
}

func (e *noteEditor) primitive() tview.Primitive {
	frame := tview.NewFrame(e)
	frame.SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	return views.NewModalState(views.CenteredModal(frame, noteModalWidth, noteModalHeight), e)
}

func (e *noteEditor) clear() {
	e.textArea.SetText("", false)
	e.app.SetFocus(e.textArea)
}

func (e *noteEditor) copy() {
	copyTextArea(e.app, e.textArea)
}

func (e *noteEditor) paste() {
	pasteFromClipboard(e.app, e.textArea)
}

func (e *noteEditor) save() {
	projectIdx, _ := e.project.GetCurrentOption()
	projectName := ""
	if e.fixedProject != nil {
		projectName = e.fixedProject.Name()
	} else if projectIdx > 0 && projectIdx < len(e.projectNames) {
		projectName = e.projectNames[projectIdx]
	}

	var selected *model.Account
	accountIdx, _ := e.account.GetCurrentOption()
	if accountIdx > 0 && accountIdx <= len(e.accounts) {
		selected = e.accounts[accountIdx-1]
	}
	email, vendor := "", ""
	if selected != nil {
		email = selected.UserEmail()
		vendor = selected.VendorName()
	}

	text := e.textArea.GetText()
	if err := e.app.Mutate(func(s *service.Service) error {
		if !e.isEdit {
			_, err := s.AddNote(text, projectName, email, vendor)
			return err
		}
		return s.UpdateNote(e.noteID, text, projectName, email, vendor)
	}); err != nil {
		views.SetErrorStatus(e.app, err)
		return
	}
	e.onClose()
}

// noteAccountOptions returns the accounts selectable in a note form. On the
// Projects screen they are the project's granted accounts; on the main screen
// they are the account in view, if any.
func noteAccountOptions(svc *service.Service, account *model.Account, fixedProject *model.Project) []*model.Account {
	if fixedProject != nil {
		accounts, err := svc.ProjectAccounts(fixedProject.Name())
		if err != nil {
			return nil
		}
		views.SortProjectAccounts(svc, accounts)
		return accounts
	}
	if account == nil {
		return nil
	}
	return []*model.Account{account}
}

// accountDisplayName formats an account as vendor and user name.
func accountDisplayName(svc *service.Service, acc *model.Account) string {
	return acc.VendorName() + " · " + views.ProjectAccountUserName(svc, acc)
}
