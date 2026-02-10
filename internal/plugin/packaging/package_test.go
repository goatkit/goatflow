package packaging

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackagePlugin(t *testing.T) {
	// Create temp plugin directory
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	
	// Create manifest
	manifest := map[string]any{
		"name":        "test-plugin",
		"version":     "1.0.0",
		"description": "Test plugin",
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.json"), manifestData, 0644); err != nil {
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
		"manifest.json":     false,
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
	
	// Create a test ZIP
	zipPath := filepath.Join(tmpDir, "test.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	
	w := zip.NewWriter(zipFile)
	
	// Add manifest
	manifest := map[string]any{
		"name":    "extracted-plugin",
		"version": "2.0.0",
	}
	manifestData, _ := json.Marshal(manifest)
	mw, _ := w.Create("manifest.json")
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
	
	// Verify WASM path
	if pkg.WASMPath == "" {
		t.Error("WASM path not set")
	}
	if _, err := os.Stat(pkg.WASMPath); os.IsNotExist(err) {
		t.Error("WASM file not extracted")
	}
	
	// Verify assets
	if len(pkg.Assets) != 1 {
		t.Errorf("expected 1 asset, got %d", len(pkg.Assets))
	}
}

func TestExtractPluginPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a malicious ZIP with path traversal
	zipPath := filepath.Join(tmpDir, "malicious.zip")
	zipFile, _ := os.Create(zipPath)
	w := zip.NewWriter(zipFile)
	
	// Add manifest
	manifest := map[string]any{"name": "evil"}
	manifestData, _ := json.Marshal(manifest)
	mw, _ := w.Create("manifest.json")
	mw.Write(manifestData)
	
	// Add WASM
	ww, _ := w.Create("plugin.wasm")
	ww.Write([]byte("wasm"))
	
	// Try path traversal (should be skipped)
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
	
	t.Run("Valid package", func(t *testing.T) {
		zipPath := filepath.Join(tmpDir, "valid.zip")
		zipFile, _ := os.Create(zipPath)
		w := zip.NewWriter(zipFile)
		
		manifest := map[string]any{"name": "valid-plugin", "version": "1.0.0"}
		manifestData, _ := json.Marshal(manifest)
		mw, _ := w.Create("manifest.json")
		mw.Write(manifestData)
		
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
	
	t.Run("Missing WASM", func(t *testing.T) {
		zipPath := filepath.Join(tmpDir, "no-wasm.zip")
		zipFile, _ := os.Create(zipPath)
		w := zip.NewWriter(zipFile)
		
		manifest := map[string]any{"name": "no-wasm"}
		manifestData, _ := json.Marshal(manifest)
		mw, _ := w.Create("manifest.json")
		mw.Write(manifestData)
		
		w.Close()
		zipFile.Close()
		
		_, err := ValidatePackage(zipPath)
		if err == nil {
			t.Error("expected error for missing WASM")
		}
	})
	
	t.Run("Missing name in manifest", func(t *testing.T) {
		zipPath := filepath.Join(tmpDir, "no-name.zip")
		zipFile, _ := os.Create(zipPath)
		w := zip.NewWriter(zipFile)
		
		manifest := map[string]any{"version": "1.0.0"} // No name
		manifestData, _ := json.Marshal(manifest)
		mw, _ := w.Create("manifest.json")
		mw.Write(manifestData)
		
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

	t.Run("invalid manifest JSON", func(t *testing.T) {
		srcDir := filepath.Join(tmpDir, "invalid-manifest")
		os.MkdirAll(srcDir, 0755)
		os.WriteFile(filepath.Join(srcDir, "manifest.json"), []byte("not valid json"), 0644)
		os.WriteFile(filepath.Join(srcDir, "plugin.wasm"), []byte("wasm"), 0644)

		err := PackagePlugin(srcDir, filepath.Join(tmpDir, "invalid.zip"))
		if err == nil {
			t.Error("expected error for invalid manifest JSON")
		}
	})

	t.Run("manifest missing name", func(t *testing.T) {
		srcDir := filepath.Join(tmpDir, "no-name-src")
		os.MkdirAll(srcDir, 0755)
		manifest, _ := json.Marshal(map[string]any{"version": "1.0.0"})
		os.WriteFile(filepath.Join(srcDir, "manifest.json"), manifest, 0644)
		os.WriteFile(filepath.Join(srcDir, "plugin.wasm"), []byte("wasm"), 0644)

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
