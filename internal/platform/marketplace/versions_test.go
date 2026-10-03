package marketplace

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/plugin/packaging"
	"github.com/goatkit/goatflow/internal/platform/plugin/signing"
	"github.com/goatkit/goatflow/internal/platform/version"
)

// setHostVersion pins the running GoatFlow version for one test.
func setHostVersion(t *testing.T, v string) {
	t.Helper()
	old := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = old })
}

// releaseServer serves plugin release assets at the GitHub download paths and
// records which ZIPs were requested.
type releaseServer struct {
	mu      sync.Mutex
	zips    map[string][]byte // "<version>" -> zip bytes
	sigs    map[string][]byte // "<version>" -> sig bytes
	fetched []string
}

func (s *releaseServer) handler(w http.ResponseWriter, r *http.Request) {
	// /<owner>/<repo>/releases/download/v<version>/<name>.zip[.sig]
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 7 {
		http.NotFound(w, r)
		return
	}
	ver := strings.TrimPrefix(parts[5], "v")
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case strings.HasSuffix(r.URL.Path, ".zip.sig"):
		if sig, ok := s.sigs[ver]; ok {
			_, _ = w.Write(sig)
			return
		}
	case strings.HasSuffix(r.URL.Path, ".zip"):
		s.fetched = append(s.fetched, ver)
		if zip, ok := s.zips[ver]; ok {
			_, _ = w.Write(zip)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *releaseServer) fetchedVersions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.fetched...)
}

// newReleaseClient returns a client whose downloads hit srv and whose index is
// entry, installing into a fresh plugins dir.
func newReleaseClient(t *testing.T, srv *releaseServer, entry PluginEntry) (*Client, string) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	t.Cleanup(ts.Close)
	u, _ := url.Parse(ts.URL)

	pluginsDir := t.TempDir()
	client := NewClient(pluginsDir)
	client.httpClient = &http.Client{Transport: &redirectTransport{target: u.Host}, Timeout: 10 * time.Second}
	client.index = &Index{Version: IndexVersion, Plugins: []PluginEntry{entry}}
	return client, pluginsDir
}

// pluginZip packages a template plugin name@ver with a marker file.
func pluginZip(t *testing.T, name, ver string) []byte {
	t.Helper()
	src := t.TempDir()
	manifest := fmt.Sprintf("name: %s\nversion: %s\nruntime: template\n", name, ver)
	if err := os.WriteFile(filepath.Join(src, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "v"+ver+".txt"), []byte(ver), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), name+".zip")
	if err := packaging.PackagePlugin(src, out); err != nil {
		t.Fatalf("PackagePlugin: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// installLocal writes an installed copy of name@ver with a marker file.
func installLocal(t *testing.T, pluginsDir, name, ver string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf("name: %s\nversion: %s\nruntime: template\n", name, ver)
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "v"+ver+".txt"), []byte(ver), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertInstalled checks name is installed at exactly ver: manifest version
// and that version's marker file, with no other version's marker left over.
func assertInstalled(t *testing.T, client *Client, pluginsDir, name, ver string) {
	t.Helper()
	if got := client.installedVersion(name); got != ver {
		t.Fatalf("installed version = %q, want %q", got, ver)
	}
	entries, err := os.ReadDir(filepath.Join(pluginsDir, name))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "v") && e.Name() != "v"+ver+".txt" {
			t.Errorf("stale file %s left from another version", e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, name, "v"+ver+".txt")); err != nil {
		t.Errorf("marker for v%s missing: %v", ver, err)
	}
	all, _ := os.ReadDir(pluginsDir)
	for _, e := range all {
		if strings.HasPrefix(e.Name(), ".gk-staging-") {
			t.Errorf("staging dir %s left behind", e.Name())
		}
	}
}

func TestUpdateRefusesIncompatibleVersion(t *testing.T) {
	setHostVersion(t, "0.10.0")
	srv := &releaseServer{zips: map[string][]byte{"2.0.0": pluginZip(t, "demo", "2.0.0")}}
	entry := PluginEntry{Name: "demo", Repo: "acme/demo", LatestVersion: "2.0.0", MinHostVersion: "0.11.0"}
	client, pluginsDir := newReleaseClient(t, srv, entry)
	installLocal(t, pluginsDir, "demo", "1.0.0")

	_, err := client.Update(&entry, "")
	var incompat *version.IncompatibleError
	if !errors.As(err, &incompat) {
		t.Fatalf("Update error = %v, want *version.IncompatibleError", err)
	}
	if want := "demo v2.0.0 requires GoatFlow >= 0.11.0, you have 0.10.0"; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
	if got := srv.fetchedVersions(); len(got) != 0 {
		t.Errorf("incompatible update downloaded %v", got)
	}
	assertInstalled(t, client, pluginsDir, "demo", "1.0.0")

	updates, err := client.CheckUpdates()
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 0 {
		t.Errorf("CheckUpdates offered %+v, want nothing (only incompatible versions are newer)", updates)
	}
}

func TestUpdateKeepsInstalledPluginWhenDownloadFails(t *testing.T) {
	setHostVersion(t, "0.10.0")
	pub, priv, err := signing.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _, err := signing.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	zip := pluginZip(t, "demo", "1.1.0")
	zipFile := filepath.Join(t.TempDir(), "demo.zip")
	if err := os.WriteFile(zipFile, zip, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := signing.SignBinary(zipFile, zipFile+".sig", priv); err != nil {
		t.Fatal(err)
	}
	sig, err := os.ReadFile(zipFile + ".sig")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		srv    *releaseServer
		pubKey []byte
	}{
		{"download 404", &releaseServer{}, pub},
		{"bad signature", &releaseServer{
			zips: map[string][]byte{"1.1.0": zip},
			sigs: map[string][]byte{"1.1.0": sig},
		}, otherPub},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := PluginEntry{Name: "demo", Repo: "acme/demo", LatestVersion: "1.1.0", PublicKey: hex.EncodeToString(tc.pubKey)}
			client, pluginsDir := newReleaseClient(t, tc.srv, entry)
			installLocal(t, pluginsDir, "demo", "1.0.0")

			if _, err := client.Update(&entry, ""); err == nil {
				t.Fatal("Update succeeded, want failure")
			}
			assertInstalled(t, client, pluginsDir, "demo", "1.0.0")
		})
	}

	t.Run("verified update swaps in new version", func(t *testing.T) {
		srv := &releaseServer{zips: map[string][]byte{"1.1.0": zip}, sigs: map[string][]byte{"1.1.0": sig}}
		entry := PluginEntry{Name: "demo", Repo: "acme/demo", LatestVersion: "1.1.0", PublicKey: hex.EncodeToString(pub)}
		client, pluginsDir := newReleaseClient(t, srv, entry)
		installLocal(t, pluginsDir, "demo", "1.0.0")

		got, err := client.Update(&entry, "")
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if got != "1.1.0" {
			t.Errorf("Update installed %q, want 1.1.0", got)
		}
		assertInstalled(t, client, pluginsDir, "demo", "1.1.0")
	})
}

func TestInstallPicksNewestCompatibleVersion(t *testing.T) {
	setHostVersion(t, "0.10.0")
	srv := &releaseServer{zips: map[string][]byte{
		"1.0.0": pluginZip(t, "demo", "1.0.0"),
		"2.0.0": pluginZip(t, "demo", "2.0.0"),
		"3.0.0": pluginZip(t, "demo", "3.0.0"),
	}}
	entry := PluginEntry{
		Name: "demo", Repo: "acme/demo", LatestVersion: "3.0.0", MinHostVersion: "0.11.0",
		Versions: []VersionEntry{
			{Version: "1.0.0"},
			{Version: "3.0.0", MinHostVersion: "0.11.0"},
			{Version: "2.0.0", MinHostVersion: "0.10.0"},
		},
	}
	client, pluginsDir := newReleaseClient(t, srv, entry)

	got, err := client.Install(&entry, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got != "2.0.0" {
		t.Errorf("Install picked %q, want 2.0.0", got)
	}
	assertInstalled(t, client, pluginsDir, "demo", "2.0.0")

	_, err = client.Install(&entry, "3.0.0")
	var incompat *version.IncompatibleError
	if !errors.As(err, &incompat) || err.Error() != "demo v3.0.0 requires GoatFlow >= 0.11.0, you have 0.10.0" {
		t.Errorf("Install 3.0.0 error = %v, want incompatible", err)
	}
	if _, err := client.Install(&entry, "9.9.9"); err == nil || !strings.Contains(err.Error(), "no version 9.9.9") {
		t.Errorf("Install unlisted version error = %v", err)
	}

	got, err = client.Install(&entry, "1.0.0")
	if err != nil {
		t.Fatalf("Install pinned 1.0.0: %v", err)
	}
	if got != "1.0.0" {
		t.Errorf("Install pinned returned %q", got)
	}
	assertInstalled(t, client, pluginsDir, "demo", "1.0.0")

	updates, err := client.CheckUpdates()
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].LatestVersion != "2.0.0" {
		t.Errorf("CheckUpdates = %+v, want one update to 2.0.0 (3.0.0 needs a newer host)", updates)
	}
	if want := []string{"2.0.0", "1.0.0"}; strings.Join(srv.fetchedVersions(), ",") != strings.Join(want, ",") {
		t.Errorf("downloaded %v, want %v", srv.fetchedVersions(), want)
	}
}

func TestEntryWithoutVersionsIsSingleVersion(t *testing.T) {
	setHostVersion(t, "0.10.0")
	// The index shape published before versions existed.
	const raw = `{"version":1,"plugins":[{"name":"demo","repo":"acme/demo","latest_version":"1.2.0","min_host_version":"0.8.0","runtime":"template"}]}`
	var idx Index
	if err := json.Unmarshal([]byte(raw), &idx); err != nil {
		t.Fatal(err)
	}
	entry := idx.Plugins[0]
	if got := entry.AllVersions(); len(got) != 1 || got[0].Version != "1.2.0" || got[0].MinHostVersion != "0.8.0" {
		t.Fatalf("AllVersions = %+v, want [{1.2.0 0.8.0}]", got)
	}

	srv := &releaseServer{zips: map[string][]byte{"1.2.0": pluginZip(t, "demo", "1.2.0")}}
	client, pluginsDir := newReleaseClient(t, srv, entry)
	got, err := client.Install(&entry, "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got != "1.2.0" {
		t.Errorf("Install picked %q", got)
	}
	assertInstalled(t, client, pluginsDir, "demo", "1.2.0")

	entry.MinHostVersion = "0.11.0"
	if _, err := client.Install(&entry, ""); err == nil || err.Error() != "demo v1.2.0 requires GoatFlow >= 0.11.0, you have 0.10.0" {
		t.Errorf("Install with too-new min_host_version error = %v", err)
	}

	setHostVersion(t, "dev")
	if _, err := entry.ResolveInstall(); err != nil {
		t.Errorf("dev build must bypass min_host_version, got %v", err)
	}
}
