package views

import (
	"fmt"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProjectsPanel is a full-screen view for managing projects: creating,
// editing, deleting, and granting/revoking account access. The operator
// navigates to it with 'p' and returns to the main view with 'Esc' or 'p'.
//
// This screen is the resolution of the TODO.md question "should we just
// have a project list with an indicator that it is unattached?" — projects
// with zero granted accounts are listed and flagged here, rather than being
// reachable only through a session's own project list as before.
type ProjectsPanel struct {
	*tview.Flex
	list       *tview.List
	infoView   *tview.TextView
	app        App
	projects   []*model.Project // mirrors list order; rebuilt on Refresh
	refreshing bool
}

// NewProjectsPanel constructs the projects management screen.
func NewProjectsPanel(app App) *ProjectsPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Projects ")

	info := tview.NewTextView().SetDynamicColors(true)
	info.SetBorder(true)
	info.SetTitle(" Project Details ")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(info, 8, 0, false) // fixed 8-row info panel (4 content lines + border + spare)

	p := &ProjectsPanel{
		Flex:     flex,
		list:     list,
		infoView: info,
		app:      app,
	}

	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if p.refreshing {
			return
		}
		p.renderInfo(p.selectedProject())
	})

	list.SetFocusFunc(func() {
		app.SetHints(HintsProjectsPanel)
	})

	list.SetInputCapture(p.handleKey)

	return p
}

// Refresh repopulates the list from the current service state.
func (p *ProjectsPanel) Refresh() {
	svc := p.app.Service()
	all := svc.Projects() // already sorted by name

	var prevName string
	if proj := p.selectedProject(); proj != nil {
		prevName = proj.Name()
	}

	p.refreshing = true
	p.projects = all
	p.list.Clear()
	for _, proj := range p.projects {
		accounts, err := svc.ProjectAccounts(proj.Name())
		if err != nil {
			accounts = nil
		}
		p.list.AddItem(projectItemText(proj, len(accounts)), "", 0, nil)
	}
	p.refreshing = false

	// Restore selection by name.
	restored := false
	for i, proj := range p.projects {
		if proj.Name() == prevName {
			p.list.SetCurrentItem(i)
			restored = true
			break
		}
	}
	if !restored && len(p.projects) > 0 {
		p.list.SetCurrentItem(0)
	}

	p.renderInfo(p.selectedProject())
}

// selectedProject returns the currently highlighted project, or nil when
// the list is empty.
func (p *ProjectsPanel) selectedProject() *model.Project {
	idx := p.list.GetCurrentItem()
	if idx < 0 || idx >= len(p.projects) {
		return nil
	}
	return p.projects[idx]
}

// renderInfo populates the detail panel for the selected project.
func (p *ProjectsPanel) renderInfo(proj *model.Project) {
	if proj == nil {
		p.infoView.SetText("  (no project selected)")
		return
	}
	svc := p.app.Service()

	var sb strings.Builder
	fmt.Fprintf(&sb, "  [::b]Name[-::-]         %s\n", proj.Name())

	desc := proj.Description()
	if desc == "" {
		desc = dimText("none")
	}
	fmt.Fprintf(&sb, "  [::b]Description[-::-]  %s\n", desc)

	instr := proj.Instructions()
	if instr == "" {
		instr = dimText("none")
	}
	fmt.Fprintf(&sb, "  [::b]Instructions[-::-] %s\n", instr)

	accounts, err := svc.ProjectAccounts(proj.Name())
	if err != nil {
		accounts = nil
	}
	if len(accounts) == 0 {
		fmt.Fprintf(&sb, "  [::b]Accounts[-::-]     %s", dimText("none (unattached)"))
	} else {
		labels := make([]string, len(accounts))
		for i, acc := range accounts {
			labels[i] = fmt.Sprintf("%s/%s", acc.UserEmail(), acc.ProviderName())
		}
		fmt.Fprintf(&sb, "  [::b]Accounts[-::-]     %s", strings.Join(labels, ", "))
	}

	p.infoView.SetText(sb.String())
}

// handleKey processes keyboard input for this panel.
func (p *ProjectsPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	// Esc and 'p' both return to the main screen.
	if event.Key() == KeyCancel {
		p.app.SwitchToMain()
		return nil
	}

	if event.Key() != tcell.KeyRune {
		return event
	}

	switch event.Rune() {
	case KeyProjects:
		p.app.SwitchToMain()
		return nil

	case KeyNew:
		modal := NewProjectForm(p.app, nil, nil, func() {
			p.app.SetHints(HintsProjectsPanel)
			p.app.HideModal()
		})
		if err := p.app.ShowModal(modal); err != nil {
			p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return nil
		}
		p.app.SetHints(HintsForm)
		return nil

	case KeyEdit:
		if proj := p.selectedProject(); proj != nil {
			modal := NewProjectForm(p.app, proj, nil, func() {
				p.app.SetHints(HintsProjectsPanel)
				p.app.HideModal()
			})
			if err := p.app.ShowModal(modal); err != nil {
				p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return nil
			}
			p.app.SetHints(HintsForm)
		}
		return nil

	case KeyDelete:
		if proj := p.selectedProject(); proj != nil {
			name := proj.Name()
			modal := NewConfirmModal(
				fmt.Sprintf("Delete project %q?\nIt must not be granted to any account.", name),
				func() {
					if err := p.app.Mutate(func(s *service.Service) error {
						return s.DeleteProject(name)
					}); err != nil {
						p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
					}
					p.app.SetHints(HintsProjectsPanel)
					p.app.HideModal()
				},
				func() {
					p.app.SetHints(HintsProjectsPanel)
					p.app.HideModal()
				},
			)
			if err := p.app.ShowModal(modal); err != nil {
				p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return nil
			}
			p.app.SetHints(HintsConfirmModal)
		}
		return nil

	case KeyGrant:
		if proj := p.selectedProject(); proj != nil {
			modal := NewGrantForm(p.app, proj, func() {
				p.app.SetHints(HintsProjectsPanel)
				p.app.HideModal()
			})
			if err := p.app.ShowModal(modal); err != nil {
				p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return nil
			}
			p.app.SetHints(HintsForm)
		}
		return nil

	case KeyRevoke:
		if proj := p.selectedProject(); proj != nil {
			modal := NewRevokeForm(p.app, proj, func() {
				p.app.SetHints(HintsProjectsPanel)
				p.app.HideModal()
			})
			if err := p.app.ShowModal(modal); err != nil {
				p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
				return nil
			}
			p.app.SetHints(HintsForm)
		}
		return nil

	case KeyHelp:
		if err := p.app.ShowModal(NewHelpOverlay(p.app)); err != nil {
			p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return nil
		}
		p.app.SetHints(HintsHelpOverlay)
		return nil

	case KeyQuit:
		modal := NewConfirmModal(
			"Quit llm-desk?",
			func() { p.app.Quit() },
			func() {
				p.app.SetHints(HintsProjectsPanel)
				p.app.HideModal()
			},
		)
		if err := p.app.ShowModal(modal); err != nil {
			p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			return nil
		}
		p.app.SetHints(HintsConfirmModal)
		return nil
	}
	return event
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// projectItemText formats a project row for the list. accountCount is the
// number of accounts currently granted this project; zero is flagged as
// "unattached".
func projectItemText(p *model.Project, accountCount int) string {
	if accountCount == 0 {
		return fmt.Sprintf("%-28s  %s", p.Name(), dimText("unattached"))
	}
	return p.Name()
}
