package config

import (
	"bufio"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func readSynthesizedEnv(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // test-owned temp file
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	vars := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("malformed line %q", line)
		}
		vars[k] = v
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return vars
}

func assertSecureKey(t *testing.T, key string) {
	t.Helper()
	if len(key) != 64 {
		t.Fatalf("GOATFLOW_SECURE_KEY = %q: want 64 hex chars", key)
	}
	raw, err := hex.DecodeString(key)
	if err != nil || len(raw) != 32 {
		t.Fatalf("GOATFLOW_SECURE_KEY = %q is not 32 bytes of hex: %v", key, err)
	}
}

// The synthesized .env must boot a production deployment: the compose files
// refuse to start without GOATFLOW_SECURE_KEY, and production rejects a JWT
// secret shorter than MinJWTSecretLength.
func TestSynthesizeEnvProducesRequiredSecrets(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".env")
	if err := NewSynthesizer(out).SynthesizeEnv(false); err != nil {
		t.Fatalf("SynthesizeEnv: %v", err)
	}
	vars := readSynthesizedEnv(t, out)

	assertSecureKey(t, vars["GOATFLOW_SECURE_KEY"])
	if got := vars["APP_URL"]; got != "http://localhost:8081" {
		t.Errorf("APP_URL = %q, want the backend's published port http://localhost:8081", got)
	}

	t.Setenv("JWT_SECRET", vars["JWT_SECRET"])
	v := NewSecretValidator(&Config{})
	v.validateJWTSecret(true)
	if len(v.errors) != 0 {
		t.Errorf("synthesized JWT_SECRET %q fails production validation: %v", vars["JWT_SECRET"], v.errors)
	}
}

// The synthesized SMTP settings must reach the mail sandbox of the development
// compose stack: docker-compose.yml forwards SMTP_HOST to every GoatFlow
// process as GOATFLOW_EMAIL_SMTP_HOST, so it has to name one of its services.
func TestSynthesizeEnvSMTPHostIsComposeService(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".env")
	if err := NewSynthesizer(out).SynthesizeEnv(false); err != nil {
		t.Fatalf("SynthesizeEnv: %v", err)
	}
	host := readSynthesizedEnv(t, out)["SMTP_HOST"]

	raw, err := os.ReadFile("../../../docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	var compose struct {
		Services map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &compose); err != nil {
		t.Fatal(err)
	}
	if _, ok := compose.Services[host]; !ok {
		t.Errorf("synthesized SMTP_HOST %q is not a docker-compose.yml service", host)
	}
}

// Rotating secrets must not replace the encryption key: values encrypted
// with the old key would become unreadable.
func TestSynthesizeEnvRotateKeepsSecureKey(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".env")
	const existingKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	const existingJWT = "prod-jwt-0123456789abcdef0123456789abcdef"
	seed := "APP_ENV=production\nGOATFLOW_SECURE_KEY=" + existingKey + "\nJWT_SECRET=" + existingJWT + "\n"
	if err := os.WriteFile(out, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := NewSynthesizer(out).SynthesizeEnv(true); err != nil {
		t.Fatalf("SynthesizeEnv(rotate): %v", err)
	}
	vars := readSynthesizedEnv(t, out)

	if got := vars["GOATFLOW_SECURE_KEY"]; got != existingKey {
		t.Errorf("GOATFLOW_SECURE_KEY rotated to %q; it must be kept", got)
	}
	if got := vars["JWT_SECRET"]; got == existingJWT || len(got) < MinJWTSecretLength {
		t.Errorf("JWT_SECRET = %q: want a new secret of at least %d chars", got, MinJWTSecretLength)
	}
}

// A first rotation of an .env that predates GOATFLOW_SECURE_KEY adds one.
func TestSynthesizeEnvRotateAddsMissingSecureKey(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(out, []byte("APP_ENV=production\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NewSynthesizer(out).SynthesizeEnv(true); err != nil {
		t.Fatalf("SynthesizeEnv(rotate): %v", err)
	}
	assertSecureKey(t, readSynthesizedEnv(t, out)["GOATFLOW_SECURE_KEY"])
}

// Rewriting .env first copies the old file aside; the copy holds the same
// secrets, so it must be readable by the owner only.
func TestWriteEnvFileBackupIsOwnerOnly(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(out, []byte("JWT_SECRET=old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NewSynthesizer(out).writeEnvFile(); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}
	backups, err := filepath.Glob(out + ".backup.*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v (err %v), want exactly one", backups, err)
	}
	st, err := os.Stat(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("backup mode = %v, want no group/other access", perm)
	}
}
