package marketplace

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

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
		if matchesQuery(p, query) {
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
		if entry.LatestVersion != inst.Version {
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

// DownloadURL returns the GitHub Release download URL for a plugin version.
func DownloadURL(repo, version, pluginName string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s.zip", repo, version, pluginName)
}

// SignatureURL returns the signature file URL for a plugin version.
func SignatureURL(repo, version, pluginName string) string {
	return DownloadURL(repo, version, pluginName) + ".sig"
}

func matchesQuery(p PluginEntry, query string) bool {
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
