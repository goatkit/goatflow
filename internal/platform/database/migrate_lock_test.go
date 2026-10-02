package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func migrateTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("migration lock tests need a test database (TEST_DB_HOST)")
	}
	require.NoError(t, InitTestDB())
	db, err := GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

// While one process holds the migration lock, another cannot take it; it
// can once the first releases it.
func TestMigrationLockExcludesOtherProcesses(t *testing.T) {
	db := migrateTestDB(t)
	driver := GetDBDriver()

	releaseA, err := lockMigrations(context.Background(), db, driver, time.Second)
	require.NoError(t, err)

	_, err = lockMigrations(context.Background(), db, driver, 3*time.Second)
	require.Error(t, err, "a second migrator must not get the lock while the first holds it")
	require.Contains(t, err.Error(), "another process is still migrating")

	releaseA()
	releaseB, err := lockMigrations(context.Background(), db, driver, 3*time.Second)
	require.NoError(t, err, "the lock must be free after release")
	releaseB()
}

// latestMigrationVersion is the highest NNNNNN_*.up.sql in the migrations
// directory RunMigrations uses.
func latestMigrationVersion(t *testing.T) int {
	t.Helper()
	dir := getMigrationsPath()
	require.NotEmpty(t, dir, "migrations directory not found")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	require.NoError(t, err)
	latest := 0
	for _, f := range files {
		n, err := strconv.Atoi(strings.SplitN(filepath.Base(f), "_", 2)[0])
		require.NoError(t, err, f)
		if n > latest {
			latest = n
		}
	}
	return latest
}

// Two processes start against a fresh database at the same time, the second
// one while the first is mid-migration (schema_migrations dirty). Both must
// succeed and the schema must end clean at the latest version. Before the
// lock, the second process forced the in-progress version and the run ended
// dirty.
func TestRunMigrationsConcurrentOnFreshDatabase(t *testing.T) {
	db := migrateTestDB(t)
	// Only a database with no schema at all is safe to migrate here: the
	// shared test databases are built without schema_migrations, so its
	// absence alone does not mean "fresh".
	var n int
	if db.QueryRow(ConvertPlaceholders("SELECT COUNT(*) FROM valid")).Scan(&n) == nil {
		t.Skip("needs an empty database (this one has the GoatFlow schema); point DB_* at a fresh database to run it")
	}
	if _, _, err := getMigrationVersion(db); err == nil {
		t.Skip("needs an empty database (schema_migrations exists); point DB_* at a fresh database to run it")
	}
	if os.Getenv("MIGRATIONS_PATH") == "" {
		t.Setenv("MIGRATIONS_PATH", filepath.Join("..", "..", "..", "migrations"))
	}
	latest := latestMigrationVersion(t)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	firstDone := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(firstDone)
		_, errs[0] = RunMigrations(db)
	}()

	// Start the second process once the first is visibly mid-migration.
	deadline := time.Now().Add(2 * time.Minute)
wait:
	for {
		select {
		case <-firstDone:
			t.Log("first migrator finished before a dirty state was observed; the race was not exercised")
			break wait
		default:
		}
		_, dirty, err := getMigrationVersion(db)
		if err == nil && dirty {
			break
		}
		require.True(t, time.Now().Before(deadline), "first migrator never reached a dirty state")
		time.Sleep(5 * time.Millisecond)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, errs[1] = RunMigrations(db)
	}()
	wg.Wait()

	require.NoError(t, errs[0], "first migrator")
	require.NoError(t, errs[1], "second migrator")
	version, dirty, err := getMigrationVersion(db)
	require.NoError(t, err)
	require.False(t, dirty, "schema must not be left dirty")
	require.Equal(t, latest, version)
}
