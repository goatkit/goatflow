//go:build integration
// +build integration

package repository

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// getTestDB returns a database connection for testing
func getTestDB() (*sql.DB, error) {
	// When running in test environment via Makefile, these will be set
	// Otherwise use production database for testing
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"  // Default to localhost if not set
	}
	// When running in container, DB_HOST will be "postgres"
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "gotrs_user"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		// Use a test password for local testing
		password = "test-password-change-me"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "gotrs"
	}
	
	connStr := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", user, password, host, dbName)
	return sql.Open("postgres", connStr)
}

// TestRequiredQueueExists verifies that a required queue exists in the database after OTRS import
func TestRequiredQueueExists(t *testing.T) {
	// This test verifies that a required OTRS-compatible queue is present
	// The queue name is parameterized via environment variable
	
	// Get the queue name to test from environment, default to a generic queue
	queueName := os.Getenv("TEST_QUEUE_NAME")
	if queueName == "" {
		queueName = "Postmaster" // Default to standard OTRS queue
	}
	
	// Connect to the database
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// Check if the queue exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM queue WHERE name = $1", queueName).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query for queue %s: %v", queueName, err)
	}
	
	// Assert that queue exists
	if count == 0 {
		t.Errorf("Required queue %s does not exist in database - this queue is required for OTRS compatibility", queueName)
	}
	
	// Also check that it's valid (active)
	if count > 0 {
		var validID int
		err = db.QueryRow("SELECT valid_id FROM queue WHERE name = $1", queueName).Scan(&validID)
		if err != nil {
			t.Fatalf("Failed to get valid_id for queue %s: %v", queueName, err)
		}
		
		if validID != 1 {
			t.Errorf("Queue %s exists but is not valid (valid_id = %d, expected 1)", queueName, validID)
		}
	}
}

// TestEssentialQueuesExist verifies that all essential OTRS queues exist
func TestEssentialQueuesExist(t *testing.T) {
	essentialQueues := []string{
		"Postmaster",
		"Raw", 
		"Junk",
		"Misc",
		"OBC", // OBC is essential for OTRS operations
	}
	
	// Connect to the database
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()
	
	for _, queueName := range essentialQueues {
		t.Run("Queue_"+queueName, func(t *testing.T) {
			var count int
			err := db.QueryRow("SELECT COUNT(*) FROM queue WHERE name = $1", queueName).Scan(&count)
			if err != nil {
				t.Fatalf("Failed to query for queue %s: %v", queueName, err)
			}
			
			if count == 0 {
				t.Errorf("Essential queue '%s' does not exist in database", queueName)
			}
		})
	}
}