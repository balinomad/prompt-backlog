package panels

import (
	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/cmd/tui/views/forms"

	"fmt"

	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/rivo/tview"
)

// emailColumnWidthCap bounds the email column in the Users list.
const emailColumnWidthCap = 32

// UsersPanel is a full-screen view for managing users.
type UsersPanel struct {
	*panel[*model.User]
}

// NewUsersPanel constructs the users management screen.
func NewUsersPanel(app views.Host) *UsersPanel {
	return &UsersPanel{
		panel: newPanel(app, panelSpec[*model.User]{
			items: func(s *service.Service) []*model.User { return s.Users() },
			key:   func(u *model.User) string { return u.Email() },
			rowCells: func(u *model.User) []string {
				return []string{
					tview.Escape(views.TruncateWidth(u.Name(), views.NameColumnWidthCap)),
					tview.Escape(views.TruncateWidth(u.Email(), emailColumnWidthCap)),
				}
			},
			detailRows: func(u *model.User) []views.InfoRow {
				return []views.InfoRow{
					{Label: "Name", Value: tview.Escape(u.Name())},
					{Label: "Email", Value: tview.Escape(u.Email())},
				}
			},
			form: forms.NewUserForm,
			deletePrompt: func(u *model.User) string {
				return fmt.Sprintf("Remove user %q?\nAll accounts for it must be removed first.", tview.Escape(u.Email()))
			},
			remove: func(s *service.Service, u *model.User) error {
				return s.DeleteUser(u.Email())
			},
			listTitle:     " Users ",
			infoTitle:     "User Details",
			emptyDetail:   "(no user selected)",
			hintsSpecific: views.HintsUsersPanelSpecific,
			hintsCommon:   views.HintsUsersPanelCommon,
		}),
	}
}
