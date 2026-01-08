// Package version provides build-time version information for GOTRS.
// These variables are set at build time via -ldflags.
package version

import (
	"fmt"
	"runtime"
)

// Build-time variables set via ldflags
var (
	// Version is the semantic version (e.g., "v0.5.1") or branch name if not a tagged build
	Version = "dev"

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
