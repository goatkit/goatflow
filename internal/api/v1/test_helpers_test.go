package v1

import (
	"sync"
	"testing"

	"github.com/goatkit/goatflow/internal/database"
)

var (
	initDBOnce sync.Once
	initDBErr  error
)

func requireDatabase(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if db, err := database.GetDB(); err == nil && db != nil {
		return
	}

	initDBOnce.Do(func() {
		initDBErr = database.InitTestDB()
	})

	if initDBErr != nil {
		t.Skipf("skipping integration test: %v", initDBErr)
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skipf("skipping integration test: %v", err)
	}
}
