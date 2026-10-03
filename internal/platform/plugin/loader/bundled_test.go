package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func readTestManifest(t *testing.T, pluginDir string) map[string]string {
	t.Helper()
	path := filepath.Join(pluginDir, BundledManifestName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return map[string]string{}
	}
	var m bundledManifest
	require.NoError(t, json.Unmarshal([]byte(readTestFile(t, path)), &m))
	return m.Files
}

// newBundle creates an image-side bundle holding stats/stats.wasm with the
// given content and returns (bundledDir, pluginDir).
func newBundle(t *testing.T, wasm string) (string, string) {
	t.Helper()
	root := t.TempDir()
	bundled := filepath.Join(root, "bundled")
	writeTestFile(t, filepath.Join(bundled, "stats", "stats.wasm"), wasm)
	return bundled, filepath.Join(root, "plugins")
}

func TestSyncBundledPlugins_InstallsMissingFilesAndRecordsThem(t *testing.T) {
	bundled, plugins := newBundle(t, "stats v2")
	writeTestFile(t, filepath.Join(bundled, "tmp", "cache"), "build scratch")

	res, err := syncBundledPlugins(bundled, plugins, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"stats/stats.wasm"}, res.Updated)
	assert.Empty(t, res.Kept)
	assert.Equal(t, "stats v2", readTestFile(t, filepath.Join(plugins, "stats", "stats.wasm")))
	assert.NoFileExists(t, filepath.Join(plugins, "tmp", "cache"), "the build scratch dir is not a plugin")
	assert.Equal(t, map[string]string{"stats/stats.wasm": sha256Hex([]byte("stats v2"))}, readTestManifest(t, plugins))
}

func TestSyncBundledPlugins_ReplacesUntouchedCopyRecordedInManifest(t *testing.T) {
	bundled, plugins := newBundle(t, "stats v1")
	_, err := syncBundledPlugins(bundled, plugins, nil)
	require.NoError(t, err)

	// The next release ships a new version.
	writeTestFile(t, filepath.Join(bundled, "stats", "stats.wasm"), "stats v2")
	res, err := syncBundledPlugins(bundled, plugins, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"stats/stats.wasm"}, res.Updated)
	assert.Equal(t, "stats v2", readTestFile(t, filepath.Join(plugins, "stats", "stats.wasm")))
	assert.Equal(t, sha256Hex([]byte("stats v2")), readTestManifest(t, plugins)["stats/stats.wasm"])
}

func TestSyncBundledPlugins_KeepsAdminReplacementOfRecordedCopy(t *testing.T) {
	bundled, plugins := newBundle(t, "stats v1")
	_, err := syncBundledPlugins(bundled, plugins, nil)
	require.NoError(t, err)

	writeTestFile(t, filepath.Join(plugins, "stats", "stats.wasm"), "admin's own stats")
	writeTestFile(t, filepath.Join(bundled, "stats", "stats.wasm"), "stats v2")
	for range 2 { // and again on the next start
		res, err := syncBundledPlugins(bundled, plugins, nil)
		require.NoError(t, err)
		assert.Empty(t, res.Updated)
		assert.Equal(t, []string{"stats/stats.wasm"}, res.Kept)
	}
	assert.Equal(t, "admin's own stats", readTestFile(t, filepath.Join(plugins, "stats", "stats.wasm")))
}

func TestSyncBundledPlugins_WithoutManifestReplacesKnownShippedCopy(t *testing.T) {
	// A plugin dir populated by a pre-manifest release (0.9.x): no manifest,
	// but the file is byte-identical to what that release shipped.
	bundled, plugins := newBundle(t, "stats v2")
	writeTestFile(t, filepath.Join(plugins, "stats", "stats.wasm"), "stats as shipped in 0.9.0")
	known := map[string][]string{"stats/stats.wasm": {sha256Hex([]byte("stats as shipped in 0.9.0"))}}

	res, err := syncBundledPlugins(bundled, plugins, known)
	require.NoError(t, err)

	assert.Equal(t, []string{"stats/stats.wasm"}, res.Updated)
	assert.Empty(t, res.Kept)
	assert.Equal(t, "stats v2", readTestFile(t, filepath.Join(plugins, "stats", "stats.wasm")))
}

func TestSyncBundledPlugins_WithoutManifestKeepsUnknownFile(t *testing.T) {
	bundled, plugins := newBundle(t, "stats v2")
	writeTestFile(t, filepath.Join(plugins, "stats", "stats.wasm"), "admin's own stats")
	known := map[string][]string{"stats/stats.wasm": {sha256Hex([]byte("stats as shipped in 0.9.0"))}}

	res, err := syncBundledPlugins(bundled, plugins, known)
	require.NoError(t, err)

	assert.Empty(t, res.Updated)
	assert.Equal(t, []string{"stats/stats.wasm"}, res.Kept)
	assert.Equal(t, "admin's own stats", readTestFile(t, filepath.Join(plugins, "stats", "stats.wasm")))
	assert.NotContains(t, readTestManifest(t, plugins), "stats/stats.wasm",
		"a file bundling did not install must not be recorded as bundled")
}

func TestSyncBundledPlugins_CorruptManifestFallsBackToKnownHashes(t *testing.T) {
	bundled, plugins := newBundle(t, "stats v2")
	writeTestFile(t, filepath.Join(plugins, "stats", "stats.wasm"), "admin's own stats")
	writeTestFile(t, filepath.Join(plugins, BundledManifestName), "{not json")

	res, err := syncBundledPlugins(bundled, plugins, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"stats/stats.wasm"}, res.Kept)
	assert.Equal(t, "admin's own stats", readTestFile(t, filepath.Join(plugins, "stats", "stats.wasm")))
}

func TestSyncBundledPlugins_LeavesOtherPluginsAndExtraFilesAlone(t *testing.T) {
	bundled, plugins := newBundle(t, "stats v2")
	writeTestFile(t, filepath.Join(plugins, "uploaded", "uploaded.wasm"), "uploaded plugin")
	writeTestFile(t, filepath.Join(plugins, "stats", "notes.txt"), "admin notes")

	_, err := syncBundledPlugins(bundled, plugins, nil)
	require.NoError(t, err)

	assert.Equal(t, "uploaded plugin", readTestFile(t, filepath.Join(plugins, "uploaded", "uploaded.wasm")))
	assert.Equal(t, "admin notes", readTestFile(t, filepath.Join(plugins, "stats", "notes.txt")))
	entries, err := os.ReadDir(filepath.Join(plugins, "stats"))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bundled-", "no temp files left behind")
	}
}

func TestSyncBundledPlugins_UpToDateDirWritesNothing(t *testing.T) {
	bundled, plugins := newBundle(t, "stats v2")
	_, err := syncBundledPlugins(bundled, plugins, nil)
	require.NoError(t, err)
	manifestPath := filepath.Join(plugins, BundledManifestName)
	before, err := os.Stat(manifestPath)
	require.NoError(t, err)

	res, err := syncBundledPlugins(bundled, plugins, nil)
	require.NoError(t, err)

	assert.Empty(t, res.Updated)
	assert.Empty(t, res.Kept)
	after, err := os.Stat(manifestPath)
	require.NoError(t, err)
	assert.True(t, os.SameFile(before, after), "manifest must not be rewritten when nothing changed")
}

func TestSyncBundledPlugins_MissingBundleIsNoOp(t *testing.T) {
	plugins := filepath.Join(t.TempDir(), "plugins")
	res, err := syncBundledPlugins(filepath.Join(t.TempDir(), "absent"), plugins, nil)
	require.NoError(t, err)
	assert.Empty(t, res.Updated)
	assert.NoDirExists(t, plugins)
}

func TestSyncBundledPlugins_DoesNotWriteThroughSymlinkLeavingPluginDir(t *testing.T) {
	// A plugin directory in the (operator-writable) volume that is a symlink
	// to somewhere outside it must not be written through.
	bundled, plugins := newBundle(t, "stats v2")
	outside := filepath.Join(t.TempDir(), "elsewhere")
	require.NoError(t, os.MkdirAll(outside, 0o750))
	require.NoError(t, os.MkdirAll(plugins, 0o750))
	require.NoError(t, os.Symlink(outside, filepath.Join(plugins, "stats")))

	res, err := syncBundledPlugins(bundled, plugins, nil)

	require.Error(t, err)
	assert.Empty(t, res.Updated)
	assert.NoFileExists(t, filepath.Join(outside, "stats.wasm"))
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	assert.Empty(t, entries, "no temp or plugin file may land outside the plugin dir")
}
