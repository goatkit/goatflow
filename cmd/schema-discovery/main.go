// Package main provides schema discovery utilities.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
	"gopkg.in/yaml.v2"

	"github.com/gotrs-io/gotrs-ce/internal/components/dynamic"
)

func main() {
	var (
		dbHost     = flag.String("host", "localhost", "Database host")
		dbPort     = flag.Int("port", 5432, "Database port")
		dbUser     = flag.String("user", "gotrs_user", "Database user")
		dbPassword = flag.String("password", os.Getenv("DB_PASSWORD"), "Database password")
		dbName     = flag.String("database", "gotrs", "Database name")
		outputDir  = flag.String("output", "modules/generated", "Output directory for YAML files")
		tableName  = flag.String("table", "", "Specific table to generate (empty for all)")
		verbose    = flag.Bool("verbose", false, "Verbose output")
	)
	flag.Parse()

	// Build connection string
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		*dbHost, *dbPort, *dbUser, *dbPassword, *dbName)

	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	if *verbose {
		log.Printf("Connected to database %s@%s:%d/%s", *dbUser, *dbHost, *dbPort, *dbName)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Create discovery instance
	discovery := dynamic.NewSchemaDiscovery(db)

	// Get tables to process
	var tables []dynamic.TableInfo
	if *tableName != "" {
		// Single table specified
		tables = []dynamic.TableInfo{{Name: *tableName}}
	} else {
		// Discover all tables
		tables, err = discovery.GetTables()
		if err != nil {
			log.Fatalf("Failed to get tables: %v", err)
		}
	}

	log.Printf("Processing %d tables...", len(tables))

	// Process each table
	successCount := 0
	failCount := 0
	skippedCount := 0

	for _, table := range tables {
		// Skip system tables and views
		if shouldSkipTable(table.Name) {
			if *verbose {
				log.Printf("Skipping system table: %s", table.Name)
			}
			skippedCount++
			continue
		}

		log.Printf("Processing table: %s", table.Name)

		// Generate module configuration
		module, err := discovery.GenerateModuleConfig(table.Name)
		if err != nil {
			log.Printf("  ❌ Failed to generate module for %s: %v", table.Name, err)
			failCount++
			continue
		}

		// Generate YAML
		yamlContent, err := yaml.Marshal(module)
		if err != nil {
			log.Printf("  ❌ Failed to generate YAML for %s: %v", table.Name, err)
			failCount++
			continue
		}

		// Write to file
		filename := filepath.Join(*outputDir, fmt.Sprintf("%s.yaml", table.Name))
		if err := os.WriteFile(filename, yamlContent, 0644); err != nil {
			log.Printf("  ❌ Failed to write file for %s: %v", table.Name, err)
			failCount++
			continue
		}

		log.Printf("  ✅ Generated: %s", filename)
		successCount++
	}

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("Schema Discovery Complete!\n")
	fmt.Printf("  Success: %d modules generated\n", successCount)
	fmt.Printf("  Failed:  %d modules failed\n", failCount)
	fmt.Printf("  Skipped: %d system tables\n", skippedCount)
	fmt.Printf("  Output:  %s/\n", *outputDir)
	fmt.Println(strings.Repeat("=", 50))

	if failCount > 0 {
		os.Exit(1)
	}
}

// shouldSkipTable determines if a table should be skipped.
func shouldSkipTable(tableName string) bool {
	// Skip PostgreSQL system tables
	if strings.HasPrefix(tableName, "pg_") ||
		strings.HasPrefix(tableName, "sql_") ||
		tableName == "information_schema" ||
		tableName == "schema_migrations" {
		return true
	}
	return false
}
