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

const (
	formFieldWidth = 40
	formNameWidth  = 30
	formPauseWidth = 20
	// formMaxWidth caps dynamic width at the former fixed modal width.
	formMaxWidth = 64
	// formChromeHeight is the border plus default vertical padding.
	formChromeHeight = 4
	// formButtonPadding is the horizontal padding around a button label.
	formButtonPadding = 5
)

// newForm returns a bordered form with a spaced title.
func newForm(title string) *tview.Form {
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" " + title + " ")
	return form
}

// centeredFormLabel is a form item that draws one centered line of text.
type centeredFormLabel struct {
	*tview.TextView
}

// newCenteredFormLabel returns a centered, non-focusable form label.
func newCenteredFormLabel(text string) *centeredFormLabel {
	label := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetWrap(false).
		SetScrollable(false)
	label.SetText(text)
	label.SetSize(1, 0)
	return &centeredFormLabel{TextView: label}
}

// SetFormAttributes ignores the form label column so the text can center
// across the full row.
func (l *centeredFormLabel) SetFormAttributes(_ int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	l.TextView.SetFormAttributes(0, labelColor, bgColor, fieldTextColor, fieldBgColor)
	return l
}

// GetFieldWidth reports the label text width so the form can size around it.
func (l *centeredFormLabel) GetFieldWidth() int {
	return tview.TaggedStringWidth(l.GetText(false))
}

// formHeight returns the height that fits form's items, one button row, the
// border, and the form's default vertical padding. Each item contributes its
// field height plus the default one-row padding.
func formHeight(form *tview.Form) int {
	height := formChromeHeight
	for i := range form.GetFormItemCount() {
		itemHeight := form.GetFormItem(i).GetFieldHeight()
		if itemHeight <= 0 {
			itemHeight = 1
		}
		height += itemHeight + 1
	}
	if form.GetButtonCount() > 0 {
		height++
	} else if form.GetFormItemCount() > 0 {
		height--
	}
	return height
}

// formWidth returns the width needed to fit form's label and field columns,
// its buttons, the border, and the form's horizontal padding.
func formWidth(form *tview.Form) int {
	maxLabel, maxField := 0, 0
	for i := range form.GetFormItemCount() {
		item := form.GetFormItem(i)
		maxLabel = max(maxLabel, tview.TaggedStringWidth(item.GetLabel()))
		maxField = max(maxField, item.GetFieldWidth())
	}

	buttonsWidth := 0
	for i := range form.GetButtonCount() {
		buttonsWidth += tview.TaggedStringWidth(form.GetButton(i).GetLabel()) + formButtonPadding
	}
	if buttonsWidth > 0 {
		buttonsWidth--
	}

	return max(maxLabel+1+maxField, buttonsWidth) + 4
}

// formModal centers form at its content width and height.
func formModal(form *tview.Form) tview.Primitive {
	return centeredModal(form, min(formWidth(form), formMaxWidth), formHeight(form))
}

// finishForm adds the shared Save/Cancel controls and returns the centred
// modal wrapper.
func finishForm(form *tview.Form, save, onClose func()) tview.Primitive {
	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)
	return formModal(form)
}

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
// items) without free text being read as a colour or region tag. Callers
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
// account. When session is nil a new session is created; otherwise it is
// edited. Queue editing is handled by NewQueueEditor.
func NewSessionForm(app App, account *model.Account, session *model.Session, onClose func()) tview.Primitive {
	if account == nil {
		return errorModal("No account selected.", onClose)
	}

	email := account.UserEmail()
	vendor := account.VendorName()

	projectNames := sessionFormProjectNames(app, email, vendor) // raw — see save() below
	isEdit := session != nil
	initTopic, initProjectIdx := sessionFormInitial(session, projectNames)

	title := "New Session"
	if isEdit {
		title = fmt.Sprintf("Edit Session: %s / %s", tview.Escape(email), tview.Escape(vendor))
	}
	form := newForm(title)

	form.AddDropDown("Project", escapeAll(projectNames), initProjectIdx, nil)
	form.AddInputField("Topic", initTopic, formFieldWidth, nil, nil)

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
			// Update the project first: if it fails, the topic remains
			// unchanged. The topic update cannot fail after a successful
			// project update in the single-threaded service.
			if err := app.Mutate(func(s *service.Service) error {
				if err := s.UpdateSessionProject(email, vendor, id, projectName); err != nil {
					return err
				}
				return s.UpdateSessionTopic(email, vendor, id, topicVal)
			}); err != nil {
				setErrorStatus(app, err)
				return
			}
		} else {
			if err := app.Mutate(func(s *service.Service) error {
				_, err := s.StartSession(email, vendor, projectName, topicVal, nil)
				return err
			}); err != nil {
				setErrorStatus(app, err)
				return
			}
		}
		onClose()
	}

	return finishForm(form, save, onClose)
}

// sessionFormProjectNames returns NewSessionForm's Project dropdown
// options: "none" followed by the account's granted projects. A
// resolution error degrades to "no granted projects" rather than
// blocking form construction, matching the read-degrades pattern already
// used in projects.go's renderInfo — account is already known valid.
func sessionFormProjectNames(app App, email, vendor string) []string {
	granted, err := app.Service().AccountProjects(email, vendor)
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

// NewProjectForm returns a form for creating or editing a project. When
// project is nil a new project is created; otherwise it is edited. Granting
// is a separate action, see NewGrantForm.
func NewProjectForm(app App, project *model.Project, onClose func()) tview.Primitive {
	isEdit := project != nil

	initName, initDesc, initInstr := "", "", ""
	if isEdit {
		initName = project.Name()
		initDesc = project.Description()
		initInstr = project.Instructions()
	}

	title := "New Project"
	if isEdit {
		title = "Edit Project: " + tview.Escape(initName)
	}
	form := newForm(title)

	if !isEdit {
		form.AddInputField("Name", initName, formFieldWidth, nil, nil)
	}
	form.AddTextArea("Description", initDesc, formFieldWidth, 2, 0, nil)
	form.AddTextArea("Instructions", initInstr, formFieldWidth, 2, 0, nil)

	save := func() {
		descField, ok := form.GetFormItemByLabel("Description").(*tview.TextArea)
		if !ok {
			app.SetStatus("[red]internal error: Description field missing[-]")
			return
		}
		descVal := strings.TrimSpace(descField.GetText())

		instrField, ok := form.GetFormItemByLabel("Instructions").(*tview.TextArea)
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
				setErrorStatus(app, err)
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
				setErrorStatus(app, err)
				return
			}
			if err := app.Mutate(func(s *service.Service) error {
				return s.AddProject(nameVal, descVal, instrVal)
			}); err != nil {
				setErrorStatus(app, err)
				return
			}
		}
		onClose()
	}

	return finishForm(form, save, onClose)
}

// ── Vendor form ─────────────────────────────────────────────────────────────

// NewVendorForm returns a form for creating or editing a vendor.
// When vendor is nil a new vendor is created; otherwise it is edited.
func NewVendorForm(app App, vendor *model.Vendor, onClose func()) tview.Primitive {
	isEdit := vendor != nil

	initName := ""
	initHasProjects := false
	initHasDeepResearch := false
	if isEdit {
		initName = vendor.Name()
		initHasProjects = vendor.HasProjects()
		initHasDeepResearch = vendor.HasDeepResearch()
	}

	title := "New Vendor"
	if isEdit {
		title = "Edit Vendor: " + tview.Escape(initName)
	}
	form := newForm(title)

	form.AddInputField("Name", initName, formNameWidth, nil, nil)
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
				return s.UpdateVendor(name, hasProjects, hasDeepResearch)
			}); err != nil {
				setErrorStatus(app, err)
				return
			}
		} else {
			nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
			if !ok {
				app.SetStatus("[red]internal error: Name field missing[-]")
				return
			}
			nameVal := strings.TrimSpace(nameField.GetText())

			if err := app.Service().ValidateVendor(nameVal, hasProjects, hasDeepResearch); err != nil {
				setErrorStatus(app, err)
				return
			}
			if err := app.Mutate(func(s *service.Service) error {
				return s.AddVendor(nameVal, hasProjects, hasDeepResearch)
			}); err != nil {
				setErrorStatus(app, err)
				return
			}
		}
		onClose()
	}

	return finishForm(form, save, onClose)

}

// ── Account form ──────────────────────────────────────────────────────────────

// NewAccountForm returns a form for creating or editing an account. Create
// mode requires an existing user; use "+ New user" to create one first. Edit
// mode changes the pause state. Vendors must already exist.
func NewAccountForm(app App, account *model.Account, onClose func()) tview.Primitive {
	if account != nil {
		return accountEditForm(app, account, onClose)
	}
	if vendors := app.Service().Vendors(); len(vendors) == 0 {
		return errorModal("No vendors configured.\nAdd a vendor before creating an account.", onClose)
	}
	return accountCreateForm(app, "", "", onClose)
}

// accountEditForm builds the edit-mode pause form. Vendor and User are
// read-only context.
func accountEditForm(app App, account *model.Account, onClose func()) tview.Primitive {
	email := account.UserEmail()
	vendor := account.VendorName()
	userName := email
	if u := app.Service().User(email); u != nil && u.Name() != "" {
		userName = u.Name()
	}
	var initPause string
	if !account.PauseUntil().IsZero() {
		initPause = model.FormatTime(account.PauseUntil())
	}

	form := newForm("Pause Account")

	form.AddFormItem(newCenteredFormLabel(vendor + " · " + userName))
	form.AddInputField("Pause until", initPause, formPauseWidth, nil, nil)

	pauseField, ok := form.GetFormItemByLabel("Pause until").(*tview.InputField)
	if !ok {
		return errorModal("Internal error: failed to initialize form fields.", onClose)
	}
	pauseField.SetPlaceholder(model.FormatTime(time.Now()) + "  (leave empty to clear)")
	pauseField.SetPlaceholderStyle(tcell.StyleDefault.
		Background(tview.Styles.ContrastBackgroundColor).
		Foreground(tview.Styles.PrimaryTextColor).
		Dim(true))

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
			return s.PauseAccount(email, vendor, until)
		}); err != nil {
			setErrorStatus(app, err)
			return
		}
		onClose()
	}

	return finishForm(form, save, onClose)
}

// accountCreateForm builds the create-mode account form. The caller ensures
// at least one vendor exists. Save reads the selected user and vendor by
// dropdown index.
func accountCreateForm(app App, initialVendor, initialUserEmail string, onClose func()) tview.Primitive {
	vendors := app.Service().Vendors() // already sorted by name; non-empty, checked by caller
	vendorNames := make([]string, len(vendors))
	for i, p := range vendors {
		vendorNames[i] = p.Name()
	}

	users := app.Service().Users() // sorted by name
	userLabels := make([]string, len(users)+1)
	userLabels[0] = "+ New user"
	for i, u := range users {
		userLabels[i+1] = fmt.Sprintf("%s (%s)", tview.Escape(u.Name()), tview.Escape(u.Email()))
	}

	form := newForm("New Account")

	form.AddDropDown("User", userLabels, accountUserIndex(users, initialUserEmail), nil)
	// vendorNames stays raw for saveAccountCreate's idx-based lookup below;
	// the dropdown only sees an escaped display copy.
	form.AddDropDown("Vendor", escapeAll(vendorNames), accountVendorIndex(vendorNames, initialVendor), nil)

	userDropDown, ok := form.GetFormItemByLabel("User").(*tview.DropDown)
	if !ok {
		return errorModal("Internal error: failed to initialize form fields.", onClose)
	}
	userDropDown.SetSelectedFunc(func(_ string, index int) {
		if index == 0 {
			openNewUserFlow(app, form, vendorNames, users, onClose)
		}
	})

	return finishForm(form, func() {
		saveAccountCreate(app, form, users, vendorNames, onClose)
	}, onClose)
}

func accountUserIndex(users []*model.User, email string) int {
	for i, u := range users {
		if u.Email() == email {
			return i + 1
		}
	}
	return 0
}

func accountVendorIndex(vendors []string, name string) int {
	for i, v := range vendors {
		if v == name {
			return i
		}
	}
	return 0
}

func openNewUserFlow(app App, form *tview.Form, vendorNames []string, users []*model.User, onClose func()) {
	vendorDropDown, ok := form.GetFormItemByLabel("Vendor").(*tview.DropDown)
	if !ok {
		app.SetStatus("[red]internal error: Vendor dropdown missing[-]")
		return
	}
	vendorIdx, _ := vendorDropDown.GetCurrentOption()
	vendorName := ""
	if vendorIdx >= 0 && vendorIdx < len(vendorNames) {
		vendorName = vendorNames[vendorIdx]
	}

	before := make(map[string]struct{}, len(users))
	for _, u := range users {
		before[u.Email()] = struct{}{}
	}
	reopen := func() {
		app.HideModal()
		selectedUser := ""
		for _, u := range app.Service().Users() {
			if _, ok := before[u.Email()]; !ok {
				selectedUser = u.Email()
				break
			}
		}
		showModal(app, accountCreateForm(app, vendorName, selectedUser, onClose), HintsFormSpecific, HintsFormCommon)
	}

	app.HideModal()
	showModal(app, NewUserForm(app, nil, reopen), HintsFormSpecific, HintsFormCommon)
}

func saveAccountCreate(app App, form *tview.Form, users []*model.User, vendorNames []string, onClose func()) {
	userDropDown, ok := form.GetFormItemByLabel("User").(*tview.DropDown)
	if !ok {
		app.SetStatus("[red]internal error: User dropdown missing[-]")
		return
	}
	userIdx, _ := userDropDown.GetCurrentOption()
	if userIdx <= 0 {
		app.SetStatus("[red]select a user before saving[-]")
		return
	}
	u := users[userIdx-1]

	vendorDropDown, ok := form.GetFormItemByLabel("Vendor").(*tview.DropDown)
	if !ok {
		app.SetStatus("[red]internal error: Vendor dropdown missing[-]")
		return
	}
	vendorIdx, _ := vendorDropDown.GetCurrentOption()
	if vendorIdx < 0 || vendorIdx >= len(vendorNames) {
		app.SetStatus("[red]internal error: invalid vendor selection[-]")
		return
	}

	if err := app.Mutate(func(s *service.Service) error {
		return s.AddAccount(u.Email(), vendorNames[vendorIdx])
	}); err != nil {
		setErrorStatus(app, err)
		return
	}
	onClose()
}

// ── User form ─────────────────────────────────────────────────────────────────

// NewUserForm returns a form for creating or editing a user. When user is nil
// a new user is created; otherwise the user's name is edited. Email is
// immutable in edit mode.
func NewUserForm(app App, user *model.User, onClose func()) tview.Primitive {
	isEdit := user != nil

	initEmail, initName := "", ""
	if isEdit {
		current := app.Service().User(user.Email())
		if current == nil {
			return errorModal("User no longer exists.", onClose)
		}
		initEmail = current.Email()
		initName = current.Name()
	}

	title := "New User"
	if isEdit {
		title = "Edit User: " + tview.Escape(initEmail)
	}
	form := newForm(title)

	form.AddInputField("Email", initEmail, formFieldWidth, nil, nil)
	form.AddInputField("Name", initName, formFieldWidth, nil, nil)

	emailField, ok := form.GetFormItemByLabel("Email").(*tview.InputField)
	if !ok {
		return errorModal("Internal error: failed to initialize form fields.", onClose)
	}
	if isEdit {
		emailField.SetDisabled(true)
	}

	save := func() {
		nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
		if !ok {
			app.SetStatus("[red]internal error: Name field missing[-]")
			return
		}
		nameVal := strings.TrimSpace(nameField.GetText())

		if isEdit {
			email := initEmail
			if err := app.Mutate(func(s *service.Service) error {
				return s.UpdateUser(email, nameVal)
			}); err != nil {
				setErrorStatus(app, err)
				return
			}
		} else {
			emailVal := strings.TrimSpace(emailField.GetText())
			if err := app.Service().ValidateUser(emailVal, nameVal); err != nil {
				setErrorStatus(app, err)
				return
			}
			if err := app.Mutate(func(s *service.Service) error {
				return s.AddUser(emailVal, nameVal)
			}); err != nil {
				setErrorStatus(app, err)
				return
			}
		}
		onClose()
	}

	return finishForm(form, save, onClose)
}

// ── Grant form ───────────────────────────────────────────────────────────────

// NewGrantForm returns a form for granting project access to an account. The
// picker lists accounts whose vendor supports projects and that do not
// already have access.
func NewGrantForm(app App, project *model.Project, onClose func()) tview.Primitive {
	if project == nil {
		return errorModal("No project selected.", onClose)
	}

	accounts, err := grantCandidates(app.Service(), project)
	if err != nil {
		return errorModal(fmt.Sprintf("Could not load accounts: %v", err), onClose)
	}
	if len(accounts) == 0 {
		return errorModal("No eligible accounts are available to grant.\nAccounts must support projects and not already have access.", onClose)
	}

	vendorWidth, nameWidth := projectAccountWidths(app.Service())
	labels := make([]string, len(accounts))
	for i, acc := range accounts {
		labels[i] = projectAccountText(app.Service(), acc, vendorWidth, nameWidth)
	}

	form := newForm(fmt.Sprintf("Grant %q to Account", tview.Escape(project.Name())))

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
		acctVendor := accounts[idx].VendorName()
		name := project.Name()

		if err := app.Mutate(func(s *service.Service) error {
			return s.GrantProject(email, acctVendor, name)
		}); err != nil {
			setErrorStatus(app, err)
			return
		}
		onClose()
	}

	return finishForm(form, save, onClose)
}

// grantCandidates returns the accounts eligible for a new grant to project:
// their vendor must support projects, and they must not already have access.
// The result is ordered alphabetically by vendor and user name.
func grantCandidates(svc *service.Service, project *model.Project) ([]*model.Account, error) {
	granted, err := svc.ProjectAccounts(project.Name())
	if err != nil {
		return nil, fmt.Errorf("project accounts: %w", err)
	}
	grantedKeys := make(map[model.AccountKey]struct{}, len(granted))
	for _, acc := range granted {
		grantedKeys[acc.Key()] = struct{}{}
	}

	all := svc.Accounts()
	candidates := make([]*model.Account, 0, len(all))
	for _, acc := range all {
		vendor := svc.Vendor(acc.VendorName())
		if vendor == nil || !vendor.HasProjects() {
			continue
		}
		if _, ok := grantedKeys[acc.Key()]; ok {
			continue
		}
		candidates = append(candidates, acc)
	}
	sortProjectAccounts(svc, candidates)
	return candidates, nil
}
