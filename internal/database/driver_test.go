//go:build integration

package database_test

import (
	"context"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/database"
	_ "github.com/goatkit/goatflow/internal/database/drivers/mysql"
	_ "github.com/goatkit/goatflow/internal/database/drivers/postgres"
	_ "github.com/goatkit/goatflow/internal/database/drivers/sqlite"
	"github.com/goatkit/goatflow/internal/database/schema"
)

func TestDatabaseAbstraction(t *testing.T) {
	// Get SQLite driver for testing
	driver, err := database.GetDriver("sqlite")
	if err != nil {
		t.Fatal("Failed to get SQLite driver:", err)
	}

	// Connect to in-memory database
	ctx := context.Background()
	if err := driver.Connect(ctx, ":memory:"); err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skipf("sqlite driver unavailable: %v", err)
		}
		t.Fatal("Failed to connect to SQLite:", err)
	}
	defer driver.Close()

	// Test schema loading
	loader := schema.NewSchemaLoader("schemas")

	yamlContent := `
customer_company:
  pk: customer_id
  columns:
    customer_id:
      type: varchar(150)
      required: true
    name:
      type: varchar(200)
      required: true
      unique: true
    valid_id:
      type: smallint
      required: true
      default: 1
  timestamps: true
`

	if err := loader.LoadFromString(yamlContent); err != nil {
		t.Fatal("Failed to load schema:", err)
	}

	// Get the schema
	tableSchema, ok := loader.GetSchema("customer_company")
	if !ok {
		t.Fatal("Schema not found")
	}

	// Test CREATE TABLE generation
	createQuery, err := driver.CreateTable(tableSchema)
	if err != nil {
		t.Fatal("Failed to generate CREATE TABLE:", err)
	}

	if createQuery.SQL == "" {
		t.Error("Expected non-empty CREATE TABLE SQL")
	}

	// Execute the CREATE TABLE
	if _, err := driver.Exec(ctx, createQuery.SQL); err != nil {
		t.Fatal("Failed to create table:", err)
	}

	// Test INSERT generation
	insertData := map[string]interface{}{
		"customer_id": "CUST001",
		"name":        "Test Company",
		"valid_id":    1,
		"create_by":   1,
		"change_by":   1,
	}

	insertQuery, err := driver.Insert("customer_company", insertData)
	if err != nil {
		t.Fatal("Failed to generate INSERT:", err)
	}

	if len(insertQuery.Args) != len(insertData) {
		t.Errorf("Expected %d args, got %d", len(insertData), len(insertQuery.Args))
	}

	// Test type mapping
	testTypes := map[string]string{
		"serial":  "INTEGER",
		"text":    "TEXT",
		"boolean": "INTEGER",
	}

	for input, expected := range testTypes {
		mapped := driver.MapType(input)
		if mapped != expected {
			t.Errorf("MapType(%s) = %s; want %s", input, mapped, expected)
		}
	}

	// Test feature detection
	if driver.SupportsArrays() {
		t.Error("SQLite should not support arrays")
	}

	if !driver.SupportsLastInsertId() {
		t.Error("SQLite should support LastInsertId")
	}
}

func TestDriverRegistry(t *testing.T) {
	// Test that drivers are registered
	drivers := []string{"sqlite", "sqlite3", "postgres", "postgresql", "mysql", "mariadb"}

	for _, name := range drivers {
		driver, err := database.GetDriver(name)
		if err != nil {
			t.Errorf("Expected driver %s to be registered", name)
		}
		if driver == nil {
			t.Errorf("Driver %s returned nil", name)
		}
	}

	// Test non-existent driver
	_, err := database.GetDriver("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent driver")
	}
}
