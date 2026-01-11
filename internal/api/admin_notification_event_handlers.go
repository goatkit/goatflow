package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/i18n"
)

// NotificationEvent represents a notification event from the database.
type NotificationEvent struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	ValidID    int       `json:"valid_id"`
	Comments   string    `json:"comments"`
	CreateTime time.Time `json:"create_time"`
	CreateBy   int       `json:"create_by"`
	ChangeTime time.Time `json:"change_time"`
	ChangeBy   int       `json:"change_by"`
}

// NotificationEventItem represents an event trigger condition.
type NotificationEventItem struct {
	NotificationID int    `json:"notification_id"`
	EventKey       string `json:"event_key"`
	EventValue     string `json:"event_value"`
}

// NotificationEventMessage represents a notification message for a specific language.
type NotificationEventMessage struct {
	ID             int    `json:"id"`
	NotificationID int    `json:"notification_id"`
	Subject        string `json:"subject"`
	Text           string `json:"text"`
	ContentType    string `json:"content_type"`
	Language       string `json:"language"`
}

// NotificationEventFull represents a complete notification with all related data.
type NotificationEventFull struct {
	NotificationEvent
	Events     []string                            `json:"events"`
	Filters    map[string][]string                 `json:"filters"`
	Recipients map[string][]string                 `json:"recipients"`
	Messages   map[string]NotificationEventMessage `json:"messages"`
}

// NotificationEventInput represents the JSON input for creating/updating notification events.
type NotificationEventInput struct {
	Name       string                              `json:"name" binding:"required"`
	ValidID    int                                 `json:"valid_id"`
	Comments   string                              `json:"comments"`
	Events     []string                            `json:"events"`
	Filters    map[string][]string                 `json:"filters"`
	Recipients map[string][]string                 `json:"recipients"`
	Messages   map[string]NotificationEventMessage `json:"messages"`
}

// Available ticket events that can trigger notifications.
var TicketEvents = []string{
	"TicketCreate",
	"TicketDelete",
	"TicketTitleUpdate",
	"TicketQueueUpdate",
	"TicketTypeUpdate",
	"TicketServiceUpdate",
	"TicketSLAUpdate",
	"TicketCustomerUpdate",
	"TicketPendingTimeUpdate",
	"TicketLockUpdate",
	"TicketStateUpdate",
	"TicketOwnerUpdate",
	"TicketResponsibleUpdate",
	"TicketPriorityUpdate",
	"TicketSubscribe",
	"TicketUnsubscribe",
	"TicketFlagSet",
	"TicketFlagDelete",
	"TicketMerge",
	"EscalationResponseTimeNotifyBefore",
	"EscalationResponseTimeStart",
	"EscalationResponseTimeStop",
	"EscalationUpdateTimeNotifyBefore",
	"EscalationUpdateTimeStart",
	"EscalationUpdateTimeStop",
	"EscalationSolutionTimeNotifyBefore",
	"EscalationSolutionTimeStart",
	"EscalationSolutionTimeStop",
}

// Article events that can trigger notifications.
var ArticleEvents = []string{
	"ArticleCreate",
	"ArticleSend",
	"ArticleBounce",
	"ArticleAgentNotification",
	"ArticleCustomerNotification",
}

// loadNotificationEvents loads all notification events from the database.
func loadNotificationEvents(ctx context.Context, db *sql.DB) ([]NotificationEvent, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id, COALESCE(comments, ''), create_time, create_by, change_time, change_by
		FROM notification_event
		ORDER BY name`)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []NotificationEvent
	for rows.Next() {
		var e NotificationEvent
		if err := rows.Scan(&e.ID, &e.Name, &e.ValidID, &e.Comments, &e.CreateTime, &e.CreateBy, &e.ChangeTime, &e.ChangeBy); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

// loadNotificationEventByID loads a single notification event by ID.
func loadNotificationEventByID(ctx context.Context, db *sql.DB, id int) (*NotificationEventFull, error) {
	// Load base notification
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id, COALESCE(comments, ''), create_time, create_by, change_time, change_by
		FROM notification_event
		WHERE id = ?`)

	var e NotificationEventFull
	err := db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.Name, &e.ValidID, &e.Comments, &e.CreateTime, &e.CreateBy, &e.ChangeTime, &e.ChangeBy)
	if err != nil {
		return nil, err
	}

	// Load event items (triggers and filters)
	e.Events = []string{}
	e.Filters = make(map[string][]string)
	e.Recipients = make(map[string][]string)

	itemQuery := database.ConvertPlaceholders(`
		SELECT event_key, event_value
		FROM notification_event_item
		WHERE notification_id = ?`)

	itemRows, err := db.QueryContext(ctx, itemQuery, id)
	if err == nil {
		defer itemRows.Close()
		for itemRows.Next() {
			var key, value string
			if err := itemRows.Scan(&key, &value); err == nil {
				switch key {
				case "Events":
					e.Events = append(e.Events, value)
				case "Recipients", "RecipientAgents", "RecipientRoles", "RecipientGroups":
					e.Recipients[key] = append(e.Recipients[key], value)
				default:
					// Filter conditions (StateID, QueueID, PriorityID, etc.)
					e.Filters[key] = append(e.Filters[key], value)
				}
			}
		}
	}

	// Load messages per language
	e.Messages = make(map[string]NotificationEventMessage)

	msgQuery := database.ConvertPlaceholders(`
		SELECT id, notification_id, subject, text, content_type, language
		FROM notification_event_message
		WHERE notification_id = ?`)

	msgRows, err := db.QueryContext(ctx, msgQuery, id)
	if err == nil {
		defer msgRows.Close()
		for msgRows.Next() {
			var msg NotificationEventMessage
			if err := msgRows.Scan(&msg.ID, &msg.NotificationID, &msg.Subject, &msg.Text, &msg.ContentType, &msg.Language); err == nil {
				e.Messages[msg.Language] = msg
			}
		}
	}

	return &e, nil
}

// loadLocksForForm loads all locks for form dropdowns.
func loadLocksForForm(ctx context.Context, db *sql.DB) []LookupItem {
	if db == nil {
		return nil
	}
	query := database.ConvertPlaceholders(`
		SELECT id, name FROM ticket_lock_type ORDER BY id`)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []LookupItem
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// loadAgentsForForm loads all valid agents for recipient selection.
func loadAgentsForForm(ctx context.Context, db *sql.DB) []LookupItem {
	if db == nil {
		return nil
	}
	query := database.ConvertPlaceholders(`
		SELECT id, CONCAT(first_name, ' ', last_name, ' (', login, ')') as name
		FROM users
		WHERE valid_id = 1
		ORDER BY login`)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []LookupItem
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// loadRolesForForm loads all valid roles for recipient selection.
func loadRolesForForm(ctx context.Context, db *sql.DB) []LookupItem {
	if db == nil {
		return nil
	}
	query := database.ConvertPlaceholders(`
		SELECT id, name FROM roles WHERE valid_id = 1 ORDER BY name`)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []LookupItem
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// loadGroupsForForm loads all valid groups for recipient selection.
func loadGroupsForForm(ctx context.Context, db *sql.DB) []LookupItem {
	if db == nil {
		return nil
	}
	query := database.ConvertPlaceholders(`
		SELECT id, name FROM groups_table WHERE valid_id = 1 ORDER BY name`)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []LookupItem
	for rows.Next() {
		var item LookupItem
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// loadLanguagesForForm loads available languages dynamically from i18n translation files.
func loadLanguagesForForm() []LookupItem {
	instance := i18n.GetInstance()
	languages := instance.GetSupportedLanguages()

	items := make([]LookupItem, len(languages))
	for i, lang := range languages {
		items[i] = LookupItem{ID: i, Name: lang}
	}
	return items
}

// HandleAdminNotificationEvents renders the notification events management page.
func HandleAdminNotificationEvents(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	events, err := loadNotificationEvents(c.Request.Context(), db)
	if err != nil {
		events = []NotificationEvent{}
	}

	if getPongo2Renderer() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template renderer not available"})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/notification_events.pongo2", pongo2.Context{
		"Title":      "Ticket Notifications",
		"Events":     events,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// HandleAdminNotificationEventNew renders the new notification event form.
func HandleAdminNotificationEventNew(c *gin.Context) {
	if getPongo2Renderer() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template renderer not available"})
		return
	}

	db, _ := database.GetDB()
	ctx := c.Request.Context()

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/notification_event_form.pongo2", pongo2.Context{
		"Title":         "New Ticket Notification",
		"IsNew":         true,
		"Event":         nil,
		"TicketEvents":  TicketEvents,
		"ArticleEvents": ArticleEvents,
		"Queues":        loadQueuesForForm(ctx, db),
		"Priorities":    loadPrioritiesForForm(ctx, db),
		"States":        loadStatesForForm(ctx, db),
		"Types":         loadTypesForForm(ctx, db),
		"Locks":         loadLocksForForm(ctx, db),
		"Agents":        loadAgentsForForm(ctx, db),
		"Roles":         loadRolesForForm(ctx, db),
		"Groups":        loadGroupsForForm(ctx, db),
		"Languages":     loadLanguagesForForm(),
		"User":          getUserMapForTemplate(c),
		"ActivePage":    "admin",
	})
}

// HandleAdminNotificationEventEdit renders the notification event edit form.
func HandleAdminNotificationEventEdit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not available"})
		return
	}

	ctx := c.Request.Context()

	event, err := loadNotificationEventByID(ctx, db, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		return
	}

	if getPongo2Renderer() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template renderer not available"})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/notification_event_form.pongo2", pongo2.Context{
		"Title":         "Edit Ticket Notification",
		"IsNew":         false,
		"Event":         event,
		"TicketEvents":  TicketEvents,
		"ArticleEvents": ArticleEvents,
		"Queues":        loadQueuesForForm(ctx, db),
		"Priorities":    loadPrioritiesForForm(ctx, db),
		"States":        loadStatesForForm(ctx, db),
		"Types":         loadTypesForForm(ctx, db),
		"Locks":         loadLocksForForm(ctx, db),
		"Agents":        loadAgentsForForm(ctx, db),
		"Roles":         loadRolesForForm(ctx, db),
		"Groups":        loadGroupsForForm(ctx, db),
		"Languages":     loadLanguagesForForm(),
		"User":          getUserMapForTemplate(c),
		"ActivePage":    "admin",
	})
}

// HandleAdminNotificationEventGet returns a notification event's details as JSON.
func HandleAdminNotificationEventGet(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid notification ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database not available"})
		return
	}

	event, err := loadNotificationEventByID(c.Request.Context(), db, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Notification not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": event})
}

// HandleCreateNotificationEvent creates a new notification event.
func HandleCreateNotificationEvent(c *gin.Context) {
	var input NotificationEventInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Notification name is required"})
		return
	}

	if input.ValidID == 0 {
		input.ValidID = 1
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database not available"})
		return
	}

	ctx := c.Request.Context()
	userID := getUserID(c)
	now := time.Now()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Insert notification_event
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO notification_event (name, valid_id, comments, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)

	result, err := tx.ExecContext(ctx, insertQuery, input.Name, input.ValidID, input.Comments, now, userID, now, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create notification: " + err.Error()})
		return
	}

	notificationID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to get notification ID"})
		return
	}

	// Insert event items (events, filters, recipients)
	itemQuery := database.ConvertPlaceholders(`
		INSERT INTO notification_event_item (notification_id, event_key, event_value)
		VALUES (?, ?, ?)`)

	// Insert events
	for _, event := range input.Events {
		if _, err := tx.ExecContext(ctx, itemQuery, notificationID, "Events", event); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save events"})
			return
		}
	}

	// Insert filters
	for key, values := range input.Filters {
		for _, value := range values {
			if _, err := tx.ExecContext(ctx, itemQuery, notificationID, key, value); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save filters"})
				return
			}
		}
	}

	// Insert recipients
	for key, values := range input.Recipients {
		for _, value := range values {
			if _, err := tx.ExecContext(ctx, itemQuery, notificationID, key, value); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save recipients"})
				return
			}
		}
	}

	// Insert messages
	msgQuery := database.ConvertPlaceholders(`
		INSERT INTO notification_event_message (notification_id, subject, text, content_type, language)
		VALUES (?, ?, ?, ?, ?)`)

	for lang, msg := range input.Messages {
		contentType := msg.ContentType
		if contentType == "" {
			contentType = "text/plain"
		}
		if _, err := tx.ExecContext(ctx, msgQuery, notificationID, msg.Subject, msg.Text, contentType, lang); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save message for " + lang})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "id": notificationID, "message": "Notification created successfully"})
}

// HandleUpdateNotificationEvent updates an existing notification event.
func HandleUpdateNotificationEvent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid notification ID"})
		return
	}

	var input NotificationEventInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request: " + err.Error()})
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Notification name is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database not available"})
		return
	}

	ctx := c.Request.Context()
	userID := getUserID(c)
	now := time.Now()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Update notification_event
	updateQuery := database.ConvertPlaceholders(`
		UPDATE notification_event
		SET name = ?, valid_id = ?, comments = ?, change_time = ?, change_by = ?
		WHERE id = ?`)

	result, err := tx.ExecContext(ctx, updateQuery, input.Name, input.ValidID, input.Comments, now, userID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update notification: " + err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Notification not found"})
		return
	}

	// Delete existing items
	deleteItemsQuery := database.ConvertPlaceholders(`DELETE FROM notification_event_item WHERE notification_id = ?`)
	if _, err := tx.ExecContext(ctx, deleteItemsQuery, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to clear existing items"})
		return
	}

	// Delete existing messages
	deleteMsgsQuery := database.ConvertPlaceholders(`DELETE FROM notification_event_message WHERE notification_id = ?`)
	if _, err := tx.ExecContext(ctx, deleteMsgsQuery, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to clear existing messages"})
		return
	}

	// Re-insert items
	itemQuery := database.ConvertPlaceholders(`
		INSERT INTO notification_event_item (notification_id, event_key, event_value)
		VALUES (?, ?, ?)`)

	for _, event := range input.Events {
		if _, err := tx.ExecContext(ctx, itemQuery, id, "Events", event); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save events"})
			return
		}
	}

	for key, values := range input.Filters {
		for _, value := range values {
			if _, err := tx.ExecContext(ctx, itemQuery, id, key, value); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save filters"})
				return
			}
		}
	}

	for key, values := range input.Recipients {
		for _, value := range values {
			if _, err := tx.ExecContext(ctx, itemQuery, id, key, value); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save recipients"})
				return
			}
		}
	}

	// Re-insert messages
	msgQuery := database.ConvertPlaceholders(`
		INSERT INTO notification_event_message (notification_id, subject, text, content_type, language)
		VALUES (?, ?, ?, ?, ?)`)

	for lang, msg := range input.Messages {
		contentType := msg.ContentType
		if contentType == "" {
			contentType = "text/plain"
		}
		if _, err := tx.ExecContext(ctx, msgQuery, id, msg.Subject, msg.Text, contentType, lang); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save message for " + lang})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Notification updated successfully"})
}

// HandleDeleteNotificationEvent deletes a notification event.
func HandleDeleteNotificationEvent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid notification ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database not available"})
		return
	}

	ctx := c.Request.Context()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Delete messages first (foreign key constraint)
	deleteMsgsQuery := database.ConvertPlaceholders(`DELETE FROM notification_event_message WHERE notification_id = ?`)
	if _, err := tx.ExecContext(ctx, deleteMsgsQuery, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete messages"})
		return
	}

	// Delete items
	deleteItemsQuery := database.ConvertPlaceholders(`DELETE FROM notification_event_item WHERE notification_id = ?`)
	if _, err := tx.ExecContext(ctx, deleteItemsQuery, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete items"})
		return
	}

	// Delete notification
	deleteQuery := database.ConvertPlaceholders(`DELETE FROM notification_event WHERE id = ?`)
	result, err := tx.ExecContext(ctx, deleteQuery, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete notification"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Notification not found"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Notification deleted successfully"})
}
