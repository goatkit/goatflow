// Package main — unit tests for the first-boot admin bootstrap.
package main

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// withMySQLDriver makes database.ConvertPlaceholders pass `?` through
// unchanged (the mysql flavour), matching what sqlmock expects.
func withMySQLDriver(t *testing.T) {
	t.Helper()
	t.Setenv("TEST_DB_DRIVER", "mysql")
}

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

// noMarkerRows returns an empty result set for the marker query.
func noMarkerRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"effective_value"})
}

func TestBootstrap_NilDB_NoPanic(t *testing.T) {
	t.Setenv("GOATFLOW_ADMIN_PASSWORD", "secret123")
	bootstrapAdminFromEnv(nil) // must be a no-op
}

func TestBootstrap_EmptyPassword_Noop(t *testing.T) {
	db, mock := newMockDB(t)
	withMySQLDriver(t)
	t.Setenv("GOATFLOW_ADMIN_PASSWORD", "   ")
	bootstrapAdminFromEnv(db)
	assert.NoError(t, mock.ExpectationsWereMet(), "no SQL should run when the env var is blank")
}

func TestBootstrap_AlreadyApplied_Skips(t *testing.T) {
	db, mock := newMockDB(t)
	withMySQLDriver(t)
	t.Setenv("GOATFLOW_ADMIN_PASSWORD", "secret123")

	// Marker row present → no user read, no update, no write.
	mock.ExpectQuery("SELECT effective_value FROM sysconfig_modified").
		WithArgs("admin.bootstrap.applied").
		WillReturnRows(sqlmock.NewRows([]string{"effective_value"}).AddRow("true"))

	bootstrapAdminFromEnv(db)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBootstrap_AdminAlreadyEnabled_Skips(t *testing.T) {
	db, mock := newMockDB(t)
	withMySQLDriver(t)
	t.Setenv("GOATFLOW_ADMIN_PASSWORD", "secret123")

	mock.ExpectQuery("SELECT effective_value FROM sysconfig_modified").
		WithArgs("admin.bootstrap.applied").
		WillReturnRows(noMarkerRows())
	mock.ExpectQuery("SELECT valid_id FROM users").
		WithArgs("root@localhost").
		WillReturnRows(sqlmock.NewRows([]string{"valid_id"}).AddRow(1)) // already enabled

	bootstrapAdminFromEnv(db)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBootstrap_FreshInstall_AppliesAndMarks(t *testing.T) {
	db, mock := newMockDB(t)
	withMySQLDriver(t)
	t.Setenv("GOATFLOW_ADMIN_PASSWORD", "secret123")

	mock.ExpectQuery("SELECT effective_value FROM sysconfig_modified").
		WithArgs("admin.bootstrap.applied").
		WillReturnRows(noMarkerRows())
	mock.ExpectQuery("SELECT valid_id FROM users").
		WithArgs("root@localhost").
		WillReturnRows(sqlmock.NewRows([]string{"valid_id"}).AddRow(2)) // factory-disabled
	mock.ExpectExec("UPDATE users").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT id FROM sysconfig_default").
		WithArgs("setup.assistant.completed").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectExec("INSERT INTO sysconfig_modified").
		WillReturnResult(sqlmock.NewResult(1, 1))

	bootstrapAdminFromEnv(db)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBootstrap_UpdateAffectsZeroRows_SkipsMarker(t *testing.T) {
	db, mock := newMockDB(t)
	withMySQLDriver(t)
	t.Setenv("GOATFLOW_ADMIN_PASSWORD", "secret123")

	mock.ExpectQuery("SELECT effective_value FROM sysconfig_modified").
		WithArgs("admin.bootstrap.applied").
		WillReturnRows(noMarkerRows())
	mock.ExpectQuery("SELECT valid_id FROM users").
		WithArgs("root@localhost").
		WillReturnRows(sqlmock.NewRows([]string{"valid_id"}).AddRow(2))
	mock.ExpectExec("UPDATE users").
		WillReturnResult(sqlmock.NewResult(0, 0)) // race: someone enabled it

	bootstrapAdminFromEnv(db)
	assert.NoError(t, mock.ExpectationsWereMet(), "marker must not be written when the update matched nothing")
}

func TestBootstrap_UpdateFails_NeverMarks(t *testing.T) {
	db, mock := newMockDB(t)
	withMySQLDriver(t)
	t.Setenv("GOATFLOW_ADMIN_PASSWORD", "secret123")

	mock.ExpectQuery("SELECT effective_value FROM sysconfig_modified").
		WithArgs("admin.bootstrap.applied").
		WillReturnRows(noMarkerRows())
	mock.ExpectQuery("SELECT valid_id FROM users").
		WithArgs("root@localhost").
		WillReturnRows(sqlmock.NewRows([]string{"valid_id"}).AddRow(2))
	mock.ExpectExec("UPDATE users").WillReturnError(errors.New("connection lost"))

	bootstrapAdminFromEnv(db)
	assert.NoError(t, mock.ExpectationsWereMet(), "a failed update must not reach the marker write")
}

func TestBootstrapHash_UsesBcrypt(t *testing.T) {
	h, err := hashForBootstrap("p@ssw0rd")
	require.NoError(t, err)
	require.True(t, len(h) >= 60 && (h[:4] == "$2a$" || h[:4] == "$2b$"), "expected a bcrypt hash, got %q", h)

	// Round-trip through the same verify the app's login paths use.
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(h), []byte("p@ssw0rd")))
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(h), []byte("wrong")))
}
