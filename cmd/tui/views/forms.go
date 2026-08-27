package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── Form helper ───────────────────────────────────────────────────────────────

// addCtrlS adds Ctrl+S as an additional save trigger to a form.
// The form's own button handles the primary click path; this handles keyboard.
func addCtrlS(form *tview.Form, save func()) {
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == KeySave {
			save()
			return nil
		}
		return event
	})
}

// escapeAll returns a copy of names with each entry passed through
// tview.Escape, for display in list-like widgets (dropdown options, list
// items) without free text being read as a color or region tag. Callers
// that also read a selection back must index into the original,
// unescaped slice — never re-parse the string a DropDown returns, since
// it reflects the escaped copy, not the source value.
func escapeAll(names []string) []string {
	escaped := make([]string, len(names))
	for i, n := range names {
		escaped[i] = tview.Escape(n)
	}
	return escaped
}

// ── Session form ──────────────────────────────────────────────────────────────

// NewSessionForm returns a form for creating or editing a session under
// account. Account is a structural prerequisite of Session — AddSession
// requires a live account — so the form always receives one directly
// rather than resolving it itself; the caller (DetailPanel) already holds
// it, and it is never created here. When session is nil a new session is
// created; otherwise the session is edited. Queue editing is out of scope
// for this form — see NewQueueEditor.
func NewSessionForm(app App, account *model.Account, session *model.Session, onClose func()) tview.Primitive {
	if account == nil {
		return errorModal("No account selected.", onClose)
	}

	email := account.UserEmail()
	provider := account.ProviderName()

	projectNames := sessionFormProjectNames(app, email, provider) // raw — see save() below
	isEdit := session != nil
	initTopic, initProjectIdx := sessionFormInitial(session, projectNames)

	form := tview.NewForm()
	form.SetBorder(true)
	if isEdit {
		form.SetTitle(fmt.Sprintf(" Edit Session: %s / %s ", tview.Escape(email), tview.Escape(provider)))
	} else {
		form.SetTitle(" New Session ")
	}

	form.AddDropDown("Project", escapeAll(projectNames), initProjectIdx, nil)
	form.AddInputField("Topic", initTopic, 40, nil, nil)

	save := func() {
		projectDropDown, ok := form.GetFormItemByLabel("Project").(*tview.DropDown)
		if !ok {
			app.SetStatus("[red]internal error: Project dropdown missing[-]")
			return
		}
		// Look up by index into the raw projectNames, not the string
		// GetCurrentOption returns — the dropdown displays the escaped
		// copy, which wouldn't resolve against the registry. idx 0 is
		// always "no project" (see sessionFormProjectNames).
		idx, _ := projectDropDown.GetCurrentOption()
		var projectName string
		if idx > 0 && idx < len(projectNames) {
			projectName = projectNames[idx]
		}

		topicField, ok := form.GetFormItemByLabel("Topic").(*tview.InputField)
		if !ok {
			app.SetStatus("[red]internal error: Topic field missing[-]")
			return
		}
		topicVal := strings.TrimSpace(topicField.GetText())

		if isEdit {
			id := session.ID()
			// Sanctioned multi-call batch: this edit touches two fields on
			// the same aggregate, committed as one save (Service.Save's
			// docstring permits this explicitly).
			if err := app.Mutate(func(s *service.Service) error {
				if err := s.UpdateSessionTopic(email, provider, id, topicVal); err != nil {
					return err
				}
				return s.UpdateSessionProject(email, provider, id, projectName)
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		} else {
			if err := app.Mutate(func(s *service.Service) error {
				// userName is unused: account is always already-resolved
				// here, so StartSession's on-demand user path never fires.
				_, err := s.StartSession(email, "", provider, projectName, topicVal, nil)
				return err
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 11)
}

// sessionFormProjectNames returns NewSessionForm's Project dropdown
// options: "none" followed by the account's granted projects. A
// resolution error degrades to "no granted projects" rather than
// blocking form construction, matching the read-degrades pattern already
// used in projects.go's renderInfo — account is already known valid.
func sessionFormProjectNames(app App, email, provider string) []string {
	granted, err := app.Service().AccountProjects(email, provider)
	if err != nil {
		granted = nil
	}
	names := make([]string, 0, len(granted)+1)
	names = append(names, "none")
	for _, p := range granted {
		names = append(names, p.Name())
	}
	return names
}

// sessionFormInitial returns the initial Topic text and Project dropdown
// index for editing session, matching session's project against
// projectNames by name. Returns ("", 0) for a nil session (create mode)
// or a session with no project assigned.
func sessionFormInitial(session *model.Session, projectNames []string) (string, int) {
	if session == nil {
		return "", 0
	}
	topic := session.Topic()
	if session.ProjectName() == "" {
		return topic, 0
	}
	// Start at 1 to skip index 0 ("none")
	for i := 1; i < len(projectNames); i++ {
		if projectNames[i] == session.ProjectName() {
			return topic, i
		}
	}
	return topic, 0
}

// ── Project form ──────────────────────────────────────────────────────────────

// NewProjectForm returns a form for creating or editing a project. Project
// has no structural prerequisites, so the form touches no other
// aggregate: no account or session parameter, and creation never grants
// the project to anything — granting is a separate action, see
// NewGrantForm. When project is nil a new project is created; otherwise
// the project is edited.
func NewProjectForm(app App, project *model.Project, onClose func()) tview.Primitive {
	isEdit := project != nil

	initName, initDesc, initInstr := "", "", ""
	if isEdit {
		initName = project.Name()
		initDesc = project.Description()
		initInstr = project.Instructions()
	}

	form := tview.NewForm()
	form.SetBorder(true)
	if isEdit {
		form.SetTitle(fmt.Sprintf(" Edit Project: %s ", tview.Escape(initName)))
	} else {
		form.SetTitle(" New Project ")
	}

	form.AddInputField("Name", initName, 40, nil, nil)
	form.AddInputField("Description", initDesc, 40, nil, nil)
	form.AddInputField("Instructions", initInstr, 40, nil, nil)

	if isEdit {
		nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
		if !ok {
			return errorModal("Internal error: failed to initialize form fields.", onClose)
		}
		nameField.SetDisabled(true)
	}

	save := func() {
		descField, ok := form.GetFormItemByLabel("Description").(*tview.InputField)
		if !ok {
			app.SetStatus("[red]internal error: Description field missing[-]")
			return
		}
		descVal := strings.TrimSpace(descField.GetText())

		instrField, ok := form.GetFormItemByLabel("Instructions").(*tview.InputField)
		if !ok {
			app.SetStatus("[red]internal error: Instructions field missing[-]")
			return
		}
		instrVal := strings.TrimSpace(instrField.GetText())

		if isEdit {
			name := initName
			if err := app.Mutate(func(s *service.Service) error {
				return s.UpdateProject(name, descVal, instrVal)
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		} else {
			nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
			if !ok {
				app.SetStatus("[red]internal error: Name field missing[-]")
				return
			}
			nameVal := strings.TrimSpace(nameField.GetText())

			if err := app.Service().ValidateProject(nameVal, descVal, instrVal); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
			if err := app.Mutate(func(s *service.Service) error {
				return s.AddProject(nameVal, descVal, instrVal)
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 13)
}

// ── Provider form ─────────────────────────────────────────────────────────────

// NewProviderForm returns a form for creating or editing a provider.
// When provider is nil a new provider is created; otherwise it is edited.
func NewProviderForm(app App, provider *model.Provider, onClose func()) tview.Primitive {
	isEdit := provider != nil

	initName := ""
	initHasProjects := false
	initHasDeepResearch := false
	if isEdit {
		initName = provider.Name()
		initHasProjects = provider.HasProjects()
		initHasDeepResearch = provider.HasDeepResearch()
	}

	form := tview.NewForm()
	form.SetBorder(true)
	if isEdit {
		form.SetTitle(fmt.Sprintf(" Edit Provider: %s ", tview.Escape(initName)))
	} else {
		form.SetTitle(" New Provider ")
	}

	form.AddInputField("Name", initName, 30, nil, nil)
	form.AddCheckbox("Supports projects", initHasProjects, nil)
	form.AddCheckbox("Supports deep research", initHasDeepResearch, nil)

	if isEdit {
		nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
		if !ok {
			return errorModal("Internal error: failed to initialize form fields.", onClose)
		}
		nameField.SetDisabled(true)
	}

	save := func() {
		projectCheckbox, ok := form.GetFormItemByLabel("Supports projects").(*tview.Checkbox)
		if !ok {
			app.SetStatus("[red]internal error: project checkbox missing[-]")
			return
		}
		hasProjects := projectCheckbox.IsChecked()

		researchCheckbox, ok := form.GetFormItemByLabel("Supports deep research").(*tview.Checkbox)
		if !ok {
			app.SetStatus("[red]internal error: deep research checkbox missing[-]")
			return
		}
		hasDeepResearch := researchCheckbox.IsChecked()

		if isEdit {
			name := initName
			if err := app.Mutate(func(s *service.Service) error {
				return s.UpdateProvider(name, hasProjects, hasDeepResearch)
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		} else {
			nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
			if !ok {
				app.SetStatus("[red]internal error: Name field missing[-]")
				return
			}
			nameVal := strings.TrimSpace(nameField.GetText())

			if err := app.Service().ValidateProvider(nameVal, hasProjects, hasDeepResearch); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
			if err := app.Mutate(func(s *service.Service) error {
				return s.AddProvider(nameVal, hasProjects, hasDeepResearch)
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 12)

}

// ── Account form ──────────────────────────────────────────────────────────────

// NewAccountForm returns a form for creating or editing an account. User is
// a structural prerequisite of Account — AddAccount requires a live user —
// so create mode resolves or creates the user on demand via
// Service.CreateAccount, the strict-account counterpart to StartSession's
// on-demand path. Edit mode changes only the account's own mutable field
// (pause state); it does not touch the underlying user's name, since that
// belongs to a different aggregate and would be a different form's job.
// Provider is a required precondition but is never created implicitly: if
// none exist, an error modal is returned instead of an empty dropdown.
// When account is nil a new account is created; otherwise it is edited.
func NewAccountForm(app App, account *model.Account, onClose func()) tview.Primitive {
	if account != nil {
		return accountEditForm(app, account, onClose)
	}
	if providers := app.Service().Providers(); len(providers) == 0 {
		return errorModal("No providers configured.\nAdd a provider before creating an account.", onClose)
	}
	return accountCreateForm(app, onClose)
}

// accountEditForm builds the edit-mode account form: pause state only.
// The underlying user's name is a different aggregate's field and is not
// touched here — see NewAccountForm's doc comment.
func accountEditForm(app App, account *model.Account, onClose func()) tview.Primitive {
	email := account.UserEmail()
	provider := account.ProviderName()
	var initPause string
	if !account.PauseUntil().IsZero() {
		initPause = model.FormatTime(account.PauseUntil())
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" Edit Account: %s / %s ", tview.Escape(email), tview.Escape(provider)))

	form.AddInputField("Email", email, 40, nil, nil)
	form.AddInputField("Provider", provider, 40, nil, nil)
	form.AddInputField("Pause until", initPause, 20, nil, nil)

	emailField, ok1 := form.GetFormItemByLabel("Email").(*tview.InputField)
	providerField, ok2 := form.GetFormItemByLabel("Provider").(*tview.InputField)
	pauseField, ok3 := form.GetFormItemByLabel("Pause until").(*tview.InputField)
	if !ok1 || !ok2 || !ok3 {
		// We just added these fields, so this should theoretically never happen.
		return errorModal("Internal error: failed to initialize form fields.", onClose)
	}
	emailField.SetDisabled(true)
	providerField.SetDisabled(true)
	pauseField.SetPlaceholder(model.TimeFormat + "  (leave empty to clear)")

	save := func() {
		field, ok := form.GetFormItemByLabel("Pause until").(*tview.InputField)
		if !ok {
			app.SetStatus("[red]internal error: Pause until field missing[-]")
			return
		}
		val := strings.TrimSpace(field.GetText())

		var until time.Time
		if val != "" {
			t, err := model.ParseTime(val)
			if err != nil {
				app.SetStatus(fmt.Sprintf("[red]pause until: use format %s[-]", model.TimeFormat))
				return
			}
			until = t
		}

		if err := app.Mutate(func(s *service.Service) error {
			return s.PauseAccount(email, provider, until)
		}); err != nil {
			app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 13)
}

// accountCreateForm builds the create-mode account form. Provider
// non-emptiness is the caller's (NewAccountForm's) responsibility, not
// checked here. Save reads the selected user from existingUsers, not
// the (disabled) field text, to avoid display/save drift.
func accountCreateForm(app App, onClose func()) tview.Primitive {
	providers := app.Service().Providers() // already sorted by name; non-empty, checked by caller
	providerNames := make([]string, len(providers))
	for i, p := range providers {
		providerNames[i] = p.Name()
	}

	existingUsers := app.Service().Users() // already sorted by email
	userLabels := make([]string, len(existingUsers)+1)
	userLabels[0] = "+ New user"
	for i, u := range existingUsers {
		userLabels[i+1] = fmt.Sprintf("%s (%s)", tview.Escape(u.Name()), tview.Escape(u.Email()))
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" New Account ")

	form.AddDropDown("User", userLabels, 0, nil)
	form.AddInputField("Email", "", 40, nil, nil)
	form.AddInputField("Name", "", 40, nil, nil)
	// providerNames stays raw for save()'s idx-based lookup below; the
	// dropdown only sees an escaped display copy.
	form.AddDropDown("Provider", escapeAll(providerNames), 0, nil)

	userDropDown, ok1 := form.GetFormItemByLabel("User").(*tview.DropDown)
	emailField, ok2 := form.GetFormItemByLabel("Email").(*tview.InputField)
	nameField, ok3 := form.GetFormItemByLabel("Name").(*tview.InputField)
	if !ok1 || !ok2 || !ok3 {
		return errorModal("Internal error: failed to initialize form fields.", onClose)
	}

	userDropDown.SetSelectedFunc(func(_ string, index int) {
		if index <= 0 {
			emailField.SetDisabled(false)
			nameField.SetDisabled(false)
			emailField.SetText("")
			nameField.SetText("")
			return
		}
		u := existingUsers[index-1]
		emailField.SetText(u.Email())
		nameField.SetText(u.Name())
		emailField.SetDisabled(true)
		nameField.SetDisabled(true)
	})

	save := func() {
		userIdx, _ := userDropDown.GetCurrentOption()

		var emailVal, nameVal string
		if userIdx > 0 {
			u := existingUsers[userIdx-1]
			emailVal = u.Email()
			nameVal = u.Name()
		} else {
			emailVal = strings.TrimSpace(emailField.GetText())
			nameVal = strings.TrimSpace(nameField.GetText())
		}

		providerDropDown, ok := form.GetFormItemByLabel("Provider").(*tview.DropDown)
		if !ok {
			app.SetStatus("[red]internal error: Provider dropdown missing[-]")
			return
		}
		// By index into the raw providerNames, not the string
		// GetCurrentOption returns — see the AddDropDown call above.
		providerIdx, _ := providerDropDown.GetCurrentOption()
		var providerVal string
		if providerIdx >= 0 && providerIdx < len(providerNames) {
			providerVal = providerNames[providerIdx]
		}

		if err := app.Service().ValidateAccount(emailVal, providerVal); err != nil {
			app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return
		}
		if err := app.Mutate(func(s *service.Service) error {
			_, err := s.CreateAccount(emailVal, nameVal, providerVal)
			return err
		}); err != nil {
			app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 16)
}

// ── User form ─────────────────────────────────────────────────────────────────

// NewUserForm returns a form for renaming the user backing account. Email
// is User's immutable natural key; only Name is editable. This is User's
// own edit path — see NewAccountForm's doc comment for why accountEditForm
// does not touch it.
func NewUserForm(app App, account *model.Account, onClose func()) tview.Primitive {
	if account == nil {
		return errorModal("No account selected.", onClose)
	}
	email := account.UserEmail()
	u := app.Service().User(email)
	if u == nil {
		return errorModal("User no longer exists.", onClose)
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" Rename User: %s ", tview.Escape(email)))

	form.AddInputField("Email", email, 40, nil, nil)
	form.AddInputField("Name", u.Name(), 40, nil, nil)

	emailField, ok := form.GetFormItemByLabel("Email").(*tview.InputField)
	if !ok {
		return errorModal("Internal error: failed to initialize form fields.", onClose)
	}
	emailField.SetDisabled(true)

	save := func() {
		nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
		if !ok {
			app.SetStatus("[red]internal error: Name field missing[-]")
			return
		}
		nameVal := strings.TrimSpace(nameField.GetText())

		if err := app.Mutate(func(s *service.Service) error {
			return s.UpdateUser(email, nameVal)
		}); err != nil {
			app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 11)
}

// ── Grant / Revoke forms ──────────────────────────────────────────────────────

// NewGrantForm returns a form for granting project's access to an account.
// This is not an entity-creation form — both project and account already
// exist; it manages the many-to-many link between them, which is why it
// legitimately references both aggregates by name (AddProjectToAccount is
// itself keyed on both sides of an existing pair, unlike a create-form
// composing an on-demand prerequisite).
func NewGrantForm(app App, project *model.Project, onClose func()) tview.Primitive {
	if project == nil {
		return errorModal("No project selected.", onClose)
	}

	accounts := app.Service().Accounts() // already sorted by composite key
	if len(accounts) == 0 {
		return errorModal("No accounts exist yet.\nCreate an account before granting project access.", onClose)
	}
	labels := make([]string, len(accounts))
	for i, a := range accounts {
		labels[i] = fmt.Sprintf("%s/%s", a.UserEmail(), a.ProviderName())
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" Grant %q to Account ", tview.Escape(project.Name())))

	// labels is only ever read back by index (below) — safe to escape
	// for display without a raw copy.
	form.AddDropDown("Account", escapeAll(labels), 0, nil)

	save := func() {
		dropDown, ok := form.GetFormItemByLabel("Account").(*tview.DropDown)
		if !ok {
			app.SetStatus("[red]internal error: Account dropdown missing[-]")
			return
		}
		idx, _ := dropDown.GetCurrentOption()
		if idx < 0 || idx >= len(accounts) {
			app.SetStatus("[red]internal error: invalid account selection[-]")
			return
		}
		email := accounts[idx].UserEmail()
		acctProvider := accounts[idx].ProviderName()
		name := project.Name()

		if err := app.Mutate(func(s *service.Service) error {
			return s.GrantProject(email, acctProvider, name)
		}); err != nil {
			app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 10)
}

// NewRevokeForm returns a form for revoking project's access from an
// account currently granted it. See NewGrantForm for why this form
// legitimately references both aggregates.
func NewRevokeForm(app App, project *model.Project, onClose func()) tview.Primitive {
	if project == nil {
		return errorModal("No project selected.", onClose)
	}

	accounts, err := app.Service().ProjectAccounts(project.Name())
	if err != nil || len(accounts) == 0 {
		return errorModal(fmt.Sprintf("Project %q is not granted to any account.", project.Name()), onClose)
	}
	labels := make([]string, len(accounts))
	for i, a := range accounts {
		labels[i] = fmt.Sprintf("%s/%s", a.UserEmail(), a.ProviderName())
	}

	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" Revoke %q from Account ", tview.Escape(project.Name())))

	form.AddDropDown("Account", escapeAll(labels), 0, nil)

	save := func() {
		dropDown, ok := form.GetFormItemByLabel("Account").(*tview.DropDown)
		if !ok {
			app.SetStatus("[red]internal error: Account dropdown missing[-]")
			return
		}
		idx, _ := dropDown.GetCurrentOption()
		if idx < 0 || idx >= len(accounts) {
			app.SetStatus("[red]internal error: invalid account selection[-]")
			return
		}
		email := accounts[idx].UserEmail()
		acctProvider := accounts[idx].ProviderName()
		name := project.Name()

		if err := app.Mutate(func(s *service.Service) error {
			return s.RevokeProject(email, acctProvider, name)
		}); err != nil {
			app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 10)
}
