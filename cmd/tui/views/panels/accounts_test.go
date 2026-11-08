package panels_test

import (
	"strings"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/cmd/tui/views/panels"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── construction ──────────────────────────────────────────────────────────────

func TestNewAccountsPanel(t *testing.T) {
	t.Parallel()
	if p := panels.NewAccountsPanel(newFakeApp(t)); p == nil {
		t.Fatal("expected non-nil AccountsPanel")
	}
}

func TestAccountsPanel_SelectedBeforeRefresh(t *testing.T) {
	t.Parallel()
	p := panels.NewAccountsPanel(newFakeApp(t))
	if a := p.Selected(); a != nil {
		t.Errorf("expected nil Selected before Refresh, got %v", a)
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestAccountsPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*testing.T, *fakeApp)
		wantCount int
	}{
		{
			name:      "two accounts",
			setup:     func(_ *testing.T, _ *fakeApp) {},
			wantCount: 2,
		},
		{
			name: "empty registry",
			setup: func(t *testing.T, app *fakeApp) {
				t.Helper()
				reg := model.NewRegistry()
				must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
				app.Svc = newService(t, reg)
			},
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			tc.setup(t, app)
			p := panels.NewAccountsPanel(app)
			p.Refresh()
			if got := p.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestAccountsPanel_Refresh_SelectedNotNil(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()
	if p.Selected() == nil {
		t.Error("expected non-nil Selected after Refresh")
	}
}

func TestAccountsPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()
	p.SetCurrentItem(1)

	want := p.Selected()
	if want == nil {
		t.Fatal("second item should not be nil")
	}
	email := want.UserEmail()
	vendor := want.VendorName()

	p.Refresh()

	got := p.Selected()
	if got == nil {
		t.Fatal("selection should survive Refresh")
	}
	if got.UserEmail() != email || got.VendorName() != vendor {
		t.Errorf("selection changed: got %s/%s, want %s/%s",
			got.UserEmail(), got.VendorName(), email, vendor)
	}
}

func TestAccountsPanel_Refresh_CallsOnSelect(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	var called bool
	p.SetOnSelect(func(_ *model.Account) { called = true })
	p.Refresh()
	if !called {
		t.Error("onSelect should be called after Refresh")
	}
}

// TestAccountsPanel_Refresh_ClearsAccountInfoOnLastAccountDeleted wires the
// accounts selection to the account info panel, then deletes the only
// account. The panel must clear.
func TestAccountsPanel_Refresh_ClearsAccountInfoOnLastAccountDeleted(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)

	app := &fakeApp{Svc: newService(t, reg)}
	accounts := panels.NewAccountsPanel(app)
	accountInfo := panels.NewAccountInfoPanel(app)
	accounts.SetOnSelect(func(acc *model.Account) {
		accountInfo.SetAccount(acc)
	})
	accounts.Refresh()

	if strings.Contains(accountInfo.GetText(false), "no account selected") {
		t.Fatal("account info should show the sole account before deletion")
	}

	must(t, reg.DeleteAccount(acc))
	accounts.Refresh()

	if !strings.Contains(accountInfo.GetText(false), "no account selected") {
		t.Error("account info should be cleared after the last account is deleted")
	}
}

// ── keyboard handlers ─────────────────────────────────────────────────────────

func TestAccountsPanel_Keys_NoHiddenAliases(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()
	capture := p.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	for _, key := range []rune{views.KeyVendors, views.KeyProjects, views.KeyUsers} {
		event := runeKey(key)
		if got := capture(event); got != event {
			t.Errorf("rune %q must pass through unchanged", key)
		}
		if app.SwitchedTo != "" {
			t.Errorf("rune %q must not switch screens, got %q", key, app.SwitchedTo)
		}
	}
}

func TestAccountsPanel_Keys_ShowModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		key          *tcell.EventKey
		wantSpecific string
		wantCommon   string
	}{
		{"new", specialKey(views.KeyInsert), views.HintsFormSpecific, views.HintsFormCommon},
		{"edit", specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsTextFormCommon},
		{"delete", specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
		{"help", runeKey(views.KeyHelp), views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon},
		{"quit", runeKey(views.KeyQuit), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
		{"quit esc", specialKey(views.KeyCancel), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			p := panels.NewAccountsPanel(app)
			p.Refresh()
			capture := p.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.Modal == nil {
				t.Errorf("key %q: expected modal to be shown", tc.name)
			}
			assertHints(t, app, tc.wantSpecific, tc.wantCommon)
		})
	}
}

func TestAccountsPanel_F2NoModal(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	capture := p.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(specialKey(tcell.KeyF2))

	if app.Modal != nil {
		t.Error("F2 should not open a modal")
	}
}

func TestAccountsPanel_EmptyList_NoModal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  *tcell.EventKey
	}{
		{"delete", specialKey(views.KeyDelete)},
		{"edit", specialKey(views.KeyEdit)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newFakeApp(t)
			reg := model.NewRegistry()
			must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
			app.Svc = newService(t, reg)
			p := panels.NewAccountsPanel(app)
			p.Refresh()
			capture := p.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.Modal != nil {
				t.Errorf("key %q with empty list should not open a modal", tc.name)
			}
		})
	}
}

// ── sortAccountsByAvailability ────────────────────────────────────────────────

// TestAccountsPanel_RowText_EscapesVendorAndUserName verifies vendor and user
// names are escaped in the rendered row.
func TestAccountsPanel_RowText_EscapesVendorAndUserName(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude]blue", false, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice[red]")))
	_, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude]blue"))
	must(t, err)

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	got, _ := p.GetItemText(0)
	if !strings.Contains(got, tview.Escape("claude]blue")) {
		t.Errorf("got %q, want it to contain the escaped vendor", got)
	}
	if !strings.Contains(got, tview.Escape("Alice[red]")) {
		t.Errorf("got %q, want it to contain the escaped user name", got)
	}
}

// TestAccountsPanel_RowText_DynamicColumnWidths verifies column widths are
// computed per batch from the longest rendered value, with the shared
// separator and padding on every cell.
func TestAccountsPanel_RowText_DynamicColumnWidths(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("ab", false, false)))
	must(t, reg.AddVendor(model.NewVendor("wxyz", false, false)))
	must(t, reg.AddUser(model.NewUser("user1@x.com", "c")))
	must(t, reg.AddUser(model.NewUser("user2@x.com", "defgh")))
	_, err := reg.AddAccount(reg.User("user1@x.com"), reg.Vendor("ab"))
	must(t, err)
	_, err = reg.AddAccount(reg.User("user2@x.com"), reg.Vendor("wxyz"))
	must(t, err)

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	// Accounts() sorts by composite key ("email|vendor"); user1 < user2.
	got0, _ := p.GetItemText(0)
	if want := " ab   │ c     "; got0 != want {
		t.Errorf("row 0 = %q, want %q", got0, want)
	}
	got1, _ := p.GetItemText(1)
	if want := " wxyz │ defgh "; got1 != want {
		t.Errorf("row 1 = %q, want %q", got1, want)
	}
}

// TestAccountsPanel_RowText_ColumnWidthCap verifies a single unusually long
// vendor name is truncated and does not widen the shared column beyond
// the cap, so a short vendor elsewhere in the same batch stays tightly
// padded instead of inheriting a huge gap.
func TestAccountsPanel_RowText_ColumnWidthCap(t *testing.T) {
	t.Parallel()
	const longVendor = "abcdefghijklmnopqrst" // 20 runes, over the 12 cap
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor(longVendor, false, false)))
	must(t, reg.AddVendor(model.NewVendor("cd", false, false)))
	must(t, reg.AddUser(model.NewUser("user1@x.com", "X")))
	must(t, reg.AddUser(model.NewUser("user2@x.com", "Y")))
	_, err := reg.AddAccount(reg.User("user1@x.com"), reg.Vendor(longVendor))
	must(t, err)
	_, err = reg.AddAccount(reg.User("user2@x.com"), reg.Vendor("cd"))
	must(t, err)

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	got0, _ := p.GetItemText(0)
	if want := " abcdefghijk… │ X "; got0 != want {
		t.Errorf("row 0 (long vendor, truncated) = %q, want %q", got0, want)
	}
	got1, _ := p.GetItemText(1)
	if want := " cd           │ Y "; got1 != want {
		t.Errorf("row 1 (short vendor, width capped, not blown out) = %q, want %q", got1, want)
	}
}

// newProjectAccountsTestApp returns a mock app with one account, ready for
// project and session setup in latest-project tests.
func newProjectAccountsTestApp(t *testing.T) (*fakeApp, *model.Registry, *model.Account) {
	t.Helper()
	reg := model.NewRegistry()
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	reg.SetClock(func() time.Time {
		now = now.Add(time.Minute)
		return now
	})
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	return &fakeApp{Svc: newService(t, reg)}, reg, acc
}

// addProjectSession grants projectName to acc when needed and adds a session
// under it. An empty projectName adds a session with no project.
func addProjectSession(t *testing.T, reg *model.Registry, acc *model.Account, projectName string) {
	t.Helper()
	var proj *model.Project
	if projectName != "" {
		proj = reg.Project(projectName)
		if proj == nil {
			proj = model.NewProject(projectName, "", "")
			must(t, reg.AddProject(proj))
			must(t, reg.AddProjectToAccount(acc, proj))
		}
	}
	_, err := reg.AddSession(acc, proj, "", []model.Prompt{model.NewPrompt("next")})
	must(t, err)
}

// TestAccountsPanel_LatestProject covers the trailing project column: it must
// mirror Account Details' green Projects entry exactly, showing only the latest
// session's project when that project is still granted.
func TestAccountsPanel_LatestProject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		setup      func(*testing.T, *model.Registry, *model.Account)
		wantSuffix string
		wantAbsent string
	}{
		{
			name: "latest granted project",
			setup: func(t *testing.T, reg *model.Registry, acc *model.Account) {
				t.Helper()
				addProjectSession(t, reg, acc, "Alpha")
				addProjectSession(t, reg, acc, "Beta")
			},
			wantSuffix: "Beta",
			wantAbsent: "Alpha",
		},
		{
			name: "older project hidden when latest session has none",
			setup: func(t *testing.T, reg *model.Registry, acc *model.Account) {
				t.Helper()
				addProjectSession(t, reg, acc, "Alpha")
				addProjectSession(t, reg, acc, "")
			},
			wantAbsent: "Alpha",
		},
		{
			name: "project name escaped",
			setup: func(t *testing.T, reg *model.Registry, acc *model.Account) {
				t.Helper()
				addProjectSession(t, reg, acc, "Be[ta]")
			},
			wantSuffix: tview.Escape("Be[ta]"),
		},
		{
			name: "long project name truncated",
			setup: func(t *testing.T, reg *model.Registry, acc *model.Account) {
				t.Helper()
				addProjectSession(t, reg, acc, "abcdefghijklmnopqrst")
			},
			wantSuffix: "abcdefghijklm…",
		},
		{
			name: "project follows pause status",
			setup: func(t *testing.T, reg *model.Registry, acc *model.Account) {
				t.Helper()
				addProjectSession(t, reg, acc, "Alpha")
				must(t, reg.PauseAccount(acc, time.Now().Add(time.Hour)))
			},
			wantSuffix: "Alpha",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app, reg, acc := newProjectAccountsTestApp(t)
			tc.setup(t, reg, acc)

			p := panels.NewAccountsPanel(app)
			p.Refresh()

			got, _ := p.GetItemText(0)
			if tc.wantSuffix != "" && !strings.HasSuffix(strings.TrimRight(got, " "), tc.wantSuffix) {
				t.Errorf("row = %q, want suffix %q", got, tc.wantSuffix)
			}
			if tc.wantAbsent != "" && strings.Contains(got, tc.wantAbsent) {
				t.Errorf("row = %q, want it not to contain %q", got, tc.wantAbsent)
			}
			if want := tview.TaggedStringWidth(got) + 2; p.RequiredWidth() < want {
				t.Errorf("RequiredWidth() = %d, want at least %d to fit row %q", p.RequiredWidth(), want, got)
			}
		})
	}
}

// TestAccountsPanel_RequiredWidth verifies the panel reports the widest row
// plus its border, the value the host App uses to size the Accounts window.
func TestAccountsPanel_RequiredWidth(t *testing.T) {
	t.Parallel()
	app, reg, acc := newProjectAccountsTestApp(t)
	addProjectSession(t, reg, acc, "Alpha")

	p := panels.NewAccountsPanel(app)
	p.Refresh()

	row, _ := p.GetItemText(0)
	want := tview.TaggedStringWidth(row) + 2
	if got := p.RequiredWidth(); got != want {
		t.Errorf("RequiredWidth() = %d, want %d", got, want)
	}
}

// TestAccountsPanel_RequiredWidth_UsesRenderedRowsOnly verifies that unused
// vendors, users, and projects do not reserve space in the Accounts panel.
func TestAccountsPanel_RequiredWidth_UsesRenderedRowsOnly(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("p", false, false)))
	must(t, reg.AddVendor(model.NewVendor("vendor-long", false, false)))
	must(t, reg.AddUser(model.NewUser("used@example.com", "U")))
	must(t, reg.AddUser(model.NewUser("unused@example.com", "LongUserNameExceedingCap")))
	_, err := reg.AddAccount(reg.User("used@example.com"), reg.Vendor("p"))
	must(t, err)
	must(t, reg.AddProject(model.NewProject("project-long-name", "", "")))

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	row, _ := p.GetItemText(0)
	if want := " p │ U "; row != want {
		t.Errorf("row = %q, want %q", row, want)
	}
	want := max(tview.TaggedStringWidth(row)+2, len(" Accounts 0/0 ")+2)
	if got := p.RequiredWidth(); got != want {
		t.Errorf("RequiredWidth() = %d, want %d", got, want)
	}
}

// TestAccountsPanel_RequiredWidth_FullPauseStatus verifies that the pause
// column is measured from the full rendered status, including a day offset,
// rather than only HH:MM.
func TestAccountsPanel_RequiredWidth_FullPauseStatus(t *testing.T) {
	t.Parallel()
	app, reg, acc := newProjectAccountsTestApp(t)
	must(t, reg.PauseAccount(acc, time.Now().AddDate(0, 0, -5)))

	p := panels.NewAccountsPanel(app)
	p.Refresh()

	row, _ := p.GetItemText(0)
	if !strings.Contains(row, " -") {
		t.Fatalf("row = %q, want full pause status with day offset", row)
	}
	minRowWidth := len("claude") + 3 + len("Alice") + 3 + len("12:34 -5")
	if got := tview.TaggedStringWidth(row); got < minRowWidth {
		t.Errorf("row width = %d, want at least %d to fit the full pause status", got, minRowWidth)
	}
	want := tview.TaggedStringWidth(row) + 2
	if got := p.RequiredWidth(); got != want {
		t.Errorf("RequiredWidth() = %d, want %d", got, want)
	}
}

// TestAccountsPanel_RequiredWidth_UpdatesWithContent verifies that Refresh
// both grows and shrinks the reported width as store content changes.
func TestAccountsPanel_RequiredWidth_UpdatesWithContent(t *testing.T) {
	t.Parallel()
	app, reg, acc := newProjectAccountsTestApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()
	initial := p.RequiredWidth()

	longProject := model.NewProject("abcdefghijklmnopqrst", "", "")
	must(t, reg.AddProject(longProject))
	must(t, reg.AddProjectToAccount(acc, longProject))
	addProjectSession(t, reg, acc, longProject.Name())
	p.Refresh()
	if got := p.RequiredWidth(); got <= initial {
		t.Fatalf("RequiredWidth() did not grow: initial %d, after project %d", initial, got)
	}

	acc = reg.Account(reg.User(acc.UserEmail()), reg.Vendor(acc.VendorName()))
	sessions, err := reg.AccountSessions(acc)
	must(t, err)
	must(t, reg.DeleteSession(sessions[len(sessions)-1]))
	must(t, reg.RemoveProjectFromAccount(acc, longProject))
	must(t, reg.DeleteProject(longProject))
	p.Refresh()
	if got := p.RequiredWidth(); got != initial {
		t.Errorf("RequiredWidth() after project removal = %d, want initial %d", got, initial)
	}
}

// TestSortAccountsByAvailability verifies account ordering through Refresh.
func TestSortAccountsByAvailability(t *testing.T) {
	t.Parallel()
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	nearFuture := now.Add(1 * time.Hour)
	farFuture := now.Add(3 * time.Hour)

	cases := []struct {
		name          string
		secondPauseAt time.Time // zero means never paused
	}{
		{"unpaused before future", time.Time{}},
		{"past before future", past},
		{"sooner future before later future", nearFuture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := model.NewRegistry()
			must(t, reg.AddVendor(model.NewVendor("p", false, false)))
			must(t, reg.AddUser(model.NewUser("first@x.com", "First")))
			must(t, reg.AddUser(model.NewUser("second@x.com", "Second")))
			firstAcc, err := reg.AddAccount(reg.User("first@x.com"), reg.Vendor("p"))
			must(t, err)
			_, err = reg.AddAccount(reg.User("second@x.com"), reg.Vendor("p"))
			must(t, err)
			must(t, reg.PauseAccount(firstAcc, farFuture))
			if !tc.secondPauseAt.IsZero() {
				secondAcc := reg.Account(reg.User("second@x.com"), reg.Vendor("p"))
				must(t, reg.PauseAccount(secondAcc, tc.secondPauseAt))
			}

			app := &fakeApp{Svc: newService(t, reg)}
			p := panels.NewAccountsPanel(app)
			p.Refresh()

			got, _ := p.GetItemText(0)
			if !strings.Contains(got, "Second") {
				t.Errorf("first item = %q, want it to contain %q (more available than First, paused to farFuture)", got, "Second")
			}
		})
	}
}

// TestAccountsPanel_SetCurrentItem_TriggersOnSelect covers the
// SetChangedFunc wiring specifically, distinct from
// TestAccountsPanel_Refresh_CallsOnSelect: Refresh calls onSelect
// directly and guards SetCurrentItem with p.refreshing while it runs, so
// it never exercises the changed-callback path. Navigating the list
// outside a refresh (arrow keys, in real use) does, and that path has its
// own onSelect != nil / index-bounds guard worth verifying independently.
func TestAccountsPanel_SetCurrentItem_TriggersOnSelect(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh() // populates the list; selection starts at index 0

	var selected *model.Account
	p.SetOnSelect(func(a *model.Account) { selected = a })

	p.SetCurrentItem(1)

	if selected == nil {
		t.Fatal("expected onSelect to fire when the selection changes outside Refresh")
	}
	if want := p.Selected(); selected.Key() != want.Key() {
		t.Errorf("onSelect received %+v, want %+v", selected.Key(), want.Key())
	}
}

// TestAccountsPanel_NonRuneKey_PassesThrough covers DispatchPanelKey's
// non-rune early return (input.go), shared by every panel that calls it.
// Exercised here via AccountsPanel as the vehicle, same principle as
// addCtrlS being tested through one form for all eight callers.
func TestAccountsPanel_NonRuneKey_PassesThrough(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	event := tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	if got := p.GetInputCapture()(event); got != event {
		t.Error("non-rune key must pass through unchanged")
	}
}

// TestAccountsPanel_New_SaveReturnsToPanel closes the loop that
// TestAccountsPanel_Keys_ShowModal leaves open: that test only checks
// that pressing Ins opens a modal with the right hints, not that
// completing the form hands control back to the panel correctly. The
// onClose closure passed to NewAccountForm (SetHints + HideModal) is
// otherwise never actually invoked by any existing test.
func TestAccountsPanel_New_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	capture := p.GetInputCapture()
	capture(specialKey(views.KeyInsert))

	form := formFromCenteredModal(t, app.Modal)
	setDropDown(t, form, "User", 0)   // alice
	setDropDown(t, form, "Vendor", 0) // chatgpt
	triggerSave(t, form)

	if app.Modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
	if _, err := app.Svc.Account("alice@example.com", "chatgpt"); err != nil {
		t.Errorf("account not created: %v", err)
	}
}

// TestAccountsPanel_Edit_SaveReturnsToPanel is the same round trip as
// TestAccountsPanel_New_SaveReturnsToPanel for handleEdit's own onClose
// closure.
func TestAccountsPanel_Edit_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh() // selects alice|claude first (sorted by key, both unpaused)

	capture := p.GetInputCapture()
	capture(specialKey(views.KeyEdit))

	form := formFromCenteredModal(t, app.Modal)
	setInputField(t, form, "Pause until", "2026-12-31 23:59")
	triggerSave(t, form)

	if app.Modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
}

// TestAccountsPanel_Delete_Cancel is the accounts.go counterpart to
// TestDetailPanel_DeleteSession_Cancel: confirming the delete is
// button-click-gated and unreachable black box (see that test's comment
// for why), so this covers the reachable half, cancelling leaves the
// account untouched and hands control back to the panel.
func TestAccountsPanel_Delete_Cancel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	before := len(app.Svc.Accounts())

	capture := p.GetInputCapture()
	capture(specialKey(views.KeyDelete))

	confirmModal := confirmModalFrom(t, app.Modal)
	confirmModal.GetInputCapture()(escKey())

	if app.Modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	assertHints(t, app, views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
	if got := len(app.Svc.Accounts()); got != before {
		t.Errorf("account count changed after cancel: got %d, want %d (unchanged)", got, before)
	}
}

func TestAccountsPanel_FilterKey(t *testing.T) {
	t.Parallel()
	app, _, acc := newProjectAccountsTestApp(t)
	must(t, app.Svc.AddProject("Alpha", "", ""))
	filter := views.NewProjectFilterState()
	filter.SetAccount(acc.Key(), true, []*model.Project{app.Svc.Project("Alpha")})
	p := panels.NewAccountsPanel(app)
	p.SetFilter(filter)
	called := false
	p.SetOnFilterChange(func() { called = true })
	p.Refresh()

	p.GetInputCapture()(runeKey(views.KeyFilter))
	if !called {
		t.Error("filter callback was not called")
	}
	if got := filter.TitleSuffix(); got != " (Alpha)" {
		t.Errorf("TitleSuffix() = %q, want (Alpha)", got)
	}
}

// TestFormatPauseUntil verifies the deterministic pause-time formatting.
func TestFormatPauseUntil(t *testing.T) {
	t.Parallel()
	// A fixed reference instant keeps the test independent of wall-clock time.
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sameDay := time.Date(2026, 8, 27, 14, 30, 0, 0, time.UTC)
	nextDay := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	prevDay := time.Date(2026, 8, 26, 20, 0, 0, 0, time.UTC)
	fiveDaysPast := time.Date(2026, 8, 22, 12, 34, 0, 0, time.UTC)
	twoDaysFuture := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	crossMonth := time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name       string
		t          time.Time
		wantText   string
		wantIsPast bool
	}{
		{"same day format", sameDay, "14:30", sameDay.Before(now)},
		{"next day format", nextDay, "09:00 +1", false},
		{"previous day format", prevDay, "20:00 -1", true},
		{"five days past format", fiveDaysPast, "12:34 -5", true},
		{"two days future format", twoDaysFuture, "09:00 +2", false},
		{"cross-month format", crossMonth, "09:00 +6", false},
		{"past is past", past, past.Format("15:04"), true},
		{"future not past", future, future.Format("15:04"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotText, gotIsPast := panels.FormatPauseUntil(tc.t, now)
			if gotText != tc.wantText {
				t.Errorf("text: got %q, want %q", gotText, tc.wantText)
			}
			if gotIsPast != tc.wantIsPast {
				t.Errorf("isPast: got %v, want %v", gotIsPast, tc.wantIsPast)
			}
		})
	}
}

func TestAccountsPanel_DoubleClickOpensPauseForm(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	doubleClickList(t, p.List)

	form := formFromCenteredModal(t, app.Modal)
	if got := form.GetTitle(); got != " Pause Account " {
		t.Errorf("title = %q, want Pause Account", got)
	}
}

func TestListTitles_AccountsPanel(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	p := panels.NewAccountsPanel(app)
	p.Refresh()

	if got, want := p.GetTitle(), " Accounts 1/2 "; got != want {
		t.Errorf("initial title = %q, want %q", got, want)
	}
	p.SetCurrentItem(1)
	if got, want := p.GetTitle(), " Accounts 2/2 "; got != want {
		t.Errorf("selected title = %q, want %q", got, want)
	}

	app.Svc = newService(t, model.NewRegistry())
	p.Refresh()
	if got, want := p.GetTitle(), " Accounts 0/0 "; got != want {
		t.Errorf("empty title = %q, want %q", got, want)
	}
}

// ── AccountInfoPanel ──────────────────────────────────────────────────────────

func TestNewAccountInfoPanel(t *testing.T) {
	t.Parallel()
	p := panels.NewAccountInfoPanel(newFakeApp(t))
	if p == nil {
		t.Fatal("expected non-nil AccountInfoPanel")
	}
	if got := p.GetTitle(); got != " Account Details " {
		t.Errorf("GetTitle() = %q, want %q", got, " Account Details ")
	}
}

func TestAccountInfoPanel_SetAccount_Nil(t *testing.T) {
	t.Parallel()
	p := panels.NewAccountInfoPanel(newFakeApp(t))
	p.SetAccount(nil)

	if got := p.GetText(false); !strings.Contains(got, "no account selected") {
		t.Errorf("got %q, want it to contain %q", got, "no account selected")
	}
	if got := p.RequiredHeight(40); got != 3 {
		t.Errorf("RequiredHeight(40) = %d, want 3", got)
	}
}

func TestAccountInfoPanel_RendersDetailsWithoutPaused(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc, err := app.Svc.Account("alice@example.com", "claude")
	must(t, err)

	p := panels.NewAccountInfoPanel(app)
	p.SetAccount(acc)

	got := p.GetText(false)
	for _, want := range []string{"User", "Alice", "alice@example.com", "Vendor", "claude", "Projects", "Alpha", "Beta"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "Paused") {
		t.Errorf("got %q, want no Paused line", got)
	}
	if gotHeight := p.RequiredHeight(80); gotHeight != 5 {
		t.Errorf("RequiredHeight(80) = %d, want 5 for three content lines", gotHeight)
	}
}

func TestAccountInfoPanel_AlignedDetailRows(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc, err := app.Svc.Account("alice@example.com", "claude")
	must(t, err)

	p := panels.NewAccountInfoPanel(app)
	p.SetAccount(acc)

	lines := strings.Split(p.GetText(true), "\n")
	wantValues := []string{"Alice", "claude", "Alpha"}
	if len(lines) != len(wantValues) {
		t.Fatalf("got %d detail lines, want %d: %q", len(lines), len(wantValues), lines)
	}
	for i, line := range lines {
		if got := strings.Index(line, wantValues[i]); got != 9 {
			t.Errorf("line %d value starts at %d, want 9: %q", i, got, line)
		}
	}
}

func TestAccountInfoPanel_WrappedLinesKeepInsets(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	project := model.NewProject(strings.Repeat("long-project-name-", 8), "", "")
	must(t, reg.AddProject(project))
	must(t, reg.AddProjectToAccount(acc, project))

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewAccountInfoPanel(app)
	p.SetAccount(reg.Account(reg.User("alice@example.com"), reg.Vendor("claude")))

	screen := tcell.NewSimulationScreen("UTF-8")
	must(t, screen.Init())
	t.Cleanup(screen.Fini)
	screen.SetSize(40, 10)
	p.SetRect(0, 0, 40, 10)
	p.Draw(screen)

	x, y, width, height := p.GetInnerRect()
	nonBlank := 0
	for i := range height {
		mainc, _, _ := screen.Get(x, y+i)
		if strings.TrimSpace(mainc) == "" {
			continue
		}
		nonBlank++
		if pad, _, _ := screen.Get(x-1, y+i); pad != " " {
			t.Errorf("line %d starts with %q at column %d, want left padding", i, pad, x-1)
		}
		if pad, _, _ := screen.Get(x+width, y+i); pad != " " {
			t.Errorf("line %d reaches %q at column %d, want right padding", i, pad, x+width)
		}
	}
	if nonBlank < 4 {
		t.Fatalf("got %d non-blank lines, want at least 4 to exercise wrapping", nonBlank)
	}
}

func TestAccountInfoPanel_NoProjects(t *testing.T) {
	t.Parallel()
	app := newFakeApp(t)
	acc, err := app.Svc.Account("bob@example.com", "chatgpt")
	must(t, err)

	p := panels.NewAccountInfoPanel(app)
	p.SetAccount(acc)

	if got := p.GetText(false); !strings.Contains(got, "none") {
		t.Errorf("got %q, want it to contain %q", got, "none")
	}
}

func TestAccountInfoPanel_RequiredHeight_Wraps(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	project := model.NewProject(strings.Repeat("long-project-name-", 8), "", "")
	must(t, reg.AddProject(project))
	must(t, reg.AddProjectToAccount(acc, project))

	app := &fakeApp{Svc: newService(t, reg)}
	p := panels.NewAccountInfoPanel(app)
	p.SetAccount(reg.Account(reg.User("alice@example.com"), reg.Vendor("claude")))

	narrow := p.RequiredHeight(20)
	wide := p.RequiredHeight(200)
	if narrow <= wide {
		t.Errorf("RequiredHeight(20) = %d, want more than RequiredHeight(200) = %d", narrow, wide)
	}
	if wide != 5 {
		t.Errorf("RequiredHeight(200) = %d, want 5", wide)
	}
}

func TestAccountInfoPanel_HighlightsLatestProject(t *testing.T) {
	t.Parallel()
	app, reg, acc := newProjectAccountsTestApp(t)
	addProjectSession(t, reg, acc, "Alpha")
	addProjectSession(t, reg, acc, "Beta")

	p := panels.NewAccountInfoPanel(app)
	p.SetAccount(reg.Account(reg.User("alice@example.com"), reg.Vendor("claude")))

	got := p.GetText(false)
	if !strings.Contains(got, "[green]Beta[-]") {
		t.Errorf("got %q, want Beta highlighted", got)
	}
	if strings.Contains(got, "[green]Alpha[-]") {
		t.Errorf("got %q, want Alpha not highlighted", got)
	}
}

func TestAccountInfoPanel_NoHighlightWhenLatestHasNoProject(t *testing.T) {
	t.Parallel()
	app, reg, acc := newProjectAccountsTestApp(t)
	addProjectSession(t, reg, acc, "Alpha")
	addProjectSession(t, reg, acc, "")

	p := panels.NewAccountInfoPanel(app)
	p.SetAccount(reg.Account(reg.User("alice@example.com"), reg.Vendor("claude")))

	got := p.GetText(false)
	if strings.Contains(got, "[green]") {
		t.Errorf("got %q, want no highlighted project", got)
	}
}
