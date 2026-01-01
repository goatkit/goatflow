package shared

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

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
