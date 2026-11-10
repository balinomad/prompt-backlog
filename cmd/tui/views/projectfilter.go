package views

import (
	"slices"
	"strings"

	"github.com/balinomad/prompt-backlog/internal/apperr"
	"github.com/balinomad/prompt-backlog/internal/model"

	"github.com/rivo/tview"
)

// projectOption is one granted project available to the filter.
type projectOption struct {
	id   uint64
	name string
}

// ProjectFilterState is the shared project filter for the main screen. The
// zero value is not ready for use; call NewProjectFilterState.
type ProjectFilterState struct {
	value      model.ProjectFilter
	granted    []projectOption
	accountKey model.AccountKey
	hasAccount bool
}

// NewProjectFilterState returns a filter set to All.
func NewProjectFilterState() *ProjectFilterState {
	return &ProjectFilterState{value: model.AllProjectFilter()}
}

// SetAccount updates the granted project list and applies the reset rules:
// the value becomes All when the account changes, and when the current value
// names a project that is no longer granted.
func (f *ProjectFilterState) SetAccount(key model.AccountKey, hasAccount bool, projects []*model.Project) error {
	if err := f.value.Validate(); err != nil {
		return err
	}
	if !hasAccount {
		if f.hasAccount {
			f.value = model.AllProjectFilter()
		}
		f.accountKey = model.AccountKey{}
		f.hasAccount = false
		f.granted = nil
		return nil
	}

	if !f.hasAccount || f.accountKey != key {
		f.value = model.AllProjectFilter()
	}
	f.accountKey = key
	f.hasAccount = true

	f.granted = f.granted[:0]
	seen := make(map[uint64]struct{}, len(projects))
	for _, project := range projects {
		if project == nil {
			return apperr.Invariantf("project filter: nil project for account %q", key.Email)
		}
		if _, ok := seen[project.ID()]; ok {
			continue
		}
		seen[project.ID()] = struct{}{}
		f.granted = append(f.granted, projectOption{id: project.ID(), name: project.Name()})
	}
	slices.SortFunc(f.granted, func(a, b projectOption) int {
		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})

	if id, ok := f.value.ProjectID(); ok && !f.hasGrantedID(id) {
		f.value = model.AllProjectFilter()
	}
	return nil
}

// Cycle advances the filter: All, each granted project in case-insensitive
// name order, None, then All again.
func (f *ProjectFilterState) Cycle() error {
	if err := f.value.Validate(); err != nil {
		return err
	}
	order := make([]model.ProjectFilter, 0, len(f.granted)+2)
	order = append(order, model.AllProjectFilter())
	for _, project := range f.granted {
		order = append(order, model.NamedProjectFilter(project.id))
	}
	order = append(order, model.NoneProjectFilter())

	for i, value := range order {
		same, err := SameProjectFilter(value, f.value)
		if err != nil {
			return err
		}
		if same {
			f.value = order[(i+1)%len(order)]
			return nil
		}
	}
	return apperr.Invariantf("project filter: current value is absent from the cycle order")
}

// Value returns the current filter.
func (f *ProjectFilterState) Value() model.ProjectFilter {
	return f.value
}

// TitleSuffix returns the title suffix for the current value, or an empty
// string for All.
func (f *ProjectFilterState) TitleSuffix() (string, error) {
	if err := f.value.Validate(); err != nil {
		return "", err
	}
	switch f.value.Kind() {
	case model.ProjectFilterAll:
		return "", nil
	case model.ProjectFilterNone:
		return " (no project)", nil
	case model.ProjectFilterNamed:
		id, err := namedProjectID(f.value)
		if err != nil {
			return "", err
		}
		for _, project := range f.granted {
			if project.id == id {
				return " (" + tview.Escape(TruncateWidth(project.name, ProjectColumnWidth)) + ")", nil
			}
		}
		return "", apperr.Invariantf("project filter: project %d is not granted", id)
	default:
		return "", apperr.Invariantf("project filter: unknown kind %d", f.value.Kind())
	}
}

func (f *ProjectFilterState) hasGrantedID(id uint64) bool {
	for _, project := range f.granted {
		if project.id == id {
			return true
		}
	}
	return false
}

// SameProjectFilter reports whether two filters select the same value.
func SameProjectFilter(a, b model.ProjectFilter) (bool, error) {
	if err := a.Validate(); err != nil {
		return false, err
	}
	if err := b.Validate(); err != nil {
		return false, err
	}
	if a.Kind() != b.Kind() {
		return false, nil
	}
	switch a.Kind() {
	case model.ProjectFilterAll, model.ProjectFilterNone:
		return true, nil
	case model.ProjectFilterNamed:
		aID, err := namedProjectID(a)
		if err != nil {
			return false, err
		}
		bID, err := namedProjectID(b)
		if err != nil {
			return false, err
		}
		return aID == bID, nil
	default:
		return false, apperr.Invariantf("project filter: unknown kind %d", a.Kind())
	}
}

func namedProjectID(f model.ProjectFilter) (uint64, error) {
	id, ok := f.ProjectID()
	if !ok {
		return 0, apperr.Invariantf("project filter: named filter has no project ID")
	}
	return id, nil
}
