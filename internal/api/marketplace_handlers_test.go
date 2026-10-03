package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/marketplace"
	"github.com/goatkit/goatflow/internal/platform/version"
)

// marketplaceFixture serves a marketplace index with a multi-version plugin
// installed at 1.0.0 and a plugin with no version this host can run.
func marketplaceFixture(t *testing.T) *gin.Engine {
	t.Helper()
	oldVer := version.Version
	version.Version = "0.10.0"
	t.Cleanup(func() { version.Version = oldVer })

	index := marketplace.Index{Version: marketplace.IndexVersion, Plugins: []marketplace.PluginEntry{
		{Name: "demo", Repo: "acme/demo", LatestVersion: "2.0.0", MinHostVersion: "0.11.0", Versions: []marketplace.VersionEntry{
			{Version: "2.0.0", MinHostVersion: "0.11.0"},
			{Version: "1.0.0", MinHostVersion: "0.9.0"},
		}},
		{Name: "future", Repo: "acme/future", LatestVersion: "3.0.0", MinHostVersion: "0.12.0"},
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/marketplace.json" {
			t.Errorf("unexpected request %s: incompatible installs must not download", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(index)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("GOATFLOW_MARKETPLACE_URL", srv.URL+"/marketplace.json")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "demo"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo", "plugin.yaml"), []byte("name: demo\nversion: 1.0.0\nruntime: template\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldDir := pluginDir
	pluginDir = dir
	t.Cleanup(func() { pluginDir = oldDir })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/plugins/marketplace", HandleMarketplaceIndex)
	r.POST("/api/v1/plugins/marketplace/install", HandleMarketplaceInstall)
	return r
}

func TestMarketplaceIndexReportsHostCompatibility(t *testing.T) {
	r := marketplaceFixture(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/marketplace", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	type verStatus struct {
		Version    string `json:"version"`
		Compatible bool   `json:"compatible"`
	}
	var resp struct {
		HostVersion string `json:"host_version"`
		Plugins     []struct {
			Name             string      `json:"name"`
			Compatible       bool        `json:"compatible"`
			MinHostVersion   string      `json:"min_host_version"`
			InstallVersion   string      `json:"install_version"`
			InstalledVersion string      `json:"installed_version"`
			UpdateAvailable  bool        `json:"update_available"`
			Versions         []verStatus `json:"versions"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.HostVersion != "0.10.0" || len(resp.Plugins) != 2 {
		t.Fatalf("response = %s", w.Body.String())
	}

	demo, future := resp.Plugins[0], resp.Plugins[1]
	if !demo.Compatible || demo.InstallVersion != "1.0.0" || demo.MinHostVersion != "0.9.0" ||
		demo.InstalledVersion != "1.0.0" || demo.UpdateAvailable {
		t.Errorf("demo = %+v; want compatible, install 1.0.0, min 0.9.0, installed 1.0.0, no update (2.0.0 needs 0.11.0)", demo)
	}
	if len(demo.Versions) != 2 || demo.Versions[0] != (verStatus{"2.0.0", false}) || demo.Versions[1] != (verStatus{"1.0.0", true}) {
		t.Errorf("demo versions = %+v", demo.Versions)
	}
	if future.Compatible || future.InstallVersion != "" || future.MinHostVersion != "0.12.0" {
		t.Errorf("future = %+v; want incompatible, no install version, min 0.12.0", future)
	}
	if len(future.Versions) != 1 || future.Versions[0] != (verStatus{"3.0.0", false}) {
		t.Errorf("future versions = %+v, want the single latest version", future.Versions)
	}
}

func TestMarketplaceInstallRefusesIncompatibleVersion(t *testing.T) {
	r := marketplaceFixture(t)
	for _, tc := range []struct{ body, want string }{
		{`{"name":"future"}`, "future v3.0.0 requires GoatFlow >= 0.12.0, you have 0.10.0"},
		{`{"name":"demo","version":"2.0.0"}`, "demo v2.0.0 requires GoatFlow >= 0.11.0, you have 0.10.0"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/marketplace/install", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		var body struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != http.StatusConflict || body.Error != tc.want {
			t.Errorf("%s: status %d error %q; want 409 %q", tc.body, w.Code, body.Error, tc.want)
		}
	}
	if _, err := os.Stat(filepath.Join(pluginDir, "demo", "plugin.yaml")); err != nil {
		t.Errorf("installed demo disturbed: %v", err)
	}
}
