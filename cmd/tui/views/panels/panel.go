package panels

import (
	"github.com/balinomad/llm-desk/cmd/tui/views"

	"strings"

	"github.com/balinomad/llm-desk/internal/model"
	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// resolvePanelAccount resolves account and synchronises filter with its
// granted projects. A stale or deleted account resolves to nil.
func resolvePanelAccount(svc *service.Service, account *model.Account, filter *views.ProjectFilterState) *model.Account {
	var acc *model.Account
	if account != nil {
		resolved, err := svc.Account(account.UserEmail(), account.VendorName())
		if err == nil {
			acc = resolved
		}
	}
	if acc != nil {
		projects, err := svc.AccountProjects(acc.UserEmail(), acc.VendorName())
		if err != nil {
			projects = nil
		}
		filter.SetAccount(acc.Key(), true, projects)
	} else {
		filter.SetAccount(model.AccountKey{}, false, nil)
	}
	return acc
}

// panelSpec configures a single-pane list and detail panel.
type panelSpec[T any] struct {
	items         func(*service.Service) []T
	key           func(T) string
	rowCells      func(T) []string
	detailRows    func(T) []views.InfoRow
	form          func(views.Host, T, func()) tview.Primitive
	deletePrompt  func(T) string
	remove        func(*service.Service, T) error
	listTitle     string
	infoTitle     string
	emptyDetail   string
	hintsSpecific string
	hintsCommon   string
}

// panel is the shared list and detail layout for single-pane management
// panels.
type panel[T any] struct {
	*tview.Flex
	app        views.Host
	spec       panelSpec[T]
	list       *tview.List
	infoView   *tview.TextView
	items      []T
	refreshing bool
}

func newPanel[T any](app views.Host, spec panelSpec[T]) *panel[T] {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	views.SetListTitle(list, spec.listTitle, 0, 0)

	info := views.NewInfoView(spec.infoTitle)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(info, 0, 0, false)

	s := &panel[T]{
		Flex:     flex,
		app:      app,
		spec:     spec,
		list:     list,
		infoView: info,
	}

	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		views.SetListTitle(list, spec.listTitle, index, len(s.items))
		if s.refreshing {
			return
		}
		s.renderInfo(s.itemAt(index))
	})

	views.WirePanelFocus(list, func() {
		app.SetHints(spec.hintsSpecific, spec.hintsCommon)
	})

	list.SetInputCapture(s.handleKey)
	views.WireListDoubleClick(list, func() { s.handleEdit() })
	return s
}

// Refresh repopulates the list from the current service state.
func (s *panel[T]) Refresh() {
	all := s.spec.items(s.app.Service())

	var prevKey string
	if item, ok := s.selected(); ok {
		prevKey = s.spec.key(item)
	}

	rawRows := make([][]string, len(all))
	for i, item := range all {
		rawRows[i] = s.spec.rowCells(item)
	}
	lines, _ := views.FormatColumns(views.DropEmptyColumns(rawRows))

	s.refreshing = true
	s.items = all
	s.list.Clear()
	for _, line := range lines {
		s.list.AddItem(line, "", 0, nil)
	}
	s.refreshing = false

	restored := false
	for i, item := range s.items {
		if s.spec.key(item) == prevKey {
			s.list.SetCurrentItem(i)
			restored = true
			break
		}
	}
	if !restored && len(s.items) > 0 {
		s.list.SetCurrentItem(0)
	}
	s.renderInfo(s.selected())
	views.SetListTitle(s.list, s.spec.listTitle, s.list.GetCurrentItem(), len(s.items))
}

// RequiredInfoHeight returns the outer height needed to render the current
// detail text at outerWidth, including the border and wrapped lines.
func (s *panel[T]) RequiredInfoHeight(outerWidth int) int {
	return views.InfoTextHeight(s.infoView.GetText(false), outerWidth)
}

// ResizeInfo sizes the detail panel to its content at outerWidth, leaving at
// least views.MinListPanelHeight rows for the list.
func (s *panel[T]) ResizeInfo(outerWidth, availableHeight int) {
	s.ResizeItem(
		s.infoView,
		views.FitInfoHeight(s.RequiredInfoHeight(outerWidth), availableHeight, views.MinListPanelHeight),
		0,
	)
}

func (s *panel[T]) itemAt(index int) (T, bool) {
	if index < 0 || index >= len(s.items) {
		var zero T
		return zero, false
	}
	return s.items[index], true
}

func (s *panel[T]) selected() (T, bool) {
	return s.itemAt(s.list.GetCurrentItem())
}

func (s *panel[T]) renderInfo(item T, ok bool) {
	if !ok {
		s.infoView.SetText(s.spec.emptyDetail)
		return
	}
	var sb strings.Builder
	views.WriteInfoRows(&sb, s.spec.detailRows(item))
	s.infoView.SetText(strings.TrimSuffix(sb.String(), "\n"))
}

func (s *panel[T]) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case views.KeyCancel:
		s.app.SwitchToMain()
		return nil
	case views.KeyInsert:
		return s.handleNew()
	case views.KeyEdit:
		return s.handleEdit()
	case views.KeyDelete:
		return s.handleDelete()
	case views.KeyFocusNext, views.KeyFocusPrev:
		return nil
	default:
	}

	if result, ok := views.DispatchPanelKey(event, s.app, s.spec.hintsSpecific, s.spec.hintsCommon); ok {
		return result
	}
	return event
}

func (s *panel[T]) handleNew() *tcell.EventKey {
	var zero T
	views.ShowModal(s.app, s.spec.form(s.app, zero, s.closeForm), views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (s *panel[T]) handleEdit() *tcell.EventKey {
	item, ok := s.selected()
	if !ok {
		return nil
	}
	views.ShowModal(s.app, s.spec.form(s.app, item, s.closeForm), views.HintsFormSpecific, views.HintsTextFormCommon)
	return nil
}

func (s *panel[T]) closeForm() {
	s.app.SetHints(s.spec.hintsSpecific, s.spec.hintsCommon)
	s.app.HideModal()
}

func (s *panel[T]) handleDelete() *tcell.EventKey {
	item, ok := s.selected()
	if !ok {
		return nil
	}
	modal := views.NewConfirmModal(
		s.spec.deletePrompt(item),
		func() {
			if err := s.app.Mutate(func(svc *service.Service) error {
				return s.spec.remove(svc, item)
			}); err != nil {
				views.SetErrorStatus(s.app, err)
			}
			s.closeForm()
		},
		s.closeForm,
	)
	views.ShowModal(s.app, modal, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)
	return nil
}
