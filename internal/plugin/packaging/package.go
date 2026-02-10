// Package packaging provides ZIP-based plugin packaging and extraction.
package packaging

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goatkit/goatflow/internal/plugin"
)

const (
	// Security limits for ZIP extraction
	MaxFileSize       = 100 << 20 // 100 MB per file
	MaxTotalSize      = 500 << 20 // 500 MB total extraction
	MaxFileCount      = 1000      // Maximum number of files in archive
)

// PluginPackage represents a packaged plugin (ZIP file).
type PluginPackage struct {
	Manifest plugin.GKRegistration
	WASMPath string            // Path to .wasm file within package
	Assets   map[string]string // asset name -> path within package
}

// PackagePlugin creates a ZIP package from a plugin directory.
// The directory should contain:
//   - manifest.json (required)
//   - *.wasm file (required for WASM plugins)
//   - assets/ directory (optional)
//   - i18n/ directory (optional)
func PackagePlugin(pluginDir, outputPath string) error {
	// Read manifest
	manifestPath := filepath.Join(pluginDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest plugin.GKRegistration
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("invalid manifest.json: %w", err)
	}

	if manifest.Name == "" {
		return fmt.Errorf("manifest.json missing required 'name' field")
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// Walk the plugin directory and add files
	err = filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(pluginDir, path)
		if err != nil {
			return err
		}

		// Skip hidden files and build artifacts
		if strings.HasPrefix(filepath.Base(relPath), ".") {
			return nil
		}
		if strings.Contains(relPath, "node_modules") || strings.Contains(relPath, "target") {
			return nil
		}

		// Add file to ZIP
		return addFileToZip(zipWriter, path, relPath)
	})

	if err != nil {
		return fmt.Errorf("failed to package plugin: %w", err)
	}

	return nil
}

// ExtractPlugin extracts a plugin package to the target directory.
// Returns the manifest and path to the extracted WASM file.
func ExtractPlugin(packagePath, targetDir string) (*PluginPackage, error) {
	reader, err := zip.OpenReader(packagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open package: %w", err)
	}
	defer reader.Close()

	pkg := &PluginPackage{
		Assets: make(map[string]string),
	}

	// Security counters to prevent zip bombs
	var totalSize int64
	var fileCount int

	// First pass: find and validate manifest
	var manifestFile *zip.File
	for _, f := range reader.File {
		if f.Name == "manifest.json" || filepath.Base(f.Name) == "manifest.json" {
			manifestFile = f
			break
		}
	}

	if manifestFile == nil {
		return nil, fmt.Errorf("package missing manifest.json")
	}

	// Read manifest
	rc, err := manifestFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	manifestData, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	if err := json.Unmarshal(manifestData, &pkg.Manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest.json: %w", err)
	}

	if pkg.Manifest.Name == "" {
		return nil, fmt.Errorf("manifest missing required 'name' field")
	}

	// Create plugin directory
	pluginDir := filepath.Join(targetDir, pkg.Manifest.Name)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Extract all files with security checks
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Security: Check file count limit
		fileCount++
		if fileCount > MaxFileCount {
			return nil, fmt.Errorf("archive contains too many files (max %d)", MaxFileCount)
		}

		// Security: Check for symlinks
		if f.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("archive contains symbolic links, which are not allowed")
		}

		// Security: Check individual file size
		if f.UncompressedSize64 > MaxFileSize {
			return nil, fmt.Errorf("file %s too large: %d bytes (max %d)", f.Name, f.UncompressedSize64, MaxFileSize)
		}

		// Security: Check total extraction size
		totalSize += int64(f.UncompressedSize64)
		if totalSize > MaxTotalSize {
			return nil, fmt.Errorf("archive too large when extracted: %d bytes (max %d)", totalSize, MaxTotalSize)
		}

		// Security: prevent path traversal
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, "..") {
			return nil, fmt.Errorf("archive contains path traversal: %s", f.Name)
		}

		destPath := filepath.Join(pluginDir, cleanName)
		
		// Additional security: ensure destination is within plugin directory
		if !strings.HasPrefix(destPath, pluginDir+string(os.PathSeparator)) {
			return nil, fmt.Errorf("path traversal detected: %s resolves outside plugin directory", f.Name)
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		// Extract file with size limit
		if err := extractZipFileWithLimits(f, destPath); err != nil {
			return nil, fmt.Errorf("failed to extract %s: %w", f.Name, err)
		}

		// Track WASM file
		if strings.HasSuffix(cleanName, ".wasm") {
			pkg.WASMPath = destPath
		}

		// Track assets
		if strings.HasPrefix(cleanName, "assets/") {
			assetName := strings.TrimPrefix(cleanName, "assets/")
			pkg.Assets[assetName] = destPath
		}
	}

	return pkg, nil
}

// ValidatePackage checks if a ZIP file is a valid plugin package.
func ValidatePackage(packagePath string) (*plugin.GKRegistration, error) {
	reader, err := zip.OpenReader(packagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open package: %w", err)
	}
	defer reader.Close()

	var hasManifest, hasWasm bool
	var manifest plugin.GKRegistration

	for _, f := range reader.File {
		name := filepath.Base(f.Name)

		if name == "manifest.json" {
			hasManifest = true
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to read manifest: %w", err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read manifest: %w", err)
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("invalid manifest.json: %w", err)
			}
		}

		if strings.HasSuffix(name, ".wasm") {
			hasWasm = true
		}
	}

	if !hasManifest {
		return nil, fmt.Errorf("package missing manifest.json")
	}

	if manifest.Name == "" {
		return nil, fmt.Errorf("manifest missing required 'name' field")
	}

	if !hasWasm {
		return nil, fmt.Errorf("package missing .wasm file")
	}

	return &manifest, nil
}

func addFileToZip(w *zip.Writer, srcPath, zipPath string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

func extractZipFile(f *zip.File, destPath string) error {
	return extractZipFileWithLimits(f, destPath)
}

func extractZipFileWithLimits(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Use LimitReader to prevent files from expanding beyond their declared size
	limitedReader := io.LimitReader(rc, MaxFileSize)
	
	written, err := io.Copy(outFile, limitedReader)
	if err != nil {
		return err
	}

	// Verify that the extracted size matches the expected size
	if written != int64(f.UncompressedSize64) {
		return fmt.Errorf("extracted size mismatch for %s: expected %d, got %d", 
			f.Name, f.UncompressedSize64, written)
	}

	return nil
}
