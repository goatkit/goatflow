package loader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// BundledPluginDirEnv overrides where the image keeps its pristine copy of
// the plugins it ships with (default "bundled-plugins" under the working
// directory, i.e. /app/bundled-plugins in the container image).
const BundledPluginDirEnv = "GOATFLOW_BUNDLED_PLUGIN_DIR"

// BundledManifestName is the file in the plugin directory recording, per
// bundled file, the sha256 of the copy that bundling last installed there.
const BundledManifestName = ".bundled-manifest.json"

// BundledSyncResult reports what SyncBundledPlugins did. Paths are relative
// to the plugin directory, sorted.
type BundledSyncResult struct {
	// Updated lists files written: missing ones, and untouched bundled copies
	// from an older release replaced by this release's version.
	Updated []string
	// Kept lists files that differ from this release's bundled version but are
	// not a copy bundling installed (an admin changed or replaced them); they
	// were left as they are.
	Kept []string
}

type bundledManifest struct {
	Files map[string]string `json:"files"`
}

// SyncBundledPlugins installs the plugins shipped with this build from
// bundledDir into pluginDir. A plugin directory that outlives the image (a
// Docker volume mounted over config/plugins) would otherwise keep the bundled
// plugins of the release that first populated it, missing every later fix.
//
// For each bundled file it writes this release's version when the file is
// missing, or when the current file is still an untouched bundled copy: its
// sha256 equals the one recorded in BundledManifestName, or, for directories
// populated before the manifest existed (0.9.x and older), a hash a release
// is known to have shipped. Any other differing file is an admin's change
// and is kept (reported in Kept). Plugins under other names are not touched.
// Writes go through a temp file and rename, so readers never see a partial
// file. A missing bundledDir is not an error (non-container runs have none).
func SyncBundledPlugins(bundledDir, pluginDir string) (BundledSyncResult, error) {
	return syncBundledPlugins(bundledDir, pluginDir, knownShippedBundledHashes)
}

func syncBundledPlugins(bundledDir, pluginDir string, known map[string][]string) (BundledSyncResult, error) {
	var res BundledSyncResult
	entries, err := os.ReadDir(bundledDir)
	if errors.Is(err, fs.ErrNotExist) {
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("read bundled plugins: %w", err)
	}

	manifest := readBundledManifest(filepath.Join(pluginDir, BundledManifestName))
	recorded := make(map[string]string, len(manifest.Files))
	for rel, sum := range manifest.Files {
		recorded[rel] = sum
	}

	for _, entry := range entries {
		// tmp holds the WASM build's scratch space, not a plugin.
		if !entry.IsDir() || entry.Name() == "tmp" {
			continue
		}
		err := filepath.WalkDir(filepath.Join(bundledDir, entry.Name()), func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(bundledDir, path)
			if err != nil {
				return err
			}
			key := filepath.ToSlash(rel)
			want, err := os.ReadFile(path) // #nosec G304 -- path comes from the image's bundled plugin tree
			if err != nil {
				return err
			}
			wantSum := sha256Hex(want)
			dst := filepath.Join(pluginDir, rel)

			have, err := os.ReadFile(dst) // #nosec G304 -- same relative path under the operator's plugin dir
			switch {
			case errors.Is(err, fs.ErrNotExist):
				// Missing: install.
			case err != nil:
				return err
			default:
				haveSum := sha256Hex(have)
				if haveSum == wantSum {
					recorded[key] = wantSum
					return nil
				}
				if !isUntouchedBundledCopy(key, haveSum, recorded, known) {
					res.Kept = append(res.Kept, key)
					return nil
				}
			}

			info, err := d.Info()
			if err != nil {
				return err
			}
			if err := writeFileAtomic(dst, want, info.Mode().Perm()); err != nil {
				return err
			}
			recorded[key] = wantSum
			res.Updated = append(res.Updated, key)
			return nil
		})
		if err != nil {
			return res, fmt.Errorf("sync bundled plugin %s: %w", entry.Name(), err)
		}
	}

	sort.Strings(res.Updated)
	sort.Strings(res.Kept)
	if !sameFiles(manifest.Files, recorded) {
		data, err := json.MarshalIndent(bundledManifest{Files: recorded}, "", "  ")
		if err != nil {
			return res, err
		}
		if err := writeFileAtomic(filepath.Join(pluginDir, BundledManifestName), append(data, '\n'), 0o640); err != nil {
			return res, fmt.Errorf("write bundled plugin manifest: %w", err)
		}
	}
	return res, nil
}

// isUntouchedBundledCopy reports whether a file whose content hashes to
// haveSum is a copy bundling installed, which may therefore be replaced.
func isUntouchedBundledCopy(key, haveSum string, recorded map[string]string, known map[string][]string) bool {
	if sum, ok := recorded[key]; ok {
		return sum == haveSum
	}
	for _, sum := range known[key] {
		if sum == haveSum {
			return true
		}
	}
	return false
}

// readBundledManifest returns the recorded manifest; a missing or unreadable
// one counts as empty, which only makes the sync more conservative.
func readBundledManifest(path string) bundledManifest {
	var m bundledManifest
	data, err := os.ReadFile(path) // #nosec G304 -- fixed name inside the operator's plugin dir
	if err == nil {
		if json.Unmarshal(data, &m) != nil {
			m = bundledManifest{}
		}
	}
	if m.Files == nil {
		m.Files = map[string]string{}
	}
	return m
}

func sameFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// writeFileAtomic writes data to a temp file beside path and renames it over
// path.
func writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".bundled-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
