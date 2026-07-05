package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

// FileStorageBackend defines the interface for pluggable storage backends.
type FileStorageBackend interface {
	Store(path string, data []byte, metadata map[string]string) error
	Get(path string) ([]byte, map[string]string, error)
	Delete(path string) error
	List(prefix string) ([]pkgplugin.FileInfo, error)
	Usage(prefix string) (int64, error) // total bytes under prefix
}

// fileStorageBackend is the active backend, set at startup.
var (
	activeBackend   FileStorageBackend
	backendInitOnce sync.Once
)

// initBackend lazily initialises the storage backend from env.
func initBackend() FileStorageBackend {
	backendInitOnce.Do(func() {
		switch os.Getenv("GOATFLOW_STORAGE_BACKEND") {
		case "s3":
			activeBackend = newS3Backend()
		default:
			activeBackend = newLocalBackend()
		}
	})
	return activeBackend
}

// ---- Path helpers ----

// pluginFilePath builds the namespaced path: <plugin>/<key> or <plugin>/org-<id>/<key>
func pluginFilePath(pluginName string, orgID int64, key string) (string, error) {
	clean := filepath.Clean(key)
	if strings.Contains(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid file key: %q", key)
	}
	if orgID > 0 {
		return filepath.Join(pluginName, fmt.Sprintf("org-%d", orgID), clean), nil
	}
	return filepath.Join(pluginName, clean), nil
}

// pluginPrefix builds the namespace prefix for listing/usage.
func pluginPrefix(pluginName string, orgID int64) string {
	if orgID > 0 {
		return filepath.Join(pluginName, fmt.Sprintf("org-%d", orgID))
	}
	return pluginName
}

// ---- ProdHostAPI file storage methods ----

func (h *ProdHostAPI) StoreFile(ctx context.Context, key string, data []byte, metadata map[string]string) error {
	pluginName := pluginNameFromCtx(ctx)
	if pluginName == "" {
		return fmt.Errorf("no plugin context for StoreFile")
	}

	orgID := orgIDFromCtx(ctx)

	// Check size limit from resource policy.
	if err := checkFileStorageLimit(pluginName, orgID, int64(len(data))); err != nil {
		return err
	}

	path, err := pluginFilePath(pluginName, orgID, key)
	if err != nil {
		return err
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["_size"] = fmt.Sprintf("%d", len(data))
	metadata["_modified"] = time.Now().UTC().Format(time.RFC3339)

	return initBackend().Store(path, data, metadata)
}

func (h *ProdHostAPI) GetFile(ctx context.Context, key string) ([]byte, map[string]string, error) {
	pluginName := pluginNameFromCtx(ctx)
	if pluginName == "" {
		return nil, nil, fmt.Errorf("no plugin context for GetFile")
	}

	path, err := pluginFilePath(pluginName, orgIDFromCtx(ctx), key)
	if err != nil {
		return nil, nil, err
	}

	return initBackend().Get(path)
}

func (h *ProdHostAPI) DeleteFile(ctx context.Context, key string) error {
	pluginName := pluginNameFromCtx(ctx)
	if pluginName == "" {
		return fmt.Errorf("no plugin context for DeleteFile")
	}

	path, err := pluginFilePath(pluginName, orgIDFromCtx(ctx), key)
	if err != nil {
		return err
	}

	return initBackend().Delete(path)
}

func (h *ProdHostAPI) ListFiles(ctx context.Context, prefix string) ([]pkgplugin.FileInfo, error) {
	pluginName := pluginNameFromCtx(ctx)
	if pluginName == "" {
		return nil, fmt.Errorf("no plugin context for ListFiles")
	}

	path, err := pluginFilePath(pluginName, orgIDFromCtx(ctx), prefix)
	if err != nil {
		return nil, err
	}

	return initBackend().List(path)
}

// GenerateThumbnail generates a thumbnail from image data using the
// platform's ThumbnailService (libvips via govips). For non-image types,
// returns a placeholder icon. The plugin does not need libvips — the
// host handles all image processing.
func (h *ProdHostAPI) GenerateThumbnail(ctx context.Context, data []byte, contentType string, maxWidth, maxHeight int) ([]byte, string, error) {
	if h.thumbnailService == nil {
		return nil, "", fmt.Errorf("thumbnail service not configured")
	}
	return h.thumbnailService.GenerateThumbnail(data, contentType, maxWidth, maxHeight)
}

// ---- Size limit enforcement ----

// maxPluginStorageBytes is the default per-plugin storage limit (500MB).
const maxPluginStorageBytes int64 = 500 * 1024 * 1024

// checkFileStorageLimit verifies the plugin hasn't exceeded its storage quota.
func checkFileStorageLimit(pluginName string, orgID int64, additionalBytes int64) error {
	limit := maxPluginStorageBytes

	// Check if the plugin has a custom limit via resource policy.
	if globalManager != nil {
		if policy := globalManager.PolicyFor(pluginName); policy != nil && policy.MaxFileStorageBytes > 0 {
			limit = policy.MaxFileStorageBytes
		}
	}

	prefix := pluginPrefix(pluginName, orgID)
	current, err := initBackend().Usage(prefix)
	if err != nil {
		return nil // can't check, allow through
	}

	if current+additionalBytes > limit {
		return fmt.Errorf("plugin %q storage limit exceeded (%d + %d > %d bytes)",
			pluginName, current, additionalBytes, limit)
	}
	return nil
}

// globalManager is set by the plugin system at startup for policy lookups.
var globalManager *Manager

// SetGlobalManager sets the plugin manager for file storage policy lookups.
func SetGlobalManager(m *Manager) {
	globalManager = m
}

// ---- Context helpers ----

func pluginNameFromCtx(ctx context.Context) string {
	if v := ctx.Value(PluginCallerKey); v != nil {
		if name, ok := v.(string); ok {
			return name
		}
	}
	return ""
}

func orgIDFromCtx(ctx context.Context) int64 {
	// Try the organisation context key used by the org middleware.
	if v := ctx.Value("org_id"); v != nil {
		switch id := v.(type) {
		case int64:
			return id
		case int:
			return int64(id)
		}
	}
	return 0
}

// ==== Local Disk Backend ====

type localBackend struct {
	base string
}

func newLocalBackend() *localBackend {
	base := os.Getenv("STORAGE_PATH")
	if base == "" {
		base = "/app/storage"
	}
	return &localBackend{base: filepath.Join(base, "plugins")}
}

func (b *localBackend) Store(path string, data []byte, metadata map[string]string) error {
	fullPath := filepath.Join(b.base, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	metaJSON, _ := json.Marshal(metadata)
	os.WriteFile(fullPath+".meta.json", metaJSON, 0644)
	return nil
}

func (b *localBackend) Get(path string) ([]byte, map[string]string, error) {
	fullPath := filepath.Join(b.base, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read file: %w", err)
	}
	var metadata map[string]string
	if metaJSON, err := os.ReadFile(fullPath + ".meta.json"); err == nil {
		json.Unmarshal(metaJSON, &metadata)
	}
	return data, metadata, nil
}

func (b *localBackend) Delete(path string) error {
	fullPath := filepath.Join(b.base, path)
	os.Remove(fullPath + ".meta.json")
	return os.Remove(fullPath)
}

func (b *localBackend) List(prefix string) ([]pkgplugin.FileInfo, error) {
	fullPath := filepath.Join(b.base, prefix)
	var files []pkgplugin.FileInfo

	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || strings.HasSuffix(path, ".meta.json") {
			return nil
		}
		relKey, _ := filepath.Rel(b.base, path)
		fi := pkgplugin.FileInfo{
			Key:        relKey,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
		}
		if metaJSON, err := os.ReadFile(path + ".meta.json"); err == nil {
			var meta map[string]string
			json.Unmarshal(metaJSON, &meta)
			fi.Metadata = meta
			fi.ContentType = meta["content-type"]
		}
		files = append(files, fi)
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return files, nil
}

func (b *localBackend) Usage(prefix string) (int64, error) {
	fullPath := filepath.Join(b.base, prefix)
	var total int64
	filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || strings.HasSuffix(path, ".meta.json") {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, nil
}

// ==== S3-Compatible Backend ====

type s3Backend struct {
	endpoint  string
	bucket    string
	accessKey string
	secretKey string
	region    string
}

func newS3Backend() *s3Backend {
	b := &s3Backend{
		endpoint:  os.Getenv("GOATFLOW_S3_ENDPOINT"),
		bucket:    os.Getenv("GOATFLOW_S3_BUCKET"),
		accessKey: os.Getenv("GOATFLOW_S3_ACCESS_KEY"),
		secretKey: os.Getenv("GOATFLOW_S3_SECRET_KEY"),
		region:    os.Getenv("GOATFLOW_S3_REGION"),
	}
	if b.bucket == "" {
		b.bucket = "goatflow-plugins"
	}
	if b.region == "" {
		b.region = "us-east-1"
	}
	log.Printf("📦 S3 file storage: endpoint=%s bucket=%s", b.endpoint, b.bucket)
	return b
}

// S3 backend uses presigned URLs and standard HTTP — no AWS SDK dependency.
// This is a minimal implementation for S3-compatible stores (MinIO, R2, AWS S3).

func (b *s3Backend) Store(path string, data []byte, metadata map[string]string) error {
	// PUT object
	url := b.objectURL(path)
	req, err := http.NewRequest("PUT", url, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if ct, ok := metadata["content-type"]; ok {
		req.Header.Set("Content-Type", ct)
	}
	b.sign(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("s3 PUT: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 PUT %s: %d %s", path, resp.StatusCode, string(body))
	}

	// Store metadata as a separate object.
	if metadata != nil {
		metaJSON, _ := json.Marshal(metadata)
		metaReq, _ := http.NewRequest("PUT", b.objectURL(path+".meta.json"), strings.NewReader(string(metaJSON)))
		metaReq.Header.Set("Content-Type", "application/json")
		b.sign(metaReq)
		metaResp, err := http.DefaultClient.Do(metaReq)
		if err == nil {
			metaResp.Body.Close()
		}
	}

	return nil
}

func (b *s3Backend) Get(path string) ([]byte, map[string]string, error) {
	url := b.objectURL(path)
	req, _ := http.NewRequest("GET", url, nil)
	b.sign(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, nil, fmt.Errorf("file not found: %s", path)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	// Read metadata.
	var metadata map[string]string
	metaReq, _ := http.NewRequest("GET", b.objectURL(path+".meta.json"), nil)
	b.sign(metaReq)
	if metaResp, err := http.DefaultClient.Do(metaReq); err == nil {
		defer metaResp.Body.Close()
		if metaResp.StatusCode == 200 {
			metaBody, _ := io.ReadAll(metaResp.Body)
			json.Unmarshal(metaBody, &metadata)
		}
	}

	return data, metadata, nil
}

func (b *s3Backend) Delete(path string) error {
	req, _ := http.NewRequest("DELETE", b.objectURL(path), nil)
	b.sign(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// Delete metadata too.
	metaReq, _ := http.NewRequest("DELETE", b.objectURL(path+".meta.json"), nil)
	b.sign(metaReq)
	if metaResp, err := http.DefaultClient.Do(metaReq); err == nil {
		metaResp.Body.Close()
	}

	return nil
}

func (b *s3Backend) List(prefix string) ([]pkgplugin.FileInfo, error) {
	// S3 ListObjectsV2 — minimal implementation.
	url := fmt.Sprintf("%s/%s?list-type=2&prefix=%s", b.endpoint, b.bucket, prefix)
	req, _ := http.NewRequest("GET", url, nil)
	b.sign(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Parse XML response — simplified, returns empty for now.
	// Full XML parsing can be added when S3 backend is actively used.
	log.Printf("⚠️  S3 ListFiles not fully implemented — use local backend for full listing support")
	return nil, nil
}

func (b *s3Backend) Usage(prefix string) (int64, error) {
	// Would need ListObjectsV2 + sum sizes. Return 0 for now.
	return 0, nil
}

func (b *s3Backend) objectURL(path string) string {
	return fmt.Sprintf("%s/%s/%s", b.endpoint, b.bucket, path)
}

// sign adds S3 v4 auth headers. This is a placeholder — for production use,
// integrate a proper S3 signing library or use pre-signed URLs.
func (b *s3Backend) sign(req *http.Request) {
	// Minimal: use basic auth header for S3-compatible stores that support it.
	// For AWS S3 proper, this needs SigV4 signing.
	if b.accessKey != "" {
		req.SetBasicAuth(b.accessKey, b.secretKey)
	}
}
