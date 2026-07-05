package plugin

// PluginManifest represents a plugin.yaml manifest file.
// This is the universal plugin descriptor used by both the loader and packaging systems.
type PluginManifest struct {
	Name         string           `yaml:"name"                   json:"name"`
	Version      string           `yaml:"version"                json:"version"`
	Runtime      string           `yaml:"runtime"                json:"runtime"`          // "wasm", "grpc", "template", or "theme"
	Binary       string           `yaml:"binary"                 json:"binary,omitempty"` // For grpc runtime: relative path to executable
	WASMFile     string           `yaml:"wasm,omitempty"         json:"wasm,omitempty"`   // For wasm runtime: defaults to name.wasm
	Icon         string           `yaml:"icon,omitempty"         json:"icon,omitempty"`   // SVG icon content or image URL (inline SVG content, FontAwesome class, or bundled icon path)
	Description  string           `yaml:"description,omitempty"  json:"description,omitempty"`
	Author       string           `yaml:"author,omitempty"       json:"author,omitempty"`
	License      string           `yaml:"license,omitempty"      json:"license,omitempty"`
	Homepage     string           `yaml:"homepage,omitempty"     json:"homepage,omitempty"`
	Resources    *ResourceRequest `yaml:"resources,omitempty"    json:"resources,omitempty"`
	Dependencies []string         `yaml:"dependencies,omitempty" json:"dependencies,omitempty"` // Plugin names this depends on
	PluginType   string           `yaml:"type,omitempty"         json:"type,omitempty"`         // "plugin" (default), "theme"
	Sidecars     []SidecarSpec    `yaml:"sidecars,omitempty"     json:"sidecars,omitempty"`     // Sidecar containers (gRPC plugins only)
}

// SidecarSpec declares a sidecar container that a plugin requires.
// In K8s mode, sidecars are injected into the plugin's pod spec.
// In Docker Compose mode, they're added as linked services on the same network.
type SidecarSpec struct {
	Name        string            `yaml:"name"                   json:"name"`                 // Container name (e.g. "adb-server")
	Image       string            `yaml:"image"                  json:"image"`                // Container image (e.g. "goatkit/adb-server:latest")
	Ports       []string          `yaml:"ports,omitempty"        json:"ports,omitempty"`      // Exposed ports (e.g. ["5037"])
	Env         map[string]string `yaml:"env,omitempty"          json:"env,omitempty"`        // Environment variables
	Volumes     []string          `yaml:"volumes,omitempty"      json:"volumes,omitempty"`    // Volume mounts (e.g. ["/dev/bus/usb:/dev/bus/usb"])
	Privileged  bool              `yaml:"privileged,omitempty"   json:"privileged,omitempty"` // Run with elevated privileges (e.g. USB access)
	MemoryMB    int               `yaml:"memory_mb,omitempty"    json:"memory_mb,omitempty"`  // Memory limit (0 = default 128Mi)
	CPUMillis   int               `yaml:"cpu_millis,omitempty"   json:"cpu_millis,omitempty"` // CPU limit in millicores (0 = default 200m)
	Healthcheck *SidecarHealth    `yaml:"healthcheck,omitempty"  json:"healthcheck,omitempty"`
}

// SidecarHealth defines a health check for a sidecar container.
type SidecarHealth struct {
	Command  []string `yaml:"command"            json:"command"`            // Health check command
	Interval string   `yaml:"interval,omitempty" json:"interval,omitempty"` // Check interval (default: "10s")
}
