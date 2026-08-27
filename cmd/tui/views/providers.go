package views

import (
	"fmt"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProvidersPanel is a full-screen view for managing providers.
// The operator navigates to it with 'r' and returns to the main view with
// 'Esc' or 'r'.
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

// Refresh repopulates the list from the current service state.
func (p *ProvidersPanel) Refresh() {
	svc := p.app.Service()
	all := svc.Providers() // already sorted by name

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
	fmt.Fprintf(&sb, "  [::b]Name[-::-]            %s\n", tview.Escape(prov.Name()))
	fmt.Fprintf(&sb, "  [::b]Projects[-::-]        %s\n", boolTag(prov.HasProjects()))
	fmt.Fprintf(&sb, "  [::b]Deep research[-::-]   %s", boolTag(prov.HasDeepResearch()))
	p.infoView.SetText(sb.String())
}

// handleKey processes keyboard input for this panel.
func (p *ProvidersPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	// Esc and 'r' both return to the main screen.
	if event.Key() == KeyCancel {
		p.app.SwitchToMain()
		return nil
	}
	if result, ok := dispatchPanelKey(event, p.app, HintsProvidersPanel); ok {
		return result
	}

	switch event.Rune() {
	case KeyProviders:
		p.app.SwitchToMain()
		return nil
	case KeyNew:
		return p.handleNew()
	case KeyEdit:
		return p.handleEdit()
	case KeyDelete:
		return p.handleDelete()
	}
	return event
}

func (p *ProvidersPanel) handleNew() *tcell.EventKey {
	modal := NewProviderForm(p.app, nil, func() {
		p.app.SetHints(HintsProvidersPanel)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsForm)
	return nil
}

func (p *ProvidersPanel) handleEdit() *tcell.EventKey {
	prov := p.selectedProvider()
	if prov == nil {
		return nil
	}
	modal := NewProviderForm(p.app, prov, func() {
		p.app.SetHints(HintsProvidersPanel)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsForm)
	return nil
}

func (p *ProvidersPanel) handleDelete() *tcell.EventKey {
	prov := p.selectedProvider()
	if prov == nil {
		return nil
	}
	name := prov.Name()
	modal := NewConfirmModal(
		fmt.Sprintf("Remove provider %q?\nAll accounts using it must be removed first.", tview.Escape(name)),
		func() {
			if err := p.app.Mutate(func(s *service.Service) error {
				return s.DeleteProvider(name)
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
	)
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsConfirmModal)
	return nil
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// providerItemText formats a provider row for the list.
func providerItemText(p *model.Provider) string {
	name := tview.Escape(p.Name())
	var tags []string
	if p.HasProjects() {
		tags = append(tags, "[green]projects[-]")
	}
	if p.HasDeepResearch() {
		tags = append(tags, "[cyan]deep-research[-]")
	}
	if len(tags) == 0 {
		return name
	}
	return fmt.Sprintf("%-24s  %s", name, strings.Join(tags, "  "))
}

// boolTag returns a coloured yes/no string for boolean capability fields.
func boolTag(v bool) string {
	if v {
		return "[green]yes[-]"
	}
	return "[::d]no[-::-]"
}
