package views

import (
	"strings"

	"github.com/balinomad/llm-desk/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// entityScreenSpec configures a single-pane entity management screen.
type entityScreenSpec[T any] struct {
	items         func(*service.Service) []T
	key           func(T) string
	rowCells      func(T) []string
	detailRows    func(T) []infoRow
	form          func(App, T, func()) tview.Primitive
	deletePrompt  func(T) string
	remove        func(*service.Service, T) error
	listTitle     string
	infoTitle     string
	emptyDetail   string
	hintsSpecific string
	hintsCommon   string
}

// entityScreen is the shared list and detail layout for single-pane entity
// management screens.
type entityScreen[T any] struct {
	*tview.Flex
	app        App
	spec       entityScreenSpec[T]
	list       *tview.List
	infoView   *tview.TextView
	items      []T
	refreshing bool
}

func newEntityScreen[T any](app App, spec entityScreenSpec[T]) *entityScreen[T] {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	setListTitle(list, spec.listTitle, 0, 0)

	info := newInfoView(spec.infoTitle)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(info, 0, 0, false)

	s := &entityScreen[T]{
		Flex:     flex,
		app:      app,
		spec:     spec,
		list:     list,
		infoView: info,
	}

	list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		setListTitle(list, spec.listTitle, index, len(s.items))
		if s.refreshing {
			return
		}
		s.renderInfo(s.itemAt(index))
	})

	wirePanelFocus(list, func() {
		app.SetHints(spec.hintsSpecific, spec.hintsCommon)
	})

	list.SetInputCapture(s.handleKey)
	return s
}

// Refresh repopulates the list from the current service state.
func (s *entityScreen[T]) Refresh() {
	all := s.spec.items(s.app.Service())

	var prevKey string
	if item, ok := s.selected(); ok {
		prevKey = s.spec.key(item)
	}

	rawRows := make([][]string, len(all))
	for i, item := range all {
		rawRows[i] = s.spec.rowCells(item)
	}
	lines, _ := formatColumns(dropEmptyColumns(rawRows))

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
	setListTitle(s.list, s.spec.listTitle, s.list.GetCurrentItem(), len(s.items))
}

// RequiredInfoHeight returns the outer height needed to render the current
// detail text at outerWidth, including the border and wrapped lines.
func (s *entityScreen[T]) RequiredInfoHeight(outerWidth int) int {
	return infoTextHeight(s.infoView.GetText(false), outerWidth)
}

// ResizeInfo sizes the detail panel to its content at outerWidth, leaving at
// least minListPanelHeight rows for the list.
func (s *entityScreen[T]) ResizeInfo(outerWidth, availableHeight int) {
	s.ResizeItem(
		s.infoView,
		fitInfoHeight(s.RequiredInfoHeight(outerWidth), availableHeight, minListPanelHeight),
		0,
	)
}

func (s *entityScreen[T]) itemAt(index int) (T, bool) {
	if index < 0 || index >= len(s.items) {
		var zero T
		return zero, false
	}
	return s.items[index], true
}

func (s *entityScreen[T]) selected() (T, bool) {
	return s.itemAt(s.list.GetCurrentItem())
}

func (s *entityScreen[T]) renderInfo(item T, ok bool) {
	if !ok {
		s.infoView.SetText(s.spec.emptyDetail)
		return
	}
	var sb strings.Builder
	writeInfoRows(&sb, s.spec.detailRows(item))
	s.infoView.SetText(strings.TrimSuffix(sb.String(), "\n"))
}

func (s *entityScreen[T]) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case KeyCancel:
		s.app.SwitchToMain()
		return nil
	case KeyInsert:
		return s.handleNew()
	case KeyEdit:
		return s.handleEdit()
	case KeyDelete:
		return s.handleDelete()
	case KeyFocusNext, KeyFocusPrev:
		return nil
	default:
	}

	if result, ok := dispatchPanelKey(event, s.app, s.spec.hintsSpecific, s.spec.hintsCommon); ok {
		return result
	}
	return event
}

func (s *entityScreen[T]) handleNew() *tcell.EventKey {
	var zero T
	showModal(s.app, s.spec.form(s.app, zero, s.closeForm), HintsFormSpecific, HintsFormCommon)
	return nil
}

func (s *entityScreen[T]) handleEdit() *tcell.EventKey {
	item, ok := s.selected()
	if !ok {
		return nil
	}
	showModal(s.app, s.spec.form(s.app, item, s.closeForm), HintsFormSpecific, HintsFormCommon)
	return nil
}

func (s *entityScreen[T]) closeForm() {
	s.app.SetHints(s.spec.hintsSpecific, s.spec.hintsCommon)
	s.app.HideModal()
}

func (s *entityScreen[T]) handleDelete() *tcell.EventKey {
	item, ok := s.selected()
	if !ok {
		return nil
	}
	modal := NewConfirmModal(
		s.spec.deletePrompt(item),
		func() {
			if err := s.app.Mutate(func(svc *service.Service) error {
				return s.spec.remove(svc, item)
			}); err != nil {
				setErrorStatus(s.app, err)
			}
			s.closeForm()
		},
		s.closeForm,
	)
	showModal(s.app, modal, HintsConfirmModalSpecific, HintsConfirmModalCommon)
	return nil
}
