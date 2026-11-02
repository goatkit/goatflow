package scheduler

import (
	"context"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/deletion"
)

// handleRecycleBinPurge permanently deletes recycle bin entries whose
// retention has expired.
func (s *Service) handleRecycleBinPurge(ctx context.Context, job *models.ScheduledJob) error {
	if s.db == nil {
		return errSchedulerNoDatabase
	}
	purged, err := deletion.NewServiceWithDB(s.db).PurgeExpired(ctx)
	if err != nil || purged == 0 {
		return err
	}
	s.logger.Printf("scheduler: recycle bin retention purged %d entries", purged)
	audit.RecordDB(ctx, s.db, audit.Event{
		Category:  audit.CategorySystem,
		Action:    "system.recycle_bin.purge",
		Outcome:   audit.OutcomeSuccess,
		ActorType: audit.ActorSystem,
		Details:   map[string]any{"purged": purged},
	})
	return nil
}
