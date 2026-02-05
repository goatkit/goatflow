package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// InternalNoteHandlers manages internal note API endpoints.
type InternalNoteHandlers struct {
	service *service.InternalNoteService
}

// getAuthUserInfo extracts user info from the gin context (set by auth middleware).
// Returns userID, userName, userEmail with sensible defaults if not authenticated.
func getAuthUserInfo(c *gin.Context) (uint, string, string) {
	userID := uint(1)   // Default system user
	userName := "System"
	userEmail := "system@localhost"

	// Try to get full user object first
	if userCtx, ok := c.Get("user"); ok {
		if user, ok := userCtx.(*models.User); ok && user.ID > 0 {
			userID = uint(user.ID)
			if user.Login != "" {
				userName = user.Login
			}
			if user.Email != "" {
				userEmail = user.Email
			}
			return userID, userName, userEmail
		}
	}

	// Fall back to individual context values
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = uint(idInt)
		}
	}
	if email, exists := c.Get("user_email"); exists {
		if emailStr, ok := email.(string); ok && emailStr != "" {
			userEmail = emailStr
			userName = emailStr // Use email as name fallback
		}
	}

	return userID, userName, userEmail
}

// NewInternalNoteHandlers creates a new internal note handlers instance.
func NewInternalNoteHandlers() *InternalNoteHandlers {
	repo := repository.NewMemoryInternalNoteRepository()
	srv := service.NewInternalNoteService(repo)

	// Initialize with some default categories and templates
	initializeDefaultNoteData(srv)

	return &InternalNoteHandlers{
		service: srv,
	}
}

// initializeDefaultNoteData creates default categories and templates.
func initializeDefaultNoteData(srv *service.InternalNoteService) {
	ctx := context.Background()

	// Default templates
	templates := []models.NoteTemplate{
		{
			Name:        "Investigation Started",
			Content:     "Investigation started at {{time}}.\n\nInitial findings:\n{{findings}}\n\nNext steps:\n{{next_steps}}",
			Category:    "Investigation",
			Tags:        []string{"investigation", "start"},
			IsImportant: false,
		},
		{
			Name:        "Resolution Applied",
			Content:     "Resolution applied:\n{{resolution}}\n\nVerification:\n{{verification}}\n\nCustomer notified: {{notified}}",
			Category:    "Resolution",
			Tags:        []string{"resolution", "solved"},
			IsImportant: true,
		},
		{
			Name:        "Escalation Note",
			Content:     "Ticket escalated to: {{team}}\n\nReason: {{reason}}\n\nExpected response time: {{response_time}}",
			Category:    "Escalation",
			Tags:        []string{"escalation", "priority"},
			IsImportant: true,
		},
		{
			Name:        "Customer Contact",
			Content:     "Customer contacted via: {{method}}\n\nSummary: {{summary}}\n\nFollow-up required: {{followup}}",
			Category:    "Communication",
			Tags:        []string{"customer", "contact"},
			IsImportant: false,
		},
		{
			Name:        "Internal Review",
			Content:     "Internal review completed.\n\nFindings:\n{{findings}}\n\nRecommendations:\n{{recommendations}}",
			Category:    "Review",
			Tags:        []string{"review", "internal"},
			IsImportant: false,
		},
	}

	for _, tmpl := range templates {
		_ = srv.CreateTemplate(ctx, &tmpl) //nolint:errcheck // Best-effort seed
	}
}

// CreateNote creates a new internal note.
func (h *InternalNoteHandlers) CreateNote(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var note models.InternalNote
	if err := c.ShouldBindJSON(&note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	note.TicketID = uint(ticketID)

	// Get author info from auth context
	note.AuthorID, note.AuthorName, note.AuthorEmail = getAuthUserInfo(c)

	if err := h.service.CreateNote(c.Request.Context(), &note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, note)
}

// CreateNoteFromTemplate creates a note from a template.
func (h *InternalNoteHandlers) CreateNoteFromTemplate(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var req struct {
		TemplateID uint              `json:"template_id" binding:"required"`
		Variables  map[string]string `json:"variables"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get author ID from auth context
	authorID, _, _ := getAuthUserInfo(c)

	note, err := h.service.CreateNoteFromTemplate(
		c.Request.Context(),
		uint(ticketID),
		req.TemplateID,
		req.Variables,
		authorID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, note)
}

// GetNotes retrieves all notes for a ticket.
func (h *InternalNoteHandlers) GetNotes(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	notes, err := h.service.GetNotesByTicket(c.Request.Context(), uint(ticketID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, notes)
}

// GetPinnedNotes retrieves pinned notes for a ticket.
func (h *InternalNoteHandlers) GetPinnedNotes(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	notes, err := h.service.GetPinnedNotes(c.Request.Context(), uint(ticketID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, notes)
}

// GetImportantNotes retrieves important notes for a ticket.
func (h *InternalNoteHandlers) GetImportantNotes(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	notes, err := h.service.GetImportantNotes(c.Request.Context(), uint(ticketID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, notes)
}

// GetNoteByID retrieves a specific note.
func (h *InternalNoteHandlers) GetNoteByID(c *gin.Context) {
	noteID, err := strconv.ParseUint(c.Param("note_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return
	}

	note, err := h.service.GetNote(c.Request.Context(), uint(noteID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
		return
	}

	// Verify the note belongs to the ticket
	ticketID, _ := strconv.ParseUint(c.Param("id"), 10, 32) //nolint:errcheck // Defaults to 0
	if note.TicketID != uint(ticketID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Note does not belong to this ticket"})
		return
	}

	c.JSON(http.StatusOK, note)
}

// UpdateNote updates an existing note.
func (h *InternalNoteHandlers) UpdateNote(c *gin.Context) {
	noteID, err := strconv.ParseUint(c.Param("note_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return
	}

	var req struct {
		Content    string `json:"content" binding:"required"`
		EditReason string `json:"edit_reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing note
	note, err := h.service.GetNote(c.Request.Context(), uint(noteID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
		return
	}

	// Verify the note belongs to the ticket
	ticketID, _ := strconv.ParseUint(c.Param("id"), 10, 32) //nolint:errcheck // Defaults to 0
	if note.TicketID != uint(ticketID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Note does not belong to this ticket"})
		return
	}

	// Update content
	note.Content = req.Content
	editorID, _, _ := getAuthUserInfo(c)
	note.EditedBy = editorID

	if err := h.service.UpdateNote(c.Request.Context(), note, req.EditReason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, note)
}

// DeleteNote deletes a note.
func (h *InternalNoteHandlers) DeleteNote(c *gin.Context) {
	noteID, err := strconv.ParseUint(c.Param("note_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return
	}

	// Get current user from auth context
	deletedBy, _, _ := getAuthUserInfo(c)

	if err := h.service.DeleteNote(c.Request.Context(), uint(noteID), deletedBy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Note deleted successfully"})
}

// PinNote pins or unpins a note.
func (h *InternalNoteHandlers) PinNote(c *gin.Context) {
	noteID, err := strconv.ParseUint(c.Param("note_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return
	}

	// Get user ID from auth context
	userID, _, _ := getAuthUserInfo(c)

	if err := h.service.PinNote(c.Request.Context(), uint(noteID), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Note pin status updated"})
}

// MarkImportant marks or unmarks a note as important.
func (h *InternalNoteHandlers) MarkImportant(c *gin.Context) {
	noteID, err := strconv.ParseUint(c.Param("note_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return
	}

	var req struct {
		Important bool `json:"important"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from auth context
	userID, _, _ := getAuthUserInfo(c)

	if err := h.service.MarkImportant(c.Request.Context(), uint(noteID), req.Important, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Note importance updated"})
}

// SearchNotes searches for notes.
func (h *InternalNoteHandlers) SearchNotes(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	filter := &models.NoteFilter{
		TicketID:    uint(ticketID),
		SearchQuery: c.Query("q"),
		Category:    c.Query("category"),
	}

	// Parse boolean filters
	if important := c.Query("important"); important != "" {
		b := important == "true"
		filter.IsImportant = &b
	}

	if pinned := c.Query("pinned"); pinned != "" {
		b := pinned == "true"
		filter.IsPinned = &b
	}

	// Parse pagination
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}

	notes, err := h.service.SearchNotes(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, notes)
}

// GetEditHistory retrieves edit history for a note.
func (h *InternalNoteHandlers) GetEditHistory(c *gin.Context) {
	noteID, err := strconv.ParseUint(c.Param("note_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID"})
		return
	}

	history, err := h.service.GetEditHistory(c.Request.Context(), uint(noteID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// GetNoteStatistics retrieves statistics for notes.
func (h *InternalNoteHandlers) GetNoteStatistics(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	stats, err := h.service.GetTicketNoteSummary(c.Request.Context(), uint(ticketID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetRecentActivity retrieves recent activity.
func (h *InternalNoteHandlers) GetRecentActivity(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	activities, err := h.service.GetRecentActivity(c.Request.Context(), uint(ticketID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, activities)
}

// ExportNotes exports notes in various formats.
func (h *InternalNoteHandlers) ExportNotes(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	format := c.Query("format")
	if format == "" {
		format = "json"
	}

	// Get user email from auth context
	_, _, exportedBy := getAuthUserInfo(c)

	data, err := h.service.ExportNotes(c.Request.Context(), uint(ticketID), format, exportedBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	contentType := "application/json"
	filename := "notes.json"

	if format == "csv" {
		contentType = "text/csv"
		filename = "notes.csv"
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, data)
}

// GetTemplates retrieves all note templates.
func (h *InternalNoteHandlers) GetTemplates(c *gin.Context) {
	templates, err := h.service.GetTemplates(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, templates)
}

// CreateTemplate creates a new template.
func (h *InternalNoteHandlers) CreateTemplate(c *gin.Context) {
	var template models.NoteTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get creator from auth context
	creatorID, _, _ := getAuthUserInfo(c)
	template.CreatedBy = creatorID

	if err := h.service.CreateTemplate(c.Request.Context(), &template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, template)
}

// UpdateTemplate updates a template.
func (h *InternalNoteHandlers) UpdateTemplate(c *gin.Context) {
	templateID, err := strconv.ParseUint(c.Param("template_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	var template models.NoteTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template.ID = uint(templateID)

	if err := h.service.UpdateTemplate(c.Request.Context(), &template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, template)
}

// DeleteTemplate deletes a template.
func (h *InternalNoteHandlers) DeleteTemplate(c *gin.Context) {
	templateID, err := strconv.ParseUint(c.Param("template_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	if err := h.service.DeleteTemplate(c.Request.Context(), uint(templateID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted successfully"})
}

// GetCategories retrieves all note categories.
func (h *InternalNoteHandlers) GetCategories(c *gin.Context) {
	categories, err := h.service.GetCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, categories)
}
