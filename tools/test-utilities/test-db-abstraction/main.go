// Package main provides test utilities for validating the database abstraction layer.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/goatkit/goatflow/internal/database"
	_ "github.com/goatkit/goatflow/internal/database/drivers/mysql"
	_ "github.com/goatkit/goatflow/internal/database/drivers/postgres"
	_ "github.com/goatkit/goatflow/internal/database/drivers/sqlite"
	"github.com/goatkit/goatflow/internal/database/schema"
)

func main() {
	// Test with SQLite in-memory database
	fmt.Println("Testing Database Abstraction Layer with SQLite...")

	// Get SQLite driver
	driver, err := database.GetDriver("sqlite")
	if err != nil {
		log.Fatal("Failed to get SQLite driver:", err)
	}

	// Connect to in-memory database
	ctx := context.Background()
	if err := driver.Connect(ctx, ":memory:"); err != nil {
		log.Fatal("Failed to connect to SQLite:", err)
	}
	defer func() { _ = driver.Close() }()

	// Load schemas
	loader := schema.NewSchemaLoader("schemas")

	// Load customer_company schema from string for testing
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
		log.Fatal("Failed to load schema:", err)
	}

	// Get the schema
	tableSchema, ok := loader.GetSchema("customer_company")
	if !ok {
		log.Fatal("Schema not found")
	}

	// Generate CREATE TABLE SQL
	createQuery, err := driver.CreateTable(tableSchema)
	if err != nil {
		log.Fatal("Failed to generate CREATE TABLE:", err)
	}

	fmt.Printf("Generated SQL:\n%s\n\n", createQuery.SQL)

	// Execute the CREATE TABLE
	if _, err := driver.Exec(ctx, createQuery.SQL); err != nil {
		log.Fatal("Failed to create table:", err)
	}

	// Test INSERT
	insertData := map[string]interface{}{
		"customer_id": "CUST001",
		"name":        "Example Company",
		"valid_id":    1,
		"create_by":   1,
		"change_by":   1,
	}

	insertQuery, err := driver.Insert("customer_company", insertData)
	if err != nil {
		log.Fatal("Failed to generate INSERT:", err)
	}

	fmt.Printf("Insert SQL: %s\nArgs: %v\n\n", insertQuery.SQL, insertQuery.Args)

	// Check if driver supports RETURNING
	if driver.SupportsReturning() {
		fmt.Println("Driver supports RETURNING clause")
	} else {
		fmt.Println("Driver does not support RETURNING clause")
	}

	// Test with different drivers
	testDrivers := []string{"postgres", "mysql"}
	for _, driverName := range testDrivers {
		fmt.Printf("\n--- Testing %s driver SQL generation ---\n", driverName)

		testDriver, err := database.GetDriver(driverName)
		if err != nil {
			fmt.Printf("Driver %s not available: %v\n", driverName, err)
			continue
		}

		// Generate CREATE TABLE for this driver
		createQuery, err := testDriver.CreateTable(tableSchema)
		if err != nil {
			fmt.Printf("Failed to generate SQL for %s: %v\n", driverName, err)
			continue
		}

		fmt.Printf("%s CREATE TABLE:\n%s\n", driverName, createQuery.SQL)

		// Test type mapping
		fmt.Printf("\nType mappings for %s:\n", driverName)
		types := []string{"serial", "varchar(200)", "text", "timestamp", "boolean"}
		for _, t := range types {
			mapped := testDriver.MapType(t)
			fmt.Printf("  %s -> %s\n", t, mapped)
		}
	}

	fmt.Println("\n✅ Database abstraction layer test complete!")
}
