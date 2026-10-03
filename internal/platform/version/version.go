// Package version provides build-time version information for GoatFlow.
// These variables are set at build time via -ldflags.
package version

import (
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
)

// Build-time variables set via ldflags
var (
	// Version is the semantic version (e.g., "v0.5.1") or branch name if not a tagged build
	Version = "0.10.0"

	// GitCommit is the short git commit SHA
	GitCommit = "unknown"

	// GitBranch is the git branch name
	GitBranch = "unknown"

	// BuildDate is the build timestamp
	BuildDate = "unknown"
)

// Info contains structured version information.
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	GitBranch string `json:"git_branch"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

// GetInfo returns the current version info.
func GetInfo() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		GitBranch: GitBranch,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
	}
}

// String returns a human-readable version string.
// Format: "v0.5.1 (abc1234)" or "main (abc1234)" for non-tagged builds
func String() string {
	return fmt.Sprintf("%s (%s)", Version, GitCommit)
}

// Short returns just the version or branch name.
func Short() string {
	return Version
}

// Full returns the full version string with all details.
func Full() string {
	return fmt.Sprintf("%s (%s) built %s with %s", Version, GitCommit, BuildDate, runtime.Version())
}

// HostCompatible reports whether the running GoatFlow satisfies a plugin's
// minimum host version. An empty minimum always matches. A development build
// (Version "dev", empty, or any non-semver branch name) matches everything, so
// untagged builds never lock plugins out.
func HostCompatible(minVersion string) bool {
	if minVersion == "" {
		return true
	}
	host := canonical(Version)
	if !semver.IsValid(host) {
		return true
	}
	return semver.Compare(host, canonical(minVersion)) >= 0
}

// IncompatibleError reports a plugin version whose minimum GoatFlow version is
// newer than the running host.
type IncompatibleError struct {
	Plugin         string
	PluginVersion  string
	MinHostVersion string
	HostVersion    string
}

func (e *IncompatibleError) Error() string {
	name := e.Plugin
	if e.PluginVersion != "" {
		name += " v" + strings.TrimPrefix(e.PluginVersion, "v")
	}
	return fmt.Sprintf("%s requires GoatFlow >= %s, you have %s",
		name, strings.TrimPrefix(e.MinHostVersion, "v"), e.HostVersion)
}

// RequireHost returns an *IncompatibleError when the running GoatFlow does not
// satisfy minVersion (see HostCompatible), nil otherwise.
func RequireHost(plugin, pluginVersion, minVersion string) error {
	if HostCompatible(minVersion) {
		return nil
	}
	return &IncompatibleError{
		Plugin:         plugin,
		PluginVersion:  pluginVersion,
		MinHostVersion: minVersion,
		HostVersion:    Version,
	}
}

// canonical adds the "v" prefix golang.org/x/mod/semver requires.
func canonical(v string) string {
	if v == "" || strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}
