package forms

import (
	"fmt"
	"slices"
	"strings"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PromptFormMode selects the prompt form's title, initial text and buttons.
type PromptFormMode int

const (
	// PromptFormAdd appends a new prompt to the queue.
	PromptFormAdd PromptFormMode = iota
	// PromptFormEdit edits the session's prompt queue.
	PromptFormEdit
)

var sessionStatusLabels = []string{"Active", "Needs closing", "Closed"}

const (
	promptTextWidth        = 60
	promptSideWidth        = 9
	promptModalWidth       = promptSideWidth + 1 + promptTextWidth + 4
	promptModalTotalHeight = 22
	promptListMaxRows      = 5
	promptListChrome       = 2
	promptMinFieldHeight   = 6
	promptAddFieldHeight   = 14
)

// promptEditorState preserves the prompt editor across the session modal.
type promptEditorState struct {
	mode           PromptFormMode
	index          int
	text           string
	status         model.SessionStatus
	queue          []model.Prompt
	focusList      bool
	onSaved        func()
	onClose        func()
	onSaveAndPause func()
}

// promptEditor is the Add/Edit prompt modal. It uses standard widgets and a
// custom layout rather than a custom tview FormItem.
type promptEditor struct {
	*tview.Flex
	app             views.Host
	label           *tview.TextView
	textArea        *tview.TextArea
	status          *tview.DropDown
	list            *tview.List
	clearButton     *tview.Button
	copyButton      *tview.Button
	pasteButton     *tview.Button
	sessionButton   *tview.Button
	savePauseButton *tview.Button
	saveButton      *tview.Button
	cancelButton    *tview.Button
	deleteButton    *tview.Button
	email           string
	vendor          string
	sessionID       uint64
	mode            PromptFormMode
	index           int
	queue           []model.Prompt
	onSaved         func()
	onClose         func()
	onSaveAndPause  func()
	focusables      []tview.Primitive
	refreshing      bool
	listHeight      int
	fieldHeight     int
}

// PromptFormOptions configures the prompt form.
type PromptFormOptions struct {
	Mode           PromptFormMode
	Index          int
	OnClose        func()
	OnSaved        func()
	OnSaveAndPause func()
}

// NewPromptForm returns a prompt form for session. mode selects add or edit;
// index is the queue index for edit mode.
func NewPromptForm(app views.Host, session *model.Session, options PromptFormOptions) tview.Primitive {
	if session == nil {
		return views.ErrorModal("No session selected.", options.OnClose)
	}
	live := app.Service().Session(session.ID())
	if live == nil {
		return views.ErrorModal("Session no longer exists.", options.OnClose)
	}
	if options.Mode == PromptFormEdit && (options.Index < 0 || options.Index >= len(live.Queue())) {
		return views.ErrorModal("Prompt no longer exists.", options.OnClose)
	}
	return newPromptEditor(app, live, options).primitive()
}

func newPromptEditor(app views.Host, session *model.Session, options PromptFormOptions) *promptEditor {
	queue := session.Queue()
	initialText := ""
	if options.Mode == PromptFormEdit {
		initialText = queue[options.Index].Text()
	} else {
		queue = nil
	}
	state := promptEditorState{
		mode:           options.Mode,
		index:          options.Index,
		text:           initialText,
		status:         session.Status(),
		queue:          queue,
		onSaved:        options.OnSaved,
		onClose:        options.OnClose,
		onSaveAndPause: options.OnSaveAndPause,
	}
	return newPromptEditorFromState(app, session, &state)
}

func newPromptEditorFromState(app views.Host, session *model.Session, state *promptEditorState) *promptEditor {
	queue := slices.Clone(state.queue)
	if state.mode != PromptFormEdit {
		queue = nil
	}
	index := state.index
	title := "New Prompt"
	if state.mode == PromptFormEdit {
		switch {
		case len(queue) == 0:
			index = -1
			title = "Edit Prompts"
		case index < 0 || index >= len(queue):
			index = 0
			title = fmt.Sprintf("Edit Prompts %d/%d", index+1, len(queue))
		default:
			title = fmt.Sprintf("Edit Prompts %d/%d", index+1, len(queue))
		}
	}

	label := tview.NewTextView().SetText("Prompt").SetWrap(false)
	label.SetTextColor(views.ColorLabel)
	label.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	textArea := tview.NewTextArea()
	textArea.SetTextStyle(tcell.StyleDefault.
		Background(tview.Styles.ContrastBackgroundColor).
		Foreground(tview.Styles.PrimaryTextColor))

	status := newDropDown("Status", sessionStatusLabels, int(state.status))
	status.SetLabelWidth(promptSideWidth + 1)

	clearButton := tview.NewButton("Clear")
	copyButton := tview.NewButton("Copy")
	pasteButton := tview.NewButton("Paste")
	sessionButton := tview.NewButton("Session")
	savePauseButton := tview.NewButton("Save & Pause")
	saveButton := tview.NewButton("Save")
	cancelButton := tview.NewButton("Cancel")
	var deleteButton *tview.Button
	var list *tview.List
	listHeight := 0
	fieldHeight := promptAddFieldHeight
	if state.mode == PromptFormEdit {
		list = tview.NewList().ShowSecondaryText(false)
		list.SetBorder(true)
		list.SetTitle(" Prompts ")
		list.SetSelectedFocusOnly(true)
		deleteButton = tview.NewButton("Delete")
		rows := min(len(queue), promptListMaxRows)
		listHeight = max(rows, 1) + promptListChrome
		fieldHeight = max(promptModalTotalHeight-9-listHeight, promptMinFieldHeight)
	}

	e := &promptEditor{
		app:             app,
		label:           label,
		textArea:        textArea,
		status:          status,
		list:            list,
		clearButton:     clearButton,
		copyButton:      copyButton,
		pasteButton:     pasteButton,
		sessionButton:   sessionButton,
		savePauseButton: savePauseButton,
		saveButton:      saveButton,
		cancelButton:    cancelButton,
		deleteButton:    deleteButton,
		email:           session.AccountKey().Email,
		vendor:          session.AccountKey().Vendor,
		sessionID:       session.ID(),
		mode:            state.mode,
		index:           index,
		queue:           queue,
		onSaved:         state.onSaved,
		onClose:         state.onClose,
		onSaveAndPause:  state.onSaveAndPause,
		listHeight:      listHeight,
		fieldHeight:     fieldHeight,
	}
	if list != nil {
		e.focusables = []tview.Primitive{status, sessionButton, list, clearButton, copyButton, pasteButton, textArea, savePauseButton, saveButton, cancelButton, deleteButton}
	} else {
		e.focusables = []tview.Primitive{status, sessionButton, clearButton, copyButton, pasteButton, textArea, savePauseButton, saveButton, cancelButton}
	}

	buttonWidth := max(tview.TaggedStringWidth("Clear"), tview.TaggedStringWidth("Copy"), tview.TaggedStringWidth("Paste")) + 4
	side := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(label, 1, 0, false).
		AddItem(nil, e.fieldHeight-6, 0, false).
		AddItem(clearButton, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(copyButton, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(pasteButton, 1, 0, false)

	editorRow := tview.NewFlex().
		AddItem(side, buttonWidth, 0, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(textArea, 0, 1, true)

	sessionRow := tview.NewFlex().
		AddItem(status, promptSideWidth+1+status.GetFieldWidth(), 0, false).
		AddItem(nil, 0, 1, false).
		AddItem(sessionButton, tview.TaggedStringWidth("Session")+4, 0, false)

	savePauseWidth := tview.TaggedStringWidth("Save & Pause") + 4
	saveWidth := tview.TaggedStringWidth("Save") + 4
	cancelWidth := tview.TaggedStringWidth("Cancel") + 4
	buttonRow := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(savePauseButton, savePauseWidth, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(saveButton, saveWidth, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(cancelButton, cancelWidth, 0, false)
	if deleteButton != nil {
		buttonRow.AddItem(nil, 1, 0, false).
			AddItem(deleteButton, tview.TaggedStringWidth("Delete")+4, 0, false)
	}
	buttonRow.AddItem(nil, 0, 1, false)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(sessionRow, 1, 0, false).
		AddItem(nil, 1, 0, false)
	if list != nil {
		flex.AddItem(list, e.listHeight, 0, false).
			AddItem(nil, 1, 0, false)
	}
	flex.AddItem(editorRow, e.fieldHeight, 0, true).
		AddItem(nil, 1, 0, false).
		AddItem(buttonRow, 1, 0, false)
	flex.SetBorder(true).SetBorderPadding(1, 1, 1, 1).SetTitle(" " + title + " ")
	flex.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	e.Flex = flex

	clearButton.SetSelectedFunc(e.clear)
	copyButton.SetSelectedFunc(e.copy)
	pasteButton.SetSelectedFunc(e.paste)
	sessionButton.SetSelectedFunc(e.openSession)
	savePauseButton.SetSelectedFunc(e.saveAndPause)
	saveButton.SetSelectedFunc(e.save)
	cancelButton.SetSelectedFunc(e.onClose)
	if deleteButton != nil {
		deleteButton.SetSelectedFunc(e.confirmDeleteCurrent)
	}
	e.installInputCapture()
	e.installList()
	e.rebuildList()
	e.textArea.SetChangedFunc(e.syncDraft)
	e.textArea.SetText(state.text, false)
	return e
}

// Focus delegates focus to the prompt field.
func (e *promptEditor) Focus(delegate func(p tview.Primitive)) {
	delegate(e.textArea)
}

// HasFocus reports whether any prompt editor child has focus.
func (e *promptEditor) HasFocus() bool {
	for _, p := range e.focusables {
		if p.HasFocus() {
			return true
		}
	}
	return false
}

func (e *promptEditor) statusValue() model.SessionStatus {
	index, _ := e.status.GetCurrentOption()
	if index < 0 || index >= len(sessionStatusLabels) {
		return model.SessionStatusActive
	}
	return model.SessionStatus(index)
}

func (e *promptEditor) installInputCapture() {
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
		case *tview.Checkbox:
			v.SetInputCapture(capture)
		case *tview.DropDown:
			v.SetInputCapture(capture)
		}
	}
}

func (e *promptEditor) installList() {
	if e.list == nil {
		return
	}
	e.list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if e.refreshing {
			return
		}
		e.selectPrompt(index)
	})
	e.list.SetInputCapture(e.listCapture)
	e.list.SetFocusFunc(func() {
		e.list.SetBorderColor(tcell.ColorYellow)
		e.app.SetHints(views.HintsPromptListSpecific, views.HintsFormCommon)
	})
	e.list.SetBlurFunc(func() {
		e.list.SetBorderColor(tview.Styles.BorderColor)
		e.app.SetHints(views.HintsFormSpecific, views.HintsTextFormCommon)
	})
	views.WireListDoubleClick(e.list, func() {
		e.app.SetFocus(e.textArea)
	})
}

func (e *promptEditor) listCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyTab:
		e.moveFocus(1)
		return nil
	case tcell.KeyBacktab:
		e.moveFocus(-1)
		return nil
	case tcell.KeyEnter:
		e.app.SetFocus(e.textArea)
		return nil
	case views.KeyInsert:
		e.addPrompt()
		return nil
	case views.KeyDelete:
		e.deletePrompt()
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

func (e *promptEditor) selectPrompt(index int) {
	if e.mode != PromptFormEdit || index < 0 || index >= len(e.queue) {
		return
	}
	e.syncDraft()
	if index == e.index {
		return
	}
	e.index = index
	e.textArea.SetText(e.queue[index].Text(), true)
	e.updateTitle()
}

func (e *promptEditor) syncDraft() {
	if e.mode != PromptFormEdit || e.index < 0 || e.index >= len(e.queue) {
		return
	}
	e.queue[e.index] = model.NewPrompt(e.textArea.GetText())
	e.syncListItem()
}

func (e *promptEditor) syncListItem() {
	if e.list == nil || e.index < 0 || e.index >= len(e.queue) {
		return
	}
	e.list.SetItemText(e.index, promptListItem(e.index, e.queue[e.index].Text()), "")
}

func (e *promptEditor) rebuildList() {
	if e.list == nil {
		return
	}
	e.refreshing = true
	e.list.Clear()
	for i, p := range e.queue {
		e.list.AddItem(promptListItem(i, p.Text()), "", 0, nil)
	}
	switch {
	case len(e.queue) == 0:
		e.index = -1
	default:
		if e.index < 0 || e.index >= len(e.queue) {
			e.index = 0
		}
		e.list.SetCurrentItem(e.index)
	}
	e.refreshing = false
	e.updateTitle()
	e.updateDeleteButton()
}

func (e *promptEditor) addPrompt() {
	if e.mode != PromptFormEdit {
		return
	}
	e.syncDraft()
	index := e.index + 1
	if e.index < 0 || e.index >= len(e.queue) {
		index = len(e.queue)
	}
	e.queue = slices.Insert(e.queue, index, model.NewPrompt(""))
	e.index = index
	e.rebuildList()
	e.textArea.SetText("", true)
	e.app.SetFocus(e.textArea)
}

func (e *promptEditor) updateDeleteButton() {
	if e.deleteButton == nil {
		return
	}
	e.deleteButton.SetDisabled(len(e.queue) == 0)
}

func (e *promptEditor) confirmDeleteCurrent() {
	if e.mode != PromptFormEdit || e.index < 0 || e.index >= len(e.queue) {
		return
	}
	e.syncDraft()
	modal := views.NewConfirmModal(
		deletePromptMessage(e.index, len(e.queue), e.queue[e.index].Text()),
		e.deleteCurrentAndSave,
		e.app.HideModal,
	)
	views.ShowModal(e.app, modal, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
}

func (e *promptEditor) deleteCurrentAndSave() {
	e.syncDraft()
	if e.index < 0 || e.index >= len(e.queue) {
		e.app.HideModal()
		return
	}
	next := slices.Delete(slices.Clone(e.queue), e.index, e.index+1)
	if err := e.mutateQueue(func([]model.Prompt) ([]model.Prompt, error) {
		return next, nil
	}); err != nil {
		e.app.HideModal()
		views.SetErrorStatus(e.app, err)
		return
	}
	e.app.HideModal()
	if len(next) == 0 {
		e.onClose()
		return
	}
	live := e.app.Service().Session(e.sessionID)
	if live == nil {
		e.onClose()
		return
	}
	index := e.index
	if index >= len(next) {
		index = len(next) - 1
	}
	nextEditor := newPromptEditor(e.app, live, PromptFormOptions{Mode: PromptFormEdit, Index: index, OnSaved: e.onSaved, OnClose: e.onClose, OnSaveAndPause: e.onSaveAndPause})
	if err := e.app.ReplaceTopModal(nextEditor.primitive()); err != nil {
		e.app.SetError(err.Error())
		return
	}
	if nextEditor.list != nil {
		e.app.SetFocus(nextEditor.list)
	}
}

func (e *promptEditor) deletePrompt() {
	if e.mode != PromptFormEdit || e.index < 0 || e.index >= len(e.queue) {
		return
	}
	state := e.snapshot(true)
	modal := views.NewConfirmModal(
		deletePromptMessage(e.index, len(e.queue), e.queue[e.index].Text()),
		func() { e.removePrompt(&state) },
		e.app.HideModal,
	)
	views.ShowModal(e.app, modal, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
}

func (e *promptEditor) removePrompt(state *promptEditorState) {
	if state.index < 0 || state.index >= len(state.queue) {
		e.showState(state)
		return
	}
	state.queue = slices.Delete(slices.Clone(state.queue), state.index, state.index+1)
	if len(state.queue) == 0 {
		state.index = -1
		state.text = ""
	} else {
		if state.index >= len(state.queue) {
			state.index = len(state.queue) - 1
		}
		state.text = state.queue[state.index].Text()
	}
	e.showState(state)
}

func (e *promptEditor) showState(state *promptEditorState) {
	e.app.HideModal()
	live := e.app.Service().Session(e.sessionID)
	if live == nil {
		e.app.SetError("session no longer exists")
		return
	}
	next := newPromptEditorFromState(e.app, live, state)
	if err := e.app.ReplaceTopModal(next.primitive()); err != nil {
		e.app.SetError(err.Error())
		return
	}
	if state.focusList && next.list != nil {
		e.app.SetFocus(next.list)
	}
}

func (e *promptEditor) snapshot(focusList bool) promptEditorState {
	return promptEditorState{
		mode:           e.mode,
		index:          e.index,
		text:           e.textArea.GetText(),
		status:         e.statusValue(),
		queue:          slices.Clone(e.queue),
		focusList:      focusList,
		onSaved:        e.onSaved,
		onClose:        e.onClose,
		onSaveAndPause: e.onSaveAndPause,
	}
}

func (e *promptEditor) updateTitle() {
	if e.mode != PromptFormEdit {
		return
	}
	title := "Edit Prompts"
	if e.index >= 0 && e.index < len(e.queue) {
		title = fmt.Sprintf("Edit Prompts %d/%d", e.index+1, len(e.queue))
	}
	e.SetTitle(" " + title + " ")
}

func promptListItem(index int, text string) string {
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	return fmt.Sprintf("%d. %s", index+1, tview.Escape(text))
}

func deletePromptMessage(index, total int, text string) string {
	return fmt.Sprintf("Delete prompt %d/%d?\n%q?", index+1, total, tview.Escape(promptPreview(text)))
}

func promptPreview(text string) string {
	line := text
	if cut := strings.IndexAny(line, "\r\n"); cut >= 0 {
		line = line[:cut]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		line = "(empty)"
	}
	return views.TruncateWidth(line, 40)
}

func (e *promptEditor) moveFocus(delta int) {
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

func (e *promptEditor) primitive() tview.Primitive {
	frame := tview.NewFrame(e)
	frame.SetBorders(0, 0, 0, 0, 0, 0)
	frame.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	return views.NewModalState(views.CenteredModal(frame, promptModalWidth, e.modalHeight()), e)
}

func (e *promptEditor) modalHeight() int {
	return promptModalTotalHeight
}

func (e *promptEditor) clear() {
	e.textArea.SetText("", false)
	e.app.SetFocus(e.textArea)
}

func (e *promptEditor) copy() {
	copyTextArea(e.app, e.textArea)
}

func (e *promptEditor) paste() {
	pasteFromClipboard(e.app, e.textArea)
}

func (e *promptEditor) openSession() {
	live := e.app.Service().Session(e.sessionID)
	if live == nil {
		e.app.SetError("session no longer exists")
		return
	}
	account, err := e.app.Service().Account(live.AccountKey().Email, live.AccountKey().Vendor)
	if err != nil {
		e.app.SetError(err.Error())
		return
	}
	values := service.SessionFields{
		Name:        live.Name(),
		URL:         live.URL(),
		ProjectName: live.ProjectName(),
	}
	onSubmit := func(fields service.SessionFields) error {
		if err := e.app.Mutate(func(svc *service.Service) error {
			return svc.UpdateSession(live.AccountKey().Email, live.AccountKey().Vendor, live.ID(), fields)
		}); err != nil {
			return fmt.Errorf("update session: %w", err)
		}
		e.app.HideModal()
		return nil
	}
	views.ShowModal(e.app, NewSessionForm(e.app, &SessionFormOptions{
		Title:       "Edit Session",
		SubmitLabel: "Save",
		Account:     account,
		Values:      values,
		OnSubmit:    onSubmit,
		OnCancel:    e.app.HideModal,
	}), views.HintsFormSpecific, views.HintsTextFormCommon)
}

func (e *promptEditor) save() {
	if e.savePrompt() {
		e.onClose()
	}
}

func (e *promptEditor) saveAndPause() {
	if !e.savePrompt() {
		return
	}
	e.onClose()
	if e.onSaveAndPause != nil {
		e.onSaveAndPause()
	}
}

func (e *promptEditor) savePrompt() bool {
	if e.mode == PromptFormAdd {
		text := model.NormalizeText(e.textArea.GetText())
		if text == "" {
			e.app.SetError(model.ErrTextRequired.Error())
			return false
		}
		if err := e.mutateQueue(func(queue []model.Prompt) ([]model.Prompt, error) {
			return append(slices.Clone(queue), model.NewPrompt(text)), nil
		}); err != nil {
			views.SetErrorStatus(e.app, err)
			return false
		}
		e.saved()
		return true
	}

	e.syncDraft()
	next := slices.Clone(e.queue)
	if len(next) == 0 {
		if text := model.NormalizeText(e.textArea.GetText()); text != "" {
			next = []model.Prompt{model.NewPrompt(text)}
		}
	}
	if err := e.mutateQueue(func([]model.Prompt) ([]model.Prompt, error) {
		return next, nil
	}); err != nil {
		views.SetErrorStatus(e.app, err)
		return false
	}
	e.saved()
	return true
}

func (e *promptEditor) saved() {
	if e.onSaved != nil {
		e.onSaved()
	}
}

func (e *promptEditor) mutateQueue(change func([]model.Prompt) ([]model.Prompt, error)) error {
	if err := e.app.Mutate(func(svc *service.Service) error {
		live := svc.Session(e.sessionID)
		if live == nil {
			return model.ErrSessionNotFound
		}
		next, err := change(live.Queue())
		if err != nil {
			return err
		}
		if err := svc.UpdateSessionPrompts(e.email, e.vendor, e.sessionID, next); err != nil {
			return err
		}
		live = svc.Session(e.sessionID)
		if live == nil {
			return model.ErrSessionNotFound
		}
		if live.Status() != e.statusValue() {
			return svc.UpdateSessionStatus(e.email, e.vendor, e.sessionID, e.statusValue())
		}
		return nil
	}); err != nil {
		return fmt.Errorf("mutate prompt: %w", err)
	}
	return nil
}
