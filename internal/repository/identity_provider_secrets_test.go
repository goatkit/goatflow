//go:build integration

package repository

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/secureconfig"
)

// TestIdentityProviderSecretsEncryptedAtRest: the OIDC client secret and the
// SAML SP private key are stored encrypted, come back in plain text from
// GetProvider, and pre-0.10.0 plain-text rows are sealed on first read.
func TestIdentityProviderSecretsEncryptedAtRest(t *testing.T) {
	db, err := getTestDB()
	require.NoError(t, err)
	defer db.Close()
	testKey := make([]byte, 32)
	_, err = rand.Read(testKey)
	require.NoError(t, err)
	t.Setenv(secureconfig.KeyEnvVar, hex.EncodeToString(testKey))
	secureconfig.SetKey(testKey)
	t.Cleanup(func() { secureconfig.SetKey(nil) })

	repo := NewIdentityProviderRepository(db)
	raw := func(id uint) (cs, pk string) {
		t.Helper()
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT client_secret, private_key FROM gk_identity_provider WHERE id = ?`), id).Scan(&cs, &pk))
		return
	}
	now := time.Now()
	const secret, key = "oidc-client-secret-value", "-----BEGIN PRIVATE KEY-----\nMIIEsecret\n-----END PRIVATE KEY-----"
	p := &models.IdentityProvider{
		Name: fmt.Sprintf("enc-test-%d", now.UnixNano()), ProviderType: "oidc", ClientID: "cid",
		ClientSecret: secret, PrivateKey: key, DiscoveryURL: "https://idp.example.com",
		Scopes: "openid", UserClaimEmail: "email", UserClaimName: "name", UserTable: "users",
		Enabled: true, CreateTime: now, CreateBy: 1, ChangeTime: now, ChangeBy: 1,
	}
	require.NoError(t, repo.CreateProvider(p))
	t.Cleanup(func() { _ = repo.DeleteProvider(p.ID) })

	cs, pk := raw(p.ID)
	assert.True(t, strings.HasPrefix(cs, sealedPrefix), "client secret stored sealed on create, got %q", cs)
	assert.True(t, strings.HasPrefix(pk, sealedPrefix), "private key stored sealed on create")
	assert.NotContains(t, cs+pk, "secret")

	got, err := repo.GetProvider(p.ID)
	require.NoError(t, err)
	assert.Equal(t, secret, got.ClientSecret)
	assert.Equal(t, key, got.PrivateKey)

	// Update keeps them sealed.
	got.ClientSecret = "rotated-secret"
	require.NoError(t, repo.UpdateProvider(got))
	cs, _ = raw(p.ID)
	assert.True(t, strings.HasPrefix(cs, sealedPrefix))
	got, err = repo.GetProvider(p.ID)
	require.NoError(t, err)
	assert.Equal(t, "rotated-secret", got.ClientSecret)

	// A plain-text row from before 0.10.0 still works and is sealed on read.
	_, err = db.Exec(database.ConvertPlaceholders(
		`UPDATE gk_identity_provider SET client_secret = ?, private_key = ? WHERE id = ?`), "legacy-plain", key, p.ID)
	require.NoError(t, err)
	got, err = repo.GetProvider(p.ID)
	require.NoError(t, err)
	assert.Equal(t, "legacy-plain", got.ClientSecret)
	assert.Equal(t, key, got.PrivateKey)
	cs, pk = raw(p.ID)
	assert.True(t, strings.HasPrefix(cs, sealedPrefix), "legacy client secret re-sealed")
	assert.True(t, strings.HasPrefix(pk, sealedPrefix), "legacy private key re-sealed")

	// Without a configured key nothing is sealed: a per-process key would make
	// the secret unreadable after a restart.
	t.Setenv(secureconfig.KeyEnvVar, "")
	got.ClientSecret = "no-key-secret"
	require.NoError(t, repo.UpdateProvider(got))
	cs, _ = raw(p.ID)
	assert.Equal(t, "no-key-secret", cs)
}
