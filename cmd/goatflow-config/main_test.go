package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportFilename(t *testing.T) {
	dir := filepath.Join("out", "export")
	got, err := exportFilename(dir, "ticket-routes")
	if err != nil {
		t.Fatalf("exportFilename: %v", err)
	}
	if want := filepath.Join(dir, "ticket-routes.yaml"); got != want {
		t.Fatalf("exportFilename = %q, want %q", got, want)
	}

	for _, name := range []string{"", ".", "..", "../escape", "a/b", `a\b`} {
		if _, err := exportFilename(dir, name); err == nil {
			t.Errorf("exportFilename(%q) accepted a name that leaves the export dir", name)
		}
	}
}

func TestWalkYAMLFilesStaysInsideDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "import")
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "a.yaml"), "a")
	write(filepath.Join(dir, "sub", "c.yml"), "c")
	write(filepath.Join(dir, "notes.txt"), "skip")
	write(filepath.Join(base, "secret.yaml"), "outside")
	if err := os.Symlink(filepath.Join(base, "secret.yaml"), filepath.Join(dir, "evil.yaml")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	read := map[string]string{}
	var failed []string
	err := walkYAMLFiles(dir, func(path string, data []byte, err error) {
		if err != nil {
			failed = append(failed, path)
			return
		}
		read[path] = string(data)
	})
	if err != nil {
		t.Fatalf("walkYAMLFiles: %v", err)
	}

	want := map[string]string{
		filepath.Join(dir, "a.yaml"):       "a",
		filepath.Join(dir, "sub", "c.yml"): "c",
	}
	if len(read) != len(want) {
		t.Fatalf("read %v, want %v", read, want)
	}
	for path, content := range want {
		if read[path] != content {
			t.Errorf("read[%q] = %q, want %q", path, read[path], content)
		}
	}
	if len(failed) != 1 || failed[0] != filepath.Join(dir, "evil.yaml") {
		t.Fatalf("failed reads = %v, want only the symlink escaping the dir", failed)
	}
}

func TestWalkYAMLFilesMissingDir(t *testing.T) {
	err := walkYAMLFiles(filepath.Join(t.TempDir(), "missing"), func(string, []byte, error) {
		t.Fatal("visit called for a missing directory")
	})
	if err == nil {
		t.Fatal("walkYAMLFiles succeeded on a missing directory")
	}
}
