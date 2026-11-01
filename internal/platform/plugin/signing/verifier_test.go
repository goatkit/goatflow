package signing

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifierFromEnv(t *testing.T) {
	pub, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	goatkit, err := ParsePublicKey(GoatKitPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "keys")
	other, _, _ := GenerateKeyPair()
	if err := os.WriteFile(keyFile, []byte("# publishers\n"+hex.EncodeToString(other)+"\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name     string
		require  string
		keys     string
		keyFile  string
		required bool
		want     []ed25519.PublicKey
	}{
		{name: "default requires signatures and trusts GoatKit", required: true, want: []ed25519.PublicKey{goatkit}},
		{name: "0 turns signatures off", require: "0", want: []ed25519.PublicKey{goatkit}},
		{name: "false turns signatures off", require: " FALSE ", want: []ed25519.PublicKey{goatkit}},
		{name: "any other value requires them", require: "no", required: true, want: []ed25519.PublicKey{goatkit}},
		{name: "env and file keys are added", keys: " " + hex.EncodeToString(pub) + " ,", keyFile: keyFile, required: true,
			want: []ed25519.PublicKey{goatkit, pub, other}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GOATFLOW_REQUIRE_SIGNATURES", tc.require)
			t.Setenv("GOATFLOW_TRUSTED_KEYS", tc.keys)
			t.Setenv("GOATFLOW_TRUSTED_KEYS_FILE", tc.keyFile)
			v, err := VerifierFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			if v.Required != tc.required {
				t.Errorf("Required = %v, want %v", v.Required, tc.required)
			}
			if len(v.Keys) != len(tc.want) {
				t.Fatalf("got %d keys, want %d", len(v.Keys), len(tc.want))
			}
			for i := range tc.want {
				if !v.Keys[i].Equal(tc.want[i]) {
					t.Errorf("key %d differs", i)
				}
			}
		})
	}

	t.Run("an invalid key is an error, not skipped", func(t *testing.T) {
		t.Setenv("GOATFLOW_TRUSTED_KEYS", "abcd")
		if _, err := VerifierFromEnv(); err == nil {
			t.Error("invalid trusted key accepted")
		}
	})
	t.Run("an unreadable key file is an error", func(t *testing.T) {
		t.Setenv("GOATFLOW_TRUSTED_KEYS_FILE", filepath.Join(t.TempDir(), "missing"))
		if _, err := VerifierFromEnv(); err == nil {
			t.Error("missing key file accepted")
		}
	})
}

func TestVerifierCheck(t *testing.T) {
	pub, priv, _ := GenerateKeyPair()
	otherPub, otherPriv, _ := GenerateKeyPair()
	dir := t.TempDir()
	write := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("plugin code "+name), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	unsigned := write("unsigned.wasm")
	signed := write("signed.wasm")
	if err := SignBinary(signed, signed+".sig", priv); err != nil {
		t.Fatal(err)
	}
	foreign := write("foreign.wasm")
	if err := SignBinary(foreign, foreign+".sig", otherPriv); err != nil {
		t.Fatal(err)
	}
	tampered := write("tampered.wasm")
	if err := SignBinary(tampered, tampered+".sig", priv); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tampered, []byte("changed after signing"), 0o600); err != nil {
		t.Fatal(err)
	}

	required := Verifier{Required: true, Keys: []ed25519.PublicKey{pub}}
	if err := required.Check(signed); err != nil {
		t.Errorf("signed by a trusted key: %v", err)
	}
	for name, path := range map[string]string{"unsigned": unsigned, "untrusted key": foreign, "changed after signing": tampered} {
		if err := required.Check(path); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if err := required.Check(unsigned); !strings.Contains(err.Error(), "not signed") {
		t.Errorf("unsigned error does not say so: %v", err)
	}
	if err := (Verifier{Required: true, Keys: []ed25519.PublicKey{otherPub}}).Check(foreign); err != nil {
		t.Errorf("trusting the other key: %v", err)
	}
	if err := (Verifier{Keys: []ed25519.PublicKey{pub}}).Check(unsigned); err != nil {
		t.Errorf("signatures not required, unsigned refused: %v", err)
	}
}
