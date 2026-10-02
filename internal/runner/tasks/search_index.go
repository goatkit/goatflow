package tasks

import (
	"context"
	"database/sql"
	"log"
	"sync/atomic"
	"time"

	"github.com/goatkit/goatflow/internal/platform/runner"
	"github.com/goatkit/goatflow/internal/platform/search"
)

// SearchIndexTask keeps the Zinc/Elasticsearch index in step with the
// database: it builds the index when needed and reindexes changed tickets,
// articles and customer users.
type SearchIndexTask struct {
	db      *sql.DB
	backend *search.ExternalBackend
	running atomic.Bool
}

// NewSearchIndexTask creates the search index task for backend.
func NewSearchIndexTask(db *sql.DB, backend *search.ExternalBackend) runner.Task {
	return &SearchIndexTask{db: db, backend: backend}
}

// Name returns the task name.
func (t *SearchIndexTask) Name() string { return "search-index" }

// Schedule runs the task every 30 seconds.
func (t *SearchIndexTask) Schedule() string { return "*/30 * * * * *" }

// Timeout bounds one run; a first build of a large database can take long.
func (t *SearchIndexTask) Timeout() time.Duration { return 2 * time.Hour }

// Run syncs the index. A run that starts while the previous one is still
// going returns immediately.
func (t *SearchIndexTask) Run(ctx context.Context) error {
	if !t.running.CompareAndSwap(false, true) {
		return nil
	}
	defer t.running.Store(false)
	stats, err := t.backend.Sync(ctx, t.db)
	switch {
	case err != nil:
		log.Printf("search-index: %s sync failed: %v", t.backend.GetBackendName(), err)
	case stats.Rebuilt:
		log.Printf("search-index: built the %s index", t.backend.GetBackendName())
	case stats.Tickets > 0 || stats.Customers > 0:
		log.Printf("search-index: reindexed %d tickets, %d customer users", stats.Tickets, stats.Customers)
	}
	return err
}
