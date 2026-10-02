package api

// Admin lookup tables, queues, priorities, and dashboard handlers.
// Split from admin_htmx_handlers.go for maintainability.

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/repository"
)

func init() {
	routing.RegisterHandler("handleAdminQueues", handleAdminQueues)
	routing.RegisterHandler("handleAdminPriorities", handleAdminPriorities)
	routing.RegisterHandler("handleAdminLookups", handleAdminLookups)
	routing.RegisterHandler("handleAdminReports", handleAdminReports)
	routing.RegisterHandler("handleAdminDashboard", handleAdminDashboard)
	routing.RegisterHandler("handleCustomerSearch", handleCustomerSearch)
}

// handleAdminQueues shows the admin queues page.
func handleAdminQueues(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// All queues, including invalid ones: the page filters by status and its
	// toggle re-enables invalid queues.
	queueRepo := repository.NewQueueRepository(db)
	queues, err := queueRepo.ListAll()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch queues")
		return
	}

	// Get groups for dropdown
	var groups []gin.H
	groupRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM groups WHERE valid_id = 1 ORDER BY name"))
	if err == nil {
		defer groupRows.Close()
		for groupRows.Next() {
			var id int
			var name string
			if err := groupRows.Scan(&id, &name); err == nil {
				groups = append(groups, gin.H{"ID": id, "Name": name})
			}
		}
		if err := groupRows.Err(); err != nil {
			log.Printf("error iterating groups: %v", err)
		}
	}

	// Populate dropdown data from OTRS-compatible tables
	systemAddresses := []gin.H{}
	addrRows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, value0, value1
		FROM system_address
		WHERE valid_id = 1
		ORDER BY id
	`))
	if err == nil {
		defer addrRows.Close()
		for addrRows.Next() {
			var (
				id          int
				email       string
				displayName sql.NullString
			)
			if scanErr := addrRows.Scan(&id, &email, &displayName); scanErr == nil {
				systemAddresses = append(systemAddresses, gin.H{
					"ID":          id,
					"Email":       email,
					"DisplayName": displayName.String,
				})
			}
		}
		if err := addrRows.Err(); err != nil {
			log.Printf("error iterating system addresses: %v", err)
		}
	}

	salutations := []gin.H{}
	salRows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name, text, content_type
		FROM salutation
		WHERE valid_id = 1
		ORDER BY name
	`))
	if err == nil {
		defer salRows.Close()
		for salRows.Next() {
			var (
				id          int
				name        string
				text        sql.NullString
				contentType sql.NullString
			)
			if scanErr := salRows.Scan(&id, &name, &text, &contentType); scanErr == nil {
				salutations = append(salutations, gin.H{
					"ID":          id,
					"Name":        name,
					"Text":        text.String,
					"ContentType": contentType.String,
				})
			}
		}
		if err := salRows.Err(); err != nil {
			log.Printf("error iterating salutations: %v", err)
		}
	}
	signatures := []gin.H{}
	sigRows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name, text, content_type
		FROM signature
		WHERE valid_id = 1
		ORDER BY name
	`))
	if err == nil {
		defer sigRows.Close()
		for sigRows.Next() {
			var (
				id          int
				name        string
				text        sql.NullString
				contentType sql.NullString
			)
			if scanErr := sigRows.Scan(&id, &name, &text, &contentType); scanErr == nil {
				signatures = append(signatures, gin.H{
					"ID":          id,
					"Name":        name,
					"Text":        text.String,
					"ContentType": contentType.String,
				})
			}
		}
		if err := sigRows.Err(); err != nil {
			log.Printf("error iterating signatures: %v", err)
		}
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/queues.pongo2", pongo2.Context{
		"Queues":          queues,
		"Groups":          groups,
		"SystemAddresses": systemAddresses,
		"Salutations":     salutations,
		"Signatures":      signatures,
		"User":            getUserMapForTemplate(c),
		"ActivePage":      "admin",
	})
}

// handleAdminPriorities shows the admin priorities page.
func handleAdminPriorities(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get priorities from database
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, name, valid_id
		FROM ticket_priority
		WHERE valid_id = 1
		ORDER BY id
	`))
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch priorities")
		return
	}
	defer rows.Close()

	var priorities []gin.H
	for rows.Next() {
		var id, validID int
		var name string

		err := rows.Scan(&id, &name, &validID)
		if err != nil {
			continue
		}

		priority := gin.H{
			"id":       id,
			"name":     name,
			"valid_id": validID,
		}

		priorities = append(priorities, priority)
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating priorities: %v", err)
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/priorities.pongo2", pongo2.Context{
		"Priorities": priorities,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// handleAdminLookups shows the admin lookups page.
func handleAdminLookups(c *gin.Context) {
	// Get the current tab from query parameter
	currentTab := c.Query("tab")
	if currentTab == "" {
		currentTab = "priorities" // Default to priorities tab
	}

	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		// Return JSON error for unavailable systems (non-test)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "System unavailable",
		})
		return
	}

	// Get various lookup data
	// Ticket States (with type name from ticket_state_type table)
	var ticketStates []gin.H
	stateRows, err := db.Query(database.ConvertPlaceholders(`
		SELECT ts.id, ts.name, ts.type_id, ts.comments, tst.name as type_name
		FROM ticket_state ts
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE ts.valid_id = 1
		ORDER BY ts.name
	`))
	if err == nil {
		defer stateRows.Close()
		for stateRows.Next() {
			var id, typeID int
			var name, typeName string
			var comments sql.NullString
			if err := stateRows.Scan(&id, &name, &typeID, &comments, &typeName); err != nil {
				continue
			}

			state := gin.H{
				"ID":       id,
				"Name":     name,
				"TypeID":   typeID,
				"TypeName": typeName,
			}
			if comments.Valid {
				state["Comments"] = comments.String
			}

			ticketStates = append(ticketStates, state)
		}
		if err := stateRows.Err(); err != nil {
			log.Printf("error iterating ticket states: %v", err)
		}
	}

	// Ticket state types for the state editor's Type select (ids differ per install)
	var stateTypes []gin.H
	stateTypeRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM ticket_state_type ORDER BY id"))
	if err != nil {
		log.Printf("error loading ticket state types: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load ticket state types"})
		return
	}
	defer stateTypeRows.Close()
	for stateTypeRows.Next() {
		var id int
		var name string
		if err := stateTypeRows.Scan(&id, &name); err != nil {
			log.Printf("error scanning ticket state type: %v", err)
			continue
		}
		stateTypes = append(stateTypes, gin.H{"ID": id, "Name": name})
	}
	if err := stateTypeRows.Err(); err != nil {
		log.Printf("error iterating ticket state types: %v", err)
	}

	// Ticket Priorities
	var priorities []gin.H
	priorityRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM ticket_priority WHERE valid_id = 1 ORDER BY id"))
	if err == nil {
		defer priorityRows.Close()
		for priorityRows.Next() {
			var id int
			var name string
			if err := priorityRows.Scan(&id, &name); err != nil {
				continue
			}

			priority := gin.H{
				"ID":   id,
				"Name": name,
			}

			priorities = append(priorities, priority)
		}
		if err := priorityRows.Err(); err != nil {
			log.Printf("error iterating priorities: %v", err)
		}
	}

	// Ticket Types
	var types []gin.H
	typeRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM ticket_type WHERE valid_id = 1 ORDER BY name"))
	if err == nil {
		defer typeRows.Close()
		for typeRows.Next() {
			var id int
			var name string
			if err := typeRows.Scan(&id, &name); err != nil {
				continue
			}

			ticketType := gin.H{
				"ID":   id,
				"Name": name,
			}

			types = append(types, ticketType)
		}
		if err := typeRows.Err(); err != nil {
			log.Printf("error iterating types: %v", err)
		}
	}

	// Services
	var services []gin.H
	serviceRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM service WHERE valid_id = 1 ORDER BY name"))
	if err == nil {
		defer serviceRows.Close()
		for serviceRows.Next() {
			var id int
			var name string
			if err := serviceRows.Scan(&id, &name); err != nil {
				continue
			}
			services = append(services, gin.H{"id": id, "name": name})
		}
		if err := serviceRows.Err(); err != nil {
			log.Printf("error iterating services: %v", err)
		}
	}

	// SLAs
	var slas []gin.H
	slaRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM sla WHERE valid_id = 1 ORDER BY name"))
	if err == nil {
		defer slaRows.Close()
		for slaRows.Next() {
			var id int
			var name string
			if err := slaRows.Scan(&id, &name); err != nil {
				continue
			}
			slas = append(slas, gin.H{"id": id, "name": name})
		}
		if err := slaRows.Err(); err != nil {
			log.Printf("error iterating SLAs: %v", err)
		}
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/lookups.pongo2", pongo2.Context{
		"TicketStates": ticketStates,
		"StateTypes":   stateTypes,
		"Priorities":   priorities,
		"TicketTypes":  types,
		"Services":     services,
		"SLAs":         slas,
		"User":         getUserMapForTemplate(c),
		"ActivePage":   "admin",
		"CurrentTab":   currentTab,
	})
}

// handleAdminReports renders the reports page. The page loads its figures from
// the queue-scoped /api/v1/statistics/* endpoints, so each viewer only sees
// tickets in queues they can read.
func handleAdminReports(c *gin.Context) {
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/reports.pongo2", pongo2.Context{
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// Admin handlers

// handleAdminDashboard shows the admin dashboard.
func handleAdminDashboard(c *gin.Context) {
	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "System unavailable",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleAdminDashboard: database unavailable: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	var userCount, groupCount, activeTickets, queueCount int
	counts := []struct {
		query string
		dest  *int
	}{
		{"SELECT COUNT(*) FROM users WHERE valid_id = 1", &userCount},
		{"SELECT COUNT(*) FROM groups WHERE valid_id = 1", &groupCount},
		{"SELECT COUNT(*) FROM queue WHERE valid_id = 1", &queueCount},
		{"SELECT COUNT(*) FROM ticket WHERE ticket_state_id IN (" + lookups.ViewableStateIDsSQL + ")", &activeTickets},
	}
	for _, q := range counts {
		if err := db.QueryRow(database.ConvertPlaceholders(q.query)).Scan(q.dest); err != nil {
			log.Printf("handleAdminDashboard: %s: %v", q.query, err)
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to load dashboard statistics")
			return
		}
	}

	// First-run nudge: on a system with no groups/queues and setup not yet marked
	// complete, send the admin to the setup wizard instead of an empty dashboard.
	done, err := setupCompleted(db)
	if err != nil {
		log.Printf("handleAdminDashboard: setup status: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load setup status")
		return
	}
	if !done && groupCount == 0 && queueCount == 0 {
		c.Redirect(http.StatusSeeOther, "/admin/setup")
		return
	}

	// Get ticket activity metrics from cache with fallback to calculation
	ticketActivity, err := getTicketActivityFromCache(c, db)
	if err != nil {
		log.Printf("handleAdminDashboard: ticket activity: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load dashboard statistics")
		return
	}

	// Get recent admin audit log entries
	recentActivity, err := getRecentAdminActivity(db)
	if err != nil {
		log.Printf("handleAdminDashboard: recent admin activity: %v", err)
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to load recent admin activity")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/dashboard.pongo2", pongo2.Context{
		"UserCount":      userCount,
		"GroupCount":     groupCount,
		"ActiveTickets":  activeTickets,
		"QueueCount":     queueCount,
		"TicketActivity": ticketActivity,
		"RecentActivity": recentActivity,
		"User":           getUserMapForTemplate(c),
		"ActivePage":     "admin",
	})
}

// setupCompleted reports whether first-run setup has been marked done via the
// setup.assistant.completed sysconfig flag. No sysconfig row means not done.
func setupCompleted(db *sql.DB) (bool, error) {
	done, _, err := sysconfigBool(db, "setup.assistant.completed")
	return done, err
}

// getRecentAdminActivity fetches the most recent admin audit log entries.
func getRecentAdminActivity(db *sql.DB) ([]map[string]string, error) {
	query := database.ConvertQuery(`
		SELECT aat.name AS action, al.target_type, al.target_identifier,
			al.reason, al.create_time, u.login AS admin_login
		FROM admin_action_log al
		LEFT JOIN admin_action_type aat ON aat.id = al.action_type_id
		LEFT JOIN users u ON u.id = al.create_by
		ORDER BY al.create_time DESC
		LIMIT 10
	`)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]string
	for rows.Next() {
		var action, targetType, targetID, reason, adminLogin sql.NullString
		var createTime time.Time
		if err := rows.Scan(&action, &targetType, &targetID, &reason, &createTime, &adminLogin); err != nil {
			return nil, err
		}

		entry := map[string]string{
			"action":     action.String,
			"target":     targetType.String,
			"target_id":  targetID.String,
			"reason":     reason.String,
			"admin":      adminLogin.String,
			"time":       createTime.Format("2006-01-02 15:04"),
			"time_human": timeAgo(createTime),
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

// timeAgo returns a human-readable relative time string.
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2 Jan 2006")
	}
}

// getTicketActivityFromCache retrieves ticket activity metrics from Valkey cache,
// falling back to direct calculation on a cache miss.
func getTicketActivityFromCache(c *gin.Context, db *sql.DB) (map[string]int, error) {
	if valkeyCache != nil {
		var cached map[string]int
		if err := valkeyCache.GetObject(c, "metrics:ticket_activity", &cached); err == nil && cached != nil {
			return cached, nil
		}
	}

	metrics := map[string]int{}
	for _, m := range []struct {
		key       string
		countType string
		days      int
	}{
		{"closed_day", "closed", 1}, {"closed_week", "closed", 7}, {"closed_month", "closed", 30},
		{"created_day", "created", 1}, {"created_week", "created", 7}, {"created_month", "created", 30},
	} {
		n, err := getTicketCountForDashboard(db, m.countType, m.days)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", m.key, err)
		}
		metrics[m.key] = n
	}
	open, err := getOpenTicketCountForDashboard(db)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	metrics["open"] = open
	return metrics, nil
}

// getTicketCountForDashboard returns the count of tickets closed or created within the specified days.
func getTicketCountForDashboard(db *sql.DB, countType string, days int) (int, error) {
	var query string
	if countType == "closed" {
		query = database.ConvertPlaceholders(`
			SELECT COUNT(*)
			FROM ticket
			WHERE ticket_state_id IN (` + lookups.ClosedStateIDsSQL + `)
			  AND change_time >= ?
		`)
	} else {
		query = database.ConvertPlaceholders(`
			SELECT COUNT(*)
			FROM ticket
			WHERE create_time >= ?
		`)
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	var count int
	err := db.QueryRow(query, cutoff).Scan(&count)
	return count, err
}

// getOpenTicketCountForDashboard returns the count of currently open tickets.
func getOpenTicketCountForDashboard(db *sql.DB) (int, error) {
	query := database.ConvertPlaceholders(`
		SELECT COUNT(*)
		FROM ticket
		WHERE ticket_state_id IN (` + lookups.ViewableStateIDsSQL + `)
	`)
	var count int
	err := db.QueryRow(query).Scan(&count)
	return count, err
}

// handleCustomerSearch handles customer search for autocomplete.
func handleCustomerSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Search for customers by login, email, first name, or last name
	// Using LOWER() LIKE LOWER() for portable case-insensitive search; supporting wildcard *
	searchTerm := strings.ReplaceAll(query, "*", "%")
	if !strings.Contains(searchTerm, "%") {
		searchTerm = "%" + searchTerm + "%"
	}

	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, login, email, first_name, last_name, customer_id
		FROM customer_user
		WHERE valid_id = 1
		  AND (LOWER(login) LIKE LOWER(?)
		       OR LOWER(email) LIKE LOWER(?)
		       OR LOWER(first_name) LIKE LOWER(?)
		       OR LOWER(last_name) LIKE LOWER(?)
		       OR LOWER(CONCAT(first_name, ' ', last_name)) LIKE LOWER(?))
		LIMIT 10`),
		searchTerm, searchTerm, searchTerm, searchTerm, searchTerm)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search customers"})
		return
	}
	defer rows.Close()

	var customers []gin.H
	for rows.Next() {
		var id int
		var login, email, firstName, lastName, customerID string
		err := rows.Scan(&id, &login, &email, &firstName, &lastName, &customerID)
		if err != nil {
			continue
		}

		customers = append(customers, gin.H{
			"id":          id,
			"login":       login,
			"email":       email,
			"first_name":  firstName,
			"last_name":   lastName,
			"full_name":   firstName + " " + lastName,
			"customer_id": customerID,
			"display":     fmt.Sprintf("%s %s (%s)", firstName, lastName, email),
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating customer search results: %v", err)
	}

	if customers == nil {
		customers = []gin.H{}
	}

	c.JSON(http.StatusOK, customers)
}
