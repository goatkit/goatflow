package marketplace

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// LoadTrustedKeys reads trusted public keys from the environment.
// Resolution order:
//  1. GOATFLOW_TRUSTED_KEYS — hex-encoded, comma-separated public keys
//  2. GOATFLOW_TRUSTED_KEYS_FILE — file path, one hex key per line (lines starting with # are ignored)
//
// Returns an empty slice if neither is set.
func LoadTrustedKeys() ([]ed25519.PublicKey, error) {
	if hexKeys := os.Getenv("GOATFLOW_TRUSTED_KEYS"); hexKeys != "" {
		return parseHexKeys(strings.Split(hexKeys, ","))
	}

	if keyFile := os.Getenv("GOATFLOW_TRUSTED_KEYS_FILE"); keyFile != "" {
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("read trusted keys file: %w", err)
		}
		return parseHexKeys(strings.Split(string(data), "\n"))
	}

	return nil, nil
}

func parseHexKeys(rawKeys []string) ([]ed25519.PublicKey, error) {
	var keys []ed25519.PublicKey
	for _, raw := range rawKeys {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		key, err := parsePublicKey(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid trusted key: %v\n", err)
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func parsePublicKey(hexKey string) (ed25519.PublicKey, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PublicKeySize, len(keyBytes))
	}
	return ed25519.PublicKey(keyBytes), nil
}
