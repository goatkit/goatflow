package api

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestMain(m *testing.M) {
	ensureTestEnvironment()

	if err := waitForTestDatabase(45 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "test database unavailable: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	database.CloseTestDB()
	os.Exit(code)
}

func init() {
	ensureTestEnvironment()
}

func ensureTestEnvironment() {
	setDefaultEnv("APP_ENV", "test")
	setDefaultEnv("DB_DRIVER", "postgres")
	setDefaultEnv("DB_HOST", "postgres-test")
	setDefaultEnv("DB_PORT", "5432")
	setDefaultEnv("DB_NAME", "gotrs_test")
	setDefaultEnv("DB_USER", "gotrs_user")
	setDefaultEnv("DB_PASSWORD", "gotrs_password")
}

func waitForTestDatabase(timeout time.Duration) error {
	driver := strings.ToLower(os.Getenv("DB_DRIVER"))

	var waitErr error
	switch driver {
	case "", "postgres", "postgresql":
		waitErr = waitForPostgresDatabase(timeout)
	case "mysql", "mariadb":
		waitErr = waitForMySQLDatabase(timeout)
	default:
		waitErr = waitForPostgresDatabase(timeout)
	}

	if waitErr != nil {
		return waitErr
	}

	database.ResetDB()
	return database.InitTestDB()
}

func waitForPostgresDatabase(timeout time.Duration) error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = os.Getenv("DB_SSL_MODE")
	}
	if sslMode == "" {
		sslMode = "disable"
	}

	if host == "" || port == "" {
		return fmt.Errorf("test database host/port not configured")
	}

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		currentHost := host
		if resolved := resolveHost(host); resolved != "" && resolved != host {
			currentHost = resolved
			_ = os.Setenv("DB_HOST", currentHost)
		}

		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", currentHost, port, user, password, dbName, sslMode)

		sqlDB, err := sql.Open("postgres", dsn)
		if err == nil {
			err = sqlDB.Ping()
			sqlDB.Close()
			if err == nil {
				return nil
			}
			lastErr = err
		} else {
			lastErr = err
		}

		time.Sleep(500 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timeout waiting for database")
	}
	return lastErr
}

func waitForMySQLDatabase(timeout time.Duration) error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if host == "" || port == "" {
		return fmt.Errorf("test database host/port not configured")
	}

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		currentHost := host
		if resolved := resolveHost(host); resolved != "" && resolved != host {
			currentHost = resolved
			_ = os.Setenv("DB_HOST", currentHost)
		}

		dsn := buildMySQLDSN(user, password, currentHost, port, dbName)
		sqlDB, err := sql.Open("mysql", dsn)
		if err == nil {
			err = sqlDB.Ping()
			sqlDB.Close()
			if err == nil {
				return nil
			}
			lastErr = err
		} else {
			lastErr = err
		}

		time.Sleep(500 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timeout waiting for database")
	}
	return lastErr
}

func buildMySQLDSN(user, password, host, port, dbName string) string {
	params := []string{"parseTime=true", "multiStatements=true", "timeout=2s"}
	paramStr := strings.Join(params, "&")

	escapedUser := url.QueryEscape(user)
	escapedPassword := url.QueryEscape(password)
	escapedDBName := url.QueryEscape(dbName)

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", escapedUser, escapedPassword, host, port, escapedDBName, paramStr)
}

func resolveHost(host string) string {
	if host == "" {
		return host
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resolver := &net.Resolver{PreferGo: true}
	if ips, err := resolver.LookupHost(ctx, host); err == nil && len(ips) > 0 {
		return ips[0]
	}

	output, err := exec.Command("getent", "hosts", host).Output()
	if err == nil {
		fields := strings.Fields(string(output))
		if len(fields) > 0 {
			return fields[0]
		}
	}

	switch strings.ToLower(host) {
	case "mariadb", "mysql", "postgres", "postgres-test", "valkey", "redis", "mailhog":
		return "127.0.0.1"
	case "host.docker.internal", "host.containers.internal":
		return "127.0.0.1"
	case "localhost":
		return "127.0.0.1"
	}

	return host
}

func setDefaultEnv(key, value string) {
	if os.Getenv(key) != "" {
		return
	}
	_ = os.Setenv(key, value)
}
