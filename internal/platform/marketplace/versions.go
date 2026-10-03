package marketplace

import (
	"errors"
	"fmt"
	"sort"

	"github.com/goatkit/goatflow/internal/platform/version"
)

// ErrNoUpdate indicates no compatible version newer than the installed one.
var ErrNoUpdate = errors.New("no compatible update available")

// AllVersions returns the installable versions of p, newest first. An entry
// without a versions list (the original index shape) offers exactly one:
// LatestVersion with the entry's MinHostVersion. A versions list that omits
// LatestVersion still offers it, with the entry's MinHostVersion.
func (p *PluginEntry) AllVersions() []VersionEntry {
	out := make([]VersionEntry, 0, len(p.Versions)+1)
	hasLatest := false
	for _, v := range p.Versions {
		if v.Version == "" {
			continue
		}
		if versionCompare(v.Version, p.LatestVersion) == 0 {
			hasLatest = true
		}
		out = append(out, v)
	}
	if !hasLatest && p.LatestVersion != "" {
		out = append(out, VersionEntry{Version: p.LatestVersion, MinHostVersion: p.MinHostVersion})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return versionCompare(out[i].Version, out[j].Version) > 0
	})
	return out
}

// Compatible reports whether this GoatFlow satisfies v's minimum host version.
func (v VersionEntry) Compatible() bool {
	return version.HostCompatible(v.MinHostVersion)
}

// incompatible builds the error for p at v.
func (p *PluginEntry) incompatible(v VersionEntry) error {
	return version.RequireHost(p.Name, v.Version, v.MinHostVersion)
}

// ResolveInstall returns the newest version of p this GoatFlow can run. When
// none is compatible the error is a *version.IncompatibleError for the oldest
// listed version (the least that would have to be satisfied).
func (p *PluginEntry) ResolveInstall() (VersionEntry, error) {
	all := p.AllVersions()
	if len(all) == 0 {
		return VersionEntry{}, fmt.Errorf("plugin %q lists no versions", p.Name)
	}
	for _, v := range all {
		if v.Compatible() {
			return v, nil
		}
	}
	return VersionEntry{}, p.incompatible(all[len(all)-1])
}

// ResolveUpdate returns the newest compatible version of p newer than
// installed. When newer versions exist but none is compatible the error is a
// *version.IncompatibleError for the oldest newer one; when nothing is newer
// it is ErrNoUpdate.
func (p *PluginEntry) ResolveUpdate(installed string) (VersionEntry, error) {
	var oldestNewer *VersionEntry
	for _, v := range p.AllVersions() {
		if versionCompare(v.Version, installed) <= 0 {
			break
		}
		if v.Compatible() {
			return v, nil
		}
		oldestNewer = &v
	}
	if oldestNewer != nil {
		return VersionEntry{}, p.incompatible(*oldestNewer)
	}
	return VersionEntry{}, ErrNoUpdate
}

// ResolveVersion returns the listed version want of p, failing when it is not
// listed or this GoatFlow is too old for it.
func (p *PluginEntry) ResolveVersion(want string) (VersionEntry, error) {
	for _, v := range p.AllVersions() {
		if versionCompare(v.Version, want) != 0 {
			continue
		}
		if err := p.incompatible(v); err != nil {
			return VersionEntry{}, err
		}
		return v, nil
	}
	return VersionEntry{}, fmt.Errorf("plugin %q has no version %s in the marketplace", p.Name, want)
}
