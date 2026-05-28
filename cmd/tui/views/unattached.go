package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UnattachedPanel is a full-screen view listing all projects that are not
// attached to any session. The operator switches to it with the 's' key and
// returns to the main view with 'Esc' or 's'.
type UnattachedPanel struct {
	*tview.Flex
	list       *tview.List
	infoView   *tview.TextView
	app        App
	projects   []*model.Project // mirrors list order; rebuilt on Refresh
	refreshing bool
}

// NewUnattachedPanel constructs the unattached projects screen.
func NewUnattachedPanel(app App) *UnattachedPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Unattached Projects ")

	info := tview.NewTextView().SetDynamicColors(true)
	info.SetBorder(true)
	info.SetTitle(" Project Details ")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(info, 7, 0, false)

	p := &UnattachedPanel{
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
		app.SetHints(HintsUnattachedPanel)
	})

	list.SetInputCapture(p.handleKey)

	return p
}

// Refresh repopulates the list from the current registry state.
func (p *UnattachedPanel) Refresh() {
	reg := p.app.Registry()
	all := reg.UnattachedProjects()
	sort.Slice(all, func(i, j int) bool {
		return all[i].Name() < all[j].Name()
	})

	var prevName string
	if proj := p.selectedProject(); proj != nil {
		prevName = proj.Name()
	}

	p.refreshing = true
	p.projects = all
	p.list.Clear()
	for _, proj := range p.projects {
		p.list.AddItem(unattachedItemText(proj), "", 0, nil)
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
func (p *UnattachedPanel) selectedProject() *model.Project {
	idx := p.list.GetCurrentItem()
	if idx < 0 || idx >= len(p.projects) {
		return nil
	}
	return p.projects[idx]
}

// renderInfo populates the detail panel for the selected project.
func (p *UnattachedPanel) renderInfo(proj *model.Project) {
	if proj == nil {
		p.infoView.SetText("  (no project selected)")
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "  [::b]Name[-::-]         %s\n", proj.Name())
	if proj.Description != "" {
		fmt.Fprintf(&sb, "  [::b]Description[-::-] %s\n", proj.Description)
	}
	if proj.Instructions != "" {
		fmt.Fprintf(&sb, "  [::b]Instructions[-::-] %s", proj.Instructions)
	}
	p.infoView.SetText(sb.String())
}

// handleKey processes keyboard input for this panel.
func (p *UnattachedPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	// Esc and 's' both return to the main screen.
	if event.Key() == KeyCancel {
		p.app.SwitchToMain()
		return nil
	}

	if event.Key() != tcell.KeyRune {
		return event
	}

	switch event.Rune() {
	case KeyUnattached:
		p.app.SwitchToMain()
		return nil

	case KeyNew:
		p.app.SetHints(HintsForm)
		p.app.ShowModal(NewProjectForm(p.app, nil, nil, func() {
			p.app.SetHints(HintsUnattachedPanel)
			p.app.HideModal()
		}))
		return nil

	case KeyEdit:
		if proj := p.selectedProject(); proj != nil {
			p.app.SetHints(HintsForm)
			p.app.ShowModal(NewProjectForm(p.app, proj, nil, func() {
				p.app.SetHints(HintsUnattachedPanel)
				p.app.HideModal()
			}))
		}
		return nil

	case KeyDelete:
		if proj := p.selectedProject(); proj != nil {
			name := proj.Name()
			p.app.SetHints(HintsConfirmModal)
			p.app.ShowModal(NewConfirmModal(
				fmt.Sprintf("Remove project\n%q?", name),
				func() {
					if err := p.app.Mutate(func(r *model.Registry) error {
						return r.RemoveProject(name)
					}); err != nil {
						p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
					}
					p.app.SetHints(HintsUnattachedPanel)
					p.app.HideModal()
				},
				func() {
					p.app.SetHints(HintsUnattachedPanel)
					p.app.HideModal()
				},
			))
		}
		return nil

	case KeyHelp:
		p.app.SetHints(HintsHelpOverlay)
		p.app.ShowModal(NewHelpOverlay(p.app))
		return nil

	case KeyQuit:
		p.app.SetHints(HintsConfirmModal)
		p.app.ShowModal(NewConfirmModal(
			"Quit llm-desk?",
			func() { p.app.Quit() },
			func() {
				p.app.SetHints(HintsUnattachedPanel)
				p.app.HideModal()
			},
		))
		return nil
	}
	return event
}

// unattachedItemText formats a project row for the unattached list.
func unattachedItemText(p *model.Project) string {
	name := p.Name()
	if p.Description != "" {
		return fmt.Sprintf("%-28s  %s", name, dimText(truncate(p.Description, 28)))
	}
	return name
}
