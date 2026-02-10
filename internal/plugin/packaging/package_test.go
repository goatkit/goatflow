package packaging

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

func createManifestYAML(t *testing.T, m pkgplugin.PluginManifest) []byte {
	t.Helper()
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPackagePlugin(t *testing.T) {
	// Create temp plugin directory
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create plugin.yaml manifest
	manifest := pkgplugin.PluginManifest{
		Name:    "test-plugin",
		Version: "1.0.0",
		Runtime: "wasm",
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), createManifestYAML(t, manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Create fake WASM file
	if err := os.WriteFile(filepath.Join(pluginDir, "test-plugin.wasm"), []byte("fake wasm"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create assets directory
	assetsDir := filepath.Join(pluginDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "style.css"), []byte("body {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Package
	outputPath := filepath.Join(tmpDir, "test-plugin.zip")
	if err := PackagePlugin(pluginDir, outputPath); err != nil {
		t.Fatalf("PackagePlugin failed: %v", err)
	}

	// Verify ZIP exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("output ZIP not created")
	}

	// Verify ZIP contents
	reader, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	expectedFiles := map[string]bool{
		"plugin.yaml":       false,
		"test-plugin.wasm":  false,
		"assets/style.css":  false,
	}

	for _, f := range reader.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("expected file %s not found in ZIP", name)
		}
	}
}

func TestPackagePluginMissingManifest(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "no-manifest")
	os.MkdirAll(pluginDir, 0755)

	outputPath := filepath.Join(tmpDir, "output.zip")
	err := PackagePlugin(pluginDir, outputPath)
	if err == nil {
		t.Error("expected error for missing manifest")
	}
}

func TestExtractPlugin(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test ZIP with plugin.yaml
	zipPath := filepath.Join(tmpDir, "test.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	w := zip.NewWriter(zipFile)

	// Add plugin.yaml manifest
	manifest := pkgplugin.PluginManifest{
		Name:    "extracted-plugin",
		Version: "2.0.0",
		Runtime: "wasm",
	}
	manifestData := createManifestYAML(t, manifest)
	mw, _ := w.Create("plugin.yaml")
	mw.Write(manifestData)

	// Add WASM
	ww, _ := w.Create("plugin.wasm")
	ww.Write([]byte("wasm content"))

	// Add asset
	aw, _ := w.Create("assets/icon.png")
	aw.Write([]byte("png content"))

	w.Close()
	zipFile.Close()

	// Extract
	targetDir := filepath.Join(tmpDir, "extracted")
	pkg, err := ExtractPlugin(zipPath, targetDir)
	if err != nil {
		t.Fatalf("ExtractPlugin failed: %v", err)
	}

	// Verify manifest
	if pkg.Manifest.Name != "extracted-plugin" {
		t.Errorf("expected name extracted-plugin, got %s", pkg.Manifest.Name)
	}
	if pkg.Manifest.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %s", pkg.Manifest.Version)
	}

	// Verify runtime type
	if pkg.RuntimeType != "wasm" {
		t.Errorf("expected runtime wasm, got %s", pkg.RuntimeType)
	}

	// Verify WASM path
	if pkg.BinaryPath == "" {
		t.Error("BinaryPath not set for wasm plugin")
	}
	if _, err := os.Stat(pkg.BinaryPath); os.IsNotExist(err) {
		t.Error("WASM file not extracted")
	}

	// Verify assets
	if len(pkg.Assets) != 1 {
		t.Errorf("expected 1 asset, got %d", len(pkg.Assets))
	}
}

func TestExtractPluginGRPC(t *testing.T) {
	tmpDir := t.TempDir()

	zipPath := filepath.Join(tmpDir, "grpc-plugin.zip")
	zipFile, _ := os.Create(zipPath)
	w := zip.NewWriter(zipFile)

	// Add plugin.yaml for gRPC plugin
	manifest := pkgplugin.PluginManifest{
		Name:    "my-grpc-plugin",
		Version: "1.0.0",
		Runtime: "grpc",
		Binary:  "my-grpc-plugin",
	}
	mw, _ := w.Create("plugin.yaml")
	mw.Write(createManifestYAML(t, manifest))

	// Add binary
	bw, _ := w.Create("my-grpc-plugin")
	bw.Write([]byte("fake binary content"))

	w.Close()
	zipFile.Close()

	targetDir := filepath.Join(tmpDir, "extracted")
	pkg, err := ExtractPlugin(zipPath, targetDir)
	if err != nil {
		t.Fatalf("ExtractPlugin failed for gRPC: %v", err)
	}

	if pkg.RuntimeType != "grpc" {
		t.Errorf("expected runtime grpc, got %s", pkg.RuntimeType)
	}
	if pkg.BinaryPath == "" {
		t.Error("BinaryPath not set for gRPC plugin")
	}

	// Verify binary is executable
	info, err := os.Stat(pkg.BinaryPath)
	if err != nil {
		t.Fatalf("binary not found: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("binary should be executable")
	}
}

func TestExtractPluginTemplate(t *testing.T) {
	tmpDir := t.TempDir()

	zipPath := filepath.Join(tmpDir, "template-plugin.zip")
	zipFile, _ := os.Create(zipPath)
	w := zip.NewWriter(zipFile)

	// Add plugin.yaml for template plugin (no runtime or runtime: template)
	manifest := pkgplugin.PluginManifest{
		Name:    "my-template-plugin",
		Version: "1.0.0",
		Runtime: "template",
	}
	mw, _ := w.Create("plugin.yaml")
	mw.Write(createManifestYAML(t, manifest))

	// Add template assets only
	tw, _ := w.Create("assets/template.html")
	tw.Write([]byte("<h1>Hello</h1>"))

	w.Close()
	zipFile.Close()

	targetDir := filepath.Join(tmpDir, "extracted")
	pkg, err := ExtractPlugin(zipPath, targetDir)
	if err != nil {
		t.Fatalf("ExtractPlugin failed for template: %v", err)
	}

	if pkg.RuntimeType != "template" {
		t.Errorf("expected runtime template, got %s", pkg.RuntimeType)
	}
	if pkg.BinaryPath != "" {
		t.Errorf("template plugin should have no binary path, got %s", pkg.BinaryPath)
	}
}

func TestExtractPluginDefaultRuntime(t *testing.T) {
	tmpDir := t.TempDir()

	zipPath := filepath.Join(tmpDir, "default-plugin.zip")
	zipFile, _ := os.Create(zipPath)
	w := zip.NewWriter(zipFile)

	// Add plugin.yaml with no runtime specified
	manifest := pkgplugin.PluginManifest{
		Name:    "default-plugin",
		Version: "1.0.0",
	}
	mw, _ := w.Create("plugin.yaml")
	mw.Write(createManifestYAML(t, manifest))

	w.Close()
	zipFile.Close()

	targetDir := filepath.Join(tmpDir, "extracted")
	pkg, err := ExtractPlugin(zipPath, targetDir)
	if err != nil {
		t.Fatalf("ExtractPlugin failed: %v", err)
	}

	if pkg.RuntimeType != "template" {
		t.Errorf("expected default runtime template, got %s", pkg.RuntimeType)
	}
}

func TestExtractPluginPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a malicious ZIP with path traversal
	zipPath := filepath.Join(tmpDir, "malicious.zip")
	zipFile, _ := os.Create(zipPath)
	w := zip.NewWriter(zipFile)

	// Add manifest
	manifest := pkgplugin.PluginManifest{Name: "evil", Runtime: "template"}
	mw, _ := w.Create("plugin.yaml")
	mw.Write(createManifestYAML(t, manifest))

	// Try path traversal (should be caught)
	pw, _ := w.Create("../../../etc/passwd")
	pw.Write([]byte("malicious"))

	w.Close()
	zipFile.Close()

	// Extract - should fail due to path traversal security check
	targetDir := filepath.Join(tmpDir, "extracted")
	_, err := ExtractPlugin(zipPath, targetDir)
	if err == nil {
		t.Fatal("ExtractPlugin should have failed due to path traversal")
	}

	// Verify the error message mentions path traversal
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

func TestValidatePackage(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("Valid wasm package", func(t *testing.T) {
		zipPath := filepath.Join(tmpDir, "valid-wasm.zip")
		zipFile, _ := os.Create(zipPath)
		w := zip.NewWriter(zipFile)

		manifest := pkgplugin.PluginManifest{Name: "valid-plugin", Version: "1.0.0", Runtime: "wasm"}
		mw, _ := w.Create("plugin.yaml")
		mw.Write(createManifestYAML(t, manifest))

		ww, _ := w.Create("plugin.wasm")
		ww.Write([]byte("wasm"))

		w.Close()
		zipFile.Close()

		m, err := ValidatePackage(zipPath)
		if err != nil {
			t.Errorf("expected valid, got error: %v", err)
		}
		if m.Name != "valid-plugin" {
			t.Errorf("expected valid-plugin, got %s", m.Name)
		}
	})

	t.Run("Valid grpc package without wasm", func(t *testing.T) {
		zipPath := filepath.Join(tmpDir, "valid-grpc.zip")
		zipFile, _ := os.Create(zipPath)
		w := zip.NewWriter(zipFile)

		manifest := pkgplugin.PluginManifest{Name: "grpc-plugin", Version: "1.0.0", Runtime: "grpc", Binary: "my-binary"}
		mw, _ := w.Create("plugin.yaml")
		mw.Write(createManifestYAML(t, manifest))

		bw, _ := w.Create("my-binary")
		bw.Write([]byte("binary"))

		w.Close()
		zipFile.Close()

		m, err := ValidatePackage(zipPath)
		if err != nil {
			t.Errorf("expected valid grpc package, got error: %v", err)
		}
		if m.Name != "grpc-plugin" {
			t.Errorf("expected grpc-plugin, got %s", m.Name)
		}
	})

	t.Run("Valid template package without wasm", func(t *testing.T) {
		zipPath := filepath.Join(tmpDir, "valid-template.zip")
		zipFile, _ := os.Create(zipPath)
		w := zip.NewWriter(zipFile)

		manifest := pkgplugin.PluginManifest{Name: "template-plugin", Version: "1.0.0", Runtime: "template"}
		mw, _ := w.Create("plugin.yaml")
		mw.Write(createManifestYAML(t, manifest))

		w.Close()
		zipFile.Close()

		m, err := ValidatePackage(zipPath)
		if err != nil {
			t.Errorf("expected valid template package, got error: %v", err)
		}
		if m.Name != "template-plugin" {
			t.Errorf("expected template-plugin, got %s", m.Name)
		}
	})

	t.Run("Missing manifest", func(t *testing.T) {
		zipPath := filepath.Join(tmpDir, "no-manifest.zip")
		zipFile, _ := os.Create(zipPath)
		w := zip.NewWriter(zipFile)

		ww, _ := w.Create("plugin.wasm")
		ww.Write([]byte("wasm"))

		w.Close()
		zipFile.Close()

		_, err := ValidatePackage(zipPath)
		if err == nil {
			t.Error("expected error for missing manifest")
		}
	})

	t.Run("Missing WASM for wasm runtime", func(t *testing.T) {
		zipPath := filepath.Join(tmpDir, "no-wasm.zip")
		zipFile, _ := os.Create(zipPath)
		w := zip.NewWriter(zipFile)

		manifest := pkgplugin.PluginManifest{Name: "no-wasm", Runtime: "wasm"}
		mw, _ := w.Create("plugin.yaml")
		mw.Write(createManifestYAML(t, manifest))

		w.Close()
		zipFile.Close()

		_, err := ValidatePackage(zipPath)
		if err == nil {
			t.Error("expected error for missing WASM with wasm runtime")
		}
	})

	t.Run("Missing name in manifest", func(t *testing.T) {
		zipPath := filepath.Join(tmpDir, "no-name.zip")
		zipFile, _ := os.Create(zipPath)
		w := zip.NewWriter(zipFile)

		manifest := pkgplugin.PluginManifest{Version: "1.0.0", Runtime: "wasm"}
		mw, _ := w.Create("plugin.yaml")
		mw.Write(createManifestYAML(t, manifest))

		ww, _ := w.Create("plugin.wasm")
		ww.Write([]byte("wasm"))

		w.Close()
		zipFile.Close()

		_, err := ValidatePackage(zipPath)
		if err == nil {
			t.Error("expected error for missing name")
		}
	})
}

func TestPackagePluginErrors(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("nonexistent source dir", func(t *testing.T) {
		err := PackagePlugin("/nonexistent/path", filepath.Join(tmpDir, "out.zip"))
		if err == nil {
			t.Error("expected error for nonexistent source")
		}
	})

	t.Run("invalid manifest YAML", func(t *testing.T) {
		srcDir := filepath.Join(tmpDir, "invalid-manifest")
		os.MkdirAll(srcDir, 0755)
		os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), []byte("not: [valid: yaml: {{"), 0644)

		err := PackagePlugin(srcDir, filepath.Join(tmpDir, "invalid.zip"))
		if err == nil {
			t.Error("expected error for invalid manifest YAML")
		}
	})

	t.Run("manifest missing name", func(t *testing.T) {
		srcDir := filepath.Join(tmpDir, "no-name-src")
		os.MkdirAll(srcDir, 0755)
		manifest := pkgplugin.PluginManifest{Version: "1.0.0"}
		data, _ := yaml.Marshal(manifest)
		os.WriteFile(filepath.Join(srcDir, "plugin.yaml"), data, 0644)

		err := PackagePlugin(srcDir, filepath.Join(tmpDir, "noname.zip"))
		if err == nil {
			t.Error("expected error for missing name in manifest")
		}
	})
}

func TestExtractPluginErrors(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("nonexistent zip", func(t *testing.T) {
		_, err := ExtractPlugin("/nonexistent/file.zip", tmpDir)
		if err == nil {
			t.Error("expected error for nonexistent zip")
		}
	})

	t.Run("invalid zip file", func(t *testing.T) {
		invalidZip := filepath.Join(tmpDir, "invalid.zip")
		os.WriteFile(invalidZip, []byte("not a zip file"), 0644)

		_, err := ExtractPlugin(invalidZip, filepath.Join(tmpDir, "out"))
		if err == nil {
			t.Error("expected error for invalid zip")
		}
	})
}
