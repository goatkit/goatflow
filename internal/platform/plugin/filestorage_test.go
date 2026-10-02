package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/organisation"
)

// TestPluginFilesKeptPerOrganisation: a plugin call in organisation A cannot
// read, list or delete the file the same plugin stored in organisation B
// under the same key. The host used to read the org from a context key
// nothing sets, so every organisation shared one namespace.
func TestPluginFilesKeptPerOrganisation(t *testing.T) {
	initBackend()
	prev := activeBackend
	activeBackend = &localBackend{base: t.TempDir()}
	t.Cleanup(func() { activeBackend = prev })

	h := NewProdHostAPI()
	base := context.WithValue(context.Background(), PluginCallerKey, "filesorg")
	orgA := organisation.WithOrgID(base, 11)
	orgB := organisation.WithOrgID(base, 12)

	if err := h.StoreFile(orgB, "notes/plan.txt", []byte("org B secret"), nil); err != nil {
		t.Fatalf("store in org B: %v", err)
	}
	if data, _, err := h.GetFile(orgA, "notes/plan.txt"); err == nil {
		t.Fatalf("org A read org B's file: %q", data)
	}
	if files, err := h.ListFiles(orgA, "notes"); err == nil && len(files) != 0 {
		t.Fatalf("org A lists org B's files: %+v", files)
	}
	_ = h.DeleteFile(orgA, "notes/plan.txt")
	data, _, err := h.GetFile(orgB, "notes/plan.txt")
	if err != nil || string(data) != "org B secret" {
		t.Fatalf("org B file after org A delete: data=%q err=%v", data, err)
	}
}

func TestLocalBackendStoreReportsMetadataWriteFailure(t *testing.T) {
	b := &localBackend{base: t.TempDir()}
	path := filepath.Join("demo", "report.txt")

	// A directory squatting on the metadata sidecar path makes the
	// metadata write fail; Store must not report success.
	if err := os.MkdirAll(filepath.Join(b.base, path+".meta.json"), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := b.Store(path, []byte("hello"), map[string]string{"content-type": "text/plain"}); err == nil {
		t.Fatal("Store succeeded although metadata could not be written")
	}
}

func TestLocalBackendStoredFilesNotWorldReadable(t *testing.T) {
	b := &localBackend{base: t.TempDir()}
	path := filepath.Join("demo", "org-1", "secret.txt")

	if err := b.Store(path, []byte("hello"), map[string]string{"content-type": "text/plain"}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	for _, p := range []string{path, path + ".meta.json", filepath.Dir(path)} {
		info, err := os.Stat(filepath.Join(b.base, p))
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if perm := info.Mode().Perm(); perm&0o007 != 0 {
			t.Errorf("%s has world permissions: %o", p, perm)
		}
	}

	data, meta, err := b.Get(path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(data) != "hello" || meta["content-type"] != "text/plain" {
		t.Errorf("round trip mismatch: data=%q meta=%v", data, meta)
	}
}

func TestLocalBackendGetToleratesCorruptMetadata(t *testing.T) {
	b := &localBackend{base: t.TempDir()}
	path := filepath.Join("demo", "file.bin")

	if err := b.Store(path, []byte("payload"), nil); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(b.base, path+".meta.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	data, meta, err := b.Get(path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(data) != "payload" {
		t.Errorf("data = %q", data)
	}
	if meta != nil {
		t.Errorf("corrupt metadata should yield nil map, got %v", meta)
	}

	files, err := b.List("demo")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(files) != 1 || files[0].Metadata != nil {
		t.Errorf("List with corrupt metadata = %+v", files)
	}
}
