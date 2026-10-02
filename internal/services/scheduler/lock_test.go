package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
)

func lockTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("scheduler lock tests need a test database (TEST_DB_HOST)")
	}
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

// replica builds a scheduler as one backend replica would: same DB, same
// job definition, its own handler counting runs into runs.
func replica(t *testing.T, db *sql.DB, job *models.ScheduledJob, runs *atomic.Int32) *Service {
	t.Helper()
	svc := NewService(db, WithJobs([]*models.ScheduledJob{job}))
	svc.RegisterHandler(job.Handler, func(context.Context, *models.ScheduledJob) error {
		runs.Add(1)
		return nil
	})
	return svc
}

func lockRow(t *testing.T, db *sql.DB, slug string) (lockedUntil time.Time, lockedBy string) {
	t.Helper()
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT locked_until, locked_by FROM gk_scheduler_job_lock WHERE job_slug = ?`), slug).
		Scan(&lockedUntil, &lockedBy))
	return lockedUntil, lockedBy
}

// Two schedulers against one database fire the same tick: the job must run
// once, and the next tick must be runnable again by either replica.
func TestSchedulerJobRunsOncePerTickAcrossReplicas(t *testing.T) {
	db := lockTestDB(t)
	slug := fmt.Sprintf("lock-test-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_scheduler_job_lock WHERE job_slug = ?`), slug)
	})
	job := &models.ScheduledJob{Slug: slug, Handler: slug, Schedule: "@every 4s"}

	var runs atomic.Int32
	a := replica(t, db, job, &runs)
	b := replica(t, db, job, &runs)
	require.NotEqual(t, a.lock.owner, b.lock.owner)

	fireBoth := func() {
		var wg sync.WaitGroup
		start := make(chan struct{})
		for _, svc := range []*Service{a, b} {
			wg.Add(1)
			go func(svc *Service) {
				defer wg.Done()
				<-start
				svc.executeJob(slug, 0)
			}(svc)
		}
		close(start)
		wg.Wait()
	}

	// Tick 1: both replicas fire together.
	fireBoth()
	require.EqualValues(t, 1, runs.Load(), "tick 1 must run on exactly one replica")
	until, owner := lockRow(t, db, slug)
	require.Contains(t, []string{a.lock.owner, b.lock.owner}, owner)
	require.True(t, until.After(time.Now().UTC().Add(-time.Second)), "claim must reach into the future, got %s", until)

	// A late duplicate of tick 1 (clock skew) is still refused.
	b.executeJob(slug, 0)
	require.EqualValues(t, 1, runs.Load(), "a late duplicate of tick 1 must not run")

	// Tick 2 (one interval later): the claim from tick 1 has expired.
	time.Sleep(4 * time.Second)
	fireBoth()
	require.EqualValues(t, 2, runs.Load(), "tick 2 must run on exactly one replica")
}

// A job without a lock row yet (first run after the migration, or a plugin
// job registered later) is claimable at once.
func TestSchedulerJobLockCreatesMissingRow(t *testing.T) {
	db := lockTestDB(t)
	slug := fmt.Sprintf("lock-new-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_scheduler_job_lock WHERE job_slug = ?`), slug)
	})
	job := &models.ScheduledJob{Slug: slug, Handler: slug, Schedule: "*/5 * * * *"}

	var runs atomic.Int32
	a := replica(t, db, job, &runs)
	a.executeJob(slug, 0)
	require.EqualValues(t, 1, runs.Load())

	_, owner := lockRow(t, db, slug)
	require.Equal(t, a.lock.owner, owner)
	snap := a.jobSnapshot(slug)
	require.NotNil(t, snap)
	require.Equal(t, statusSuccess, snap.LastStatus)
}
