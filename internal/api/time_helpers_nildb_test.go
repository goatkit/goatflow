package api

import "testing"

// A time entry that cannot be stored must be reported, not swallowed: the
// callers surface it (X-Guru-Error / 500) instead of claiming it was saved.
func TestSaveTimeEntryWithoutDatabaseFails(t *testing.T) {
	if err := saveTimeEntry(nil, 1, nil, 15, 1); err == nil {
		t.Fatal("saveTimeEntry without a database must return an error")
	}
	if err := saveTimeEntry(nil, 1, nil, 0, 1); err != nil {
		t.Fatalf("nothing to record is not an error: %v", err)
	}
}
