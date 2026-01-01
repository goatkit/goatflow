package ticket_number

import (
	"database/sql"
	"fmt"
	"os"
)

// NewGeneratorFromConfig creates a ticket number generator based on configuration.
func NewGeneratorFromConfig(db *sql.DB, config map[string]interface{}) (TicketNumberGenerator, error) {
	// Get generator type, default to "date"
	generatorType, _ := config["type"].(string)
	if generatorType == "" {
		// Check environment variable as fallback
		generatorType = os.Getenv("TICKET_NUMBER_GENERATOR")
		if generatorType == "" {
			generatorType = "date"
		}
	}

	switch generatorType {
	case "date":
		return createDateGenerator(db, config)

	case "auto_increment":
		return createAutoIncrementGenerator(db, config)

	case "random":
		return createRandomGenerator(config)

	case "date_checksum":
		return createDateChecksumGenerator(db, config)

	default:
		return nil, fmt.Errorf("unknown generator type: %s", generatorType)
	}
}

func createDateGenerator(db *sql.DB, config map[string]interface{}) (*DateGenerator, error) {
	dateConfig := DateConfig{
		IncludeHour:   true,
		CounterDigits: 6,
		ResetDaily:    true,
	}

	// Parse date-specific config
	if dateSettings, ok := config["date"].(map[string]interface{}); ok {
		if v, ok := dateSettings["include_hour"].(bool); ok {
			dateConfig.IncludeHour = v
		}
		if v, ok := dateSettings["counter_digits"].(int); ok {
			dateConfig.CounterDigits = v
		}
		if v, ok := dateSettings["reset_daily"].(bool); ok {
			dateConfig.ResetDaily = v
		}
	}

	return NewDateGenerator(db, dateConfig), nil
}

func createAutoIncrementGenerator(db *sql.DB, config map[string]interface{}) (*AutoIncrementGenerator, error) {
	aiConfig := AutoIncrementConfig{
		Prefix:    "T-",
		MinDigits: 7,
		StartFrom: 1000,
	}

	// Parse auto_increment-specific config
	if aiSettings, ok := config["auto_increment"].(map[string]interface{}); ok {
		if v, ok := aiSettings["prefix"].(string); ok {
			aiConfig.Prefix = v
		}
		if v, ok := aiSettings["min_digits"].(int); ok {
			aiConfig.MinDigits = v
		}
		if v, ok := aiSettings["start_from"].(int); ok {
			aiConfig.StartFrom = int64(v)
		}
		// Handle float64 (JSON numbers are decoded as float64)
		if v, ok := aiSettings["start_from"].(float64); ok {
			aiConfig.StartFrom = int64(v)
		}
	}

	return NewAutoIncrementGenerator(db, aiConfig), nil
}

func createRandomGenerator(config map[string]interface{}) (*RandomGenerator, error) {
	randomConfig := RandomConfig{
		Length:  8,
		Charset: "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		Prefix:  "TKT-",
	}

	// Parse random-specific config
	if randomSettings, ok := config["random"].(map[string]interface{}); ok {
		if v, ok := randomSettings["length"].(int); ok {
			randomConfig.Length = v
		}
		if v, ok := randomSettings["charset"].(string); ok {
			randomConfig.Charset = v
		}
		if v, ok := randomSettings["prefix"].(string); ok {
			randomConfig.Prefix = v
		}
	}

	return NewRandomGenerator(randomConfig), nil
}

func createDateChecksumGenerator(db *sql.DB, config map[string]interface{}) (*DateChecksumGenerator, error) {
	dcConfig := DateChecksumConfig{
		Separator:      "-",
		CounterDigits:  6,
		ChecksumLength: 2,
		ResetDaily:     true,
	}

	// Parse date_checksum-specific config
	if dcSettings, ok := config["date_checksum"].(map[string]interface{}); ok {
		if v, ok := dcSettings["separator"].(string); ok {
			dcConfig.Separator = v
		}
		if v, ok := dcSettings["counter_digits"].(int); ok {
			dcConfig.CounterDigits = v
		}
		if v, ok := dcSettings["checksum_length"].(int); ok {
			dcConfig.ChecksumLength = v
		}
		if v, ok := dcSettings["reset_daily"].(bool); ok {
			dcConfig.ResetDaily = v
		}
	}

	return NewDateChecksumGenerator(db, dcConfig), nil
}
