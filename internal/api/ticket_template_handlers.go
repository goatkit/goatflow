package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/history"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// Get or create singleton template repository and service
var (
	templateRepo    repository.TicketTemplateRepository
	templateService *service.TicketTemplateService
)

func GetTemplateService() *service.TicketTemplateService {
	if templateService == nil {
		templateRepo = repository.NewMemoryTicketTemplateRepository()
		templateService = service.NewTicketTemplateService(templateRepo, GetTicketService())
	}
	return templateService
}

// Template management page
func handleTemplatesPage(c *gin.Context) {
	templateService := GetTemplateService()

	templates, err := templateService.GetActiveTemplates(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load templates: %v", err)
		return
	}

	categories, err := templateService.GetCategories(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load categories: %v", err)
		return
	}

	tmpl, err := loadTemplate(
		"templates/layouts/base.html",
		"templates/components/guru_meditation.html",
		"templates/pages/tickets/templates.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "templates.html", gin.H{
		"Title":      "Ticket Templates - GOTRS",
		"Templates":  templates,
		"Categories": categories,
		"User":       getUserFromContext(c),
		"ActivePage": "templates",
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// API: Get all active templates
func handleGetTemplates(c *gin.Context) {
	templateService := GetTemplateService()

	// Check for category filter
	category := c.Query("category")

	var templates []models.TicketTemplate
	var err error

	if category != "" {
		templates, err = templateService.GetTemplatesByCategory(c.Request.Context(), category)
	} else {
		templates, err = templateService.GetActiveTemplates(c.Request.Context())
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    templates,
	})
}

// API: Get template by ID
func handleGetTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid template ID",
		})
		return
	}

	templateService := GetTemplateService()
	template, err := templateService.GetTemplate(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Template not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    template,
	})
}

// API: Create new template
func handleCreateTemplate(c *gin.Context) {
	// Check admin permission
	userRole := c.GetString("user_role")
	if userRole != "Admin" && userRole != "Agent" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Permission denied",
		})
		return
	}

	var template models.TicketTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Set creator
	template.CreatedBy = c.GetInt("user_id")
	if template.CreatedBy == 0 {
		template.CreatedBy = 1 // Default to system user
	}
	template.Active = true

	templateService := GetTemplateService()
	if err := templateService.CreateTemplate(c.Request.Context(), &template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    template,
		"message": "Template created successfully",
	})
}

// API: Update template
func handleUpdateTemplate(c *gin.Context) {
	// Check admin permission
	userRole := c.GetString("user_role")
	if userRole != "Admin" && userRole != "Agent" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Permission denied",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid template ID",
		})
		return
	}

	var template models.TicketTemplate
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	template.ID = uint(id)
	template.UpdatedBy = c.GetInt("user_id")
	if template.UpdatedBy == 0 {
		template.UpdatedBy = 1
	}

	templateService := GetTemplateService()
	if err := templateService.UpdateTemplate(c.Request.Context(), &template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    template,
		"message": "Template updated successfully",
	})
}

// API: Delete template
func handleDeleteTemplate(c *gin.Context) {
	// Check admin permission
	userRole := c.GetString("user_role")
	if userRole != "Admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Admin access required",
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid template ID",
		})
		return
	}

	templateService := GetTemplateService()
	if err := templateService.DeleteTemplate(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Template deleted successfully",
	})
}

// API: Search templates
func handleSearchTemplates(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Search query is required",
		})
		return
	}

	templateService := GetTemplateService()
	templates, err := templateService.SearchTemplates(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    templates,
		"query":   query,
	})
}

// API: Get template categories
func handleGetTemplateCategories(c *gin.Context) {
	templateService := GetTemplateService()
	categories, err := templateService.GetCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    categories,
	})
}

// API: Get popular templates
func handleGetPopularTemplates(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20 // Cap at 20
	}

	templateService := GetTemplateService()
	templates, err := templateService.GetPopularTemplates(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    templates,
	})
}

// API: Apply template to create a ticket
func handleApplyTemplate(c *gin.Context) {
	var application models.TemplateApplication
	if err := c.ShouldBindJSON(&application); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	templateService := GetTemplateService()
	ticket, err := templateService.ApplyTemplate(c.Request.Context(), &application)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    ticket,
		"message": "Ticket created from template successfully",
	})
}

// HTMX: Template selection modal
func handleTemplateSelectionModal(c *gin.Context) {
	templateService := GetTemplateService()

	templates, err := templateService.GetActiveTemplates(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load templates")
		return
	}

	categories, err := templateService.GetCategories(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load categories")
		return
	}

	tmpl, err := loadTemplate("templates/components/template_selector.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "template_selector.html", gin.H{
		"Templates":  templates,
		"Categories": categories,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// HTMX: Load template into form
func handleLoadTemplateIntoForm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid template ID")
		return
	}

	templateService := GetTemplateService()
	template, err := templateService.GetTemplate(c.Request.Context(), uint(id))
	if err != nil {
		c.String(http.StatusNotFound, "Template not found")
		return
	}

	// Return JavaScript to populate the form
	c.Header("Content-Type", "text/javascript")
	c.String(http.StatusOK, `
		document.getElementById('subject').value = %q;
		document.getElementById('body').value = %q;
		document.getElementById('priority').value = %q;
		document.getElementById('queue_id').value = %q;
		document.getElementById('type_id').value = %q;
		
		// Show variables dialog if template has variables
		%s
		
		// Close template modal
		const modal = document.getElementById('template-modal');
		if (modal) {
			modal.style.display = 'none';
		}
		
		// Show success message
		const messageDiv = document.getElementById('form-messages');
		if (messageDiv) {
			messageDiv.innerHTML = '<div class="rounded-md bg-green-50 p-4 mb-4"><p class="text-sm text-green-800">Template loaded successfully</p></div>';
			setTimeout(() => messageDiv.innerHTML = '', 3000);
		}
	`,
		template.Subject,
		template.Body,
		template.Priority,
		strconv.Itoa(template.QueueID),
		strconv.Itoa(template.TypeID),
		generateVariableJS(template.Variables),
	)
}

// HTMX: Ticket history fragment for tab panel
func HandleTicketHistoryFragment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ticket id")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewTicketRepository(db)
	entries, err := repo.GetTicketHistoryEntries(uint(id), 25)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load history")
		return
	}

	historyRows := make([]map[string]string, 0, len(entries))
	for _, entry := range entries {
		actor := strings.TrimSpace(entry.CreatorFullName)
		if actor == "" {
			actor = entry.CreatorLogin
		}
		if actor == "" {
			actor = "System"
		}

		when := entry.CreatedAt.In(time.Local).Format("02 Jan 2006 15:04")

		displayName := history.NormalizeHistoryName(entry)
		event := entry.HistoryType
		if displayName != "" && !strings.EqualFold(displayName, event) {
			if event != "" {
				event = fmt.Sprintf("%s — %s", event, displayName)
			} else {
				event = displayName
			}
		}
		if event == "" {
			event = "Ticket update"
		}

		metaParts := make([]string, 0, 4)
		if entry.QueueName != "" {
			metaParts = append(metaParts, entry.QueueName)
		}
		if entry.StateName != "" {
			metaParts = append(metaParts, entry.StateName)
		}
		if entry.PriorityName != "" {
			metaParts = append(metaParts, fmt.Sprintf("Priority %s", entry.PriorityName))
		}
		if entry.ArticleSubject != "" {
			metaParts = append(metaParts, entry.ArticleSubject)
		}
		meta := strings.Join(metaParts, " • ")
		if meta == "" {
			meta = fmt.Sprintf("#%d", entry.ID)
		}

		historyRows = append(historyRows, map[string]string{
			"event": event,
			"by":    actor,
			"when":  when,
			"meta":  meta,
		})
	}

	renderer := GetPongo2Renderer()
	if renderer == nil {
		c.String(http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	renderer.HTML(c, http.StatusOK, "partials/tickets/ticket_history.pongo2", pongo2.Context{
		"history": historyRows,
	})
}

// HTMX: Ticket links fragment for tab panel
func HandleTicketLinksFragment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ticket id")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database unavailable")
		return
	}

	repo := repository.NewTicketRepository(db)
	entries, err := repo.GetTicketLinks(uint(id), 25)
	if err != nil {
		log.Printf("ticket links load error (ticketID=%d): %v", id, err)
		c.String(http.StatusInternalServerError, "Failed to load links")
		return
	}

	links := make([]map[string]string, 0, len(entries))
	for _, entry := range entries {
		titleParts := make([]string, 0, 2)
		if entry.RelatedTicketTN != "" {
			titleParts = append(titleParts, entry.RelatedTicketTN)
		}
		if entry.RelatedTicketTitle != "" {
			titleParts = append(titleParts, entry.RelatedTicketTitle)
		}
		title := strings.Join(titleParts, " — ")
		if title == "" {
			title = fmt.Sprintf("Ticket #%d", entry.RelatedTicketID)
		}

		typeLabel := entry.LinkType
		if typeLabel == "" {
			typeLabel = "related"
		}
		if entry.Direction != "" {
			dirLabel := entry.Direction
			if len(dirLabel) > 0 {
				dirLabel = strings.ToUpper(dirLabel[:1]) + dirLabel[1:]
			}
			typeLabel = fmt.Sprintf("%s (%s)", typeLabel, dirLabel)
		}

		actor := strings.TrimSpace(entry.CreatorFullName)
		if actor == "" {
			actor = entry.CreatorLogin
		}
		if actor == "" {
			actor = "System"
		}

		noteParts := make([]string, 0, 2)
		if entry.LinkState != "" {
			noteParts = append(noteParts, entry.LinkState)
		}
		noteParts = append(noteParts, fmt.Sprintf("%s on %s", actor, entry.CreatedAt.In(time.Local).Format("02 Jan 2006 15:04")))
		note := strings.Join(noteParts, " • ")

		links = append(links, map[string]string{
			"href":  fmt.Sprintf("/agent/tickets/%d", entry.RelatedTicketID),
			"title": title,
			"type":  typeLabel,
			"note":  note,
		})
	}

	renderer := GetPongo2Renderer()
	if renderer == nil {
		c.String(http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	renderer.HTML(c, http.StatusOK, "partials/tickets/ticket_links.pongo2", pongo2.Context{
		"links": links,
	})
}

// Helper to generate JavaScript for template variables
func generateVariableJS(variables []models.TemplateVariable) string {
	if len(variables) == 0 {
		return "// No variables"
	}

	js := `
	// Show variable input dialog
	const variables = ` + toJSON(variables) + `;
	if (variables.length > 0) {
		// TODO: Implement variable input dialog
		console.log('Template has variables:', variables);
	}
	`
	return js
}
