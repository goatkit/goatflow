package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleAgentNewTicket displays the agent ticket creation form with necessary data
func HandleAgentNewTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		skipDB := htmxHandlerSkipDB() || strings.TrimSpace(os.Getenv("SKIP_DB_WAIT")) == "1"
		// Test-mode fallback for when database access is intentionally skipped
		if skipDB {
			renderTicketCreationFallback(c, "email")
			return
		}

		// Get database connection
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database connection failed"})
			return
		}

		// Get queues
		queues, err := getQueuesForAgent(db)
		if err != nil {
			log.Printf("Error getting queues: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load queues"})
			return
		}

		// Get ticket types
		types, err := getTypesForAgent(db)
		if err != nil {
			log.Printf("Error getting types: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load types"})
			return
		}

		// Get priorities
		priorities, err := getPrioritiesForAgent(db)
		if err != nil {
			log.Printf("Error getting priorities: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load priorities"})
			return
		}

		// Get services
		services, err := getServicesForAgent(db)
		if err != nil {
			log.Printf("Error getting services: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load services"})
			return
		}

		// Get customer users
		customerUsers, err := getCustomerUsersForAgent(db)
		if err != nil {
			log.Printf("Error getting customer users: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load customer users"})
			return
		}

		stateOptions := []gin.H{}
		stateLookup := map[string]gin.H{}
		if opts, lookup, stateErr := LoadTicketStatesForForm(db); stateErr != nil {
			log.Printf("agent new ticket: failed to load ticket states: %v", stateErr)
		} else {
			stateOptions = opts
			stateLookup = lookup
		}

		// Get dynamic fields for AgentTicketPhone screen
		var dynamicFields []FieldWithScreenConfig
		if dfFields, dfErr := GetFieldsForScreenWithConfig("AgentTicketPhone", DFObjectTicket); dfErr != nil {
			log.Printf("Warning: failed to load dynamic fields for agent new ticket: %v", dfErr)
		} else {
			dynamicFields = dfFields
		}

		// Render the form with data
		renderer := GetPongo2Renderer()
		if renderer != nil {
			user := GetUserMapForTemplate(c)
			isInAdminGroup := false
			if v, ok := user["IsInAdminGroup"].(bool); ok {
				isInAdminGroup = v
			}
			renderer.HTML(c, http.StatusOK, "pages/tickets/new.pongo2", gin.H{
				"Title":             "New Ticket - GOTRS",
				"User":              user,
				"ActivePage":        "tickets",
				"IsInAdminGroup":    isInAdminGroup,
				"Queues":            queues,
				"Types":             types,
				"Priorities":        priorities,
				"Services":          services,
				"CustomerUsers":     customerUsers,
				"TicketStates":      stateOptions,
				"TicketStateLookup": stateLookup,
				"DynamicFields":     dynamicFields,
			})
		} else {
			renderTicketCreationFallback(c, "email")
		}
	}
}

// getQueuesForAgent gets queues available for agent ticket creation
func getQueuesForAgent(db *sql.DB) ([]gin.H, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name 
		FROM queue 
		WHERE valid_id = 1 
		ORDER BY name
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queues []gin.H
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		queues = append(queues, gin.H{"ID": id, "Name": name})
	}
	return queues, nil
}

// getTypesForAgent gets ticket types available for agent ticket creation
func getTypesForAgent(db *sql.DB) ([]gin.H, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name 
		FROM ticket_type 
		WHERE valid_id = 1 
		ORDER BY name
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []gin.H
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		types = append(types, gin.H{"ID": id, "Label": name})
	}
	return types, nil
}

// getPrioritiesForAgent gets priorities available for agent ticket creation
func getPrioritiesForAgent(db *sql.DB) ([]gin.H, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name 
		FROM ticket_priority 
		WHERE valid_id = 1 
		ORDER BY id
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var priorities []gin.H
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		// Provide ID/Name directly for template
		priorities = append(priorities, gin.H{"ID": id, "Name": name})
	}
	return priorities, nil
}

// getServicesForAgent gets services available for agent ticket creation
func getServicesForAgent(db *sql.DB) ([]gin.H, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name 
		FROM service 
		WHERE valid_id = 1 
		ORDER BY name
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []gin.H
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		services = append(services, gin.H{"ID": id, "Name": name})
	}
	return services, nil
}

// getCustomerUsersForAgent gets customer users available for agent ticket creation
func getCustomerUsersForAgent(db *sql.DB) ([]gin.H, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT login, email, first_name, last_name, customer_id
		FROM customer_user
		WHERE valid_id = 1
		ORDER BY first_name, last_name, email
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type customerUserRecord struct {
		Login      string
		Email      string
		FirstName  string
		LastName   string
		CustomerID string
	}

	records := make([]customerUserRecord, 0)
	rawCustomerIDs := make([]string, 0)
	seenCustomers := make(map[string]struct{})
	logins := make([]string, 0)
	seenLogins := make(map[string]struct{})

	for rows.Next() {
		var rec customerUserRecord
		if err := rows.Scan(&rec.Login, &rec.Email, &rec.FirstName, &rec.LastName, &rec.CustomerID); err != nil {
			return nil, err
		}
		records = append(records, rec)
		trimmedID := strings.TrimSpace(rec.CustomerID)
		if trimmedID != "" {
			if _, exists := seenCustomers[trimmedID]; !exists {
				seenCustomers[trimmedID] = struct{}{}
				rawCustomerIDs = append(rawCustomerIDs, trimmedID)
			}
		}
		trimmedLogin := strings.TrimSpace(rec.Login)
		if trimmedLogin != "" {
			if _, exists := seenLogins[trimmedLogin]; !exists {
				seenLogins[trimmedLogin] = struct{}{}
				logins = append(logins, trimmedLogin)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	preferredQueuesForCustomers, err := loadPreferredQueuesForCustomers(db, rawCustomerIDs)
	if err != nil {
		return nil, err
	}
	preferredQueuesForLogins, err := loadPreferredQueuesForCustomerUsers(db, logins)
	if err != nil {
		return nil, err
	}

	customerUsers := make([]gin.H, 0, len(records))
	for _, rec := range records {
		entry := gin.H{
			"Login":      rec.Login,
			"Email":      rec.Email,
			"FirstName":  rec.FirstName,
			"LastName":   rec.LastName,
			"CustomerID": rec.CustomerID,
		}
		if pref, ok := preferredQueuesForLogins[strings.TrimSpace(rec.Login)]; ok {
			entry["PreferredQueueID"] = strconv.Itoa(pref.ID)
			entry["PreferredQueueName"] = pref.Name
		} else if pref, ok := preferredQueuesForCustomers[strings.TrimSpace(rec.CustomerID)]; ok {
			entry["PreferredQueueID"] = strconv.Itoa(pref.ID)
			entry["PreferredQueueName"] = pref.Name
		}
		customerUsers = append(customerUsers, entry)
	}

	return customerUsers, nil
}

type preferredQueue struct {
	ID   int
	Name string
}

func loadPreferredQueuesForCustomers(db *sql.DB, customerIDs []string) (map[string]preferredQueue, error) {
	uniqueIDs := make([]string, 0, len(customerIDs))
	seen := make(map[string]struct{}, len(customerIDs))
	for _, id := range customerIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		uniqueIDs = append(uniqueIDs, trimmed)
	}

	if len(uniqueIDs) == 0 {
		return map[string]preferredQueue{}, nil
	}

	placeholders := make([]string, len(uniqueIDs))
	args := make([]interface{}, len(uniqueIDs))
	for i, id := range uniqueIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT gc.customer_id, q.id, q.name, gc.permission_key
		FROM group_customer gc
		JOIN queue q ON q.group_id = gc.group_id
		WHERE gc.permission_value = 1
		  AND q.valid_id = 1
		  AND gc.customer_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rankedQueue struct {
		preferredQueue
		rank int
	}

	ranked := make(map[string]rankedQueue, len(uniqueIDs))
	for rows.Next() {
		var customerID, queueName, permissionKey string
		var queueID int
		if err := rows.Scan(&customerID, &queueID, &queueName, &permissionKey); err != nil {
			return nil, err
		}
		rank := queuePermissionRank(permissionKey)
		existing, ok := ranked[customerID]
		if !ok || rank < existing.rank || (rank == existing.rank && queueID < existing.ID) {
			ranked[customerID] = rankedQueue{preferredQueue: preferredQueue{ID: queueID, Name: queueName}, rank: rank}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make(map[string]preferredQueue, len(ranked))
	for customerID, entry := range ranked {
		result[customerID] = entry.preferredQueue
	}

	return result, nil
}

func loadPreferredQueuesForCustomerUsers(db *sql.DB, logins []string) (map[string]preferredQueue, error) {
	uniqueLogins := make([]string, 0, len(logins))
	seen := make(map[string]struct{}, len(logins))
	for _, login := range logins {
		trimmed := strings.TrimSpace(login)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		uniqueLogins = append(uniqueLogins, trimmed)
	}

	if len(uniqueLogins) == 0 {
		return map[string]preferredQueue{}, nil
	}

	placeholders := make([]string, len(uniqueLogins))
	args := make([]interface{}, len(uniqueLogins))
	for i, login := range uniqueLogins {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = login
	}

	query := fmt.Sprintf(`
		SELECT gcu.user_id, q.id, q.name, gcu.permission_key
		FROM group_customer_user gcu
		JOIN queue q ON q.group_id = gcu.group_id
		WHERE gcu.permission_value = 1
		  AND q.valid_id = 1
		  AND gcu.user_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rankedQueue struct {
		preferredQueue
		rank int
	}

	ranked := make(map[string]rankedQueue, len(uniqueLogins))
	for rows.Next() {
		var login, queueName, permissionKey string
		var queueID int
		if err := rows.Scan(&login, &queueID, &queueName, &permissionKey); err != nil {
			return nil, err
		}
		rank := queuePermissionRank(permissionKey)
		existing, ok := ranked[login]
		if !ok || rank < existing.rank || (rank == existing.rank && queueID < existing.ID) {
			ranked[login] = rankedQueue{preferredQueue: preferredQueue{ID: queueID, Name: queueName}, rank: rank}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make(map[string]preferredQueue, len(ranked))
	for login, entry := range ranked {
		result[login] = entry.preferredQueue
	}

	return result, nil
}

func queuePermissionRank(key string) int {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "rw":
		return 0
	case "create", "move_into":
		return 1
	case "note", "owner", "priority":
		return 2
	case "ro":
		return 3
	default:
		return 4
	}
}
