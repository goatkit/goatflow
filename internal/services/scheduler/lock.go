package scheduler

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// claimTimeout bounds the database round trips of one claim.
const claimTimeout = 5 * time.Second

// jobLock makes each scheduled run happen once across every backend
// replica that shares the database. Every replica keeps its own cron
// schedule; when a job fires, the replicas race to claim it and only the
// winner runs the handler.
type jobLock struct {
	db    *sql.DB
	owner string
}

func newJobLock(db *sql.DB) *jobLock {
	return &jobLock{db: db, owner: instanceID()}
}

// instanceID names this process in gk_scheduler_job_lock.locked_by.
func instanceID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	return fmt.Sprintf("%s:%d:%s", host, os.Getpid(), hex.EncodeToString(suffix))
}

// claim takes the run of slug at now and holds it until until. It returns
// false when another replica already holds this run. The row is created on
// first use, already expired.
func (l *jobLock) claim(ctx context.Context, slug string, now, until time.Time) (bool, error) {
	now, until = now.UTC(), until.UTC()
	claimed, err := l.tryClaim(ctx, slug, now, until)
	if err != nil || claimed {
		return claimed, err
	}
	// No row matched: either another replica holds the run or the job has
	// no row yet. Create the row (a no-op when it exists) and try again.
	if _, err := l.db.ExecContext(ctx, database.ConvertUpsert(`
		INSERT INTO gk_scheduler_job_lock (job_slug, locked_until, locked_by, change_time)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE job_slug = VALUES(job_slug)`, "job_slug"),
		slug, time.Unix(0, 0).UTC(), "", now); err != nil {
		return false, fmt.Errorf("create lock row for %s: %w", slug, err)
	}
	return l.tryClaim(ctx, slug, now, until)
}

func (l *jobLock) tryClaim(ctx context.Context, slug string, now, until time.Time) (bool, error) {
	res, err := l.db.ExecContext(ctx, database.ConvertPlaceholders(`
		UPDATE gk_scheduler_job_lock
		SET locked_until = ?, locked_by = ?, change_time = ?
		WHERE job_slug = ? AND locked_until <= ?`),
		until, l.owner, now, slug, now)
	if err != nil {
		return false, fmt.Errorf("claim %s: %w", slug, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim %s: %w", slug, err)
	}
	return n == 1, nil
}

// claimRun decides whether this replica runs job now. The claim is held
// until halfway to the job's next scheduled time: replicas firing for the
// same tick (clock skew up to half the interval) find it taken, and the next
// tick finds it free again.
func (s *Service) claimRun(job *models.ScheduledJob) (bool, error) {
	now := s.now()
	until := now.Add(time.Minute)
	if sched, err := s.parser.Parse(job.Schedule); err == nil {
		if next := sched.Next(now); next.After(now) {
			until = now.Add(next.Sub(now) / 2)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), claimTimeout)
	defer cancel()
	return s.lock.claim(ctx, job.Slug, now, until)
}
