//go:build integration

package filters

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/email/inbound/connector"
	_ "github.com/lib/pq"
)

// getTestDB returns a database connection for testing.
func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	driver := currentDriver()

	host := firstNonEmpty(os.Getenv("TEST_DB_HOST"), os.Getenv("DB_HOST"), defaultHost(driver))
	user := firstNonEmpty(os.Getenv("TEST_DB_USER"), os.Getenv("DB_USER"), defaultUser(driver))
	password := firstNonEmpty(os.Getenv("TEST_DB_PASSWORD"), os.Getenv("DB_PASSWORD"), defaultPassword(driver))
	dbName := firstNonEmpty(os.Getenv("TEST_DB_NAME"), os.Getenv("DB_NAME"), defaultDBName(driver))
	port := firstNonEmpty(os.Getenv("TEST_DB_PORT"), os.Getenv("DB_PORT"), defaultPort(driver))

	var db *sql.DB
	var err error

	switch driver {
	case "postgres", "pgsql", "pg":
		sslMode := firstNonEmpty(os.Getenv("TEST_DB_SSLMODE"), os.Getenv("DB_SSL_MODE"), "disable")
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbName, sslMode)
		db, err = sql.Open("postgres", connStr)
	case "mysql", "mariadb":
		connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC", user, password, host, port, dbName)
		db, err = sql.Open("mysql", connStr)
	default:
		t.Fatalf("unsupported TEST_DB_DRIVER %q", driver)
	}

	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	return db
}

// seedFilters inserts test filters into the database.
// Filter names are prefixed with numbers to control execution order (alphabetical).
func seedFilters(t *testing.T, db *sql.DB) {
	t.Helper()

	// Clean up any existing test filters
	cleanupFilters(t, db)

	// Insert test filters with numeric prefixes to ensure predictable order
	// Filters run in alphabetical order by name

	// 001 - VIP routing runs first (has stop=true)
	insertFilter(t, db, "001-VIPRouting", true, "Match", "From", ".*@vip\\.example\\.com", false)
	insertFilter(t, db, "001-VIPRouting", true, "Set", "X-GoatFlow-Queue", "VIP Support", false)
	insertFilter(t, db, "001-VIPRouting", true, "Set", "X-GoatFlow-PriorityID", "4", false)

	// 002 - Spam filter runs second (won't run if VIP matches due to stop flag)
	insertFilter(t, db, "002-SpamFilter", false, "Match", "Subject", "(?i)buy now|free offer", false)
	insertFilter(t, db, "002-SpamFilter", false, "Set", "X-GoatFlow-Ignore", "1", false)

	// 003 - Not from spammer filter
	insertFilter(t, db, "003-NotFromSpammer", false, "Match", "From", "spammer@bad\\.com", true) // NOT match
	insertFilter(t, db, "003-NotFromSpammer", false, "Set", "X-GoatFlow-Queue", "Clean Queue", false)

	// 004 - Multi-match filter (requires all conditions)
	insertFilter(t, db, "004-MultiMatch", false, "Match", "From", ".*@example\\.com", false)
	insertFilter(t, db, "004-MultiMatch", false, "Match", "Subject", "\\[URGENT\\]", false)
	insertFilter(t, db, "004-MultiMatch", false, "Set", "X-GoatFlow-PriorityID", "5", false)
}

func insertFilter(t *testing.T, db *sql.DB, name string, stop bool, fType, key, value string, not bool) {
	t.Helper()
	query := database.ConvertPlaceholders(`
		INSERT INTO postmaster_filter (f_name, f_stop, f_type, f_key, f_value, f_not)
		VALUES (?, ?, ?, ?, ?, ?)`)

	stopVal := int16(0)
	if stop {
		stopVal = 1
	}
	notVal := int16(0)
	if not {
		notVal = 1
	}

	_, err := db.Exec(query, name, stopVal, fType, key, value, notVal)
	if err != nil {
		t.Fatalf("failed to insert filter: %v", err)
	}
}

func cleanupFilters(t *testing.T, db *sql.DB) {
	t.Helper()
	query := database.ConvertPlaceholders(`
		DELETE FROM postmaster_filter WHERE f_name IN (?, ?, ?, ?)`)
	_, err := db.Exec(query, "001-VIPRouting", "002-SpamFilter", "003-NotFromSpammer", "004-MultiMatch")
	if err != nil {
		t.Fatalf("failed to cleanup filters: %v", err)
	}
}

func TestDBSourceFilter_VIPRouting(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	seedFilters(t, db)
	defer cleanupFilters(t, db)

	filter := NewDBSourceFilter(db, nil)

	// Create a message from VIP domain
	rawEmail := "From: john@vip.example.com\r\nTo: support@company.com\r\nSubject: Help needed\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(rawEmail)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: make(map[string]any),
	}

	err := filter.Apply(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Check that VIP routing was applied
	if got, ok := ctx.Annotations[AnnotationQueueNameOverride].(string); !ok || got != "VIP Support" {
		t.Errorf("expected queue override 'VIP Support', got %v", ctx.Annotations[AnnotationQueueNameOverride])
	}

	if got, ok := ctx.Annotations[AnnotationPriorityIDOverride].(int); !ok || got != 4 {
		t.Errorf("expected priority override 4, got %v", ctx.Annotations[AnnotationPriorityIDOverride])
	}
}

func TestDBSourceFilter_SpamFilter(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	seedFilters(t, db)
	defer cleanupFilters(t, db)

	filter := NewDBSourceFilter(db, nil)

	// Create a spam message
	rawEmail := "From: sender@example.com\r\nTo: support@company.com\r\nSubject: BUY NOW - Free Offer!!!\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(rawEmail)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: make(map[string]any),
	}

	err := filter.Apply(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Check that ignore was set
	if got, ok := ctx.Annotations[AnnotationIgnoreMessage].(bool); !ok || !got {
		t.Errorf("expected ignore message true, got %v", ctx.Annotations[AnnotationIgnoreMessage])
	}
}

func TestDBSourceFilter_NotMatch(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	seedFilters(t, db)
	defer cleanupFilters(t, db)

	filter := NewDBSourceFilter(db, nil)

	// Create a message NOT from spammer (should match the NotFromSpammer filter)
	rawEmail := "From: goodguy@example.com\r\nTo: support@company.com\r\nSubject: Hello\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(rawEmail)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: make(map[string]any),
	}

	err := filter.Apply(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Check that Clean Queue was set (NOT match worked)
	if got, ok := ctx.Annotations[AnnotationQueueNameOverride].(string); !ok || got != "Clean Queue" {
		t.Errorf("expected queue override 'Clean Queue', got %v", ctx.Annotations[AnnotationQueueNameOverride])
	}
}

func TestDBSourceFilter_NotMatchNegative(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	seedFilters(t, db)
	defer cleanupFilters(t, db)

	filter := NewDBSourceFilter(db, nil)

	// Create a message FROM spammer (should NOT match the NotFromSpammer filter)
	rawEmail := "From: spammer@bad.com\r\nTo: support@company.com\r\nSubject: Hello\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(rawEmail)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: make(map[string]any),
	}

	err := filter.Apply(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Check that NotFromSpammer did NOT set queue (spammer should not match NOT rule)
	if got := ctx.Annotations[AnnotationQueueNameOverride]; got == "Clean Queue" {
		t.Errorf("expected no queue override for spammer, but got 'Clean Queue'")
	}
}

func TestDBSourceFilter_MultiMatch(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	seedFilters(t, db)
	defer cleanupFilters(t, db)

	filter := NewDBSourceFilter(db, nil)

	// Create a message that matches BOTH conditions
	rawEmail := "From: user@example.com\r\nTo: support@company.com\r\nSubject: [URGENT] Need help!\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(rawEmail)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: make(map[string]any),
	}

	err := filter.Apply(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Check that high priority was set (multi-match worked)
	if got, ok := ctx.Annotations[AnnotationPriorityIDOverride].(int); !ok || got != 5 {
		t.Errorf("expected priority override 5, got %v", ctx.Annotations[AnnotationPriorityIDOverride])
	}
}

func TestDBSourceFilter_MultiMatchPartial(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	seedFilters(t, db)
	defer cleanupFilters(t, db)

	filter := NewDBSourceFilter(db, nil)

	// Create a message that matches only ONE condition (should not apply filter)
	rawEmail := "From: user@example.com\r\nTo: support@company.com\r\nSubject: Normal subject\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(rawEmail)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: make(map[string]any),
	}

	err := filter.Apply(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Check that high priority was NOT set (partial match should not work)
	if got := ctx.Annotations[AnnotationPriorityIDOverride]; got == 5 {
		t.Errorf("expected no priority override for partial match, but got 5")
	}
}

func TestDBSourceFilter_StopFlag(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	seedFilters(t, db)
	defer cleanupFilters(t, db)

	filter := NewDBSourceFilter(db, nil)

	// Create a VIP message that also looks like spam
	// VIPRouting has stop=1, so SpamFilter should not run
	rawEmail := "From: john@vip.example.com\r\nTo: support@company.com\r\nSubject: BUY NOW\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(rawEmail)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: make(map[string]any),
	}

	err := filter.Apply(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// VIP routing should have applied
	if got, ok := ctx.Annotations[AnnotationQueueNameOverride].(string); !ok || got != "VIP Support" {
		t.Errorf("expected queue override 'VIP Support', got %v", ctx.Annotations[AnnotationQueueNameOverride])
	}

	// Spam filter should NOT have applied due to stop flag
	if got, ok := ctx.Annotations[AnnotationIgnoreMessage].(bool); ok && got {
		t.Errorf("expected ignore message NOT set due to stop flag, but it was set")
	}
}

func TestDBSourceFilter_NoFilters(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	// Clean up all test filters
	cleanupFilters(t, db)

	filter := NewDBSourceFilter(db, nil)

	rawEmail := "From: user@example.com\r\nTo: support@company.com\r\nSubject: Hello\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(rawEmail)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: make(map[string]any),
	}

	err := filter.Apply(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// No annotations should be set
	if len(ctx.Annotations) > 0 {
		t.Errorf("expected no annotations, got %v", ctx.Annotations)
	}
}

func TestDBSourceFilter_NilDB(t *testing.T) {
	filter := NewDBSourceFilter(nil, nil)

	rawEmail := "From: user@example.com\r\nTo: support@company.com\r\nSubject: Hello\r\n\r\nBody"
	msg := &connector.FetchedMessage{Raw: []byte(rawEmail)}
	ctx := &MessageContext{
		Message:     msg,
		Annotations: make(map[string]any),
	}

	err := filter.Apply(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Apply failed with nil DB: %v", err)
	}
}

// Helper functions

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
