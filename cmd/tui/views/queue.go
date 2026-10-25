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
	qe.app.SetHints(HintsQueueEditorSpecific, HintsQueueEditorCommon)
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
		// Every other special key falls through to the rune-only guard
		// and switch below.
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
	if err := qe.app.ShowModal(modal); err != nil {
		qe.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	qe.app.SetHints(HintsFormSpecific, HintsFormCommon)
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
	qe.app.SetHints(HintsFormSpecific, HintsFormCommon)
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
	if err := qe.app.ShowModal(modal); err != nil {
		qe.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	qe.app.SetHints(HintsConfirmModalSpecific, HintsConfirmModalCommon)
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

// handleHelp opens the keyboard reference overlay. Not routed through the
// shared showHelp helper: this view is itself shown via App.ShowModal, so
// a second ShowModal call here would replace it on the "modal" page
// (Pages.AddPage replaces a same-named page), and showHelp's own close
// behaviour (a bare HideModal) would land on whatever was behind the
// editor, not the editor itself. qe.reopen reconstructs a fresh queue
// editor instead — the same pattern handleDelete already uses for its own
// nested confirm dialog.
func (qe *queueEditor) handleHelp() *tcell.EventKey {
	qe.app.HideModal()
	if err := qe.app.ShowModal(NewHelpOverlay(qe.reopen)); err != nil {
		qe.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	qe.app.SetHints(HintsHelpOverlaySpecific, HintsHelpOverlayCommon)
	return nil
}

// handleQuit opens the quit confirmation, built directly on
// NewConfirmModal rather than the shared confirmQuit helper for the same
// reason as handleHelp: onCancel must reopen the editor, not just hide it.
func (qe *queueEditor) handleQuit() *tcell.EventKey {
	modal := NewConfirmModal(
		"Quit "+qe.app.AppName()+"?",
		func() { qe.app.Quit() },
		qe.reopen,
	)
	qe.app.HideModal()
	if err := qe.app.ShowModal(modal); err != nil {
		qe.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	qe.app.SetHints(HintsConfirmModalSpecific, HintsConfirmModalCommon)
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
