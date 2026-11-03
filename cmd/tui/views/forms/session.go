package forms

import (
	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/rivo/tview"
)

// SessionFormOptions configures the shared session form used for starting and
// editing sessions.
type SessionFormOptions struct {
	Title            string
	SubmitLabel      string
	Account          *model.Account
	Values           service.SessionFields
	FixedProjectName string
	OnSubmit         func(service.SessionFields) error
	OnCancel         func()
}

// NewSessionForm returns the shared session form. It renders the account,
// name, URL, and project fields, and returns submitted values to the caller.
// It does not create prompts, consume notes, or create projects.
func NewSessionForm(app views.Host, options *SessionFormOptions) tview.Primitive {
	if options.Account == nil {
		return views.ErrorModal("No account selected.", options.OnCancel)
	}

	form := newForm(options.Title)
	form.AddFormItem(newCenteredFormLabel(accountDisplayName(app.Service(), options.Account)))

	projectNames := []string(nil)
	if options.FixedProjectName != "" {
		form.AddFormItem(newCenteredFormLabel("Project: " + tview.Escape(options.FixedProjectName)))
	} else {
		projectNames = sessionFormProjectNames(app, options.Account.UserEmail(), options.Account.VendorName())
		projectIndex := 0
		for i, name := range projectNames {
			if name == options.Values.ProjectName {
				projectIndex = i
				break
			}
		}
		addDropDown(form, "Project", escapeAll(projectNames), projectIndex)
	}

	form.AddInputField("Name", options.Values.Name, formFieldWidth, nil, nil)
	form.AddInputField("URL", options.Values.URL, formFieldWidth, nil, nil)

	submit := func() {
		projectName := options.FixedProjectName
		if projectName == "" {
			dropDown, ok := form.GetFormItemByLabel("Project").(*tview.DropDown)
			if !ok {
				app.SetError("internal error: Project dropdown missing")
				return
			}
			index, _ := dropDown.GetCurrentOption()
			if index > 0 && index < len(projectNames) {
				projectName = projectNames[index]
			}
		}

		nameField, ok := form.GetFormItemByLabel("Name").(*tview.InputField)
		if !ok {
			app.SetError("internal error: Name field missing")
			return
		}
		urlField, ok := form.GetFormItemByLabel("URL").(*tview.InputField)
		if !ok {
			app.SetError("internal error: URL field missing")
			return
		}

		values := service.SessionFields{
			Name:        model.NormalizeText(nameField.GetText()),
			URL:         model.NormalizeText(urlField.GetText()),
			ProjectName: projectName,
		}
		if err := options.OnSubmit(values); err != nil {
			views.SetErrorStatus(app, err)
		}
	}

	form.AddButton(options.SubmitLabel, submit)
	form.AddButton("Cancel", options.OnCancel)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetCancelFunc(options.OnCancel)
	addCtrlS(form, submit)
	return formModal(form)
}
