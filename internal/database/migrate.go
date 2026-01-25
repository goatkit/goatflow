package database

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

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

	// Get current version before migration
	versionBefore, dirty, err := getMigrationVersion(db)
	if err != nil {
		log.Printf("migrations: could not get current version: %v", err)
		versionBefore = 0
	}

	// Handle dirty state
	if dirty {
		log.Printf("migrations: WARNING - database is in dirty state at version %d, attempting to fix", versionBefore)
		cmd := exec.Command(migrateBin, "-path", migrationsPath, "-database", dbURL, "force", strconv.Itoa(versionBefore))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return 0, fmt.Errorf("failed to fix dirty state: %s", stderr.String())
		}
		log.Printf("migrations: cleared dirty state at version %d", versionBefore)
	}

	// Run migrations
	cmd := exec.Command(migrateBin, "-path", migrationsPath, "-database", dbURL, "up")
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
func buildDatabaseURL(driver string) (string, error) {
	dbHost := firstNonEmpty(os.Getenv("DB_HOST"), "mariadb")
	dbPort := firstNonEmpty(os.Getenv("DB_PORT"), "3306")
	dbUser := firstNonEmpty(os.Getenv("DB_USER"), "otrs")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := firstNonEmpty(os.Getenv("DB_NAME"), "otrs")

	switch driver {
	case "mysql", "mariadb":
		// mysql://user:password@tcp(host:port)/database?multiStatements=true
		return fmt.Sprintf("mysql://%s:%s@tcp(%s:%s)/%s?multiStatements=true",
			dbUser, dbPass, dbHost, dbPort, dbName), nil

	case "postgres", "postgresql":
		// postgres://user:password@host:port/database?sslmode=disable
		sslMode := firstNonEmpty(os.Getenv("DB_SSL_MODE"), "disable")
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

	query := "SELECT version, dirty FROM schema_migrations LIMIT 1"
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
	case "postgres", "postgresql":
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
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
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

// GetMigrationVersion returns the current migration version (public API).
func GetMigrationVersion(db *sql.DB) (uint, bool, error) {
	if db == nil {
		return 0, false, fmt.Errorf("database connection is nil")
	}

	version, dirty, err := getMigrationVersion(db)
	if err != nil {
		return 0, false, err
	}

	return uint(version), dirty, nil
}
