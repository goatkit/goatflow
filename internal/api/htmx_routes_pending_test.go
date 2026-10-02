package api

import "testing"

func TestIsPendingAutoState(t *testing.T) {
	cases := []struct {
		name      string
		stateName string
		stateType string
		expected  bool
	}{
		{"typeMatch", "waiting on response", "pending auto", true},
		{"nameMatch", "Pending Auto-Close+", "new", true},
		{"reminderTypeIsNotAuto", "waiting", "pending reminder", false},
		{"noMatch", "open", "open", false},
	}

	for _, tc := range cases {
		if got := isPendingAutoState(tc.stateName, tc.stateType); got != tc.expected {
			t.Fatalf("%s: expected %v got %v", tc.name, tc.expected, got)
		}
	}
}
