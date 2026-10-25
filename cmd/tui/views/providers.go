package views

import (
	"fmt"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// providerListNameWidthCap bounds the provider-name column in the Providers
// list so a long name cannot push capabilities off screen.
const providerListNameWidthCap = 24

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

	info := newInfoView("Provider Details")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(info, 0, 0, false) // Height is set from content before each draw.

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

	wirePanelFocus(list, func() {
		app.SetHints(HintsProvidersPanelSpecific, HintsProvidersPanelCommon)
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

	rawRows := make([][]string, len(all))
	hasCapabilities := false
	for i, prov := range all {
		caps := providerCapabilityText(prov)
		hasCapabilities = hasCapabilities || caps != ""
		rawRows[i] = []string{
			tview.Escape(truncateWidth(prov.Name(), providerListNameWidthCap)),
			caps,
		}
	}

	rows := make([][]string, len(rawRows))
	for i, row := range rawRows {
		rows[i] = []string{row[0]}
		if hasCapabilities {
			rows[i] = append(rows[i], row[1])
		}
	}
	lines, _ := formatColumns(rows)

	p.refreshing = true
	p.providers = all
	p.list.Clear()
	for _, line := range lines {
		p.list.AddItem(line, "", 0, nil)
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

// RequiredInfoHeight returns the outer height needed to render the current
// detail text at outerWidth, including the border and wrapped lines.
func (p *ProvidersPanel) RequiredInfoHeight(outerWidth int) int {
	return infoTextHeight(p.infoView.GetText(false), outerWidth)
}

// ResizeInfo sizes the detail panel to its content at outerWidth, leaving at
// least minListPanelHeight rows for the list.
func (p *ProvidersPanel) ResizeInfo(outerWidth, availableHeight int) {
	p.ResizeItem(
		p.infoView,
		fitInfoHeight(p.RequiredInfoHeight(outerWidth), availableHeight, minListPanelHeight),
		0,
	)
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
		p.infoView.SetText("(no provider selected)")
		return
	}
	var sb strings.Builder
	writeInfoRows(&sb, []infoRow{
		{label: "Name", value: tview.Escape(prov.Name())},
		{label: "Projects", value: boolTag(prov.HasProjects())},
		{label: "Deep research", value: boolTag(prov.HasDeepResearch())},
	})
	p.infoView.SetText(strings.TrimSuffix(sb.String(), "\n"))
}

// handleKey processes keyboard input for this panel.
func (p *ProvidersPanel) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case KeyCancel:
		// Esc and 'r' both return to the main screen.
		p.app.SwitchToMain()
		return nil
	case KeyInsert:
		return p.handleNew()
	case KeyEdit:
		return p.handleEdit()
	case KeyDelete:
		return p.handleDelete()
	case KeyFocusNext, KeyFocusPrev:
		// The providers screen is a single-pane view; Tab must not move
		// focus into the list.
		return nil
	default:
		// Every other special key falls through to dispatchPanelKey and
		// the rune switch below.
	}

	if result, ok := dispatchPanelKey(event, p.app, HintsProvidersPanelSpecific, HintsProvidersPanelCommon); ok {
		return result
	}

	return event
}

func (p *ProvidersPanel) handleNew() *tcell.EventKey {
	modal := NewProviderForm(p.app, nil, func() {
		p.app.SetHints(HintsProvidersPanelSpecific, HintsProvidersPanelCommon)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsFormSpecific, HintsFormCommon)
	return nil
}

func (p *ProvidersPanel) handleEdit() *tcell.EventKey {
	prov := p.selectedProvider()
	if prov == nil {
		return nil
	}
	modal := NewProviderForm(p.app, prov, func() {
		p.app.SetHints(HintsProvidersPanelSpecific, HintsProvidersPanelCommon)
		p.app.HideModal()
	})
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsFormSpecific, HintsFormCommon)
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
			p.app.SetHints(HintsProvidersPanelSpecific, HintsProvidersPanelCommon)
			p.app.HideModal()
		},
		func() {
			p.app.SetHints(HintsProvidersPanelSpecific, HintsProvidersPanelCommon)
			p.app.HideModal()
		},
	)
	if err := p.app.ShowModal(modal); err != nil {
		p.app.SetStatus(fmt.Sprintf("[red]%v[-]", err))
		return nil
	}
	p.app.SetHints(HintsConfirmModalSpecific, HintsConfirmModalCommon)
	return nil
}

// ── Package-level helpers ─────────────────────────────────────────────────────

// providerCapabilityText returns the coloured capability list for one
// provider, or "" when it has none.
func providerCapabilityText(p *model.Provider) string {
	var tags []string
	if p.HasProjects() {
		tags = append(tags, "[green]projects[-]")
	}
	if p.HasDeepResearch() {
		tags = append(tags, "[cyan]deep-research[-]")
	}
	return strings.Join(tags, ", ")
}

// boolTag returns a coloured yes/no string for boolean capability fields.
func boolTag(v bool) string {
	if v {
		return "[green]yes[-]"
	}
	return "[::d]no[-::-]"
}
