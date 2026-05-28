package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProvidersPanel is a full-screen view for managing providers.
// The operator navigates to it with 'p' and returns to the main view with
// 'Esc' or 'p'.
type ProvidersPanel struct {
	*tview.Flex
	list       *tview.List
	infoView   *tview.TextView
	app        App
	providers  []*model.Provider // mirrors list order; rebuilt on Refresh
	refreshing bool
}

// NewProvidersPanel constructs the providers management screen.
func NewProvidersPanel(app App) *ProvidersPanel {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	list.SetTitle(" Providers ")

	info := tview.NewTextView().SetDynamicColors(true)
	info.SetBorder(true)
	info.SetTitle(" Provider Details ")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(info, 6, 0, false)

	p := &ProvidersPanel{
		Flex:     flex,
		list:     list,
		infoView: info,
		app:      app,
	}

	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if p.refreshing {
			return
		}
		p.renderInfo(p.selectedProvider())
	})

	list.SetFocusFunc(func() {
		app.SetHints(HintsProvidersPanel)
	})

	list.SetInputCapture(p.handleKey)

	return p
}

// Refresh repopulates the list from the current registry state.
func (p *ProvidersPanel) Refresh() {
	reg := p.app.Registry()
	all := reg.AllProviders()
	sort.Slice(all, func(i, j int) bool {
		return all[i].Name() < all[j].Name()
	})

	var prevName string
	if prov := p.selectedProvider(); prov != nil {
		prevName = prov.Name()
	}

	p.refreshing = true
	p.providers = all
	p.list.Clear()
	for _, prov := range p.providers {
		p.list.AddItem(providerItemText(prov), "", 0, nil)
	}
	p.refreshing = false

	// Restore selection by name.
	restored := false
	for i, prov := range p.providers {
		if prov.Name() == prevName {
			p.list.SetCurrentItem(i)
			restored = true
			break
		}
	}
	if !restored && len(p.providers) > 0 {
		p.list.SetCurrentItem(0)
	}

	p.renderInfo(p.selectedProvider())
}

// selectedProvider returns the currently highlighted provider, or nil when
// the list is empty.
func (p *ProvidersPanel) selectedProvider() *model.Provider {
	idx := p.list.GetCurrentItem()
	if idx < 0 || idx >= len(p.providers) {
		return nil
	}
	return p.providers[idx]
}

// renderInfo populates the detail panel for the selected provider.
func (p *ProvidersPanel) renderInfo(prov *model.Provider) {
	if prov == nil {
		p.infoView.SetText("  (no provider selected)")
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "  [::b]Name[-::-]            %s\n", prov.Name())
	fmt.Fprintf(&sb, "  [::b]Projects[-::-]        %s\n", boolTag(prov.HasProjects))
	fmt.Fprintf(&sb, "  [::b]Deep research[-::-]   %s", boolTag(prov.HasDeepResearch))
	p.infoView.SetText(sb.String())
}

// handleKey processes keyboard input for this panel.
func (p *ProvidersPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	// Esc and 'p' both return to the main screen.
	if event.Key() == KeyCancel {
		p.app.SwitchToMain()
		return nil
	}

	if event.Key() != tcell.KeyRune {
		return event
	}

	switch event.Rune() {
	case KeyProviders:
		p.app.SwitchToMain()
		return nil

	case KeyNew:
		p.app.SetHints(HintsForm)
		p.app.ShowModal(NewProviderForm(p.app, nil, func() {
			p.app.SetHints(HintsProvidersPanel)
			p.app.HideModal()
		}))
		return nil

	case KeyEdit:
		if prov := p.selectedProvider(); prov != nil {
			p.app.SetHints(HintsForm)
			p.app.ShowModal(NewProviderForm(p.app, prov, func() {
				p.app.SetHints(HintsProvidersPanel)
				p.app.HideModal()
			}))
		}
		return nil

	case KeyDelete:
		if prov := p.selectedProvider(); prov != nil {
			name := prov.Name()
			p.app.SetHints(HintsConfirmModal)
			p.app.ShowModal(NewConfirmModal(
				fmt.Sprintf("Remove provider %q?\nAll sessions using it must be removed first.", name),
				func() {
					if err := p.app.Mutate(func(r *model.Registry) error {
						return r.RemoveProvider(name)
					}); err != nil {
						p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
					}
					p.app.SetHints(HintsProvidersPanel)
					p.app.HideModal()
				},
				func() {
					p.app.SetHints(HintsProvidersPanel)
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
				p.app.SetHints(HintsProvidersPanel)
				p.app.HideModal()
			},
		))
		return nil
	}
	return event
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// providerItemText formats a provider row for the list.
func providerItemText(p *model.Provider) string {
	var tags []string
	if p.HasProjects {
		tags = append(tags, "[green]projects[-]")
	}
	if p.HasDeepResearch {
		tags = append(tags, "[cyan]deep-research[-]")
	}
	if len(tags) == 0 {
		return p.Name()
	}
	return fmt.Sprintf("%-24s  %s", p.Name(), strings.Join(tags, "  "))
}

// boolTag returns a coloured yes/no string for boolean capability fields.
func boolTag(v bool) string {
	if v {
		return "[green]yes[-]"
	}
	return "[::d]no[-::-]"
}
