package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StorageType represents the type of storage backend.
type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeS3    StorageType = "s3"
)

// FileMetadata contains metadata about a stored file.
type FileMetadata struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	StoragePath  string    `json:"storage_path"`
	ContentType  string    `json:"content_type"`
	Size         int64     `json:"size"`
	Checksum     string    `json:"checksum"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

// StorageService defines the interface for file storage operations.
type StorageService interface {
	// Store saves a file and returns its metadata
	Store(ctx context.Context, file multipart.File, header *multipart.FileHeader, path string) (*FileMetadata, error)

	// Retrieve gets a file by its storage path
	Retrieve(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete removes a file from storage
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists
	Exists(ctx context.Context, path string) (bool, error)

	// GetURL returns a URL for accessing the file (for S3 pre-signed URLs)
	GetURL(ctx context.Context, path string, expiry time.Duration) (string, error)

	// GetMetadata retrieves file metadata without downloading the file
	GetMetadata(ctx context.Context, path string) (*FileMetadata, error)
}

// Context keys for passing storage-related metadata (article/user IDs).
type ctxKey string

const (
	// CtxKeyArticleID is the context key to pass the target article ID.
	CtxKeyArticleID ctxKey = "article_id"
	// CtxKeyUserID is the context key to pass the acting user ID.
	CtxKeyUserID ctxKey = "user_id"
)

// WithArticleID attaches an article ID to context.
func WithArticleID(ctx context.Context, articleID int) context.Context {
	return context.WithValue(ctx, CtxKeyArticleID, articleID)
}

// ArticleIDFromContext retrieves article ID from context.
func ArticleIDFromContext(ctx context.Context) (int, bool) {
	v := ctx.Value(CtxKeyArticleID)
	if v == nil {
		return 0, false
	}
	if id, ok := v.(int); ok {
		return id, true
	}
	return 0, false
}

// WithUserID attaches a user ID to context.
func WithUserID(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, CtxKeyUserID, userID)
}

// UserIDFromContext retrieves user ID from context.
func UserIDFromContext(ctx context.Context) (int, bool) {
	v := ctx.Value(CtxKeyUserID)
	if v == nil {
		return 0, false
	}
	if id, ok := v.(int); ok {
		return id, true
	}
	return 0, false
}

// LocalStorageService implements StorageService for local file system.
type LocalStorageService struct {
	basePath string
}

// NewLocalStorageService creates a new local storage service.
func NewLocalStorageService(basePath string) (*LocalStorageService, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Make absolute path
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	return &LocalStorageService{
		basePath: absPath,
	}, nil
}

// Store saves a file to the local file system.
func (s *LocalStorageService) Store(ctx context.Context, file multipart.File, header *multipart.FileHeader, path string) (*FileMetadata, error) {
	// Sanitize the path to prevent directory traversal
	cleanPath := sanitizePath(path)
	fullPath := filepath.Join(s.basePath, cleanPath)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the file
	dst, err := os.Create(fullPath) //nolint:gosec // G304 false positive - sanitized path
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy file content
	size, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(fullPath) // Clean up on error
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Generate metadata
	metadata := &FileMetadata{
		ID:           generateFileID(),
		OriginalName: header.Filename,
		StoragePath:  cleanPath,
		ContentType:  header.Header.Get("Content-Type"),
		Size:         size,
		UploadedAt:   time.Now(),
	}

	return metadata, nil
}

// Retrieve gets a file from the local file system.
func (s *LocalStorageService) Retrieve(ctx context.Context, path string) (io.ReadCloser, error) {
	cleanPath := sanitizePath(path)
	fullPath := filepath.Join(s.basePath, cleanPath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	// Open file
	file, err := os.Open(fullPath) //nolint:gosec // G304 false positive - sanitized path
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete removes a file from the local file system.
func (s *LocalStorageService) Delete(ctx context.Context, path string) error {
	cleanPath := sanitizePath(path)
	fullPath := filepath.Join(s.basePath, cleanPath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil // File doesn't exist, consider it deleted
	}

	// Delete file
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Try to remove empty directories (ignore errors)
	dir := filepath.Dir(fullPath)
	os.Remove(dir) // Will fail if not empty, which is fine

	return nil
}

// Exists checks if a file exists in the local file system.
func (s *LocalStorageService) Exists(ctx context.Context, path string) (bool, error) {
	cleanPath := sanitizePath(path)
	fullPath := filepath.Join(s.basePath, cleanPath)

	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// GetURL returns the file path for local storage (no pre-signed URLs).
func (s *LocalStorageService) GetURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	// For local storage, just return the path
	// In production, this might return a URL to a file server
	return "/files/" + sanitizePath(path), nil
}

// GetMetadata retrieves file metadata.
func (s *LocalStorageService) GetMetadata(ctx context.Context, path string) (*FileMetadata, error) {
	cleanPath := sanitizePath(path)
	fullPath := filepath.Join(s.basePath, cleanPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	metadata := &FileMetadata{
		ID:           generateFileID(),
		OriginalName: filepath.Base(path),
		StoragePath:  cleanPath,
		Size:         info.Size(),
		UploadedAt:   info.ModTime(),
	}

	return metadata, nil
}

// Helper functions

// sanitizePath cleans a file path to prevent directory traversal attacks.
func sanitizePath(path string) string {
	// Remove any .. or . components
	path = filepath.Clean(path)

	// Remove leading slashes
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "\\")

	// Replace any remaining .. with _
	path = strings.ReplaceAll(path, "..", "_")

	return path
}

// generateFileID creates a unique file identifier.
func generateFileID() string {
	// In production, use a proper UUID library
	return fmt.Sprintf("%d_%d", time.Now().Unix(), time.Now().Nanosecond())
}

// (Legacy GoatFlow layout removed)

// GenerateOTRSStoragePath creates an OTRS-style path for storing files
// Format: var/article/{year}/{month}/{day}/{ticket_id}/{article_id}/{filename}
// Note: We do not add a timestamp suffix here as uniqueness is provided by article_id scoping.
func GenerateOTRSStoragePath(ticketID int, articleID int, filename string) string {
	now := time.Now()

	// Sanitize filename
	safeFilename := sanitizeFilename(filename)
	// Ensure uniqueness for mock/no-article flows
	if articleID <= 0 {
		ext := filepath.Ext(safeFilename)
		nameWithoutExt := strings.TrimSuffix(safeFilename, ext)
		safeFilename = fmt.Sprintf("%s_%d%s", nameWithoutExt, now.Unix(), ext)
		articleID = 0
	}

	// Build path mirroring OTRS article storage directory structure
	path := fmt.Sprintf("var/article/%d/%02d/%02d/%d/%d/%s",
		now.Year(),
		now.Month(),
		now.Day(),
		ticketID,
		articleID,
		safeFilename,
	)

	return path
}

// sanitizeFilename makes a filename safe for storage.
func sanitizeFilename(filename string) string {
	// Remove directory components
	filename = filepath.Base(filename)

	// Replace problematic characters
	replacer := strings.NewReplacer(
		" ", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"{", "_",
		"}", "_",
		"<", "_",
		">", "_",
		":", "_",
		";", "_",
		",", "_",
		"?", "_",
		"*", "_",
		"|", "_",
		"\\", "_",
		"/", "_",
		"\"", "_",
		"'", "_",
	)

	safe := replacer.Replace(filename)

	// Ensure it's not empty
	if safe == "" {
		safe = "unnamed_file"
	}

	// Limit length
	if len(safe) > 255 {
		ext := filepath.Ext(safe)
		safe = safe[:255-len(ext)] + ext
	}

	return safe
}
