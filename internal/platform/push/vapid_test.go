package push

import (
	"encoding/base64"
	"testing"
)

func TestGenerateVAPIDKeys(t *testing.T) {
	pub, priv, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateVAPIDKeys() error: %v", err)
	}

	if pub == "" {
		t.Fatal("public key is empty")
	}
	if priv == "" {
		t.Fatal("private key is empty")
	}

	// Public key should be 65 bytes (uncompressed EC point) base64url-encoded
	pubBytes, err := base64.RawURLEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("public key base64 decode error: %v", err)
	}
	if len(pubBytes) != 65 {
		t.Errorf("public key length = %d, want 65", len(pubBytes))
	}
	if pubBytes[0] != 0x04 {
		t.Errorf("public key first byte = 0x%02x, want 0x04 (uncompressed)", pubBytes[0])
	}

	// Private key should be 32 bytes base64url-encoded
	privBytes, err := base64.RawURLEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("private key base64 decode error: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("private key length = %d, want 32", len(privBytes))
	}
}

func TestGenerateVAPIDKeys_Unique(t *testing.T) {
	pub1, _, _ := GenerateVAPIDKeys()
	pub2, _, _ := GenerateVAPIDKeys()
	if pub1 == pub2 {
		t.Error("two generated key pairs should have different public keys")
	}
}

func TestGetVAPIDKeys_Configured(t *testing.T) {
	pub, priv, err := GetVAPIDKeys("test-pub", "test-priv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub != "test-pub" || priv != "test-priv" {
		t.Errorf("expected configured keys, got pub=%q priv=%q", pub, priv)
	}
}

func TestGetVAPIDKeys_AutoGenerate(t *testing.T) {
	pub, priv, err := GetVAPIDKeys("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub == "" || priv == "" {
		t.Error("auto-generated keys should not be empty")
	}
}

func TestDecodeVAPIDPrivateKey(t *testing.T) {
	_, priv, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateVAPIDKeys() error: %v", err)
	}

	key, err := DecodeVAPIDPrivateKey(priv)
	if err != nil {
		t.Fatalf("DecodeVAPIDPrivateKey() error: %v", err)
	}
	if key == nil {
		t.Fatal("decoded key is nil")
	}
	if key.PublicKey.X == nil || key.PublicKey.Y == nil {
		t.Error("decoded key has nil public key coordinates")
	}
}
