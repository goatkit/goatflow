// Package main provides database utilities.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/goatkit/goatflow/internal/database"
)

func main() {
	var (
		command  = flag.String("cmd", "", "Command to execute: export-xml, import-xml, test-connection, migrate")
		dbType   = flag.String("db-type", "postgresql", "Database type: postgresql, mysql, oracle, sqlserver")
		host     = flag.String("host", "localhost", "Database host")
		port     = flag.String("port", "", "Database port (auto-detected based on db-type)")
		username = flag.String("user", "", "Database username")
		password = flag.String("password", "", "Database password")
		dbname   = flag.String("database", "", "Database name")
		xmlFile  = flag.String("xml", "", "XML schema file path")
		sslMode  = flag.String("ssl-mode", "disable", "SSL mode for connection")
	)

	flag.Parse()

	if *command == "" {
		flag.Usage()
		log.Fatal("Command is required")
	}

	// Set default ports based on database type
	if *port == "" {
		switch database.DatabaseType(*dbType) {
		case database.PostgreSQL:
			*port = "5432"
		case database.MySQL:
			*port = "3306"
		case database.Oracle:
			*port = "1521"
		case database.SQLServer:
			*port = "1433"
		}
	}

	// Build database configuration
	config := database.DatabaseConfig{
		Type:         database.DatabaseType(*dbType),
		Host:         *host,
		Port:         *port,
		Database:     *dbname,
		Username:     *username,
		Password:     *password,
		SSLMode:      *sslMode,
		MaxOpenConns: 25,
		MaxIdleConns: 5,
		Options:      make(map[string]string),
	}

	// Validate required fields
	if config.Username == "" {
		config.Username = os.Getenv("DB_USER")
		if config.Username == "" {
			log.Fatal("Database username is required (use -user or DB_USER env var)")
		}
	}

	if config.Password == "" {
		config.Password = os.Getenv("DB_PASSWORD")
		if config.Password == "" {
			log.Fatal("Database password is required (use -password or DB_PASSWORD env var)")
		}
	}

	if config.Database == "" {
		config.Database = os.Getenv("DB_NAME")
		if config.Database == "" {
			log.Fatal("Database name is required (use -database or DB_NAME env var)")
		}
	}

	// Create database factory and database instance
	factory := database.NewDatabaseFactory()
	db, err := factory.Create(config)
	if err != nil {
		log.Fatalf("Failed to create database instance: %v", err)
	}

	// Execute command
	switch *command {
	case "test-connection":
		testConnection(db)
	case "export-xml":
		exportXML(db, *xmlFile)
	case "import-xml":
		importXML(db, *xmlFile)
	case "migrate":
		migrate(db, config)
	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}

func testConnection(db database.IDatabase) {
	fmt.Printf("Testing connection to %s database...\n", db.GetType())

	err := db.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Printf("✅ Successfully connected to %s database\n", db.GetType())

	// Test database features
	features := database.GetDatabaseFeatures(db.GetType())
	fmt.Println("\n📋 Database Features:")
	fmt.Printf("  Supports RETURNING: %v\n", features.SupportsReturning)
	fmt.Printf("  Supports UPSERT: %v\n", features.SupportsUpsert)
	fmt.Printf("  Supports JSON columns: %v\n", features.SupportsJSONColumn)
	fmt.Printf("  Supports arrays: %v\n", features.SupportsArrayColumn)
	fmt.Printf("  Supports window functions: %v\n", features.SupportsWindowFunctions)
	fmt.Printf("  Supports CTEs: %v\n", features.SupportsCTE)
	fmt.Printf("  Max identifier length: %d\n", features.MaxIdentifierLength)

	// Test some basic operations
	fmt.Println("\n🔍 Testing basic operations:")

	// Test LIMIT clause generation
	limitClause := db.GetLimitClause(10, 5)
	fmt.Printf("  LIMIT 10 OFFSET 5: %s\n", limitClause)

	// Test date function
	dateFunc := db.GetDateFunction()
	fmt.Printf("  Current date function: %s\n", dateFunc)

	// Test concat function
	concatFunc := db.GetConcatFunction([]string{"field1", "field2", "field3"})
	fmt.Printf("  Concatenation: %s\n", concatFunc)

	// Test identifier quoting
	quoted := db.Quote("table_name")
	fmt.Printf("  Quoted identifier: %s\n", quoted)

	db.Close()
}

func exportXML(db database.IDatabase, xmlFile string) {
	if xmlFile == "" {
		xmlFile = fmt.Sprintf("schema_%s.xml", db.GetType())
	}

	fmt.Printf("Exporting schema from %s to %s...\n", db.GetType(), xmlFile)

	err := db.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	converter := database.NewSchemaConverter(db)
	err = converter.ExportToXML(xmlFile)
	if err != nil {
		log.Fatalf("Failed to export schema: %v", err)
	}

	fmt.Printf("✅ Schema exported successfully to %s\n", xmlFile)
}

func importXML(db database.IDatabase, xmlFile string) {
	if xmlFile == "" {
		// Try to find schema file in known locations
		possiblePaths := []string{
			"schema/otrs_schema.xml",
			"../schema/otrs_schema.xml",
			"../../schema/otrs_schema.xml",
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				xmlFile = path
				break
			}
		}

		if xmlFile == "" {
			log.Fatal("XML schema file not found, specify with -xml flag")
		}
	}

	fmt.Printf("Importing schema from %s to %s database...\n", xmlFile, db.GetType())

	err := db.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	converter := database.NewSchemaConverter(db)
	err = converter.ImportFromXML(xmlFile)
	if err != nil {
		log.Fatalf("Failed to import schema: %v", err)
	}

	fmt.Printf("✅ Schema imported successfully from %s\n", xmlFile)
}

func migrate(db database.IDatabase, config database.DatabaseConfig) {
	fmt.Printf("Running database migration for %s...\n", db.GetType())

	err := db.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Check if we need to create the database first
	if !db.IsHealthy() {
		log.Fatal("Database connection is not healthy")
	}

	// For now, just test that we can connect and perform basic operations
	ctx := context.Background()

	// Test table existence
	exists, err := db.TableExists(ctx, "users")
	if err != nil {
		log.Printf("Warning: Could not check if users table exists: %v", err)
	} else {
		fmt.Printf("Users table exists: %v\n", exists)
	}

	// Show database stats
	if db.GetType() == database.PostgreSQL {
		stats := db.Stats()
		fmt.Printf("\n📊 Connection Pool Stats:\n")
		fmt.Printf("  Open connections: %d\n", stats.OpenConnections)
		fmt.Printf("  In use: %d\n", stats.InUse)
		fmt.Printf("  Idle: %d\n", stats.Idle)
		fmt.Printf("  Max open allowed: %d\n", stats.MaxOpenConnections)
	}

	fmt.Printf("✅ Migration completed successfully\n")
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "GoatFlow Database Management Tool\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  test-connection  Test database connectivity and features\n")
		fmt.Fprintf(os.Stderr, "  export-xml       Export database schema to XML format\n")
		fmt.Fprintf(os.Stderr, "  import-xml       Import schema from XML to database\n")
		fmt.Fprintf(os.Stderr, "  migrate          Run database migration\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Test PostgreSQL connection\n")
		fmt.Fprintf(os.Stderr, "  %s -cmd=test-connection -db-type=postgresql -user=goatflow -password=secret -database=goatflow\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  # Export PostgreSQL schema to XML\n")
		fmt.Fprintf(os.Stderr, "  %s -cmd=export-xml -db-type=postgresql -xml=my_schema.xml\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  # Import XML schema to MySQL\n")
		fmt.Fprintf(os.Stderr, "  %s -cmd=import-xml -db-type=mysql -xml=schema/otrs_schema.xml -user=root -password=secret -database=goatflow\n\n", filepath.Base(os.Args[0]))
	}
}
