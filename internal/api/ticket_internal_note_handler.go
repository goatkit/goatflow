package api

import (
	"database/sql"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/pkg/markdown"
)

// InternalNote is an internal (agent-only) note on a ticket. It is stored as an OTRS
// article on the Internal communication channel that is not visible to the customer.
type InternalNote struct {
	ID               int       `json:"id"`
	TicketID         int       `json:"ticket_id"`
	Content          string    `json:"content"`
	FormattedContent string    `json:"formatted_content"`
	AuthorID         int       `json:"author_id"`
	AuthorName       string    `json:"author_name"`
	Visibility       string    `json:"visibility"`       // always "internal"
	CustomerVisible  bool      `json:"customer_visible"` // always false
	Mentions         []string  `json:"mentions"`
	HasMentions      bool      `json:"has_mentions"`
	HasTeamMention   bool      `json:"has_team_mention"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

const internalNoteSelect = `
	SELECT a.id, a.ticket_id, a.create_by, a.create_time, a.change_time,
	       COALESCE(adm.a_body, ''),
	       COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), COALESCE(u.login, '')
	FROM article a
	LEFT JOIN article_data_mime adm ON adm.article_id = a.id
	LEFT JOIN users u ON u.id = a.create_by
	WHERE a.ticket_id = ? AND a.communication_channel_id = ? AND a.is_visible_for_customer = 0`

func scanInternalNote(row interface{ Scan(...any) error }) (*InternalNote, error) {
	var n InternalNote
	var first, last, login string
	if err := row.Scan(&n.ID, &n.TicketID, &n.AuthorID, &n.CreatedAt, &n.UpdatedAt, &n.Content, &first, &last, &login); err != nil {
		return nil, err
	}
	n.AuthorName = strings.TrimSpace(first + " " + last)
	if n.AuthorName == "" {
		n.AuthorName = login
	}
	n.FormattedContent = RenderMarkdown(n.Content)
	n.Visibility = "internal"
	n.CustomerVisible = false
	n.Mentions = uniqueStrings(extractMentions(n.Content))
	n.HasMentions = len(n.Mentions) > 0
	for _, m := range n.Mentions {
		if strings.HasPrefix(m, "team-") {
			n.HasTeamMention = true
			break
		}
	}
	return &n, nil
}

func loadInternalNote(db *sql.DB, ticketID, noteID int) (*InternalNote, error) {
	return scanInternalNote(db.QueryRow(database.ConvertPlaceholders(internalNoteSelect+` AND a.id = ?`),
		ticketID, constants.CommunicationChannelInternal, noteID))
}

func internalNoteDB(c *gin.Context, op string) *sql.DB {
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("%s: database unavailable: %v", op, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return nil
	}
	return db
}

// canModifyInternalNote reports whether the current user may edit/delete the note:
// its author, or an admin.
func canModifyInternalNote(c *gin.Context, note *InternalNote) bool {
	if role, _ := c.Get("user_role"); role == "admin" || role == "Admin" { //nolint:errcheck // nil when absent
		return true
	}
	return note.AuthorID == GetUserIDFromCtx(c, 0)
}

// RenderMarkdown converts markdown content to sanitized HTML with Tailwind
// styling. Markdown→HTML defers to the shared pkg/markdown renderer (goldmark
// GFM + bluemonday), so ticket notes and plugin UI render identically and any
// raw/unsafe HTML is stripped; addTailwindClasses layers Tailwind styling on
// top of the sanitized output.
func RenderMarkdown(content string) string {
	return addTailwindClasses(markdown.Render(content))
}

// addTailwindClasses adds Tailwind CSS classes to HTML elements for consistent styling.
func addTailwindClasses(html string) string {
	// Headers
	html = strings.ReplaceAll(html, "<h1>", `<h1 class="text-xl font-bold mb-2 text-gray-900 dark:text-white">`)
	html = strings.ReplaceAll(html, "<h2>", `<h2 class="text-lg font-semibold mb-2 mt-4 text-gray-800 dark:text-gray-100">`)
	html = strings.ReplaceAll(html, "<h3>", `<h3 class="text-base font-medium mb-1 mt-3 text-gray-700 dark:text-gray-200">`)
	html = strings.ReplaceAll(html, "<h4>", `<h4 class="text-sm font-medium mb-1 mt-2 text-gray-600 dark:text-gray-300">`)

	// Text elements
	html = strings.ReplaceAll(html, "<p>", `<p class="mb-2 text-gray-700 dark:text-gray-300">`)
	html = strings.ReplaceAll(html, "<strong>", `<strong class="font-semibold">`)
	html = strings.ReplaceAll(html, "<em>", `<em class="italic">`)
	html = strings.ReplaceAll(html, "<del>", `<del class="line-through">`)
	html = strings.ReplaceAll(html, "<code>", `<code class="bg-gray-100 dark:bg-gray-800 px-1 py-0.5 rounded text-sm font-mono">`)

	// Lists
	html = strings.ReplaceAll(html, "<ul>", `<ul class="list-disc mb-2 space-y-1 text-gray-700 dark:text-gray-300">`)
	html = strings.ReplaceAll(html, "<ol>", `<ol class="list-decimal mb-2 space-y-1 text-gray-700 dark:text-gray-300">`)
	html = strings.ReplaceAll(html, "<li>", `<li class="ml-4">`)

	// Blockquotes
	html = strings.ReplaceAll(html, "<blockquote>",
		`<blockquote class="border-l-4 border-gray-300 pl-4 italic text-gray-600 dark:text-gray-400 mb-2">`)

	// Code blocks
	html = strings.ReplaceAll(html, "<pre>", `<pre class="bg-gray-100 dark:bg-gray-800 p-3 rounded mb-2 overflow-x-auto">`)

	// Tables - let CSS handle the styling completely
	html = strings.ReplaceAll(html, "<table>", `<table>`)
	html = strings.ReplaceAll(html, "<thead>", `<thead>`)
	html = strings.ReplaceAll(html, "<tbody>", `<tbody>`)
	html = strings.ReplaceAll(html, "<tr>", `<tr>`)
	html = strings.ReplaceAll(html, "<th>", `<th>`)
	html = strings.ReplaceAll(html, "<td>", `<td>`)

	return html
}

// HandleCreateInternalNote creates a new internal note.
func HandleCreateInternalNote(c *gin.Context) {
	ticketID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	if userRole, _ := c.Get("user_role"); userRole == "customer" { //nolint:errcheck // nil when absent
		c.JSON(http.StatusForbidden, gin.H{"error": "Only agents can create internal notes"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	db := internalNoteDB(c, "HandleCreateInternalNote")
	if db == nil {
		return
	}

	var exists int
	if err := db.QueryRow(database.ConvertPlaceholders(`SELECT 1 FROM ticket WHERE id = ?`), ticketID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
		log.Printf("HandleCreateInternalNote: lookup ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create internal note"})
		return
	}

	authorID, ok := auditUserID(c)
	if !ok {
		return
	}
	tx, err := db.Begin()
	if err != nil {
		log.Printf("HandleCreateInternalNote: begin: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create internal note"})
		return
	}
	defer func() { _ = tx.Rollback() }()

	articleID, err := insertArticle(tx, ArticleInsertParams{
		TicketID:             int64(ticketID),
		CommunicationChannel: constants.CommunicationChannelInternal,
		IsVisibleForCustomer: 0,
		CreateBy:             int64(authorID),
	})
	if err == nil {
		err = insertArticleMimeData(tx, ArticleMimeParams{
			ArticleID:    articleID,
			From:         "Agent",
			Subject:      defaultNoteSubject(constants.CommunicationChannelInternal),
			Body:         req.Content,
			ContentType:  "text/plain; charset=utf-8",
			IncomingTime: time.Now().Unix(),
			CreateBy:     int64(authorID),
		})
	}
	if err == nil {
		_, err = tx.Exec(database.ConvertPlaceholders(
			`UPDATE ticket SET change_time = CURRENT_TIMESTAMP, change_by = ? WHERE id = ?`), authorID, ticketID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		log.Printf("HandleCreateInternalNote: ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create internal note"})
		return
	}

	note, err := loadInternalNote(db, ticketID, int(articleID))
	if err != nil {
		log.Printf("HandleCreateInternalNote: reload note %d: %v", articleID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load internal note"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Internal note added successfully",
		"note_id": note.ID,
		"note":    note,
	})
}

// HandleGetInternalNotes returns internal notes for a ticket.
func HandleGetInternalNotes(c *gin.Context) {
	ticketID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	if userRole, _ := c.Get("user_role"); userRole == "customer" { //nolint:errcheck // nil when absent
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view internal notes"})
		return
	}

	search := strings.ToLower(c.Query("search"))
	onlyMentions := c.Query("has_mentions") == "true"

	db := internalNoteDB(c, "HandleGetInternalNotes")
	if db == nil {
		return
	}

	rows, err := db.Query(database.ConvertPlaceholders(internalNoteSelect+` ORDER BY a.create_time, a.id`),
		ticketID, constants.CommunicationChannelInternal)
	if err != nil {
		log.Printf("HandleGetInternalNotes: ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load internal notes"})
		return
	}
	defer rows.Close()

	notes := []InternalNote{}
	for rows.Next() {
		note, err := scanInternalNote(rows)
		if err != nil {
			log.Printf("HandleGetInternalNotes: scan ticket %d: %v", ticketID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load internal notes"})
			return
		}
		if search != "" && !strings.Contains(strings.ToLower(note.Content), search) {
			continue
		}
		if onlyMentions && !note.HasMentions {
			continue
		}
		notes = append(notes, *note)
	}
	if err := rows.Err(); err != nil {
		log.Printf("HandleGetInternalNotes: iterate ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load internal notes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"notes": notes,
		"total": len(notes),
	})
}

// parseInternalNoteIDs reads :id and :note_id; it writes a 400 and returns ok=false on error.
func parseInternalNoteIDs(c *gin.Context) (ticketID, noteID int, ok bool) {
	ticketID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return 0, 0, false
	}
	noteID, err = strconv.Atoi(c.Param("note_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return 0, 0, false
	}
	return ticketID, noteID, true
}

// findInternalNoteForChange loads the note and checks ownership, writing the error response itself.
func findInternalNoteForChange(c *gin.Context, db *sql.DB, ticketID, noteID int, op, denied string) *InternalNote {
	note, err := loadInternalNote(db, ticketID, noteID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Internal note not found"})
		return nil
	}
	if err != nil {
		log.Printf("%s: load note %d: %v", op, noteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load internal note"})
		return nil
	}
	if !canModifyInternalNote(c, note) {
		c.JSON(http.StatusForbidden, gin.H{"error": denied})
		return nil
	}
	return note
}

// HandleUpdateInternalNote updates the content of an existing internal note.
func HandleUpdateInternalNote(c *gin.Context) {
	ticketID, noteID, ok := parseInternalNoteIDs(c)
	if !ok {
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	db := internalNoteDB(c, "HandleUpdateInternalNote")
	if db == nil {
		return
	}
	if findInternalNoteForChange(c, db, ticketID, noteID, "HandleUpdateInternalNote", "You can only edit your own notes") == nil {
		return
	}

	userID, ok := auditUserID(c)
	if !ok {
		return
	}
	tx, err := db.Begin()
	if err != nil {
		log.Printf("HandleUpdateInternalNote: begin: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update internal note"})
		return
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec(database.ConvertPlaceholders(
		`UPDATE article_data_mime SET a_body = ?, change_time = CURRENT_TIMESTAMP, change_by = ? WHERE article_id = ?`),
		req.Content, userID, noteID)
	if err == nil {
		_, err = tx.Exec(database.ConvertPlaceholders(
			`UPDATE article SET change_time = CURRENT_TIMESTAMP, change_by = ? WHERE id = ?`), userID, noteID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		log.Printf("HandleUpdateInternalNote: note %d: %v", noteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update internal note"})
		return
	}

	note, err := loadInternalNote(db, ticketID, noteID)
	if err != nil {
		log.Printf("HandleUpdateInternalNote: reload note %d: %v", noteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load internal note"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Internal note updated successfully",
		"note":    note,
	})
}

// HandleDeleteInternalNote deletes an internal note (the underlying article).
func HandleDeleteInternalNote(c *gin.Context) {
	ticketID, noteID, ok := parseInternalNoteIDs(c)
	if !ok {
		return
	}

	db := internalNoteDB(c, "HandleDeleteInternalNote")
	if db == nil {
		return
	}
	if findInternalNoteForChange(c, db, ticketID, noteID, "HandleDeleteInternalNote", "You can only delete your own notes") == nil {
		return
	}

	actorID, ok := auditUserID(c)
	if !ok {
		return
	}
	if err := deleteArticle(c.Request.Context(), db, ticketID, noteID, actorID); err != nil {
		log.Printf("HandleDeleteInternalNote: note %d: %v", noteID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete internal note"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Internal note deleted successfully",
	})
}

// Helper functions

func extractMentions(content string) []string {
	mentions := []string{}
	// Match @username or @team-name patterns
	re := regexp.MustCompile(`@([\w\.\-]+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			mentions = append(mentions, match[1])
		}
	}
	return mentions
}

func uniqueStrings(strings []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range strings {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
