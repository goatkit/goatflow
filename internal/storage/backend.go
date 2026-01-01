// Package storage provides article attachment storage backends.
package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Backend defines the interface for article storage backends.
type Backend interface {
	// Store saves article content and returns a storage reference
	Store(ctx context.Context, articleID int64, content *ArticleContent) (*StorageReference, error)

	// Retrieve gets article content by reference
	Retrieve(ctx context.Context, ref *StorageReference) (*ArticleContent, error)

	// Delete removes article content
	Delete(ctx context.Context, ref *StorageReference) error

	// Exists checks if article content exists
	Exists(ctx context.Context, ref *StorageReference) (bool, error)

	// List returns all storage references for an article
	List(ctx context.Context, articleID int64) ([]*StorageReference, error)

	// Migrate moves content between backends
	Migrate(ctx context.Context, ref *StorageReference, target Backend) (*StorageReference, error)

	// GetInfo returns backend information
	GetInfo() *BackendInfo

	// HealthCheck verifies backend is operational
	HealthCheck(ctx context.Context) error
}

// ArticleContent represents the content to be stored.
type ArticleContent struct {
	ArticleID   int64
	ContentType string
	FileName    string
	FileSize    int64
	Content     []byte
	Metadata    map[string]string
	CreatedTime time.Time
	CreatedBy   int
}

// StorageReference points to stored content.
type StorageReference struct {
	ID           int64
	ArticleID    int64
	Backend      string
	Location     string
	ContentType  string
	FileName     string
	FileSize     int64
	Checksum     string
	CreatedTime  time.Time
	AccessedTime time.Time
}

// BackendInfo provides information about a storage backend.
type BackendInfo struct {
	Name         string
	Type         string
	Capabilities []string
	Status       string
	Statistics   *BackendStats
}

// BackendStats contains usage statistics.
type BackendStats struct {
	TotalFiles   int64
	TotalSize    int64
	FreeSpace    int64
	ReadLatency  time.Duration
	WriteLatency time.Duration
}

// Factory creates storage backends based on configuration.
type Factory interface {
	// Create instantiates a storage backend
	Create(backendType string, config map[string]interface{}) (Backend, error)

	// Register adds a new backend type
	Register(backendType string, constructor BackendConstructor)

	// List returns available backend types
	List() []string
}

// BackendConstructor creates a new backend instance.
type BackendConstructor func(config map[string]interface{}) (Backend, error)

// DefaultFactory is the global storage backend factory.
var DefaultFactory Factory = NewStorageFactory()

// StorageFactory implements the Factory interface.
type StorageFactory struct {
	constructors map[string]BackendConstructor
}

// NewStorageFactory creates a new storage factory.
func NewStorageFactory() *StorageFactory {
	return &StorageFactory{
		constructors: make(map[string]BackendConstructor),
	}
}

// Create instantiates a storage backend.
func (f *StorageFactory) Create(backendType string, config map[string]interface{}) (Backend, error) {
	constructor, exists := f.constructors[backendType]
	if !exists {
		return nil, fmt.Errorf("unknown storage backend type: %s", backendType)
	}

	return constructor(config)
}

// Register adds a new backend type.
func (f *StorageFactory) Register(backendType string, constructor BackendConstructor) {
	f.constructors[backendType] = constructor
}

// List returns available backend types.
func (f *StorageFactory) List() []string {
	types := make([]string, 0, len(f.constructors))
	for t := range f.constructors {
		types = append(types, t)
	}
	return types
}

// MixedModeBackend supports reading from multiple backends.
type MixedModeBackend struct {
	primary   Backend
	fallbacks []Backend
}

// NewMixedModeBackend creates a backend that checks multiple storage locations.
func NewMixedModeBackend(primary Backend, fallbacks ...Backend) *MixedModeBackend {
	return &MixedModeBackend{
		primary:   primary,
		fallbacks: fallbacks,
	}
}

// Store saves to the primary backend.
func (m *MixedModeBackend) Store(ctx context.Context, articleID int64, content *ArticleContent) (*StorageReference, error) {
	return m.primary.Store(ctx, articleID, content)
}

// Retrieve tries primary first, then fallbacks.
func (m *MixedModeBackend) Retrieve(ctx context.Context, ref *StorageReference) (*ArticleContent, error) {
	// Try primary backend first
	content, err := m.primary.Retrieve(ctx, ref)
	if err == nil {
		return content, nil
	}

	// Try fallback backends
	for _, backend := range m.fallbacks {
		content, err = backend.Retrieve(ctx, ref)
		if err == nil {
			return content, nil
		}
	}

	return nil, fmt.Errorf("content not found in any backend")
}

// Delete removes from all backends.
func (m *MixedModeBackend) Delete(ctx context.Context, ref *StorageReference) error {
	var lastErr error

	// Try to delete from primary
	if err := m.primary.Delete(ctx, ref); err != nil {
		lastErr = err
	}

	// Try to delete from fallbacks
	for _, backend := range m.fallbacks {
		if err := backend.Delete(ctx, ref); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Exists checks all backends.
func (m *MixedModeBackend) Exists(ctx context.Context, ref *StorageReference) (bool, error) {
	// Check primary
	if exists, err := m.primary.Exists(ctx, ref); err == nil && exists {
		return true, nil
	}

	// Check fallbacks
	for _, backend := range m.fallbacks {
		if exists, err := backend.Exists(ctx, ref); err == nil && exists {
			return true, nil
		}
	}

	return false, nil
}

// List combines results from all backends.
func (m *MixedModeBackend) List(ctx context.Context, articleID int64) ([]*StorageReference, error) {
	refs := make([]*StorageReference, 0)
	seen := make(map[string]bool)

	// Get from primary
	if primaryRefs, err := m.primary.List(ctx, articleID); err == nil {
		for _, ref := range primaryRefs {
			key := fmt.Sprintf("%s:%s", ref.Backend, ref.Location)
			if !seen[key] {
				refs = append(refs, ref)
				seen[key] = true
			}
		}
	}

	// Get from fallbacks
	for _, backend := range m.fallbacks {
		if fallbackRefs, err := backend.List(ctx, articleID); err == nil {
			for _, ref := range fallbackRefs {
				key := fmt.Sprintf("%s:%s", ref.Backend, ref.Location)
				if !seen[key] {
					refs = append(refs, ref)
					seen[key] = true
				}
			}
		}
	}

	return refs, nil
}

// Migrate moves content to target backend.
func (m *MixedModeBackend) Migrate(ctx context.Context, ref *StorageReference, target Backend) (*StorageReference, error) {
	// Retrieve from any backend
	content, err := m.Retrieve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve content for migration: %w", err)
	}

	// Store in target
	newRef, err := target.Store(ctx, ref.ArticleID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to store content in target backend: %w", err)
	}

	// Delete from source (optional, depends on migration strategy)
	// m.Delete(ctx, ref)

	return newRef, nil
}

// GetInfo returns mixed mode backend information.
func (m *MixedModeBackend) GetInfo() *BackendInfo {
	return &BackendInfo{
		Name: "MixedMode",
		Type: "mixed",
		Capabilities: []string{
			"read-multiple",
			"write-primary",
			"fallback-support",
		},
		Status: "active",
	}
}

// HealthCheck verifies all backends are operational.
func (m *MixedModeBackend) HealthCheck(ctx context.Context) error {
	// Check primary
	if err := m.primary.HealthCheck(ctx); err != nil {
		return fmt.Errorf("primary backend unhealthy: %w", err)
	}

	// Check fallbacks (don't fail if fallback is down)
	for i, backend := range m.fallbacks {
		if err := backend.HealthCheck(ctx); err != nil {
			// Log warning but don't fail
			fmt.Printf("Warning: fallback backend %d unhealthy: %v\n", i, err)
		}
	}

	return nil
}

// StreamingBackend extends Backend with streaming capabilities.
type StreamingBackend interface {
	Backend

	// StoreStream saves content from a reader
	StoreStream(ctx context.Context, articleID int64, reader io.Reader, metadata *ArticleContent) (*StorageReference, error)

	// RetrieveStream gets content as a reader
	RetrieveStream(ctx context.Context, ref *StorageReference) (io.ReadCloser, error)
}
