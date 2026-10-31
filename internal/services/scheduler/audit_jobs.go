package scheduler

import (
	"context"
	"database/sql"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

// handleAuditPurge deletes audit events older than Audit::Settings
// retention_days; 0 keeps them forever.
func (s *Service) handleAuditPurge(ctx context.Context, job *models.ScheduledJob) error {
	if s.db == nil {
		return errSchedulerNoDatabase
	}
	deleted, err := purgeAuditEvents(ctx, s.db, time.Now())
	if deleted > 0 {
		s.logger.Printf("scheduler: audit retention removed %d events", deleted)
	}
	return err
}

func purgeAuditEvents(ctx context.Context, db *sql.DB, now time.Time) (int64, error) {
	cfg, err := sysconfig.LoadAuditSettings(db)
	if err != nil {
		return 0, err
	}
	if cfg.RetentionDays <= 0 {
		return 0, nil
	}
	deleted, err := audit.Purge(ctx, db, now.AddDate(0, 0, -cfg.RetentionDays))
	if err != nil || deleted == 0 {
		return deleted, err
	}
	audit.RecordDB(ctx, db, audit.Event{
		Category:  audit.CategorySystem,
		Action:    "system.audit.purge",
		Outcome:   audit.OutcomeSuccess,
		ActorType: audit.ActorSystem,
		Details:   map[string]any{"deleted": deleted, "retention_days": cfg.RetentionDays},
	})
	return deleted, nil
}
