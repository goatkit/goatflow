package api

import (
	"database/sql"
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// ticketEscalationView lists the ticket's escalation destinations (OTRS
// first response, update and solution time) from its escalation index for
// the ticket zoom. Each entry has kind, at, at_iso, overdue and relative.
func ticketEscalationView(t *models.Ticket, now time.Time) []gin.H {
	kinds := []struct {
		kind string
		at   int
	}{
		{"first_response", t.EscalationResponseTime},
		{"update", t.EscalationUpdateTime},
		{"solution", t.EscalationSolutionTime},
	}
	var out []gin.H
	for _, k := range kinds {
		if k.at <= 0 {
			continue
		}
		at := time.Unix(int64(k.at), 0).UTC()
		diff := at.Sub(now)
		out = append(out, gin.H{
			"kind":     k.kind,
			"at":       at.Format("2006-01-02 15:04"),
			"at_iso":   at.Format(time.RFC3339),
			"overdue":  diff <= 0,
			"relative": humanizeDuration(diff),
		})
	}
	return out
}

// ticketServiceSLANames returns the names of the ticket's service and SLA,
// "-" where it has none.
func ticketServiceSLANames(db *sql.DB, t *models.Ticket) (service, sla string, err error) {
	service, sla = "-", "-"
	lookup := func(query string, id *int, name *string) error {
		if id == nil || *id <= 0 {
			return nil
		}
		err := db.QueryRow(database.ConvertPlaceholders(query), *id).Scan(name)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if err = lookup(`SELECT name FROM service WHERE id = ?`, t.ServiceID, &service); err != nil {
		return "-", "-", err
	}
	if err = lookup(`SELECT name FROM sla WHERE id = ?`, t.SLAID, &sla); err != nil {
		return "-", "-", err
	}
	return service, sla, nil
}
