package api

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func insertWebhookRow(t *testing.T, query string, args ...interface{}) int {
	t.Helper()

	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	// Use adapter which handles placeholder conversion, arg remapping, and RETURNING
	id64, err := database.GetAdapter().InsertWithReturning(db, query, args...)
	require.NoError(t, err)
	return int(id64)
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
