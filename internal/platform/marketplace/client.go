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
	"github.com/goatkit/goatflow/internal/platform/version"
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
		data, err := os.ReadFile(manifestPath)
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

// CheckUpdates compares installed plugins against the marketplace index.
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
		if versionCompare(entry.LatestVersion, inst.Version) > 0 {
			updates = append(updates, UpdateAvailable{
				Name:           inst.Name,
				CurrentVersion: inst.Version,
				LatestVersion:  entry.LatestVersion,
				Repo:           entry.Repo,
			})
		}
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

// Install downloads, verifies, and extracts a plugin from the marketplace.
// Returns ErrAlreadyInstalled if the plugin is already at the requested version.
func (c *Client) Install(entry *PluginEntry) error {
	if entry.MinHostVersion != "" {
		hostVer := version.Short()
		if hostVer != "dev" && hostVer != "" {
			if versionCompare(hostVer, entry.MinHostVersion) < 0 {
				return fmt.Errorf("plugin requires GoatFlow >= %s, you have %s", entry.MinHostVersion, hostVer)
			}
		}
	}

	installed, _ := c.ListInstalled()
	for _, inst := range installed {
		if inst.Name == entry.Name && versionCompare(inst.Version, entry.LatestVersion) == 0 {
			return ErrAlreadyInstalled
		}
	}

	return c.fetchAndExtract(entry)
}

// Update replaces an installed plugin with the marketplace version.
// The existing plugin directory is removed before extraction for a clean replacement.
func (c *Client) Update(entry *PluginEntry) error {
	pluginDir := filepath.Join(c.pluginsDir, entry.Name)
	_ = os.RemoveAll(pluginDir)

	return c.fetchAndExtract(entry)
}

// fetchAndExtract downloads a plugin ZIP, verifies its signature if available,
// and extracts it to the plugins directory.
func (c *Client) fetchAndExtract(entry *PluginEntry) error {
	zipURL := DownloadURL(entry.Repo, entry.LatestVersion, entry.Name)
	zipFile, err := os.CreateTemp("", "gk-plugin-*.zip")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	zipPath := zipFile.Name()
	defer os.Remove(zipPath)

	resp, err := c.httpClient.Get(zipURL)
	if err != nil {
		zipFile.Close()
		return fmt.Errorf("download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		zipFile.Close()
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(zipFile, resp.Body); err != nil {
		zipFile.Close()
		return fmt.Errorf("write plugin zip: %w", err)
	}
	zipFile.Close()

	// Verify signature if a .sig file is available.
	sigURL := SignatureURL(entry.Repo, entry.LatestVersion, entry.Name)
	sigResp, sigErr := c.httpClient.Get(sigURL)
	hasSig := sigErr == nil && sigResp != nil && sigResp.StatusCode == http.StatusOK
	if !hasSig {
		if sigResp != nil {
			sigResp.Body.Close()
		}
		if signing.IsSignatureRequired() {
			return fmt.Errorf("plugin %q is not signed but signatures are required (GOATFLOW_REQUIRE_SIGNATURES=1)", entry.Name)
		}
	} else {
		sigFile, err := os.CreateTemp("", "gk-plugin-*.sig")
		if err != nil {
			sigResp.Body.Close()
			return fmt.Errorf("create temp sig file: %w", err)
		}
		sigPath := sigFile.Name()
		defer os.Remove(sigPath)

		if _, err := io.Copy(sigFile, sigResp.Body); err != nil {
			sigFile.Close()
			sigResp.Body.Close()
			return fmt.Errorf("write signature: %w", err)
		}
		sigFile.Close()
		sigResp.Body.Close()

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
		if len(keys) > 0 {
			if err := signing.VerifyBinary(zipPath, sigPath, keys); err != nil {
				return fmt.Errorf("signature verification failed: %w", err)
			}
		} else {
			fmt.Fprintln(os.Stderr, "Warning: signature file exists but no trusted keys configured — skipping verification")
		}
	}

	// Extract plugin package.
	pkg, err := packaging.ExtractPlugin(zipPath, c.pluginsDir)
	if err != nil {
		return fmt.Errorf("extract plugin: %w", err)
	}

	// Install theme assets if applicable.
	pluginDir := filepath.Join(c.pluginsDir, pkg.Manifest.Name)
	if IsThemePlugin(&pkg.Manifest) {
		if err := InstallTheme(pluginDir, &pkg.Manifest); err != nil {
			return fmt.Errorf("install theme: %w", err)
		}
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
