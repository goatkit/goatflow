//go:build db

package api

import "os"

// dbAvailable indicates whether integration-level DB dependent tests should run.
// It returns true only when the harness signals readiness via GOTRS_TEST_DB_READY=1
// AND the database connection was successfully established during TestMain.
// This keeps tests DB-neutral by default while allowing driver-specific suites to activate automatically.
func dbAvailable() bool {
	if skipTestsNoDBAvailable {
		return false
	}
	return os.Getenv("GOTRS_TEST_DB_READY") == "1"
}
