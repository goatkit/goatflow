package repository

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/secureconfig"
)

// sealedPrefix marks an identity provider secret (OIDC client secret, SAML SP
// private key) stored AES-256-GCM encrypted with the platform secure key.
// Values without it are plain text written before 0.10.0; they are still read
// and are sealed the next time the provider is loaded or saved.
const sealedPrefix = "enc:v1:"

func sealProviderSecret(plain string) (string, error) {
	if plain == "" || strings.HasPrefix(plain, sealedPrefix) {
		return plain, nil
	}
	if os.Getenv(secureconfig.KeyEnvVar) == "" {
		// Without a configured key every restart generates a new one, so a
		// sealed secret would become unreadable and SSO would lock everyone
		// out. Keep the old plain-text storage until the key is set.
		slog.Warn("identity provider secret stored unencrypted: set " + secureconfig.KeyEnvVar + " to encrypt it")
		return plain, nil
	}
	key, err := secureconfig.GetKey()
	if err != nil {
		return "", fmt.Errorf("secure config key: %w", err)
	}
	enc, err := secureconfig.Encrypt([]byte(plain), key)
	if err != nil {
		return "", err
	}
	return sealedPrefix + base64.StdEncoding.EncodeToString(enc), nil
}

// openProviderSecret returns the plain value and whether the stored value was
// legacy plain text that should be re-sealed.
func openProviderSecret(stored string) (plain string, legacy bool, err error) {
	if stored == "" {
		return "", false, nil
	}
	if !strings.HasPrefix(stored, sealedPrefix) {
		return stored, true, nil
	}
	enc, err := base64.StdEncoding.DecodeString(stored[len(sealedPrefix):])
	if err != nil {
		return "", false, fmt.Errorf("decode identity provider secret: %w", err)
	}
	key, err := secureconfig.GetKey()
	if err != nil {
		return "", false, fmt.Errorf("secure config key: %w", err)
	}
	out, err := secureconfig.Decrypt(enc, key)
	if err != nil {
		return "", false, fmt.Errorf("cannot decrypt identity provider secret (is %s the same for every GoatFlow process?): %w",
			secureconfig.KeyEnvVar, err)
	}
	return string(out), false, nil
}

// openProviderSecrets decrypts the secret fields loaded into p and re-seals any
// legacy plain-text values in the database.
func (r *IdentityProviderRepository) openProviderSecrets(p *models.IdentityProvider, clientSecret, privateKey string) error {
	var legacyCS, legacyPK bool
	var err error
	if p.ClientSecret, legacyCS, err = openProviderSecret(clientSecret); err != nil {
		return err
	}
	if p.PrivateKey, legacyPK, err = openProviderSecret(privateKey); err != nil {
		return err
	}
	if legacyCS || legacyPK {
		return r.writeProviderSecrets(p.ID, p.ClientSecret, p.PrivateKey)
	}
	return nil
}

func (r *IdentityProviderRepository) writeProviderSecrets(id uint, clientSecret, privateKey string) error {
	cs, err := sealProviderSecret(clientSecret)
	if err != nil {
		return err
	}
	pk, err := sealProviderSecret(privateKey)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(database.ConvertPlaceholders(
		`UPDATE gk_identity_provider SET client_secret = ?, private_key = ? WHERE id = ?`), cs, pk, id)
	if err != nil {
		return fmt.Errorf("seal identity provider secrets: %w", err)
	}
	return nil
}
