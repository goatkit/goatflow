// Package marketplace provides plugin discovery, installation, and update
// functionality using GitHub Releases as the distribution backend.
package marketplace

import (
	"time"
)

// IndexVersion is the current marketplace index schema version.
const IndexVersion = 1

// DefaultIndexURL is the raw GitHub URL for the marketplace index.
const DefaultIndexURL = "https://raw.githubusercontent.com/goatkit/marketplace/main/marketplace.json"

// Index represents the marketplace.json index file.
type Index struct {
	Version   int            `json:"version"`
	UpdatedAt time.Time      `json:"updated_at"`
	Plugins   []PluginEntry  `json:"plugins"`
}

// PluginEntry represents a plugin listing in the marketplace.
type PluginEntry struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Author         string   `json:"author"`
	Licence        string   `json:"licence"`
	Homepage       string   `json:"homepage"`
	Repo           string   `json:"repo"`             // GitHub owner/repo
	Category       string   `json:"category"`         // business, integration, theme, utility
	Tags           []string `json:"tags"`
	LatestVersion  string   `json:"latest_version"`
	MinHostVersion string   `json:"min_host_version"`
	Runtime        string   `json:"runtime"`          // wasm, grpc
	Verified       bool     `json:"verified"`         // has ed25519 signature
	Dependencies   []string `json:"dependencies,omitempty"` // plugin names this depends on
}

// InstalledPlugin represents a locally installed plugin read from its manifest.
type InstalledPlugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Runtime string `json:"runtime"`
	Path    string `json:"path"` // directory path
}

// UpdateAvailable represents a plugin with a newer version in the marketplace.
type UpdateAvailable struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	Repo           string `json:"repo"`
}
