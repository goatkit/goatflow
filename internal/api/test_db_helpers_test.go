package api

import "os"

// dbAvailable indicates whether integration-level DB dependent tests should run.
// It returns true when the harness signals readiness via GOATFLOW_TEST_DB_READY=1.
// If DB is unavailable, TestMain now hard-fails, so this check primarily
// allows tests to be more granular about when they need full DB access.
func dbAvailable() bool {
	return os.Getenv("GOATFLOW_TEST_DB_READY") == "1"
}
