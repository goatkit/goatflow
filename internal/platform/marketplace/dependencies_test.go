package marketplace

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/plugin/signing"
	"github.com/goatkit/goatflow/internal/platform/version"
)

// multiReleaseServer serves "<name>@<version>" release ZIPs and records the
// order they were downloaded in.
type multiReleaseServer struct {
	mu      sync.Mutex
	zips    map[string][]byte
	fetched []string
}

func (s *multiReleaseServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// /<owner>/<repo>/releases/download/v<version>/<name>.zip
	if !strings.HasSuffix(r.URL.Path, ".zip") {
		http.NotFound(w, r)
		return
	}
	ref := strings.TrimSuffix(path.Base(r.URL.Path), ".zip") + "@" + strings.TrimPrefix(path.Base(path.Dir(r.URL.Path)), "v")
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetched = append(s.fetched, ref)
	if zip, ok := s.zips[ref]; ok {
		_, _ = w.Write(zip)
		return
	}
	http.NotFound(w, r)
}

func newDependencyClient(t *testing.T, srv *multiReleaseServer, plugins ...PluginEntry) (*Client, string) {
	t.Helper()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	u, _ := url.Parse(ts.URL)
	pluginsDir := t.TempDir()
	client := NewClient(pluginsDir)
	client.verifier = signing.Verifier{} // unsigned fixtures; signatures are tested in versions_test.go
	client.httpClient = &http.Client{Transport: &redirectTransport{target: u.Host}, Timeout: 10 * time.Second}
	client.index = &Index{Version: IndexVersion, Plugins: plugins}
	return client, pluginsDir
}

func depEntry(name, ver string, deps ...string) PluginEntry {
	return PluginEntry{Name: name, Repo: "acme/" + name, LatestVersion: ver, Dependencies: deps}
}

func stepNames(steps []DependencyStep) string {
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.Entry.Name+"@"+s.Version.Version+"<"+s.RequiredBy)
	}
	return strings.Join(names, ",")
}

func TestPlanAndInstallDependencies(t *testing.T) {
	setHostVersion(t, "0.10.0")
	// app needs lib-a and lib-b, which both need core; audit is installed.
	srv := &multiReleaseServer{zips: map[string][]byte{
		"core@1.0.0":  pluginZip(t, "core", "1.0.0"),
		"lib-a@2.0.0": pluginZip(t, "lib-a", "2.0.0"),
		"lib-b@1.1.0": pluginZip(t, "lib-b", "1.1.0"),
	}}
	app := depEntry("app", "1.0.0", "lib-a", "audit", "lib-b")
	client, pluginsDir := newDependencyClient(t, srv,
		app,
		depEntry("lib-a", "2.0.0", "core"),
		depEntry("lib-b", "1.1.0", "core"),
		depEntry("core", "1.0.0"),
		depEntry("audit", "3.0.0"),
	)
	installLocal(t, pluginsDir, "audit", "1.0.0")

	steps, err := client.PlanDependencies(&app)
	if err != nil {
		t.Fatalf("PlanDependencies: %v", err)
	}
	if got, want := stepNames(steps), "core@1.0.0<lib-a,lib-a@2.0.0<app,lib-b@1.1.0<app"; got != want {
		t.Fatalf("plan = %s, want %s (installed audit must not be reinstalled or upgraded)", got, want)
	}

	done, err := client.InstallDependencies(steps)
	if err != nil {
		t.Fatalf("InstallDependencies: %v", err)
	}
	if len(done) != 3 {
		t.Fatalf("installed %d dependencies, want 3", len(done))
	}
	if got, want := strings.Join(srv.fetched, ","), "core@1.0.0,lib-a@2.0.0,lib-b@1.1.0"; got != want {
		t.Errorf("downloaded %s, want %s", got, want)
	}
	assertInstalled(t, client, pluginsDir, "core", "1.0.0")
	assertInstalled(t, client, pluginsDir, "lib-a", "2.0.0")
	assertInstalled(t, client, pluginsDir, "lib-b", "1.1.0")
	assertInstalled(t, client, pluginsDir, "audit", "1.0.0")

	again, err := client.PlanDependencies(&app)
	if err != nil || len(again) != 0 {
		t.Errorf("plan after install = %s, %v; want nothing", stepNames(again), err)
	}
}

func TestPlanDependenciesRefusesBeforeDownloading(t *testing.T) {
	setHostVersion(t, "0.10.0")
	cases := []struct {
		name    string
		plugins []PluginEntry
		want    string
	}{
		{"not in marketplace", []PluginEntry{depEntry("app", "1.0.0", "lib"), depEntry("lib", "1.0.0", "ghost")},
			`lib needs plugin "ghost", which is not in the marketplace`},
		{"cycle", []PluginEntry{depEntry("app", "1.0.0", "lib"), depEntry("lib", "1.0.0", "core"), depEntry("core", "1.0.0", "app")},
			"circular plugin dependency: app -> lib -> core -> app"},
		{"dependency needs newer GoatFlow", []PluginEntry{depEntry("app", "1.0.0", "lib"),
			{Name: "lib", Repo: "acme/lib", LatestVersion: "2.0.0", MinHostVersion: "0.11.0"}},
			"app needs plugin lib: lib v2.0.0 requires GoatFlow >= 0.11.0, you have 0.10.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &multiReleaseServer{}
			client, _ := newDependencyClient(t, srv, tc.plugins...)
			steps, err := client.PlanDependencies(&tc.plugins[0])
			if err == nil || err.Error() != tc.want {
				t.Fatalf("PlanDependencies = %s, %v; want error %q", stepNames(steps), err, tc.want)
			}
			if len(srv.fetched) != 0 {
				t.Errorf("downloaded %v before refusing", srv.fetched)
			}
		})
	}

	t.Run("incompatible error stays typed", func(t *testing.T) {
		plugins := []PluginEntry{depEntry("app", "1.0.0", "lib"), {Name: "lib", Repo: "acme/lib", LatestVersion: "2.0.0", MinHostVersion: "0.11.0"}}
		client, _ := newDependencyClient(t, &multiReleaseServer{}, plugins...)
		_, err := client.PlanDependencies(&plugins[0])
		var incompat *version.IncompatibleError
		if !errors.As(err, &incompat) {
			t.Errorf("error %v is not a *version.IncompatibleError", err)
		}
	})
}

func TestInstallDependenciesStopsAtFirstFailure(t *testing.T) {
	setHostVersion(t, "0.10.0")
	srv := &multiReleaseServer{zips: map[string][]byte{"core@1.0.0": pluginZip(t, "core", "1.0.0")}}
	app := depEntry("app", "1.0.0", "core", "lib")
	client, pluginsDir := newDependencyClient(t, srv, app, depEntry("core", "1.0.0"), depEntry("lib", "1.0.0"), depEntry("tail", "1.0.0"))
	steps, err := client.PlanDependencies(&app)
	if err != nil {
		t.Fatal(err)
	}
	steps = append(steps, DependencyStep{Entry: &client.index.Plugins[3], Version: VersionEntry{Version: "1.0.0"}, RequiredBy: "app"})

	done, err := client.InstallDependencies(steps)
	if err == nil || !strings.HasPrefix(err.Error(), "install dependency lib v1.0.0 (needed by app): ") {
		t.Fatalf("error = %v, want lib download failure", err)
	}
	if len(done) != 1 || done[0].Entry.Name != "core" {
		t.Errorf("done = %s, want core only", stepNames(done))
	}
	assertInstalled(t, client, pluginsDir, "core", "1.0.0")
	if got := strings.Join(srv.fetched, ","); got != "core@1.0.0,lib@1.0.0" {
		t.Errorf("downloaded %s; nothing after the failure should be fetched", got)
	}
}
