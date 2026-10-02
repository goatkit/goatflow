package sysconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// The deployed config holds effective settings, credentials included, so the
// file must not be readable by other users.
func TestDeployWritesOwnerOnlyFile(t *testing.T) {
	m := &Manager{settings: map[string]*Setting{}}
	out := filepath.Join(t.TempDir(), "conf", "deployed.yaml")
	if err := m.Deploy(out); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("deployed config mode = %v, want no group/other access", perm)
	}
}
