package api

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// InternalNote represents an internal note on a ticket
type InternalNote struct {
	ID              int       `json:"id"`
	TicketID        int       `json:"ticket_id"`
	Content         string    `json:"content"`
	FormattedContent string   `json:"formatted_content"`
	AuthorID        int       `json:"author_id"`
	AuthorName      string    `json:"author_name"`
	Visibility      string    `json:"visibility"` // always "internal"
	CustomerVisible bool      `json:"customer_visible"` // always false
	IsPriority      bool      `json:"is_priority"`
	Category        string    `json:"category"`
	Mentions        []string  `json:"mentions"`
	HasMentions     bool      `json:"has_mentions"`
	HasTeamMention  bool      `json:"has_team_mention"`
	Attachments     []int     `json:"attachments"`
	IsEdited        bool      `json:"is_edited"`
	EditedAt        *time.Time `json:"edited_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// NoteHistory represents edit history for a note
type NoteHistory struct {
	ID        int       `json:"id"`
	NoteID    int       `json:"note_id"`
	Content   string    `json:"content"`
	EditedBy  int       `json:"edited_by"`
	EditedAt  time.Time `json:"edited_at"`
	Version   int       `json:"version"`
}

// Mock data for development
var internalNotes = map[int]map[int]*InternalNote{
	1: { // Ticket ID 1
		1: {
			ID:              1,
			TicketID:        1,
			Content:         "Customer has VIP status - expedite resolution",
			FormattedContent: "Customer has VIP status - expedite resolution",
			AuthorID:        1,
			AuthorName:      "John Agent",
			Visibility:      "internal",
			CustomerVisible: false,
			Category:        "general",
			CreatedAt:       time.Now().Add(-24 * time.Hour),
			UpdatedAt:       time.Now().Add(-24 * time.Hour),
		},
		2: {
			ID:              2,
			TicketID:        1,
			Content:         "Technical analysis: Database connection timeout issue",
			FormattedContent: "Technical analysis: Database connection timeout issue",
			AuthorID:        1,
			AuthorName:      "John Agent",
			Visibility:      "internal",
			CustomerVisible: false,
			Category:        "technical",
			CreatedAt:       time.Now().Add(-12 * time.Hour),
			UpdatedAt:       time.Now().Add(-12 * time.Hour),
		},
		3: {
			ID:              3,
			TicketID:        1,
			Content:         "Note from another user",
			FormattedContent: "Note from another user",
			AuthorID:        2,
			AuthorName:      "Jane Support",
			Visibility:      "internal",
			CustomerVisible: false,
			CreatedAt:       time.Now().Add(-6 * time.Hour),
			UpdatedAt:       time.Now().Add(-6 * time.Hour),
		},
	},
}

var nextNoteID = 4
var noteHistories = map[int][]NoteHistory{
	1: { // Mock history for note ID 1
		{
			ID:       1,
			NoteID:   1,
			Content:  "Original content before edit",
			EditedBy: 1,
			EditedAt: time.Now().Add(-1 * time.Hour),
			Version:  1,
		},
	},
}

// Mock tickets for validation
var mockTickets = map[int]bool{
	1: true,
	2: true,
	3: true,
}

// RenderMarkdown converts markdown content to HTML with Tailwind styling
func RenderMarkdown(content string) string {
	// Create a Goldmark instance with extensions
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
		),
		goldmark.WithParserOptions(
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML in markdown
		),
	)

	// Render to HTML
	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		log.Printf("ERROR: Failed to render markdown: %v", err)
		return content // Fallback to original content
	}

	htmlContent := buf.String()

	// Add Tailwind classes with minimal string replacements
	htmlContent = addTailwindClasses(htmlContent)

	return htmlContent
}

// addTailwindClasses adds Tailwind CSS classes to HTML elements for consistent styling
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
	html = strings.ReplaceAll(html, "<blockquote>", `<blockquote class="border-l-4 border-gray-300 pl-4 italic text-gray-600 dark:text-gray-400 mb-2">`)

	// Code blocks
	html = strings.ReplaceAll(html, "<pre>", `<pre class="bg-gray-100 dark:bg-gray-800 p-3 rounded mb-2 overflow-x-auto">`)

	// Tables
	html = strings.ReplaceAll(html, "<table>", `<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 mb-2">`)
	html = strings.ReplaceAll(html, "<thead>", `<thead class="bg-gray-50 dark:bg-gray-800">`)
	html = strings.ReplaceAll(html, "<tbody>", `<tbody class="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">`)
	html = strings.ReplaceAll(html, "<tr>", `<tr class="text-gray-700 dark:text-gray-300">`)
	html = strings.ReplaceAll(html, "<th>", `<th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">`)
	html = strings.ReplaceAll(html, "<td>", `<td class="px-3 py-2 whitespace-nowrap text-sm">`)

	return html
}

// HandleCreateInternalNote creates a new internal note
func HandleCreateInternalNote(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Check if ticket exists
	if !mockTickets[ticketID] {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	// Check permissions
	userRole, _ := c.Get("user_role")
	if userRole == "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only agents can create internal notes"})
		return
	}

	var req struct {
		Content       string `json:"content"`
		Visibility    string `json:"visibility"`
		IsPriority    bool   `json:"is_priority"`
		Category      string `json:"category"`
		Mentions      []string `json:"mentions"`
		AttachmentIDs []int  `json:"attachment_ids"`
		NotifyUsers   bool   `json:"notify_users"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	userID, _ := c.Get("user_id")
	userName, exists := c.Get("user_name")
	if !exists {
		userName = "Test User"
	}

	// Extract mentions from content
	mentions := extractMentions(req.Content)
	if len(req.Mentions) > 0 {
		mentions = append(mentions, req.Mentions...)
	}
	mentions = uniqueStrings(mentions)

	// Check for team mentions
	hasTeamMention := false
	for _, mention := range mentions {
		if strings.HasPrefix(mention, "team-") {
			hasTeamMention = true
			break
		}
	}

	// Create note
	note := &InternalNote{
		ID:              nextNoteID,
		TicketID:        ticketID,
		Content:         req.Content,
		FormattedContent: RenderMarkdown(req.Content),
		AuthorID:        userID.(int),
		AuthorName:      userName.(string),
		Visibility:      "internal",
		CustomerVisible: false,
		IsPriority:      req.IsPriority,
		Category:        req.Category,
		Mentions:        mentions,
		HasMentions:     len(mentions) > 0,
		HasTeamMention:  hasTeamMention,
		Attachments:     req.AttachmentIDs,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Initialize ticket notes map if needed
	if internalNotes[ticketID] == nil {
		internalNotes[ticketID] = make(map[int]*InternalNote)
	}
	
	internalNotes[ticketID][nextNoteID] = note
	nextNoteID++

	response := gin.H{
		"message": "Internal note added successfully",
		"note_id": note.ID,
		"note":    note,
	}

	// Mock notification sending
	if req.NotifyUsers && len(mentions) > 0 {
		response["notifications_sent"] = mentions
	}

	c.JSON(http.StatusCreated, response)
}

// HandleGetInternalNotes returns internal notes for a ticket
func HandleGetInternalNotes(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Check permissions
	userRole, _ := c.Get("user_role")
	if userRole == "customer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to view internal notes"})
		return
	}

	// Parse filters
	category := c.Query("category")
	priorityStr := c.Query("priority")
	search := c.Query("search")
	hasMentionsStr := c.Query("has_mentions")

	notes := []InternalNote{}
	
	if ticketNotes, exists := internalNotes[ticketID]; exists {
		for _, note := range ticketNotes {
			// Apply filters
			if category != "" && note.Category != category {
				continue
			}
			
			if priorityStr == "true" && !note.IsPriority {
				continue
			}
			
			if search != "" && !strings.Contains(strings.ToLower(note.Content), strings.ToLower(search)) {
				continue
			}
			
			if hasMentionsStr == "true" && !note.HasMentions {
				continue
			}
			
			notes = append(notes, *note)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"notes": notes,
		"total": len(notes),
	})
}

// HandleUpdateInternalNote updates an existing internal note
func HandleUpdateInternalNote(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	noteIDStr := c.Param("note_id")
	noteID, err := strconv.Atoi(noteIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return
	}

	// Find note
	ticketNotes, exists := internalNotes[ticketID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Internal note not found"})
		return
	}

	note, exists := ticketNotes[noteID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Internal note not found"})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	// Check permissions
	if userRole != "admin" && note.AuthorID != userID.(int) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only edit your own notes"})
		return
	}

	var req struct {
		Content    string `json:"content"`
		IsPriority bool   `json:"is_priority"`
		Category   string `json:"category"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save to history before updating
	if req.Content != "" && req.Content != note.Content {
		history := NoteHistory{
			ID:       len(noteHistories[noteID]) + 1,
			NoteID:   noteID,
			Content:  note.Content,
			EditedBy: userID.(int),
			EditedAt: time.Now(),
			Version:  len(noteHistories[noteID]) + 1,
		}
		noteHistories[noteID] = append(noteHistories[noteID], history)
		
		note.Content = req.Content
		note.FormattedContent = RenderMarkdown(req.Content)
		note.IsEdited = true
		now := time.Now()
		note.EditedAt = &now
	}

	if req.Category != "" {
		note.Category = req.Category
	}
	
	note.IsPriority = req.IsPriority
	note.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"message": "Internal note updated successfully",
		"note":    note,
	})
}

// HandleDeleteInternalNote deletes an internal note
func HandleDeleteInternalNote(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	noteIDStr := c.Param("note_id")
	noteID, err := strconv.Atoi(noteIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return
	}

	// Find note
	ticketNotes, exists := internalNotes[ticketID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Internal note not found"})
		return
	}

	note, exists := ticketNotes[noteID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Internal note not found"})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	// Check permissions
	if userRole != "admin" && note.AuthorID != userID.(int) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own notes"})
		return
	}

	delete(ticketNotes, noteID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Internal note deleted successfully",
	})
}

// handleGetInternalNoteHistory returns edit history for a note
func handleGetInternalNoteHistory(c *gin.Context) {
	noteIDStr := c.Param("note_id")
	noteID, err := strconv.Atoi(noteIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return
	}

	history := noteHistories[noteID]
	if history == nil {
		history = []NoteHistory{}
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   len(history),
	})
}

// handleGetInternalNoteStats returns statistics for internal notes
func handleGetInternalNoteStats(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Calculate statistics
	totalNotes := 0
	priorityNotes := 0
	notesWithMentions := 0
	notesByCategory := make(map[string]int)
	notesByAuthor := make(map[string]int)
	totalLength := 0

	if ticketNotes, exists := internalNotes[ticketID]; exists {
		for _, note := range ticketNotes {
			totalNotes++
			totalLength += len(note.Content)
			
			if note.IsPriority {
				priorityNotes++
			}
			if note.HasMentions {
				notesWithMentions++
			}
			if note.Category != "" {
				notesByCategory[note.Category]++
			}
			notesByAuthor[note.AuthorName]++
		}
	}

	avgLength := 0
	if totalNotes > 0 {
		avgLength = totalLength / totalNotes
	}

	c.JSON(http.StatusOK, gin.H{
		"statistics": gin.H{
			"total_notes":          totalNotes,
			"priority_notes":       priorityNotes,
			"notes_with_mentions":  notesWithMentions,
			"notes_by_category":    notesByCategory,
			"notes_by_author":      notesByAuthor,
			"average_note_length":  avgLength,
		},
	})
}

// handleExportInternalNotes exports internal notes in various formats
func handleExportInternalNotes(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	format := c.Query("format")
	if format == "" {
		format = "json"
	}

	// Collect notes
	notes := []InternalNote{}
	if ticketNotes, exists := internalNotes[ticketID]; exists {
		for _, note := range ticketNotes {
			notes = append(notes, *note)
		}
	}

	timestamp := time.Now().Format("20060102-150405")

	switch format {
	case "json":
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"internal-notes-ticket-%d-%s.json\"", ticketID, timestamp))
		c.JSON(http.StatusOK, gin.H{
			"ticket_id":   ticketID,
			"notes":       notes,
			"exported_at": time.Now(),
		})

	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"internal-notes-ticket-%d-%s.csv\"", ticketID, timestamp))
		
		writer := csv.NewWriter(c.Writer)
		writer.Write([]string{"ID", "Author", "Content", "Category", "Priority", "Created"})
		
		for _, note := range notes {
			writer.Write([]string{
				strconv.Itoa(note.ID),
				note.AuthorName,
				note.Content,
				note.Category,
				strconv.FormatBool(note.IsPriority),
				note.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		writer.Flush()

	case "pdf":
		// Mock PDF export
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"internal-notes-ticket-%d-%s.pdf\"", ticketID, timestamp))
		c.String(http.StatusOK, "PDF content would be generated here")

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Supported: json, csv, pdf"})
	}
}

// handleCreateNoteFromTemplate creates a note from a template
func handleCreateNoteFromTemplate(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Check if ticket exists
	if !mockTickets[ticketID] {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	var req struct {
		TemplateID int               `json:"template_id"`
		Variables  map[string]string `json:"variables"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Mock template content
	templateContent := "Issue Type: {{issue_type}}\nRoot Cause: {{root_cause}}\nResolution: Pending investigation"
	
	// Replace variables
	for key, value := range req.Variables {
		placeholder := "{{" + key + "}}"
		templateContent = strings.ReplaceAll(templateContent, placeholder, value)
	}

	userID, _ := c.Get("user_id")
	userName, exists := c.Get("user_name")
	if !exists {
		userName = "Test User"
	}

	// Create note
	note := &InternalNote{
		ID:              nextNoteID,
		TicketID:        ticketID,
		Content:         templateContent,
		AuthorID:        userID.(int),
		AuthorName:      userName.(string),
		Visibility:      "internal",
		CustomerVisible: false,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Initialize ticket notes map if needed
	if internalNotes[ticketID] == nil {
		internalNotes[ticketID] = make(map[int]*InternalNote)
	}
	
	internalNotes[ticketID][nextNoteID] = note
	nextNoteID++

	c.JSON(http.StatusCreated, gin.H{
		"message": "Internal note created from template",
		"note_id": note.ID,
		"note":    note,
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