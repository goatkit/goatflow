package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/storage"
	_ "github.com/lib/pq"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func main() {
	// Define command-line flags
	var (
		dbHost     = flag.String("db-host", getEnv("DB_HOST", "localhost"), "Database host")
		dbPort     = flag.String("db-port", getEnv("DB_PORT", "5432"), "Database port")
		dbName     = flag.String("db-name", getEnv("DB_NAME", "gotrs"), "Database name")
		dbUser     = flag.String("db-user", getEnv("DB_USER", "gotrs"), "Database user")
		dbPassword = flag.String("db-password", getEnv("DB_PASSWORD", ""), "Database password")
		
		command = flag.String("command", "", "Command to execute: switch, status, verify")
		target  = flag.String("target", "", "Target backend: DB or FS")
		
		fsPath = flag.String("fs-path", getEnv("ARTICLE_STORAGE_FS_PATH", "/opt/gotrs/var/article"), "Filesystem storage path")
		
		tolerant     = flag.Bool("tolerant", false, "Continue on errors")
		dryRun       = flag.Bool("dry-run", false, "Perform dry run without making changes")
		batchSize    = flag.Int("batch-size", 100, "Number of articles to process in each batch")
		sleepMs      = flag.Int("sleep-ms", 10, "Milliseconds to sleep between batches")
		closedBefore = flag.String("closed-before", "", "Only migrate tickets closed before this date (YYYY-MM-DD)")
		createdAfter = flag.String("created-after", "", "Only migrate tickets created after this date (YYYY-MM-DD)")
		
		verbose = flag.Bool("verbose", false, "Enable verbose output")
	)
	
	flag.Parse()
	
	// Validate command
	if *command == "" {
		fmt.Println("Usage: gotrs-storage -command <switch|status|verify> [options]")
		fmt.Println("\nCommands:")
		fmt.Println("  switch  - Switch storage backend (requires -target)")
		fmt.Println("  status  - Show current storage status")
		fmt.Println("  verify  - Verify storage integrity")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(1)
	}
	
	// Connect to database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		*dbHost, *dbPort, *dbUser, *dbPassword, *dbName)
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	
	// Execute command
	ctx := context.Background()
	
	switch *command {
	case "switch":
		if *target == "" {
			log.Fatal("Target backend is required for switch command")
		}
		err = switchBackend(ctx, db, *target, &SwitchOptions{
			FSPath:       *fsPath,
			Tolerant:     *tolerant,
			DryRun:       *dryRun,
			BatchSize:    *batchSize,
			SleepMs:      *sleepMs,
			ClosedBefore: *closedBefore,
			CreatedAfter: *createdAfter,
			Verbose:      *verbose,
		})
		
	case "status":
		err = showStatus(ctx, db, *fsPath, *verbose)
		
	case "verify":
		err = verifyStorage(ctx, db, *fsPath, *verbose)
		
	default:
		log.Fatalf("Unknown command: %s", *command)
	}
	
	if err != nil {
		log.Fatalf("Command failed: %v", err)
	}
}

// SwitchOptions contains options for storage backend switching
type SwitchOptions struct {
	FSPath       string
	Tolerant     bool
	DryRun       bool
	BatchSize    int
	SleepMs      int
	ClosedBefore string
	CreatedAfter string
	Verbose      bool
}

// switchBackend switches the storage backend
func switchBackend(ctx context.Context, db *sql.DB, targetBackend string, opts *SwitchOptions) error {
	fmt.Printf("Switching storage backend to: %s\n", targetBackend)
	
	if opts.DryRun {
		fmt.Println("DRY RUN MODE - No changes will be made")
	}
	
	// Determine current backend
	currentBackend := "DB" // Default
	if targetBackend == "DB" {
		currentBackend = "FS"
	}
	
	// Create backends
	var sourceBackend, targetBackendObj storage.Backend
	var err error
	
	if currentBackend == "DB" {
		sourceBackend = storage.NewDatabaseBackend(db)
		targetBackendObj, err = storage.NewFilesystemBackend(opts.FSPath, db)
	} else {
		sourceBackend, err = storage.NewFilesystemBackend(opts.FSPath, db)
		if err == nil {
			targetBackendObj = storage.NewDatabaseBackend(db)
		}
	}
	
	if err != nil {
		return fmt.Errorf("failed to create backends: %w", err)
	}
	
	// Start migration tracking
	migrationID, err := startMigration(ctx, db, currentBackend, targetBackend)
	if err != nil {
		return fmt.Errorf("failed to start migration: %w", err)
	}
	
	// Get articles to migrate
	query := `
		SELECT DISTINCT a.id, a.ticket_id 
		FROM article a
		JOIN ticket t ON t.id = a.ticket_id
		WHERE 1=1`
	
	args := []interface{}{}
	argNum := 1
	
	if opts.ClosedBefore != "" {
		query += fmt.Sprintf(" AND t.change_time < $%d AND t.ticket_state_id IN (5, 6)", argNum)
		args = append(args, opts.ClosedBefore)
		argNum++
	}
	
	if opts.CreatedAfter != "" {
		query += fmt.Sprintf(" AND t.create_time > $%d", argNum)
		args = append(args, opts.CreatedAfter)
		argNum++
	}
	
	query += " ORDER BY a.id"
	
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to query articles: %w", err)
	}
	defer rows.Close()
	
	// Process articles in batches
	var totalArticles, processedArticles, failedArticles int
	batch := make([]int64, 0, opts.BatchSize)
	
	for rows.Next() {
		var articleID, ticketID int64
		if err := rows.Scan(&articleID, &ticketID); err != nil {
			continue
		}
		
		totalArticles++
		batch = append(batch, articleID)
		
		if len(batch) >= opts.BatchSize {
			processed, failed := processBatch(ctx, sourceBackend, targetBackendObj, batch, opts)
			processedArticles += processed
			failedArticles += failed
			
			// Update migration status
			updateMigration(ctx, db, migrationID, totalArticles, processedArticles, failedArticles, articleID)
			
			// Sleep between batches
			if opts.SleepMs > 0 {
				time.Sleep(time.Duration(opts.SleepMs) * time.Millisecond)
			}
			
			batch = batch[:0]
		}
	}
	
	// Process remaining articles
	if len(batch) > 0 {
		processed, failed := processBatch(ctx, sourceBackend, targetBackendObj, batch, opts)
		processedArticles += processed
		failedArticles += failed
	}
	
	// Complete migration
	completeMigration(ctx, db, migrationID, totalArticles, processedArticles, failedArticles)
	
	fmt.Printf("\nMigration complete!\n")
	fmt.Printf("Total articles: %d\n", totalArticles)
	fmt.Printf("Processed: %d\n", processedArticles)
	fmt.Printf("Failed: %d\n", failedArticles)
	
	if !opts.DryRun && failedArticles == 0 {
		fmt.Printf("\nYou can now update your configuration to use the %s backend.\n", targetBackend)
		fmt.Printf("Set ARTICLE_STORAGE_BACKEND=%s in your environment.\n", targetBackend)
	}
	
	return nil
}

// processBatch processes a batch of articles
func processBatch(ctx context.Context, source, target storage.Backend, articleIDs []int64, opts *SwitchOptions) (processed, failed int) {
	for _, articleID := range articleIDs {
		if opts.Verbose {
			fmt.Printf("Processing article %d...", articleID)
		}
		
		if opts.DryRun {
			processed++
			if opts.Verbose {
				fmt.Println(" [DRY RUN]")
			}
			continue
		}
		
		// Get all storage references for this article
		refs, err := source.List(ctx, articleID)
		if err != nil {
			if opts.Verbose {
				fmt.Printf(" ERROR: %v\n", err)
			}
			if !opts.Tolerant {
				return processed, failed + 1
			}
			failed++
			continue
		}
		
		// Migrate each reference
		migrated := 0
		for _, ref := range refs {
			_, err := source.Migrate(ctx, ref, target)
			if err != nil {
				if opts.Verbose {
					fmt.Printf(" ERROR migrating %s: %v\n", ref.FileName, err)
				}
				if !opts.Tolerant {
					return processed, failed + 1
				}
			} else {
				migrated++
			}
		}
		
		if migrated == len(refs) {
			processed++
			if opts.Verbose {
				fmt.Printf(" OK (%d files)\n", migrated)
			}
		} else {
			failed++
			if opts.Verbose {
				fmt.Printf(" PARTIAL (%d/%d files)\n", migrated, len(refs))
			}
		}
	}
	
	return processed, failed
}

// showStatus shows current storage status
func showStatus(ctx context.Context, db *sql.DB, fsPath string, verbose bool) error {
	fmt.Println("Storage Status")
	fmt.Println("==============")
	
	// Check database storage
	var dbArticles, dbAttachments int
	var dbSize int64
	
	db.QueryRow("SELECT COUNT(*) FROM article_data_mime").Scan(&dbArticles)
	db.QueryRow("SELECT COUNT(*) FROM article_data_mime_attachment").Scan(&dbAttachments)
	db.QueryRow("SELECT COALESCE(SUM(octet_length(a_body)), 0) FROM article_data_mime").Scan(&dbSize)
	
	var attachmentSize int64
	db.QueryRow("SELECT COALESCE(SUM(octet_length(content)), 0) FROM article_data_mime_attachment").Scan(&attachmentSize)
	dbSize += attachmentSize
	
	fmt.Printf("\nDatabase Storage (ArticleStorageDB):\n")
	fmt.Printf("  Articles: %d\n", dbArticles)
	fmt.Printf("  Attachments: %d\n", dbAttachments)
	fmt.Printf("  Total Size: %s\n", formatBytes(dbSize))
	
	// Check filesystem storage
	var fsReferences int
	db.QueryRow("SELECT COUNT(*) FROM article_storage_references WHERE backend = 'FS'").Scan(&fsReferences)
	
	fmt.Printf("\nFilesystem Storage (ArticleStorageFS):\n")
	fmt.Printf("  Base Path: %s\n", fsPath)
	fmt.Printf("  References: %d\n", fsReferences)
	
	if fsReferences > 0 && verbose {
		// Show sample files
        rows, _ := db.Query(database.ConvertPlaceholders(`
            SELECT article_id, file_name, file_size, created_time 
            FROM article_storage_references 
            WHERE backend = 'FS' 
            ORDER BY created_time DESC 
            LIMIT 5`))
        defer rows.Close()
		
		fmt.Println("\n  Recent Files:")
		for rows.Next() {
			var articleID int64
			var fileName string
			var fileSize int64
			var createdTime time.Time
			
			rows.Scan(&articleID, &fileName, &fileSize, &createdTime)
			fmt.Printf("    Article %d: %s (%s) - %s\n", 
				articleID, fileName, formatBytes(fileSize), createdTime.Format("2006-01-02 15:04"))
		}
	}
	
	// Check for active migrations
	var activeMigrations int
	db.QueryRow("SELECT COUNT(*) FROM article_storage_migration WHERE status IN ('pending', 'in_progress')").Scan(&activeMigrations)
	
	if activeMigrations > 0 {
		fmt.Printf("\nActive Migrations: %d\n", activeMigrations)
		
        rows, _ := db.Query(database.ConvertPlaceholders(`
            SELECT id, source_backend, target_backend, status, 
                   processed_articles, total_articles, start_time
            FROM article_storage_migration 
            WHERE status IN ('pending', 'in_progress')
            ORDER BY id DESC`))
        defer rows.Close()
		
		for rows.Next() {
			var id int
			var source, target, status string
			var processed, total int
			var startTime sql.NullTime
			
			rows.Scan(&id, &source, &target, &status, &processed, &total, &startTime)
			
			fmt.Printf("\n  Migration #%d: %s -> %s\n", id, source, target)
			fmt.Printf("    Status: %s\n", status)
			fmt.Printf("    Progress: %d/%d articles\n", processed, total)
			if startTime.Valid {
				fmt.Printf("    Started: %s\n", startTime.Time.Format("2006-01-02 15:04"))
			}
		}
	}
	
	return nil
}

// verifyStorage verifies storage integrity
func verifyStorage(ctx context.Context, db *sql.DB, fsPath string, verbose bool) error {
	fmt.Println("Verifying Storage Integrity")
	fmt.Println("===========================")
	
	// Create backends
	dbBackend := storage.NewDatabaseBackend(db)
	fsBackend, err := storage.NewFilesystemBackend(fsPath, db)
	if err != nil {
		return fmt.Errorf("failed to create filesystem backend: %w", err)
	}
	
	// Get all articles
	rows, err := db.Query("SELECT id FROM article ORDER BY id")
	if err != nil {
		return fmt.Errorf("failed to query articles: %w", err)
	}
	defer rows.Close()
	
	var totalArticles, dbArticles, fsArticles, bothArticles, missingArticles int
	
	for rows.Next() {
		var articleID int64
		if err := rows.Scan(&articleID); err != nil {
			continue
		}
		
		totalArticles++
		
		// Check database storage
		dbRefs, _ := dbBackend.List(ctx, articleID)
		hasDB := len(dbRefs) > 0
		
		// Check filesystem storage  
		fsRefs, _ := fsBackend.List(ctx, articleID)
		hasFS := len(fsRefs) > 0
		
		if hasDB && hasFS {
			bothArticles++
			if verbose {
				fmt.Printf("Article %d: Found in both backends (DB: %d files, FS: %d files)\n", 
					articleID, len(dbRefs), len(fsRefs))
			}
		} else if hasDB {
			dbArticles++
		} else if hasFS {
			fsArticles++
		} else {
			missingArticles++
			if verbose {
				fmt.Printf("Article %d: WARNING - No storage found!\n", articleID)
			}
		}
	}
	
	fmt.Printf("\nTotal Articles: %d\n", totalArticles)
	fmt.Printf("  Database only: %d\n", dbArticles)
	fmt.Printf("  Filesystem only: %d\n", fsArticles)
	fmt.Printf("  Both backends: %d\n", bothArticles)
	fmt.Printf("  Missing: %d\n", missingArticles)
	
	if missingArticles > 0 {
		fmt.Println("\nWARNING: Some articles have no storage!")
		fmt.Println("This may indicate data loss or corruption.")
	}
	
	return nil
}

// Helper functions

func startMigration(ctx context.Context, db *sql.DB, source, target string) (int, error) {
	var id int
	err := db.QueryRowContext(ctx, `
		INSERT INTO article_storage_migration (
			source_backend, target_backend, status, start_time
		) VALUES ($1, $2, 'in_progress', NOW())
		RETURNING id`, source, target).Scan(&id)
	return id, err
}

func updateMigration(ctx context.Context, db *sql.DB, id, total, processed, failed int, lastArticleID int64) {
	db.ExecContext(ctx, `
		UPDATE article_storage_migration 
		SET total_articles = $2,
		    processed_articles = $3,
		    failed_articles = $4,
		    last_article_id = $5,
		    updated_at = NOW()
		WHERE id = $1`, id, total, processed, failed, lastArticleID)
}

func completeMigration(ctx context.Context, db *sql.DB, id, total, processed, failed int) {
	status := "completed"
	if failed > 0 {
		status = "completed_with_errors"
	}
	
	db.ExecContext(ctx, `
		UPDATE article_storage_migration 
		SET status = $2,
		    total_articles = $3,
		    processed_articles = $4,
		    failed_articles = $5,
		    end_time = NOW(),
		    updated_at = NOW()
		WHERE id = $1`, id, status, total, processed, failed)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}