package api

import (
	"bytes"
	"crypto/ed25519"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/plugin/packaging"
	"github.com/goatkit/goatflow/internal/platform/plugin/signing"
)

// uploadRouter serves HandlePluginUpload into a fresh plugin dir with a
// verifier that requires signatures from pub.
func uploadRouter(t *testing.T, pub ed25519.PublicKey) (*gin.Engine, string) {
	t.Helper()
	dir := t.TempDir()
	oldDir, oldVerifier, oldReload, oldUnload := pluginDir, pluginVerifier, pluginReloader, pluginUnloader
	pluginDir, pluginReloader, pluginUnloader = dir, nil, nil
	pluginVerifier = signing.Verifier{Required: true, Keys: []ed25519.PublicKey{pub}}
	t.Cleanup(func() {
		pluginDir, pluginVerifier, pluginReloader, pluginUnloader = oldDir, oldVerifier, oldReload, oldUnload
	})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/upload", HandlePluginUpload)
	return r, dir
}

// postUpload sends files as multipart fields (field -> filename, content).
func postUpload(t *testing.T, r *gin.Engine, files map[string][2]string) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for field, f := range files {
		fw, err := w.CreateFormFile(field, f[0])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write([]byte(f[1]))
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func sigFor(t *testing.T, content string, priv ed25519.PrivateKey) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := signing.SignBinary(p, p+".sig", priv); err != nil {
		t.Fatal(err)
	}
	sig, _ := os.ReadFile(p + ".sig")
	return string(sig)
}

func TestPluginUploadRequiresSignedWASM(t *testing.T) {
	pub, priv, _ := signing.GenerateKeyPair()
	_, otherPriv, _ := signing.GenerateKeyPair()
	const module = "wasm module bytes"

	for name, files := range map[string]map[string][2]string{
		"no signature":             {"plugin": {"demo.wasm", module}},
		"untrusted key":            {"plugin": {"demo.wasm", module}, "signature": {"demo.wasm.sig", sigFor(t, module, otherPriv)}},
		"signature of other bytes": {"plugin": {"demo.wasm", module}, "signature": {"demo.wasm.sig", sigFor(t, "other", priv)}},
	} {
		t.Run(name, func(t *testing.T) {
			r, dir := uploadRouter(t, pub)
			if rec := postUpload(t, r, files); rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Errorf("refused upload left %d files in the plugin dir", len(entries))
			}
		})
	}

	t.Run("signed by a trusted key", func(t *testing.T) {
		r, dir := uploadRouter(t, pub)
		rec := postUpload(t, r, map[string][2]string{
			"plugin":    {"demo.wasm", module},
			"signature": {"demo.wasm.sig", sigFor(t, module, priv)},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		if err := pluginVerifier.Check(filepath.Join(dir, "demo.wasm")); err != nil {
			t.Errorf("installed module does not verify for the loader: %v", err)
		}
	})
}

func TestPluginUploadChecksPackageBinary(t *testing.T) {
	pub, priv, _ := signing.GenerateKeyPair()
	pkg := func(signer ed25519.PrivateKey) string {
		src := t.TempDir()
		_ = os.WriteFile(filepath.Join(src, "plugin.yaml"), []byte("name: demo\nversion: 2.0.0\nruntime: wasm\n"), 0o600)
		module := filepath.Join(src, "demo.wasm")
		_ = os.WriteFile(module, []byte("module v2"), 0o600)
		if signer != nil {
			if err := signing.SignBinary(module, module+".sig", signer); err != nil {
				t.Fatal(err)
			}
		}
		out := filepath.Join(t.TempDir(), "demo.zip")
		if err := packaging.PackagePlugin(src, out); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(out)
		return string(data)
	}

	t.Run("unsigned binary refused, installed version kept", func(t *testing.T) {
		r, dir := uploadRouter(t, pub)
		installed := filepath.Join(dir, "demo", "plugin.yaml")
		_ = os.MkdirAll(filepath.Dir(installed), 0o750)
		_ = os.WriteFile(installed, []byte("name: demo\nversion: 1.0.0\nruntime: wasm\n"), 0o600)

		if rec := postUpload(t, r, map[string][2]string{"plugin": {"demo.zip", pkg(nil)}}); rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
		}
		got, _ := os.ReadFile(installed)
		if !bytes.Contains(got, []byte("1.0.0")) {
			t.Errorf("installed plugin changed: %s", got)
		}
		if _, err := os.Stat(filepath.Join(dir, "demo", "demo.wasm")); err == nil {
			t.Error("unsigned module was extracted")
		}
	})

	t.Run("signed binary installed", func(t *testing.T) {
		r, dir := uploadRouter(t, pub)
		if rec := postUpload(t, r, map[string][2]string{"plugin": {"demo.zip", pkg(priv)}}); rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		if err := pluginVerifier.Check(filepath.Join(dir, "demo", "demo.wasm")); err != nil {
			t.Errorf("installed module does not verify for the loader: %v", err)
		}
	})
}
