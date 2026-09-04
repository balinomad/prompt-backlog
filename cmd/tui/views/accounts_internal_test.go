package views

import (
	"testing"
	"time"
)

// TestFormatPauseUntil is the one deliberate white-box exception in this
// otherwise black-box (package views_test) suite: formatPauseUntil takes
// now as an explicit parameter specifically to keep the same-day /
// different-day comparison deterministic — see its doc comment.
// accountItemText, its only caller, uses real time.Now(), so testing this
// indirectly through AccountsPanel would reintroduce exactly the
// midnight-boundary flakiness the parameter exists to prevent.
func TestFormatPauseUntil(t *testing.T) {
	t.Parallel()
	// A fixed reference instant, not time.Now(): now that formatPauseUntil
	// takes it as a parameter, hardcoding it here removes any dependency
	// on wall-clock timing — see formatPauseUntil's doc comment for the
	// flake this replaces.
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	sameDay := time.Date(2026, 8, 27, 14, 30, 0, 0, time.UTC)
	otherDay := time.Date(2027, 6, 15, 9, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name       string
		t          time.Time
		wantText   string
		wantIsPast bool
	}{
		{"same day format", sameDay, "14:30", sameDay.Before(now)},
		{"other day format", otherDay, "06-15 09:00", false},
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
