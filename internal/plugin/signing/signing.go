package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
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
	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	hash := sha256.Sum256(binaryData)

	// Sign the hash
	signature := ed25519.Sign(privateKey, hash[:])

	// Write signature to file (hex encoded for readability)
	sigHex := hex.EncodeToString(signature)
	if err := os.WriteFile(outputSigPath, []byte(sigHex), 0644); err != nil {
		return fmt.Errorf("failed to write signature: %w", err)
	}

	return nil
}

// VerifyBinary verifies a plugin binary against its signature file.
// Returns nil if the signature is valid and from a trusted key.
func VerifyBinary(binaryPath, signaturePath string, trustedKeys []ed25519.PublicKey) error {
	// Read binary and compute hash
	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to read binary: %w", err)
	}

	hash := sha256.Sum256(binaryData)

	// Read signature file
	sigData, err := os.ReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("failed to read signature file: %w", err)
	}

	// Decode hex signature
	signature, err := hex.DecodeString(string(sigData))
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

	return fmt.Errorf("signature verification failed: no matching trusted key")
}

// DefaultSignaturePath returns the default signature file path for a binary.
// For binary "/path/to/plugin", returns "/path/to/plugin.sig"
func DefaultSignaturePath(binaryPath string) string {
	return binaryPath + ".sig"
}

// IsSignatureRequired checks if signature verification should be enforced.
// This can be configured via environment variable or build tag.
func IsSignatureRequired() bool {
	// Check environment variable
	if os.Getenv("GOATFLOW_REQUIRE_SIGNATURES") == "1" {
		return true
	}
	
	// In production builds, this could default to true
	// For development, we default to false (opt-in)
	return false
}