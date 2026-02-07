package shared

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// Pending state type IDs
const (
	PendingReminderStateTypeID = 4
	PendingAutoStateTypeID     = 5
)

// DefaultPendingDuration is the default duration to use when a pending ticket
// has no explicit pending_until time set. This provides a fallback for
// legacy/migrated data that may be missing the date.
const DefaultPendingDuration = 24 * time.Hour

// GetEffectivePendingTime returns the effective pending time for a ticket.
// If untilTime is set (> 0), it returns that time.
// If untilTime is not set (0 or negative), it returns now + DefaultPendingDuration.
// This ensures pending tickets always have a usable deadline.
func GetEffectivePendingTime(untilTime int) time.Time {
	if untilTime > 0 {
		return time.Unix(int64(untilTime), 0).UTC()
	}
	return time.Now().UTC().Add(DefaultPendingDuration)
}

// GetEffectivePendingTimeUnix returns the effective pending time as a unix timestamp.
// This is useful for SQL queries and comparisons.
func GetEffectivePendingTimeUnix(untilTime int) int64 {
	return GetEffectivePendingTime(untilTime).Unix()
}

// EnsurePendingTime returns the provided untilTime if valid, or a default
// time (now + DefaultPendingDuration) as a unix timestamp. This is useful
// when setting the pending time on state changes.
func EnsurePendingTime(untilTime int) int {
	if untilTime > 0 {
		return untilTime
	}
	return int(time.Now().UTC().Add(DefaultPendingDuration).Unix())
}

// IsPendingStateType returns true if the state type is a pending state
// (either pending reminder or pending auto-close).
func IsPendingStateType(typeID int) bool {
	return typeID == PendingReminderStateTypeID || typeID == PendingAutoStateTypeID
}

// LoadTicketStatesForForm fetches valid ticket states and builds alias lookup data for forms.
func LoadTicketStatesForForm(db *sql.DB) ([]gin.H, map[string]gin.H, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("nil database connection")
	}
	rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT id, name, type_id
			FROM ticket_state
			WHERE valid_id = 1
			ORDER BY name
	`))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	states := make([]gin.H, 0)
	lookup := make(map[string]gin.H)
	for rows.Next() {
		var (
			id     int
			name   string
			typeID int
		)
		if scanErr := rows.Scan(&id, &name, &typeID); scanErr != nil {
			continue
		}
		slug := buildTicketStateSlug(name)
		state := gin.H{
			"ID":     id,
			"Name":   name,
			"TypeID": typeID,
			"Slug":   slug,
		}
		states = append(states, state)
		for _, key := range ticketStateLookupKeys(name) {
			if key != "" {
				lookup[key] = state
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return states, lookup, nil
}

func buildTicketStateSlug(name string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	if base == "" {
		return ""
	}
	collapsed := strings.Join(strings.Fields(base), " ")
	return strings.ReplaceAll(collapsed, " ", "_")
}

func ticketStateLookupKeys(name string) []string {
	keys := make([]string, 0)
	lower := strings.ToLower(name)

	// Add the full lowercase name
	keys = append(keys, lower)

	// Add words individually
	words := strings.Fields(lower)
	for _, word := range words {
		if word != "" {
			keys = append(keys, word)
		}
	}

	// Add common abbreviations/aliases
	aliases := map[string]string{
		"open":     "new",
		"pending":  "waiting",
		"resolved": "closed",
		"closed":   "resolved",
	}

	for key, alias := range aliases {
		if strings.Contains(lower, key) {
			keys = append(keys, alias)
		}
	}

	return keys
}
