package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateKeyPair generates a new ed25519 key pair for plugin signing.
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return publicKey, privateKey, nil
}

// SignBinary creates a signature file for a plugin binary.
// The signature file will be created at outputSigPath and contains
// the ed25519 signature of the binary's SHA-256 hash.
func SignBinary(binaryPath, outputSigPath string, privateKey ed25519.PrivateKey) error {
	// Read and hash the binary
	binaryData, err := os.ReadFile(binaryPath) // #nosec G304 -- binaryPath is the gk CLI operator's file argument
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	hash := sha256.Sum256(binaryData)

	// Sign the hash
	signature := ed25519.Sign(privateKey, hash[:])

	// Write signature to file (hex encoded for readability)
	sigHex := hex.EncodeToString(signature)
	if err := os.WriteFile(outputSigPath, []byte(sigHex), 0o644); err != nil { // #nosec G306 G703 -- detached signature is public and shipped with the binary; path is the CLI file argument + ".sig"
		return fmt.Errorf("failed to write signature: %w", err)
	}

	return nil
}

// VerifyBinary verifies a plugin binary against its signature file.
// Returns nil if the signature is valid and from a trusted key.
func VerifyBinary(binaryPath, signaturePath string, trustedKeys []ed25519.PublicKey) error {
	// Read binary and compute hash
	binaryData, err := os.ReadFile(binaryPath) // #nosec G304 -- host-chosen path (plugin dir binary or marketplace temp download), not request input
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	hash := sha256.Sum256(binaryData)

	// Read signature file
	sigData, err := os.ReadFile(signaturePath) // #nosec G304 -- host-chosen path (plugin dir .sig or marketplace temp download), not request input
	if err != nil {
		return fmt.Errorf("failed to read signature file: %w", err)
	}

	// Decode hex signature
	signature, err := hex.DecodeString(strings.TrimSpace(string(sigData)))
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length: expected %d, got %d", ed25519.SignatureSize, len(signature))
	}

	// Try to verify against each trusted key
	for i, publicKey := range trustedKeys {
		if ed25519.Verify(publicKey, hash[:], signature) {
			return nil // Valid signature found
		}
		_ = i // Suppress unused variable warning
	}

	return fmt.Errorf("%w: no matching trusted key", ErrUntrusted)
}

// ErrUntrusted marks plugin code refused because it is unsigned or not
// signed by a trusted key.
var ErrUntrusted = errors.New("plugin signature check failed")

// DefaultSignaturePath returns the default signature file path for a binary.
// For binary "/path/to/plugin", returns "/path/to/plugin.sig"
func DefaultSignaturePath(binaryPath string) string {
	return binaryPath + ".sig"
}

// GoatKitPublicKey is the GoatKit publisher key. It signs the first-party
// plugins and the plugins bundled with GoatFlow, and is always trusted.
const GoatKitPublicKey = "fa81e60073ee6f3e5825c575c18a7c2556f2abf2640dcf0e296a98fe4eb60fa6"

// Verifier decides whether plugin code may be installed and run.
type Verifier struct {
	// Required: a plugin binary (gRPC executable or WASM module) must carry a
	// .sig file from a trusted key, and a marketplace package must be signed.
	Required bool
	// Keys are the trusted publisher keys.
	Keys []ed25519.PublicKey
}

// VerifierFromEnv returns the configured verifier. Signatures are required
// unless GOATFLOW_REQUIRE_SIGNATURES is "0" or "false". The trusted keys are
// the GoatKit key plus GOATFLOW_TRUSTED_KEYS (comma-separated hex) and
// GOATFLOW_TRUSTED_KEYS_FILE (one hex key per line, # comments); an invalid
// key is an error rather than being skipped.
func VerifierFromEnv() (Verifier, error) {
	v := Verifier{Required: true}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GOATFLOW_REQUIRE_SIGNATURES"))) {
	case "0", "false":
		v.Required = false
	}
	raw := []string{GoatKitPublicKey}
	raw = append(raw, strings.Split(os.Getenv("GOATFLOW_TRUSTED_KEYS"), ",")...)
	if keyFile := os.Getenv("GOATFLOW_TRUSTED_KEYS_FILE"); keyFile != "" {
		data, err := os.ReadFile(keyFile) // #nosec G304 G703 -- path from operator-set GOATFLOW_TRUSTED_KEYS_FILE, not request input
		if err != nil {
			return v, fmt.Errorf("read GOATFLOW_TRUSTED_KEYS_FILE: %w", err)
		}
		raw = append(raw, strings.Split(string(data), "\n")...)
	}
	for _, k := range raw {
		k = strings.TrimSpace(k)
		if k == "" || strings.HasPrefix(k, "#") {
			continue
		}
		key, err := ParsePublicKey(k)
		if err != nil {
			return v, fmt.Errorf("trusted key %q: %w", k, err)
		}
		v.Keys = append(v.Keys, key)
	}
	return v, nil
}

// Check verifies the file at path against path+".sig" when signatures are
// required; otherwise it accepts the file.
func (v Verifier) Check(path string) error {
	if !v.Required {
		return nil
	}
	if _, err := os.Stat(DefaultSignaturePath(path)); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s is not signed (no %s) and plugin signatures are required; "+
			"set GOATFLOW_REQUIRE_SIGNATURES=0 only on a development system", ErrUntrusted, filepath.Base(path), filepath.Base(DefaultSignaturePath(path)))
	}
	return VerifyBinary(path, DefaultSignaturePath(path), v.Keys)
}

// ParsePublicKey decodes a hex ed25519 public key.
func ParsePublicKey(hexKey string) (ed25519.PublicKey, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PublicKeySize, len(keyBytes))
	}
	return ed25519.PublicKey(keyBytes), nil
}
