// Package secureconfig provides encrypted key-value storage for plugin secrets.
//
// Plugins store API keys, PINs, webhook secrets via HostAPI SecureConfigGet/Set.
// The platform manages AES-256-GCM encryption with a platform-managed key.
package secureconfig

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

const (
	// KeyEnvVar is the environment variable for the encryption key.
	KeyEnvVar = "GOATFLOW_SECURE_KEY"
	// keyLength is the required key length in bytes (AES-256).
	keyLength = 32
	// nonceLength is the GCM nonce length.
	nonceLength = 12
)

var (
	encKey     []byte
	encKeyOnce sync.Once
	encKeyErr  error
)

// GetKey returns the platform encryption key, initialising it on first call.
// Key source priority: SetKey override > env var > auto-generated.
func GetKey() ([]byte, error) {
	if encKey != nil {
		return encKey, encKeyErr
	}
	encKeyOnce.Do(func() {
		encKey, encKeyErr = loadOrGenerateKey()
	})
	return encKey, encKeyErr
}

// SetKey sets the encryption key directly (for testing).
func SetKey(key []byte) {
	encKey = key
	if key == nil {
		encKeyErr = nil
	} else {
		encKeyErr = nil
	}
}

func loadOrGenerateKey() ([]byte, error) {
	// Try environment variable first.
	if hexKey := os.Getenv(KeyEnvVar); hexKey != "" {
		key, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", KeyEnvVar, err)
		}
		if len(key) != keyLength {
			return nil, fmt.Errorf("%s must be %d bytes (%d hex chars), got %d bytes", KeyEnvVar, keyLength, keyLength*2, len(key))
		}
		return key, nil
	}

	// Auto-generate key.
	key := make([]byte, keyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	slog.Warn("generated secure settings key — set "+KeyEnvVar+" in production",
		"key_hex", hex.EncodeToString(key))
	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns [12-byte nonce][ciphertext][16-byte GCM tag].
func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, nonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts AES-256-GCM ciphertext.
// Input format: [12-byte nonce][ciphertext][16-byte GCM tag].
func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	if len(ciphertext) < nonceLength+16 {
		return nil, fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := ciphertext[:nonceLength]
	ct := ciphertext[nonceLength:]

	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed (key mismatch or tampered data): %w", err)
	}

	return plaintext, nil
}

// ValueHint returns the last 4 characters of a string for masked display.
// Returns the full string if shorter than 5 characters.
func ValueHint(value string) string {
	if len(value) <= 4 {
		return value
	}
	return value[len(value)-4:]
}

// MaskedDisplay returns a masked version for admin display: "••••••••abcd".
func MaskedDisplay(hint string) string {
	if hint == "" {
		return "••••••••"
	}
	return "••••••••" + hint
}
