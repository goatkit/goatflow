package yamlmgmt

import (
	"os"
	"path/filepath"
	"testing"
)

// Saved versions are reloaded by a new manager over the same directory; a
// symlink under .versions that points outside it is not followed.
func TestVersionsReloadAndIgnoreEscapingSymlink(t *testing.T) {
	dir := t.TempDir()
	vm := NewVersionManager(dir)
	doc := &YAMLDocument{APIVersion: "v1", Kind: string(KindConfig), Spec: map[string]interface{}{"a": 1}}
	if _, err := vm.CreateVersion(KindConfig, "main", doc, "first"); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	// A version-shaped file outside the store, linked into it.
	outside := t.TempDir()
	foreign := `{"number":"v9","name":"evil","kind":"Config","hash":"deadbeefdead","document":{"metadata":{"name":"evil"}}}`
	if err := os.WriteFile(filepath.Join(outside, "evil.json"), []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".versions", string(KindConfig), "evil_v9_deadbeef.json")
	if err := os.Symlink(filepath.Join(outside, "evil.json"), link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	reloaded := NewVersionManager(dir)
	versions, err := reloaded.ListVersions(KindConfig, "main")
	if err != nil || len(versions) != 1 {
		t.Fatalf("reloaded versions of main = %d (err %v), want 1", len(versions), err)
	}
	if got, _ := reloaded.ListVersions(KindConfig, "evil"); len(got) != 0 {
		t.Fatalf("version behind an escaping symlink was loaded: %+v", got)
	}
}
