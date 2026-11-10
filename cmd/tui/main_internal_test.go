package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/balinomad/prompt-backlog/cmd/tui/views"
	"github.com/balinomad/prompt-backlog/internal/apperr"
	"github.com/balinomad/prompt-backlog/internal/model"
	"github.com/balinomad/prompt-backlog/internal/service"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/terminfo"
	"github.com/rivo/tview"
)

func TestWin32InputModeDisable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		goos string
		want string
	}{
		{"linux", "linux", "\x1b[?9001l"},
		{"darwin", "darwin", "\x1b[?9001l"},
		{"windows keeps protocol", "windows", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := win32InputModeDisable(tc.goos); got != tc.want {
				t.Errorf("win32InputModeDisable(%q) = %q, want %q", tc.goos, got, tc.want)
			}
		})
	}
}

func TestNumericKeypadTerminfo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		enterKeypad string
		want        string
	}{
		{"removes deckpam", "\x1b[?1h\x1b=", "\x1b[?1h"},
		{"keeps sequence without deckpam", "\x1b[?1h", "\x1b[?1h"},
		{"empty stays empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ti := &terminfo.Terminfo{
				EnterKeypad: tc.enterKeypad,
				ExitKeypad:  "\x1b[?1l\x1b>",
			}
			got := numericKeypadTerminfo(ti)
			if got == ti {
				t.Fatal("numericKeypadTerminfo returned the original pointer")
			}
			if got.EnterKeypad != tc.want {
				t.Errorf("EnterKeypad = %q, want %q", got.EnterKeypad, tc.want)
			}
			if got.ExitKeypad != ti.ExitKeypad {
				t.Errorf("ExitKeypad = %q, want %q", got.ExitKeypad, ti.ExitKeypad)
			}
		})
	}

	t.Run("nil terminfo", func(t *testing.T) {
		t.Parallel()
		if got := numericKeypadTerminfo(nil); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

// layoutStore is an in-memory store for layout tests.
type layoutStore struct{ reg *model.Registry }

func (s layoutStore) Load() (*model.Registry, error) { return s.reg, nil }
func (s layoutStore) Save(*model.Registry) error     { return nil }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// newLayoutApp returns an App backed by an in-memory registry with one vendor
// so the first-run vendor redirect does not fire.
func newLayoutApp(t *testing.T) *App {
	t.Helper()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	svc, err := service.New(layoutStore{reg: reg})
	must(t, err)
	app, err := New(svc)
	must(t, err)
	return app
}

func TestApp_FatalStoresFirstError(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	first := apperr.Invariantf("first")
	second := apperr.Invariantf("second")

	app.Fatal(first)
	app.Fatal(second)

	if got := app.fatalError(); !errors.Is(got, first) {
		t.Errorf("fatalError() = %v, want the first error", got)
	}
}

func TestApp_HandleGlobalKeys_SwitchScreens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		startPage string
		key       rune
		wantPage  string
	}{
		{
			name:      "main to vendors",
			startPage: "main",
			key:       views.KeyVendors,
			wantPage:  "vendors",
		},
		{
			name:      "main to projects",
			startPage: "main",
			key:       views.KeyProjects,
			wantPage:  "projects",
		},
		{
			name:      "main to users",
			startPage: "main",
			key:       views.KeyUsers,
			wantPage:  "users",
		},
		{
			name:      "vendors to projects",
			startPage: "vendors",
			key:       views.KeyProjects,
			wantPage:  "projects",
		},
		{
			name:      "projects to users",
			startPage: "projects",
			key:       views.KeyUsers,
			wantPage:  "users",
		},
		{
			name:      "users to vendors",
			startPage: "users",
			key:       views.KeyVendors,
			wantPage:  "vendors",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newLayoutApp(t)
			app.pages.SwitchToPage(tc.startPage)
			event := tcell.NewEventKey(tcell.KeyRune, tc.key, tcell.ModNone)
			if got := app.handleGlobalKeys(event); got != nil {
				t.Errorf("handleGlobalKeys() = %v, want nil", got)
			}
			got, _ := app.pages.GetFrontPage()
			if got != tc.wantPage {
				t.Errorf("front page = %q, want %q", got, tc.wantPage)
			}
		})
	}
}

func TestApp_HandleGlobalKeys_IgnoresModal(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	if err := app.ShowModal(tview.NewBox()); err != nil {
		t.Fatalf("ShowModal: %v", err)
	}
	t.Cleanup(app.HideModal)

	event := tcell.NewEventKey(tcell.KeyRune, views.KeyUsers, tcell.ModNone)
	if got := app.handleGlobalKeys(event); got != event {
		t.Errorf("handleGlobalKeys() = %v, want original event", got)
	}
	if len(app.modals) != 1 {
		t.Errorf("modal stack length = %d, want 1", len(app.modals))
	}
}

func TestApp_HandleGlobalKeys_TooSmall(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	app.tooSmall = true

	event := tcell.NewEventKey(tcell.KeyRune, views.KeyUsers, tcell.ModNone)
	if got := app.handleGlobalKeys(event); got != nil {
		t.Errorf("handleGlobalKeys() = %v for a too-small terminal, want nil", got)
	}
	if got, _ := app.pages.GetFrontPage(); got != "main" {
		t.Errorf("front page = %q, want main", got)
	}

	ctrlC := tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone)
	if got := app.handleGlobalKeys(ctrlC); got != ctrlC {
		t.Errorf("handleGlobalKeys(Ctrl-C) = %v, want the original event", got)
	}
}

func TestApp_HandleGlobalMouse_TooSmall(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	event := tcell.NewEventMouse(1, 1, tcell.ButtonPrimary, tcell.ModNone)

	app.tooSmall = true
	if gotEvent, gotAction := app.handleGlobalMouse(event, tview.MouseLeftClick); gotEvent != nil || gotAction != tview.MouseLeftClick {
		t.Errorf("handleGlobalMouse() = (%v, %v), want (nil, %v)", gotEvent, gotAction, tview.MouseLeftClick)
	}

	app.tooSmall = false
	if gotEvent, gotAction := app.handleGlobalMouse(event, tview.MouseLeftClick); gotEvent != event || gotAction != tview.MouseLeftClick {
		t.Errorf("handleGlobalMouse() = (%v, %v), want (%v, %v)", gotEvent, gotAction, event, tview.MouseLeftClick)
	}
}

func TestModalMouseGuard_ConsumesUnhandled(t *testing.T) {
	t.Parallel()
	inner := tview.NewBox()
	inner.SetRect(0, 0, 10, 10)
	guard := &modalMouseGuard{Primitive: inner}
	event := tcell.NewEventMouse(2, 2, tcell.ButtonPrimary, tcell.ModNone)

	consumed, capture := guard.MouseHandler()(tview.MouseLeftDoubleClick, event, func(tview.Primitive) {})
	if !consumed || capture != nil {
		t.Errorf("mouse handler = (%v, %v), want (true, nil)", consumed, capture)
	}
}

func TestModalMouseGuard_ForwardsConsumed(t *testing.T) {
	t.Parallel()
	list := tview.NewList()
	list.SetRect(0, 0, 10, 10)
	called := false
	views.WireListDoubleClick(list, func() { called = true })
	guard := &modalMouseGuard{Primitive: list}
	event := tcell.NewEventMouse(2, 2, tcell.ButtonPrimary, tcell.ModNone)

	consumed, capture := guard.MouseHandler()(tview.MouseLeftDoubleClick, event, func(tview.Primitive) {})
	if !consumed || capture != nil {
		t.Errorf("mouse handler = (%v, %v), want (true, nil)", consumed, capture)
	}
	if !called {
		t.Error("modal did not receive the consumed double-click")
	}
}

func TestApp_ShowModalWrapsMouseGuard(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	if err := app.ShowModal(tview.NewBox()); err != nil {
		t.Fatalf("ShowModal: %v", err)
	}
	t.Cleanup(app.HideModal)

	_, page := app.pages.GetFrontPage()
	if _, ok := page.(*modalMouseGuard); !ok {
		t.Errorf("modal page type = %T, want *modalMouseGuard", page)
	}
}

func newBorderedModal() (*tview.Flex, *tview.InputField) {
	box := tview.NewFlex().SetDirection(tview.FlexRow)
	field := tview.NewInputField()
	box.AddItem(field, 1, 0, true)
	box.SetBorder(true)
	return box, field
}

func TestApp_ModalStack_RestoresFocusHintsAndBorder(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	rootFocus := app.tv.GetFocus()
	app.SetHints("root specific", "root common")

	parent, parentField := newBorderedModal()
	if err := app.ShowModal(views.NewModalState(parent, parent)); err != nil {
		t.Fatalf("ShowModal parent: %v", err)
	}
	app.SetHints("parent specific", "parent common")
	app.SetFocus(parentField)

	child, _ := newBorderedModal()
	if err := app.ShowModal(views.NewModalState(child, child)); err != nil {
		t.Fatalf("ShowModal child: %v", err)
	}
	app.SetHints("child specific", "child common")

	if got := len(app.modals); got != 2 {
		t.Fatalf("modal stack length = %d, want 2", got)
	}
	if got := parent.GetBorderColor(); got != views.ColorInactive {
		t.Errorf("inactive parent border = %v, want gray", got)
	}
	if app.hintsSpec != "child specific" {
		t.Errorf("hints = %q, want child specific", app.hintsSpec)
	}

	app.HideModal()
	if got := len(app.modals); got != 1 {
		t.Fatalf("modal stack length after pop = %d, want 1", got)
	}
	if got := app.tv.GetFocus(); got != parentField {
		t.Errorf("focus after pop = %T, want parent field", got)
	}
	if app.hintsSpec != "parent specific" {
		t.Errorf("hints after pop = %q, want parent specific", app.hintsSpec)
	}
	if got := parent.GetBorderColor(); got != views.ColorAccent {
		t.Errorf("restored parent border = %v, want accent", got)
	}

	app.HideModal()
	if got := len(app.modals); got != 0 {
		t.Fatalf("modal stack length after final pop = %d, want 0", got)
	}
	if got := app.tv.GetFocus(); got != rootFocus {
		t.Errorf("focus after final pop = %T, want root focus", got)
	}
	if app.hintsSpec != "root specific" {
		t.Errorf("hints after final pop = %q, want root specific", app.hintsSpec)
	}
}

func TestApp_ModalStack_ReplaceTop(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	app.SetHints("root specific", "root common")

	parent, _ := newBorderedModal()
	if err := app.ShowModal(views.NewModalState(parent, parent)); err != nil {
		t.Fatalf("ShowModal: %v", err)
	}
	app.SetHints("parent specific", "parent common")

	replacement, replacementField := newBorderedModal()
	if err := app.ReplaceTopModal(views.NewModalState(replacement, replacement)); err != nil {
		t.Fatalf("ReplaceTopModal: %v", err)
	}
	if got := len(app.modals); got != 1 {
		t.Errorf("modal stack length = %d, want 1", got)
	}
	if got := app.tv.GetFocus(); got != replacementField {
		t.Errorf("focus after replace = %T, want replacement field", got)
	}
	if app.hintsSpec != "parent specific" {
		t.Errorf("hints after replace = %q, want parent specific", app.hintsSpec)
	}
}

func TestApp_ReplaceTopModal_NoModal(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	if err := app.ReplaceTopModal(tview.NewBox()); err == nil {
		t.Error("ReplaceTopModal with no modal returned nil error")
	}
}

func TestTerminalTooSmall(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		width  int
		height int
		want   bool
	}{
		{
			name:   "minimum size",
			width:  minTerminalWidth,
			height: minTerminalHeight,
			want:   false,
		},
		{
			name:   "wider",
			width:  minTerminalWidth + 1,
			height: minTerminalHeight,
			want:   false,
		},
		{
			name:   "taller",
			width:  minTerminalWidth,
			height: minTerminalHeight + 1,
			want:   false,
		},
		{
			name:   "too narrow",
			width:  minTerminalWidth - 1,
			height: minTerminalHeight,
			want:   true,
		},
		{
			name:   "too short",
			width:  minTerminalWidth,
			height: minTerminalHeight - 1,
			want:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := terminalTooSmall(tc.width, tc.height); got != tc.want {
				t.Errorf("terminalTooSmall(%d, %d) = %v, want %v", tc.width, tc.height, got, tc.want)
			}
		})
	}
}

func TestDrawTerminalTooSmall(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	const width, height = 40, 12
	screen.SetSize(width, height)
	drawTerminalTooSmall(screen, width, height)
	screen.Show()

	cells, gotWidth, gotHeight := screen.GetContents()
	if gotWidth != width || gotHeight != height {
		t.Fatalf("screen size = %dx%d, want %dx%d", gotWidth, gotHeight, width, height)
	}
	var text strings.Builder
	for _, cell := range cells {
		if len(cell.Bytes) == 0 {
			text.WriteByte(' ')
			continue
		}
		text.Write(cell.Bytes)
	}
	for _, want := range []string{
		"Terminal too small",
		fmt.Sprintf("Minimum: %dx%d", minTerminalWidth, minTerminalHeight),
		"Current: 40x12",
		"Resize to continue.",
	} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("screen text does not contain %q: %q", want, text.String())
		}
	}
	if _, _, visible := screen.GetCursor(); visible {
		t.Error("cursor should be hidden")
	}
}

func TestApp_BeforeDraw_TerminalSize(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	screen.SetSize(minTerminalWidth-1, minTerminalHeight)
	if !app.beforeDraw(screen) {
		t.Error("beforeDraw() = false for a too-small terminal, want true")
	}
	if !app.tooSmall {
		t.Error("tooSmall = false after a too-small draw, want true")
	}

	screen.SetSize(minTerminalWidth, minTerminalHeight)
	if app.beforeDraw(screen) {
		t.Error("beforeDraw() = true for a supported terminal, want false")
	}
	if app.tooSmall {
		t.Error("tooSmall = true after a supported draw, want false")
	}
}

func TestRun(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
	}{
		{
			name:       "missing data flag",
			args:       nil,
			wantCode:   exitUsage,
			wantStderr: "-data flag is required",
		},
		{
			name:       "unsupported format",
			args:       []string{"-data", "registry.txt"},
			wantCode:   exitFailure,
			wantStderr: "unsupported data file format",
		},
		{
			name:       "unknown flag",
			args:       []string{"-unknown"},
			wantCode:   exitUsage,
			wantStderr: "flag provided but not defined",
		},
		{
			name:       "help",
			args:       []string{"-h"},
			wantCode:   exitSuccess,
			wantStderr: "Usage of prompt-backlog",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			if got := run(tc.args, &stderr); got != tc.wantCode {
				t.Fatalf("run() = %d, want %d; stderr: %s", got, tc.wantCode, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}

func TestOpenStore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"json extension", "registry.json", false},
		{"unsupported extension", "registry.txt", true},
		{"no extension", "registry", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, err := openStore(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s == nil {
				t.Fatal("expected non-nil Store")
			}
		})
	}
}

// TestNew_SizesAccountsPanelToWidestRow verifies the main layout allocates and
// updates the accounts panel width to fit the widest row, including its border
// and the full pause status.
func TestNew_SizesAccountsPanelToWidestRow(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	proj := model.NewProject("abcdefghijklmnopqrst", "", "")
	must(t, reg.AddProject(proj))
	must(t, reg.AddProjectToAccount(acc, proj))
	_, err = reg.AddSession(acc, proj, "", nil)
	must(t, err)

	svc, err := service.New(layoutStore{reg: reg})
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	app, err := New(svc)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(200, 20)

	app.mainFlex.SetRect(0, 0, 200, 20)
	app.mainFlex.Draw(screen)

	_, _, width, _ := app.accounts.GetRect()
	if want := app.accounts.RequiredWidth(); width != want {
		t.Errorf("accounts panel width = %d, want %d", width, want)
	}
	row, _ := app.accounts.GetItemText(0)
	if want := tview.TaggedStringWidth(row) + 2; width < want {
		t.Errorf("accounts panel width = %d, want at least %d to fit row %q", width, want, row)
	}

	initialWidth := width
	must(t, reg.PauseAccount(acc, time.Now().AddDate(0, 0, -5)))
	app.refreshAccounts()
	app.mainFlex.Draw(screen)

	_, _, width, _ = app.accounts.GetRect()
	if want := app.accounts.RequiredWidth(); width != want {
		t.Errorf("accounts panel width after pause = %d, want %d", width, want)
	}
	if width <= initialWidth {
		t.Errorf("accounts panel width after pause = %d, want greater than %d", width, initialWidth)
	}
	row, _ = app.accounts.GetItemText(0)
	if want := tview.TaggedStringWidth(row) + 2; width < want {
		t.Errorf("accounts panel width after pause = %d, want at least %d to fit row %q", width, want, row)
	}
}

// TestNew_AccountInfoFullWidthAndWrapHeight verifies the account panel spans
// the content width and grows with wrapped project text, while the top row
// keeps its minimum height on very short terminals.
func TestNew_AccountInfoFullWidthAndWrapHeight(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	must(t, reg.AddVendor(model.NewVendor("claude", true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor("claude"))
	must(t, err)
	proj := model.NewProject(strings.Repeat("long-project-", 12), "", "")
	must(t, reg.AddProject(proj))
	must(t, reg.AddProjectToAccount(acc, proj))
	_, err = reg.AddSession(acc, proj, "", nil)
	must(t, err)

	svc, err := service.New(layoutStore{reg: reg})
	must(t, err)
	app, err := New(svc)
	must(t, err)

	screen := tcell.NewSimulationScreen("UTF-8")
	must(t, screen.Init())
	t.Cleanup(screen.Fini)

	app.resizeAccountInfo(80, 24, statusBarHeight)
	app.mainFlex.SetRect(0, 0, 80, 24-statusBarHeight)
	app.mainFlex.Draw(screen)

	_, _, infoWidth, infoHeight := app.accountInfo.GetRect()
	if infoWidth != 80 {
		t.Errorf("account info width = %d, want 80", infoWidth)
	}
	wantHeight := app.accountInfo.RequiredHeight(80)
	if infoHeight != wantHeight {
		t.Errorf("account info height = %d, want %d", infoHeight, wantHeight)
	}
	if wantHeight <= 5 {
		t.Errorf("account info height = %d, want more than 5 for wrapped project text", wantHeight)
	}
	_, _, _, topHeight := app.mainTop.GetRect()
	if topHeight < minMainTopHeight {
		t.Errorf("main top height = %d, want at least %d", topHeight, minMainTopHeight)
	}

	app.resizeAccountInfo(80, statusBarHeight+minMainTopHeight, statusBarHeight)
	app.mainFlex.SetRect(0, 0, 80, minMainTopHeight)
	app.mainFlex.Draw(screen)

	_, _, _, infoHeight = app.accountInfo.GetRect()
	if infoHeight != 0 {
		t.Errorf("account info height = %d, want 0 when only the top minimum fits", infoHeight)
	}
	_, _, _, topHeight = app.mainTop.GetRect()
	if topHeight != minMainTopHeight {
		t.Errorf("main top height = %d, want %d", topHeight, minMainTopHeight)
	}
}

// TestNew_DetailPanelsFitContent verifies the vendor, project, and user
// detail panels are sized to their wrapped content rather than a fixed height.
func TestNew_DetailPanelsFitContent(t *testing.T) {
	t.Parallel()
	reg := model.NewRegistry()
	vendorName := strings.Repeat("vendor-", 12)
	must(t, reg.AddVendor(model.NewVendor(vendorName, true, false)))
	must(t, reg.AddUser(model.NewUser("alice@example.com", "Alice")))
	must(t, reg.AddUser(model.NewUser(strings.Repeat("long-", 20)+"@example.com", "Aa"+strings.Repeat("long ", 20))))
	acc, err := reg.AddAccount(reg.User("alice@example.com"), reg.Vendor(vendorName))
	must(t, err)
	proj := model.NewProject("Alpha", strings.Repeat("description ", 12), "instructions")
	must(t, reg.AddProject(proj))
	must(t, reg.AddProjectToAccount(acc, proj))

	svc, err := service.New(layoutStore{reg: reg})
	must(t, err)
	app, err := New(svc)
	must(t, err)

	screen := tcell.NewSimulationScreen("UTF-8")
	must(t, screen.Init())
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	app.resizeDetailPanels(80, 24, statusBarHeight)

	app.vendors.SetRect(0, 0, 80, 24-statusBarHeight)
	app.vendors.Draw(screen)
	vendorInfo, ok := app.vendors.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected vendor info *tview.TextView, got %T", app.vendors.GetItem(1))
	}
	_, _, _, vendorHeight := vendorInfo.GetRect()
	if want := app.vendors.RequiredInfoHeight(80); vendorHeight != want {
		t.Errorf("vendor info height = %d, want %d", vendorHeight, want)
	}
	if want := app.vendors.RequiredInfoHeight(80); want <= 5 {
		t.Errorf("vendor info height = %d, want more than 5 for wrapped name", want)
	}

	app.projects.SetRect(0, 0, 80, 24-statusBarHeight)
	app.projects.Draw(screen)
	projectInfo, ok := app.projects.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected project info *tview.TextView, got %T", app.projects.GetItem(1))
	}
	_, _, _, projectHeight := projectInfo.GetRect()
	if want := app.projects.RequiredInfoHeight(80); projectHeight != want {
		t.Errorf("project info height = %d, want %d", projectHeight, want)
	}
	if want := app.projects.RequiredInfoHeight(80); want <= 5 {
		t.Errorf("project info height = %d, want more than 5 for wrapped description", want)
	}

	app.users.SetRect(0, 0, 80, 24-statusBarHeight)
	app.users.Draw(screen)
	userInfo, ok := app.users.GetItem(1).(*tview.TextView)
	if !ok {
		t.Fatalf("expected user info *tview.TextView, got %T", app.users.GetItem(1))
	}
	_, _, _, userHeight := userInfo.GetRect()
	if want := app.users.RequiredInfoHeight(80); userHeight != want {
		t.Errorf("user info height = %d, want %d", userHeight, want)
	}
	if want := app.users.RequiredInfoHeight(80); want <= 5 {
		t.Errorf("user info height = %d, want more than 5 for wrapped name/email", want)
	}
}

func TestNew_RightColumnHeights(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	must(t, screen.Init())
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	app.resizeAccountInfo(80, 24, statusBarHeight)
	app.resizeMainRight(80, 24, statusBarHeight)
	app.mainFlex.SetRect(0, 0, 80, 24-statusBarHeight)
	app.mainFlex.Draw(screen)

	_, _, _, topHeight := app.mainTop.GetRect()
	_, _, _, sessionsHeight := app.sessions.GetRect()
	_, _, _, notesHeight := app.notes.GetRect()
	wantNotes := max(3, topHeight/3)
	if notesHeight != wantNotes {
		t.Errorf("notes height = %d, want %d", notesHeight, wantNotes)
	}
	if sessionsHeight != topHeight-wantNotes {
		t.Errorf("sessions height = %d, want %d", sessionsHeight, topHeight-wantNotes)
	}

	app.resizeAccountInfo(80, statusBarHeight+minMainTopHeight, statusBarHeight)
	app.resizeMainRight(80, statusBarHeight+minMainTopHeight, statusBarHeight)
	app.mainFlex.SetRect(0, 0, 80, minMainTopHeight)
	app.mainFlex.Draw(screen)
	_, _, _, sessionsHeight = app.sessions.GetRect()
	_, _, _, notesHeight = app.notes.GetRect()
	if sessionsHeight != 3 || notesHeight != 3 {
		t.Errorf("short layout = sessions %d notes %d, want 3 and 3", sessionsHeight, notesHeight)
	}
}

func TestNew_MinMainTopHeight(t *testing.T) {
	t.Parallel()
	if minMainTopHeight != 6 {
		t.Errorf("minMainTopHeight = %d, want 6", minMainTopHeight)
	}
}

func TestApp_FocusCycle(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	if got := app.tv.GetFocus(); got != app.accounts {
		t.Fatalf("initial focus = %T, want accounts", got)
	}

	next := tcell.NewEventKey(views.KeyFocusNext, 0, tcell.ModNone)
	prev := tcell.NewEventKey(views.KeyFocusPrev, 0, tcell.ModNone)
	app.handleGlobalKeys(next)
	if got := app.tv.GetFocus(); got != app.sessions {
		t.Errorf("Tab focus = %T, want sessions", got)
	}
	app.handleGlobalKeys(next)
	if got := app.tv.GetFocus(); got != app.notes {
		t.Errorf("Tab focus = %T, want notes", got)
	}
	app.handleGlobalKeys(next)
	if got := app.tv.GetFocus(); got != app.accounts {
		t.Errorf("Tab focus = %T, want accounts", got)
	}
	app.handleGlobalKeys(prev)
	if got := app.tv.GetFocus(); got != app.notes {
		t.Errorf("Shift+Tab focus = %T, want notes", got)
	}

	app.pages.SwitchToPage("vendors")
	if got := app.handleGlobalKeys(next); got != next {
		t.Error("Tab on a non-main page should pass through")
	}
}

func TestApp_FocusCycle_ChildFocus(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	prev := tcell.NewEventKey(views.KeyFocusPrev, 0, tcell.ModNone)

	app.tv.SetFocus(app.sessions.List)
	app.handleGlobalKeys(prev)
	if got := app.tv.GetFocus(); got != app.accounts {
		t.Errorf("Shift+Tab from sessions child = %T, want accounts", got)
	}

	app.tv.SetFocus(app.notes.List)
	app.handleGlobalKeys(prev)
	if got := app.tv.GetFocus(); got != app.sessions {
		t.Errorf("Shift+Tab from notes child = %T, want sessions", got)
	}
}

func TestApp_CopyToClipboard(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)

	var copied string
	app.copyFn = func(text string) error {
		copied = text
		return nil
	}
	if err := app.CopyToClipboard("hello"); err != nil {
		t.Fatalf("CopyToClipboard: %v", err)
	}
	if copied != "hello" {
		t.Errorf("copied = %q, want hello", copied)
	}

	app.copyFn = func(string) error { return errors.New("boom") }
	if err := app.CopyToClipboard("x"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want boom", err)
	}

	app.copyFn = nil
	if err := app.CopyToClipboard("x"); !errors.Is(err, apperr.ErrNil) {
		t.Errorf("error = %v, want ErrNil", err)
	}
}

func TestApp_ReadFromClipboard(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)

	app.readFn = func() (string, error) { return "hello", nil }
	if got, err := app.ReadFromClipboard(); err != nil || got != "hello" {
		t.Errorf("ReadFromClipboard() = %q, %v; want hello, nil", got, err)
	}

	app.readFn = func() (string, error) { return "", errors.New("boom") }
	if _, err := app.ReadFromClipboard(); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want boom", err)
	}

	app.readFn = nil
	if _, err := app.ReadFromClipboard(); !errors.Is(err, apperr.ErrNil) {
		t.Errorf("error = %v, want ErrNil", err)
	}
}

func statusTexts(t *testing.T, app *App) (string, string) {
	t.Helper()

	lowerRow, ok := app.statusBar.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected lower row *tview.Flex, got %T", app.statusBar.GetItem(1))
	}
	message, ok := lowerRow.GetItem(3).(*tview.TextView)
	if !ok {
		t.Fatalf("expected message *tview.TextView, got %T", lowerRow.GetItem(3))
	}
	area, ok := app.statusBar.GetItem(2).(*tview.TextView)
	if !ok {
		t.Fatalf("expected message area *tview.TextView, got %T", app.statusBar.GetItem(2))
	}
	return message.GetText(false), area.GetText(false)
}

func newStatusUpdates(t *testing.T) (func(func()), <-chan func()) {
	t.Helper()
	updates := make(chan func(), 1)
	return func(fn func()) { updates <- fn }, updates
}

func TestApp_SetStatus_ClearsAfterTimeout(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	app.statusTimeout = 10 * time.Millisecond
	queue, updates := newStatusUpdates(t)
	app.statusQueue = queue

	app.SetStatus("[green]saved[-]")
	if got, _ := statusTexts(t, app); got != "[green]saved[-]" {
		t.Errorf("status = %q, want saved", got)
	}

	select {
	case fn := <-updates:
		fn()
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the status to clear")
	}
	if got, _ := statusTexts(t, app); got != "" {
		t.Errorf("status = %q, want empty after timeout", got)
	}
}

func TestApp_SetNotice_ClearsAfterTimeout(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	app.statusTimeout = 10 * time.Millisecond
	queue, updates := newStatusUpdates(t)
	app.statusQueue = queue

	app.SetNotice("No vendors configured.")
	if _, got := statusTexts(t, app); got != "No vendors configured." {
		t.Errorf("notice = %q, want the configured notice", got)
	}

	select {
	case fn := <-updates:
		fn()
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the notice to clear")
	}
	if _, got := statusTexts(t, app); got != "" {
		t.Errorf("notice = %q, want empty after timeout", got)
	}
}

func TestApp_SetError_DoesNotClearAfterTimeout(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	app.statusTimeout = 10 * time.Millisecond
	queue, updates := newStatusUpdates(t)
	app.statusQueue = queue

	app.SetError("save failed")

	select {
	case <-updates:
		t.Fatal("error status must not clear automatically")
	case <-time.After(30 * time.Millisecond):
	}
	if _, got := statusTexts(t, app); got != "save failed" {
		t.Errorf("error = %q, want save failed", got)
	}
}

func TestApp_SetError_CancelsPendingStatusTimer(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	app.statusTimeout = 10 * time.Millisecond
	queue, updates := newStatusUpdates(t)
	app.statusQueue = queue

	app.SetStatus("[green]saved[-]")
	app.SetError("save failed")

	select {
	case <-updates:
		t.Fatal("a pending status timer must not clear a newer error")
	case <-time.After(30 * time.Millisecond):
	}
	if _, got := statusTexts(t, app); got != "save failed" {
		t.Errorf("error = %q, want save failed", got)
	}
}

func TestApp_Quit_CancelsPendingStatusTimer(t *testing.T) {
	t.Parallel()
	app := newLayoutApp(t)
	app.statusTimeout = 10 * time.Millisecond
	queue, updates := newStatusUpdates(t)
	app.statusQueue = queue

	app.SetStatus("[green]saved[-]")
	app.Quit()

	select {
	case <-updates:
		t.Fatal("Quit must cancel a pending status timer")
	case <-time.After(30 * time.Millisecond):
	}
}
