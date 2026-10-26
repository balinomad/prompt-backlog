package views

import (
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
	*entityScreen[*model.Vendor]
}

// NewVendorsPanel constructs the vendors management screen.
func NewVendorsPanel(app App) *VendorsPanel {
	return &VendorsPanel{
		entityScreen: newEntityScreen(app, entityScreenSpec[*model.Vendor]{
			items: func(s *service.Service) []*model.Vendor { return s.Vendors() },
			key:   func(p *model.Vendor) string { return p.Name() },
			rowCells: func(p *model.Vendor) []string {
				return []string{
					tview.Escape(truncateWidth(p.Name(), vendorListNameWidthCap)),
					vendorCapabilityText(p),
				}
			},
			detailRows: func(p *model.Vendor) []infoRow {
				return []infoRow{
					{label: "Name", value: tview.Escape(p.Name())},
					{label: "Projects", value: boolTag(p.HasProjects())},
					{label: "Deep research", value: boolTag(p.HasDeepResearch())},
				}
			},
			form: NewVendorForm,
			deletePrompt: func(p *model.Vendor) string {
				return fmt.Sprintf("Remove vendor %q?\nAll accounts using it must be removed first.", tview.Escape(p.Name()))
			},
			remove: func(s *service.Service, p *model.Vendor) error {
				return s.DeleteVendor(p.Name())
			},
			listTitle:     " Vendors ",
			infoTitle:     "Vendor Details",
			emptyDetail:   "(no vendor selected)",
			hintsSpecific: HintsVendorsPanelSpecific,
			hintsCommon:   HintsVendorsPanelCommon,
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
	return dimText("no")
}
