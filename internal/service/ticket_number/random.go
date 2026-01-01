package ticket_number

import (
	"crypto/rand"
	"math/big"
)

// RandomConfig holds configuration for random generator.
type RandomConfig struct {
	Length  int
	Charset string
	Prefix  string
}

// Format: PREFIX + random string (e.g., TKT-A3X9K2M8).
type RandomGenerator struct {
	config RandomConfig
}

// This generator doesn't need a database connection.
func NewRandomGenerator(config RandomConfig) *RandomGenerator {
	// Set defaults
	if config.Length == 0 {
		config.Length = 8
	}
	if config.Charset == "" {
		config.Charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}

	return &RandomGenerator{
		config: config,
	}
}

// Generate creates a new random ticket number.
func (g *RandomGenerator) Generate() (string, error) {
	// Generate random string
	randomStr, err := generateRandomString(g.config.Length, g.config.Charset)
	if err != nil {
		return "", err
	}

	// Combine prefix and random string
	ticketNumber := g.config.Prefix + randomStr

	return ticketNumber, nil
}

// Reset does nothing for random generator.
func (g *RandomGenerator) Reset() error {
	// Random generator doesn't have a counter to reset
	return nil
}

// generateRandomString generates a random string of specified length from charset.
func generateRandomString(length int, charset string) (string, error) {
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		// Get random index
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}

		result[i] = charset[idx.Int64()]
	}

	return string(result), nil
}
