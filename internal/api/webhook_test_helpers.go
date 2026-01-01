package api

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func insertWebhookRow(t *testing.T, query string, args ...interface{}) int {
	t.Helper()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	placeholderQuery := database.ConvertPlaceholders(query)
	converted, needsLastInsert := database.ConvertReturning(placeholderQuery)

	if database.IsMySQL() {
		// Run inserts inside a transaction so LAST_INSERT_ID reads use the same session.
		tx, txErr := db.Begin()
		require.NoError(t, txErr)
		defer func() { _ = tx.Rollback() }()

		execArgs := database.RemapArgsForMySQL(query, args)
		result, execErr := tx.Exec(converted, execArgs...)
		require.NoError(t, execErr)

		id := lastInsertID(t, tx, result)
		require.NoError(t, tx.Commit())
		return id
	}

	if needsLastInsert {
		var id int
		require.NoError(t, db.QueryRow(converted, args...).Scan(&id))
		return id
	}

	_, execErr := db.Exec(converted, args...)
	require.NoError(t, execErr)
	return 0
}

func lastInsertID(t *testing.T, querier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}, result sql.Result) int {
	t.Helper()

	id, err := result.LastInsertId()
	if err == nil && id != 0 {
		return int(id)
	}

	var fallback int64
	require.NoError(t, querier.QueryRow("SELECT LAST_INSERT_ID()").Scan(&fallback))
	require.NotZero(t, fallback)
	return int(fallback)
}

func ensureWebhookTables(t *testing.T) {
	t.Helper()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	var createWebhooks string
	var createDeliveries string

	if database.IsMySQL() {
		createWebhooks = `
			CREATE TABLE IF NOT EXISTS webhooks (
				id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				url VARCHAR(1024) NOT NULL,
				secret VARCHAR(255),
				events TEXT,
				active BOOLEAN DEFAULT true,
				retry_count INT DEFAULT 3,
				timeout_seconds INT DEFAULT 30,
				headers TEXT,
				create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				create_by BIGINT UNSIGNED,
				change_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				change_by BIGINT UNSIGNED
			)`
		createDeliveries = `
			CREATE TABLE IF NOT EXISTS webhook_deliveries (
				id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
				webhook_id BIGINT UNSIGNED,
				event_type VARCHAR(100),
				payload TEXT,
				status_code INT,
				response TEXT,
				attempts INT DEFAULT 0,
				delivered_at TIMESTAMP NULL,
				next_retry TIMESTAMP NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				success BOOLEAN DEFAULT false,
				INDEX idx_webhook_deliveries_webhook_id (webhook_id)
			)`
	} else {
		createWebhooks = `
			CREATE TABLE IF NOT EXISTS webhooks (
				id SERIAL PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				url VARCHAR(1024) NOT NULL,
				secret VARCHAR(255),
				events TEXT,
				active BOOLEAN DEFAULT true,
				retry_count INTEGER DEFAULT 3,
				timeout_seconds INTEGER DEFAULT 30,
				headers TEXT,
				create_time TIMESTAMP DEFAULT NOW(),
				create_by INTEGER,
				change_time TIMESTAMP DEFAULT NOW(),
				change_by INTEGER
			)`
		createDeliveries = `
			CREATE TABLE IF NOT EXISTS webhook_deliveries (
				id SERIAL PRIMARY KEY,
				webhook_id INTEGER REFERENCES webhooks(id),
				event_type VARCHAR(100),
				payload TEXT,
				status_code INTEGER,
				response TEXT,
				attempts INTEGER DEFAULT 0,
				delivered_at TIMESTAMP,
				next_retry TIMESTAMP,
				created_at TIMESTAMP DEFAULT NOW(),
				success BOOLEAN DEFAULT false
			)`
	}

	_, err = db.Exec(createWebhooks)
	require.NoError(t, err)
	_, err = db.Exec(createDeliveries)
	require.NoError(t, err)
}
