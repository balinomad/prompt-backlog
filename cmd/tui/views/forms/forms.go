// Package forms contains the tview forms used by the TUI.
package forms

import (
	"github.com/balinomad/prompt-backlog/cmd/tui/views"

	"fmt"
	"strings"
	"time"

	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── Form helper ───────────────────────────────────────────────────────────────

const (
	formFieldWidth = 40
	formNameWidth  = 30
	// formPauseWidth fits the timestamp plus one cell for the cursor.
	formPauseWidth = len(model.TimeFormat) + 1
	// formMaxWidth caps dynamic width at the former fixed modal width.
	formMaxWidth = 64
	// noteFieldWidth and noteTextHeight make note editing roomier.
	noteFieldWidth = 60
	noteTextHeight = 12
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
	form.SetLabelColor(views.ColorLabel)
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
	return formModalWithMax(form, formMaxWidth)
}

func formModalWithMax(form *tview.Form, maxWidth int) tview.Primitive {
	return views.NewModalState(views.CenteredModal(form, min(formWidth(form), maxWidth), formHeight(form)), form)
}

// finishForm adds the shared Save/Cancel controls and returns the centred
// modal wrapper.
func finishForm(app views.Host, form *tview.Form, save, onClose func()) tview.Primitive {
	return finishFormWithMax(app, form, save, onClose, formMaxWidth)
}

func finishFormWithMax(app views.Host, form *tview.Form, save, onClose func(), maxWidth int) tview.Primitive {
	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)
	installFormClipboard(app, form)
	return formModalWithMax(form, maxWidth)
}

// addCtrlS adds Ctrl+S as an additional save trigger to a form.
// The form's own button handles the primary click path; this handles keyboard.
func addCtrlS(form *tview.Form, save func()) {
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == views.KeySave {
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

// dropDownArrow marks a closed dropdown as a dropdown.
const dropDownArrow = " ▾"

// dropDownMaxWidth caps a dropdown at the normal field width.
const dropDownMaxWidth = formFieldWidth

// newDropDown returns a dropdown with a visible arrow and a field width that
// fits the longest option, capped at dropDownMaxWidth.
func newDropDown(label string, options []string, current int) *tview.DropDown {
	displayOptions := make([]string, len(options))
	for i, option := range options {
		displayOptions[i] = views.TruncateWidth(option, dropDownMaxWidth-tview.TaggedStringWidth(dropDownArrow))
	}
	dropDown := tview.NewDropDown().SetLabel(label)
	dropDown.SetOptions(displayOptions, nil)
	dropDown.SetCurrentOption(current)
	dropDown.SetTextOptions("", "", "", dropDownArrow, "")
	dropDown.SetFieldWidth(dropDownWidth(displayOptions))
	dropDown.SetListStyles(
		tcell.StyleDefault.Background(views.ColorPopup).Foreground(tview.Styles.PrimaryTextColor),
		tcell.StyleDefault.Background(tview.Styles.PrimaryTextColor).Foreground(views.ColorPopup),
	)
	dropDown.SetDisabledStyle(
		tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor).Foreground(tcell.ColorGray),
	)
	return dropDown
}

func addDropDown(form *tview.Form, label string, options []string, current int) {
	form.AddFormItem(newDropDown(label, options, current))
}

// formField returns the form item with label as type T. It reports a missing
// item through app and returns false so the caller can abort the save.
func formField[T tview.FormItem](app views.Host, form *tview.Form, label, missing string) (T, bool) {
	field, ok := form.GetFormItemByLabel(label).(T)
	if !ok {
		app.SetError("internal error: " + missing + " missing")
		return field, false
	}
	return field, true
}

func dropDownWidth(options []string) int {
	width := 0
	for _, option := range options {
		width = max(width, tview.TaggedStringWidth(option))
	}
	return min(width+tview.TaggedStringWidth(dropDownArrow), dropDownMaxWidth)
}

// ── Session form ──────────────────────────────────────────────────────────────

// sessionFormProjectNames returns NewSessionForm's Project dropdown
// options: "none" followed by the account's granted projects. A
// resolution error degrades to "no granted projects" rather than
// blocking form construction, matching the read-degrades pattern already
// used in projects.go's renderInfo — account is already known valid.
func sessionFormProjectNames(app views.Host, email, vendor string) []string {
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

// ── Project form ──────────────────────────────────────────────────────────────

// NewProjectForm returns a form for creating or editing a project. When
// project is nil a new project is created; otherwise it is edited. Granting
// is a separate action, see NewGrantForm.
func NewProjectForm(app views.Host, project *model.Project, onClose func()) tview.Primitive {
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

	addInputField(form, "Name", initName, formFieldWidth)
	form.AddTextArea("Description", initDesc, formFieldWidth, 2, 0, nil)
	form.AddTextArea("Instructions", initInstr, formFieldWidth, 2, 0, nil)

	save := func() {
		nameField, ok := formField[*InputField](app, form, "Name", "Name field")
		if !ok {
			return
		}
		nameVal := strings.TrimSpace(nameField.GetText())

		descField, ok := formField[*tview.TextArea](app, form, "Description", "Description field")
		if !ok {
			return
		}
		descVal := strings.TrimSpace(descField.GetText())

		instrField, ok := formField[*tview.TextArea](app, form, "Instructions", "Instructions field")
		if !ok {
			return
		}
		instrVal := strings.TrimSpace(instrField.GetText())

		if isEdit {
			if err := app.Mutate(func(s *service.Service) error {
				currentName := initName
				if nameVal != initName {
					if _, err := s.RenameProject(initName, nameVal); err != nil {
						return err
					}
					currentName = nameVal
				}
				return s.UpdateProject(currentName, descVal, instrVal)
			}); err != nil {
				views.SetErrorStatus(app, err)
				return
			}
		} else {
			if err := app.Service().ValidateProject(nameVal, descVal, instrVal); err != nil {
				views.SetErrorStatus(app, err)
				return
			}
			if err := app.Mutate(func(s *service.Service) error {
				return s.AddProject(nameVal, descVal, instrVal)
			}); err != nil {
				views.SetErrorStatus(app, err)
				return
			}
		}
		onClose()
	}

	return finishForm(app, form, save, onClose)
}

// ── Vendor form ─────────────────────────────────────────────────────────────

// NewVendorForm returns a form for creating or editing a vendor.
// When vendor is nil a new vendor is created; otherwise it is edited.
func NewVendorForm(app views.Host, vendor *model.Vendor, onClose func()) tview.Primitive {
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

	addInputField(form, "Name", initName, formNameWidth)
	form.AddCheckbox("Supports projects", initHasProjects, nil)
	form.AddCheckbox("Supports deep research", initHasDeepResearch, nil)

	if isEdit {
		nameField, ok := form.GetFormItemByLabel("Name").(*InputField)
		if !ok {
			return views.ErrorModal("Internal error: failed to initialize form fields.", onClose)
		}
		nameField.SetDisabled(true)
	}

	save := func() {
		projectCheckbox, ok := formField[*tview.Checkbox](app, form, "Supports projects", "project checkbox")
		if !ok {
			return
		}
		hasProjects := projectCheckbox.IsChecked()

		researchCheckbox, ok := formField[*tview.Checkbox](app, form, "Supports deep research", "deep research checkbox")
		if !ok {
			return
		}
		hasDeepResearch := researchCheckbox.IsChecked()

		if isEdit {
			name := initName
			if err := app.Mutate(func(s *service.Service) error {
				return s.UpdateVendor(name, hasProjects, hasDeepResearch)
			}); err != nil {
				views.SetErrorStatus(app, err)
				return
			}
		} else {
			nameField, ok := formField[*InputField](app, form, "Name", "Name field")
			if !ok {
				return
			}
			nameVal := strings.TrimSpace(nameField.GetText())

			if err := app.Service().ValidateVendor(nameVal, hasProjects, hasDeepResearch); err != nil {
				views.SetErrorStatus(app, err)
				return
			}
			if err := app.Mutate(func(s *service.Service) error {
				return s.AddVendor(nameVal, hasProjects, hasDeepResearch)
			}); err != nil {
				views.SetErrorStatus(app, err)
				return
			}
		}
		onClose()
	}

	return finishForm(app, form, save, onClose)

}

// ── Account form ──────────────────────────────────────────────────────────────

// NewAccountForm returns a form for creating or editing an account. Create
// mode requires an existing user. Edit mode changes the pause state. Vendors
// and users must already exist.
func NewAccountForm(app views.Host, account *model.Account, onClose func()) tview.Primitive {
	if account != nil {
		return accountEditForm(app, account, onClose)
	}
	if vendors := app.Service().Vendors(); len(vendors) == 0 {
		return views.ErrorModal("No vendors configured.\nAdd a vendor before creating an account.", onClose)
	}
	if users := app.Service().Users(); len(users) == 0 {
		return views.ErrorModal("No users configured.\nAdd a user before creating an account.", onClose)
	}
	return accountCreateForm(app, "", "", onClose)
}

// accountEditForm builds the edit-mode pause form. Vendor and User are
// read-only context.
func accountEditForm(app views.Host, account *model.Account, onClose func()) tview.Primitive {
	email := account.UserEmail()
	vendor := account.VendorName()
	userName := views.UserDisplayName(app.Service(), email)
	var initPause string
	if !account.PauseUntil().IsZero() {
		initPause = model.FormatTime(account.PauseUntil())
	}

	form := newForm("Pause Account")

	form.AddFormItem(newCenteredFormLabel(vendor + " · " + userName))
	addInputField(form, "Pause until", initPause, formPauseWidth)

	pauseField, ok := form.GetFormItemByLabel("Pause until").(*InputField)
	if !ok {
		return views.ErrorModal("Internal error: failed to initialize form fields.", onClose)
	}
	pauseField.SetPlaceholder(model.FormatTime(time.Now()) + "  (leave empty to clear)")
	pauseField.SetPlaceholderStyle(tcell.StyleDefault.
		Background(tview.Styles.ContrastBackgroundColor).
		Foreground(tview.Styles.PrimaryTextColor).
		Dim(true))

	save := func() {
		field, ok := formField[*InputField](app, form, "Pause until", "Pause until field")
		if !ok {
			return
		}
		val := strings.TrimSpace(field.GetText())

		var until time.Time
		if val != "" {
			t, err := model.ParseTime(val)
			if err != nil {
				app.SetError("pause until: use format " + model.TimeFormat)
				return
			}
			until = t
		}

		if err := app.Mutate(func(s *service.Service) error {
			return s.PauseAccount(email, vendor, until)
		}); err != nil {
			views.SetErrorStatus(app, err)
			return
		}
		onClose()
	}

	return finishForm(app, form, save, onClose)
}

// accountCreateForm builds the create-mode account form. The caller ensures
// at least one vendor exists. Save reads the selected user and vendor by
// dropdown index.
func accountCreateForm(app views.Host, initialVendor, initialUserEmail string, onClose func()) tview.Primitive {
	vendors := app.Service().Vendors() // already sorted by name; non-empty, checked by caller
	vendorNames := make([]string, len(vendors))
	for i, p := range vendors {
		vendorNames[i] = p.Name()
	}

	users := app.Service().Users() // sorted by name
	userLabels := make([]string, len(users))
	for i, u := range users {
		userLabels[i] = fmt.Sprintf("%s (%s)", tview.Escape(u.Name()), tview.Escape(u.Email()))
	}

	form := newForm("New Account")

	addDropDown(form, "User", userLabels, accountUserIndex(users, initialUserEmail))
	// vendorNames stays raw for saveAccountCreate's idx-based lookup below;
	// the dropdown only sees an escaped display copy.
	addDropDown(form, "Vendor", escapeAll(vendorNames), accountVendorIndex(vendorNames, initialVendor))

	return finishForm(app, form, func() {
		saveAccountCreate(app, form, users, vendorNames, onClose)
	}, onClose)
}

func accountUserIndex(users []*model.User, email string) int {
	for i, u := range users {
		if u.Email() == email {
			return i
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

func saveAccountCreate(app views.Host, form *tview.Form, users []*model.User, vendorNames []string, onClose func()) {
	userDropDown, ok := formField[*tview.DropDown](app, form, "User", "User dropdown")
	if !ok {
		return
	}
	userIdx, _ := userDropDown.GetCurrentOption()
	if userIdx < 0 || userIdx >= len(users) {
		app.SetError("select a user before saving")
		return
	}
	u := users[userIdx]

	vendorDropDown, ok := formField[*tview.DropDown](app, form, "Vendor", "Vendor dropdown")
	if !ok {
		return
	}
	vendorIdx, _ := vendorDropDown.GetCurrentOption()
	if vendorIdx < 0 || vendorIdx >= len(vendorNames) {
		app.SetError("internal error: invalid vendor selection")
		return
	}

	if err := app.Mutate(func(s *service.Service) error {
		return s.AddAccount(u.Email(), vendorNames[vendorIdx])
	}); err != nil {
		views.SetErrorStatus(app, err)
		return
	}
	onClose()
}

// ── User form ─────────────────────────────────────────────────────────────────

// NewUserForm returns a form for creating or editing a user. When user is nil
// a new user is created; otherwise the user's name is edited. Email is
// immutable in edit mode.
func NewUserForm(app views.Host, user *model.User, onClose func()) tview.Primitive {
	isEdit := user != nil

	initEmail, initName := "", ""
	if isEdit {
		current := app.Service().User(user.Email())
		if current == nil {
			return views.ErrorModal("User no longer exists.", onClose)
		}
		initEmail = current.Email()
		initName = current.Name()
	}

	title := "New User"
	if isEdit {
		title = "Edit User: " + tview.Escape(initEmail)
	}
	form := newForm(title)

	addInputField(form, "Email", initEmail, formFieldWidth)
	addInputField(form, "Name", initName, formFieldWidth)

	emailField, ok := form.GetFormItemByLabel("Email").(*InputField)
	if !ok {
		return views.ErrorModal("Internal error: failed to initialize form fields.", onClose)
	}
	if isEdit {
		emailField.SetDisabled(true)
	}

	save := func() {
		nameField, ok := formField[*InputField](app, form, "Name", "Name field")
		if !ok {
			return
		}
		nameVal := strings.TrimSpace(nameField.GetText())

		if isEdit {
			email := initEmail
			if err := app.Mutate(func(s *service.Service) error {
				return s.UpdateUser(email, nameVal)
			}); err != nil {
				views.SetErrorStatus(app, err)
				return
			}
		} else {
			emailVal := strings.TrimSpace(emailField.GetText())
			if err := app.Service().ValidateUser(emailVal, nameVal); err != nil {
				views.SetErrorStatus(app, err)
				return
			}
			if err := app.Mutate(func(s *service.Service) error {
				return s.AddUser(emailVal, nameVal)
			}); err != nil {
				views.SetErrorStatus(app, err)
				return
			}
		}
		onClose()
	}

	return finishForm(app, form, save, onClose)
}

// ── Grant form ───────────────────────────────────────────────────────────────

// NewGrantForm returns a form for granting project access to an account. The
// picker lists accounts whose vendor supports projects and that do not
// already have access.
func NewGrantForm(app views.Host, project *model.Project, onClose func()) tview.Primitive {
	if project == nil {
		return views.ErrorModal("No project selected.", onClose)
	}

	accounts, err := grantCandidates(app.Service(), project)
	if err != nil {
		return views.ErrorModal(fmt.Sprintf("Could not load accounts: %v", err), onClose)
	}
	if len(accounts) == 0 {
		return views.ErrorModal("No eligible accounts are available to grant.\nAccounts must support projects and not already have access.", onClose)
	}

	vendorWidth, nameWidth := views.ProjectAccountWidths(app.Service())
	labels := make([]string, len(accounts))
	for i, acc := range accounts {
		labels[i] = views.ProjectAccountText(app.Service(), acc, vendorWidth, nameWidth)
	}

	form := newForm(fmt.Sprintf("Grant %q to Account", tview.Escape(project.Name())))

	addDropDown(form, "Account", escapeAll(labels), 0)

	save := func() {
		dropDown, ok := formField[*tview.DropDown](app, form, "Account", "Account dropdown")
		if !ok {
			return
		}
		idx, _ := dropDown.GetCurrentOption()
		if idx < 0 || idx >= len(accounts) {
			app.SetError("internal error: invalid account selection")
			return
		}
		email := accounts[idx].UserEmail()
		acctVendor := accounts[idx].VendorName()
		name := project.Name()

		if err := app.Mutate(func(s *service.Service) error {
			return s.GrantProject(email, acctVendor, name)
		}); err != nil {
			views.SetErrorStatus(app, err)
			return
		}
		onClose()
	}

	return finishForm(app, form, save, onClose)
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
	views.SortProjectAccounts(svc, candidates)
	return candidates, nil
}
