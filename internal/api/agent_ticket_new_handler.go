package api

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleAgentNewTicket displays the agent ticket creation form with necessary data
func HandleAgentNewTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Test-mode fallback for when database is not available
		if os.Getenv("APP_ENV") == "test" && db == nil {
			// Return mock data for testing
			renderer := GetPongo2Renderer()
			if renderer != nil {
				user := GetUserMapForTemplate(c)
				isInAdminGroup := false
				if v, ok := user["IsInAdminGroup"].(bool); ok { isInAdminGroup = v }
				renderer.HTML(c, http.StatusOK, "pages/tickets/new.pongo2", gin.H{
					"Title":      "New Ticket - GOTRS",
					"User":       user,
					"ActivePage": "tickets",
					"IsInAdminGroup": isInAdminGroup,
					"Queues": []gin.H{
						{"ID": 1, "Name": "Raw"},
						{"ID": 2, "Name": "Junk"},
						{"ID": 3, "Name": "OBC"},
					},
					"Types": []gin.H{
						{"ID": 1, "Label": "Unclassified"},
						{"ID": 2, "Label": "Incident"},
						{"ID": 3, "Label": "Request"},
					},
					"Priorities": []gin.H{
						{"ID": 1, "Name": "1 very low"},
						{"ID": 2, "Name": "2 low"},
						{"ID": 3, "Name": "3 normal"},
						{"ID": 4, "Name": "4 high"},
						{"ID": 5, "Name": "5 very high"},
					},
					"CustomerUsers": []gin.H{
						{"Login": "test@example.com", "Email": "test@example.com", "FirstName": "Test", "LastName": "User"},
						{"Login": "john@example.com", "Email": "john@example.com", "FirstName": "John", "LastName": "Doe"},
					},
				})
			} else {
				// Fallback when template renderer is not available
				c.String(http.StatusOK, "Template renderer not available")
			}
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

		// Get customer users
		customerUsers, err := getCustomerUsersForAgent(db)
		if err != nil {
			log.Printf("Error getting customer users: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load customer users"})
			return
		}

		// Render the form with data
		renderer := GetPongo2Renderer()
		if renderer != nil {
			user := GetUserMapForTemplate(c)
			isInAdminGroup := false
			if v, ok := user["IsInAdminGroup"].(bool); ok { isInAdminGroup = v }
			renderer.HTML(c, http.StatusOK, "pages/tickets/new.pongo2", gin.H{
				"Title":         "New Ticket - GOTRS",
				"User":          user,
				"ActivePage":    "tickets",
				"IsInAdminGroup": isInAdminGroup,
				"Queues":        queues,
				"Types":         types,
				"Priorities":    priorities,
				"CustomerUsers": customerUsers,
			})
		} else {
			// Fallback when template renderer is not available
			c.String(http.StatusOK, "Template renderer not available")
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

// getCustomerUsersForAgent gets customer users available for agent ticket creation
func getCustomerUsersForAgent(db *sql.DB) ([]gin.H, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT login, email, first_name, last_name
		FROM customer_user
		WHERE valid_id = 1
		ORDER BY first_name, last_name, email
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customerUsers []gin.H
	for rows.Next() {
		var login, email, firstName, lastName string
		if err := rows.Scan(&login, &email, &firstName, &lastName); err != nil {
			return nil, err
		}
		customerUsers = append(customerUsers, gin.H{
			"Login":     login,
			"Email":     email,
			"FirstName": firstName,
			"LastName":  lastName,
		})
	}
	return customerUsers, nil
}
