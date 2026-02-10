package plugin

// PluginManifest represents a plugin.yaml manifest file.
// This is the universal plugin descriptor used by both the loader and packaging systems.
type PluginManifest struct {
	Name        string           `yaml:"name"                   json:"name"`
	Version     string           `yaml:"version"                json:"version"`
	Runtime     string           `yaml:"runtime"                json:"runtime"`               // "wasm", "grpc", or "template"
	Binary      string           `yaml:"binary"                 json:"binary,omitempty"`       // For grpc runtime: relative path to executable
	WASMFile    string           `yaml:"wasm,omitempty"         json:"wasm,omitempty"`         // For wasm runtime: defaults to name.wasm
	Description string           `yaml:"description,omitempty"  json:"description,omitempty"`
	Author      string           `yaml:"author,omitempty"       json:"author,omitempty"`
	License     string           `yaml:"license,omitempty"      json:"license,omitempty"`
	Homepage    string           `yaml:"homepage,omitempty"     json:"homepage,omitempty"`
	Resources   *ResourceRequest `yaml:"resources,omitempty"    json:"resources,omitempty"`
}
