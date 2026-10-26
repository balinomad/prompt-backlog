package views

import (
	"fmt"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/rivo/tview"
)

// emailColumnWidthCap bounds the email column in the Users list.
const emailColumnWidthCap = 32

// UsersPanel is a full-screen view for managing users.
type UsersPanel struct {
	*entityScreen[*model.User]
}

// NewUsersPanel constructs the users management screen.
func NewUsersPanel(app App) *UsersPanel {
	return &UsersPanel{
		entityScreen: newEntityScreen(app, entityScreenSpec[*model.User]{
			items: func(s *service.Service) []*model.User { return s.Users() },
			key:   func(u *model.User) string { return u.Email() },
			rowCells: func(u *model.User) []string {
				return []string{
					tview.Escape(truncateWidth(u.Name(), nameColumnWidthCap)),
					tview.Escape(truncateWidth(u.Email(), emailColumnWidthCap)),
				}
			},
			detailRows: func(u *model.User) []infoRow {
				return []infoRow{
					{label: "Name", value: tview.Escape(u.Name())},
					{label: "Email", value: tview.Escape(u.Email())},
				}
			},
			form: NewUserForm,
			deletePrompt: func(u *model.User) string {
				return fmt.Sprintf("Remove user %q?\nAll accounts for it must be removed first.", tview.Escape(u.Email()))
			},
			remove: func(s *service.Service, u *model.User) error {
				return s.DeleteUser(u.Email())
			},
			listTitle:     " Users ",
			infoTitle:     "User Details",
			emptyDetail:   "(no user selected)",
			hintsSpecific: HintsUsersPanelSpecific,
			hintsCommon:   HintsUsersPanelCommon,
		}),
	}
}
