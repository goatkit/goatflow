package secureconfig

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := testKey(t)
	plaintext := []byte("secret_abc123def456")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext should not equal plaintext")
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted = %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestEncrypt_DifferentNonce(t *testing.T) {
	key := testKey(t)
	plaintext := []byte("same input")

	ct1, _ := Encrypt(plaintext, key)
	ct2, _ := Encrypt(plaintext, key)

	if bytes.Equal(ct1, ct2) {
		t.Error("same plaintext should produce different ciphertext (random nonce)")
	}

	// Both should decrypt to same value.
	d1, _ := Decrypt(ct1, key)
	d2, _ := Decrypt(ct2, key)
	if !bytes.Equal(d1, d2) {
		t.Error("both should decrypt to same value")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := testKey(t)
	key2 := testKey(t)
	plaintext := []byte("secret")

	ciphertext, _ := Encrypt(plaintext, key1)

	_, err := Decrypt(ciphertext, key2)
	if err == nil {
		t.Error("expected error with wrong key")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := testKey(t)
	plaintext := []byte("secret")

	ciphertext, _ := Encrypt(plaintext, key)

	// Flip a byte in the ciphertext.
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xff

	_, err := Decrypt(tampered, key)
	if err == nil {
		t.Error("expected error with tampered ciphertext")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	key := testKey(t)
	_, err := Decrypt([]byte("short"), key)
	if err == nil {
		t.Error("expected error with short ciphertext")
	}
}

func TestEncrypt_EmptyPlaintext(t *testing.T) {
	key := testKey(t)
	ciphertext, err := Encrypt([]byte(""), key)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != "" {
		t.Errorf("expected empty string, got %q", string(decrypted))
	}
}

func TestValueHint(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"secret_abc123def456", "f456"},
		{"abcd", "abcd"},
		{"abc", "abc"},
		{"ab", "ab"},
		{"", ""},
		{"12345", "2345"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ValueHint(tt.input); got != tt.want {
				t.Errorf("ValueHint(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMaskedDisplay(t *testing.T) {
	tests := []struct {
		hint string
		want string
	}{
		{"f456", "••••••••f456"},
		{"", "••••••••"},
		{"ab", "••••••••ab"},
	}
	for _, tt := range tests {
		t.Run(tt.hint, func(t *testing.T) {
			if got := MaskedDisplay(tt.hint); got != tt.want {
				t.Errorf("MaskedDisplay(%q) = %q, want %q", tt.hint, got, tt.want)
			}
		})
	}
}

func TestSetKey_And_GetKey(t *testing.T) {
	key := testKey(t)
	SetKey(key)
	defer SetKey(nil) // clean up

	got, err := GetKey()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, key) {
		t.Error("GetKey should return the key set via SetKey")
	}
}
