package database

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A dirty schema_migrations row means a migration failed or died part-way.
// RunMigrations must report it and leave it for an operator; before, it forced
// the dirty version as applied (skipping the failed migration for good) and
// went on migrating, so an upgrade whose migration failed came up
// half-upgraded on the very next start.
func TestRunMigrationsRefusesDirtySchema(t *testing.T) {
	db := migrateTestDB(t)
	if _, _, err := getMigrationVersion(db); err == nil {
		t.Skip("needs a database without schema_migrations (the shared test databases have none)")
	}
	t.Setenv("MIGRATIONS_PATH", filepath.Join("..", "..", "..", "migrations"))
	latest := latestMigrationVersion(t)

	_, err := db.Exec(ConvertPlaceholders("CREATE TABLE schema_migrations (version BIGINT NOT NULL PRIMARY KEY, dirty BOOLEAN NOT NULL)"))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec(ConvertPlaceholders("DROP TABLE schema_migrations"))
		require.NoError(t, err)
	})
	_, err = db.Exec(ConvertPlaceholders("INSERT INTO schema_migrations (version, dirty) VALUES (?, ?)"), latest, true)
	require.NoError(t, err)

	_, err = RunMigrations(db)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dirty")

	version, dirty, err := getMigrationVersion(db)
	require.NoError(t, err)
	require.True(t, dirty, "the failed migration must stay flagged, not be marked applied")
	require.Equal(t, latest, version)
}
