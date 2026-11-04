package forms

import (
	"github.com/balinomad/llm-desk/cmd/tui/views"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/rivo/tview"
)

// NewNoteForm returns a note form. On the main screen account is the account
// in view and fixedProject is nil. On the Projects screen fixedProject is the
// selected project, the Project dropdown is disabled, and account is ignored.
//
//nolint:cyclop // Form construction has many independent UI branches.
func NewNoteForm(app views.Host, note *model.Note, account *model.Account, fixedProject *model.Project, onClose func()) tview.Primitive {
	svc := app.Service()
	isEdit := note != nil
	if isEdit {
		live := svc.Note(note.ID())
		if live == nil {
			return views.ErrorModal("Note no longer exists.", onClose)
		}
		note = live
	}

	projectNames := []string{"none"}
	projectIndex := 0
	if fixedProject != nil {
		projectNames = []string{fixedProject.Name()}
	} else {
		projects := svc.Projects()
		for _, p := range projects {
			projectNames = append(projectNames, p.Name())
		}
		if isEdit && note.ProjectID() != 0 {
			for i, p := range projects {
				if p.ID() == note.ProjectID() {
					projectIndex = i + 1
					break
				}
			}
		}
	}

	accounts := noteAccountOptions(svc, account, fixedProject)
	accountLabels := make([]string, 0, len(accounts)+1)
	accountLabels = append(accountLabels, "none")
	for _, acc := range accounts {
		accountLabels = append(accountLabels, accountDisplayName(svc, acc))
	}
	accountIndex := 0
	if isEdit {
		if key, ok := note.AccountKey(); ok {
			for i, acc := range accounts {
				if acc.Key() == key {
					accountIndex = i + 1
					break
				}
			}
		}
	} else if fixedProject == nil && account != nil {
		accountIndex = 1
	}

	title := "New Note"
	initialText := ""
	if isEdit {
		title = "Edit Note"
		initialText = note.Text()
	}
	form := newForm(title)
	form.AddTextArea("Text", initialText, noteFieldWidth, noteTextHeight, 0, nil)
	addDropDown(form, "Project", escapeAll(projectNames), projectIndex)
	addDropDown(form, "Account", escapeAll(accountLabels), accountIndex)
	if fixedProject != nil {
		if projectDropDown, ok := form.GetFormItemByLabel("Project").(*tview.DropDown); ok {
			projectDropDown.SetDisabled(true)
		}
	}

	save := func() {
		textArea, ok := form.GetFormItemByLabel("Text").(*tview.TextArea)
		if !ok {
			app.SetError("internal error: Text field missing")
			return
		}
		projectDropDown, ok := form.GetFormItemByLabel("Project").(*tview.DropDown)
		if !ok {
			app.SetError("internal error: Project dropdown missing")
			return
		}
		accountDropDown, ok := form.GetFormItemByLabel("Account").(*tview.DropDown)
		if !ok {
			app.SetError("internal error: Account dropdown missing")
			return
		}

		projectName := ""
		if fixedProject != nil {
			projectName = fixedProject.Name()
		} else {
			projectIdx, _ := projectDropDown.GetCurrentOption()
			if projectIdx > 0 && projectIdx < len(projectNames) {
				projectName = projectNames[projectIdx]
			}
		}

		var selected *model.Account
		accountIdx, _ := accountDropDown.GetCurrentOption()
		if accountIdx > 0 && accountIdx <= len(accounts) {
			selected = accounts[accountIdx-1]
		}
		email, vendor := "", ""
		if selected != nil {
			email = selected.UserEmail()
			vendor = selected.VendorName()
		}

		text := textArea.GetText()
		noteID := uint64(0)
		if isEdit {
			noteID = note.ID()
		}
		if err := app.Mutate(func(s *service.Service) error {
			if !isEdit {
				_, err := s.AddNote(text, projectName, email, vendor)
				return err
			}
			return s.UpdateNote(noteID, text, projectName, email, vendor)
		}); err != nil {
			views.SetErrorStatus(app, err)
			return
		}
		onClose()
	}

	return finishFormWithMax(form, save, onClose, noteFormMaxWidth)
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
