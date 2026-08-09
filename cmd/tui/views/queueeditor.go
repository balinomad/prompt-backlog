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
// commits immediately via Service.UpdateSessionPrompts — there is no
// separate save step for the editor as a whole — matching the
// per-action-commit convention already used elsewhere in this package
// (e.g. DetailPanel's KeyToggle handler).
//
// The session is re-fetched fresh from the Service on every (re)construction
// of this view rather than trusting the passed-in pointer's own field
// values: UpdateSessionPrompts replaces the session via Copy-on-Write, so
// the caller's pointer goes stale the instant any action commits.
//
// Every sub-action (add, edit, delete) transitions through
// App.ShowModal/HideModal rather than an internal tview.Pages page switch.
// App's modal system is proven correct for moving keyboard focus — see
// ShowModal's implementation and TestApp_ShowModal_Success — whereas a
// hand-rolled internal page switch inside one returned primitive has no
// such guarantee available to verify in this codebase, and getting focus
// routing wrong would silently swallow keystrokes rather than fail loudly.
// Every transition pairs a HideModal immediately before the next ShowModal,
// so App's single prevFocus slot always re-anchors to the true pre-editor
// focus (e.g. DetailPanel's session list) instead of accumulating a chain
// of intermediate modals, which would restore focus to the wrong place
// when the operator finally closes the editor.
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

// reopen returns to a freshly (re)constructed queue editor, re-fetching
// the session so a just-committed change is reflected. See NewQueueEditor's
// doc comment for why this goes through HideModal+ShowModal rather than an
// in-place refresh.
func (qe *queueEditor) reopen() {
	qe.app.HideModal()
	if err := qe.app.ShowModal(NewQueueEditor(qe.app, qe.account, qe.session, qe.onClose)); err != nil {
		qe.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return
	}
	qe.app.SetHints(HintsQueueEditor)
}

// commit persists newQueue and reports success. Callers that cannot
// introduce a new validation failure by construction — delete removes an
// already-valid entry; reorder only permutes already-valid entries — may
// ignore the result, since there is nothing for the operator to fix.
// Callers that accept new free-text input (add/edit) must check it, to
// keep the form open for a retry on failure.
func (qe *queueEditor) commit(newQueue []model.Prompt) bool {
	if err := qe.app.Mutate(func(s *service.Service) error {
		return s.UpdateSessionPrompts(qe.email, qe.provider, qe.id, newQueue)
	}); err != nil {
		qe.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return false
	}
	return true
}

func (qe *queueEditor) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == KeyCancel {
		qe.onClose()
		return nil
	}
	if event.Key() != tcell.KeyRune {
		return event
	}

	switch event.Rune() {
	case KeyNew:
		return qe.handleNew()
	case KeyEdit:
		return qe.handleEdit()
	case KeyDelete:
		return qe.handleDelete()
	case KeyMoveUp:
		return qe.handleMoveUp()
	case KeyMoveDown:
		return qe.handleMoveDown()
	}
	return event
}

func (qe *queueEditor) handleNew() *tcell.EventKey {
	modal := newPromptForm(qe.app, qe.queue, -1, qe.commit, qe.reopen)
	qe.app.HideModal()
	if err := qe.app.ShowModal(modal); err != nil {
		qe.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	qe.app.SetHints(HintsForm)
	return nil
}

func (qe *queueEditor) handleEdit() *tcell.EventKey {
	idx := qe.list.GetCurrentItem()
	if idx < 0 || idx >= len(qe.queue) {
		return nil
	}
	modal := newPromptForm(qe.app, qe.queue, idx, qe.commit, qe.reopen)
	qe.app.HideModal()
	if err := qe.app.ShowModal(modal); err != nil {
		qe.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	qe.app.SetHints(HintsForm)
	return nil
}

func (qe *queueEditor) handleDelete() *tcell.EventKey {
	idx := qe.list.GetCurrentItem()
	if idx < 0 || idx >= len(qe.queue) {
		return nil
	}
	label := qe.queue[idx].Text()
	modal := NewConfirmModal(
		fmt.Sprintf("Delete prompt\n%q?", label),
		func() {
			qe.commit(slices.Delete(slices.Clone(qe.queue), idx, idx+1))
			qe.reopen()
		},
		qe.reopen,
	)
	qe.app.HideModal()
	if err := qe.app.ShowModal(modal); err != nil {
		qe.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	qe.app.SetHints(HintsConfirmModal)
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

// newPromptForm returns a small form for adding (idx == -1) or editing
// (idx >= 0) a single prompt in queue. commit persists the resulting
// queue; onDone is called after a successful commit and also on
// Cancel/Esc without committing — this form never returns to itself, it
// always hands control back to the queue list via onDone, matching
// NewQueueEditor's ShowModal/HideModal transition pattern.
func newPromptForm(app App, queue []model.Prompt, idx int, commit func([]model.Prompt) bool, onDone func()) tview.Primitive {
	isNew := idx < 0

	initText, initSpawn := "", false
	if !isNew {
		initText = queue[idx].Text()
		initSpawn = queue[idx].SpawnsSession()
	}

	form := tview.NewForm()
	form.SetBorder(true)
	if isNew {
		form.SetTitle(" New Prompt ")
	} else {
		form.SetTitle(" Edit Prompt ")
	}

	form.AddInputField("Text", initText, 40, nil, nil)
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

	return centeredModal(form, 11)
}

// promptItemText formats a prompt row for the queue list.
func promptItemText(p model.Prompt) string {
	if p.SpawnsSession() {
		return p.Text() + "  [cyan]→ new session[-]"
	}
	return p.Text()
}
