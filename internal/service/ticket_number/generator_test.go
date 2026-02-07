//go:build integration

package ticket_number

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test database connection for integration tests.
func getTestDB(t *testing.T) *sql.DB {
	driver := os.Getenv("TEST_DB_DRIVER")
	if driver == "" {
		driver = os.Getenv("DB_DRIVER")
	}
	if driver == "" {
		driver = "postgres"
	}

	var openDriver string
	var dsn string

	switch driver {
	case "mysql", "mariadb":
		openDriver = "mysql"
		host := envOrDefault("TEST_DB_HOST", "127.0.0.1")
		port := envOrDefault("TEST_DB_PORT", "3306")
		user := envOrDefault("TEST_DB_USER", "root")
		pass := envOrDefault("TEST_DB_PASSWORD", "")
		name := envOrDefault("TEST_DB_NAME", "test")
		params := envOrDefault("TEST_DB_PARAMS", "parseTime=true&loc=UTC&charset=utf8mb4,utf8")
		if params != "" {
			dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", user, pass, host, port, name, params)
		} else {
			dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, name)
		}
	default:
		openDriver = "postgres"
		dsn = os.Getenv("TEST_DB_DSN")
		if dsn == "" {
			host := envOrDefault("TEST_DB_HOST", "localhost")
			port := envOrDefault("TEST_DB_PORT", "5432")
			user := envOrDefault("TEST_DB_USER", "goatflow_user")
			pass := envOrDefault("TEST_DB_PASSWORD", "")
			name := envOrDefault("TEST_DB_NAME", "goatflow")
			sslMode := envOrDefault("TEST_DB_SSLMODE", "disable")
			dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, name, sslMode)
		}
	}

	db, err := sql.Open(openDriver, dsn)
	if err != nil {
		t.Skipf("Database not available for integration test: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Database ping failed: %v", err)
	}

	_, _ = db.Exec("DELETE FROM ticket_number_counter")

	return db
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Test Date Generator.
func TestDateGenerator(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	config := DateConfig{
		IncludeHour:   true,
		CounterDigits: 6,
		ResetDaily:    true,
	}

	generator := NewDateGenerator(db, config)

	t.Run("GeneratesCorrectFormat", func(t *testing.T) {
		ticketNum, err := generator.Generate()
		require.NoError(t, err)

		// Format: YYYYMMDDHH + 6 digits = 16 chars total
		assert.Len(t, ticketNum, 16)

		// Verify format: 2025082814000001
		now := time.Now()
		expectedPrefix := fmt.Sprintf("%04d%02d%02d%02d",
			now.Year(), now.Month(), now.Day(), now.Hour())
		assert.True(t, strings.HasPrefix(ticketNum, expectedPrefix))

		// Verify counter part is numeric
		counterPart := ticketNum[10:]
		assert.Regexp(t, `^\d{6}$`, counterPart)
	})

	t.Run("IncrementsCounter", func(t *testing.T) {
		require.NoError(t, resetCounter(db, generator.getCounterUID(time.Now()), 0))

		first, err := generator.Generate()
		require.NoError(t, err)

		second, err := generator.Generate()
		require.NoError(t, err)

		// Second should be one more than first
		assert.NotEqual(t, first, second)

		// Extract counters
		firstCounter := first[10:]
		secondCounter := second[10:]

		assert.Equal(t, "000001", firstCounter)
		assert.Equal(t, "000002", secondCounter)
	})

	t.Run("ThreadSafe", func(t *testing.T) {
		require.NoError(t, resetCounter(db, generator.getCounterUID(time.Now()), 0))

		var wg sync.WaitGroup
		results := make(map[string]bool)
		var mu sync.Mutex

		// Generate 10 ticket numbers concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				num, err := generator.Generate()
				assert.NoError(t, err)

				mu.Lock()
				results[num] = true
				mu.Unlock()
			}()
		}

		wg.Wait()

		// All should be unique
		assert.Len(t, results, 10)
	})
}

// Test AutoIncrement Generator.
func TestAutoIncrementGenerator(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	config := AutoIncrementConfig{
		Prefix:    "T-",
		MinDigits: 7,
		StartFrom: 1000,
	}

	generator := NewAutoIncrementGenerator(db, config)

	t.Run("GeneratesCorrectFormat", func(t *testing.T) {
		ticketNum, err := generator.Generate()
		require.NoError(t, err)

		// Format: T-0001000
		assert.True(t, strings.HasPrefix(ticketNum, "T-"))
		assert.Regexp(t, `^T-\d{7,}$`, ticketNum)
	})

	t.Run("StartsFromConfiguredValue", func(t *testing.T) {
		// Reset for clean test
		_, _ = db.Exec("DELETE FROM ticket_number_counter WHERE counter_uid = 'test_auto_increment'")

		testGen := &AutoIncrementGenerator{
			db:         db,
			config:     config,
			counterUID: "test_auto_increment",
		}

		first, err := testGen.Generate()
		require.NoError(t, err)
		assert.Equal(t, "T-0001000", first)

		second, err := testGen.Generate()
		require.NoError(t, err)
		assert.Equal(t, "T-0001001", second)
	})

	t.Run("PersistsAcrossInstances", func(t *testing.T) {
		require.NoError(t, resetCounter(db, "auto_increment", config.StartFrom-1))

		// Create first generator and generate number
		gen1 := NewAutoIncrementGenerator(db, config)
		num1, err := gen1.Generate()
		require.NoError(t, err)

		// Create new generator instance
		gen2 := NewAutoIncrementGenerator(db, config)
		num2, err := gen2.Generate()
		require.NoError(t, err)

		// Should continue from where gen1 left off
		assert.NotEqual(t, num1, num2)
	})
}

// Test Random Generator.
func TestRandomGenerator(t *testing.T) {
	config := RandomConfig{
		Length:  8,
		Charset: "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		Prefix:  "TKT-",
	}

	generator := NewRandomGenerator(config)

	t.Run("GeneratesCorrectFormat", func(t *testing.T) {
		ticketNum, err := generator.Generate()
		require.NoError(t, err)

		// Format: TKT-XXXXXXXX (prefix + 8 chars)
		assert.Equal(t, 12, len(ticketNum)) // 4 + 8
		assert.True(t, strings.HasPrefix(ticketNum, "TKT-"))

		// Verify charset
		randomPart := ticketNum[4:]
		assert.Regexp(t, `^[A-Z0-9]{8}$`, randomPart)
	})

	t.Run("GeneratesUniqueNumbers", func(t *testing.T) {
		generated := make(map[string]bool)

		// Generate 100 numbers
		for i := 0; i < 100; i++ {
			num, err := generator.Generate()
			require.NoError(t, err)

			// Should not have seen this before
			assert.False(t, generated[num], "Duplicate found: %s", num)
			generated[num] = true
		}
	})

	t.Run("NoDatabaseRequired", func(t *testing.T) {
		// Random generator should work without database
		gen := NewRandomGenerator(config)
		_, err := gen.Generate()
		assert.NoError(t, err)
	})
}

// Test DateChecksum Generator.
func TestDateChecksumGenerator(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	config := DateChecksumConfig{
		Separator:      "-",
		CounterDigits:  6,
		ChecksumLength: 2,
		ResetDaily:     true,
	}

	generator := NewDateChecksumGenerator(db, config)

	t.Run("GeneratesCorrectFormat", func(t *testing.T) {
		ticketNum, err := generator.Generate()
		require.NoError(t, err)

		// Format: 20250828-000001-42
		parts := strings.Split(ticketNum, "-")
		require.Len(t, parts, 3)

		// Date part
		assert.Len(t, parts[0], 8)
		assert.Regexp(t, `^\d{8}$`, parts[0])

		// Counter part
		assert.Len(t, parts[1], 6)
		assert.Regexp(t, `^\d{6}$`, parts[1])

		// Checksum part
		assert.Len(t, parts[2], 2)
		assert.Regexp(t, `^\d{2}$`, parts[2])
	})

	t.Run("ChecksumIsConsistent", func(t *testing.T) {
		// Generate a number
		ticketNum, err := generator.Generate()
		require.NoError(t, err)

		// Verify checksum
		parts := strings.Split(ticketNum, "-")
		dateAndCounter := parts[0] + parts[1]
		expectedChecksum := generator.calculateChecksum(dateAndCounter)

		assert.Equal(t, expectedChecksum, parts[2])
	})

	t.Run("ChecksumPreventsTampering", func(t *testing.T) {
		ticketNum, err := generator.Generate()
		require.NoError(t, err)

		// Try to validate correct number
		assert.True(t, generator.Validate(ticketNum))

		// Tamper with the number
		parts := strings.Split(ticketNum, "-")
		originalCounter, err := strconv.Atoi(parts[1])
		require.NoError(t, err)

		var tamperedCounter string
		maxCounter := 1
		for i := 0; i < generator.config.CounterDigits; i++ {
			maxCounter *= 10
		}
		for offset := 1; offset < maxCounter; offset++ {
			next := (originalCounter + offset) % maxCounter
			candidate := fmt.Sprintf("%0*d", generator.config.CounterDigits, next)
			if generator.calculateChecksum(parts[0]+candidate) != parts[2] {
				tamperedCounter = candidate
				break
			}
		}
		require.NotEmpty(t, tamperedCounter, "expected checksum to detect tampering candidate")

		tampered := strings.Join([]string{
			parts[0],
			tamperedCounter,
			parts[2],
		}, generator.config.Separator)
		assert.False(t, generator.Validate(tampered))
	})
}

// Test Factory.
func TestGeneratorFactory(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	t.Run("CreatesDateGenerator", func(t *testing.T) {
		config := map[string]interface{}{
			"type": "date",
			"date": map[string]interface{}{
				"include_hour":   true,
				"counter_digits": 6,
				"reset_daily":    true,
			},
		}

		generator, err := NewGeneratorFromConfig(db, config)
		require.NoError(t, err)
		assert.IsType(t, &DateGenerator{}, generator)
	})

	t.Run("CreatesAutoIncrementGenerator", func(t *testing.T) {
		config := map[string]interface{}{
			"type": "auto_increment",
			"auto_increment": map[string]interface{}{
				"prefix":     "T-",
				"min_digits": 7,
				"start_from": 1000,
			},
		}

		generator, err := NewGeneratorFromConfig(db, config)
		require.NoError(t, err)
		assert.IsType(t, &AutoIncrementGenerator{}, generator)
	})

	t.Run("CreatesRandomGenerator", func(t *testing.T) {
		config := map[string]interface{}{
			"type": "random",
			"random": map[string]interface{}{
				"length":  8,
				"charset": "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
				"prefix":  "TKT-",
			},
		}

		generator, err := NewGeneratorFromConfig(db, config)
		require.NoError(t, err)
		assert.IsType(t, &RandomGenerator{}, generator)
	})

	t.Run("CreatesDateChecksumGenerator", func(t *testing.T) {
		config := map[string]interface{}{
			"type": "date_checksum",
			"date_checksum": map[string]interface{}{
				"separator":       "-",
				"counter_digits":  6,
				"checksum_length": 2,
				"reset_daily":     true,
			},
		}

		generator, err := NewGeneratorFromConfig(db, config)
		require.NoError(t, err)
		assert.IsType(t, &DateChecksumGenerator{}, generator)
	})

	t.Run("DefaultsToDateGenerator", func(t *testing.T) {
		config := map[string]interface{}{}

		generator, err := NewGeneratorFromConfig(db, config)
		require.NoError(t, err)
		assert.IsType(t, &DateGenerator{}, generator)
	})
}

// Test Max Length Constraint (OTRS tn field is VARCHAR(50)).
func TestMaxLengthConstraint(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	generators := []struct {
		name      string
		generator TicketNumberGenerator
	}{
		{
			name: "Date",
			generator: NewDateGenerator(db, DateConfig{
				IncludeHour:   true,
				CounterDigits: 6,
				ResetDaily:    true,
			}),
		},
		{
			name: "AutoIncrement",
			generator: NewAutoIncrementGenerator(db, AutoIncrementConfig{
				Prefix:    "T-",
				MinDigits: 7,
				StartFrom: 1000,
			}),
		},
		{
			name: "Random",
			generator: NewRandomGenerator(RandomConfig{
				Length:  8,
				Charset: "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
				Prefix:  "TKT-",
			}),
		},
		{
			name: "DateChecksum",
			generator: NewDateChecksumGenerator(db, DateChecksumConfig{
				Separator:      "-",
				CounterDigits:  6,
				ChecksumLength: 2,
				ResetDaily:     true,
			}),
		},
	}

	for _, tc := range generators {
		t.Run(tc.name, func(t *testing.T) {
			num, err := tc.generator.Generate()
			require.NoError(t, err)
			assert.LessOrEqual(t, len(num), 50,
				"Ticket number too long for OTRS tn field: %s", num)
		})
	}
}
