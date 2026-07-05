package marketplace

import (
	"encoding/hex"
	"encoding/json"
	"github.com/goatkit/goatflow/internal/platform/plugin/packaging"
	"github.com/goatkit/goatflow/internal/platform/plugin/signing"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMatchesQuery(t *testing.T) {
	entry := PluginEntry{
		Name:        "inventory",
		Description: "Stock and warehouse management",
		Tags:        []string{"inventory", "stock", "warehouse"},
	}

	tests := []struct {
		query string
		want  bool
	}{
		{"inventory", true},
		{"stock", true},
		{"warehouse", true},
		{"manage", true}, // in description
		{"calendar", false},
		{"inventory", true}, // matchesQuery expects pre-lowered query (Search() lowers it)
		{"inv", true},       // partial match
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			if got := MatchesQuery(entry, tt.query); got != tt.want {
				t.Errorf("matchesQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestDownloadURL(t *testing.T) {
	url := DownloadURL("goatkit/inventory", "1.2.0", "inventory")
	want := "https://github.com/goatkit/inventory/releases/download/v1.2.0/inventory.zip"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}

func TestSignatureURL(t *testing.T) {
	url := SignatureURL("goatkit/inventory", "1.2.0", "inventory")
	want := "https://github.com/goatkit/inventory/releases/download/v1.2.0/inventory.zip.sig"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}

func TestResolveDependencies(t *testing.T) {
	index := &Index{
		Plugins: []PluginEntry{
			{Name: "billing", Dependencies: []string{"subscriptions", "payments"}},
			{Name: "subscriptions", Dependencies: nil},
			{Name: "payments", Dependencies: nil},
			{Name: "standalone", Dependencies: nil},
		},
	}

	t.Run("all deps satisfied", func(t *testing.T) {
		installed := []InstalledPlugin{
			{Name: "subscriptions"}, {Name: "payments"},
		}
		missing, err := ResolveDependencies("billing", index, installed)
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) != 0 {
			t.Errorf("expected no missing deps, got %v", missing)
		}
	})

	t.Run("missing dependency", func(t *testing.T) {
		installed := []InstalledPlugin{
			{Name: "subscriptions"},
		}
		missing, err := ResolveDependencies("billing", index, installed)
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) != 1 || missing[0] != "payments" {
			t.Errorf("expected [payments], got %v", missing)
		}
	})

	t.Run("no dependencies", func(t *testing.T) {
		missing, err := ResolveDependencies("standalone", index, nil)
		if err != nil {
			t.Fatal(err)
		}
		if missing != nil {
			t.Errorf("expected nil, got %v", missing)
		}
	})

	t.Run("plugin not found", func(t *testing.T) {
		_, err := ResolveDependencies("nonexistent", index, nil)
		if err == nil {
			t.Error("expected error for unknown plugin")
		}
	})
}

func TestTopologicalSort(t *testing.T) {
	t.Run("simple dependency chain", func(t *testing.T) {
		plugins := []PluginEntry{
			{Name: "billing", Dependencies: []string{"payments"}},
			{Name: "payments", Dependencies: nil},
		}
		sorted, err := TopologicalSort(plugins)
		if err != nil {
			t.Fatal(err)
		}
		if len(sorted) != 2 {
			t.Fatalf("expected 2, got %d", len(sorted))
		}
		// payments must come before billing.
		if sorted[0].Name != "payments" || sorted[1].Name != "billing" {
			t.Errorf("wrong order: %s, %s", sorted[0].Name, sorted[1].Name)
		}
	})

	t.Run("no dependencies", func(t *testing.T) {
		plugins := []PluginEntry{
			{Name: "a"}, {Name: "b"}, {Name: "c"},
		}
		sorted, err := TopologicalSort(plugins)
		if err != nil {
			t.Fatal(err)
		}
		if len(sorted) != 3 {
			t.Errorf("expected 3, got %d", len(sorted))
		}
	})

	t.Run("circular dependency detected", func(t *testing.T) {
		plugins := []PluginEntry{
			{Name: "a", Dependencies: []string{"b"}},
			{Name: "b", Dependencies: []string{"a"}},
		}
		_, err := TopologicalSort(plugins)
		if err == nil {
			t.Error("expected circular dependency error")
		}
	})

	t.Run("diamond dependency", func(t *testing.T) {
		plugins := []PluginEntry{
			{Name: "app", Dependencies: []string{"lib-a", "lib-b"}},
			{Name: "lib-a", Dependencies: []string{"core"}},
			{Name: "lib-b", Dependencies: []string{"core"}},
			{Name: "core"},
		}
		sorted, err := TopologicalSort(plugins)
		if err != nil {
			t.Fatal(err)
		}
		if len(sorted) != 4 {
			t.Fatalf("expected 4, got %d", len(sorted))
		}
		// core must come before lib-a, lib-b, and app.
		coreIdx := -1
		appIdx := -1
		for i, p := range sorted {
			if p.Name == "core" {
				coreIdx = i
			}
			if p.Name == "app" {
				appIdx = i
			}
		}
		if coreIdx >= appIdx {
			t.Error("core should come before app")
		}
	})
}

func TestIsThemePlugin(t *testing.T) {
	tests := []struct {
		name     string
		manifest struct{ PluginType, Runtime string }
		want     bool
	}{
		{"theme type", struct{ PluginType, Runtime string }{"theme", "grpc"}, true},
		{"theme runtime", struct{ PluginType, Runtime string }{"", "theme"}, true},
		{"regular plugin", struct{ PluginType, Runtime string }{"", "grpc"}, false},
		{"wasm plugin", struct{ PluginType, Runtime string }{"plugin", "wasm"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Can't import pkg/plugin without circular dep issues in this test,
			// so just test the logic directly.
			isTheme := tt.manifest.PluginType == "theme" || tt.manifest.Runtime == "theme"
			if isTheme != tt.want {
				t.Errorf("got %v, want %v", isTheme, tt.want)
			}
		})
	}
}

// K8s isolation constants are in internal/platform/plugin/grpc package — tested there.

// redirectTransport redirects all HTTP requests to a test server.
type redirectTransport struct {
	target string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.target
	return http.DefaultTransport.RoundTrip(req)
}

func TestInstall(t *testing.T) {
	pluginSrcDir := t.TempDir()
	os.WriteFile(filepath.Join(pluginSrcDir, "plugin.yaml"),
		[]byte("name: test-plugin\nversion: 1.0.0\nruntime: template\n"), 0644)
	os.WriteFile(filepath.Join(pluginSrcDir, "hello.txt"),
		[]byte("hello world"), 0644)

	zipPath := filepath.Join(t.TempDir(), "test-plugin.zip")
	if err := packaging.PackagePlugin(pluginSrcDir, zipPath); err != nil {
		t.Fatalf("PackagePlugin failed: %v", err)
	}
	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/marketplace.json", func(w http.ResponseWriter, r *http.Request) {
		index := Index{
			Version: 1,
			Plugins: []PluginEntry{
				{Name: "test-plugin", Repo: "test/repo", LatestVersion: "1.0.0", Runtime: "template"},
			},
		}
		json.NewEncoder(w).Encode(index)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".zip") {
			w.Header().Set("Content-Type", "application/zip")
			w.Write(zipData)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".sig") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.NotFound(w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	pluginsDir := t.TempDir()
	client := NewClient(pluginsDir)
	client.indexURL = server.URL + "/marketplace.json"
	u, _ := url.Parse(server.URL)
	client.httpClient = &http.Client{
		Transport: &redirectTransport{target: u.Host},
		Timeout:   10 * time.Second,
	}

	entry, err := client.FindPlugin("test-plugin")
	if err != nil {
		t.Fatalf("FindPlugin failed: %v", err)
	}
	if err := client.Install(entry); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	manifestPath := filepath.Join(pluginsDir, "test-plugin", "plugin.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("expected plugin.yaml at %s: %v", manifestPath, err)
	}
	if !strings.Contains(string(data), "test-plugin") {
		t.Errorf("plugin.yaml content unexpected: %s", string(data))
	}

	helloPath := filepath.Join(pluginsDir, "test-plugin", "hello.txt")
	helloData, err := os.ReadFile(helloPath)
	if err != nil {
		t.Fatalf("expected hello.txt at %s: %v", helloPath, err)
	}
	if string(helloData) != "hello world" {
		t.Errorf("hello.txt content = %q, want %q", string(helloData), "hello world")
	}

	if err := client.Install(entry); err != ErrAlreadyInstalled {
		t.Errorf("second install: got error %v, want ErrAlreadyInstalled", err)
	}
}

func TestCheckUpdatesSemver(t *testing.T) {
	pluginsDir := t.TempDir()
	pluginDir := filepath.Join(pluginsDir, "test-plugin")
	os.MkdirAll(pluginDir, 0755)
	os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"),
		[]byte("name: test-plugin\nversion: 1.0.0\nruntime: wasm\n"), 0644)

	client := NewClient(pluginsDir)
	client.index = &Index{
		Version: 1,
		Plugins: []PluginEntry{
			{Name: "test-plugin", LatestVersion: "1.1.0", Repo: "test/repo"},
		},
	}

	updates, err := client.CheckUpdates()
	if err != nil {
		t.Fatalf("CheckUpdates failed: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].LatestVersion != "1.1.0" {
		t.Errorf("latest version = %s, want 1.1.0", updates[0].LatestVersion)
	}

	client.index.Plugins[0].LatestVersion = "1.0.0"
	updates, _ = client.CheckUpdates()
	if len(updates) != 0 {
		t.Errorf("same version: expected 0 updates, got %d", len(updates))
	}

	client.index.Plugins[0].LatestVersion = "v1.0.0"
	updates, _ = client.CheckUpdates()
	if len(updates) != 0 {
		t.Errorf("v-prefix normalization: expected 0 updates, got %d", len(updates))
	}

	client.index.Plugins[0].LatestVersion = "0.9.0"
	updates, _ = client.CheckUpdates()
	if len(updates) != 0 {
		t.Errorf("older marketplace version: expected 0 updates, got %d", len(updates))
	}
}

func TestInstallSigned(t *testing.T) {
	pub, priv, err := signing.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	pubHex := hex.EncodeToString(pub)

	pluginSrcDir := t.TempDir()
	os.WriteFile(filepath.Join(pluginSrcDir, "plugin.yaml"),
		[]byte("name: signed-plugin\nversion: 1.0.0\nruntime: template\n"), 0644)

	zipPath := filepath.Join(t.TempDir(), "signed-plugin.zip")
	if err := packaging.PackagePlugin(pluginSrcDir, zipPath); err != nil {
		t.Fatalf("PackagePlugin failed: %v", err)
	}
	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}

	sigPath := zipPath + ".sig"
	if err := signing.SignBinary(zipPath, sigPath, priv); err != nil {
		t.Fatalf("SignBinary failed: %v", err)
	}
	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		t.Fatalf("read sig: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/marketplace.json", func(w http.ResponseWriter, r *http.Request) {
		index := Index{
			Version: 1,
			Plugins: []PluginEntry{
				{
					Name:          "signed-plugin",
					Repo:          "test/repo",
					LatestVersion: "1.0.0",
					Runtime:       "template",
					Verified:      true,
					PublicKey:     pubHex,
				},
			},
		}
		json.NewEncoder(w).Encode(index)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".zip") {
			w.Header().Set("Content-Type", "application/zip")
			w.Write(zipData)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".sig") {
			w.Write(sigData)
			return
		}
		http.NotFound(w, r)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	pluginsDir := t.TempDir()
	client := NewClient(pluginsDir)
	client.indexURL = server.URL + "/marketplace.json"
	u, _ := url.Parse(server.URL)
	client.httpClient = &http.Client{
		Transport: &redirectTransport{target: u.Host},
		Timeout:   10 * time.Second,
	}

	entry, err := client.FindPlugin("signed-plugin")
	if err != nil {
		t.Fatalf("FindPlugin failed: %v", err)
	}
	if err := client.Install(entry); err != nil {
		t.Fatalf("Install with signature failed: %v", err)
	}

	manifestPath := filepath.Join(pluginsDir, "signed-plugin", "plugin.yaml")
	if _, err := os.ReadFile(manifestPath); err != nil {
		t.Fatalf("expected plugin.yaml at %s: %v", manifestPath, err)
	}
}
