package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goatkit/goatflow/internal/platform/config"
)

// DefaultStorageRoot is the storage root when neither STORAGE_PATH nor
// storage.local.path is set.
const DefaultStorageRoot = "/app/storage"

// Config selects the article storage backend.
type Config struct {
	// Backend is BackendDB or BackendFS (case-insensitive).
	Backend string
	// ArticleDir is the ArticleStorageFS root (OTRS ArticleDataDir); FS only.
	ArticleDir string
}

var (
	activeMu sync.RWMutex
	active   = Config{Backend: BackendDB}
)

// Configure validates cfg and makes it the backend used by ForDB. For FS the
// article directory is created when missing.
func Configure(cfg Config) error {
	cfg.Backend = strings.ToUpper(strings.TrimSpace(cfg.Backend))
	switch cfg.Backend {
	case BackendDB:
		cfg.ArticleDir = ""
	case BackendFS:
		if cfg.ArticleDir == "" {
			return fmt.Errorf("article storage FS needs an article directory")
		}
		if err := os.MkdirAll(cfg.ArticleDir, dirPerm); err != nil {
			return fmt.Errorf("article storage directory %s: %w", cfg.ArticleDir, err)
		}
	default:
		return fmt.Errorf("unknown article storage backend %q (want DB or FS)", cfg.Backend)
	}
	activeMu.Lock()
	active = cfg
	activeMu.Unlock()
	return nil
}

// Active returns the configured backend.
func Active() Config {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return active
}

// ForDB returns the configured article store on db.
func ForDB(db *sql.DB) ArticleStore {
	return New(Active(), db)
}

// New returns the store for a validated cfg.
func New(cfg Config, db *sql.DB) ArticleStore {
	if strings.EqualFold(cfg.Backend, BackendFS) {
		return NewFilesystemStore(cfg.ArticleDir, db)
	}
	return NewDatabaseStore(db)
}

// ConfigFromApp resolves the article storage configuration: STORAGE_TYPE or
// storage.type picks the backend (DB when unset); the FS tree is
// <root>/var/article, root being STORAGE_PATH, storage.local.path or
// DefaultStorageRoot. cfg may be nil.
func ConfigFromApp(cfg *config.Config) Config {
	backend := os.Getenv("STORAGE_TYPE")
	root := os.Getenv("STORAGE_PATH")
	if cfg != nil {
		if backend == "" {
			backend = cfg.Storage.Type
		}
		if root == "" {
			root = cfg.Storage.Local.Path
		}
	}
	if backend == "" {
		backend = BackendDB
	}
	if root == "" {
		root = DefaultStorageRoot
	}
	return Config{Backend: backend, ArticleDir: filepath.Join(root, "var", "article")}
}
