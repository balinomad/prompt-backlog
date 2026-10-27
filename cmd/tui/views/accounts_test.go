package views_test

import (
	"strings"
	"testing"
	"time"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/internal/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── construction ──────────────────────────────────────────────────────────────

func TestNewAccountsPanel(t *testing.T) {
	t.Parallel()
	if p := views.NewAccountsPanel(newMockApp(t)); p == nil {
		t.Fatal("expected non-nil AccountsPanel")
	}
}

func TestAccountsPanel_SelectedBeforeRefresh(t *testing.T) {
	t.Parallel()
	p := views.NewAccountsPanel(newMockApp(t))
	if a := p.Selected(); a != nil {
		t.Errorf("expected nil Selected before Refresh, got %v", a)
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestAccountsPanel_Refresh(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		setup     func(*testing.T, *mockApp)
		wantCount int
	}{
		{
			name:      "two accounts",
			setup:     func(_ *testing.T, _ *mockApp) {},
			wantCount: 2,
		},
		{
			name: "empty registry",
			setup: func(t *testing.T, app *mockApp) {
				t.Helper()
				reg := model.NewRegistry()
				must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
				app.svc = newTestService(t, reg)
			},
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			tc.setup(t, app)
			p := views.NewAccountsPanel(app)
			p.Refresh()
			if got := p.GetItemCount(); got != tc.wantCount {
				t.Errorf("item count: got %d, want %d", got, tc.wantCount)
			}
		})
	}
}

func TestAccountsPanel_Refresh_SelectedNotNil(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()
	if p.Selected() == nil {
		t.Error("expected non-nil Selected after Refresh")
	}
}

func TestAccountsPanel_Refresh_PreservesSelection(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
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
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	var called bool
	p.SetOnSelect(func(_ *model.Account) { called = true })
	p.Refresh()
	if !called {
		t.Error("onSelect should be called after Refresh")
	}
}

// TestAccountsPanel_Refresh_ClearsDetailPanelsOnLastAccountDeleted wires
// the accounts selection to both account views, then deletes the only
// account. The sessions list and the account info panel must both clear.
func TestAccountsPanel_Refresh_ClearsDetailPanelsOnLastAccountDeleted(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)

	app := &mockApp{svc: newTestService(t, reg)}
	accounts := views.NewAccountsPanel(app)
	detail := views.NewDetailPanel(app)
	accountInfo := views.NewAccountInfoPanel(app)
	accounts.SetOnSelect(func(acc *model.Account) {
		detail.SetAccount(acc)
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
	if detail.GetItemCount() != 0 {
		t.Error("detail panel's session list should be empty after the last account is deleted")
	}
}

// ── keyboard handlers ─────────────────────────────────────────────────────────

func TestAccountsPanel_Keys_NoHiddenAliases(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
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
		if app.switchedTo != "" {
			t.Errorf("rune %q must not switch screens, got %q", key, app.switchedTo)
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
		{"edit", specialKey(views.KeyEdit), views.HintsFormSpecific, views.HintsFormCommon},
		{"delete", specialKey(views.KeyDelete), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
		{"help", runeKey(views.KeyHelp), views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon},
		{"quit", runeKey(views.KeyQuit), views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newMockApp(t)
			p := views.NewAccountsPanel(app)
			p.Refresh()
			capture := p.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.modal == nil {
				t.Errorf("key %q: expected modal to be shown", tc.name)
			}
			assertHints(t, app, tc.wantSpecific, tc.wantCommon)
		})
	}
}

func TestAccountsPanel_F2NoModal(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()

	capture := p.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(specialKey(tcell.KeyF2))

	if app.modal != nil {
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
			app := newMockApp(t)
			reg := model.NewRegistry()
			must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
			app.svc = newTestService(t, reg)
			p := views.NewAccountsPanel(app)
			p.Refresh()
			capture := p.GetInputCapture()
			if capture == nil {
				t.Fatal("expected non-nil input capture")
			}
			capture(tc.key)
			if app.modal != nil {
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

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewAccountsPanel(app)
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

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewAccountsPanel(app)
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

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewAccountsPanel(app)
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
func newProjectAccountsTestApp(t *testing.T) (*mockApp, *model.Registry, *model.Account) {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	return &mockApp{svc: newTestService(t, reg)}, reg, acc
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
	_, err := reg.AddSession(acc, proj, "", nil)
	must(t, err)
}

// TestAccountsPanel_LatestProject covers the trailing project column: it must
// mirror DetailPanel's green Projects entry exactly, showing only the latest
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

			p := views.NewAccountsPanel(app)
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

	p := views.NewAccountsPanel(app)
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

	app := &mockApp{svc: newTestService(t, reg)}
	p := views.NewAccountsPanel(app)
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

	p := views.NewAccountsPanel(app)
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
	p := views.NewAccountsPanel(app)
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

			app := &mockApp{svc: newTestService(t, reg)}
			p := views.NewAccountsPanel(app)
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
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
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

// TestAccountsPanel_NonRuneKey_PassesThrough covers dispatchPanelKey's
// non-rune early return (keys.go), shared by every panel that calls it.
// Exercised here via AccountsPanel as the vehicle, same principle as
// addCtrlS being tested through one form for all eight callers.
func TestAccountsPanel_NonRuneKey_PassesThrough(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
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
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()

	capture := p.GetInputCapture()
	capture(specialKey(views.KeyInsert))

	form := formFromCenteredModal(t, app.modal)
	setDropDown(t, form, "User", 1)   // alice
	setDropDown(t, form, "Vendor", 0) // chatgpt
	triggerSave(t, form)

	if app.modal != nil {
		t.Error("expected modal to be hidden after a successful save")
	}
	assertHints(t, app, views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
	if _, err := app.svc.Account("alice@example.com", "chatgpt"); err != nil {
		t.Errorf("account not created: %v", err)
	}
}

// TestAccountsPanel_Edit_SaveReturnsToPanel is the same round trip as
// TestAccountsPanel_New_SaveReturnsToPanel for handleEdit's own onClose
// closure.
func TestAccountsPanel_Edit_SaveReturnsToPanel(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh() // selects alice|claude first (sorted by key, both unpaused)

	capture := p.GetInputCapture()
	capture(specialKey(views.KeyEdit))

	form := formFromCenteredModal(t, app.modal)
	setInputField(t, form, "Pause until", "2026-12-31 23:59")
	triggerSave(t, form)

	if app.modal != nil {
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
	app := newMockApp(t)
	p := views.NewAccountsPanel(app)
	p.Refresh()

	before := len(app.svc.Accounts())

	capture := p.GetInputCapture()
	capture(specialKey(views.KeyDelete))

	confirmModal := confirmModalFrom(t, app.modal)
	confirmModal.GetInputCapture()(escKey())

	if app.modal != nil {
		t.Error("expected modal to be hidden after cancelling")
	}
	assertHints(t, app, views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
	if got := len(app.svc.Accounts()); got != before {
		t.Errorf("account count changed after cancel: got %d, want %d (unchanged)", got, before)
	}
}
