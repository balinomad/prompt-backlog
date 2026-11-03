package views_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/balinomad/llm-desk/cmd/tui/views"
	"github.com/balinomad/llm-desk/cmd/tui/views/panels"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// statusBarParts returns sb's text views: the upper specific hints, the lower
// common hints and inline message, and the separate message area.
//
//nolint:nonamedreturns // multiple returns of the same type, hard to differentiate
func statusBarParts(t *testing.T, sb *views.StatusBar) (specificHints, commonHints, message, messageArea *tview.TextView) {
	t.Helper()
	spec, ok := sb.GetItem(0).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 0, got %T", sb.GetItem(0))
	}
	lowerRow, ok := sb.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected inner *tview.Flex at index 1, got %T", sb.GetItem(1))
	}
	common, ok := lowerRow.GetItem(0).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at lower row index 0, got %T", lowerRow.GetItem(0))
	}
	msg, ok := lowerRow.GetItem(3).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at lower row index 3, got %T", lowerRow.GetItem(3))
	}
	area, ok := sb.GetItem(2).(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView at index 2, got %T", sb.GetItem(2))
	}
	return spec, common, msg, area
}

func TestNewStatusBar(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	if sb == nil {
		t.Fatal("expected non-nil StatusBar")
	}
}

func TestStatusBar_DefaultHints(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	spec, common, _, _ := statusBarParts(t, sb)
	if spec.GetText(false) != views.HintsAccountsPanelSpecific {
		t.Errorf("default specific hints: got %q, want HintsAccountsPanelSpecific", spec.GetText(false))
	}
	if common.GetText(false) != views.HintsAccountsPanelCommon {
		t.Errorf("default common hints: got %q, want HintsAccountsPanelCommon", common.GetText(false))
	}
}

func TestStatusBar_DefaultMessageEmpty(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	_, _, message, area := statusBarParts(t, sb)
	if message.GetText(false) != "" {
		t.Errorf("expected empty inline message, got %q", message.GetText(false))
	}
	if area.GetText(false) != "" {
		t.Errorf("expected empty notice/error area, got %q", area.GetText(false))
	}
}

func TestStatusBar_SetHints(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		specific string
		common   string
	}{
		{"accounts panel", views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon},
		{"sessions pane", views.HintsSessionsPanelSpecific, views.HintsSessionsPanelCommon},
		{"notes pane", views.HintsNotesPanelSpecific, views.HintsNotesPanelCommon},
		{"projects panel", views.HintsProjectsPanelSpecific, views.HintsProjectsPanelCommon},
		{"project accounts panel", views.HintsProjectAccountsSpecific, views.HintsProjectAccountsCommon},
		{"project notes panel", views.HintsProjectNotesSpecific, views.HintsProjectNotesCommon},
		{"vendors panel", views.HintsVendorsPanelSpecific, views.HintsVendorsPanelCommon},
		{"users panel", views.HintsUsersPanelSpecific, views.HintsUsersPanelCommon},
		{"form", views.HintsFormSpecific, views.HintsFormCommon},
		{"confirm modal", views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon},
		{"help overlay", views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sb := views.NewStatusBar()
			sb.SetHints(tc.specific, tc.common)
			spec, common, _, _ := statusBarParts(t, sb)
			if spec.GetText(false) != tc.specific {
				t.Errorf("SetHints(%q): specific got %q", tc.name, spec.GetText(false))
			}
			if common.GetText(false) != tc.common {
				t.Errorf("SetHints(%q): common got %q", tc.name, common.GetText(false))
			}
		})
	}
}

// TestStatusBar_CommonHintsConstantsNonEmpty verifies every context has at
// least one common hint. Context-specific lines may be empty.
func TestStatusBar_CommonHintsConstantsNonEmpty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value string
	}{
		{"HintsAccountsPanelCommon", views.HintsAccountsPanelCommon},
		{"HintsSessionsPanelCommon", views.HintsSessionsPanelCommon},
		{"HintsNotesPanelCommon", views.HintsNotesPanelCommon},
		{"HintsProjectsPanelCommon", views.HintsProjectsPanelCommon},
		{"HintsProjectAccountsCommon", views.HintsProjectAccountsCommon},
		{"HintsProjectNotesCommon", views.HintsProjectNotesCommon},
		{"HintsVendorsPanelCommon", views.HintsVendorsPanelCommon},
		{"HintsUsersPanelCommon", views.HintsUsersPanelCommon},
		{"HintsFormCommon", views.HintsFormCommon},
		{"HintsConfirmModalCommon", views.HintsConfirmModalCommon},
		{"HintsHelpOverlayCommon", views.HintsHelpOverlayCommon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.value == "" {
				t.Errorf("%s must not be empty", tc.name)
			}
		})
	}
}

func TestStatusBar_SetStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		msg  string
	}{
		{name: "non-empty message", msg: "saved"},
		{name: "empty clears message", msg: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sb := views.NewStatusBar()
			sb.SetStatus(tc.msg)
			_, _, message, area := statusBarParts(t, sb)
			if message.GetText(false) != tc.msg {
				t.Errorf("SetStatus(%q): got %q", tc.msg, message.GetText(false))
			}
			if area.GetText(false) != "" {
				t.Errorf("SetStatus(%q): notice/error area = %q, want empty", tc.msg, area.GetText(false))
			}
		})
	}
}

func TestStatusBar_SetNotice(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	sb.SetStatus("saved")
	sb.SetNotice("No vendors configured.")
	_, _, message, area := statusBarParts(t, sb)
	if message.GetText(false) != "" {
		t.Errorf("inline message = %q, want empty", message.GetText(false))
	}
	if got, want := area.GetText(false), "No vendors configured."; got != want {
		t.Errorf("notice = %q, want %q", got, want)
	}
	if got, want := sb.Layout(80), views.StatusBarBaseHeight+1; got != want {
		t.Errorf("Layout(80) = %d, want %d", got, want)
	}
}

func TestStatusBar_NoticeColor(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 3)

	sb.SetNotice("No vendors configured.")
	sb.Layout(80)
	sb.SetRect(0, 0, 80, 3)
	sb.Draw(screen)
	screen.Show()

	cells, width, _ := screen.GetContents()
	cell := cells[2*width]
	if got, _, _ := cell.Style.Decompose(); got != tcell.ColorYellow {
		t.Errorf("notice foreground = %v, want %v", got, tcell.ColorYellow)
	}
}

func TestStatusBar_SetErrorWrapsAndCaps(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	sb.SetStatus("saved")
	sb.SetError(strings.Repeat("error ", 100))
	_, _, message, area := statusBarParts(t, sb)
	if message.GetText(false) != "" {
		t.Errorf("inline message = %q, want empty", message.GetText(false))
	}
	if area.GetText(false) == "" {
		t.Error("error area is empty")
	}
	if got, want := sb.Layout(40), views.StatusBarBaseHeight+3; got != want {
		t.Errorf("Layout(40) = %d, want %d", got, want)
	}
}

func TestStatusBar_LayoutHeight(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	if got := sb.Layout(80); got != views.StatusBarBaseHeight {
		t.Errorf("empty Layout(80) = %d, want %d", got, views.StatusBarBaseHeight)
	}
	sb.SetStatus("saved")
	if got := sb.Layout(80); got != views.StatusBarBaseHeight {
		t.Errorf("inline status Layout(80) = %d, want %d", got, views.StatusBarBaseHeight)
	}
}

func TestStatusBar_InlineMessageWidth(t *testing.T) {
	t.Parallel()
	sb := views.NewStatusBar()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 2)

	_, common, message, _ := statusBarParts(t, sb)
	lowerRow, ok := sb.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected lower row *tview.Flex, got %T", sb.GetItem(1))
	}
	spacer, ok := lowerRow.GetItem(1).(*tview.Box)
	if !ok {
		t.Fatalf("expected spacer *tview.Box, got %T", lowerRow.GetItem(1))
	}
	draw := func() (int, int, int, int) {
		sb.Layout(80)
		sb.SetRect(0, 0, 80, 2)
		sb.Draw(screen)
		_, _, commonWidth, _ := common.GetRect()
		_, _, spacerWidth, _ := spacer.GetRect()
		messageX, _, messageWidth, _ := message.GetRect()
		return commonWidth, spacerWidth, messageWidth, messageX
	}

	commonWidth, spacerWidth, messageWidth, _ := draw()
	if commonWidth != 50 {
		t.Errorf("empty common width = %d, want 50", commonWidth)
	}
	if spacerWidth != 0 {
		t.Errorf("empty spacer width = %d, want 0", spacerWidth)
	}
	if messageWidth != 0 {
		t.Errorf("empty message width = %d, want 0", messageWidth)
	}

	sb.SetStatus("saved ✓")
	commonWidth, spacerWidth, messageWidth, messageX := draw()
	if commonWidth != 50 {
		t.Errorf("inline common width = %d, want 50", commonWidth)
	}
	if spacerWidth != 2 {
		t.Errorf("inline spacer width = %d, want 2", spacerWidth)
	}
	if messageWidth != 7 {
		t.Errorf("inline message width = %d, want 7", messageWidth)
	}
	if messageX+messageWidth != 80 {
		t.Errorf("inline message right edge = %d, want 80", messageX+messageWidth)
	}
}

func TestHintLinesExactText(t *testing.T) {
	t.Parallel()
	strip := regexp.MustCompile(`\[[^\]]*\]`)
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"accounts", views.HintsAccountsPanelSpecific, "Tab sessions · Enter pause · Ins add · Del delete"},
		{"sessions", views.HintsSessionsPanelSpecific, "Enter edit · Ins new session · Del delete"},
		{"notes", views.HintsNotesPanelSpecific, "Enter edit · Ins add · Del delete · s start"},
		{"project accounts", views.HintsProjectAccountsSpecific, "Tab project notes · Ins add · Del delete"},
		{"project notes", views.HintsProjectNotesSpecific, "Tab projects · Enter edit · Ins add · Del delete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := strip.ReplaceAllString(tc.got, ""); got != tc.want {
				t.Errorf("hint = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMinimumWidth(t *testing.T) {
	t.Parallel()
	if got, want := views.MinimumWidth(7), 74; got != want {
		t.Errorf("MinimumWidth(7) = %d, want %d", got, want)
	}
}

// ── NewConfirmModal ───────────────────────────────────────────────────────────

func TestNewConfirmModal_EscCallsCancel(t *testing.T) {
	t.Parallel()
	var cancelled bool
	prim := views.UnwrapModal(views.NewConfirmModal("test?", func() {}, func() { cancelled = true }))
	modal, ok := prim.(*tview.Modal)
	if !ok {
		t.Fatal("NewConfirmModal must return *tview.Modal")
	}
	capture := modal.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	result := capture(escKey())
	if result != nil {
		t.Error("Esc must consume the event (return nil)")
	}
	if !cancelled {
		t.Error("Esc must call onCancel")
	}
}

func TestNewConfirmModal_NonEscPassesThrough(t *testing.T) {
	t.Parallel()
	prim := views.UnwrapModal(views.NewConfirmModal("test?", func() {}, func() {}))
	modal, ok := prim.(*tview.Modal)
	if !ok {
		t.Fatal("NewConfirmModal must return *tview.Modal")
	}
	capture := modal.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	event := runeKey('n')
	if got := capture(event); got != event {
		t.Error("non-Esc key must pass through unchanged")
	}
}

func TestNewConfirmModal_ReturnsModal(t *testing.T) {
	t.Parallel()
	prim := views.UnwrapModal(views.NewConfirmModal("message", func() {}, func() {}))
	if _, ok := prim.(*tview.Modal); !ok {
		t.Error("expected *tview.Modal")
	}
}

// ── NewHelpOverlay / showHelp ───────────────────────────────────────────────────

// helpOverlayTextView drills into the Flex/Flex/Frame structure
// NewHelpOverlay returns to reach the inner TextView, where its Esc
// handling is actually registered.
func helpOverlayTextView(t *testing.T, overlay tview.Primitive) *tview.TextView {
	t.Helper()
	overlay = views.UnwrapModal(overlay)
	outer, ok := overlay.(*tview.Flex)
	if !ok {
		t.Fatalf("expected *tview.Flex, got %T", overlay)
	}
	inner, ok := outer.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("expected inner *tview.Flex, got %T", outer.GetItem(1))
	}
	frame, ok := inner.GetItem(1).(*tview.Frame)
	if !ok {
		t.Fatalf("expected *tview.Frame, got %T", inner.GetItem(1))
	}
	text, ok := frame.GetPrimitive().(*tview.TextView)
	if !ok {
		t.Fatalf("expected *tview.TextView, got %T", frame.GetPrimitive())
	}
	return text
}

func TestNewHelpOverlay_EscCallsOnClose(t *testing.T) {
	t.Parallel()
	var closed bool
	overlay := views.NewHelpOverlay(func() { closed = true })
	capture := helpOverlayTextView(t, overlay).GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	result := capture(escKey())
	if result != nil {
		t.Error("Esc must consume the event (return nil)")
	}
	if !closed {
		t.Error("Esc must call onClose")
	}
}

// TestShowHelp_EscRestoresHintsAndHidesModal exercises the unexported
// showHelp indirectly through AccountsPanel's KeyHelp handling —
// AccountsPanel embeds *tview.List directly, so its GetInputCapture()
// reaches handleKey from outside the package. The restored-hints value is
// necessarily HintsAccountsPanel here, since that's what
// AccountsPanel.handleKey passes to showHelp; the round trip itself
// (open, HintsHelpOverlay, Esc, hints restored, modal hidden) is what's
// under test, not the specific string.
func TestShowHelp_EscRestoresHintsAndHidesModal(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := panels.NewAccountsPanel(app)

	capture := p.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(runeKey(views.KeyHelp))

	if app.Modal == nil {
		t.Fatal("expected a modal to be shown")
	}
	assertHints(t, app, views.HintsHelpOverlaySpecific, views.HintsHelpOverlayCommon)

	helpOverlayTextView(t, app.Modal).GetInputCapture()(escKey())

	if app.Modal != nil {
		t.Error("expected modal to be hidden after Esc")
	}
	assertHints(t, app, views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
}

// TestConfirmQuit_EscRestoresHintsAndHidesModal exercises the unexported
// confirmQuit indirectly through AccountsPanel's KeyQuit handling, the
// same way TestShowHelp_EscRestoresHintsAndHidesModal reaches showHelp.
// AccountsPanel embeds *tview.List directly, so its GetInputCapture()
// reaches handleKey from outside the package. The restored-hints value is
// necessarily HintsAccountsPanel here, since that's what
// AccountsPanel.handleKey passes to confirmQuit via DispatchPanelKey; the
// round trip itself (open, HintsConfirmModal, Esc, hints restored, modal
// hidden, Quit not called) is what's under test, not the specific string.
func TestConfirmQuit_EscRestoresHintsAndHidesModal(t *testing.T) {
	t.Parallel()
	app := newMockApp(t)
	p := panels.NewAccountsPanel(app)

	capture := p.GetInputCapture()
	if capture == nil {
		t.Fatal("expected non-nil input capture")
	}
	capture(runeKey(views.KeyQuit))

	if app.Modal == nil {
		t.Fatal("expected a modal to be shown")
	}
	assertHints(t, app, views.HintsConfirmModalSpecific, views.HintsConfirmModalCommon)

	confirmModal := confirmModalFrom(t, app.Modal)
	confirmModal.GetInputCapture()(escKey())

	if app.Modal != nil {
		t.Error("expected modal to be hidden after Esc")
	}
	assertHints(t, app, views.HintsAccountsPanelSpecific, views.HintsAccountsPanelCommon)
	if app.Quitted {
		t.Error("cancelling quit must not actually quit")
	}
}

func TestHelpOverlayContent(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"Accounts list",
		"Sessions pane",
		"Notes pane",
		"Cycle project filter",
		"Start session from note",
		"Edit prompt",
		"Project notes",
		"Switch to project notes",
		"Add note to project",
	} {
		if !strings.Contains(views.HelpText, want) {
			t.Errorf("help text does not contain %q", want)
		}
	}
	for _, removed := range []string{"Session detail", "Messages editor", "messages editor", "Space"} {
		if strings.Contains(views.HelpText, removed) {
			t.Errorf("help text still contains %q", removed)
		}
	}
}
