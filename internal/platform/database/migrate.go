package database

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Every goats process (backend replicas, runner, customer frontend) runs
// RunMigrations at startup. The dirty check, the force and the `migrate up`
// run under one database lock so a process never sees another process's
// in-progress (dirty) migration and "repairs" it with a force.
// The lock differs from golang-migrate's own lock (keyed by the database
// name), which the migrate subprocess takes while we hold ours.
const (
	migrationLockName = "goatflow_schema_migrations" // MySQL/MariaDB GET_LOCK name
	migrationLockKey  = int64(0x676f6174666c6f77)    // PostgreSQL advisory key ("goatflow")
	// migrationLockWait bounds how long a process waits for another one to
	// finish migrating before it gives up.
	migrationLockWait = 15 * time.Minute
)

// lockMigrations takes the cross-process migration lock on a dedicated
// connection (both lock kinds are session-scoped) and returns its release.
func lockMigrations(ctx context.Context, db *sql.DB, driver string, wait time.Duration) (func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration lock: %w", err)
	}
	isMySQL := driver == "mysql" || driver == "mariadb"
	deadline := time.Now().Add(wait)
	logged := false
	for {
		var got bool
		if isMySQL {
			// GET_LOCK waits up to 2 seconds server-side: 1 = acquired,
			// 0 = held by another session.
			var res sql.NullInt64
			err = conn.QueryRowContext(ctx, ConvertPlaceholders("SELECT GET_LOCK(?, 2)"), migrationLockName).Scan(&res)
			got = res.Valid && res.Int64 == 1
		} else {
			err = conn.QueryRowContext(ctx, ConvertPlaceholders("SELECT pg_try_advisory_lock(?)"), migrationLockKey).Scan(&got)
		}
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("migration lock: %w", err)
		}
		if got {
			break
		}
		if time.Now().After(deadline) {
			_ = conn.Close()
			return nil, fmt.Errorf("migration lock: another process is still migrating after %s", wait)
		}
		if !logged {
			log.Printf("migrations: another process is migrating; waiting for it to finish")
			logged = true
		}
		if !isMySQL {
			select {
			case <-ctx.Done():
				_ = conn.Close()
				return nil, fmt.Errorf("migration lock: %w", ctx.Err())
			case <-time.After(time.Second):
			}
		}
	}
	return func() {
		var err error
		if isMySQL {
			_, err = conn.ExecContext(context.Background(), ConvertPlaceholders("SELECT RELEASE_LOCK(?)"), migrationLockName)
		} else {
			_, err = conn.ExecContext(context.Background(), ConvertPlaceholders("SELECT pg_advisory_unlock(?)"), migrationLockKey)
		}
		if err != nil {
			log.Printf("migrations: releasing migration lock: %v", err)
		}
		// Closing the session releases the lock in any case.
		_ = conn.Close()
	}, nil
}

// RunMigrations runs database migrations using the migrate CLI tool.
// Returns the number of migrations applied and any error encountered.
func RunMigrations(db *sql.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("database connection is nil")
	}

	// Find migrate binary
	migrateBin := findMigrateBinary()
	if migrateBin == "" {
		return 0, fmt.Errorf("migrate binary not found")
	}

	// Determine migrations path
	migrationsPath := getMigrationsPath()
	if migrationsPath == "" {
		return 0, fmt.Errorf("migrations directory not found")
	}

	// Verify directory exists
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("migrations directory does not exist: %s", migrationsPath)
	}

	driver := GetDBDriver()
	log.Printf("migrations: using driver %s, path %s", driver, migrationsPath)

	// Build database URL
	dbURL, err := buildDatabaseURL(driver)
	if err != nil {
		return 0, fmt.Errorf("failed to build database URL: %w", err)
	}

	// Hold the migration lock from the version check to the end of
	// `migrate up`, so the state we read cannot be another process's
	// in-progress migration.
	release, err := lockMigrations(context.Background(), db, driver, migrationLockWait)
	if err != nil {
		return 0, err
	}
	defer release()

	// Get current version before migration
	versionBefore, dirty, err := getMigrationVersion(db)
	if err != nil {
		log.Printf("migrations: could not get current version: %v", err)
		versionBefore = 0
	}

	// Handle dirty state: with the lock held no GoatFlow process is
	// migrating, so a dirty flag is left over from a migration that died.
	if dirty {
		log.Printf("migrations: WARNING - database is in dirty state at version %d, attempting to fix", versionBefore)
		cmd := exec.Command(migrateBin, "-path", migrationsPath, "-database", dbURL, "force", strconv.Itoa(versionBefore)) // #nosec G204 -- binary from findMigrateBinary, args from server DB config; no shell, no request input
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return 0, fmt.Errorf("failed to fix dirty state: %s", stderr.String())
		}
		log.Printf("migrations: cleared dirty state at version %d", versionBefore)
	}

	// Run migrations
	cmd := exec.Command(migrateBin, "-path", migrationsPath, "-database", dbURL, "up") // #nosec G204 -- binary from findMigrateBinary, args from server DB config; no shell, no request input
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output := stdout.String() + stderr.String()

	// Check for "no change" which is not an error
	if strings.Contains(output, "no change") {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("migration failed: %s", output)
	}

	// Get version after migration
	versionAfter, _, err := getMigrationVersion(db)
	if err != nil {
		log.Printf("migrations: could not get version after migration: %v", err)
		return 0, nil
	}

	migrationsApplied := versionAfter - versionBefore
	return migrationsApplied, nil
}

// findMigrateBinary locates the migrate CLI binary.
func findMigrateBinary() string {
	candidates := []string{
		"./migrate",
		"/app/migrate",
		"/usr/local/bin/migrate",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try PATH
	if path, err := exec.LookPath("migrate"); err == nil {
		return path
	}

	return ""
}

// buildDatabaseURL builds the database connection URL for migrate.
// Uses namespaced per-driver vars (DB_MYSQL_* / DB_PGSQL_*) selected by DB_DRIVER.
func buildDatabaseURL(driver string) (string, error) {
	dbHost := firstNonEmpty(Env("HOST"), "mariadb")
	dbPort := firstNonEmpty(Env("PORT"), "3306")
	dbUser := firstNonEmpty(Env("USER"), "otrs")
	dbPass := Env("PASSWORD")
	dbName := firstNonEmpty(Env("NAME"), "otrs")

	switch driver {
	case "mysql", "mariadb":
		// mysql://user:password@tcp(host:port)/database?multiStatements=true
		return fmt.Sprintf("mysql://%s:%s@tcp(%s:%s)/%s?multiStatements=true",
			dbUser, dbPass, dbHost, dbPort, dbName), nil

	case "postgres":
		// postgres://user:password@host:port/database?sslmode=disable
		sslMode := firstNonEmpty(Env("SSLMODE"), "disable")
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			dbUser, dbPass, dbHost, dbPort, dbName, sslMode), nil

	default:
		return "", fmt.Errorf("unsupported database driver: %s", driver)
	}
}

// getMigrationVersion queries the schema_migrations table for current version.
func getMigrationVersion(db *sql.DB) (int, bool, error) {
	var version int
	var dirty bool

	query := ConvertPlaceholders("SELECT version, dirty FROM schema_migrations LIMIT 1")
	err := db.QueryRow(query).Scan(&version, &dirty)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}

	return version, dirty, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// getMigrationsPath returns the path to migrations based on the current driver.
func getMigrationsPath() string {
	driver := GetDBDriver()

	// Map driver to subdirectory
	var subdir string
	switch driver {
	case "mysql", "mariadb":
		subdir = "mysql"
	case "postgres":
		subdir = "postgres"
	default:
		subdir = "mysql" // Default fallback
	}

	// Check common locations
	candidates := []string{
		filepath.Join("migrations", subdir),
		filepath.Join("/app/migrations", subdir),
		filepath.Join(".", "migrations", subdir),
	}

	// Also check MIGRATIONS_PATH env var
	if envPath := os.Getenv("MIGRATIONS_PATH"); envPath != "" {
		// If env path includes driver subdir, use as-is; otherwise append
		if strings.HasSuffix(envPath, subdir) || strings.HasSuffix(envPath, subdir+"/") {
			candidates = append([]string{envPath}, candidates...)
		} else {
			candidates = append([]string{filepath.Join(envPath, subdir)}, candidates...)
		}
	}

	for _, path := range candidates {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && info.IsDir() { // #nosec G703 -- fixed candidate dirs or operator-set MIGRATIONS_PATH env var
			// Check if directory has any .sql files
			entries, err := os.ReadDir(absPath)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".sql") {
					return absPath
				}
			}
		}
	}

	return ""
}
