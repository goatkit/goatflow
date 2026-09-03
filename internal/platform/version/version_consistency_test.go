package version_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/version"
)

// TestVersionConsistency is the DRY guardrail for release version bumps.
//
// A release used to mean editing ~8 files by hand (version.go, Helm chart,
// TrueNAS app.yaml + ix_values.yaml, changelog, roadmap, README). This test
// fails the build/CI the moment any committed pin drifts from the version
// constant, so a partial bump can never ship silently.
//
// It enforces:
//  1. The version constant is a semver (rejects branch names / "dev").
//  2. charts/goatflow/Chart.yaml appVersion == constant
//  3. docs/truenas-app/goatflow/app.yaml app_version == constant
//  4. docs/truenas-app/goatflow/ix_values.yaml goatflow image tag == constant
//  5. The latest dated CHANGELOG entry == constant, and a [Unreleased]
//     section still exists above it (fresh releases start there).
//  6. ROADMAP.md and README.md no longer advertise an older "Current" version.
//
// Run locally:  go test ./internal/platform/version/...
// It runs in CI as part of the standard unit suite.
func TestVersionConsistency(t *testing.T) {
	repoRoot := repoRoot(t)
	want := version.Version

	t.Run("version constant is semver", func(t *testing.T) {
		if !semverRe.MatchString(want) {
			t.Errorf("version.Version = %q: not a plain semver (expected like 0.10.0)", want)
		}
	})

	t.Run("Chart.yaml appVersion", func(t *testing.T) {
		got := yamlScalar(t, filepath.Join(repoRoot, "charts/goatflow/Chart.yaml"), `^appVersion:\s*"([^"]*)"\s*$`)
		assertPin(t, "charts/goatflow/Chart.yaml", "appVersion", got, want)
	})

	t.Run("TrueNAS app.yaml app_version", func(t *testing.T) {
		got := yamlScalar(t, filepath.Join(repoRoot, "docs/truenas-app/goatflow/app.yaml"), `^app_version:\s*(\S+)`)
		assertPin(t, "docs/truenas-app/goatflow/app.yaml", "app_version", got, want)
	})

	t.Run("TrueNAS ix_values.yaml image tag", func(t *testing.T) {
		// Only the goatflow image pin — the valkey image has its own tag.
		ix, err := os.ReadFile(filepath.Join(repoRoot, "docs/truenas-app/goatflow/ix_values.yaml"))
		if err != nil {
			t.Fatalf("read ix_values.yaml: %v", err)
		}
		var got string
		inGoatflow := false
		for _, line := range strings.Split(string(ix), "\n") {
			if strings.Contains(line, "ghcr.io/goatkit/goatflow") {
				inGoatflow = true
			}
			if inGoatflow {
				if m := ixTagRe.FindStringSubmatch(line); m != nil {
					got = m[1]
					break
				}
			}
		}
		if got == "" {
			t.Fatalf("ix_values.yaml: could not find image tag under ghcr.io/goatkit/goatflow")
		}
		assertPin(t, "docs/truenas-app/goatflow/ix_values.yaml", "image tag", got, want)
	})

	t.Run("CHANGELOG top dated entry version", func(t *testing.T) {
		cl, err := os.ReadFile(filepath.Join(repoRoot, "CHANGELOG.md"))
		if err != nil {
			t.Fatalf("read CHANGELOG.md: %v", err)
		}
		lines := strings.Split(string(cl), "\n")

		// A section where new work lands must exist somewhere: '## [Unreleased]'
		// or '## [X.Y.Z] - Unreleased'.
		hasUnreleased := false
		for _, l := range lines {
			if strings.HasPrefix(l, "## [Unreleased]") || clUnreleasedTagged.MatchString(l) {
				hasUnreleased = true
				break
			}
		}
		if !hasUnreleased {
			t.Errorf("CHANGELOG.md: no '## [Unreleased]' section found — new work should start there")
		}

		// Invariant 1: when the in-development version is labelled in the
		// changelog ('## [0.11.0] - Unreleased'), it must equal version.go.
		for _, l := range lines {
			if m := clUnreleasedTagged.FindStringSubmatch(l); m != nil && m[1] != want {
				t.Errorf("CHANGELOG.md: in-development header is %q but version.Version = %q", "## ["+m[1]+"] - Unreleased", want)
			}
		}

		// Invariant 2: version.go is the release IN DEVELOPMENT, so it must be
		// at least as new as the newest dated release (never behind it).
		var topVersion string
		for _, l := range lines {
			if m := clDatedHeader.FindStringSubmatch(l); m != nil {
				topVersion = m[1]
				break
			}
		}
		if topVersion == "" {
			t.Fatalf("CHANGELOG.md: no dated release header found")
		}
		assertLessOrEqual(t, "CHANGELOG.md", "newest dated entry", topVersion, want)
	})

	t.Run("ROADMAP.md current version", func(t *testing.T) {
		rm, err := os.ReadFile(filepath.Join(repoRoot, "ROADMAP.md"))
		if err != nil {
			t.Fatalf("read ROADMAP.md: %v", err)
		}
		// Header line: **Version**: X.Y.Z ...
		var got string
		for _, line := range strings.Split(string(rm), "\n") {
			if m := roadmapVersionRe.FindStringSubmatch(line); m != nil {
				got = m[1]
				break
			}
		}
		if got == "" {
			t.Fatalf("ROADMAP.md: no '**Version**:' header line found")
		}
		// Note: the roadmap's **Version** header tracks the last RELEASed
		// version, while version.Version is the release IN DEVELOPMENT (bumped
		// at prepare-release time). So the header may legitimately trail —
		// the invariant is that it never AHEADS the in-development version.
		assertLessOrEqual(t, "ROADMAP.md", "**Version** header", got, want)

		// The Version Summary table must carry a row for the in-development
		// version (added when that release cycle is prepared).
		if !strings.Contains(string(rm), "| "+want+" |") {
			t.Errorf("ROADMAP.md: Version Summary table has no row for %s (added at prepare-release)", want)
		}
	})

	t.Run("README.md TrueNAS pin", func(t *testing.T) {
		rm, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
		if err != nil {
			t.Fatalf("read README.md: %v", err)
		}
		if !strings.Contains(string(rm), "ghcr.io/goatkit/goatflow:"+want) {
			t.Errorf("README.md: TrueNAS section should pin ghcr.io/goatkit/goatflow:%s (search for ':0.' and update)", want)
		}
	})
}

func assertPin(t *testing.T, file, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s %s = %q, want %q (version.Version) — bump drifted; run 'make prepare-release VERSION=%s' or fix the pin",
			file, field, got, want, want)
	}
}

// assertLessOrEqual passes when got <= want (semver), and fails with a message
// when got is ahead of want.
func assertLessOrEqual(t *testing.T, file, field, got, want string) {
	t.Helper()
	if cmp := compareSemver(got, want); cmp > 0 {
		t.Errorf("%s %s = %q is ahead of version.Version %q — the roadmap can't advertise an unreleased current version",
			file, field, got, want)
	}
}

func compareSemver(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(as) {
			av, _ = parseNum(as[i])
		}
		if i < len(bs) {
			bv, _ = parseNum(bs[i])
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseNum(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-numeric in %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func yamlScalar(t *testing.T, path, pattern string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	re := regexp.MustCompile(pattern)
	for _, line := range strings.Split(string(data), "\n") {
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	t.Fatalf("%s: no line matching %q", path, pattern)
	return ""
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// Tests run in the package dir: internal/platform/version
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..", "..")
}

var (
	semverRe         = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)
	ixTagRe          = regexp.MustCompile(`tag:\s*"?([0-9][0-9a-zA-Z.~-]*)"?\s*$`)
	roadmapVersionRe = regexp.MustCompile(`\*\*Version\*\*:\s*(\d+\.\d+\.\d+)`)
	// Dated release header: '## [0.9.0] - 2026-08-06'
	clDatedHeader = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - \d{4}-\d{2}-\d{2}`)
	// Unreleased variants: '## [0.10.0] - Unreleased' (version captured)
	clUnreleasedTagged = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - Unreleased$`)
)
