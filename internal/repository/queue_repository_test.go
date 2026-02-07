//go:build integration

package repository

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// getTestDB returns a database connection for testing.
func getTestDB() (*sql.DB, error) {
	driver := currentDriver()

	host := firstNonEmpty(os.Getenv("TEST_DB_HOST"), os.Getenv("DB_HOST"), defaultHost(driver))
	user := firstNonEmpty(os.Getenv("TEST_DB_USER"), os.Getenv("DB_USER"), defaultUser(driver))
	password := firstNonEmpty(os.Getenv("TEST_DB_PASSWORD"), os.Getenv("DB_PASSWORD"), defaultPassword(driver))
	dbName := firstNonEmpty(os.Getenv("TEST_DB_NAME"), os.Getenv("DB_NAME"), defaultDBName(driver))
	port := firstNonEmpty(os.Getenv("TEST_DB_PORT"), os.Getenv("DB_PORT"), defaultPort(driver))

	switch driver {
	case "postgres", "pgsql", "pg":
		sslMode := firstNonEmpty(os.Getenv("TEST_DB_SSLMODE"), os.Getenv("DB_SSL_MODE"), "disable")
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbName, sslMode)
		return sql.Open("postgres", connStr)
	case "mysql", "mariadb":
		connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC", user, password, host, port, dbName)
		return sql.Open("mysql", connStr)
	default:
		return nil, fmt.Errorf("unsupported TEST_DB_DRIVER %q", driver)
	}
}

// TestRequiredQueueExists verifies that a required queue exists in the database after OTRS import.
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
	query := fmt.Sprintf("SELECT COUNT(*) FROM queue WHERE name = %s", placeholder(currentDriver(), 1))
	err = db.QueryRow(query, queueName).Scan(&count)
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
		validQuery := fmt.Sprintf("SELECT valid_id FROM queue WHERE name = %s", placeholder(currentDriver(), 1))
		err = db.QueryRow(validQuery, queueName).Scan(&validID)
		if err != nil {
			t.Fatalf("Failed to get valid_id for queue %s: %v", queueName, err)
		}

		if validID != 1 {
			t.Errorf("Queue %s exists but is not valid (valid_id = %d, expected 1)", queueName, validID)
		}
	}
}

// TestEssentialQueuesExist verifies that all essential OTRS queues exist.
func TestEssentialQueuesExist(t *testing.T) {
	essentialQueues := []string{
		"Postmaster",
		"Raw",
		"Junk",
		"Misc",
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
			query := fmt.Sprintf("SELECT COUNT(*) FROM queue WHERE name = %s", placeholder(currentDriver(), 1))
			err := db.QueryRow(query, queueName).Scan(&count)
			if err != nil {
				t.Fatalf("Failed to query for queue %s: %v", queueName, err)
			}

			if count == 0 {
				t.Errorf("Essential queue '%s' does not exist in database", queueName)
			}
		})
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func defaultHost(driver string) string {
	switch driver {
	case "mysql", "mariadb":
		return "mariadb-test"
	default:
		return "postgres-test"
	}
}

func defaultPort(driver string) string {
	switch driver {
	case "mysql", "mariadb":
		return "3306"
	default:
		return "5432"
	}
}

func defaultUser(driver string) string {
	switch driver {
	case "mysql", "mariadb":
		return "otrs"
	default:
		return "goatflow_user"
	}
}

func defaultPassword(driver string) string {
	// Password must come from environment - no hardcoded defaults
	if pw := os.Getenv("TEST_DB_PASSWORD"); pw != "" {
		return pw
	}
	switch driver {
	case "mysql", "mariadb":
		if pw := os.Getenv("TEST_DB_MYSQL_PASSWORD"); pw != "" {
			return pw
		}
	default:
		if pw := os.Getenv("TEST_DB_POSTGRES_PASSWORD"); pw != "" {
			return pw
		}
	}
	// This will cause connection failure - password must be set
	return ""
}

func defaultDBName(driver string) string {
	switch driver {
	case "mysql", "mariadb":
		return "otrs_test"
	default:
		return "goatflow_test"
	}
}

func currentDriver() string {
	driver := strings.ToLower(firstNonEmpty(os.Getenv("TEST_DB_DRIVER"), os.Getenv("DB_DRIVER")))
	if driver == "" {
		if strings.Contains(strings.ToLower(os.Getenv("DATABASE_URL")), "mysql") {
			driver = "mysql"
		} else {
			driver = "postgres"
		}
	}
	return driver
}

func placeholder(driver string, position int) string {
	switch driver {
	case "mysql", "mariadb":
		return "?"
	default:
		return fmt.Sprintf("$%d", position)
	}
}
