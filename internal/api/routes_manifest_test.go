//go:build db

package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildRoutesManifestStructure(t *testing.T) {
	orig, _ := os.Getwd()
	restore := func() { _ = os.Chdir(orig) }
	defer restore()
	if _, err := os.Stat("routes"); os.IsNotExist(err) {
		// Walk up to 4 levels to locate project root containing routes dir
		found := false
		dir := orig
		for i := 0; i < 4; i++ {
			try := filepath.Join(dir, "routes")
			if st, err2 := os.Stat(try); err2 == nil && st.IsDir() {
				_ = os.Chdir(dir)
				found = true
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
		if !found {
			t.Fatalf("unable to locate routes directory from %s", orig)
		}
	}
	b, err := BuildRoutesManifest()
	if err != nil {
		t.Fatalf("BuildRoutesManifest error: %v", err)
	}
	var parsed struct {
		GeneratedAt string `json:"generatedAt"`
		Routes      []struct {
			Group  string `json:"group"`
			Method string `json:"method"`
			Path   string `json:"path"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.GeneratedAt == "" {
		t.Errorf("missing generatedAt timestamp")
	}
	if len(parsed.Routes) == 0 {
		t.Fatalf("expected at least one route in manifest")
	}
	for _, r := range parsed.Routes {
		if r.Group == "" || r.Method == "" || r.Path == "" {
			t.Errorf("incomplete route entry: %+v", r)
		}
	}
}
