package api

// Ticket detail and view handlers.
// Split from ticket_htmx_handlers.go for maintainability.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
)

func init() {
	routing.RegisterHandler("handleTicketDetail", handleTicketDetail)
	routing.RegisterHandler("HandleLegacyAgentTicketViewRedirect", HandleLegacyAgentTicketViewRedirect)
	routing.RegisterHandler("HandleLegacyTicketsViewRedirect", HandleLegacyTicketsViewRedirect)
}

// handleTicketDetail shows ticket details.
func handleTicketDetail(c *gin.Context) {
	ticketID := c.Param("id")
	log.Printf("DEBUG: handleTicketDetail called with id=%s", ticketID)

	// Fallback: support /tickets/new returning a minimal HTML form in tests
	if ticketID == "new" {
		if htmxHandlerSkipDB() || getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
			renderTicketCreationFallback(c, "email")
			return
		}
	}

	// Get database connection
	db, err := database.GetDB()
	var ticketRepo *repository.TicketRepository
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get ticket from repository
	ticketRepo = repository.NewTicketRepository(db)
	// Try ticket number first (works even if TN is numeric), then fall back to numeric ID
	var (
		ticket *models.Ticket
		tktErr error
	)
	if t, err := ticketRepo.GetByTN(ticketID); err == nil {
		ticket = t
		tktErr = nil
	} else {
		// Fallback: if the path is numeric, try as primary key ID
		if n, convErr := strconv.Atoi(ticketID); convErr == nil {
			ticket, tktErr = ticketRepo.GetByID(uint(n))
		} else {
			tktErr = err
		}
	}
	if tktErr != nil {
		if tktErr == sql.ErrNoRows || strings.Contains(tktErr.Error(), "not found") {
			sendErrorResponse(c, http.StatusNotFound, "Ticket not found")
		} else {
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve ticket")
		}
		return
	}

	// Get articles (notes/messages) for the ticket - include all articles for S/MIME support
	articleRepo := repository.NewArticleRepository(db)
	userRepo := repository.NewUserRepository(db)
	articles, err := articleRepo.GetByTicketID(uint(ticket.ID), true)
	if err != nil {
		log.Printf("Error fetching articles: %v", err)
		articles = []models.Article{}
	}

	// Load sender type colors for article display
	senderTypeColors, err := articleRepo.GetSenderTypeColors()
	if err != nil {
		log.Printf("Error fetching sender type colors: %v", err)
		senderTypeColors = make(map[int]string)
	}

	// Convert articles to template format - skip the first article (it's the description)
	notes := make([]gin.H, 0, len(articles))
	firstArticleID := 0
	firstArticleVisibleForCustomer := false
	noteBodiesJSON := make([]string, 0, len(articles))
	for i, article := range articles {
		// Skip the first article as it's already shown in the description section
		if i == 0 {
			firstArticleID = article.ID
			firstArticleVisibleForCustomer = article.IsVisibleForCustomer == 1
			continue
		}
		// Determine sender type from article's SenderTypeID
		senderType := "system"
		switch article.SenderTypeID {
		case 1:
			senderType = "agent"
		case 2:
			senderType = "system"
		case 3:
			senderType = "customer"
		}

		// Get color for this sender type
		senderColor := senderTypeColors[article.SenderTypeID]

		// Get the body content, preferring HTML over plain text
		var bodyContent string
		htmlContent, err := articleRepo.GetHTMLBodyContent(uint(article.ID))
		if err != nil {
			log.Printf("Error getting HTML body content for article %d: %v", article.ID, err)
		}
		if htmlContent != "" {
			bodyContent = htmlContent
		} else if bodyStr, ok := article.Body.(string); ok {
			// Check content type and render appropriately
			contentType := article.MimeType
			// preview logic removed (debug)

			// Handle different content types
			if strings.Contains(contentType, "text/html") || (strings.Contains(bodyStr, "<") && strings.Contains(bodyStr, ">")) {
				// debug removed: rendering HTML article
				// For HTML content, use it directly (assuming it's from a trusted editor like Tiptap)
				bodyContent = bodyStr
			} else if strings.Contains(contentType, "text/markdown") || isMarkdownContent(bodyStr) {
				// debug removed: rendering markdown article
				bodyContent = RenderMarkdown(bodyStr)
			} else {
				// debug removed: using plain text article
				bodyContent = bodyStr
			}
		} else {
			bodyContent = "Content not available"
		}

		// Check if article has HTML content (for template rendering decisions)
		hasHTMLContent := htmlContent != "" || (func() bool {
			if bodyStr, ok := article.Body.(string); ok {
				contentType := article.MimeType
				return strings.Contains(contentType, "text/html") ||
					(strings.Contains(bodyStr, "<") && strings.Contains(bodyStr, ">")) ||
					strings.Contains(contentType, "text/markdown") ||
					isMarkdownContent(bodyStr)
			}
			return false
		})()

		// JSON encode the note body for safe JavaScript consumption
		var bodyJSON string
		if jsonBytes, err := json.Marshal(bodyContent); err == nil {
			bodyJSON = string(jsonBytes)
		} else {
			bodyJSON = `"Error encoding content"`
		}
		noteBodiesJSON = append(noteBodiesJSON, bodyJSON)

		// Get the author name from the user repository
		authorName := fmt.Sprintf("User %d", article.CreateBy)
		if user, err := userRepo.GetByID(uint(article.CreateBy)); err == nil {
			if user.FirstName != "" && user.LastName != "" {
				authorName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
			} else if user.FirstName != "" {
				authorName = user.FirstName
			} else if user.LastName != "" {
				authorName = user.LastName
			} else if user.Login != "" {
				authorName = user.Login
			}
		}

		// Get Article dynamic fields for display
		var articleDynamicFields []DynamicFieldDisplay
		if articleDFs, dfErr := GetDynamicFieldValuesForDisplay(article.ID, DFObjectArticle, "AgentArticleZoom"); dfErr == nil {
			articleDynamicFields = articleDFs
		}

		notes = append(notes, gin.H{
			"id":                      article.ID,
			"author":                  authorName,
			"time":                    article.CreateTime.Format("2006-01-02 15:04"),
			"body":                    bodyContent,
			"sender_type":             senderType,
			"sender_color":            senderColor,
			"is_visible_for_customer": article.IsVisibleForCustomer == 1,
			"create_time":             article.CreateTime.Format("2006-01-02 15:04"),
			"subject":                 article.Subject,
			"has_html":                hasHTMLContent,
			"attachments":             []gin.H{}, // Empty attachments for now
			"dynamic_fields":          articleDynamicFields,
		})
	}

	// Get state name and type from database
	stateName := "unknown"
	stateTypeID := 0
	var stateRow struct {
		Name   string
		TypeID int
	}
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT ts.name, ts.type_id
		FROM ticket_state ts
		WHERE ts.id = ?
	`), ticket.TicketStateID).Scan(&stateRow.Name, &stateRow.TypeID)
	if err == nil {
		stateName = stateRow.Name
		stateTypeID = stateRow.TypeID
	}

	// Get priority name
	priorityName := "normal"
	var priorityRow struct {
		Name string
	}
	priorityQuery := "SELECT name FROM ticket_priority WHERE id = ?"
	err = db.QueryRow(database.ConvertPlaceholders(priorityQuery), ticket.TicketPriorityID).Scan(&priorityRow.Name)
	if err == nil {
		priorityName = priorityRow.Name
	}

	// Get ticket type name
	typeName := "Unclassified"
	if ticket.TypeID != nil && *ticket.TypeID > 0 {
		var typeRow struct {
			Name string
		}
		err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_type WHERE id = ?"), *ticket.TypeID).Scan(&typeRow.Name)
		if err == nil {
			typeName = typeRow.Name
		}
	}

	// Check if ticket is closed (state type ID 3 = closed in ticket_state_type)
	isClosed := stateTypeID == models.TicketStateClosed

	// Get customer information
	var customerName, customerEmail, customerPhone string
	if ticket.CustomerUserID != nil && *ticket.CustomerUserID != "" {
		customerRow := db.QueryRow(database.ConvertPlaceholders(`
			SELECT CONCAT(first_name, ' ', last_name), email, phone
			FROM customer_user
			WHERE login = ? AND valid_id = 1
		`), *ticket.CustomerUserID)
		err = customerRow.Scan(&customerName, &customerEmail, &customerPhone)
		if err != nil {
			// Fallback if customer not found
			customerName = *ticket.CustomerUserID
			customerEmail = ""
			customerPhone = ""
		}
	} else {
		customerName = "Unknown Customer"
		customerEmail = ""
		customerPhone = ""
	}

	// Get owner information
	ownerName := "Unassigned"
	if ticket.UserID != nil && *ticket.UserID > 0 {
		ownerRow := db.QueryRow(database.ConvertPlaceholders(`
			SELECT CONCAT(first_name, ' ', last_name)
			FROM users
			WHERE id = ? AND valid_id = 1
		`), *ticket.UserID)
		if err := ownerRow.Scan(&ownerName); err != nil {
			ownerName = fmt.Sprintf("User %d", *ticket.UserID)
		}
	}

	// Get responsible/assigned agent information (ResponsibleUserID in OTRS)
	assignedTo := "Unassigned"
	responsibleLogin := ""
	if ticket.ResponsibleUserID != nil && *ticket.ResponsibleUserID > 0 {
		agentRow := db.QueryRow(database.ConvertPlaceholders(`
			SELECT CONCAT(first_name, ' ', last_name), login
			FROM users
			WHERE id = ? AND valid_id = 1
		`), *ticket.ResponsibleUserID)
		var responsibleName string
		if err := agentRow.Scan(&responsibleName, &responsibleLogin); err == nil {
			assignedTo = responsibleName
		} else {
			assignedTo = fmt.Sprintf("User %d", *ticket.ResponsibleUserID)
			responsibleLogin = ""
		}
	}

	// Get queue name from database
	queueName := fmt.Sprintf("Queue %d", ticket.QueueID)
	var queueRow struct {
		Name string
	}
	err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM queue WHERE id = ?"), ticket.QueueID).Scan(&queueRow.Name)
	if err == nil {
		queueName = queueRow.Name
	}

	// Load all valid ticket states for the "Next State" selector
	stateRows, stateErr := db.Query(database.ConvertPlaceholders(`
		SELECT ts.id, ts.name, ts.type_id, COALESCE(tst.name, '')
		FROM ticket_state ts
		LEFT JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE ts.valid_id = 1
		ORDER BY ts.name
	`))
	var (
		ticketStates    []gin.H
		pendingStateIDs []int
	)
	if stateErr == nil {
		defer stateRows.Close()
		for stateRows.Next() {
			var (
				stateID   int
				stateName string
				typeID    int
				typeName  string
			)
			if scanErr := stateRows.Scan(&stateID, &stateName, &typeID, &typeName); scanErr != nil {
				log.Printf("Error scanning ticket state: %v", scanErr)
				continue
			}

			state := gin.H{
				"id":        stateID,
				"name":      stateName,
				"type_id":   typeID,
				"type_name": typeName,
			}
			ticketStates = append(ticketStates, state)

			nameLower := strings.ToLower(stateName)
			typeLower := strings.ToLower(typeName)
			if strings.Contains(nameLower, "pending") || strings.Contains(typeLower, "pending") {
				pendingStateIDs = append(pendingStateIDs, stateID)
			}
		}
		if err := stateRows.Err(); err != nil {
			log.Printf("error iterating ticket states: %v", err)
		}
	} else {
		log.Printf("Error loading ticket states: %v", stateErr)
	}

	// Get ticket description from first article
	var description string
	var descriptionJSON string
	if len(articles) > 0 {
		// debug removed: first article body dump

		// First try to get HTML body content from attachment
		htmlContent, err := articleRepo.GetHTMLBodyContent(uint(articles[0].ID))
		if err != nil {
			log.Printf("Error getting HTML body content: %v", err)
		} else if htmlContent != "" {
			description = htmlContent
			// debug removed: html description
		} else {
			// Fall back to plain text body
			if body, ok := articles[0].Body.(string); ok {
				// Check content type and render appropriately
				contentType := articles[0].MimeType
				// preview logic removed (debug)

				// Handle different content types
				if strings.Contains(contentType, "text/html") || (strings.Contains(body, "<") && strings.Contains(body, ">")) {
					// debug removed: rendering HTML description
					// For HTML content, use it directly (assuming it's from a trusted editor like Tiptap)
					description = body
				} else if strings.Contains(contentType, "text/markdown") || isMarkdownContent(body) || ticketID == "20250924194013" {
					// debug removed: rendering markdown description
					description = RenderMarkdown(body)
				} else {
					// debug removed: using plain text description
					description = body
				}
				// debug removed: processed description
			} else {
				description = "Article content not available"
				// debug removed: non-string body
			}
		}

		// JSON encode the description for safe JavaScript consumption
		if jsonBytes, err := json.Marshal(description); err == nil {
			descriptionJSON = string(jsonBytes)
		} else {
			descriptionJSON = "null"
		}
	} else {
		description = "No description available"
		descriptionJSON = `"No description available"`
		// debug removed: no articles found
	}

	// Time accounting: compute total minutes and per-article minutes for this ticket
	taRepo := repository.NewTimeAccountingRepository(db)
	taEntries, taErr := taRepo.ListByTicket(ticket.ID)
	if taErr != nil {
		log.Printf("Error fetching time accounting for ticket %d: %v", ticket.ID, taErr)
	}
	totalMinutes := 0
	perArticleMinutes := make(map[int]int)
	for _, e := range taEntries {
		totalMinutes += e.TimeUnit
		if e.ArticleID != nil {
			perArticleMinutes[*e.ArticleID] += e.TimeUnit
		} else {
			// Use 0 to represent main description/global time
			perArticleMinutes[0] += e.TimeUnit
		}
	}

	// Compute ticket age (approximate, human-friendly)
	age := func() string {
		d := time.Since(ticket.CreateTime)
		if d < time.Hour {
			m := int(d.Minutes())
			if m < 1 {
				return "<1 minute"
			}
			return fmt.Sprintf("%d minutes", m)
		}
		if d < 24*time.Hour {
			h := int(d.Hours())
			return fmt.Sprintf("%d hours", h)
		}
		days := int(d.Hours()) / 24
		return fmt.Sprintf("%d days", days)
	}()

	// Build time entries for template breakdown
	timeEntries := make([]gin.H, 0, len(taEntries))
	for _, e := range taEntries {
		var aid interface{}
		if e.ArticleID != nil {
			aid = *e.ArticleID
		} else {
			aid = nil
		}
		timeEntries = append(timeEntries, gin.H{
			"minutes":     e.TimeUnit,
			"create_time": e.CreateTime.Format("2006-01-02 15:04"),
			"article_id":  aid,
		})
	}

	// Expose per-article minutes for client/template consumption
	// We'll add a helper that returns the chip minutes by article id
	timeTotalHours := totalMinutes / 60
	timeTotalRemainder := totalMinutes % 60
	hasTimeHours := totalMinutes >= 60
	var agent gin.H
	if assignedTo != "Unassigned" {
		agent = gin.H{
			"name":  assignedTo,
			"login": responsibleLogin,
		}
	}
	autoCloseMeta := computeAutoCloseMeta(ticket, stateName, stateTypeID, time.Now().UTC())
	pendingReminderMeta := computePendingReminderMeta(ticket, stateName, stateTypeID, time.Now().UTC())
	ticketData := gin.H{
		"id":                 ticket.ID,
		"tn":                 ticket.TicketNumber,
		"subject":            ticket.Title,
		"status":             stateName,
		"state_type":         strings.ToLower(strings.Fields(stateName)[0]), // First word of state for badge colors
		"auto_close_pending": autoCloseMeta.pending,
		"pending_reminder":   pendingReminderMeta.pending,
		"is_closed":          isClosed,
		"priority":           priorityName,
		"priority_id":        ticket.TicketPriorityID,
		"queue":              queueName,
		"queue_id":           ticket.QueueID,
		"customer_name":      customerName,
		"customer_user_id":   ticket.CustomerUserID,
		"customer_id": func() string {
			if ticket.CustomerID != nil {
				return *ticket.CustomerID
			}
			return ""
		}(),
		"customer": gin.H{
			"name":  customerName,
			"email": customerEmail,
			"phone": customerPhone,
		},
		"agent":                              agent,
		"assigned_to":                        assignedTo,
		"owner":                              ownerName,
		"type":                               typeName,
		"service":                            "-", // TODO: Get from service table
		"sla":                                "-", // TODO: Get from SLA table
		"created":                            ticket.CreateTime.Format("2006-01-02 15:04"),
		"created_iso":                        ticket.CreateTime.UTC().Format(time.RFC3339),
		"updated":                            ticket.ChangeTime.Format("2006-01-02 15:04"),
		"updated_iso":                        ticket.ChangeTime.UTC().Format(time.RFC3339),
		"description":                        description,     // Raw description for display
		"description_json":                   descriptionJSON, // JSON-encoded for JavaScript
		"notes":                              notes,           // Pass notes array directly
		"note_bodies_json":                   noteBodiesJSON,  // JSON-encoded note bodies for JavaScript
		"description_is_html":                strings.Contains(description, "<") && strings.Contains(description, ">"),
		"time_total_minutes":                 totalMinutes,
		"time_total_hours":                   timeTotalHours,
		"time_total_remaining_minutes":       timeTotalRemainder,
		"time_total_has_hours":               hasTimeHours,
		"time_entries":                       timeEntries,
		"time_by_article":                    perArticleMinutes,
		"first_article_id":                   firstArticleID,
		"first_article_visible_for_customer": firstArticleVisibleForCustomer,
		"age":                                age,
		"status_id":                          ticket.TicketStateID,
	}

	if autoCloseMeta.at != "" && autoCloseMeta.pending {
		ticketData["auto_close_at"] = autoCloseMeta.at
		if autoCloseMeta.atISO != "" {
			ticketData["auto_close_at_iso"] = autoCloseMeta.atISO
		}
		ticketData["auto_close_overdue"] = autoCloseMeta.overdue
		ticketData["auto_close_relative"] = autoCloseMeta.relative
	}
	if pendingReminderMeta.pending {
		ticketData["pending_reminder"] = true
		ticketData["pending_reminder_has_time"] = pendingReminderMeta.hasTime
		if pendingReminderMeta.hasTime {
			ticketData["pending_reminder_at"] = pendingReminderMeta.at
			ticketData["pending_reminder_overdue"] = pendingReminderMeta.overdue
			if pendingReminderMeta.relative != "" {
				ticketData["pending_reminder_relative"] = pendingReminderMeta.relative
			}
			if pendingReminderMeta.atISO != "" {
				ticketData["pending_reminder_at_iso"] = pendingReminderMeta.atISO
			}
		} else {
			if pendingReminderMeta.message != "" {
				ticketData["pending_reminder_message"] = pendingReminderMeta.message
			}
			ticketData["pending_reminder_overdue"] = false
		}
	}

	// Customer panel (DRY: same details as ticket creation selection panel)
	var panelUser = gin.H{}
	var panelCompany = gin.H{}
	panelOpen := 0
	if ticket.CustomerUserID != nil && *ticket.CustomerUserID != "" {
		// Fetch customer user + company in one query
		var title, firstName, lastName, login, email, phone, mobile, customerID, compName, street, zip, city, country, url sql.NullString
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT cu.title, cu.first_name, cu.last_name, cu.login, cu.email, cu.phone, cu.mobile, cu.customer_id,
				   cc.name, cc.street, cc.zip, cc.city, cc.country, cc.url
			FROM customer_user cu
			LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
			WHERE cu.login = ?
		`), *ticket.CustomerUserID).Scan(
			&title, &firstName, &lastName, &login, &email, &phone, &mobile, &customerID,
			&compName, &street, &zip, &city, &country, &url)
		if err == nil {
			panelUser = gin.H{
				"Title":     title.String,
				"FirstName": firstName.String,
				"LastName":  lastName.String,
				"Login":     login.String,
				"Email":     email.String,
				"Phone":     phone.String,
				"Mobile":    mobile.String,
				"Comment":   "",
			}
			panelCompany = gin.H{
				"Name":     compName.String,
				"Street":   street.String,
				"Postcode": zip.String,
				"City":     city.String,
				"Country":  country.String,
				"URL":      url.String,
			}
			// Open tickets count for the same customer_id (exclude closed states)
			if customerID.Valid && customerID.String != "" {
				row := db.QueryRow(database.ConvertPlaceholders(`
					SELECT COUNT(*)
					FROM ticket t
					JOIN ticket_state s ON s.id = t.ticket_state_id
					WHERE t.customer_id = ? AND LOWER(s.name) NOT LIKE 'closed%'
				`), customerID.String)
				_ = row.Scan(&panelOpen) //nolint:errcheck // Defaults to 0
			}
		} else {
			// Fallback: show at least the login when customer user not found
			panelUser = gin.H{
				"Login":     *ticket.CustomerUserID,
				"FirstName": "",
				"LastName":  *ticket.CustomerUserID, // Show login as last name for display
				"Email":     "",
				"Phone":     "",
				"Mobile":    "",
				"Comment":   "(Customer user not found)",
			}
			// Try to get company info from customer_id if set
			if ticket.CustomerID != nil && *ticket.CustomerID != "" {
				var ccName, ccStreet, ccZip, ccCity, ccCountry, ccURL sql.NullString
				ccErr := db.QueryRow(database.ConvertPlaceholders(`
					SELECT name, street, zip, city, country, url FROM customer_company WHERE customer_id = ?
				`), *ticket.CustomerID).Scan(&ccName, &ccStreet, &ccZip, &ccCity, &ccCountry, &ccURL)
				if ccErr == nil {
					panelCompany = gin.H{
						"Name":     ccName.String,
						"Street":   ccStreet.String,
						"Postcode": ccZip.String,
						"City":     ccCity.String,
						"Country":  ccCountry.String,
						"URL":      ccURL.String,
					}
				}
				// Still count open tickets for this customer_id
				row := db.QueryRow(database.ConvertPlaceholders(`
					SELECT COUNT(*)
					FROM ticket t
					JOIN ticket_state s ON s.id = t.ticket_state_id
					WHERE t.customer_id = ? AND LOWER(s.name) NOT LIKE 'closed%'
				`), *ticket.CustomerID)
				_ = row.Scan(&panelOpen) //nolint:errcheck // Defaults to 0
			}
		}
	}

	requireTimeUnits := isTimeUnitsRequired(db)

	// Get dynamic field values for display on ticket zoom
	var dynamicFieldsDisplay []DynamicFieldDisplay
	dfDisplay, dfErr := GetDynamicFieldValuesForDisplay(ticket.ID, DFObjectTicket, "AgentTicketZoom")
	if dfErr != nil {
		log.Printf("Error getting dynamic field values for ticket %d: %v", ticket.ID, dfErr)
	} else {
		dynamicFieldsDisplay = dfDisplay
	}

	// Get dynamic fields for the note form (editable) - Ticket fields
	var noteFormDynamicFields []FieldWithScreenConfig
	noteFields, noteErr := GetFieldsForScreenWithConfig("AgentTicketNote", DFObjectTicket)
	if noteErr != nil {
		log.Printf("Error getting note form dynamic fields: %v", noteErr)
	} else {
		noteFormDynamicFields = noteFields
	}

	// Get Article dynamic fields for the note form
	var noteArticleDynamicFields []FieldWithScreenConfig
	noteArticleFields, noteArticleErr := GetFieldsForScreenWithConfig("AgentArticleNote", DFObjectArticle)
	if noteArticleErr != nil {
		log.Printf("Error getting note article dynamic fields: %v", noteArticleErr)
	} else {
		noteArticleDynamicFields = noteArticleFields
	}

	// Get dynamic fields for the close form (editable) - Ticket fields
	var closeFormDynamicFields []FieldWithScreenConfig
	closeFields, closeErr := GetFieldsForScreenWithConfig("AgentTicketClose", DFObjectTicket)
	if closeErr != nil {
		log.Printf("Error getting close form dynamic fields: %v", closeErr)
	} else {
		closeFormDynamicFields = closeFields
	}

	// Get Article dynamic fields for the close form
	var closeArticleDynamicFields []FieldWithScreenConfig
	closeArticleFields, closeArticleErr := GetFieldsForScreenWithConfig("AgentArticleClose", DFObjectArticle)
	if closeArticleErr != nil {
		log.Printf("Error getting close article dynamic fields: %v", closeArticleErr)
	} else {
		closeArticleDynamicFields = closeArticleFields
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/ticket_detail.pongo2", pongo2.Context{
		"Ticket":                    ticketData,
		"User":                      getUserMapForTemplate(c),
		"ActivePage":                "tickets",
		"CustomerPanelUser":         panelUser,
		"CustomerPanelCompany":      panelCompany,
		"CustomerPanelOpen":         panelOpen,
		"RequireNoteTimeUnits":      requireTimeUnits,
		"TicketStates":              ticketStates,
		"PendingStateIDs":           pendingStateIDs,
		"DynamicFields":             dynamicFieldsDisplay,
		"NoteFormDynamicFields":     noteFormDynamicFields,
		"NoteArticleDynamicFields":  noteArticleDynamicFields,
		"CloseFormDynamicFields":    closeFormDynamicFields,
		"CloseArticleDynamicFields": closeArticleDynamicFields,
	})
}

// handleGetTicket returns a specific ticket.
func handleGetTicket(c *gin.Context) {
	ticketID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Get ticket from repository
	ticketRepo := repository.NewTicketRepository(db)
	ticket, err := ticketRepo.GetByTicketNumber(ticketID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Ticket not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to retrieve ticket",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"ticket":  ticket,
	})
}

// redirectLegacyTicketID handles legacy ticket ID to TN conversion.
func redirectLegacyTicketID(c *gin.Context, legacyID string) {
	if strings.TrimSpace(legacyID) == "" {
		c.Redirect(http.StatusFound, "/tickets")
		return
	}

	// If it's clearly a TN already (non-numeric), redirect directly
	if _, err := strconv.Atoi(legacyID); err != nil {
		c.Redirect(http.StatusMovedPermanently, "/ticket/"+legacyID)
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.Redirect(http.StatusFound, "/ticket/"+legacyID)
		return
	}

	ticketRepo := repository.NewTicketRepository(db)
	idNum, _ := strconv.Atoi(legacyID) //nolint:errcheck // Validated earlier
	t, terr := ticketRepo.GetByID(uint(idNum))
	if terr == nil && t != nil && t.TicketNumber != "" {
		c.Redirect(http.StatusMovedPermanently, "/ticket/"+t.TicketNumber)
		return
	}

	c.Redirect(http.StatusFound, "/ticket/"+legacyID)
}

// HandleLegacyAgentTicketViewRedirect exported for YAML routing.
func HandleLegacyAgentTicketViewRedirect(c *gin.Context) {
	redirectLegacyTicketID(c, c.Param("id"))
}

// HandleLegacyTicketsViewRedirect exported for YAML routing.
func HandleLegacyTicketsViewRedirect(c *gin.Context) {
	redirectLegacyTicketID(c, c.Param("id"))
}
