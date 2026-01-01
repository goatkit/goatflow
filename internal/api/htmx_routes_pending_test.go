package api

import "testing"

func TestIsPendingAutoState(t *testing.T) {
	cases := []struct {
		name      string
		stateName string
		stateType int
		expected  bool
	}{
		{"typeMatch", "waiting on response", pendingAutoStateTypeID, true},
		{"nameMatch", "Pending Auto-Close+", 1, true},
		{"noMatch", "open", 1, false},
	}

	for _, tc := range cases {
		if got := isPendingAutoState(tc.stateName, tc.stateType); got != tc.expected {
			t.Fatalf("%s: expected %v got %v", tc.name, tc.expected, got)
		}
	}
}
