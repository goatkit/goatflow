package api

// Ticket creation handlers (new ticket forms, create operations).
// Split from ticket_htmx_handlers.go for maintainability.

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

func init() {
	routing.RegisterHandler("handleNewTicket", handleNewTicket)
	routing.RegisterHandler("handleNewEmailTicket", handleNewEmailTicket)
	routing.RegisterHandler("handleNewPhoneTicket", handleNewPhoneTicket)
	routing.RegisterHandler("handleCreateTicket", handleCreateTicket)
}

// ticketFormData holds common data for ticket creation forms.
type ticketFormData struct {
	Queues        []gin.H
	Priorities    []gin.H
	Types         []gin.H
	StateOptions  []gin.H
	StateLookup   map[string]gin.H
	CustomerUsers []gin.H
	DynamicFields []FieldWithScreenConfig
}

// loadTicketFormData loads common form data for ticket creation.
func loadTicketFormData(db *sql.DB, screenName string) ticketFormData {
	data := ticketFormData{
		Queues:      []gin.H{},
		Priorities:  []gin.H{},
		Types:       []gin.H{},
		StateLookup: map[string]gin.H{},
	}

	// Get queues from database
	qRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM queue WHERE valid_id = 1 ORDER BY name"))
	if err == nil {
		defer qRows.Close()
		for qRows.Next() {
			var id int
			var name string
			if err := qRows.Scan(&id, &name); err == nil {
				data.Queues = append(data.Queues, gin.H{"id": strconv.Itoa(id), "name": name})
			}
		}
		if err := qRows.Err(); err != nil {
			log.Printf("error iterating queues: %v", err)
		}
	}

	// Get priorities from database
	pRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM ticket_priority WHERE valid_id = 1 ORDER BY id"))
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var id int
			var name string
			if err := pRows.Scan(&id, &name); err == nil {
				color := "gray"
				switch id {
				case 1, 2:
					color = "green"
				case 3:
					color = "yellow"
				case 4:
					color = "orange"
				case 5:
					color = "red"
				}
				data.Priorities = append(data.Priorities, gin.H{"id": strconv.Itoa(id), "name": name, "color": color})
			}
		}
		if err := pRows.Err(); err != nil {
			log.Printf("error iterating priorities: %v", err)
		}
	}

	// Get ticket types from database
	tRows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM ticket_type WHERE valid_id = 1 ORDER BY name"))
	if err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var id int
			var name string
			if err := tRows.Scan(&id, &name); err == nil {
				data.Types = append(data.Types, gin.H{"id": strconv.Itoa(id), "name": name})
			}
		}
		if err := tRows.Err(); err != nil {
			log.Printf("error iterating types: %v", err)
		}
	}

	if opts, lookup, stateErr := shared.LoadTicketStatesForForm(db); stateErr != nil {
		log.Printf("new ticket: failed to load ticket states: %v", stateErr)
	} else {
		data.StateOptions = opts
		data.StateLookup = lookup
	}

	if cu, cuErr := getCustomerUsersForAgent(db); cuErr != nil {
		log.Printf("new ticket: failed to load customer users: %v", cuErr)
	} else {
		data.CustomerUsers = cu
	}

	if dfFields, dfErr := GetFieldsForScreenWithConfig(screenName, DFObjectTicket); dfErr != nil {
		log.Printf("Error getting ticket create dynamic fields for %s: %v", screenName, dfErr)
	} else {
		data.DynamicFields = dfFields
	}

	return data
}

// handleNewTicket shows the new ticket form.
func handleNewTicket(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil || getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		log.Printf("handleNewTicket: system unavailable (db err: %v)", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "System unavailable"})
		return
	}

	data := loadTicketFormData(db, "AgentTicketPhone")

	// Derive IsInAdminGroup for nav consistency
	isInAdminGroup := false
	if userMap, ok := getUserMapForTemplate(c)["ID"]; ok {
		if db != nil {
			var cnt int
			row := db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM group_user ug JOIN groups g ON ug.group_id = g.id WHERE ug.user_id = ? AND g.name = 'admin'`), userMap)
			_ = row.Scan(&cnt) //nolint:errcheck // Defaults to 0
			if cnt > 0 {
				isInAdminGroup = true
			}
		}
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/tickets/new.pongo2", pongo2.Context{
		"User":              getUserMapForTemplate(c),
		"IsInAdminGroup":    isInAdminGroup,
		"ActivePage":        "tickets",
		"Queues":            data.Queues,
		"Priorities":        data.Priorities,
		"Types":             data.Types,
		"TicketStates":      data.StateOptions,
		"TicketStateLookup": data.StateLookup,
		"CustomerUsers":     data.CustomerUsers,
		"DynamicFields":     data.DynamicFields,
	})
}

// handleNewEmailTicket shows the email ticket creation form.
func handleNewEmailTicket(c *gin.Context) {
	handleNewTicketByChannel(c, "email", "AgentTicketEmail")
}

// handleNewPhoneTicket shows the phone ticket creation form.
func handleNewPhoneTicket(c *gin.Context) {
	handleNewTicketByChannel(c, "phone", "AgentTicketPhone")
}

// handleNewTicketByChannel is the shared implementation for email and phone ticket forms.
func handleNewTicketByChannel(c *gin.Context, ticketType, screenName string) {
	db, err := database.GetDB()
	if err != nil || db == nil || getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		log.Printf("handleNewTicketByChannel(%s): system unavailable (db err: %v)", ticketType, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "System unavailable"})
		return
	}

	data := loadTicketFormData(db, screenName)

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/tickets/new.pongo2", pongo2.Context{
		"User":              getUserMapForTemplate(c),
		"ActivePage":        "tickets",
		"Queues":            data.Queues,
		"Priorities":        data.Priorities,
		"Types":             data.Types,
		"TicketType":        ticketType,
		"TicketStates":      data.StateOptions,
		"TicketStateLookup": data.StateLookup,
		"CustomerUsers":     data.CustomerUsers,
		"DynamicFields":     data.DynamicFields,
	})
}

// handleCreateTicket creates a new ticket.
func handleCreateTicket(c *gin.Context) {
	handleCreateTicketWithAttachments(c)
}
