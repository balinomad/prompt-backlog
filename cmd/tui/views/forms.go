package views

import (
	"fmt"
	"sort"
	"strings"

	"time"

	"github.com/balinomad/llm-desk/internal/model"

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

// ── Session form ──────────────────────────────────────────────────────────────

// NewSessionForm returns a form for creating or editing a session.
// When session is nil a new session is created; otherwise the session is edited.
// The form is wrapped in a centred modal overlay ready for app.ShowModal.
func NewSessionForm(app App, session *model.Session, onClose func()) tview.Primitive {
	reg := app.Registry()

	// Build sorted provider list for the dropdown.
	allProviders := reg.AllProviders()
	if len(allProviders) == 0 {
		return errorModal("No providers configured.\nAdd a provider before creating a session.", onClose)
	}
	sort.Slice(allProviders, func(i, j int) bool {
		return allProviders[i].Name() < allProviders[j].Name()
	})
	providerNames := make([]string, len(allProviders))
	for i, p := range allProviders {
		providerNames[i] = p.Name()
	}

	isEdit := session != nil

	// Determine initial field values.
	initEmail, initName, initProvider, initNext := "", "", providerNames[0], ""
	initProviderIdx := 0
	if isEdit {
		initEmail = session.User().Email()
		initName = session.User().Name
		initProvider = session.Provider().Name()
		for i, n := range providerNames {
			if n == initProvider {
				initProviderIdx = i
				break
			}
		}
		if session.NextPromptAt != nil {
			initNext = model.FormatTime(*session.NextPromptAt)
		}
	}
	_ = initProvider // used via initProviderIdx

	form := tview.NewForm()
	form.SetBorder(true)
	if isEdit {
		form.SetTitle(fmt.Sprintf(" Edit Session: %s / %s ", initEmail, initProvider))
	} else {
		form.SetTitle(" New Session ")
	}

	// ── Fields ────────────────────────────────────────────────────────────────

	if isEdit {
		// Email and provider are immutable in edit mode; show as read-only text.
		form.AddInputField("Email (fixed)", initEmail, 40, func(_ string, _ rune) bool { return false }, nil)
		form.AddInputField("Provider (fixed)", initProvider, 40, func(_ string, _ rune) bool { return false }, nil)
	} else {
		form.AddInputField("Email", initEmail, 40, nil, nil)
		form.AddDropDown("Provider", providerNames, initProviderIdx, nil)
	}
	form.AddInputField("Name", initName, 40, nil, nil)
	form.AddInputField("Next prompt at", initNext, 20, nil, nil)
	form.GetFormItemByLabel("Next prompt at").(*tview.InputField).
		SetPlaceholder(model.TimeFormat + "  (leave empty to clear)")

	// ── Save logic ────────────────────────────────────────────────────────────

	save := func() {
		nameVal := strings.TrimSpace(form.GetFormItemByLabel("Name").(*tview.InputField).GetText())
		nextVal := strings.TrimSpace(form.GetFormItemByLabel("Next prompt at").(*tview.InputField).GetText())

		// Parse the optional timestamp.
		var nextTime *time.Time
		if nextVal != "" {
			t, err := model.ParseTime(nextVal)
			if err != nil {
				app.SetStatus(fmt.Sprintf("[red]next prompt at: use format %s[-]", model.TimeFormat))
				return
			}
			nextTime = &t
		}

		if isEdit {
			// Edit: update Name and NextPromptAt only.
			if strings.TrimSpace(nameVal) == "" {
				app.SetStatus("[red]name: required[-]")
				return
			}
			email := initEmail
			provider := initProvider
			if err := app.Mutate(func(r *model.Registry) error {
				u := r.UserByEmail(email)
				if u == nil {
					return fmt.Errorf("user %q not found", email)
				}
				u.Name = nameVal
				s := r.SessionAt(email, provider)
				if s == nil {
					return fmt.Errorf("session not found")
				}
				s.NextPromptAt = nextTime
				return nil
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		} else {
			// New: create user (if absent) and session.
			emailVal := strings.TrimSpace(form.GetFormItemByLabel("Email").(*tview.InputField).GetText())
			_, providerName := form.GetFormItemByLabel("Provider").(*tview.DropDown).GetCurrentOption()

			if errs := model.ValidateUser(model.NewUser(emailVal, nameVal)); len(errs) != 0 {
				app.SetStatus("[red]" + model.FormatErrors(errs) + "[-]")
				return
			}
			if err := app.Mutate(func(r *model.Registry) error {
				u := r.UserByEmail(emailVal)
				if u == nil {
					u = model.NewUser(emailVal, nameVal)
					if err := r.AddUser(u); err != nil {
						return err
					}
				} else {
					u.Name = nameVal
				}
				p := r.ProviderByName(providerName)
				if p == nil {
					return fmt.Errorf("provider %q not found", providerName)
				}
				s := model.NewSession(u, p)
				s.NextPromptAt = nextTime
				return r.AddSession(s)
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	height := 16
	if isEdit {
		height = 14
	}
	return centeredModal(form, 64, height)
}

// ── Project form ──────────────────────────────────────────────────────────────

// NewProjectForm returns a form for creating or editing a project.
// When project is nil a new project is created attached to session (or
// unattached when session is also nil); otherwise the project is edited.
func NewProjectForm(app App, project *model.Project, session *model.Session, onClose func()) tview.Primitive {
	isEdit := project != nil

	initName, initDesc, initInstr := "", "", ""
	if isEdit {
		initName = project.Name()
		initDesc = project.Description
		initInstr = project.Instructions
	}

	form := tview.NewForm()
	form.SetBorder(true)
	if isEdit {
		form.SetTitle(fmt.Sprintf(" Edit Project: %s ", initName))
	} else {
		form.SetTitle(" New Project ")
	}

	if isEdit {
		form.AddInputField("Name (fixed)", initName, 40,
			func(_ string, _ rune) bool { return false }, nil)
	} else {
		form.AddInputField("Name", initName, 40, nil, nil)
	}
	form.AddInputField("Description", initDesc, 40, nil, nil)
	form.AddInputField("Instructions", initInstr, 40, nil, nil)

	save := func() {
		descVal := strings.TrimSpace(form.GetFormItemByLabel("Description").(*tview.InputField).GetText())
		instrVal := strings.TrimSpace(form.GetFormItemByLabel("Instructions").(*tview.InputField).GetText())

		if isEdit {
			name := initName
			if err := app.Mutate(func(r *model.Registry) error {
				p := r.ProjectByName(name)
				if p == nil {
					return fmt.Errorf("project %q not found", name)
				}
				p.Description = descVal
				p.Instructions = instrVal
				return nil
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		} else {
			nameVal := strings.TrimSpace(form.GetFormItemByLabel("Name").(*tview.InputField).GetText())
			if errs := model.ValidateProject(model.NewProject(nameVal)); len(errs) != 0 {
				app.SetStatus("[red]" + model.FormatErrors(errs) + "[-]")
				return
			}
			if err := app.Mutate(func(r *model.Registry) error {
				var p *model.Project
				if session != nil {
					s := r.SessionAt(session.User().Email(), session.Provider().Name())
					if s == nil {
						return fmt.Errorf("session not found")
					}
					p = model.NewSessionProject(nameVal, s)
				} else {
					p = model.NewProject(nameVal)
				}
				p.Description = descVal
				p.Instructions = instrVal
				return r.AddProject(p)
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 64, 13)
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
		initHasProjects = provider.HasProjects
		initHasDeepResearch = provider.HasDeepResearch
	}

	form := tview.NewForm()
	form.SetBorder(true)
	if isEdit {
		form.SetTitle(fmt.Sprintf(" Edit Provider: %s ", initName))
	} else {
		form.SetTitle(" New Provider ")
	}

	if isEdit {
		form.AddInputField("Name (fixed)", initName, 40,
			func(_ string, _ rune) bool { return false }, nil)
	} else {
		form.AddInputField("Name", initName, 40, nil, nil)
	}
	form.AddCheckbox("Supports projects", initHasProjects, nil)
	form.AddCheckbox("Supports deep research", initHasDeepResearch, nil)

	save := func() {
		hasProjects := form.GetFormItemByLabel("Supports projects").(*tview.Checkbox).IsChecked()
		hasDeepResearch := form.GetFormItemByLabel("Supports deep research").(*tview.Checkbox).IsChecked()

		if isEdit {
			name := initName
			if err := app.Mutate(func(r *model.Registry) error {
				p := r.ProviderByName(name)
				if p == nil {
					return fmt.Errorf("provider %q not found", name)
				}
				p.HasProjects = hasProjects
				p.HasDeepResearch = hasDeepResearch
				return nil
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		} else {
			nameVal := strings.TrimSpace(form.GetFormItemByLabel("Name").(*tview.InputField).GetText())
			p := model.NewProvider(nameVal, hasProjects, hasDeepResearch)
			if errs := model.ValidateProvider(p); len(errs) != 0 {
				app.SetStatus("[red]" + model.FormatErrors(errs) + "[-]")
				return
			}
			if err := app.Mutate(func(r *model.Registry) error {
				return r.AddProvider(p)
			}); err != nil {
				app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return
			}
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 64, 12)
}

// ── Last question form ────────────────────────────────────────────────────────

// NewLastQuestionForm returns a form for editing the session's LastQuestion.
// If session.LastQuestion is nil, a new one is created on save.
func NewLastQuestionForm(app App, session *model.Session, onClose func()) tview.Primitive {
	if session == nil {
		return errorModal("No session selected.", onClose)
	}

	reg := app.Registry()

	// Build project dropdown: "none" + sorted session projects.
	sessionProjects := reg.ProjectsBySession(session)
	sort.Slice(sessionProjects, func(i, j int) bool {
		return sessionProjects[i].Name() < sessionProjects[j].Name()
	})
	projectOptions := make([]string, 0, len(sessionProjects)+1)
	projectOptions = append(projectOptions, "none")
	for _, p := range sessionProjects {
		projectOptions = append(projectOptions, p.Name())
	}

	lq := session.LastQuestion
	initProjectIdx := 0
	initTopic := ""
	initMidAnswer := false
	initNextPrompt := ""
	if lq != nil {
		initTopic = lq.Topic
		initMidAnswer = lq.MidAnswer
		initNextPrompt = lq.NextPrompt
		if lq.Project() != nil {
			for i, name := range projectOptions {
				if name == lq.Project().Name() {
					initProjectIdx = i
					break
				}
			}
		}
	}

	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" Last Question ")

	form.AddDropDown("Project", projectOptions, initProjectIdx, nil)
	form.AddInputField("Topic", initTopic, 40, nil, nil)
	form.AddCheckbox("Mid-answer", initMidAnswer, nil)
	form.AddInputField("Next prompt", initNextPrompt, 40, nil, nil)

	save := func() {
		_, projectName := form.GetFormItemByLabel("Project").(*tview.DropDown).GetCurrentOption()
		topic := strings.TrimSpace(form.GetFormItemByLabel("Topic").(*tview.InputField).GetText())
		midAnswer := form.GetFormItemByLabel("Mid-answer").(*tview.Checkbox).IsChecked()
		nextPrompt := strings.TrimSpace(form.GetFormItemByLabel("Next prompt").(*tview.InputField).GetText())

		email := session.User().Email()
		provider := session.Provider().Name()

		if err := app.Mutate(func(r *model.Registry) error {
			s := r.SessionAt(email, provider)
			if s == nil {
				return fmt.Errorf("session not found")
			}
			var proj *model.Project
			if projectName != "none" {
				proj = r.ProjectByName(projectName)
			}
			lq := model.NewLastQuestion(proj)
			lq.Topic = topic
			lq.MidAnswer = midAnswer
			lq.NextPrompt = nextPrompt
			s.LastQuestion = lq
			return nil
		}); err != nil {
			app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return
		}
		onClose()
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", onClose)
	form.SetCancelFunc(onClose)
	addCtrlS(form, save)

	return centeredModal(form, 64, 14)
}
