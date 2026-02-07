package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/service"
)

// HandleAgentNewTicket displays the agent ticket creation form with necessary data.
func HandleAgentNewTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse pre-selected interaction type from URL query parameter
		interactionType := c.Query("type")
		if interactionType == "" {
			interactionType = "phone" // default
		}
		// Validate it's a known type
		validTypes := map[string]bool{"phone": true, "email": true}
		if !validTypes[interactionType] {
			interactionType = "phone"
		}

		skipDB := htmxHandlerSkipDB() || strings.TrimSpace(os.Getenv("SKIP_DB_WAIT")) == "1"
		// Test-mode fallback for when database access is intentionally skipped
		if skipDB {
			renderTicketCreationFallback(c, interactionType)
			return
		}

		// Get database connection
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database connection failed"})
			return
		}

		// Get queues filtered by user's create permission
		// Use context values from middleware if available
		isQueueAdmin := false
		if val, exists := c.Get("is_queue_admin"); exists {
			if admin, ok := val.(bool); ok {
				isQueueAdmin = admin
			}
		}

		var queues []gin.H
		var queueErr error
		if isQueueAdmin {
			// Admin users get all queues
			queues, queueErr = getAllQueues(db)
		} else if accessibleQueueIDs, exists := c.Get("accessible_queue_ids"); exists {
			// Use queue IDs from middleware
			if queueIDs, ok := accessibleQueueIDs.([]uint); ok {
				queues, queueErr = getQueuesByIDs(db, queueIDs)
			}
		}
		// Fallback to querying directly if no context values
		if queues == nil && queueErr == nil {
			userIDUint := GetUserIDFromCtxUint(c, 0)
			queues, queueErr = getQueuesForAgent(c.Request.Context(), db, userIDUint)
		}
		if queueErr != nil {
			log.Printf("Error getting queues: %v", queueErr)
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

		// Get article colors for interaction type styling
		articleColors, err := getArticleColors(db)
		if err != nil {
			log.Printf("Warning: failed to load article colors: %v", err)
			articleColors = make(map[string]string)
		}

		// Render the form with data
		renderer := getPongo2Renderer()
		if renderer != nil {
			user := GetUserMapForTemplate(c)
			isInAdminGroup := false
			if v, ok := user["IsInAdminGroup"].(bool); ok {
				isInAdminGroup = v
			}
			renderer.HTML(c, http.StatusOK, "pages/tickets/new.pongo2", gin.H{
				"Title":             "New Ticket - GoatFlow",
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
				"PreSelectedType":   interactionType,
				"ArticleColors":     articleColors,
			})
		} else {
			renderTicketCreationFallback(c, interactionType)
		}
	}
}

// getQueuesForAgent gets queues available for agent ticket creation.
// Filters by user's "create" permission unless user is admin.
func getQueuesForAgent(ctx context.Context, db *sql.DB, userID uint) ([]gin.H, error) {
	// If userID is provided, filter by permission
	if userID > 0 {
		queueAccessSvc := service.NewQueueAccessService(db)

		// Check if user is admin (gets all queues)
		isAdmin, err := queueAccessSvc.IsAdmin(ctx, userID)
		if err != nil {
			return nil, err
		}

		if !isAdmin {
			// Get queues user has "create" permission for
			accessibleQueues, err := queueAccessSvc.GetAccessibleQueues(ctx, userID, "create")
			if err != nil {
				return nil, err
			}

			var queues []gin.H
			for _, q := range accessibleQueues {
				queues = append(queues, gin.H{"ID": q.QueueID, "Name": q.QueueName})
			}
			return queues, nil
		}
	}

	// Admin users or no user ID: return all valid queues
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return queues, nil
}

// getAllQueues returns all valid queues (for admin users).
func getAllQueues(db *sql.DB) ([]gin.H, error) {
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return queues, nil
}

// getQueuesByIDs returns queues by their IDs.
func getQueuesByIDs(db *sql.DB, queueIDs []uint) ([]gin.H, error) {
	if len(queueIDs) == 0 {
		return []gin.H{}, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(queueIDs))
	args := make([]interface{}, len(queueIDs))
	for i, id := range queueIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, name
		FROM queue
		WHERE id IN (%s) AND valid_id = 1
		ORDER BY name
	`, strings.Join(placeholders, ","))

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return queues, nil
}

// getTypesForAgent gets ticket types available for agent ticket creation.
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return types, nil
}

// getPrioritiesForAgent gets priorities available for agent ticket creation.
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return priorities, nil
}

// getServicesForAgent gets services available for agent ticket creation.
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return services, nil
}

// getCustomerUsersForAgent gets customer users available for agent ticket creation.
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

// loadPreferredQueuesInternal is the shared implementation for loading preferred queues.
func loadPreferredQueuesInternal(db *sql.DB, identifiers []string, tableName, idColumn string) (map[string]preferredQueue, error) {
	uniqueIDs := make([]string, 0, len(identifiers))
	seen := make(map[string]struct{}, len(identifiers))
	for _, id := range identifiers {
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
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT t.%s, q.id, q.name, t.permission_key
		FROM %s t
		JOIN queue q ON q.group_id = t.group_id
		WHERE t.permission_value = 1
		  AND q.valid_id = 1
		  AND t.%s IN (%s)
	`, idColumn, tableName, idColumn, strings.Join(placeholders, ","))

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
		var identifier, queueName, permissionKey string
		var queueID int
		if err := rows.Scan(&identifier, &queueID, &queueName, &permissionKey); err != nil {
			return nil, err
		}
		rank := queuePermissionRank(permissionKey)
		existing, ok := ranked[identifier]
		if !ok || rank < existing.rank || (rank == existing.rank && queueID < existing.ID) {
			ranked[identifier] = rankedQueue{preferredQueue: preferredQueue{ID: queueID, Name: queueName}, rank: rank}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make(map[string]preferredQueue, len(ranked))
	for identifier, entry := range ranked {
		result[identifier] = entry.preferredQueue
	}

	return result, nil
}

func loadPreferredQueuesForCustomers(db *sql.DB, customerIDs []string) (map[string]preferredQueue, error) {
	return loadPreferredQueuesInternal(db, customerIDs, "group_customer", "customer_id")
}

func loadPreferredQueuesForCustomerUsers(db *sql.DB, logins []string) (map[string]preferredQueue, error) {
	return loadPreferredQueuesInternal(db, logins, "group_customer_user", "user_id")
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

// getArticleColors returns all article colors from the database as a map of name -> color.
func getArticleColors(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT LOWER(name), color
		FROM article_color
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colors := make(map[string]string)
	for rows.Next() {
		var name, color string
		if err := rows.Scan(&name, &color); err != nil {
			return nil, err
		}
		if color != "" {
			colors[name] = color
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return colors, nil
}
