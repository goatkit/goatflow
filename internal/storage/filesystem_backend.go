package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// FilesystemBackend implements article storage on the filesystem (OTRS ArticleStorageFS).
type FilesystemBackend struct {
	basePath string
	db       *sql.DB // For storing metadata
}

// NewFilesystemBackend creates a new filesystem storage backend.
func NewFilesystemBackend(basePath string, db *sql.DB) (*FilesystemBackend, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	return &FilesystemBackend{
		basePath: basePath,
		db:       db,
	}, nil
}

// Store saves article content to the filesystem.
func (f *FilesystemBackend) Store(ctx context.Context, articleID int64, content *ArticleContent) (*StorageReference, error) {
	// Calculate checksum
	hash := sha256.Sum256(content.Content)
	checksum := hex.EncodeToString(hash[:])

	// Create directory structure: YYYY/MM/DD/ArticleID/
	now := content.CreatedTime
	dirPath := f.getArticlePath(articleID, now)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Determine filename
	filename := content.FileName
	if filename == "" || filename == "body" {
		filename = "plain.txt"
		if content.ContentType == "text/html" {
			filename = "file-2" // HTML part
		} else {
			filename = "file-1" // Plain text part
		}
	}

	// Write content to file
	filePath := filepath.Join(dirPath, filename)
	if err := os.WriteFile(filePath, content.Content, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Write metadata file
	metadataPath := filepath.Join(dirPath, filename+".meta")
	metadata := map[string]interface{}{
		"article_id":   articleID,
		"content_type": content.ContentType,
		"file_size":    content.FileSize,
		"checksum":     checksum,
		"created_time": content.CreatedTime,
		"created_by":   content.CreatedBy,
		"metadata":     content.Metadata,
	}

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	// Store reference in database
	ref := &StorageReference{
		ArticleID:   articleID,
		Backend:     "FS",
		Location:    filePath,
		ContentType: content.ContentType,
		FileName:    filename,
		FileSize:    content.FileSize,
		Checksum:    checksum,
		CreatedTime: content.CreatedTime,
	}

	if err := f.storeReference(ctx, ref); err != nil {
		// Clean up files on error
		os.Remove(filePath)
		os.Remove(metadataPath)
		return nil, fmt.Errorf("failed to store reference: %w", err)
	}

	return ref, nil
}

// Retrieve gets article content from the filesystem.
func (f *FilesystemBackend) Retrieve(ctx context.Context, ref *StorageReference) (*ArticleContent, error) {
	// Read content from file
	contentBytes, err := os.ReadFile(ref.Location)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", ref.Location)
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Read metadata
	metadataPath := ref.Location + ".meta"
	var metadata map[string]interface{}

	if metadataBytes, err := os.ReadFile(metadataPath); err == nil {
		if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
			// Metadata loaded successfully
		}
	}

	content := &ArticleContent{
		ArticleID:   ref.ArticleID,
		ContentType: ref.ContentType,
		FileName:    ref.FileName,
		FileSize:    int64(len(contentBytes)),
		Content:     contentBytes,
		Metadata:    make(map[string]string),
		CreatedTime: ref.CreatedTime,
	}

	// Extract metadata strings
	if metadata != nil {
		if meta, ok := metadata["metadata"].(map[string]interface{}); ok {
			for k, v := range meta {
				if str, ok := v.(string); ok {
					content.Metadata[k] = str
				}
			}
		}
		if createdBy, ok := metadata["created_by"].(float64); ok {
			content.CreatedBy = int(createdBy)
		}
	}

	return content, nil
}

// Delete removes article content from the filesystem.
func (f *FilesystemBackend) Delete(ctx context.Context, ref *StorageReference) error {
	// Remove file
	if err := os.Remove(ref.Location); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Remove metadata file
	metadataPath := ref.Location + ".meta"
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		// Don't fail if metadata doesn't exist
	}

	// Try to remove directory if empty
	dir := filepath.Dir(ref.Location)
	os.Remove(dir) // Ignore error, directory might not be empty

	// Remove reference from database
	if err := f.deleteReference(ctx, ref); err != nil {
		return fmt.Errorf("failed to delete reference: %w", err)
	}

	return nil
}

// Exists checks if article content exists on the filesystem.
func (f *FilesystemBackend) Exists(ctx context.Context, ref *StorageReference) (bool, error) {
	_, err := os.Stat(ref.Location)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// List returns all storage references for an article.
func (f *FilesystemBackend) List(ctx context.Context, articleID int64) ([]*StorageReference, error) {
	// Get references from database
	refs, err := f.getReferences(ctx, articleID)
	if err != nil {
		return nil, err
	}

	// Verify files still exist
	validRefs := make([]*StorageReference, 0, len(refs))
	for _, ref := range refs {
		if _, err := os.Stat(ref.Location); err == nil {
			validRefs = append(validRefs, ref)
		}
	}

	return validRefs, nil
}

// Migrate moves content to another backend.
func (f *FilesystemBackend) Migrate(ctx context.Context, ref *StorageReference, target Backend) (*StorageReference, error) {
	// Retrieve content
	content, err := f.Retrieve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve content for migration: %w", err)
	}

	// Store in target
	newRef, err := target.Store(ctx, ref.ArticleID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to store content in target: %w", err)
	}

	return newRef, nil
}

// GetInfo returns backend information.
func (f *FilesystemBackend) GetInfo() *BackendInfo {
	stats := &BackendStats{}

	// Get filesystem statistics
	var totalSize int64
	var totalFiles int64

	filepath.Walk(f.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && !strings.HasSuffix(path, ".meta") {
			totalFiles++
			totalSize += info.Size()
		}
		return nil
	})

	stats.TotalFiles = totalFiles
	stats.TotalSize = totalSize

	// Get free space
	if _, err := os.Stat(f.basePath); err == nil {
		// This is platform-specific and would need proper implementation
		stats.FreeSpace = 0 // TODO: Implement free space calculation
	}

	return &BackendInfo{
		Name: "FilesystemBackend",
		Type: "FS",
		Capabilities: []string{
			"store",
			"retrieve",
			"delete",
			"list",
			"streaming",
		},
		Status:     "active",
		Statistics: stats,
	}
}

// HealthCheck verifies the filesystem is accessible.
func (f *FilesystemBackend) HealthCheck(ctx context.Context) error {
	// Check if base path is accessible
	testFile := filepath.Join(f.basePath, ".health_check")
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return fmt.Errorf("filesystem not writable: %w", err)
	}

	if err := os.Remove(testFile); err != nil {
		return fmt.Errorf("filesystem cleanup failed: %w", err)
	}

	// Check database connection
	if f.db != nil {
		if err := f.db.PingContext(ctx); err != nil {
			return fmt.Errorf("database not accessible: %w", err)
		}
	}

	return nil
}

// Helper methods

func (f *FilesystemBackend) getArticlePath(articleID int64, t time.Time) string {
	year := t.Format("2006")
	month := t.Format("01")
	day := t.Format("02")
	return filepath.Join(f.basePath, year, month, day, fmt.Sprintf("%d", articleID))
}

func (f *FilesystemBackend) storeReference(ctx context.Context, ref *StorageReference) error {
	if f.db == nil {
		return nil // No database, references not tracked
	}

	query := database.ConvertPlaceholders(`
        INSERT INTO article_storage_references (
            article_id, backend, location, content_type,
            file_name, file_size, checksum, created_time
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `)

	if database.IsMySQL() {
		// Try update first
		_, _ = f.db.ExecContext(ctx, database.ConvertPlaceholders(`
            UPDATE article_storage_references
            SET location = $4, checksum = $7, accessed_time = NOW()
            WHERE article_id = $1 AND file_name = $5 AND backend = $2
        `),
			ref.ArticleID, ref.Backend, ref.Location, ref.Location, ref.FileName, ref.FileSize, ref.Checksum,
		)
		res, err := f.db.ExecContext(ctx, query,
			ref.ArticleID,
			ref.Backend,
			ref.Location,
			ref.ContentType,
			ref.FileName,
			ref.FileSize,
			ref.Checksum,
			ref.CreatedTime,
		)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		ref.ID = id
		return nil
	}

	err := f.db.QueryRowContext(ctx, query+" RETURNING id",
		ref.ArticleID,
		ref.Backend,
		ref.Location,
		ref.ContentType,
		ref.FileName,
		ref.FileSize,
		ref.Checksum,
		ref.CreatedTime,
	).Scan(&ref.ID)

	return err
}

func (f *FilesystemBackend) getReferences(ctx context.Context, articleID int64) ([]*StorageReference, error) {
	if f.db == nil {
		// No database, scan filesystem
		return f.scanFilesystem(articleID)
	}

	query := `
		SELECT 
			id, article_id, backend, location, content_type,
			file_name, file_size, checksum, created_time, accessed_time
		FROM article_storage_references
		WHERE article_id = $1 AND backend = 'FS'
		ORDER BY id`

	rows, err := f.db.QueryContext(ctx, query, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]*StorageReference, 0)
	for rows.Next() {
		var ref StorageReference
		var accessedTime sql.NullTime

		err := rows.Scan(
			&ref.ID,
			&ref.ArticleID,
			&ref.Backend,
			&ref.Location,
			&ref.ContentType,
			&ref.FileName,
			&ref.FileSize,
			&ref.Checksum,
			&ref.CreatedTime,
			&accessedTime,
		)
		if err != nil {
			continue
		}

		if accessedTime.Valid {
			ref.AccessedTime = accessedTime.Time
		}

		refs = append(refs, &ref)
	}

	return refs, nil
}

func (f *FilesystemBackend) scanFilesystem(articleID int64) ([]*StorageReference, error) {
	refs := make([]*StorageReference, 0)

	// Search for article directories
	pattern := filepath.Join(f.basePath, "*", "*", "*", fmt.Sprintf("%d", articleID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, dir := range matches {
		// List files in directory
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || strings.HasSuffix(file.Name(), ".meta") {
				continue
			}

			info, err := file.Info()
			if err != nil {
				continue
			}

			ref := &StorageReference{
				ArticleID:   articleID,
				Backend:     "FS",
				Location:    filepath.Join(dir, file.Name()),
				FileName:    file.Name(),
				FileSize:    info.Size(),
				CreatedTime: info.ModTime(),
			}

			// Try to read metadata
			if metaBytes, err := os.ReadFile(ref.Location + ".meta"); err == nil {
				var metadata map[string]interface{}
				if err := json.Unmarshal(metaBytes, &metadata); err == nil {
					if ct, ok := metadata["content_type"].(string); ok {
						ref.ContentType = ct
					}
					if cs, ok := metadata["checksum"].(string); ok {
						ref.Checksum = cs
					}
				}
			}

			refs = append(refs, ref)
		}
	}

	return refs, nil
}

func (f *FilesystemBackend) deleteReference(ctx context.Context, ref *StorageReference) error {
	if f.db == nil {
		return nil
	}

	query := `
		DELETE FROM article_storage_references
		WHERE article_id = $1 AND backend = 'FS' AND location = $2`

	_, err := f.db.ExecContext(ctx, query, ref.ArticleID, ref.Location)
	return err
}

// Register the filesystem backend with the factory.
func init() {
	DefaultFactory.Register("FS", func(config map[string]interface{}) (Backend, error) {
		basePath, ok := config["base_path"].(string)
		if !ok {
			return nil, fmt.Errorf("filesystem backend requires 'base_path' configuration")
		}

		// Database is optional for reference tracking
		var db *sql.DB
		if dbInterface, ok := config["db"]; ok {
			db, _ = dbInterface.(*sql.DB)
		}

		return NewFilesystemBackend(basePath, db)
	})
}
