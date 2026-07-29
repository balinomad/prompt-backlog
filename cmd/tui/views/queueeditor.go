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

	email := account.UserEmail()
	provider := account.ProviderName()
	id := session.ID()

	live := app.Service().Session(id)
	if live == nil {
		return errorModal("Session no longer exists.", onClose)
	}
	queue := live.Queue()

	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(fmt.Sprintf(" Queue: session #%d ", id))

	rebuildList := func() {
		list.Clear()
		for _, p := range queue {
			list.AddItem(promptItemText(p), "", 0, nil)
		}
	}
	rebuildList()

	// reopen returns to a freshly (re)constructed list view, re-fetching
	// the session so a just-committed change is reflected. See the
	// function doc for why this goes through HideModal+ShowModal rather
	// than an in-place refresh.
	reopen := func() {
		app.HideModal()
		if err := app.ShowModal(NewQueueEditor(app, account, session, onClose)); err != nil {
			app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return
		}
		app.SetHints(HintsQueueEditor)
	}

	// commit persists newQueue and reports success. Callers that cannot
	// introduce a new validation failure by construction — delete removes
	// an already-valid entry; reorder only permutes already-valid entries
	// — may ignore the result, since there is nothing for the operator to
	// fix. Callers that accept new free-text input (add/edit) must check
	// it, to keep the form open for a retry on failure.
	commit := func(newQueue []model.Prompt) bool {
		if err := app.Mutate(func(s *service.Service) error {
			return s.UpdateSessionPrompts(email, provider, id, newQueue)
		}); err != nil {
			app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return false
		}
		return true
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == KeyCancel {
			onClose()
			return nil
		}
		if event.Key() != tcell.KeyRune {
			return event
		}

		switch event.Rune() {
		case KeyNew:
			modal := newPromptForm(app, queue, -1, commit, reopen)
			app.HideModal()
			if err := app.ShowModal(modal); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return nil
			}
			app.SetHints(HintsForm)
			return nil

		case KeyEdit:
			idx := list.GetCurrentItem()
			if idx < 0 || idx >= len(queue) {
				return nil
			}
			modal := newPromptForm(app, queue, idx, commit, reopen)
			app.HideModal()
			if err := app.ShowModal(modal); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return nil
			}
			app.SetHints(HintsForm)
			return nil

		case KeyDelete:
			idx := list.GetCurrentItem()
			if idx < 0 || idx >= len(queue) {
				return nil
			}
			label := queue[idx].Text()
			modal := NewConfirmModal(
				fmt.Sprintf("Delete prompt\n%q?", label),
				func() {
					commit(slices.Delete(slices.Clone(queue), idx, idx+1))
					reopen()
				},
				reopen,
			)
			app.HideModal()
			if err := app.ShowModal(modal); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return nil
			}
			app.SetHints(HintsConfirmModal)
			return nil

		case KeyMoveUp:
			idx := list.GetCurrentItem()
			if idx <= 0 || idx >= len(queue) {
				return nil
			}
			newQueue := slices.Clone(queue)
			newQueue[idx-1], newQueue[idx] = newQueue[idx], newQueue[idx-1]
			if commit(newQueue) {
				queue = newQueue
				rebuildList()
				list.SetCurrentItem(idx - 1)
			}
			return nil

		case KeyMoveDown:
			idx := list.GetCurrentItem()
			if idx < 0 || idx >= len(queue)-1 {
				return nil
			}
			newQueue := slices.Clone(queue)
			newQueue[idx], newQueue[idx+1] = newQueue[idx+1], newQueue[idx]
			if commit(newQueue) {
				queue = newQueue
				rebuildList()
				list.SetCurrentItem(idx + 1)
			}
			return nil
		}
		return event
	})

	return list
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

	return centeredModal(form, 64, 11)
}

// promptItemText formats a prompt row for the queue list.
func promptItemText(p model.Prompt) string {
	if p.SpawnsSession() {
		return fmt.Sprintf("%s  [cyan]→ new session[-]", p.Text())
	}
	return p.Text()
}
