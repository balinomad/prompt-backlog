package panels

import (
	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/cmd/tui/views/forms"

	"fmt"
	"strings"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/rivo/tview"
)

// vendorListNameWidthCap bounds the vendor-name column in the Vendors
// list so a long name cannot push capabilities off screen.
const vendorListNameWidthCap = 24

// VendorsPanel is a full-screen view for managing vendors.
// The operator navigates to it with 'v' and returns to the main view with Esc.
type VendorsPanel struct {
	*panel[*model.Vendor]
}

// NewVendorsPanel constructs the vendors management screen.
func NewVendorsPanel(app views.Host) *VendorsPanel {
	return &VendorsPanel{
		panel: newPanel(app, panelSpec[*model.Vendor]{
			items: func(s *service.Service) []*model.Vendor { return s.Vendors() },
			key:   func(p *model.Vendor) string { return p.Name() },
			rowCells: func(p *model.Vendor) []string {
				return []string{
					tview.Escape(views.TruncateWidth(p.Name(), vendorListNameWidthCap)),
					vendorCapabilityText(p),
				}
			},
			detailRows: func(p *model.Vendor) []views.InfoRow {
				return []views.InfoRow{
					{Label: "Name", Value: tview.Escape(p.Name())},
					{Label: "Projects", Value: boolTag(p.HasProjects())},
					{Label: "Deep research", Value: boolTag(p.HasDeepResearch())},
				}
			},
			form: forms.NewVendorForm,
			deletePrompt: func(p *model.Vendor) string {
				return fmt.Sprintf("Remove vendor %q?\nAll accounts using it must be removed first.", tview.Escape(p.Name()))
			},
			remove: func(s *service.Service, p *model.Vendor) error {
				return s.DeleteVendor(p.Name())
			},
			listTitle:     " Vendors ",
			infoTitle:     "Vendor Details",
			emptyDetail:   "(no vendor selected)",
			hintsSpecific: views.HintsVendorsPanelSpecific,
			hintsCommon:   views.HintsVendorsPanelCommon,
		}),
	}
}

// vendorCapabilityText returns the coloured capability list for one
// vendor, or "" when it has none.
func vendorCapabilityText(p *model.Vendor) string {
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
	return views.DimText("no")
}
