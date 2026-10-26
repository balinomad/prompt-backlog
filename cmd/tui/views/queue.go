package views

import (
	"fmt"
	"slices"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// queueEditor holds the mutable state behind one NewQueueEditor view: the
// live queue snapshot and the list rendering it. Key handling is
// implemented as methods, not a closure passed to SetInputCapture, so
// each case is scored for cyclomatic complexity on its own rather than
// folded into NewQueueEditor.
type queueEditor struct {
	app      App
	account  *model.Account
	session  *model.Session
	onClose  func()
	email    string
	provider string
	id       uint64
	queue    []model.Prompt
	list     *tview.List
}

// NewQueueEditor returns a view for managing a session's prompt queue:
// listing, adding, editing, deleting, and reordering prompts. Every action
// commits immediately via Service.UpdateSessionPrompts. The session is
// re-fetched on every (re)construction because updates replace it via
// Copy-on-Write. Sub-actions use App.ShowModal/HideModal so focus returns
// to the editor when they close.
func NewQueueEditor(app App, account *model.Account, session *model.Session, onClose func()) tview.Primitive {
	if account == nil || session == nil {
		return errorModal("No session selected.", onClose)
	}

	id := session.ID()
	live := app.Service().Session(id)
	if live == nil {
		return errorModal("Session no longer exists.", onClose)
	}

	qe := &queueEditor{
		app:      app,
		account:  account,
		session:  session,
		onClose:  onClose,
		email:    account.UserEmail(),
		provider: account.ProviderName(),
		id:       id,
		queue:    live.Queue(),
	}

	qe.list = tview.NewList().ShowSecondaryText(false)
	qe.list.SetBorder(true)
	qe.list.SetTitle(fmt.Sprintf(" Queue: session #%d ", id))
	qe.rebuildList()
	qe.list.SetInputCapture(qe.handleKey)

	return qe.list
}

// rebuildList repopulates qe.list from qe.queue.
func (qe *queueEditor) rebuildList() {
	qe.list.Clear()
	for _, p := range qe.queue {
		qe.list.AddItem(promptItemText(p), "", 0, nil)
	}
}

// reopen returns to a freshly constructed queue editor.
func (qe *queueEditor) reopen() {
	qe.app.HideModal()
	showModal(qe.app, NewQueueEditor(qe.app, qe.account, qe.session, qe.onClose), HintsQueueEditorSpecific, HintsQueueEditorCommon)
}

// commit persists newQueue and reports success. Callers that cannot introduce
// a new validation failure may ignore the result; callers that accept new
// free-text input must check it.
func (qe *queueEditor) commit(newQueue []model.Prompt) bool {
	if err := qe.app.Mutate(func(s *service.Service) error {
		return s.UpdateSessionPrompts(qe.email, qe.provider, qe.id, newQueue)
	}); err != nil {
		setErrorStatus(qe.app, err)
		return false
	}
	return true
}

func (qe *queueEditor) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case KeyCancel:
		qe.onClose()
		return nil
	case KeyInsert:
		return qe.handleNew()
	case KeyEdit:
		return qe.handleEdit()
	case KeyDelete:
		return qe.handleDelete()
	case KeyFocusNext, KeyFocusPrev:
		// The queue editor is a single-pane modal; Tab must not move focus
		// into the editor's underlying list or form fields.
		return nil
	case tcell.KeyUp:
		if event.Modifiers()&tcell.ModCtrl == 0 {
			return event
		}
		return qe.handleMoveUp()
	case tcell.KeyDown:
		if event.Modifiers()&tcell.ModCtrl == 0 {
			return event
		}
		return qe.handleMoveDown()
	default:
	}
	if event.Key() != tcell.KeyRune {
		return event
	}

	switch event.Rune() {
	case KeyHelp:
		return qe.handleHelp()
	case KeyQuit:
		return qe.handleQuit()
	}
	return event
}

func (qe *queueEditor) handleNew() *tcell.EventKey {
	modal := newPromptForm(qe.app, qe.queue, -1, qe.commit, qe.reopen)
	qe.app.HideModal()
	showModal(qe.app, modal, HintsFormSpecific, HintsFormCommon)
	return nil
}

func (qe *queueEditor) handleEdit() *tcell.EventKey {
	idx := qe.list.GetCurrentItem()
	if idx < 0 || idx >= len(qe.queue) {
		return nil
	}
	modal := newPromptForm(qe.app, qe.queue, idx, qe.commit, qe.reopen)
	qe.app.HideModal()
	showModal(qe.app, modal, HintsFormSpecific, HintsFormCommon)
	return nil
}

func (qe *queueEditor) handleDelete() *tcell.EventKey {
	idx := qe.list.GetCurrentItem()
	if idx < 0 || idx >= len(qe.queue) {
		return nil
	}
	label := qe.queue[idx].Text()
	modal := NewConfirmModal(
		fmt.Sprintf("Delete prompt\n%q?", tview.Escape(label)),
		func() {
			qe.commit(slices.Delete(slices.Clone(qe.queue), idx, idx+1))
			qe.reopen()
		},
		qe.reopen,
	)
	qe.app.HideModal()
	showModal(qe.app, modal, HintsConfirmModalSpecific, HintsConfirmModalCommon)
	return nil
}

func (qe *queueEditor) handleMoveUp() *tcell.EventKey {
	idx := qe.list.GetCurrentItem()
	if idx <= 0 || idx >= len(qe.queue) {
		return nil
	}
	newQueue := slices.Clone(qe.queue)
	newQueue[idx-1], newQueue[idx] = newQueue[idx], newQueue[idx-1]
	if qe.commit(newQueue) {
		qe.queue = newQueue
		qe.rebuildList()
		qe.list.SetCurrentItem(idx - 1)
	}
	return nil
}

func (qe *queueEditor) handleMoveDown() *tcell.EventKey {
	idx := qe.list.GetCurrentItem()
	if idx < 0 || idx >= len(qe.queue)-1 {
		return nil
	}
	newQueue := slices.Clone(qe.queue)
	newQueue[idx], newQueue[idx+1] = newQueue[idx+1], newQueue[idx]
	if qe.commit(newQueue) {
		qe.queue = newQueue
		qe.rebuildList()
		qe.list.SetCurrentItem(idx + 1)
	}
	return nil
}

// handleHelp opens the keyboard reference overlay and reopens the editor when
// the overlay closes.
func (qe *queueEditor) handleHelp() *tcell.EventKey {
	qe.app.HideModal()
	showModal(qe.app, NewHelpOverlay(qe.reopen), HintsHelpOverlaySpecific, HintsHelpOverlayCommon)
	return nil
}

// handleQuit opens the quit confirmation and reopens the editor on cancel.
func (qe *queueEditor) handleQuit() *tcell.EventKey {
	modal := NewConfirmModal(
		"Quit "+qe.app.AppName()+"?",
		func() { qe.app.Quit() },
		qe.reopen,
	)
	qe.app.HideModal()
	showModal(qe.app, modal, HintsConfirmModalSpecific, HintsConfirmModalCommon)
	return nil
}

// newPromptForm returns a small form for adding (idx == -1) or editing
// (idx >= 0) a single prompt in queue. commit persists the resulting queue;
// onDone is called after a successful commit and on cancel.
func newPromptForm(app App, queue []model.Prompt, idx int, commit func([]model.Prompt) bool, onDone func()) tview.Primitive {
	isNew := idx < 0

	initText, initSpawn := "", false
	if !isNew {
		initText = queue[idx].Text()
		initSpawn = queue[idx].SpawnsSession()
	}

	title := "New Prompt"
	if !isNew {
		title = "Edit Prompt"
	}
	form := newForm(title)

	form.AddInputField("Text", initText, formFieldWidth, nil, nil)
	form.AddCheckbox("Spawns new session", initSpawn, nil)

	save := func() {
		textField, ok := form.GetFormItemByLabel("Text").(*tview.InputField)
		if !ok {
			app.SetStatus("[red]internal error: Text field missing[-]")
			return
		}
		textVal := strings.TrimSpace(textField.GetText())

		spawnCheckbox, ok := form.GetFormItemByLabel("Spawns new session").(*tview.Checkbox)
		if !ok {
			app.SetStatus("[red]internal error: Spawns new session checkbox missing[-]")
			return
		}
		spawnVal := spawnCheckbox.IsChecked()

		newPrompt := model.NewPrompt(textVal, spawnVal)
		newQueue := slices.Clone(queue)
		if isNew {
			newQueue = append(newQueue, newPrompt)
		} else {
			newQueue[idx] = newPrompt
		}

		if !commit(newQueue) {
			return
		}
		onDone()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onDone)
	form.SetCancelFunc(onDone)
	addCtrlS(form, save)

	return formModal(form)
}

// promptItemText formats a prompt row for the queue list.
func promptItemText(p model.Prompt) string {
	text := tview.Escape(p.Text())
	if p.SpawnsSession() {
		text += "  [cyan]→ new session[-]"
	}
	return insetListRow(text)
}
