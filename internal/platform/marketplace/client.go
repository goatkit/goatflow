package marketplace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"

	"github.com/goatkit/goatflow/internal/platform/plugin/packaging"
	"github.com/goatkit/goatflow/internal/platform/plugin/signing"
	"github.com/goatkit/goatflow/pkg/plugin"
)

// Client provides marketplace operations: fetch index, install, update, search.
type Client struct {
	indexURL   string
	pluginsDir string
	httpClient *http.Client
	index      *Index
}

// NewClient creates a marketplace client.
func NewClient(pluginsDir string) *Client {
	indexURL := os.Getenv("GOATFLOW_MARKETPLACE_URL")
	if indexURL == "" {
		indexURL = DefaultIndexURL
	}
	return &Client{
		indexURL:   indexURL,
		pluginsDir: pluginsDir,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchIndex downloads and parses the marketplace index.
func (c *Client) FetchIndex() (*Index, error) {
	if c.index != nil {
		return c.index, nil
	}

	resp, err := c.httpClient.Get(c.indexURL)
	if err != nil {
		return nil, fmt.Errorf("fetch marketplace index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("marketplace index returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read marketplace index: %w", err)
	}

	var index Index
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, fmt.Errorf("parse marketplace index: %w", err)
	}

	c.index = &index
	return &index, nil
}

// Search finds plugins matching a query string (name, description, or tags).
func (c *Client) Search(query string) ([]PluginEntry, error) {
	index, err := c.FetchIndex()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []PluginEntry
	for _, p := range index.Plugins {
		if MatchesQuery(p, query) {
			results = append(results, p)
		}
	}
	return results, nil
}

// FindPlugin looks up a specific plugin by name.
func (c *Client) FindPlugin(name string) (*PluginEntry, error) {
	index, err := c.FetchIndex()
	if err != nil {
		return nil, err
	}

	for _, p := range index.Plugins {
		if strings.EqualFold(p.Name, name) {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("plugin %q not found in marketplace", name)
}

// ListInstalled reads all installed plugins from the plugins directory.
func (c *Client) ListInstalled() ([]InstalledPlugin, error) {
	entries, err := os.ReadDir(c.pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read plugins dir: %w", err)
	}

	var installed []InstalledPlugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(c.pluginsDir, entry.Name(), "plugin.yaml")
		data, err := os.ReadFile(manifestPath) // #nosec G304 -- entry under the operator-configured plugins dir, name from os.ReadDir
		if err != nil {
			continue // Not a plugin directory
		}

		var manifest plugin.PluginManifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			continue
		}

		installed = append(installed, InstalledPlugin{
			Name:    manifest.Name,
			Version: manifest.Version,
			Runtime: manifest.Runtime,
			Path:    filepath.Join(c.pluginsDir, entry.Name()),
		})
	}
	return installed, nil
}

// CheckUpdates compares installed plugins against the marketplace index and
// reports, per plugin, the newest version this GoatFlow can run that is newer
// than the installed one. Newer releases needing a newer GoatFlow are skipped.
func (c *Client) CheckUpdates() ([]UpdateAvailable, error) {
	installed, err := c.ListInstalled()
	if err != nil {
		return nil, err
	}

	index, err := c.FetchIndex()
	if err != nil {
		return nil, err
	}

	// Build lookup map.
	marketMap := make(map[string]*PluginEntry, len(index.Plugins))
	for i := range index.Plugins {
		marketMap[index.Plugins[i].Name] = &index.Plugins[i]
	}

	var updates []UpdateAvailable
	for _, inst := range installed {
		entry, ok := marketMap[inst.Name]
		if !ok {
			continue
		}
		next, err := entry.ResolveUpdate(inst.Version)
		if err != nil {
			continue
		}
		updates = append(updates, UpdateAvailable{
			Name:           inst.Name,
			CurrentVersion: inst.Version,
			LatestVersion:  next.Version,
			Repo:           entry.Repo,
		})
	}
	return updates, nil
}

// ErrAlreadyInstalled indicates a plugin is already installed at the requested version.
var ErrAlreadyInstalled = errors.New("plugin already installed at the requested version")

// ensureVersionPrefix adds a "v" prefix to a version string if missing.
// golang.org/x/mod/semver requires the "v" prefix.
func ensureVersionPrefix(v string) string {
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// versionCompare returns -1, 0, or 1 comparing a vs b using semver.
func versionCompare(a, b string) int {
	return semver.Compare(ensureVersionPrefix(a), ensureVersionPrefix(b))
}

// CompareVersions returns -1, 0, or 1 comparing a vs b using semver.
// Exported for use by API handlers that need to check for updates.
func CompareVersions(a, b string) int {
	return versionCompare(a, b)
}

// installedVersion returns the installed version of name, or "" if absent.
func (c *Client) installedVersion(name string) string {
	installed, _ := c.ListInstalled()
	for _, inst := range installed {
		if inst.Name == name {
			return inst.Version
		}
	}
	return ""
}

// Install downloads, verifies, and extracts a plugin from the marketplace.
// want selects a listed version; "" picks the newest version this GoatFlow
// can run. Returns the version installed, or ErrAlreadyInstalled if the plugin
// is already at that version.
func (c *Client) Install(entry *PluginEntry, want string) (string, error) {
	v, err := resolveWanted(entry, want)
	if err != nil {
		return "", err
	}
	if cur := c.installedVersion(entry.Name); cur != "" && versionCompare(cur, v.Version) == 0 {
		return v.Version, ErrAlreadyInstalled
	}
	return v.Version, c.installVersion(entry, v.Version)
}

// Update replaces an installed plugin with another marketplace version. want
// selects a listed version; "" picks the newest compatible version newer than
// the installed one (ErrNoUpdate if there is none). The new version is
// downloaded and verified before the installed one is touched, so a failed
// update leaves the existing plugin in place.
func (c *Client) Update(entry *PluginEntry, want string) (string, error) {
	var v VersionEntry
	var err error
	if want == "" {
		v, err = entry.ResolveUpdate(c.installedVersion(entry.Name))
	} else {
		v, err = entry.ResolveVersion(want)
	}
	if err != nil {
		return "", err
	}
	return v.Version, c.installVersion(entry, v.Version)
}

func resolveWanted(entry *PluginEntry, want string) (VersionEntry, error) {
	if want == "" {
		return entry.ResolveInstall()
	}
	return entry.ResolveVersion(want)
}

// installVersion downloads and verifies ver, extracts it into a staging
// directory inside the plugins dir, then swaps it into place. Any existing
// install is only moved aside once the new one is fully staged, and is put
// back if the swap fails.
func (c *Client) installVersion(entry *PluginEntry, ver string) error {
	zipPath, cleanup, err := c.download(entry, ver)
	defer cleanup()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(c.pluginsDir, 0o750); err != nil {
		return fmt.Errorf("create plugins dir: %w", err)
	}
	// Dot-prefixed so the loaders' directory scans skip it.
	staging, err := os.MkdirTemp(c.pluginsDir, ".gk-staging-*")
	if err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	pkg, err := packaging.ExtractPlugin(zipPath, filepath.Join(staging, "new"))
	if err != nil {
		return fmt.Errorf("extract plugin: %w", err)
	}
	if pkg.Manifest.Name != entry.Name {
		return fmt.Errorf("package is plugin %q, marketplace entry is %q", pkg.Manifest.Name, entry.Name)
	}

	pluginDir := filepath.Join(c.pluginsDir, entry.Name)
	if err := swapDir(filepath.Join(staging, "new", entry.Name), pluginDir, filepath.Join(staging, "old")); err != nil {
		return err
	}

	// Install theme assets if applicable.
	if IsThemePlugin(&pkg.Manifest) {
		if err := InstallTheme(pluginDir, &pkg.Manifest); err != nil {
			return fmt.Errorf("install theme: %w", err)
		}
	}
	return nil
}

// swapDir moves src to dst. An existing dst is first renamed to aside and is
// restored if moving src in fails; the caller removes aside afterwards.
func swapDir(src, dst, aside string) error {
	hadOld := false
	if _, err := os.Lstat(dst); err == nil {
		if err := os.Rename(dst, aside); err != nil {
			return fmt.Errorf("move installed plugin aside: %w", err)
		}
		hadOld = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat installed plugin: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		if hadOld {
			if rerr := os.Rename(aside, dst); rerr != nil {
				return fmt.Errorf("move new plugin into place: %w (restoring previous version failed: %v)", err, rerr)
			}
		}
		return fmt.Errorf("move new plugin into place: %w", err)
	}
	return nil
}

// download fetches the plugin ZIP for ver into a temp file and verifies its
// signature when one is published. cleanup removes the temp file and is safe
// to call even when err is non-nil.
func (c *Client) download(entry *PluginEntry, ver string) (zipPath string, cleanup func(), err error) {
	cleanup = func() {}
	zipFile, err := os.CreateTemp("", "gk-plugin-*.zip")
	if err != nil {
		return "", cleanup, fmt.Errorf("create temp file: %w", err)
	}
	zipPath = zipFile.Name()
	cleanup = func() { _ = os.Remove(zipPath) }

	resp, err := c.httpClient.Get(DownloadURL(entry.Repo, ver, entry.Name))
	if err != nil {
		_ = zipFile.Close()
		return "", cleanup, fmt.Errorf("download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_ = zipFile.Close()
		return "", cleanup, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(zipFile, resp.Body); err != nil {
		_ = zipFile.Close()
		return "", cleanup, fmt.Errorf("write plugin zip: %w", err)
	}
	if err := zipFile.Close(); err != nil {
		return "", cleanup, fmt.Errorf("write plugin zip: %w", err)
	}

	if err := c.verifySignature(entry, ver, zipPath); err != nil {
		return "", cleanup, err
	}
	return zipPath, cleanup, nil
}

// verifySignature checks zipPath against the published .sig for ver if one
// exists; an unsigned plugin is refused when signatures are required.
func (c *Client) verifySignature(entry *PluginEntry, ver, zipPath string) error {
	sigResp, sigErr := c.httpClient.Get(SignatureURL(entry.Repo, ver, entry.Name))
	hasSig := sigErr == nil && sigResp != nil && sigResp.StatusCode == http.StatusOK
	if !hasSig {
		if sigResp != nil {
			_ = sigResp.Body.Close()
		}
		if signing.IsSignatureRequired() {
			return fmt.Errorf("plugin %q is not signed but signatures are required (GOATFLOW_REQUIRE_SIGNATURES=1)", entry.Name)
		}
		return nil
	}

	sigFile, err := os.CreateTemp("", "gk-plugin-*.sig")
	if err != nil {
		_ = sigResp.Body.Close()
		return fmt.Errorf("create temp sig file: %w", err)
	}
	sigPath := sigFile.Name()
	defer os.Remove(sigPath)

	if _, err := io.Copy(sigFile, sigResp.Body); err != nil {
		_ = sigFile.Close()
		_ = sigResp.Body.Close()
		return fmt.Errorf("write signature: %w", err)
	}
	_ = sigResp.Body.Close()
	if err := sigFile.Close(); err != nil {
		return fmt.Errorf("write signature: %w", err)
	}

	keys, err := LoadTrustedKeys()
	if err != nil {
		return fmt.Errorf("load trusted keys: %w", err)
	}
	if entry.PublicKey != "" {
		indexKey, err := parsePublicKey(entry.PublicKey)
		if err != nil {
			return fmt.Errorf("invalid public key in marketplace index: %w", err)
		}
		keys = append(keys, indexKey)
	}
	if len(keys) == 0 {
		fmt.Fprintln(os.Stderr, "Warning: signature file exists but no trusted keys configured — skipping verification")
		return nil
	}
	if err := signing.VerifyBinary(zipPath, sigPath, keys); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	return nil
}

// DownloadURL returns the GitHub Release download URL for a plugin version.
func DownloadURL(repo, version, pluginName string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s.zip", repo, version, pluginName)
}

// SignatureURL returns the signature file URL for a plugin version.
func SignatureURL(repo, version, pluginName string) string {
	return DownloadURL(repo, version, pluginName) + ".sig"
}

func MatchesQuery(p PluginEntry, query string) bool {
	if strings.Contains(strings.ToLower(p.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(p.Description), query) {
		return true
	}
	for _, tag := range p.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
