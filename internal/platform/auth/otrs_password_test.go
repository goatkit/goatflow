package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Vectors for password "Tr0ub4dor&3", computed outside this code:
//
//	md5-crypt: openssl passwd -1 -salt Ab3dEf7h
//	apr1:      openssl passwd -apr1 -salt Zy9xWv8u
//	sha1/512:  printf %s pw | sha1sum / sha512sum
//	BCRYPT:    python bcrypt.hashpw with the 16-char salt as raw bcrypt salt
//	           (what Crypt::Eksblowfish::Bcrypt::bcrypt_hash does in OTRS),
//	           written in OTRS's "BCRYPT:cost:salt:hash" layout
var otrsVectors = map[string]string{
	"sha2 (default)": otrsSHA2("Tr0ub4dor&3"),
	"sha512":         "c72bb621c7040cf4b6474063a9a7972690e252f35adbe7cfaff5b1ee2316efe7382e723d41e013a2bfbc33c3007c50c5847d21a889cce838536eac727808de40",
	"sha1":           "874572e7a5ae6a49466a6ac578b98adba78c6aa6",
	"md5-crypt":      "$1$Ab3dEf7h$CxJxqdAiRRK4Lfp0V2j6s1",
	"apr1":           "$apr1$Zy9xWv8u$Ngxd4chGty.4w9W8n/A9B0",
	"bcrypt (OTRS)":  "BCRYPT:5:Q7rTx2LmP9vKa4Zc:GaMb8oyK3GyY0ZLg4MKgt8VZP5.ZARi",
}

func TestVerifyPasswordAcceptsOTRSFormats(t *testing.T) {
	for _, cfg := range []string{"bcrypt", "sha256"} {
		t.Setenv(EnvPasswordHashType, cfg)
		h := NewPasswordHasher()
		for name, stored := range otrsVectors {
			assert.True(t, h.VerifyPassword("Tr0ub4dor&3", stored), "%s must verify (config %s)", name, cfg)
			assert.False(t, h.VerifyPassword("Tr0ub4dor&4", stored), "%s accepted a wrong password", name)
		}
	}
}

func TestVerifyPasswordRejectsUnverifiableStoredValues(t *testing.T) {
	h := NewPasswordHasher()
	for _, stored := range []string{
		"Tr0ub4dor&3",                              // plaintext: only OTRS CryptType=plain accepts it
		"BCRYPT:5:Q7rTx2LmP9vKa4Zc:short",          // malformed OTRS bcrypt
		"$5$rounds=5000$x$y",                       // modular crypt OTRS cannot verify either
		"874572E7A5AE6A49466A6AC578B98ADBA78C6AA6", // OTRS compares lowercase hexdigest
	} {
		assert.False(t, h.VerifyPassword(stored, stored), "%q must not verify", stored)
	}
}

// Every OTRS format is upgraded to the configured type on login.
func TestMigratePasswordHashUpgradesOTRSFormats(t *testing.T) {
	t.Setenv(EnvPasswordHashType, "")
	t.Setenv(EnvMigratePasswordHashes, "true")
	h := NewPasswordHasher()
	for name, stored := range otrsVectors {
		newHash, changed, err := h.MigratePasswordHash("Tr0ub4dor&3", stored)
		assert.NoError(t, err)
		assert.True(t, changed, name)
		assert.Equal(t, HashTypeBcrypt, hashTypeOf(newHash), name)
		assert.True(t, h.VerifyPassword("Tr0ub4dor&3", newHash), name)
	}
}
