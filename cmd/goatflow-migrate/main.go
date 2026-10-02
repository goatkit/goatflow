package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func main() {
	var (
		command   = flag.String("cmd", "", "Command: analyze, import, validate")
		sourceDSN = flag.String("source", "", "OTRS database to import from: postgres://… or a MySQL DSN (user:pass@tcp(host:3306)/otrs)")
		sqlFile   = flag.String("sql", "", "OTRS mysqldump / mariadb-dump file to import from (instead of -source)")
		dbURL     = flag.String("db", "", "GoatFlow database: postgres://… or a MySQL DSN (user:pass@tcp(host:3306)/goatflow)")
		verbose   = flag.Bool("v", false, "Verbose output")
		dryRun    = flag.Bool("dry-run", false, "Print the import plan and source row counts without writing")
		force     = flag.Bool("force", false, "Delete the target's tickets, articles and customers before importing (DESTRUCTIVE!)")
	)

	flag.Usage = func() {
		name := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "GoatFlow Migration Tool - Import an OTRS 6 / Znuny 6 database\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", name)
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  analyze   List the source's tables, row counts and what the import does with each\n")
		fmt.Fprintf(os.Stderr, "  import    Import the OTRS data into a GoatFlow database\n")
		fmt.Fprintf(os.Stderr, "  validate  Check the imported data\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Import from the OTRS database (MySQL/MariaDB or PostgreSQL), plan only\n")
		fmt.Fprintf(os.Stderr, "  %s -cmd=import -source='otrs:pw@tcp(otrs-db:3306)/otrs' -db=postgres://user:pass@localhost/goatflow -dry-run\n\n", name)
		fmt.Fprintf(os.Stderr, "  # Import from an OTRS on PostgreSQL\n")
		fmt.Fprintf(os.Stderr, "  %s -cmd=import -source=postgres://otrs:pw@otrs-db/otrs -db='user:pass@tcp(localhost:3306)/goatflow'\n\n", name)
		fmt.Fprintf(os.Stderr, "  # Import from a mysqldump file\n")
		fmt.Fprintf(os.Stderr, "  %s -cmd=import -sql=DatabaseBackup.sql -db=postgres://user:pass@localhost/goatflow\n\n", name)
	}

	flag.Parse()

	if *command == "" {
		flag.Usage()
		log.Fatal("Command is required")
	}
	if *dbURL == "" {
		*dbURL = os.Getenv("DATABASE_URL")
	}

	switch *command {
	case "analyze":
		src, err := openSource(*sourceDSN, *sqlFile)
		if err != nil {
			log.Fatalf("Analysis failed: %v", err)
		}
		defer src.Close()
		if err := analyzeSource(src); err != nil {
			log.Fatalf("Analysis failed: %v", err)
		}
	case "import":
		if *dbURL == "" {
			log.Fatal("Database URL is required (use -db or DATABASE_URL env var)")
		}
		src, err := openSource(*sourceDSN, *sqlFile)
		if err != nil {
			log.Fatalf("Import failed: %v", err)
		}
		defer src.Close()
		if err := runImport(src, *dbURL, *verbose, *dryRun, *force); err != nil {
			log.Fatalf("Import failed: %v", err)
		}
	case "validate":
		if *dbURL == "" {
			log.Fatal("Database URL is required for validation")
		}
		if err := validateImportedData(*dbURL); err != nil {
			log.Fatalf("Validation failed: %v", err)
		}
	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}

// openSource opens the OTRS database (-source) or mysqldump file (-sql).
func openSource(dsn, sqlFile string) (source, error) {
	switch {
	case dsn != "" && sqlFile != "":
		return nil, fmt.Errorf("use either -source or -sql, not both")
	case dsn != "":
		return openDBSource(dsn)
	case sqlFile != "":
		return openMySQLDump(sqlFile)
	default:
		return nil, fmt.Errorf("an OTRS source is required: -source=<database DSN> or -sql=<mysqldump file>")
	}
}

// openDB opens the GoatFlow database at dbURL with the driver the database
// package converts SQL for. When neither DB_DRIVER nor TEST_DB_DRIVER is set,
// the URL decides the driver.
func openDB(dbURL string) (*sql.DB, error) {
	driver, dsn := sourceDriver(dbURL)
	if os.Getenv("DB_DRIVER") == "" && os.Getenv("TEST_DB_DRIVER") == "" {
		if err := os.Setenv("DB_DRIVER", driver); err != nil {
			return nil, err
		}
	}
	if (driver == "postgres") != database.IsPostgreSQL() {
		return nil, fmt.Errorf("database URL is for %s but DB_DRIVER selects %s", driver, database.GetDBDriver())
	}
	return sql.Open(driver, dsn)
}

// analyzeSource lists every source table with its row count and the import plan's decision.
func analyzeSource(src source) error {
	fmt.Printf("🔍 Analyzing %s\n\n", src.describe())
	tables := src.tables()
	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Printf("%-34s %8s  %-9s %s\n", "Table", "Rows", "Action", "Notes")
	total := 0
	for _, name := range names {
		n, err := src.count(name)
		if err != nil {
			return fmt.Errorf("count %s: %w", name, err)
		}
		total += n
		action, note := "unknown", "GoatFlow has no such table (add-on?)"
		if p, ok := planFor(name); ok {
			action, note = p.mode.String(), p.reason
		}
		fmt.Printf("%-34s %8d  %-9s %s\n", name, n, action, note)
	}
	fmt.Printf("\n%d tables, %d rows\n", len(names), total)
	return nil
}

func validateImportedData(dbURL string) error {
	fmt.Printf("🔍 Validating imported OTRS data\n")

	db, err := openDB(dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	fmt.Printf("✅ Connected to database\n")

	coreTables := []string{
		"users", "groups", "roles", "group_user", "role_user", "group_role", "queue", "ticket", "article",
		"customer_user", "customer_company",
	}

	fmt.Printf("\n📊 Data Validation:\n")
	ctx := context.Background()
	totalRows := 0

	for _, table := range coreTables {
		var count int
		query := database.ConvertPlaceholders("SELECT COUNT(*) FROM `" + table + "`")
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			fmt.Printf("  %-20s ❌ Error: %v\n", table, err)
			continue
		}
		status := "✅"
		if count == 0 {
			status = "⚠️  Empty"
		}
		fmt.Printf("  %-20s %s %d rows\n", table, status, count)
		totalRows += count
	}

	fmt.Printf("\n📈 Total imported rows: %d\n", totalRows)

	fmt.Printf("\n🔗 Data Integrity Checks:\n")
	checks := []struct{ label, query string }{
		{"Tickets without articles", `SELECT COUNT(*) FROM ticket t WHERE NOT EXISTS (SELECT 1 FROM article a WHERE a.ticket_id = t.id)`},
		{"Customers without company", `SELECT COUNT(*) FROM customer_user cu WHERE cu.customer_id = '' OR cu.customer_id IS NULL`},
		{"Valid agents without group or role", `SELECT COUNT(*) FROM users u WHERE u.valid_id = 1
			AND NOT EXISTS (SELECT 1 FROM group_user gu WHERE gu.user_id = u.id)
			AND NOT EXISTS (SELECT 1 FROM role_user ru WHERE ru.user_id = u.id)`},
	}
	for _, c := range checks {
		var n int
		if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(c.query)).Scan(&n); err != nil {
			fmt.Printf("  %s: ❌ Error checking\n", c.label)
			continue
		}
		status := "✅"
		if n > 0 {
			status = "⚠️ "
		}
		fmt.Printf("  %s: %s %d\n", c.label, status, n)
	}
	fmt.Printf("\n✅ Validation completed\n")
	return nil
}
