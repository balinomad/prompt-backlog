package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── detailEntry ───────────────────────────────────────────────────────────────

// detailEntry tracks what each row in the projects list represents.
type detailEntry struct {
	project *model.Project // non-nil for project rows
	isLastQ bool           // true for the LastQuestion row
	isSep   bool           // true for the visual separator (not selectable)
}

// ── DetailPanel ───────────────────────────────────────────────────────────────

// DetailPanel occupies the right side of the screen. The upper section lists
// the selected session's projects followed by its LastQuestion; the lower
// section shows static session information (user, provider, next-prompt time).
type DetailPanel struct {
	*tview.Flex
	projectsList *tview.List
	infoView     *tview.TextView
	app          App
	session      *model.Session // currently displayed session; may be nil
	entries      []detailEntry  // parallel to projectsList items; rebuilt on Refresh
	prevListIdx  int            // used to detect navigation direction for separator skip
	refreshing   bool           // suppresses callbacks during Refresh
}

// NewDetailPanel constructs the detail panel. Call Refresh after the sessions
// panel has provided an initial selection.
func NewDetailPanel(app App) *DetailPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Projects ")

	info := tview.NewTextView().SetDynamicColors(true)
	info.SetBorder(true)
	info.SetTitle(" Session ")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(info, 7, 0, false) // fixed 7-row info panel (5 lines + 2 border)

	p := &DetailPanel{
		Flex:         flex,
		projectsList: list,
		infoView:     info,
		app:          app,
	}

	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if p.refreshing {
			return
		}
		p.skipSeparator(index)
	})

	list.SetFocusFunc(func() {
		app.SetHints(HintsProjectsPanel)
	})

	list.SetInputCapture(p.handleKey)

	return p
}

// SetSession sets the session to display and refreshes the panel.
// Called by the host App when the sessions panel selection changes.
func (p *DetailPanel) SetSession(s *model.Session) {
	p.session = s
	p.Refresh()
}

// Refresh repopulates the panel from the current registry state.
// The session reference is re-validated against the registry so that a
// deleted session gracefully clears the panel rather than showing stale data.
func (p *DetailPanel) Refresh() {
	reg := p.app.Registry()

	// Re-validate: look the session up by identity in case it was deleted.
	var session *model.Session
	if p.session != nil {
		session = reg.SessionAt(
			p.session.User().Email(),
			p.session.Provider().Name(),
		)
	}
	p.session = session

	p.refreshing = true
	p.rebuildList(reg, session)
	p.refreshing = false

	p.renderInfo(session)
}

// selectedEntry returns the detailEntry for the currently highlighted row,
// or a zero-value entry when the list is empty or a separator is selected.
func (p *DetailPanel) selectedEntry() detailEntry {
	idx := p.projectsList.GetCurrentItem()
	if idx < 0 || idx >= len(p.entries) {
		return detailEntry{}
	}
	return p.entries[idx]
}

// ── Private: list population ──────────────────────────────────────────────────

func (p *DetailPanel) rebuildList(reg model.Registry, session *model.Session) {
	// Remember selection before clearing.
	prevEntry := p.selectedEntry()

	p.entries = nil
	p.projectsList.Clear()

	if session == nil {
		return
	}

	// Projects for this session, sorted by name.
	projects := reg.ProjectsBySession(session)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name() < projects[j].Name()
	})
	for _, proj := range projects {
		p.entries = append(p.entries, detailEntry{project: proj})
		p.projectsList.AddItem(projectItemText(proj), "", 0, nil)
	}

	// Separator.
	sep := strings.Repeat("─", 38)
	p.entries = append(p.entries, detailEntry{isSep: true})
	p.projectsList.AddItem(sep, "", 0, nil)

	// LastQuestion row.
	p.entries = append(p.entries, detailEntry{isLastQ: true})
	p.projectsList.AddItem(lastQuestionItemText(session.LastQuestion), "", 0, nil)

	// Restore selection by matching type and name.
	restored := false
	for i, e := range p.entries {
		if e.isSep {
			continue
		}
		if prevEntry.isLastQ && e.isLastQ {
			p.projectsList.SetCurrentItem(i)
			restored = true
			break
		}
		if prevEntry.project != nil && e.project != nil &&
			e.project.Name() == prevEntry.project.Name() {
			p.projectsList.SetCurrentItem(i)
			restored = true
			break
		}
	}
	if !restored {
		// Default to first selectable entry.
		for i, e := range p.entries {
			if !e.isSep {
				p.projectsList.SetCurrentItem(i)
				break
			}
		}
	}
	p.prevListIdx = p.projectsList.GetCurrentItem()
}

// skipSeparator is called by SetChangedFunc. When the newly selected index is
// the separator, it advances or retreats one step depending on travel direction.
func (p *DetailPanel) skipSeparator(index int) {
	if index < 0 || index >= len(p.entries) {
		return
	}
	if !p.entries[index].isSep {
		p.prevListIdx = index
		return
	}
	// Determine direction from previous index.
	next := index + 1
	if index < p.prevListIdx {
		next = index - 1
	}
	if next >= 0 && next < len(p.entries) {
		p.projectsList.SetCurrentItem(next)
	}
}

// ── Private: rendering ────────────────────────────────────────────────────────

func (p *DetailPanel) renderInfo(session *model.Session) {
	if session == nil {
		p.infoView.SetText("  (no session selected)")
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "  [::b]User[-::-]     %s (%s)\n",
		session.User().Name, session.User().Email())
	fmt.Fprintf(&sb, "  [::b]Provider[-::-] %s\n", session.Provider().Name())
	if t := session.NextPromptAt; t != nil {
		timeStr, isPast := formatNextPromptAt(t)
		if isPast {
			fmt.Fprintf(&sb, "  [::b]Next[-::-]     [red]%s (ready)[-]", timeStr)
		} else {
			fmt.Fprintf(&sb, "  [::b]Next[-::-]     %s", timeStr)
		}
	} else {
		fmt.Fprintf(&sb, "  [::b]Next[-::-]     [green]available[-]")
	}
	p.infoView.SetText(sb.String())
}

// ── Private: key handling ─────────────────────────────────────────────────────

func (p *DetailPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	entry := p.selectedEntry()

	switch event.Rune() {
	case KeyNew:
		if p.session != nil {
			p.app.SetHints(HintsForm)
			p.app.ShowModal(NewProjectForm(p.app, nil, p.session, func() {
				p.app.SetHints(HintsProjectsPanel)
				p.app.HideModal()
			}))
		}
		return nil

	case KeyEdit:
		switch {
		case entry.project != nil:
			p.app.SetHints(HintsForm)
			p.app.ShowModal(NewProjectForm(p.app, entry.project, p.session, func() {
				p.app.SetHints(HintsProjectsPanel)
				p.app.HideModal()
			}))
		case entry.isLastQ:
			p.app.SetHints(HintsForm)
			p.app.ShowModal(NewLastQuestionForm(p.app, p.session, func() {
				p.app.SetHints(HintsProjectsPanel)
				p.app.HideModal()
			}))
		}
		return nil

	case KeyDelete:
		switch {
		case entry.project != nil:
			name := entry.project.Name()
			p.app.SetHints(HintsConfirmModal)
			p.app.ShowModal(NewConfirmModal(
				fmt.Sprintf("Remove project\n%q?", name),
				func() {
					if err := p.app.Mutate(func(r *model.Registry) error {
						return r.RemoveProject(name)
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
			))
		case entry.isLastQ && p.session != nil && p.session.LastQuestion != nil:
			p.app.SetHints(HintsConfirmModal)
			p.app.ShowModal(NewConfirmModal(
				"Clear last question?",
				func() {
					email := p.session.User().Email()
					provider := p.session.Provider().Name()
					if err := p.app.Mutate(func(r *model.Registry) error {
						s := r.SessionAt(email, provider)
						if s == nil {
							return fmt.Errorf("session not found")
						}
						s.LastQuestion = nil
						return nil
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
			))
		}
		return nil

	case KeyToggle:
		// Toggle MidAnswer on the LastQuestion only.
		if entry.isLastQ && p.session != nil {
			email := p.session.User().Email()
			provider := p.session.Provider().Name()
			if err := p.app.Mutate(func(r *model.Registry) error {
				s := r.SessionAt(email, provider)
				if s == nil {
					return fmt.Errorf("session not found")
				}
				if s.LastQuestion == nil {
					s.LastQuestion = model.NewLastQuestion(nil)
				}
				s.LastQuestion.MidAnswer = !s.LastQuestion.MidAnswer
				return nil
			}); err != nil {
				p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
			}
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
				p.app.SetHints(HintsProjectsPanel)
				p.app.HideModal()
			},
		))
		return nil
	}
	return event
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// projectItemText formats a project row for the list.
func projectItemText(p *model.Project) string {
	name := p.Name()
	if p.Description != "" {
		return fmt.Sprintf("%-28s  %s", name, dimText(truncate(p.Description, 20)))
	}
	return name
}

// lastQuestionItemText formats the LastQuestion row for the list.
// When lq is nil, a placeholder is shown.
func lastQuestionItemText(lq *model.LastQuestion) string {
	if lq == nil {
		return dimText("(no last question)")
	}
	var parts []string
	if lq.Topic != "" {
		parts = append(parts, lq.Topic)
	}
	if lq.MidAnswer {
		parts = append(parts, "[yellow][mid-answer][-]")
	}
	if lq.NextPrompt != "" {
		parts = append(parts, dimText("→ "+truncate(lq.NextPrompt, 24)))
	}
	if len(parts) == 0 {
		return dimText("(last question: no details)")
	}
	return strings.Join(parts, "  ")
}

// truncate shortens s to max runes, appending "…" when truncated.
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// dimText wraps s in tview's dim color tag.
func dimText(s string) string {
	return "[::d]" + s + "[-::-]"
}
