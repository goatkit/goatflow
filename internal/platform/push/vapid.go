package push

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
)

// GenerateVAPIDKeys generates a new VAPID key pair encoded as base64url strings.
func GenerateVAPIDKeys() (publicKey, privateKey string, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ECDSA key: %w", err)
	}

	// Encode private key (32 bytes, big-endian)
	privBytes := priv.D.Bytes()
	// Pad to 32 bytes
	padded := make([]byte, 32)
	copy(padded[32-len(privBytes):], privBytes)
	privateKey = base64.RawURLEncoding.EncodeToString(padded)

	// Encode public key (uncompressed point: 0x04 || X || Y, 65 bytes)
	pubBytes := elliptic.Marshal(priv.PublicKey.Curve, priv.PublicKey.X, priv.PublicKey.Y)
	publicKey = base64.RawURLEncoding.EncodeToString(pubBytes)

	return publicKey, privateKey, nil
}

// DecodeVAPIDPrivateKey decodes a base64url-encoded VAPID private key into an ECDSA key.
func DecodeVAPIDPrivateKey(privKeyB64 string) (*ecdsa.PrivateKey, error) {
	privBytes, err := base64.RawURLEncoding.DecodeString(privKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	curve := elliptic.P256()
	d := new(big.Int).SetBytes(privBytes)
	x, y := curve.ScalarBaseMult(d.Bytes())

	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: d,
	}, nil
}

// GetVAPIDKeys reads VAPID keys from the given public/private strings.
// If empty, generates new keys and logs a warning.
func GetVAPIDKeys(vapidPublic, vapidPrivate string) (pub, priv string, err error) {
	if vapidPublic != "" && vapidPrivate != "" {
		return vapidPublic, vapidPrivate, nil
	}

	log.Println("WARNING: VAPID keys not configured. Generating ephemeral keys. " +
		"Set GOATFLOW_PUSH_VAPID_PUBLIC_KEY and GOATFLOW_PUSH_VAPID_PRIVATE_KEY " +
		"to persist push subscriptions across restarts.")

	return GenerateVAPIDKeys()
}
