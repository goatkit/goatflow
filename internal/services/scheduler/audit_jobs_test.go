package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/sysconfig"
)

func TestPurgeAuditEventsDeletesOnlyOlderThanRetention(t *testing.T) {
	db := lockTestDB(t)
	ctx := context.Background()
	require.NoError(t, sysconfig.SaveAuditSettings(db, sysconfig.AuditSettings{RetentionDays: 30}, 1))
	t.Cleanup(func() { _ = sysconfig.SaveAuditSettings(db, sysconfig.DefaultAuditSettings(), 1) })

	now := time.Now()
	marker := fmt.Sprintf("test.purge.%d", now.UnixNano())
	for _, age := range []time.Duration{31 * 24 * time.Hour, 29 * 24 * time.Hour} {
		audit.RecordDB(ctx, db, audit.Event{
			Time: now.Add(-age), Category: audit.CategorySystem, Action: marker,
			Outcome: audit.OutcomeSuccess, TargetID: fmt.Sprint(int(age.Hours() / 24)),
		})
	}
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_audit_event WHERE action = ?`), marker)
	})

	deleted, err := purgeAuditEvents(ctx, db, now)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))

	var kept []string
	rows, err := db.Query(database.ConvertPlaceholders(`SELECT target_id FROM gk_audit_event WHERE action = ?`), marker)
	require.NoError(t, err)
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		kept = append(kept, id)
	}
	require.NoError(t, rows.Close())
	require.Equal(t, []string{"29"}, kept)

	var purgeEvents int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM gk_audit_event WHERE action = ? AND actor_type = ? AND create_time >= ?`),
		"system.audit.purge", audit.ActorSystem, now.Add(-time.Minute).UTC()).Scan(&purgeEvents))
	require.GreaterOrEqual(t, purgeEvents, 1)

	again, err := purgeAuditEvents(ctx, db, now)
	require.NoError(t, err)
	require.Zero(t, again)
}

func TestPurgeAuditEventsKeepsForeverWhenRetentionZero(t *testing.T) {
	db := lockTestDB(t)
	ctx := context.Background()
	require.NoError(t, sysconfig.SaveAuditSettings(db, sysconfig.AuditSettings{RetentionDays: 0}, 1))
	t.Cleanup(func() { _ = sysconfig.SaveAuditSettings(db, sysconfig.DefaultAuditSettings(), 1) })

	marker := fmt.Sprintf("test.purge.forever.%d", time.Now().UnixNano())
	audit.RecordDB(ctx, db, audit.Event{
		Time: time.Now().AddDate(-20, 0, 0), Category: audit.CategorySystem, Action: marker, Outcome: audit.OutcomeSuccess,
	})
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_audit_event WHERE action = ?`), marker)
	})

	deleted, err := purgeAuditEvents(ctx, db, time.Now())
	require.NoError(t, err)
	require.Zero(t, deleted)
}
