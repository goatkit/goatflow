package loader

import (
	"crypto/rand"
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
// file. All file access is scoped to the two directories (os.Root), so a
// symlink in the plugin directory cannot redirect a write outside it. A
// missing bundledDir is not an error (non-container runs have none).
func SyncBundledPlugins(bundledDir, pluginDir string) (BundledSyncResult, error) {
	return syncBundledPlugins(bundledDir, pluginDir, knownShippedBundledHashes)
}

func syncBundledPlugins(bundledDir, pluginDir string, known map[string][]string) (BundledSyncResult, error) {
	var res BundledSyncResult
	src, err := os.OpenRoot(bundledDir)
	if errors.Is(err, fs.ErrNotExist) {
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("open bundled plugins: %w", err)
	}
	defer src.Close()
	bundled := src.FS()
	entries, err := fs.ReadDir(bundled, ".")
	if err != nil {
		return res, fmt.Errorf("read bundled plugins: %w", err)
	}

	if err := os.MkdirAll(pluginDir, 0o750); err != nil {
		return res, fmt.Errorf("create plugin dir: %w", err)
	}
	dst, err := os.OpenRoot(pluginDir)
	if err != nil {
		return res, fmt.Errorf("open plugin dir: %w", err)
	}
	defer dst.Close()

	manifest := readBundledManifest(dst)
	recorded := make(map[string]string, len(manifest.Files))
	for rel, sum := range manifest.Files {
		recorded[rel] = sum
	}

	for _, entry := range entries {
		// tmp holds the WASM build's scratch space, not a plugin.
		if !entry.IsDir() || entry.Name() == "tmp" {
			continue
		}
		err := fs.WalkDir(bundled, entry.Name(), func(key string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !d.Type().IsRegular() {
				return nil
			}
			want, err := fs.ReadFile(bundled, key)
			if err != nil {
				return err
			}
			wantSum := sha256Hex(want)
			rel := filepath.FromSlash(key)

			have, err := dst.ReadFile(rel)
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
			if err := writeFileAtomic(dst, rel, want, info.Mode().Perm()); err != nil {
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
		if err := writeFileAtomic(dst, BundledManifestName, append(data, '\n'), 0o640); err != nil {
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
func readBundledManifest(dir *os.Root) bundledManifest {
	var m bundledManifest
	data, err := dir.ReadFile(BundledManifestName)
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

// writeFileAtomic writes data to a temp file beside name (relative to dir)
// and renames it over name.
func writeFileAtomic(dir *os.Root, name string, data []byte, perm fs.FileMode) error {
	parent := filepath.Dir(name)
	if err := dir.MkdirAll(parent, 0o750); err != nil {
		return err
	}
	tmp := filepath.Join(parent, ".bundled-"+rand.Text())
	f, err := dir.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Remove(tmp) }() // fails harmlessly once renamed
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := dir.Chmod(tmp, perm); err != nil {
		return err
	}
	return dir.Rename(tmp, name)
}
