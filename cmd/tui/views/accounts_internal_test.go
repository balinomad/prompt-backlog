package views

import (
	"testing"
	"time"
)

// TestFormatPauseUntil is the one deliberate white-box exception in this
// otherwise black-box (package views_test) suite: formatPauseUntil takes
// now as an explicit parameter specifically to keep the same-day /
// different-day comparison deterministic — see its doc comment.
// formatPauseUntil is called via pauseStatusText from the Accounts list,
// which uses real time.Now(), so testing this indirectly would reintroduce
// exactly the midnight-boundary flakiness the parameter exists to prevent.
func TestFormatPauseUntil(t *testing.T) {
	t.Parallel()
	// A fixed reference instant, not time.Now(): now that formatPauseUntil
	// takes it as a parameter, hardcoding it here removes any dependency
	// on wall-clock timing — see formatPauseUntil's doc comment for the
	// flake this replaces.
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
			gotText, gotIsPast := formatPauseUntil(tc.t, now)
			if gotText != tc.wantText {
				t.Errorf("text: got %q, want %q", gotText, tc.wantText)
			}
			if gotIsPast != tc.wantIsPast {
				t.Errorf("isPast: got %v, want %v", gotIsPast, tc.wantIsPast)
			}
		})
	}
}
