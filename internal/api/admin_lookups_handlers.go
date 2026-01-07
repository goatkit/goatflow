package api

// Admin lookup tables, queues, priorities, and dashboard handlers.
// Split from admin_htmx_handlers.go for maintainability.

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
)

func init() {
	routing.RegisterHandler("handleAdminQueues", handleAdminQueues)
	routing.RegisterHandler("handleAdminPriorities", handleAdminPriorities)
	routing.RegisterHandler("handleAdminLookups", handleAdminLookups)
	routing.RegisterHandler("handleAdminSettings", handleAdminSettings)
	routing.RegisterHandler("handleAdminTemplates", handleAdminTemplates)
	routing.RegisterHandler("handleAdminReports", handleAdminReports)
	routing.RegisterHandler("handleAdminLogs", handleAdminLogs)
	routing.RegisterHandler("handleAdminBackup", handleAdminBackup)
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

	// Get queues from database
	queueRepo := repository.NewQueueRepository(db)
	queues, err := queueRepo.List()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch queues")
		return
	}

	// Get groups for dropdown
	var groups []gin.H
	groupRows, err := db.Query("SELECT id, name FROM groups WHERE valid_id = 1 ORDER BY name")
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

	// Provide a minimal fallback when tests skip templates or renderer is unavailable
	if htmxHandlerSkipDB() || getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		html := `<!doctype html><html><head><title>Manage Lookup Values</title></head><body>
			<h1>Manage Lookup Values</h1>
			<nav>
				<ul>
					<li>Queues</li>
					<li>Priorities</li>
					<li>Ticket Types</li>
					<li>Statuses</li>
				</ul>
			</nav>
			<button>Refresh Cache</button>
		</body></html>`
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
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
	stateRows, err := db.Query(`
		SELECT ts.id, ts.name, ts.type_id, ts.comments, tst.name as type_name
		FROM ticket_state ts
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE ts.valid_id = 1
		ORDER BY ts.name
	`)
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

	// Ticket Priorities
	var priorities []gin.H
	priorityRows, err := db.Query("SELECT id, name FROM ticket_priority WHERE valid_id = 1 ORDER BY id")
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
	typeRows, err := db.Query("SELECT id, name FROM ticket_type WHERE valid_id = 1 ORDER BY name")
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
	serviceRows, err := db.Query("SELECT id, name FROM service WHERE valid_id = 1 ORDER BY name")
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
	slaRows, err := db.Query("SELECT id, name FROM sla WHERE valid_id = 1 ORDER BY name")
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
		"Priorities":   priorities,
		"TicketTypes":  types,
		"Services":     services,
		"SLAs":         slas,
		"User":         getUserMapForTemplate(c),
		"ActivePage":   "admin",
		"CurrentTab":   currentTab,
	})
}

func handleAdminSettings(c *gin.Context) {
	underConstruction("System Settings")(c)
}

func handleAdminTemplates(c *gin.Context) {
	underConstruction("Template Management")(c)
}

func handleAdminReports(c *gin.Context) {
	underConstruction("Reports")(c)
}

func handleAdminLogs(c *gin.Context) {
	underConstruction("Audit Logs")(c)
}

func handleAdminBackup(c *gin.Context) {
	underConstruction("Backup & Restore")(c)
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

	userCount := 0
	groupCount := 0
	activeTickets := 0
	queueCount := 0

	if db, _ := database.GetDB(); db != nil { //nolint:errcheck // Dashboard stats - default to 0 on error
		_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE valid_id = 1").Scan(&userCount)                      //nolint:errcheck
		_ = db.QueryRow("SELECT COUNT(*) FROM groups WHERE valid_id = 1").Scan(&groupCount)                    //nolint:errcheck
		_ = db.QueryRow("SELECT COUNT(*) FROM queue WHERE valid_id = 1").Scan(&queueCount)                     //nolint:errcheck
		_ = db.QueryRow("SELECT COUNT(*) FROM ticket WHERE ticket_state_id IN (1,2,3,4)").Scan(&activeTickets) //nolint:errcheck
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/dashboard.pongo2", pongo2.Context{
		"UserCount":     userCount,
		"GroupCount":    groupCount,
		"ActiveTickets": activeTickets,
		"QueueCount":    queueCount,
		"User":          getUserMapForTemplate(c),
		"ActivePage":    "admin",
	})
}

// Helper function to show under construction message.
func underConstruction(feature string) gin.HandlerFunc {
	return func(c *gin.Context) {
		getPongo2Renderer().HTML(c, http.StatusOK, "pages/under_construction.pongo2", pongo2.Context{
			"Feature":    feature,
			"User":       getUserMapForTemplate(c),
			"ActivePage": "admin",
		})
	}
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
		searchTerm)

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
