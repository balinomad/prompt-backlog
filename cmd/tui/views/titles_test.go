package views_test

import (
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/rivo/tview"
)

func TestListTitles_AccountsPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()

	if got, want := p.GetTitle(), " Accounts 1/2 "; got != want {
		t.Errorf("initial title = %q, want %q", got, want)
	}
	p.SetCurrentItem(1)
	if got, want := p.GetTitle(), " Accounts 2/2 "; got != want {
		t.Errorf("selected title = %q, want %q", got, want)
	}

	app.svc = newTestService(t, model.NewRegistry())
	p.Refresh()
	if got, want := p.GetTitle(), " Accounts 0/0 "; got != want {
		t.Errorf("empty title = %q, want %q", got, want)
	}
}

func TestListTitles_DetailPanel(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	_, err = reg.AddSession(acc, nil, "one", nil)
	must(t, err)
	_, err = reg.AddSession(acc, nil, "two", nil)
	must(t, err)

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewDetailPanel(app)
	p.SetAccount(acc)
	if got, want := p.GetTitle(), " Sessions 1/2 "; got != want {
		t.Errorf("initial title = %q, want %q", got, want)
	}
	p.SetCurrentItem(1)
	if got, want := p.GetTitle(), " Sessions 2/2 "; got != want {
		t.Errorf("selected title = %q, want %q", got, want)
	}
	p.SetAccount(nil)
	if got, want := p.GetTitle(), " Sessions 0/0 "; got != want {
		t.Errorf("empty title = %q, want %q", got, want)
	}
}

func TestListTitles_EntityScreens(t *testing.T) {
	t.Parallel()
	t.Run("vendors", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := views.NewVendorsPanel(app)
		p.Refresh()
		list, _ := vendorsPanelParts(t, p)
		if got, want := list.GetTitle(), " Vendors 1/2 "; got != want {
			t.Errorf("initial title = %q, want %q", got, want)
		}
		list.SetCurrentItem(1)
		if got, want := list.GetTitle(), " Vendors 2/2 "; got != want {
			t.Errorf("selected title = %q, want %q", got, want)
		}
		app.svc = newTestService(t, model.NewRegistry())
		p.Refresh()
		if got, want := list.GetTitle(), " Vendors 0/0 "; got != want {
			t.Errorf("empty title = %q, want %q", got, want)
		}
	})

	t.Run("users", func(t *testing.T) {
		t.Parallel()
		app := newMockApp(t)
		p := views.NewUsersPanel(app)
		p.Refresh()
		list, _ := usersPanelParts(t, p)
		if got, want := list.GetTitle(), " Users 1/2 "; got != want {
			t.Errorf("initial title = %q, want %q", got, want)
		}
		list.SetCurrentItem(1)
		if got, want := list.GetTitle(), " Users 2/2 "; got != want {
			t.Errorf("selected title = %q, want %q", got, want)
		}
		app.svc = newTestService(t, model.NewRegistry())
		p.Refresh()
		if got, want := list.GetTitle(), " Users 0/0 "; got != want {
			t.Errorf("empty title = %q, want %q", got, want)
		}
	})
}

func TestListTitles_ProjectsPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewProjectsPanel(app)
	p.Refresh()
	projectList, accountList, _ := projectsPanelParts(t, p)

	if got, want := projectList.GetTitle(), " Projects 1/3 "; got != want {
		t.Errorf("initial project title = %q, want %q", got, want)
	}
	if got, want := accountList.GetTitle(), " Project Accounts 1/1 "; got != want {
		t.Errorf("initial account title = %q, want %q", got, want)
	}

	projectList.SetCurrentItem(2)
	if got, want := projectList.GetTitle(), " Projects 3/3 "; got != want {
		t.Errorf("selected project title = %q, want %q", got, want)
	}
	if got, want := accountList.GetTitle(), " Project Accounts 0/0 "; got != want {
		t.Errorf("empty account title = %q, want %q", got, want)
	}
}

func TestListTitles_QueueEditor(t *testing.T) {
	t.Parallel()
	app, acc, sess := newQueueTestApp(t)
	result := views.NewQueueEditor(app, acc, sess, func() {})
	list, ok := result.(*tview.List)
	if !ok {
		t.Fatalf("expected *tview.List, got %T", result)
	}
	if got, want := list.GetTitle(), " Messages: session #1 1/3 "; got != want {
		t.Errorf("initial title = %q, want %q", got, want)
	}
	list.SetCurrentItem(2)
	if got, want := list.GetTitle(), " Messages: session #1 3/3 "; got != want {
		t.Errorf("selected title = %q, want %q", got, want)
	}
}
