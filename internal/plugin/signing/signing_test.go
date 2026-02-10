package signing

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	if len(publicKey) != ed25519.PublicKeySize {
		t.Errorf("Public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKey))
	}

	if len(privateKey) != ed25519.PrivateKeySize {
		t.Errorf("Private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}

	// Keys should be different each time
	publicKey2, privateKey2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Second GenerateKeyPair failed: %v", err)
	}

	if string(publicKey) == string(publicKey2) {
		t.Error("Generated identical public keys (extremely unlikely)")
	}

	if string(privateKey) == string(privateKey2) {
		t.Error("Generated identical private keys (extremely unlikely)")
	}
}

func TestSignAndVerifyBinary(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create a test binary file
	binaryPath := filepath.Join(tempDir, "test-plugin")
	testContent := "This is a test plugin binary content"
	if err := os.WriteFile(binaryPath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Generate signing key
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Sign the binary
	sigPath := filepath.Join(tempDir, "test-plugin.sig")
	if err := SignBinary(binaryPath, sigPath, privateKey); err != nil {
		t.Fatalf("Failed to sign binary: %v", err)
	}

	// Verify signature exists and is readable
	if _, err := os.Stat(sigPath); os.IsNotExist(err) {
		t.Fatal("Signature file was not created")
	}

	// Verify the signature
	trustedKeys := []ed25519.PublicKey{publicKey}
	if err := VerifyBinary(binaryPath, sigPath, trustedKeys); err != nil {
		t.Fatalf("Failed to verify valid signature: %v", err)
	}
}

func TestVerifyBinaryWithWrongKey(t *testing.T) {
	tempDir := t.TempDir()

	// Create test binary
	binaryPath := filepath.Join(tempDir, "test-plugin")
	if err := os.WriteFile(binaryPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Generate signing key and sign
	_, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate signing key: %v", err)
	}

	sigPath := filepath.Join(tempDir, "test-plugin.sig")
	if err := SignBinary(binaryPath, sigPath, privateKey); err != nil {
		t.Fatalf("Failed to sign binary: %v", err)
	}

	// Generate different key for verification
	wrongPublicKey, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate wrong key: %v", err)
	}

	// Verify should fail with wrong key
	trustedKeys := []ed25519.PublicKey{wrongPublicKey}
	err = VerifyBinary(binaryPath, sigPath, trustedKeys)
	if err == nil {
		t.Fatal("Expected verification to fail with wrong key, but it succeeded")
	}

	if !strings.Contains(err.Error(), "signature verification failed") {
		t.Errorf("Expected signature verification error, got: %v", err)
	}
}

func TestVerifyBinaryModified(t *testing.T) {
	tempDir := t.TempDir()

	// Create and sign binary
	binaryPath := filepath.Join(tempDir, "test-plugin")
	originalContent := "original content"
	if err := os.WriteFile(binaryPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	sigPath := filepath.Join(tempDir, "test-plugin.sig")
	if err := SignBinary(binaryPath, sigPath, privateKey); err != nil {
		t.Fatalf("Failed to sign binary: %v", err)
	}

	// Modify the binary after signing
	modifiedContent := "modified content"
	if err := os.WriteFile(binaryPath, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify binary: %v", err)
	}

	// Verification should fail
	trustedKeys := []ed25519.PublicKey{publicKey}
	err = VerifyBinary(binaryPath, sigPath, trustedKeys)
	if err == nil {
		t.Fatal("Expected verification to fail for modified binary, but it succeeded")
	}
}

func TestVerifyBinaryMissingSignature(t *testing.T) {
	tempDir := t.TempDir()

	// Create binary but no signature
	binaryPath := filepath.Join(tempDir, "test-plugin")
	if err := os.WriteFile(binaryPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	publicKey, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Try to verify without signature file
	sigPath := filepath.Join(tempDir, "nonexistent.sig")
	trustedKeys := []ed25519.PublicKey{publicKey}
	err = VerifyBinary(binaryPath, sigPath, trustedKeys)
	if err == nil {
		t.Fatal("Expected verification to fail for missing signature")
	}

	if !strings.Contains(err.Error(), "failed to read signature file") {
		t.Errorf("Expected missing signature file error, got: %v", err)
	}
}

func TestDefaultSignaturePath(t *testing.T) {
	tests := []struct {
		binary   string
		expected string
	}{
		{"/path/to/plugin", "/path/to/plugin.sig"},
		{"plugin.wasm", "plugin.wasm.sig"},
		{"./relative/path", "./relative/path.sig"},
		{"", ".sig"},
	}

	for _, test := range tests {
		result := DefaultSignaturePath(test.binary)
		if result != test.expected {
			t.Errorf("DefaultSignaturePath(%q) = %q, expected %q", test.binary, result, test.expected)
		}
	}
}

func TestVerifyWithMultipleTrustedKeys(t *testing.T) {
	tempDir := t.TempDir()

	// Create test binary
	binaryPath := filepath.Join(tempDir, "test-plugin")
	if err := os.WriteFile(binaryPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Generate multiple keys
	publicKey1, privateKey1, _ := GenerateKeyPair()
	publicKey2, _, _ := GenerateKeyPair()
	publicKey3, _, _ := GenerateKeyPair()

	// Sign with first key
	sigPath := filepath.Join(tempDir, "test-plugin.sig")
	if err := SignBinary(binaryPath, sigPath, privateKey1); err != nil {
		t.Fatalf("Failed to sign binary: %v", err)
	}

	// Verify should succeed when first key is in trusted list
	trustedKeys := []ed25519.PublicKey{publicKey2, publicKey1, publicKey3}
	if err := VerifyBinary(binaryPath, sigPath, trustedKeys); err != nil {
		t.Fatalf("Failed to verify with multiple trusted keys: %v", err)
	}

	// Verify should fail when signing key is not in trusted list
	trustedKeysWithoutSigner := []ed25519.PublicKey{publicKey2, publicKey3}
	err := VerifyBinary(binaryPath, sigPath, trustedKeysWithoutSigner)
	if err == nil {
		t.Fatal("Expected verification to fail when signer key not in trusted list")
	}
}