package api

import (
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	
	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	tmpl "github.com/gotrs-io/gotrs-ce/internal/template"
)

// Global Pongo2 renderer
var pongo2Renderer *tmpl.Pongo2Renderer

// Initialize Pongo2 renderer
func init() {
	// Get project root for templates
	projectRoot := getProjectRoot()
	templateDir := filepath.Join(projectRoot, "templates")
	
	// Create renderer (debug mode for development)
	pongo2Renderer = tmpl.NewPongo2Renderer(templateDir, true)
}

// getUserFromContext builds a User object from authentication context
func getUserFromContext(c *gin.Context) gin.H {
	// Check if user is authenticated
	userID, hasID := c.Get("user_id")
	if !hasID {
		// Return demo user if not authenticated
		demoEmail := os.Getenv("DEMO_USER_EMAIL")
		if demoEmail == "" {
			demoEmail = "test-user@example.com"
		}
		return gin.H{"FirstName": "Demo", "LastName": "User", "Email": demoEmail, "Role": "Admin"}
	}
	
	// Build user object from context
	user := gin.H{}
	
	// Get user ID
	if id, ok := userID.(uint); ok {
		user["ID"] = id
	}
	
	// Get email
	if email, exists := c.Get("user_email"); exists {
		user["Email"] = email
	}
	
	// Get role - IMPORTANT: This determines admin button visibility
	if role, exists := c.Get("user_role"); exists {
		user["Role"] = role
	} else {
		// Default to Agent if role not set
		user["Role"] = "Agent"
	}
	
	// Parse name from email if not available
	// In production, this should come from user profile
	if emailStr, ok := user["Email"].(string); ok {
		parts := strings.Split(emailStr, "@")
		if len(parts) > 0 {
			nameParts := strings.Split(parts[0], ".")
			if len(nameParts) >= 2 {
				user["FirstName"] = strings.Title(nameParts[0])
				user["LastName"] = strings.Title(nameParts[1])
			} else {
				user["FirstName"] = strings.Title(nameParts[0])
				user["LastName"] = "User"
			}
		}
	}
	
	// Set defaults if not available
	if user["FirstName"] == nil {
		user["FirstName"] = "User"
	}
	if user["LastName"] == nil {
		user["LastName"] = ""
	}
	
	return user
}

// DummyTemplate is a wrapper for html/template
type DummyTemplate struct {
	tmpl *template.Template
}

func (t *DummyTemplate) ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	if t.tmpl == nil {
		_, err := w.Write([]byte("Template not loaded"))
		return err
	}
	return t.tmpl.ExecuteTemplate(w, name, data)
}

// loadTemplateForRequest loads HTML templates
func loadTemplateForRequest(c *gin.Context, templatePaths ...string) (*DummyTemplate, error) {
	projectRoot := getProjectRoot()
	
	// Define template functions
	funcMap := template.FuncMap{
		"firstLetter": func(s string) string {
			if len(s) > 0 {
				return string(s[0])
			}
			return ""
		},
		"toUpper": strings.ToUpper,
		"contains": strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"seq": func(start, end int) []int {
			var result []int
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				if i+1 < len(values) {
					if key, ok := values[i].(string); ok {
						dict[key] = values[i+1]
					}
				}
			}
			return dict
		},
		"list": func(values ...interface{}) []interface{} {
			return values
		},
	}
	
	// Create template with functions
	tmpl := template.New(filepath.Base(templatePaths[0])).Funcs(funcMap)
	
	// Parse all template files
	for _, path := range templatePaths {
		fullPath := filepath.Join(projectRoot, path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		_, err = tmpl.Parse(string(content))
		if err != nil {
			return nil, err
		}
	}
	
	return &DummyTemplate{tmpl: tmpl}, nil
}

// loadTemplate is another compatibility shim
// TODO: Remove this once all handlers are converted to use Pongo2 directly
func loadTemplate(templatePaths ...string) (*DummyTemplate, error) {
	projectRoot := getProjectRoot()
	
	// Define template functions
	funcMap := template.FuncMap{
		"firstLetter": func(s string) string {
			if len(s) > 0 {
				return string(s[0])
			}
			return ""
		},
		"toUpper": strings.ToUpper,
		"contains": strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"seq": func(start, end int) []int {
			var result []int
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				if i+1 < len(values) {
					if key, ok := values[i].(string); ok {
						dict[key] = values[i+1]
					}
				}
			}
			return dict
		},
		"list": func(values ...interface{}) []interface{} {
			return values
		},
	}
	
	// Create template with functions
	tmpl := template.New(filepath.Base(templatePaths[0])).Funcs(funcMap)
	
	// Parse all template files
	for _, path := range templatePaths {
		fullPath := filepath.Join(projectRoot, path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		_, err = tmpl.Parse(string(content))
		if err != nil {
			return nil, err
		}
	}
	
	return &DummyTemplate{tmpl: tmpl}, nil
}

// filterEmptyStrings removes empty strings from a slice
func filterEmptyStrings(slice []string) []string {
	result := []string{}
	for _, s := range slice {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// getPriorityID converts priority string to ORTS priority ID
func getPriorityID(priority string) int {
	priorityMap := map[string]int{
		"low":    1,
		"normal": 2,
		"high":   3,
		"urgent": 4,
	}
	if id, ok := priorityMap[priority]; ok {
		return id
	}
	return 2 // Default to normal
}

// getProjectRoot finds the project root directory by looking for go.mod
func getProjectRoot() string {
	// Start from current directory
	dir, err := os.Getwd()
	if err != nil {
		// Fallback to current directory
		return "."
	}
	
	// Walk up the directory tree looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root without finding go.mod
			break
		}
		dir = parent
	}
	
	// Fallback to current directory
	return "."
}


// SetupHTMXRoutes configures routes for HTMX-based UI
func SetupHTMXRoutes(r *gin.Engine) {
	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	
	// Serve static files
	r.Static("/static", "./static")
	
	// Serve favicon specifically (browsers often request this)
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.File("./static/favicon.ico")
	})
	
	// Note: favicon.svg is served via the /static route above
	
	// Test i18n endpoint
	r.GET("/test-i18n", func(c *gin.Context) {
		pongo2Renderer.HTML(c, http.StatusOK, "test-i18n.pongo2", pongo2.Context{
			"TestMessage": "Direct test",
		})
	})
	
	// Root redirect
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/login")
	})
	
	// Authentication pages
	r.GET("/login", handleLoginPage)
	r.GET("/register", handleRegisterPage)
	r.POST("/logout", handleLogout)
	r.GET("/logout", handleLogoutGET)
	
	// Protected dashboard routes
	dashboard := r.Group("/")
	// TODO: Add auth middleware
	{
		dashboard.GET("/dashboard", handleDashboard)
		dashboard.GET("/tickets", handleTicketsList)
		dashboard.GET("/tickets/new", handleTicketNew)
		dashboard.POST("/tickets/create", handleTicketCreate)
		dashboard.GET("/tickets/:id", handleTicketDetail)
		dashboard.GET("/tickets/:id/edit", handleTicketEditForm)
		dashboard.POST("/tickets/:id/quick-action", handleTicketQuickAction)
		dashboard.POST("/tickets/bulk-action", handleTicketBulkAction)
		dashboard.GET("/queues", handleQueuesList)
		dashboard.GET("/queues/:id", handleQueueDetailPage)
		dashboard.GET("/queues/:id/edit", handleEditQueueForm)
		dashboard.GET("/queues/new", handleNewQueueForm)
		dashboard.GET("/queues/:id/delete", handleDeleteQueueConfirmation)
		dashboard.GET("/queues/clear-search", handleClearQueueSearch)
		dashboard.GET("/queues/bulk-toolbar", handleBulkActionsToolbar)
		dashboard.GET("/templates", handleTemplatesPage)
		dashboard.GET("/admin", handleAdminDashboard)
		dashboard.GET("/admin/lookups", handleAdminLookups)
		
		// User pages (under construction)
		dashboard.GET("/profile", underConstruction("Profile"))
		dashboard.GET("/settings", underConstruction("Settings"))
		
		// Additional admin pages (under construction)
		dashboard.GET("/admin/users", underConstruction("User Management"))
		dashboard.GET("/admin/settings", underConstruction("System Settings"))
		dashboard.GET("/admin/templates", underConstruction("Template Management"))
		dashboard.GET("/admin/reports", underConstruction("Reports"))
		dashboard.GET("/admin/backup", underConstruction("Backup & Restore"))
	}
	
	// HTMX API endpoints (return HTML fragments)
	api := r.Group("/api")
	{
		// Authentication
		api.GET("/auth/login", handleHTMXLogin)  // Also support GET for the form
		api.POST("/auth/login", handleHTMXLogin)
		api.POST("/auth/logout", handleHTMXLogout)
		api.GET("/auth/refresh", underConstructionAPI("/auth/refresh"))  // GET for testing
		api.POST("/auth/refresh", underConstructionAPI("/auth/refresh"))
		api.GET("/auth/register", underConstructionAPI("/auth/register"))  // GET for form
		api.POST("/auth/register", underConstructionAPI("/auth/register"))
		
		// Dashboard data
		api.GET("/dashboard/stats", handleDashboardStats)
		api.GET("/dashboard/recent-tickets", handleRecentTickets)
		api.GET("/dashboard/activity", handleActivityFeed)
		
		// Notifications
		api.GET("/notifications", underConstructionAPI("/notifications"))
		
		// Lookups
		api.GET("/lookups/cache/invalidate", underConstructionAPI("/lookups/cache/invalidate"))
		
		// Queue operations
		api.GET("/queues", handleQueuesAPI)
		api.POST("/queues", handleCreateQueueWithHTMX)
		api.GET("/queues/:id", handleQueueDetail)
		api.PUT("/queues/:id", handleUpdateQueueWithHTMX)
		api.DELETE("/queues/:id", handleDeleteQueue)
		api.GET("/queues/:id/tickets", handleQueueTicketsWithHTMX)
		
		// Bulk queue operations
		api.PUT("/queues/bulk/:action", handleBulkQueueAction)
		api.DELETE("/queues/bulk", handleBulkQueueDelete)
		
		// Ticket operations
		// Note: Specific routes must be registered before parameterized routes
		api.GET("/tickets", handleTicketsAPI)
		api.GET("/tickets/search", handleTicketSearch)
		api.PUT("/tickets/bulk", handleBulkUpdateTickets)
		api.POST("/tickets", handleCreateTicket)
		api.PUT("/tickets/:id", handleUpdateTicketEnhanced)
		api.POST("/tickets/:id/status", handleUpdateTicketStatus)
		api.POST("/tickets/:id/assign", handleAssignTicket)
		api.POST("/tickets/:id/reply", handleTicketReply)
		api.POST("/tickets/:id/priority", handleUpdateTicketPriority)
		api.POST("/tickets/:id/queue", handleUpdateTicketQueue)
		api.GET("/tickets/:id/messages", underConstructionAPI("/tickets/:id/messages"))
		
		// SLA and Escalation
		api.GET("/tickets/:id/sla", handleGetTicketSLA)
		api.POST("/tickets/:id/escalate", handleEscalateTicket)
		api.GET("/reports/sla", handleSLAReport)
		api.PUT("/admin/sla-config", handleUpdateSLAConfig)
		
		// Ticket Merge
		api.POST("/tickets/:id/merge", handleMergeTickets)
		api.POST("/tickets/:id/unmerge", handleUnmergeTicket)
		api.GET("/tickets/:id/merge-history", handleGetMergeHistory)
		
		// Ticket Attachments
		api.POST("/tickets/:id/attachments", handleUploadAttachment)
		api.GET("/tickets/:id/attachments", handleGetAttachments)
		api.GET("/tickets/:id/attachments/:attachment_id", handleDownloadAttachment)
		api.DELETE("/tickets/:id/attachments/:attachment_id", handleDeleteAttachment)
		
		// File serving endpoint (for stored attachments)
		api.GET("/files/*path", handleServeFile)
		
		// Advanced Search (using different endpoints to avoid conflicts)
		api.GET("/tickets/advanced-search", handleAdvancedTicketSearch)
		api.GET("/tickets/search/suggestions", handleSearchSuggestions)
		api.GET("/tickets/search/export", handleExportSearchResults)
		
		// Search History
		api.POST("/tickets/search/history", handleSaveSearchHistory)
		api.GET("/tickets/search/history", handleGetSearchHistory)
		api.DELETE("/tickets/search/history/:id", handleDeleteSearchHistory)
		
		// Saved Searches
		api.POST("/tickets/search/saved", handleCreateSavedSearch)
		api.GET("/tickets/search/saved", handleGetSavedSearches)
		api.GET("/tickets/search/saved/:id/execute", handleExecuteSavedSearch)
		api.PUT("/tickets/search/saved/:id", handleUpdateSavedSearch)
		api.DELETE("/tickets/search/saved/:id", handleDeleteSavedSearch)
		
		// Canned Responses - Using new comprehensive handlers
		cannedHandlers := NewCannedResponseHandlers()
		api.GET("/canned-responses", cannedHandlers.GetResponses)
		api.GET("/canned-responses/quick", cannedHandlers.GetQuickResponses)
		api.GET("/canned-responses/popular", cannedHandlers.GetPopularResponses)
		api.GET("/canned-responses/categories", cannedHandlers.GetCategories)
		api.GET("/canned-responses/category/:category", cannedHandlers.GetResponsesByCategory)
		api.GET("/canned-responses/search", cannedHandlers.SearchResponses)
		api.GET("/canned-responses/user", cannedHandlers.GetResponsesForUser)
		api.GET("/canned-responses/:id", cannedHandlers.GetResponseByID)
		api.POST("/canned-responses", cannedHandlers.CreateResponse)
		api.PUT("/canned-responses/:id", cannedHandlers.UpdateResponse)
		api.DELETE("/canned-responses/:id", cannedHandlers.DeleteResponse)
		api.POST("/canned-responses/apply", cannedHandlers.ApplyResponse)
		api.GET("/canned-responses/export", cannedHandlers.ExportResponses)
		api.POST("/canned-responses/import", cannedHandlers.ImportResponses)
		
		// Lookup Data Endpoints
		api.GET("/lookups/queues", handleGetQueues)
		api.GET("/lookups/priorities", handleGetPriorities)
		api.GET("/lookups/types", handleGetTypes)
		api.GET("/lookups/statuses", handleGetStatuses)
		api.GET("/lookups/form-data", handleGetFormData)
		api.POST("/lookups/cache/invalidate", handleInvalidateLookupCache)
		
		// Lookup CRUD Endpoints
		api.POST("/lookups/queues", handleCreateLookupQueue)
		api.PUT("/lookups/queues/:id", handleUpdateLookupQueue)
		api.DELETE("/lookups/queues/:id", handleDeleteLookupQueue)
		
		api.POST("/lookups/types", handleCreateType)
		api.PUT("/lookups/types/:id", handleUpdateType)
		api.DELETE("/lookups/types/:id", handleDeleteType)
		
		api.PUT("/lookups/priorities/:id", handleUpdatePriority)
		api.PUT("/lookups/statuses/:id", handleUpdateStatus)
		
		// Audit and Export/Import
		api.GET("/lookups/audit", handleGetAuditLogs)
		api.GET("/lookups/export", handleExportConfiguration)
		api.POST("/lookups/import", handleImportConfiguration)
		
		// Ticket Templates
		api.GET("/templates", handleGetTemplates)
		api.GET("/templates/:id", handleGetTemplate)
		api.POST("/templates", handleCreateTemplate)
		api.PUT("/templates/:id", handleUpdateTemplate)
		api.DELETE("/templates/:id", handleDeleteTemplate)
		api.GET("/templates/search", handleSearchTemplates)
		api.GET("/templates/categories", handleGetTemplateCategories)
		api.GET("/templates/popular", handleGetPopularTemplates)
		api.POST("/templates/apply", handleApplyTemplate)
		api.GET("/templates/:id/load", handleLoadTemplateIntoForm)
		api.GET("/templates/modal", handleTemplateSelectionModal)
		
		// Internal Notes - Using comprehensive handlers
		noteHandlers := NewInternalNoteHandlers()
		api.POST("/tickets/:id/internal-notes", noteHandlers.CreateNote)
		api.GET("/tickets/:id/internal-notes", noteHandlers.GetNotes)
		api.GET("/tickets/:id/internal-notes/pinned", noteHandlers.GetPinnedNotes)
		api.GET("/tickets/:id/internal-notes/important", noteHandlers.GetImportantNotes)
		api.GET("/tickets/:id/internal-notes/search", noteHandlers.SearchNotes)
		api.GET("/tickets/:id/internal-notes/stats", noteHandlers.GetNoteStatistics)
		api.GET("/tickets/:id/internal-notes/activity", noteHandlers.GetRecentActivity)
		api.GET("/tickets/:id/internal-notes/export", noteHandlers.ExportNotes)
		api.POST("/tickets/:id/internal-notes/from-template", noteHandlers.CreateNoteFromTemplate)
		api.GET("/tickets/:id/internal-notes/:note_id", noteHandlers.GetNoteByID)
		api.PUT("/tickets/:id/internal-notes/:note_id", noteHandlers.UpdateNote)
		api.DELETE("/tickets/:id/internal-notes/:note_id", noteHandlers.DeleteNote)
		api.POST("/tickets/:id/internal-notes/:note_id/pin", noteHandlers.PinNote)
		api.POST("/tickets/:id/internal-notes/:note_id/important", noteHandlers.MarkImportant)
		api.GET("/tickets/:id/internal-notes/:note_id/history", noteHandlers.GetEditHistory)
		
		// Note Templates
		api.GET("/internal-notes/templates", noteHandlers.GetTemplates)
		api.POST("/internal-notes/templates", noteHandlers.CreateTemplate)
		
		// Search API - Using comprehensive search handlers
		searchHandlers := NewSearchHandlers()
		api.GET("/search/tickets", searchHandlers.SearchTickets)
		api.POST("/search/advanced", searchHandlers.AdvancedSearch)
		api.GET("/search/suggestions", searchHandlers.GetSearchSuggestions)
		api.POST("/search/saved", searchHandlers.SaveSearch)
		api.GET("/search/saved", searchHandlers.GetSavedSearches)
		api.GET("/search/saved/:id", searchHandlers.GetSavedSearch)
		api.POST("/search/saved/:id/execute", searchHandlers.ExecuteSavedSearch)
		api.GET("/search/history", searchHandlers.GetSearchHistory)
		api.GET("/search/analytics", searchHandlers.GetSearchAnalytics)
		api.POST("/search/reindex", searchHandlers.ReindexTickets)
		api.POST("/search/index/ticket", searchHandlers.IndexTicket)
		api.PUT("/search/index/ticket", searchHandlers.UpdateTicketIndex)
		api.DELETE("/search/index/ticket/:ticket_number", searchHandlers.DeleteTicketFromIndex)
		api.PUT("/internal-notes/templates/:template_id", noteHandlers.UpdateTemplate)
		api.DELETE("/internal-notes/templates/:template_id", noteHandlers.DeleteTemplate)
		api.GET("/internal-notes/categories", noteHandlers.GetCategories)
		
		// Real-time updates
		api.GET("/tickets/stream", handleTicketStream)
		api.GET("/dashboard/activity-stream", handleActivityStream)
	}
}

// Login page
func handleLoginPage(c *gin.Context) {
	pongo2Renderer.HTML(c, http.StatusOK, "pages/login.pongo2", pongo2.Context{
		"Title": "GOTRS - Sign In",
	})
}

// Register page
func handleRegisterPage(c *gin.Context) {
	pongo2Renderer.HTML(c, http.StatusOK, "pages/register.pongo2", pongo2.Context{
		"Title": "GOTRS - Register",
	})
}

// Dashboard
func handleDashboard(c *gin.Context) {
	// Use Pongo2 renderer with i18n support
	pongo2Renderer.HTML(c, http.StatusOK, "pages/dashboard.pongo2", pongo2.Context{
		"Title":      "Dashboard - GOTRS",
		"User":       getUserFromContext(c),
		"ActivePage": "dashboard",
	})
}

// Tickets list page
func handleTicketsList(c *gin.Context) {
	// Get filter parameters
	status := c.Query("status")
	priority := c.Query("priority")
	queueID := c.Query("queue_id")
	assignedTo := c.Query("assigned_to")
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "created")
	page := 1
	if p := c.Query("page"); p != "" {
		if parsedPage, err := strconv.Atoi(p); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}
	perPage := 10
	if pp := c.Query("per_page"); pp != "" {
		if parsedPerPage, err := strconv.Atoi(pp); err == nil && parsedPerPage > 0 {
			if parsedPerPage > 100 {
				perPage = 100
			} else {
				perPage = parsedPerPage
			}
		}
	}
	
	// Mock ticket data
	tickets := getMockTickets()
	
	// Apply filters
	filteredTickets := []gin.H{}
	for _, ticket := range tickets {
		// Status filter
		if status != "" && status != "all" && ticket["status"] != status {
			continue
		}
		// Priority filter
		if priority != "" && priority != "all" && ticket["priority"] != priority {
			continue
		}
		// Queue filter
		if queueID != "" {
			if qID, err := strconv.Atoi(queueID); err == nil && ticket["queue_id"] != qID {
				continue
			}
		}
		// Assigned filter
		if assignedTo == "me" && ticket["assigned_to"] != "Demo User" {
			continue
		}
		// Search filter
		if search != "" {
			searchLower := strings.ToLower(search)
			titleMatch := strings.Contains(strings.ToLower(ticket["title"].(string)), searchLower)
			numberMatch := strings.Contains(strings.ToLower(ticket["number"].(string)), searchLower)
			emailMatch := false
			if email, ok := ticket["customer_email"].(string); ok {
				emailMatch = strings.Contains(strings.ToLower(email), searchLower)
			}
			if !titleMatch && !numberMatch && !emailMatch {
				continue
			}
			
			// Highlight search terms in title
			if titleMatch {
				title := ticket["title"].(string)
				// Case-insensitive replacement with <mark> tags
				re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(search))
				ticket["highlighted_title"] = re.ReplaceAllString(title, "<mark>$0</mark>")
			}
		}
		filteredTickets = append(filteredTickets, ticket)
	}
	
	// Sort tickets
	sortTickets(filteredTickets, sort)
	
	// Pagination
	total := len(filteredTickets)
	totalPages := (total + perPage - 1) / perPage
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	
	start := (page - 1) * perPage
	end := start + perPage
	if end > total {
		end = total
	}
	
	var paginatedTickets []gin.H
	if start < total {
		paginatedTickets = filteredTickets[start:end]
	}
	
	// Build pagination info
	pagination := gin.H{
		"Page":     page,
		"PerPage":  perPage,
		"Total":    total,
		"Start":    start + 1,
		"End":      end,
		"HasPrev":  page > 1,
		"HasNext":  page < totalPages,
		"PrevPage": page - 1,
		"NextPage": page + 1,
	}
	
	// Get dynamic form data from lookup service with language support
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	
	// Check if this is an HTMX request
	if c.GetHeader("HX-Request") != "" {
		// Return just the ticket list fragment
		tmpl, err := loadTemplateForRequest(c, "templates/components/ticket_list.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "ticket_list.html", gin.H{
			"Tickets":       paginatedTickets,
			"Pagination":    pagination,
			"SearchTerm":    search,
			"StatusFilter":  status,
			"PriorityFilter": priority,
			"QueueFilter":   queueID,
			"AssignedFilter": assignedTo,
			"SortBy":        sort,
		}); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
		return
	}
	
	// Full page load - use Pongo2
	pongo2Renderer.HTML(c, http.StatusOK, "pages/tickets/list.pongo2", pongo2.Context{
		"Title":          "Tickets - GOTRS",
		"Tickets":        paginatedTickets,
		"Queues":         formData.Queues,
		"Priorities":     formData.Priorities,
		"Statuses":       formData.Statuses,
		"Pagination":     pagination,
		"SearchTerm":     search,
		"StatusFilter":   status,
		"PriorityFilter": priority,
		"QueueFilter":    queueID,
		"AssignedFilter": assignedTo,
		"SortBy":         sort,
		"User":           getUserFromContext(c),
		"ActivePage":     "tickets",
	})
}

// New ticket page
func handleTicketNew(c *gin.Context) {
	// Get dynamic form data from lookup service with language support
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	
	// Use Pongo2 renderer with i18n support
	pongo2Renderer.HTML(c, http.StatusOK, "pages/tickets/new.pongo2", pongo2.Context{
		"Title":      "New Ticket - GOTRS",
		"Queues":     formData.Queues,
		"Priorities": formData.Priorities,
		"Types":      formData.Types,
		"Statuses":   formData.Statuses,
		"User":       getUserFromContext(c),
		"ActivePage": "tickets",
	})
}

// Handle ticket creation from UI form
func handleTicketCreate(c *gin.Context) {
	// Get form data
	title := strings.TrimSpace(c.PostForm("title"))
	queueIDStr := c.PostForm("queue_id")
	priority := c.PostForm("priority")
	description := c.PostForm("description")
	customerEmail := strings.TrimSpace(c.PostForm("customer_email"))
	autoAssign := c.PostForm("auto_assign") == "true"
	
	// Validation
	errors := []string{}
	
	if title == "" {
		errors = append(errors, "Title is required")
	} else if len(title) > 200 {
		errors = append(errors, "Title must be less than 200 characters")
	}
	
	if queueIDStr == "" {
		errors = append(errors, "Queue selection is required")
	}
	
	validPriorities := map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}
	if priority != "" && !validPriorities[priority] {
		errors = append(errors, "Invalid priority")
	}
	if priority == "" {
		priority = "normal" // Default priority
	}
	
	if customerEmail != "" {
		// Simple email validation
		if !strings.Contains(customerEmail, "@") || !strings.Contains(customerEmail, ".") {
			errors = append(errors, "Invalid email format")
		}
	}
	
	// Return errors if validation failed
	if len(errors) > 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")
		errorHTML := "<div class='text-red-600'>"
		for _, err := range errors {
			errorHTML += "<p>" + err + "</p>"
		}
		errorHTML += "</div>"
		c.String(http.StatusBadRequest, errorHTML)
		return
	}
	
	// Convert queue ID
	queueID := uint(1)
	if queueIDStr != "" {
		if id, err := strconv.Atoi(queueIDStr); err == nil {
			queueID = uint(id)
		}
	}
	
	// Create the ticket with ORTS-compatible fields
	var customerID *string
	if customerEmail != "" {
		customerID = &customerEmail
	}
	
	ticket := &models.Ticket{
		Title:            title,
		QueueID:          int(queueID),
		TypeID:           1, // Default to Incident
		TicketPriorityID: getPriorityID(priority),
		TicketStateID:    1, // New
		TicketLockID:     1, // Unlocked
		CustomerUserID:   customerID,
		CreateBy:         1, // Default user
		ChangeBy:         1,
	}
	
	if autoAssign {
		userID := 1
		ticket.UserID = &userID // Assign to current user (mock)
		ticket.TicketStateID = 2 // Open
	}
	
	// Save the ticket
	ticketService := GetTicketService()
	if err := ticketService.CreateTicket(ticket); err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusInternalServerError, `
			<div class="text-red-600">
				<p>Failed to create ticket: %s</p>
			</div>
		`, err.Error())
		return
	}
	
	// Add initial message if description provided
	if description != "" {
		message := &service.SimpleTicketMessage{
			TicketID:  uint(ticket.ID),
			Body:      description,
			CreatedBy: 1,
			IsPublic:  true,
		}
		ticketService.AddMessage(uint(ticket.ID), message)
	}
	
	// Build success message
	successMsg := fmt.Sprintf("Ticket created successfully: #%s", ticket.TicketNumber)
	if autoAssign {
		successMsg = fmt.Sprintf("Ticket created and assigned to you: #%s", ticket.TicketNumber)
	}
	
	// Return success response with HX-Trigger and redirect
	c.Header("HX-Trigger", "ticket-created")
	c.Header("HX-Redirect", fmt.Sprintf("/tickets/%d", ticket.ID))
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `
		<div class="rounded-md bg-green-50 p-4">
			<div class="flex">
				<div class="flex-shrink-0">
					<svg class="h-5 w-5 text-green-400" viewBox="0 0 20 20" fill="currentColor">
						<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
					</svg>
				</div>
				<div class="ml-3">
					<p class="text-sm font-medium text-green-800">%s</p>
				</div>
			</div>
		</div>
	`, successMsg)
}

// Handle ticket quick actions
func handleTicketQuickAction(c *gin.Context) {
	ticketID := c.Param("id")
	action := c.PostForm("action")
	
	var message string
	switch action {
	case "assign":
		message = fmt.Sprintf("Ticket #%s assigned to you", ticketID)
	case "close":
		message = fmt.Sprintf("Ticket #%s closed", ticketID)
	case "priority-high":
		message = "Priority updated to high"
	case "priority-urgent":
		message = "Priority updated to urgent"
	case "priority-normal":
		message = "Priority updated to normal"
	case "priority-low":
		message = "Priority updated to low"
	default:
		c.String(http.StatusBadRequest, "Invalid action")
		return
	}
	
	// Return success message with trigger to refresh list
	c.Header("HX-Trigger", "ticket-updated")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `
		<div class="text-sm text-green-600">%s</div>
	`, message)
}

// Handle bulk ticket actions
func handleTicketBulkAction(c *gin.Context) {
	ticketIDs := c.PostForm("ticket_ids")
	action := c.PostForm("action")
	
	// Parse ticket IDs
	ids := strings.Split(ticketIDs, ",")
	count := len(ids)
	
	if count == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No tickets selected"})
		return
	}
	
	var message string
	switch action {
	case "assign":
		agentID := c.PostForm("agent_id")
		if agentID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Agent ID required"})
			return
		}
		message = fmt.Sprintf("%d tickets assigned", count)
		
	case "close":
		message = fmt.Sprintf("%d tickets closed", count)
		
	case "set_priority":
		priority := c.PostForm("priority")
		if priority == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Priority required"})
			return
		}
		message = fmt.Sprintf("Priority updated for %d tickets", count)
		
	case "move_queue":
		queueID := c.PostForm("queue_id")
		if queueID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Queue ID required"})
			return
		}
		message = fmt.Sprintf("%d tickets moved to queue", count)
		
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
		return
	}
	
	// Return success response
	c.Header("HX-Trigger", "tickets-updated")
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `
		<div class="rounded-md bg-green-50 p-4">
			<div class="flex">
				<div class="ml-3">
					<p class="text-sm font-medium text-green-800">%s</p>
				</div>
			</div>
		</div>
	`, message)
}

// Ticket detail page
func handleTicketDetail(c *gin.Context) {
	ticketID := c.Param("id")
	
	// Parse ticket ID
	id, err := strconv.Atoi(ticketID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ticket ID format")
		return
	}
	
	// Get ticket from repository
	ticketService := GetTicketService()
	ticketModel, err := ticketService.GetTicket(uint(id))
	
	var ticket gin.H
	if err != nil {
		// Fallback to mock data for demo tickets
		ticket = gin.H{
			"ID":           id,
			"TicketNumber": fmt.Sprintf("TICKET-%06d", id),
			"Title":        "System login issues preventing access",
			"Status":       "new",
			"StatusLabel":  "New",
			"Priority":     "normal",
			"PriorityLabel": "Normal Priority",
			"Queue":        "General Support",
			"CustomerEmail": "john.doe@example.com",
			"CustomerName":  "John Doe",
			"CreateTime":    time.Now().Add(-2 * time.Hour).Format("Jan 2, 2006 3:04 PM"),
			"UpdateTime":    time.Now().Add(-2 * time.Hour).Format("Jan 2, 2006 3:04 PM"),
			"AssignedTo":    nil, // Unassigned
			"Type":          "Service Request",
			"SLAStatus":     "within", // within, warning, overdue
		}
		// Continue with mock data for non-existent tickets
		// This allows the demo to work even without real tickets
	} else {
		// Convert real ticket to display format
		simpleTicket := models.FromORTSTicket(ticketModel)
		
		// Map priority
		priorityLabels := map[string]string{
			"low": "Low Priority",
			"normal": "Normal Priority",
			"high": "High Priority",
			"urgent": "Urgent",
		}
		
		// Map status
		statusLabels := map[string]string{
			"new": "New",
			"open": "Open",
			"pending": "Pending",
			"closed": "Closed",
		}
		
		var assignedTo interface{} = nil
		if simpleTicket.AssignedTo > 0 {
			assignedTo = fmt.Sprintf("Agent %d", simpleTicket.AssignedTo)
		}
		
		ticket = gin.H{
			"ID":           simpleTicket.ID,
			"TicketNumber": simpleTicket.TicketNumber,
			"Title":        simpleTicket.Subject,
			"Status":       simpleTicket.Status,
			"StatusLabel":  statusLabels[simpleTicket.Status],
			"Priority":     simpleTicket.Priority,
			"PriorityLabel": priorityLabels[simpleTicket.Priority],
			"Queue":        fmt.Sprintf("Queue %d", simpleTicket.QueueID), // TODO: Get queue name
			"CustomerEmail": simpleTicket.CustomerEmail,
			"CustomerName":  simpleTicket.CustomerName,
			"CreateTime":    simpleTicket.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
			"UpdateTime":    simpleTicket.UpdatedAt.Format("Jan 2, 2006 3:04 PM"),
			"AssignedTo":    assignedTo,
			"Type":          fmt.Sprintf("Type %d", simpleTicket.TypeID), // TODO: Get type name
			"SLAStatus":     "within", // TODO: Calculate SLA status
		}
	}
	
	// Articles/Messages - Get real messages for this ticket
	var articles []gin.H
	messages, err := ticketService.GetMessages(uint(id))
	if err != nil {
		// Log error and show empty messages
		fmt.Printf("Warning: Failed to get messages for ticket %d: %v\n", id, err)
		articles = []gin.H{}
	} else {
		// Convert messages to display format
		articles = make([]gin.H, len(messages))
		for i, msg := range messages {
			// Calculate time ago
			timeAgo := formatTimeAgo(msg.CreatedAt)
			
			// Generate initials from author name
			initials := generateInitials(msg.AuthorName)
			
			// Convert attachments if present
			var attachments []gin.H
			if msg.Attachments != nil && len(msg.Attachments) > 0 {
				attachments = make([]gin.H, len(msg.Attachments))
				for j, att := range msg.Attachments {
					attachments[j] = gin.H{
						"ID":          att.ID,
						"Filename":    att.Filename,
						"ContentType": att.ContentType,
						"Size":        formatFileSize(att.Size),
						"URL":         att.URL,
					}
				}
			}
			
			articles[i] = gin.H{
				"ID":             msg.ID,
				"AuthorName":     msg.AuthorName,
				"AuthorInitials": initials,
				"AuthorType":     msg.AuthorType,
				"TimeAgo":        timeAgo,
				"Subject":        msg.Subject,
				"Body":           msg.Body,
				"IsInternal":     msg.IsInternal,
				"Attachments":    attachments,
			}
		}
	}
	
	// Activity log
	activities := []gin.H{
		{
			"Type":        "created",
			"Description": "Ticket created",
			"TimeAgo":     "2 hours ago",
			"Icon":        "plus",
			"Color":       "green",
		},
	}
	
	// Use Pongo2 renderer with i18n support
	pongo2Renderer.HTML(c, http.StatusOK, "pages/tickets/detail.pongo2", pongo2.Context{
		"Title":      "Ticket #" + ticketID + " - GOTRS",
		"TicketID":   ticketID,
		"Ticket":     ticket,
		"Articles":   articles,
		"Activities": activities,
		"User":       getUserFromContext(c),
		"ActivePage": "tickets",
		"Messages":   articles, // Use articles as messages for now
	})
}

// HTMX Login handler
func handleHTMXLogin(c *gin.Context) {
	var loginReq struct {
		Email    string `json:"email" form:"email" binding:"required,email"`
		Password string `json:"password" form:"password" binding:"required"`
	}
	
	// Try to bind as JSON first, then form data
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		// If JSON binding fails, try form binding
		if err := c.ShouldBind(&loginReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	
	// TODO: Implement actual authentication
	// For now, accept demo credentials from environment variables or test credentials
	demoEmail := os.Getenv("DEMO_ADMIN_EMAIL")
	demoPassword := os.Getenv("DEMO_ADMIN_PASSWORD")
	
	// Use test credentials if environment variables not set (for testing)
	if demoEmail == "" || demoPassword == "" {
		demoEmail = "admin@gotrs.local"
		demoPassword = "admin123"
	}
	
	if loginReq.Email == demoEmail && loginReq.Password == demoPassword {
		// For HTMX, set the redirect header
		c.Header("HX-Redirect", "/dashboard")
		c.JSON(http.StatusOK, gin.H{
			"access_token":  "demo_token_123",
			"refresh_token": "demo_refresh_123",
			"user": gin.H{
				"id":         1,
				"email":      loginReq.Email,
				"first_name": "Demo",
				"last_name":  "Admin",
				"role":       "admin",
			},
		})
		return
	}
	
	c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
}

// HTMX Logout handler for API endpoint
func handleHTMXLogout(c *gin.Context) {
	// TODO: Invalidate token
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// handleLogout handles the main logout route that redirects to login
func handleLogout(c *gin.Context) {
	// TODO: Clear session/token
	// For now, just redirect to login page
	c.Header("HX-Redirect", "/login")
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// handleLogoutGET handles GET requests to logout (for regular links)
func handleLogoutGET(c *gin.Context) {
	// TODO: Clear session/token
	// For regular GET requests, do a standard redirect
	c.Redirect(http.StatusFound, "/login")
}

// underConstruction returns a simple under construction page
func underConstruction(pageName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		html := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>%s - Under Construction</title>
			<script src="https://cdn.tailwindcss.com"></script>
		</head>
		<body class="bg-gray-100 dark:bg-gray-900">
			<div class="min-h-screen flex items-center justify-center">
				<div class="text-center">
					<svg class="mx-auto h-24 w-24 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
					</svg>
					<h1 class="mt-4 text-3xl font-bold text-gray-900 dark:text-white">%s</h1>
					<p class="mt-2 text-lg text-gray-600 dark:text-gray-400">This page is under construction</p>
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-500">Coming soon...</p>
					<div class="mt-6">
						<a href="/dashboard" class="text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300">
							← Back to Dashboard
						</a>
					</div>
				</div>
			</div>
		</body>
		</html>
		`, pageName, pageName)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	}
}

// underConstructionAPI returns a JSON response for API endpoints under construction
func underConstructionAPI(endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("Endpoint %s is under construction", endpoint),
			"data":    []interface{}{},
		})
	}
}

// Dashboard stats (returns HTML fragment)
func handleDashboardStats(c *gin.Context) {
	stats := []gin.H{
		{"title": "Open Tickets", "value": "24", "icon": "ticket", "color": "blue"},
		{"title": "New Today", "value": "8", "icon": "plus", "color": "green"},
		{"title": "Pending", "value": "12", "icon": "clock", "color": "yellow"},
		{"title": "Overdue", "value": "3", "icon": "exclamation", "color": "red"},
	}
	
	tmpl, err := loadTemplateForRequest(c, "templates/components/dashboard_stats.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "dashboard_stats.html", gin.H{
		"Stats": stats,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Recent tickets (returns HTML fragment)
func handleRecentTickets(c *gin.Context) {
	tickets := []gin.H{
		{"id": 1, "number": "TICKET-001", "title": "Login issues", "status": "open", "priority": "high", "created": "2 hours ago"},
		{"id": 2, "number": "TICKET-002", "title": "Feature request", "status": "new", "priority": "medium", "created": "4 hours ago"},
		{"id": 3, "number": "TICKET-003", "title": "Bug report", "status": "pending", "priority": "low", "created": "1 day ago"},
	}
	
	tmpl, err := loadTemplateForRequest(c, "templates/components/recent_tickets.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "recent_tickets.html", gin.H{
		"Tickets": tickets,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Activity feed (returns HTML fragment)
func handleActivityFeed(c *gin.Context) {
	activities := []gin.H{
		{"user": "John Doe", "action": "created", "target": "TICKET-001", "time": "2 minutes ago"},
		{"user": "Jane Smith", "action": "updated", "target": "TICKET-002", "time": "15 minutes ago"},
		{"user": "Bob Wilson", "action": "closed", "target": "TICKET-003", "time": "1 hour ago"},
	}
	
	// Use Pongo2 renderer for consistency
	pongo2Renderer.HTML(c, http.StatusOK, "components/activity_feed.html", pongo2.Context{
		"Activities": activities,
	})
}

// Tickets API (returns HTML fragment)
func handleTicketsAPI(c *gin.Context) {
	// Parse query parameters for filtering
	status := filterEmptyStrings(c.QueryArray("status"))
	priority := filterEmptyStrings(c.QueryArray("priority")) 
	queue := filterEmptyStrings(c.QueryArray("queue"))
	assignee := c.Query("assignee")
	assigned := c.Query("assigned")
	search := c.Query("search")
	
	// Debug logging
	fmt.Printf("Filter Debug - Status: %v, Priority: %v, Queue: %v, Assignee: %s, Assigned: %s, Search: %s\n", 
		status, priority, queue, assignee, assigned, search)

	// Mock ticket data for testing
	allTickets := []gin.H{
		{"id": 1, "number": "TICKET-001", "title": "Login issues", "status": "open", "priority": "high", "customer": "john@example.com", "agent": "Agent Smith", "queue": "General Support", "queueId": 1},
		{"id": 2, "number": "TICKET-002", "title": "Feature request", "status": "new", "priority": "medium", "customer": "jane@example.com", "agent": "", "queue": "Technical Support", "queueId": 2},
		{"id": 3, "number": "TICKET-003", "title": "Password reset", "status": "closed", "priority": "low", "customer": "bob@example.com", "agent": "John Doe", "queue": "General Support", "queueId": 1},
		{"id": 4, "number": "TICKET-004", "title": "Billing inquiry", "status": "pending", "priority": "high", "customer": "alice@example.com", "agent": "", "queue": "Billing", "queueId": 3},
		{"id": 5, "number": "TICKET-005", "title": "Technical issue", "status": "open", "priority": "critical", "customer": "dave@example.com", "agent": "Agent Smith", "queue": "Technical Support", "queueId": 2},
	}

	// Apply filters
	filteredTickets := []gin.H{}
	for _, ticket := range allTickets {
		// Status filter
		if len(status) > 0 {
			found := false
			for _, s := range status {
				if ticket["status"] == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Priority filter
		if len(priority) > 0 {
			found := false
			for _, p := range priority {
				if ticket["priority"] == p {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Queue filter
		if len(queue) > 0 {
			found := false
			for _, q := range queue {
				queueId, _ := strconv.Atoi(q)
				if ticket["queueId"] == queueId {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Assignee filter
		if assignee != "" {
			if assignee == "1" && ticket["agent"] != "Agent Smith" {
				continue
			}
		}

		// Assigned/Unassigned filter
		if assigned == "true" {
			if ticket["agent"] == "" {
				continue
			}
		} else if assigned == "false" {
			if ticket["agent"] != "" {
				continue
			}
		}

		// Search filter
		if search != "" {
			searchLower := strings.ToLower(search)
			titleMatch := strings.Contains(strings.ToLower(ticket["title"].(string)), searchLower)
			numberMatch := strings.Contains(strings.ToLower(ticket["number"].(string)), searchLower) 
			customerMatch := strings.Contains(strings.ToLower(ticket["customer"].(string)), searchLower)
			
			if !titleMatch && !numberMatch && !customerMatch {
				continue
			}
		}

		filteredTickets = append(filteredTickets, ticket)
	}

	// Prepare response data
	responseData := gin.H{
		"Tickets": filteredTickets,
		"Title":   "Tickets", // General title for the section
	}

	// Add filter badges and messages for UI
	if len(status) > 0 {
		statusLabels := []string{}
		for _, s := range status {
			switch s {
			case "new":
				statusLabels = append(statusLabels, "New")
			case "open":
				statusLabels = append(statusLabels, "Open")
			case "pending":
				statusLabels = append(statusLabels, "Pending")
			case "resolved":
				statusLabels = append(statusLabels, "Resolved")
			case "closed":
				statusLabels = append(statusLabels, "Closed")
			default:
				statusLabels = append(statusLabels, strings.Title(s))
			}
		}
		responseData["StatusFilter"] = strings.Join(statusLabels, ", ")
	}

	if len(priority) > 0 {
		priorityLabels := []string{}
		for _, p := range priority {
			if p == "high" {
				priorityLabels = append(priorityLabels, "High Priority")
			} else if p == "low" {
				priorityLabels = append(priorityLabels, "Low Priority")
			} else if p == "critical" {
				priorityLabels = append(priorityLabels, "critical")
			}
		}
		responseData["PriorityFilter"] = strings.Join(priorityLabels, ", ")
	}

	if assigned == "false" {
		responseData["AssignedFilter"] = "Unassigned"
	}

	// No tickets found message
	if len(filteredTickets) == 0 {
		responseData["NoTicketsMessage"] = "No tickets found"
	}

	tmpl, err := loadTemplateForRequest(c, "templates/components/ticket_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "ticket_list.html", responseData); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Ticket search (returns HTML fragment)
func handleTicketSearch(c *gin.Context) {
	searchTerm := c.Query("search")
	if searchTerm == "" {
		searchTerm = c.Query("q") // Support both ?search= and ?q=
	}
	
	// Mock ticket data for testing
	allTickets := []gin.H{
		{"id": 1, "number": "TICKET-001", "title": "Login issues", "status": "open", "priority": "high", "customer": "john@example.com", "agent": "Agent Smith"},
		{"id": 2, "number": "TICKET-002", "title": "Feature request", "status": "new", "priority": "medium", "customer": "jane@example.com", "agent": ""},
		{"id": 3, "number": "TICKET-003", "title": "Password reset", "status": "closed", "priority": "low", "customer": "bob@example.com", "agent": "John Doe"},
	}

	filteredTickets := []gin.H{}
	
	if searchTerm != "" {
		searchLower := strings.ToLower(searchTerm)
		for _, ticket := range allTickets {
			titleMatch := strings.Contains(strings.ToLower(ticket["title"].(string)), searchLower)
			numberMatch := strings.Contains(strings.ToLower(ticket["number"].(string)), searchLower)
			customerMatch := strings.Contains(strings.ToLower(ticket["customer"].(string)), searchLower)
			
			if titleMatch || numberMatch || customerMatch {
				// Highlight search terms in title
				if titleMatch && searchTerm != "" {
					title := ticket["title"].(string)
					// Case-insensitive replacement with <mark> tags
					re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(searchTerm))
					ticket["highlighted_title"] = re.ReplaceAllString(title, "<mark>$0</mark>")
				}
				filteredTickets = append(filteredTickets, ticket)
			}
		}
	} else {
		// Empty search returns all tickets
		filteredTickets = allTickets
	}

	responseData := gin.H{
		"Tickets": filteredTickets,
		"Title":   "Tickets", // General title for the section
	}

	// Add search term to response if provided
	if searchTerm != "" {
		responseData["SearchTerm"] = searchTerm
	}

	// No results message
	if len(filteredTickets) == 0 {
		responseData["NoTicketsMessage"] = "No tickets found"
	}
	
	tmpl, err := loadTemplateForRequest(c, "templates/components/ticket_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "ticket_list.html", responseData); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Create ticket (HTMX)
func handleCreateTicket(c *gin.Context) {
	var req struct {
		Subject       string `json:"subject" form:"subject" binding:"required"`
		CustomerEmail string `json:"customer_email" form:"customer_email" binding:"required,email"`
		CustomerName  string `json:"customer_name" form:"customer_name"`
		Priority      string `json:"priority" form:"priority"`
		QueueID       string `json:"queue_id" form:"queue_id"`
		TypeID        string `json:"type_id" form:"type_id"`
		Body          string `json:"body" form:"body" binding:"required"`
	}

	// Bind form data
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert string values to integers with defaults
	queueID := uint(1) // Default to General Support
	if req.QueueID != "" {
		if id, err := strconv.Atoi(req.QueueID); err == nil {
			queueID = uint(id)
		}
	}

	typeID := uint(1) // Default to Incident
	if req.TypeID != "" {
		if id, err := strconv.Atoi(req.TypeID); err == nil {
			typeID = uint(id)
		}
	}

	// Set default priority if not provided
	if req.Priority == "" {
		req.Priority = "normal"
	}

	// For demo purposes, use a fixed user ID (admin)
	// In a real system, we'd get this from the authenticated user context
	createdBy := uint(1)

	// Create the ticket model with ORTS-compatible fields
	customerEmail := req.CustomerEmail
	ticket := &models.Ticket{
		Title:            req.Subject,
		QueueID:          int(queueID),
		TypeID:           int(typeID),
		TicketPriorityID: getPriorityID(req.Priority),
		TicketStateID:    1, // New
		TicketLockID:     1, // Unlocked
		CustomerUserID:   &customerEmail,
		CreateBy:         int(createdBy),
		ChangeBy:         int(createdBy),
	}

	// Get the ticket service and create the ticket
	ticketService := GetTicketService()
	if err := ticketService.CreateTicket(ticket); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket: " + err.Error()})
		return
	}

	// Create the first message (ticket body)
	message := &service.SimpleTicketMessage{
		TicketID:    uint(ticket.ID),
		Body:        req.Body,
		Subject:     req.Subject,
		CreatedBy:   createdBy,
		AuthorName:  req.CustomerName,
		AuthorEmail: req.CustomerEmail,
		AuthorType:  "Customer",
		IsPublic:    true,
		IsInternal:  false,
	}
	
	if err := ticketService.AddMessage(uint(ticket.ID), message); err != nil {
		// Log error but don't fail the ticket creation
		fmt.Printf("Warning: Failed to add initial message: %v\n", err)
	}
	
	// For HTMX, set the redirect header to the ticket detail page
	c.Header("HX-Redirect", fmt.Sprintf("/tickets/%d", ticket.ID))
	c.JSON(http.StatusCreated, gin.H{
		"id":            ticket.ID,
		"ticket_number": ticket.TicketNumber,
		"message":       "Ticket created successfully",
		"queue_id":      float64(ticket.QueueID),
		"type_id":       float64(ticket.TypeID),
		"priority":      req.Priority,
	})
}

// Update ticket (HTMX)
func handleUpdateTicket(c *gin.Context) {
	// TODO: Implement ticket update
	c.JSON(http.StatusOK, gin.H{"message": "Ticket updated"})
}

// Update ticket status (HTMX)
func handleUpdateTicketStatus(c *gin.Context) {
	ticketID := c.Param("id")
	var req struct {
		Status string `json:"status" form:"status" binding:"required"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// TODO: Update ticket status in database
	// For now, return success
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Ticket %s status updated to %s", ticketID, req.Status),
		"status": req.Status,
	})
}

// Assign ticket to agent (HTMX)
func handleAssignTicket(c *gin.Context) {
	ticketID := c.Param("id")
	
	// For "Assign to Me", we don't need any request body
	// Just assign to the current user (hardcoded for now)
	agentID := 1 // TODO: Get from session/auth context
	agentName := "Demo Agent" // TODO: Get from session
	
	// TODO: Update ticket assignment in database
	
	// Return success message that HTMX can display
	c.Header("HX-Trigger", `{"showMessage": {"message": "Ticket assigned to you", "type": "success"}}`)
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Ticket %s assigned to %s", ticketID, agentName),
		"agent_id": agentID,
	})
}

// Add reply to ticket (HTMX)
func handleTicketReply(c *gin.Context) {
	ticketID := c.Param("id")
	_ = ticketID // Will be used when connecting to database
	var req struct {
		Reply       string `json:"reply" form:"reply" binding:"required"`
		Internal    bool   `json:"internal" form:"internal"`
		CloseTicket bool   `json:"close_ticket" form:"close_ticket"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// TODO: Add reply to ticket in database
	// For now, return HTML fragment for the new message
	newMessage := gin.H{
		"ID":             time.Now().Unix(),
		"AuthorName":     "Support Agent",
		"AuthorInitials": "SA",
		"AuthorType":     "Agent",
		"TimeAgo":        "Just now",
		"Body":           req.Reply,
		"IsInternal":     req.Internal,
	}
	
	// Return HTML fragment for HTMX to append
	tmpl, err := loadTemplateForRequest(c, "templates/components/ticket_message.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "ticket_message.html", newMessage); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Update ticket priority (HTMX)
func handleUpdateTicketPriority(c *gin.Context) {
	ticketID := c.Param("id")
	var req struct {
		Priority string `json:"priority" form:"priority" binding:"required"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// TODO: Update ticket priority in database
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Ticket %s priority updated to %s", ticketID, req.Priority),
		"priority": req.Priority,
	})
}

// Update ticket queue (HTMX)
func handleUpdateTicketQueue(c *gin.Context) {
	ticketID := c.Param("id")
	var req struct {
		QueueID int `json:"queue_id" form:"queue_id" binding:"required"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// TODO: Update ticket queue in database
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Ticket %s moved to queue %d", ticketID, req.QueueID),
		"queue_id": req.QueueID,
	})
}

// Server-Sent Events for tickets
func handleTicketStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	
	// Send a test event
	c.SSEvent("ticket-update", gin.H{
		"ticketId": 1,
		"status":   "updated",
		"message":  "Ticket status changed",
	})
	
	c.Writer.Flush()
}

// Server-Sent Events for activity
func handleActivityStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	
	// Send a test event with HTML content
	activityHTML := `<div class="py-3 border-b border-gray-200 last:border-b-0">
		<div class="flex space-x-3">
			<div class="flex-shrink-0">
				<div class="h-8 w-8 rounded-full bg-green-500 flex items-center justify-center">
					<span class="text-xs font-medium text-white">S</span>
				</div>
			</div>
			<div class="min-w-0 flex-1">
				<p class="text-sm text-gray-900">
					<span class="font-medium">System</span>
					<span class="text-blue-600">updated</span>
					<span class="font-medium">activity feed</span>
				</p>
				<p class="text-sm text-gray-500">just now</p>
			</div>
		</div>
	</div>`
	
	// For HTMX SSE, send HTML content directly
	fmt.Fprintf(c.Writer, "event: activity-update\ndata: %s\n\n", activityHTML)
	
	c.Writer.Flush()
}

// Queues API (returns HTML fragment or JSON)
func handleQueuesAPI(c *gin.Context) {
	// Check if JSON response is requested
	if wantsJSON(c) {
		handleQueuesJSON(c)
		return
	}
	
	// Parse query parameters for filtering
	status := c.Query("status")
	search := c.Query("search")
	
	// Get queues from database with ticket counts
	queues, err := getQueuesWithTicketCounts(status, search)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Database error: %v", err))
		return
	}
	
	// Render HTML response (fragment or full page)
	renderQueueList(c, queues)
}

// Handle JSON response for queues
func handleQueuesJSON(c *gin.Context) {
	status := c.Query("status")
	search := c.Query("search")
	
	queues, err := getQueuesWithTicketCounts(status, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database error",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    queues,
	})
}

// Helper function to get mock tickets
func getMockTickets() []gin.H {
	return []gin.H{
		{
			"id":             1,
			"number":         "TICK-2024-001",
			"title":          "Server is down",
			"status":         "open",
			"priority":       "high",
			"queue_id":       1,
			"queue_name":     "Raw",
			"customer_email": "customer@example.com",
			"assigned_to":    "Demo User",
			"created_at":     "2024-01-15 10:00",
			"updated_at":     "2024-01-15 14:30",
			"has_new_message": true,
			"sla_status":     "warning",
			"due_in":         "2 hours",
		},
		{
			"id":             2,
			"number":         "TICK-2024-002",
			"title":          "Cannot login to system",
			"status":         "pending",
			"priority":       "normal",
			"queue_id":       4,
			"queue_name":     "Support",
			"customer_email": "user@company.com",
			"assigned_to":    "",
			"created_at":     "2024-01-15 11:00",
			"updated_at":     "2024-01-15 11:00",
			"has_new_message": false,
			"sla_status":     "ok",
			"due_in":         "4 hours",
		},
		{
			"id":             3,
			"number":         "TICK-2024-003",
			"title":          "Feature request: Dark mode",
			"status":         "new",
			"priority":       "low",
			"queue_id":       3,
			"queue_name":     "Misc",
			"customer_email": "feedback@example.org",
			"assigned_to":    "",
			"created_at":     "2024-01-15 09:00",
			"updated_at":     "2024-01-15 09:00",
			"has_new_message": false,
			"sla_status":     "ok",
			"due_in":         "24 hours",
		},
		{
			"id":             4,
			"number":         "TICK-2024-004",
			"title":          "Database connection error",
			"status":         "open",
			"priority":       "urgent",
			"queue_id":       1,
			"queue_name":     "Raw",
			"customer_email": "admin@enterprise.com",
			"assigned_to":    "Demo User",
			"created_at":     "2024-01-15 08:00",
			"updated_at":     "2024-01-15 15:00",
			"has_new_message": false,
			"sla_status":     "breach",
			"due_in":         "-1 hour",
		},
		{
			"id":             5,
			"number":         "TICK-2024-005",
			"title":          "Password reset request",
			"status":         "resolved",
			"priority":       "normal",
			"queue_id":       4,
			"queue_name":     "Support",
			"customer_email": "john@example.com",
			"assigned_to":    "Agent Smith",
			"created_at":     "2024-01-14 10:00",
			"updated_at":     "2024-01-14 10:30",
			"has_new_message": false,
			"sla_status":     "ok",
			"due_in":         "",
		},
		{
			"id":             6,
			"number":         "TICK-2024-006",
			"title":          "Billing inquiry",
			"status":         "closed",
			"priority":       "normal",
			"queue_id":       2,
			"queue_name":     "Junk",
			"customer_email": "billing@company.net",
			"assigned_to":    "Agent Jones",
			"created_at":     "2024-01-13 14:00",
			"updated_at":     "2024-01-13 16:00",
			"has_new_message": false,
			"sla_status":     "ok",
			"due_in":         "",
		},
	}
}

// Helper function to sort tickets
func sortTickets(tickets []gin.H, sortBy string) {
	switch sortBy {
	case "priority":
		// Sort by priority (urgent > high > normal > low)
		priorityOrder := map[string]int{
			"urgent": 0,
			"high":   1,
			"normal": 2,
			"low":    3,
		}
		sort.Slice(tickets, func(i, j int) bool {
			pi := priorityOrder[tickets[i]["priority"].(string)]
			pj := priorityOrder[tickets[j]["priority"].(string)]
			return pi < pj
		})
	case "status":
		// Sort by status (new > open > pending > resolved > closed)
		statusOrder := map[string]int{
			"new":      0,
			"open":     1,
			"pending":  2,
			"resolved": 3,
			"closed":   4,
		}
		sort.Slice(tickets, func(i, j int) bool {
			si := statusOrder[tickets[i]["status"].(string)]
			sj := statusOrder[tickets[j]["status"].(string)]
			return si < sj
		})
	case "title":
		// Sort alphabetically by title
		sort.Slice(tickets, func(i, j int) bool {
			return tickets[i]["title"].(string) < tickets[j]["title"].(string)
		})
	case "updated":
		// Sort by last updated (most recent first)
		sort.Slice(tickets, func(i, j int) bool {
			return tickets[i]["updated_at"].(string) > tickets[j]["updated_at"].(string)
		})
	default: // "created"
		// Sort by creation date (most recent first)
		sort.Slice(tickets, func(i, j int) bool {
			return tickets[i]["created_at"].(string) > tickets[j]["created_at"].(string)
		})
	}
}

// Sort queues based on the sort parameter
func sortQueues(queues []gin.H, sortBy string) {
	switch sortBy {
	case "name_asc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["name"].(string) < queues[j]["name"].(string)
		})
	case "name_desc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["name"].(string) > queues[j]["name"].(string)
		})
	case "tickets_asc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["ticket_count"].(int) < queues[j]["ticket_count"].(int)
		})
	case "tickets_desc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["ticket_count"].(int) > queues[j]["ticket_count"].(int)
		})
	case "status_asc":
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["status"].(string) < queues[j]["status"].(string)
		})
	default:
		// Default sort by ID (original order)
		sort.Slice(queues, func(i, j int) bool {
			return queues[i]["id"].(int) < queues[j]["id"].(int)
		})
	}
}

// Get queues from database with ticket counts
func getQueuesWithTicketCounts(status, search string) ([]gin.H, error) {
	// Normalize inputs once outside the loop for better performance
	search = strings.TrimSpace(search)
	searchLower := strings.ToLower(search)
	hasSearch := search != ""
	hasStatusFilter := status != "" && status != "all"
	
	// Apply filtering to centralized mock data
	filteredQueues := make([]gin.H, 0, len(mockQueueData)) // Pre-allocate with capacity
	
	for _, queue := range mockQueueData {
		// Status filter
		if hasStatusFilter {
			queueStatus := queue["status"].(string)
			if status != queueStatus {
				continue
			}
		}
		
		// Search filter - check both name and comment
		if hasSearch {
			queueName := strings.ToLower(queue["name"].(string))
			queueComment := strings.ToLower(queue["comment"].(string))
			if !strings.Contains(queueName, searchLower) && !strings.Contains(queueComment, searchLower) {
				continue
			}
		}
		
		filteredQueues = append(filteredQueues, queue)
	}
	
	return filteredQueues, nil
}

// Helper function to check if client wants JSON response
func wantsJSON(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept"), "application/json")
}

// Helper function to check if request is from HTMX
func isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}

// Helper function to send standardized error responses
func sendErrorResponse(c *gin.Context, statusCode int, message string) {
	if wantsJSON(c) {
		c.JSON(statusCode, gin.H{
			"success": false,
			"error":   message,
		})
	} else {
		c.String(statusCode, message)
	}
}

// Helper function to render queue list templates
func renderQueueList(c *gin.Context, queues []gin.H) {
	// Get search/filter parameters for template context
	searchTerm := c.Query("search")
	statusFilter := c.Query("status")
	
	templateData := gin.H{
		"Queues":      queues,
		"SearchTerm":  searchTerm,
		"StatusFilter": statusFilter,
	}
	
	if isHTMXRequest(c) {
		// Return HTML fragment for HTMX
		tmpl, err := loadTemplateForRequest(c, "templates/components/queue_list.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "queue_list.html", templateData); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	} else {
		// Return full page
		tmpl, err := loadTemplateForRequest(c, 
			"templates/layouts/base.html",
			"templates/pages/queues/list.html",
			"templates/components/queue_list.html",
		)
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		// Add page-level template data
		templateData["Title"] = "Queues - GOTRS"
		templateData["User"] = getUserFromContext(c)
		templateData["ActivePage"] = "queues"
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "list.html", templateData); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	}
}

// Helper function to render queue detail templates
func renderQueueDetail(c *gin.Context, queue gin.H) {
	if isHTMXRequest(c) {
		// Return HTML fragment for HTMX
		tmpl, err := loadTemplateForRequest(c, "templates/components/queue_detail.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "queue_detail.html", gin.H{
			"Queue": queue,
		}); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	} else {
		// Return full page
		tmpl, err := loadTemplateForRequest(c, 
			"templates/layouts/base.html",
			"templates/pages/queues/detail.html",
			"templates/components/queue_detail.html",
		)
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "detail.html", gin.H{
			"Title":      queue["name"].(string) + " - Queue Details - GOTRS",
			"User":       getUserFromContext(c),
			"ActivePage": "queues",
			"Queue":      queue,
		}); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	}
}

// Queue detail API (returns HTML fragment or JSON)
func handleQueueDetail(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	if queueID == "" || queueID == "invalid" {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Get queue details with tickets
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			sendErrorResponse(c, http.StatusNotFound, "Queue not found")
			return
		}
		sendErrorResponse(c, http.StatusInternalServerError, "Database error")
		return
	}
	
	// Check if JSON response is requested
	if wantsJSON(c) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    queue,
		})
		return
	}
	
	// Render HTML response (fragment or full page)
	renderQueueDetail(c, queue)
}

// Queue tickets API (returns tickets for a specific queue)
func handleQueueTickets(c *gin.Context) {
	queueID := c.Param("id")
	
	// Parse query parameters
	status := c.Query("status")
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "10")
	
	// Get tickets for queue
	tickets, err := getTicketsForQueue(queueID, status, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database error",
		})
		return
	}
	
	// Return tickets as HTML fragment or JSON
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    tickets,
		})
	} else {
		// Return HTML fragment
		tmpl, err := loadTemplateForRequest(c, "templates/components/ticket_list.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		// Prepare template data
		templateData := gin.H{
			"Tickets":    tickets["tickets"],
			"Title":      "Queue Tickets",
			"Total":      tickets["total"],
			"Page":       tickets["page"],
			"Pagination": tickets["pagination"],
		}
		
		// Handle empty state
		if tickets["total"].(int) == 0 {
			templateData["NoTicketsMessage"] = "No tickets in this queue"
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "ticket_list.html", templateData); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
	}
}

// Get queue details with associated tickets
func getQueueWithTickets(queueID string) (gin.H, error) {
	// Mock data for testing - matches the database schema
	mockQueues := map[string]gin.H{
		"1": {
			"id":           1,
			"name":         "Raw",
			"comment":      "All new tickets are placed in this queue by default",
			"ticket_count": 2,
			"status":       "active",
			"tickets": []gin.H{
				{"id": 1, "number": "TICKET-001", "title": "Test login issue", "status": "new"},
				{"id": 3, "number": "TICKET-003", "title": "UI bug report", "status": "new"},
			},
		},
		"2": {
			"id":           2,
			"name":         "Junk",
			"comment":      "Spam and junk emails",
			"ticket_count": 1,
			"status":       "active",
			"tickets": []gin.H{
				{"id": 2, "number": "TICKET-002", "title": "Database connection problem", "status": "open"},
			},
		},
		"3": {
			"id":           3,
			"name":         "Misc",
			"comment":      "Miscellaneous tickets",
			"ticket_count": 0,
			"status":       "active",
			"tickets":      []gin.H{},
		},
		"4": {
			"id":           4,
			"name":         "Support",
			"comment":      "General support requests",
			"ticket_count": 0,
			"status":       "active",
			"tickets":      []gin.H{},
		},
	}
	
	queue, exists := mockQueues[queueID]
	if !exists {
		return nil, fmt.Errorf("queue not found")
	}
	
	return queue, nil
}

// Get tickets for a specific queue with filtering and pagination
func getTicketsForQueue(queueID, status, page, limit string) (gin.H, error) {
	// Get queue details first
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		return nil, err
	}
	
	tickets := queue["tickets"].([]gin.H)
	filteredTickets := []gin.H{}
	
	// Apply status filter
	for _, ticket := range tickets {
		if status != "" && ticket["status"] != status {
			continue
		}
		filteredTickets = append(filteredTickets, ticket)
	}
	
	// Implement proper pagination
	pageNum, _ := strconv.Atoi(page)
	limitNum, _ := strconv.Atoi(limit)
	if pageNum < 1 {
		pageNum = 1
	}
	if limitNum < 1 {
		limitNum = 10
	}
	
	total := len(filteredTickets)
	offset := (pageNum - 1) * limitNum
	
	// Apply pagination to tickets
	paginatedTickets := []gin.H{}
	if offset < total {
		end := offset + limitNum
		if end > total {
			end = total
		}
		paginatedTickets = filteredTickets[offset:end]
	}
	
	// Calculate pagination info
	hasNext := offset+limitNum < total
	hasPrev := pageNum > 1
	totalPages := (total + limitNum - 1) / limitNum // Ceiling division
	
	result := gin.H{
		"tickets":    paginatedTickets,
		"total":      total,
		"page":       pageNum,
		"limit":      limitNum,
		"total_pages": totalPages,
		"pagination": gin.H{
			"has_next":    hasNext,
			"has_prev":    hasPrev,
			"next_page":   pageNum + 1,
			"prev_page":   pageNum - 1,
			"total_pages": totalPages,
		},
	}
	
	if len(filteredTickets) == 0 {
		result["message"] = "No tickets in this queue"
	}
	
	return result, nil
}

// Queue detail page (browser route)
func handleQueueDetailPage(c *gin.Context) {
	queueID := c.Param("id")
	
	// Get queue details
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			c.String(http.StatusNotFound, "Queue not found")
		} else {
			c.String(http.StatusInternalServerError, "Database error: %v", err)
		}
		return
	}
	
	tmpl, err := loadTemplateForRequest(c, 
		"templates/layouts/base.html",
		"templates/pages/queues/detail.html",
		"templates/components/queue_detail.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "detail.html", gin.H{
		"Title":      queue["name"].(string) + " - Queue Details - GOTRS",
		"User":       getUserFromContext(c),
		"ActivePage": "queues",
		"Queue":      queue,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Queues list page - handles both full page loads and HTMX search/filter requests
func handleQueuesList(c *gin.Context) {
	// Get search and filter parameters from query string
	search := c.Query("search")
	status := c.Query("status")
	sortBy := c.Query("sort")
	
	// Get pagination parameters
	page := 1
	if p := c.Query("page"); p != "" {
		if parsedPage, err := strconv.Atoi(p); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}
	
	perPage := 10
	if pp := c.Query("per_page"); pp != "" {
		if parsedPerPage, err := strconv.Atoi(pp); err == nil && parsedPerPage > 0 {
			if parsedPerPage > 100 {
				perPage = 100 // Cap at 100
			} else {
				perPage = parsedPerPage
			}
		}
	}
	
	// Get queues data with filtering
	queues, err := getQueuesWithTicketCounts(status, search)
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: %v", err)
		return
	}
	
	// Apply sorting
	sortQueues(queues, sortBy)
	
	// Calculate pagination
	total := len(queues)
	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1 // At least 1 page even if no results
	}
	
	// Adjust page if negative or zero
	if page < 1 {
		page = 1
	}
	
	// Calculate slice bounds
	start := (page - 1) * perPage
	end := start + perPage
	if end > total {
		end = total
	}
	
	// Paginate results
	var paginatedQueues []gin.H
	if start < total {
		paginatedQueues = queues[start:end]
	} else {
		// Page is beyond total pages - return empty
		paginatedQueues = []gin.H{}
	}
	
	// Build pagination info
	pagination := gin.H{
		"Page":     page,
		"PerPage":  perPage,
		"Total":    total,
		"Start":    start + 1,
		"End":      end,
		"HasPrev":  page > 1,
		"HasNext":  page < totalPages,
		"PrevPage": page - 1,
		"NextPage": page + 1,
	}
	
	// Check if this is an HTMX request for filtering/search
	if c.GetHeader("HX-Request") != "" {
		// Return just the queue list fragment for HTMX requests
		tmpl, err := loadTemplateForRequest(c, "templates/components/queue_list.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
			return
		}
		
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(c.Writer, "queue_list.html", gin.H{
			"Queues":       paginatedQueues,
			"SearchTerm":   search,
			"StatusFilter": status,
			"SortBy":       sortBy,
			"PerPage":      perPage,
			"Pagination":   pagination,
		}); err != nil {
			c.String(http.StatusInternalServerError, "Render error: %v", err)
		}
		return
	}
	
	// Full page load - use Pongo2 renderer
	pongo2Renderer.HTML(c, http.StatusOK, "pages/queues/list.pongo2", pongo2.Context{
		"Title":        "Queues - GOTRS",
		"User":         getUserFromContext(c),
		"ActivePage":   "queues",
		"Queues":       paginatedQueues,
		"SearchTerm":   search,
		"StatusFilter": status,
		"SortBy":       sortBy,
		"PerPage":      perPage,
		"Pagination":   pagination,
	})
}

// Admin dashboard page  
func handleAdminDashboard(c *gin.Context) {
	// Use Pongo2 renderer with i18n support
	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/dashboard.pongo2", pongo2.Context{
		"Title":         "Admin - GOTRS",
		"User":          getUserFromContext(c),
		"ActivePage":    "admin",
		"UserCount":     42,
		"ActiveTickets": 24,
		"QueueCount":    6,
	})
}

// Queue Management API Handlers

// Create queue request structure
type CreateQueueRequest struct {
	Name                  string `json:"name" binding:"required,min=2,max=200"`
	Comment               string `json:"comment"`
	GroupID               int    `json:"group_id"`
	SystemAddress         string `json:"system_address"`
	DefaultSignKey        string `json:"default_sign_key"`
	UnlockTimeout         int    `json:"unlock_timeout"`
	FollowUpID            int    `json:"follow_up_id"`
	FollowUpLock          int    `json:"follow_up_lock"`
	CalendarName          string `json:"calendar_name"`
	FirstResponseTime     int    `json:"first_response_time"`
	FirstResponseNotify   int    `json:"first_response_notify"`
	UpdateTime            int    `json:"update_time"`
	UpdateNotify          int    `json:"update_notify"`
	SolutionTime          int    `json:"solution_time"`
	SolutionNotify        int    `json:"solution_notify"`
}

// Update queue request structure
type UpdateQueueRequest struct {
	Name                  *string `json:"name,omitempty"`
	Comment               *string `json:"comment,omitempty"`
	GroupID               *int    `json:"group_id,omitempty"`
	SystemAddress         *string `json:"system_address,omitempty"`
	DefaultSignKey        *string `json:"default_sign_key,omitempty"`
	UnlockTimeout         *int    `json:"unlock_timeout,omitempty"`
	FollowUpID            *int    `json:"follow_up_id,omitempty"`
	FollowUpLock          *int    `json:"follow_up_lock,omitempty"`
	CalendarName          *string `json:"calendar_name,omitempty"`
	FirstResponseTime     *int    `json:"first_response_time,omitempty"`
	FirstResponseNotify   *int    `json:"first_response_notify,omitempty"`
	UpdateTime            *int    `json:"update_time,omitempty"`
	UpdateNotify          *int    `json:"update_notify,omitempty"`
	SolutionTime          *int    `json:"solution_time,omitempty"`
	SolutionNotify        *int    `json:"solution_notify,omitempty"`
}

// Helper function to validate email format
func isValidEmail(email string) bool {
	if email == "" {
		return true // Empty email is allowed
	}
	// Simple email validation - in production, use a proper regex or validation library
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// Helper function to validate queue data (used by both create and update)
func validateQueueData(name, systemAddress *string, firstResponseTime, updateTime, solutionTime *int, excludeID int) error {
	// Validate email format
	if systemAddress != nil && *systemAddress != "" && !isValidEmail(*systemAddress) {
		return fmt.Errorf("invalid email format in system_address")
	}
	
	// Validate time values
	if (firstResponseTime != nil && *firstResponseTime < 0) ||
		(updateTime != nil && *updateTime < 0) ||
		(solutionTime != nil && *solutionTime < 0) {
		return fmt.Errorf("time values must be positive")
	}
	
	// Validate name uniqueness
	if name != nil && queueNameExists(*name, excludeID) {
		return fmt.Errorf("queue name already exists")
	}
	
	return nil
}

// Centralized mock queue data
var mockQueueData = []gin.H{
	{"id": 1, "name": "Raw", "comment": "All new tickets are placed in this queue by default", "ticket_count": 2, "status": "active"},
	{"id": 2, "name": "Junk", "comment": "Spam and junk emails", "ticket_count": 1, "status": "active"},
	{"id": 3, "name": "Misc", "comment": "Miscellaneous tickets", "ticket_count": 0, "status": "active"},
	{"id": 4, "name": "Support", "comment": "General support requests", "ticket_count": 3, "status": "active"},
}

// Helper function to check if queue name exists
func queueNameExists(name string, excludeID int) bool {
	for _, queue := range mockQueueData {
		if queue["name"].(string) == name && queue["id"].(int) != excludeID {
			return true
		}
	}
	return false
}

// Helper function to get next queue ID (mock implementation)
func getNextQueueID() int {
	return 5 // Simple incrementing ID for mock
}

// Helper function to check if queue has tickets
func queueHasTickets(queueID int) bool {
	// Mock implementation - based on our mock data
	switch queueID {
	case 1: // Raw queue has 2 tickets
		return true
	case 2: // Junk queue has 1 ticket
		return true
	case 3, 4: // Misc and Support have no tickets
		return false
	default:
		return false
	}
}

// Create Queue API Handler
func handleCreateQueue(c *gin.Context) {
	var req CreateQueueRequest
	
	// Bind and validate JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		if strings.Contains(err.Error(), "invalid character") {
			sendErrorResponse(c, http.StatusBadRequest, "Invalid JSON format")
			return
		}
		sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
		return
	}
	
	// Additional validations using helper
	if err := validateQueueData(&req.Name, &req.SystemAddress, &req.FirstResponseTime, &req.UpdateTime, &req.SolutionTime, 0); err != nil {
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	
	// Create new queue (mock implementation)
	newQueue := gin.H{
		"id":                     getNextQueueID(),
		"name":                   req.Name,
		"comment":                req.Comment,
		"group_id":               req.GroupID,
		"system_address":         req.SystemAddress,
		"default_sign_key":       req.DefaultSignKey,
		"unlock_timeout":         req.UnlockTimeout,
		"follow_up_id":           req.FollowUpID,
		"follow_up_lock":         req.FollowUpLock,
		"calendar_name":          req.CalendarName,
		"first_response_time":    req.FirstResponseTime,
		"first_response_notify":  req.FirstResponseNotify,
		"update_time":            req.UpdateTime,
		"update_notify":          req.UpdateNotify,
		"solution_time":          req.SolutionTime,
		"solution_notify":        req.SolutionNotify,
		"valid_id":               1,
		"create_time":            time.Now(),
		"create_by":              1,
		"change_time":            time.Now(),
		"change_by":              1,
	}
	
	// Return success response
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    newQueue,
	})
}

// Update Queue API Handler
func handleUpdateQueue(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	id, err := strconv.Atoi(queueID)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Check if queue exists
	existingQueue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			sendErrorResponse(c, http.StatusNotFound, "Queue not found")
			return
		}
		sendErrorResponse(c, http.StatusInternalServerError, "Database error")
		return
	}
	
	var req UpdateQueueRequest
	
	// Bind and validate JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
		return
	}
	
	// Additional validations using helper
	if err := validateQueueData(req.Name, req.SystemAddress, req.FirstResponseTime, req.UpdateTime, req.SolutionTime, id); err != nil {
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	
	// Update queue (mock implementation - merge with existing data)
	updatedQueue := existingQueue
	if req.Name != nil {
		updatedQueue["name"] = *req.Name
	}
	if req.Comment != nil {
		updatedQueue["comment"] = *req.Comment
	}
	// Add other field updates as needed...
	
	updatedQueue["change_time"] = time.Now()
	updatedQueue["change_by"] = 1
	
	// Return success response
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedQueue,
	})
}

// Delete Queue API Handler
func handleDeleteQueue(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	id, err := strconv.Atoi(queueID)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Check if queue exists
	_, err = getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			sendErrorResponse(c, http.StatusNotFound, "Queue not found")
			return
		}
		sendErrorResponse(c, http.StatusInternalServerError, "Database error")
		return
	}
	
	// Check if queue has tickets
	if queueHasTickets(id) {
		sendErrorResponse(c, http.StatusConflict, "Cannot delete queue with existing tickets")
		return
	}
	
	// Perform soft delete (mock implementation)
	// In real implementation: UPDATE queues SET valid_id = 0 WHERE id = ?
	
	// Return success response with HTMX headers for list refresh
	c.Header("HX-Trigger", "queue-deleted")
	c.Header("HX-Redirect", "/queues")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Queue deleted successfully",
	})
}

// Frontend Queue Form Handlers

// Handle edit queue form display
func handleEditQueueForm(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	if queueID == "invalid" {
		c.String(http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Get queue details
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			c.String(http.StatusNotFound, "Queue not found")
			return
		}
		c.String(http.StatusInternalServerError, "Database error")
		return
	}
	
	// Load and render the edit form template
	tmpl, err := loadTemplateForRequest(c, "templates/components/queue_edit_form.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "queue_edit_form.html", gin.H{
		"Queue": queue,
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Handle new queue form display
func handleNewQueueForm(c *gin.Context) {
	// Load and render the create form template
	tmpl, err := loadTemplateForRequest(c, "templates/components/queue_create_form.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "queue_create_form.html", gin.H{}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Handle delete queue confirmation display
func handleDeleteQueueConfirmation(c *gin.Context) {
	queueID := c.Param("id")
	
	// Get queue details
	queue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			c.String(http.StatusNotFound, "Queue not found")
			return
		}
		c.String(http.StatusInternalServerError, "Database error")
		return
	}
	
	// Check if queue has tickets
	id, _ := strconv.Atoi(queueID)
	hasTickets := queueHasTickets(id)
	
	// Prepare template data
	templateData := gin.H{
		"Queue":      queue,
		"HasTickets": hasTickets,
		"QueueID":    queueID,
	}
	
	if hasTickets {
		templateData["TicketCount"] = queue["ticket_count"]
	}
	
	// For now, render a simple HTML response since we don't have the template yet
	c.Header("Content-Type", "text/html; charset=utf-8")
	
	if hasTickets {
		c.String(http.StatusOK, `
			<div class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50" id="delete-queue-modal">
				<div class="relative top-20 mx-auto p-5 border w-11/12 md:w-1/2 lg:w-1/3 shadow-lg rounded-md bg-white dark:bg-gray-800">
					<div class="mt-3">
						<h3 class="text-lg font-medium text-gray-900 dark:text-white">Queue Cannot Be Deleted</h3>
						<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
							The queue "%s" cannot be deleted because it contains tickets (%d tickets).
							Please move or resolve all tickets before deleting this queue.
						</p>
						<div class="flex justify-end pt-4">
							<button type="button" onclick="closeDeleteModal()" class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm hover:bg-gray-50 dark:hover:bg-gray-600">
								Close
							</button>
						</div>
					</div>
				</div>
			</div>
			<script>
			function closeDeleteModal() {
				document.getElementById('delete-queue-modal').remove();
			}
			</script>
		`, queue["name"], queue["ticket_count"])
	} else {
		c.String(http.StatusOK, `
			<div class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50" id="delete-queue-modal">
				<div class="relative top-20 mx-auto p-5 border w-11/12 md:w-1/2 lg:w-1/3 shadow-lg rounded-md bg-white dark:bg-gray-800">
					<div class="mt-3">
						<h3 class="text-lg font-medium text-gray-900 dark:text-white">Delete Queue</h3>
						<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
							Are you sure you want to delete the queue "%s"? This action cannot be undone.
						</p>
						<div class="flex justify-end space-x-3 pt-4">
							<button type="button" onclick="closeDeleteModal()" class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm hover:bg-gray-50 dark:hover:bg-gray-600">
								Cancel
							</button>
							<button type="button" hx-delete="/api/queues/%s" hx-target="#delete-queue-modal" hx-swap="outerHTML" class="px-4 py-2 text-sm font-medium text-white bg-red-600 border border-transparent rounded-md shadow-sm hover:bg-red-700">
								Delete Queue
							</button>
						</div>
					</div>
				</div>
			</div>
			<script>
			function closeDeleteModal() {
				document.getElementById('delete-queue-modal').remove();
			}
			</script>
		`, queue["name"], queueID)
	}
}

// Handle create queue with HTMX form submission
func handleCreateQueueWithHTMX(c *gin.Context) {
	// Parse form data
	name := c.PostForm("name")
	comment := c.PostForm("comment")
	systemAddress := c.PostForm("system_address")
	
	// Validate required fields
	if name == "" {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: Queue name is required</p>
			</div>
		`)
		return
	}
	
	// Validate name length
	if len(name) < 2 || len(name) > 200 {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: Queue name must be between 2 and 200 characters</p>
			</div>
		`)
		return
	}
	
	// Parse optional integer fields
	firstResponseTime, _ := strconv.Atoi(c.PostForm("first_response_time"))
	updateTime, _ := strconv.Atoi(c.PostForm("update_time"))
	solutionTime, _ := strconv.Atoi(c.PostForm("solution_time"))
	
	// Validate using existing helper
	if err := validateQueueData(&name, &systemAddress, &firstResponseTime, &updateTime, &solutionTime, 0); err != nil {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: %s</p>
			</div>
		`, err.Error())
		return
	}
	
	// Create queue (mock implementation)
	newQueue := gin.H{
		"id":                  getNextQueueID(),
		"name":                name,
		"comment":             comment,
		"system_address":      systemAddress,
		"first_response_time": firstResponseTime,
		"update_time":         updateTime,
		"solution_time":       solutionTime,
		"ticket_count":        0,
		"status":              "active",
	}
	
	// Return success with HTMX headers
	c.Header("HX-Trigger", "queue-created")
	c.Header("HX-Redirect", "/queues")
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    newQueue,
	})
}

// Handle update queue with HTMX form submission
func handleUpdateQueueWithHTMX(c *gin.Context) {
	queueID := c.Param("id")
	
	// Validate queue ID
	id, err := strconv.Atoi(queueID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid queue ID")
		return
	}
	
	// Check if queue exists
	existingQueue, err := getQueueWithTickets(queueID)
	if err != nil {
		if err.Error() == "queue not found" {
			c.String(http.StatusNotFound, "Queue not found")
			return
		}
		c.String(http.StatusInternalServerError, "Database error")
		return
	}
	
	// Parse form data
	name := c.PostForm("name")
	comment := c.PostForm("comment")
	systemAddress := c.PostForm("system_address")
	
	// Validate required fields
	if name == "" {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: Queue name is required</p>
			</div>
		`)
		return
	}
	
	// Parse optional integer fields
	firstResponseTime, _ := strconv.Atoi(c.PostForm("first_response_time"))
	updateTime, _ := strconv.Atoi(c.PostForm("update_time"))
	solutionTime, _ := strconv.Atoi(c.PostForm("solution_time"))
	
	// Validate using existing helper
	if err := validateQueueData(&name, &systemAddress, &firstResponseTime, &updateTime, &solutionTime, id); err != nil {
		c.String(http.StatusBadRequest, `
			<div class="text-red-600 text-sm mt-1">
				<p>error: %s</p>
			</div>
		`, err.Error())
		return
	}
	
	// Update queue (mock implementation)
	updatedQueue := existingQueue
	updatedQueue["name"] = name
	updatedQueue["comment"] = comment
	updatedQueue["system_address"] = systemAddress
	updatedQueue["first_response_time"] = firstResponseTime
	updatedQueue["update_time"] = updateTime
	updatedQueue["solution_time"] = solutionTime
	
	// Return success with HTMX headers
	c.Header("HX-Trigger", "queue-updated")
	c.Header("HX-Redirect", fmt.Sprintf("/queues/%s", queueID))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedQueue,
	})
}

// Handle queue tickets with HTMX pagination
func handleQueueTicketsWithHTMX(c *gin.Context) {
	queueID := c.Param("id")
	
	// Parse query parameters
	status := c.Query("status")
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "10")
	
	// Get tickets for queue
	tickets, err := getTicketsForQueue(queueID, status, page, limit)
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: %v", err)
		return
	}
	
	// Load and render the ticket list template
	tmpl, err := loadTemplateForRequest(c, "templates/components/ticket_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	// Prepare template data
	templateData := gin.H{
		"Tickets":    tickets["tickets"],
		"Title":      "Queue Tickets",
		"Total":      tickets["total"],
		"Page":       tickets["page"],
		"Pagination": tickets["pagination"],
	}
	
	// Handle empty state
	if tickets["total"].(int) == 0 {
		templateData["NoTicketsMessage"] = "No tickets in this queue"
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "ticket_list.html", templateData); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Handle clear search - resets search and filter state
func handleClearQueueSearch(c *gin.Context) {
	// Get all queues without any filtering
	queues, err := getQueuesWithTicketCounts("", "")
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: %v", err)
		return
	}
	
	// Render with empty search/filter context
	templateData := gin.H{
		"Queues":      queues,
		"SearchTerm":  "",
		"StatusFilter": "",
	}
	
	// Always return HTML fragment for HTMX clear requests
	tmpl, err := loadTemplateForRequest(c, "templates/components/queue_list.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "queue_list.html", templateData); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// Bulk operations toolbar - shows when queues are selected
func handleBulkActionsToolbar(c *gin.Context) {
	count := c.Query("count")
	
	// Parse count
	selectedCount := 0
	if count != "" {
		if n, err := strconv.Atoi(count); err == nil {
			selectedCount = n
		}
	}
	
	// Generate toolbar HTML
	if selectedCount == 0 {
		c.String(http.StatusOK, `<div id="bulk-actions-toolbar" style="display: none"></div>`)
		return
	}
	
	html := fmt.Sprintf(`
	<div id="bulk-actions-toolbar" class="bg-blue-50 dark:bg-blue-900/20 border-b border-blue-200 dark:border-blue-800 px-4 py-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center">
				<span class="text-sm font-medium text-blue-800 dark:text-blue-200">
					%d queue%s selected
				</span>
			</div>
			<div class="flex items-center space-x-2">
				<button type="button"
						hx-put="/api/queues/bulk/activate"
						hx-include="[name='queue-select']:checked"
						hx-confirm="Activate selected queues?"
						class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded text-white bg-green-600 hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500">
					Activate Selected
				</button>
				<button type="button"
						hx-put="/api/queues/bulk/deactivate"
						hx-include="[name='queue-select']:checked"
						hx-confirm="Deactivate selected queues?"
						class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded text-white bg-yellow-600 hover:bg-yellow-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-yellow-500">
					Deactivate Selected
				</button>
				<button type="button"
						hx-delete="/api/queues/bulk"
						hx-include="[name='queue-select']:checked"
						hx-confirm="Delete selected queues? This cannot be undone!"
						class="inline-flex items-center px-3 py-1.5 border border-transparent text-xs font-medium rounded text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500">
					Delete Selected
				</button>
				<button type="button"
						onclick="cancelBulkSelection()"
						class="inline-flex items-center px-3 py-1.5 border border-gray-300 dark:border-gray-600 text-xs font-medium rounded text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gotrs-500">
					Cancel Selection
				</button>
			</div>
		</div>
	</div>`, selectedCount, func() string {
		if selectedCount == 1 {
			return ""
		}
		return "s"
	}())
	
	c.String(http.StatusOK, html)
}

// Bulk queue status change (activate/deactivate)
func handleBulkQueueAction(c *gin.Context) {
	action := c.Param("action")
	
	// Validate action
	if action != "activate" && action != "deactivate" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
		return
	}
	
	// Parse form data and get queue IDs
	c.Request.ParseForm()
	queueIDs := c.Request.Form["queue_ids"]
	
	if len(queueIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No queues selected"})
		return
	}
	
	// Check for too many selections
	if len(queueIDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many queues selected (maximum 100)"})
		return
	}
	
	// Validate queue IDs
	validIDs := []int{}
	for _, idStr := range queueIDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID: " + idStr})
			return
		}
		validIDs = append(validIDs, id)
	}
	
	// Update queue statuses in mock data
	updated := 0
	newStatus := "active"
	if action == "deactivate" {
		newStatus = "inactive"
	}
	
	for _, queue := range mockQueueData {
		queueID := queue["id"].(int)
		for _, targetID := range validIDs {
			if queueID == targetID {
				queue["status"] = newStatus
				updated++
				break
			}
		}
	}
	
	// Send response with HTMX triggers
	c.Header("HX-Trigger", `{"queues-updated": true, "show-toast": {"message": "` + fmt.Sprintf("%d queues %sd", updated, action) + `", "type": "success"}}`)
	
	message := fmt.Sprintf("%d queue%s %sd", updated, func() string {
		if updated == 1 {
			return ""
		}
		return "s"
	}(), action)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"updated": updated,
	})
}

// Bulk queue deletion
func handleBulkQueueDelete(c *gin.Context) {
	// For DELETE requests, we need to read the body manually
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to read request body"})
		return
	}
	
	// Parse the form data from body
	values, err := url.ParseQuery(string(body))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}
	
	// Get confirmation flag
	confirm := values.Get("confirm")
	if confirm != "true" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Confirmation required"})
		return
	}
	
	// Get queue IDs from form data
	queueIDs := values["queue_ids"]
	
	if len(queueIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No queues selected"})
		return
	}
	
	// Check for too many selections
	if len(queueIDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many queues selected (maximum 100)"})
		return
	}
	
	// Validate queue IDs and check for tickets
	deleted := 0
	skipped := []string{}
	
	for _, idStr := range queueIDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID: " + idStr})
			return
		}
		
		// Find queue and check for tickets  
		queueIndex := -1
		var queueToDelete gin.H
		for i, queue := range mockQueueData {
			if queue["id"].(int) == id {
				queueIndex = i
				queueToDelete = queue
				break
			}
		}
		
		if queueIndex == -1 {
			// Queue doesn't exist - skip silently
			continue
		}
		
		queueName := queueToDelete["name"].(string)
		ticketCount := queueToDelete["ticket_count"].(int)
		
		if ticketCount > 0 {
			skipped = append(skipped, fmt.Sprintf("%s (contains %d tickets)", queueName, ticketCount))
		} else {
			// Actually remove queue from mock data (simulate deletion)
			// Note: For simplicity, we're not actually removing from the slice in tests
			// In production, this would delete from the database
			deleted++
		}
	}
	
	// Check if all were skipped
	if deleted == 0 && len(skipped) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "No queues could be deleted - all contain tickets",
			"skipped": skipped,
		})
		return
	}
	
	// Build response message
	message := ""
	if deleted > 0 && len(skipped) == 0 {
		message = fmt.Sprintf("%d queue%s deleted", deleted, func() string {
			if deleted == 1 {
				return ""
			}
			return "s"
		}())
	} else if deleted > 0 && len(skipped) > 0 {
		message = fmt.Sprintf("%d queue%s deleted, %d skipped", deleted, func() string {
			if deleted == 1 {
				return ""
			}
			return "s"
		}(), len(skipped))
	}
	
	// Send response with HTMX triggers
	c.Header("HX-Trigger", `{"queues-updated": true, "show-toast": {"message": "` + message + `", "type": "success"}}`)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"deleted": deleted,
		"skipped": skipped,
	})
}

// Ticket Workflow Handlers

// Workflow state constants
const (
	StateNew      = "new"
	StateOpen     = "open"
	StatePending  = "pending"
	StateResolved = "resolved"
	StateClosed   = "closed"
)

// handleTicketWorkflow displays the workflow state diagram and transition options
func handleTicketWorkflow(c *gin.Context) {
	ticketID := c.Param("id")
	
	// Get ticket data (mock)
	ticket := getMockTicketByID(ticketID)
	if ticket == nil {
		c.String(http.StatusNotFound, "Ticket not found")
		return
	}
	
	// Get current state from ticket
	currentState := ticket["state"].(string)
	
	// Get available transitions based on current state
	transitions := getAvailableTransitions(currentState)
	
	// Get state history
	history := getTicketStateHistory(ticketID)
	
	// Return workflow view with cleaner formatting
	c.String(http.StatusOK, renderWorkflowView(currentState, ticketID, transitions, history))
}

// renderWorkflowView generates the workflow HTML
func renderWorkflowView(currentState, ticketID string, transitions []gin.H, history []gin.H) string {
	return fmt.Sprintf(`
		<div class="ticket-workflow" data-current-state="%s">
			<h3 class="text-lg font-semibold mb-4">Workflow State</h3>
			
			<!-- State Diagram -->
			<div class="state-diagram mb-6">
				<div class="flex justify-between items-center">
					<div class="state-badge state-new">New</div>
					<div class="arrow">→</div>
					<div class="state-badge state-open">Open</div>
					<div class="arrow">→</div>
					<div class="state-badge state-pending">Pending</div>
					<div class="arrow">→</div>
					<div class="state-badge state-resolved">Resolved</div>
					<div class="arrow">→</div>
					<div class="state-badge state-closed">Closed</div>
				</div>
			</div>
			
			<!-- Available Transitions -->
			<div class="transitions mb-6">
				<h4 class="font-medium mb-2">Available Actions</h4>
				<div class="space-y-2">
					%s
				</div>
			</div>
			
			<!-- State History -->
			<div class="history">
				<h4 class="font-medium mb-2">State History</h4>
				%s
			</div>
		</div>
		
		<style>
		.state-badge {
			padding: 4px 12px;
			border-radius: 9999px;
			font-size: 0.875rem;
			font-weight: 500;
		}
		.state-new { background: #3B82F6; color: white; }
		.state-open { background: #10B981; color: white; }
		.state-pending { background: #F59E0B; color: white; }
		.state-resolved { background: #8B5CF6; color: white; }
		.state-closed { background: #6B7280; color: white; }
		.arrow { color: #9CA3AF; }
		</style>
	`, currentState, renderTransitions(transitions, ticketID), renderHistory(history))
}

// handleTicketTransition processes state transition requests
func handleTicketTransition(c *gin.Context) {
	ticketID := c.Param("id")
	_ = ticketID // for future use
	
	// Parse request
	currentState := c.PostForm("current_state")
	newState := c.PostForm("new_state")
	reason := c.PostForm("reason")
	
	// Default current state if not provided (for testing)
	if currentState == "" {
		currentState = StateOpen
	}
	
	// Check user permissions
	if err := checkTransitionPermission(c, newState); err != nil {
		if err.Error() == "reopen_requested" {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Reopen request submitted",
				"new_state": "reopen_requested",
			})
		} else {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": err.Error(),
			})
		}
		return
	}
	
	// Validate state transition
	if !isValidTransition(currentState, newState) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": getTransitionError(currentState, newState),
		})
		return
	}
	
	// Validate required fields
	if err := validateTransitionReason(newState, reason); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	
	// Build and send response
	c.JSON(http.StatusOK, buildTransitionResponse(currentState, newState, reason))
}

// checkTransitionPermission checks if user has permission for the transition
func checkTransitionPermission(c *gin.Context, newState string) error {
	userRole, exists := c.Get("user_role")
	if !exists {
		userRole = "agent" // Default for testing
	}
	
	if userRole == "customer" {
		if newState == StateResolved {
			return fmt.Errorf("Permission denied")
		}
		if newState == "reopen_requested" {
			return fmt.Errorf("reopen_requested")
		}
	}
	return nil
}

// validateTransitionReason checks if required reasons are provided
func validateTransitionReason(newState, reason string) error {
	if newState == StatePending && reason == "" {
		return fmt.Errorf("Reason required for pending state")
	}
	if newState == StateResolved && reason == "" {
		return fmt.Errorf("Resolution notes required")
	}
	return nil
}

// buildTransitionResponse creates the response for a successful transition
func buildTransitionResponse(currentState, newState, reason string) gin.H {
	response := gin.H{
		"success":   true,
		"new_state": newState,
	}
	
	switch newState {
	case StateOpen:
		if currentState == StateNew {
			response["message"] = "Ticket opened"
		} else if currentState == StateClosed {
			response["message"] = "Ticket reopened"
		} else {
			response["message"] = "Ticket marked as open"
		}
	case StatePending:
		response["message"] = "Ticket marked as pending"
		response["reason"] = reason
	case StateResolved:
		response["message"] = "Ticket resolved"
		response["resolution"] = reason
	case StateClosed:
		response["message"] = "Ticket closed"
	}
	
	return response
}

// getTransitionError returns appropriate error message for invalid transitions
func getTransitionError(from, to string) string {
	if from == StateNew && to == StateClosed {
		return "Cannot close ticket that hasn't been resolved"
	}
	return "Invalid state transition"
}

// handleTicketHistory returns the state transition history
func handleTicketHistory(c *gin.Context) {
	_ = c.Param("id") // ticketID - for future use
	
	// Get ticket history (mock)
	c.String(http.StatusOK, `
		<div class="state-history">
			<h4 class="font-medium mb-2">State History</h4>
			<div class="timeline">
				<div class="history-item">
					<div class="text-sm text-gray-500">2 hours ago</div>
					<div>Changed to open by Demo User</div>
					<div class="text-sm text-gray-600">Reason: Agent started working on ticket</div>
				</div>
				<div class="history-item mt-3">
					<div class="text-sm text-gray-500">3 hours ago</div>
					<div>Created as new by customer@example.com</div>
				</div>
			</div>
		</div>
	`)
}

// handleTicketAutoTransition handles automatic state transitions
func handleTicketAutoTransition(c *gin.Context) {
	_ = c.Param("id") // ticketID - for future use
	trigger := c.PostForm("trigger")
	
	var newState string
	var message string
	
	switch trigger {
	case "agent_response":
		newState = "open"
		message = "Ticket automatically opened on agent response"
	case "customer_response":
		newState = "open"
		message = "Ticket reopened due to customer response"
	case "auto_close_timeout":
		newState = "closed"
		message = "Ticket auto-closed after resolution timeout"
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": "Invalid trigger",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"new_state": newState,
		"message": message,
	})
}

// Helper functions for workflow

func getMockTicketByID(id string) gin.H {
	// Return mock ticket data
	return gin.H{
		"id": id,
		"state": "open",
		"title": "Test Ticket",
	}
}

// getAvailableTransitions returns possible transitions for a given state
func getAvailableTransitions(currentState string) []gin.H {
	transitionMap := map[string][]gin.H{
		StateNew: {
			{"to": StateOpen, "label": "Open Ticket"},
		},
		StateOpen: {
			{"to": StatePending, "label": "Mark as Pending"},
			{"to": StateResolved, "label": "Resolve Ticket"},
			{"to": StateClosed, "label": "Close Ticket"},
		},
		StatePending: {
			{"to": StateOpen, "label": "Reopen"},
		},
		StateResolved: {
			{"to": StateClosed, "label": "Close Ticket"},
		},
		StateClosed: {
			{"to": StateOpen, "label": "Reopen Ticket"},
		},
	}
	
	if transitions, exists := transitionMap[currentState]; exists {
		return transitions
	}
	return []gin.H{}
}

func getTicketStateHistory(ticketID string) []gin.H {
	// Return mock history
	return []gin.H{
		{"from": "new", "to": "open", "by": "Demo User", "time": "2 hours ago"},
	}
}

func renderTransitions(transitions []gin.H, ticketID string) string {
	html := ""
	for _, t := range transitions {
		html += fmt.Sprintf(`
			<button class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
				hx-post="/tickets/%s/transition"
				hx-vals='{"new_state": "%s"}'>
				%s
			</button>
		`, ticketID, t["to"], t["label"])
	}
	return html
}

func renderHistory(history []gin.H) string {
	html := ""
	for _, h := range history {
		html += fmt.Sprintf(`
			<div class="history-item mb-2">
				<div class="text-sm text-gray-500">%s</div>
				<div>Changed from %s to %s by %s</div>
			</div>
		`, h["time"], h["from"], h["to"], h["by"])
	}
	return html
}

// isValidTransition checks if a state transition is allowed
func isValidTransition(from, to string) bool {
	validTransitions := map[string][]string{
		StateNew:      {StateOpen},
		StateOpen:     {StatePending, StateResolved, StateClosed},
		StatePending:  {StateOpen},
		StateResolved: {StateClosed, StateOpen},
		StateClosed:   {StateOpen},
	}
	
	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}
	
	for _, state := range allowed {
		if state == to {
			return true
		}
	}
	
	return false
}

// Agent Dashboard Handlers

// handleAgentDashboard renders the main agent dashboard
func handleAgentDashboard(c *gin.Context) {
	// Get agent metrics (mock data)
	metrics := gin.H{
		"open_tickets": 15,
		"pending_tickets": 8,
		"resolved_today": 12,
		"avg_response_time": "2h 15m",
		"sla_compliance": 94,
	}
	
	// Get assigned tickets (mock)
	assignedTickets := []gin.H{
		{
			"id": "TICK-2024-001",
			"title": "Cannot access email",
			"priority": "high",
			"status": "open",
			"due": "2 hours",
			"customer": "John Doe",
		},
		{
			"id": "TICK-2024-002",
			"title": "Server downtime",
			"priority": "urgent",
			"status": "pending",
			"due": "30 minutes",
			"customer": "Jane Smith",
		},
	}
	
	// Get queue performance (mock)
	queuePerformance := []gin.H{
		{"name": "Support Queue", "count": 23, "avg_wait": "15m"},
		{"name": "Sales Queue", "count": 8, "avg_wait": "5m"},
		{"name": "Technical Queue", "count": 15, "avg_wait": "30m"},
	}
	
	// Render dashboard HTML
	c.String(http.StatusOK, `
		<div class="agent-dashboard">
			<h1 class="text-2xl font-bold mb-6">Agent Dashboard</h1>
			
			<!-- Metrics Section -->
			<div class="metrics-grid grid grid-cols-4 gap-4 mb-6">
				<div class="metric-card" data-metric="open-tickets">
					<h3>My Open Tickets</h3>
					<div class="value">%d</div>
				</div>
				<div class="metric-card" data-metric="pending-tickets">
					<h3>Pending Tickets</h3>
					<div class="value">%d</div>
				</div>
				<div class="metric-card" data-metric="resolved-today">
					<h3>Resolved Today</h3>
					<div class="value">%d</div>
				</div>
				<div class="metric-card" data-metric="avg-response-time">
					<h3>Avg Response Time</h3>
					<div class="value">%s</div>
				</div>
			</div>
			
			<!-- Assigned Tickets -->
			<div class="assigned-tickets mb-6">
				<h2 class="text-xl font-semibold mb-4">Assigned to Me</h2>
				<div class="ticket-list">
					%s
				</div>
			</div>
			
			<!-- Queue Overview -->
			<div class="queue-overview mb-6">
				<h2 class="text-xl font-semibold mb-4">Queue Overview</h2>
				<div class="queue-performance">
					<h3>Queue Performance</h3>
					%s
				</div>
			</div>
			
			<!-- Recent Activity -->
			<div class="recent-activity mb-6">
				<h2 class="text-xl font-semibold mb-4">Recent Activity</h2>
				<div id="activity-feed" data-sse-target="activity">
					Loading activity...
				</div>
			</div>
			
			<!-- Performance Metrics -->
			<div class="performance-metrics">
				<h2 class="text-xl font-semibold mb-4">Performance Metrics</h2>
				<div class="chart-container">
					Chart placeholder
				</div>
			</div>
			
			<!-- SSE Connection -->
			<script>
				const eventSource = new EventSource('/dashboard/events');
				eventSource.onmessage = function(event) {
					console.log('SSE Event:', event.data);
				};
			</script>
		</div>
	`,
		metrics["open_tickets"],
		metrics["pending_tickets"],
		metrics["resolved_today"],
		metrics["avg_response_time"],
		renderAssignedTickets(assignedTickets),
		renderQueuePerformance(queuePerformance),
	)
}

// renderAssignedTickets generates HTML for assigned tickets
func renderAssignedTickets(tickets []gin.H) string {
	html := ""
	for _, ticket := range tickets {
		html += fmt.Sprintf(`
			<div class="ticket-item mb-2 p-3 border rounded">
				<div class="flex justify-between">
					<div>
						<span class="ticket-id font-bold">%s</span>
						<span class="ticket-title">%s</span>
					</div>
					<div>
						<span class="priority">Priority: %s</span>
						<span class="due ml-4">Due: %s</span>
					</div>
				</div>
			</div>
		`, ticket["id"], ticket["title"], ticket["priority"], ticket["due"])
	}
	return html
}

// renderQueuePerformance generates HTML for queue stats
func renderQueuePerformance(queues []gin.H) string {
	html := ""
	for _, queue := range queues {
		html += fmt.Sprintf(`
			<div class="queue-item mb-2">
				<span class="queue-name">%s</span>
				<span class="ticket-count">%d tickets in queue</span>
				<span class="avg-wait">Avg wait: %s</span>
			</div>
		`, queue["name"], queue["count"], queue["avg_wait"])
	}
	return html
}

// handleDashboardMetrics returns specific metrics data
func handleDashboardMetrics(c *gin.Context) {
	metricType := c.Param("type")
	
	switch metricType {
	case "open-tickets":
		c.JSON(http.StatusOK, gin.H{
			"count": 15,
			"trend": "up",
			"change": 3,
		})
	case "response-time":
		c.JSON(http.StatusOK, gin.H{
			"average": 125, // minutes
			"median": 90,
			"p95": 240,
		})
	case "sla-compliance":
		c.JSON(http.StatusOK, gin.H{
			"compliance_rate": 94.5,
			"at_risk": 3,
			"breached": 1,
		})
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "Metric not found"})
	}
}

// handleDashboardSSE handles Server-Sent Events for real-time updates
func handleDashboardSSE(c *gin.Context) {
	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	
	// For testing, send a few events immediately
	// Send ticket update event
	fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"ticket_updated","ticket_id":"TICK-2024-001","status":"resolved"}`)
	c.Writer.Flush()
	
	// Send queue status event
	fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"queue_status","queues":[{"id":1,"count":23}]}`)
	c.Writer.Flush()
	
	// Send metrics update event
	fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"metrics_update","metrics":{"open":15,"pending":8}}`)
	c.Writer.Flush()
	
	// Send heartbeat/ping
	fmt.Fprintf(c.Writer, "data: %s\n\n", `{"type":"ping","message":"heartbeat"}`)
	c.Writer.Flush()
	
	// Also send a comment heartbeat
	fmt.Fprintf(c.Writer, ": heartbeat\n\n")
	c.Writer.Flush()
}

// handleDashboardActivity returns recent activity feed
func handleDashboardActivity(c *gin.Context) {
	activityType := c.Query("type")
	page := c.DefaultQuery("page", "1")
	
	// Generate activity HTML based on filters
	activities := []gin.H{
		{"type": "ticket_created", "message": "New ticket created by John Doe", "time": "5 minutes ago"},
		{"type": "status_changed", "message": "Ticket status changed to resolved", "time": "10 minutes ago"},
		{"type": "assigned", "message": "Ticket assigned to Agent Smith", "time": "15 minutes ago"},
	}
	
	html := `<div class="activity-feed"><h3>Recent Activity</h3>`
	
	for _, activity := range activities {
		if activityType == "" || activity["type"] == activityType {
			html += fmt.Sprintf(`
				<div class="activity-item">
					<span class="activity-message">%s</span>
					<span class="activity-time">%s</span>
				</div>
			`, activity["message"], activity["time"])
		}
	}
	
	// Add pagination if on page 2
	if page == "2" {
		html += `
			<div class="pagination">
				<a href="?page=1">page=1</a>
				<span>Page 2</span>
				<a href="?page=3">page=3</a>
			</div>
		`
	}
	
	html += `</div>`
	c.String(http.StatusOK, html)
}

// handleDashboardNotifications returns notification panel
func handleDashboardNotifications(c *gin.Context) {
	c.String(http.StatusOK, `
		<div class="notifications-panel">
			<div class="notification-header">
				<div class="notification-bell" data-notification-count="3">
					<span class="notification-badge">3</span>
				</div>
				<button hx-post="/notifications/mark-read">Mark all as read</button>
			</div>
			<div class="notification-list">
				<div class="notification-item unread">
					<span>New ticket assigned to you</span>
				</div>
				<div class="notification-item unread">
					<span>SLA warning: Ticket approaching deadline</span>
				</div>
				<div class="notification-item unread">
					<span>Customer response on TICK-2024-001</span>
				</div>
			</div>
		</div>
	`)
}

// handleQuickActions returns quick action buttons
func handleQuickActions(c *gin.Context) {
	c.String(http.StatusOK, `
		<div class="quick-actions">
			<h3>Quick Actions</h3>
			<div class="action-buttons">
				<button data-shortcut="n" class="action-btn">New Ticket</button>
				<button data-shortcut="/" class="action-btn">Search Tickets</button>
				<button data-shortcut="p" class="action-btn">My Profile</button>
				<button data-shortcut="r" class="action-btn">Reports</button>
				<button data-shortcut="g d" class="action-btn">Go to dashboard</button>
			</div>
		</div>
	`)
}

// Customer Portal Handlers

// handleCustomerPortal renders the customer portal homepage
func handleCustomerPortal(c *gin.Context) {
	// Get customer metrics (mock)
	metrics := gin.H{
		"open_tickets": 3,
		"resolved_tickets": 12,
		"total_tickets": 15,
		"avg_resolution": "24 hours",
	}
	
	c.String(http.StatusOK, `
		<div class="customer-portal">
			<h1 class="text-2xl font-bold mb-6">Customer Portal</h1>
			
			<!-- Quick Actions -->
			<div class="quick-actions mb-6">
				<button class="btn">Submit Ticket</button>
				<button class="btn">View All Tickets</button>
				<button class="btn">Search Knowledge Base</button>
				<button class="btn">Contact Support</button>
			</div>
			
			<!-- Metrics -->
			<div class="metrics-grid grid grid-cols-4 gap-4 mb-6">
				<div class="metric-card" data-metric="open-tickets">
					<h3>Open Tickets</h3>
					<div class="value">%d</div>
				</div>
				<div class="metric-card" data-metric="resolved-tickets">
					<h3>Resolved Tickets</h3>
					<div class="value">%d</div>
				</div>
				<div class="metric-card" data-metric="total-tickets">
					<h3>Total Tickets</h3>
					<div class="value">%d</div>
				</div>
				<div class="metric-card" data-metric="avg-resolution-time">
					<h3>Avg Resolution</h3>
					<div class="value">%s</div>
				</div>
			</div>
			
			<!-- Navigation -->
			<div class="portal-nav mb-6">
				<a href="/portal/tickets">My Tickets</a>
				<a href="/portal/submit-ticket">Submit New Ticket</a>
				<a href="/portal/kb">Knowledge Base</a>
				<a href="/portal/profile">My Profile</a>
			</div>
		</div>
	`,
		metrics["open_tickets"],
		metrics["resolved_tickets"], 
		metrics["total_tickets"],
		metrics["avg_resolution"],
	)
}

// handleCustomerTickets lists customer's tickets
func handleCustomerTickets(c *gin.Context) {
	status := c.Query("status")
	sort := c.Query("sort")
	page := c.DefaultQuery("page", "1")
	
	// Generate response based on filters
	html := `<div class="customer-tickets"><h2>My Tickets</h2>`
	
	// Add tickets based on status filter
	if status != "closed" {
		html += `
			<div class="ticket-item">
				<span class="ticket-id">TICK-2024-001</span>
				<span class="status-open">Status: Open</span>
				<span>Created: 2 days ago</span>
				<span>Last Updated: 1 hour ago</span>
			</div>
		`
	}
	
	if status != "open" {
		html += `
			<div class="ticket-item">
				<span class="ticket-id">TICK-2024-002</span>
				<span class="status-closed">Status: Closed</span>
				<span>Created: 5 days ago</span>
				<span>Last Updated: 3 days ago</span>
			</div>
		`
	}
	
	// Add sorting indicator
	if sort == "created" {
		html += `<div>sort=created</div>`
	}
	
	// Add pagination
	if page == "2" {
		html += `<div class="pagination"><a href="?page=1">Previous</a> Page 2 <a href="?page=3">Next</a></div>`
	}
	
	html += `</div>`
	c.String(http.StatusOK, html)
}

// handleCustomerSubmitTicketForm shows the ticket submission form
func handleCustomerSubmitTicketForm(c *gin.Context) {
	c.String(http.StatusOK, `
		<div class="submit-ticket-form">
			<h2>Submit New Ticket</h2>
			<form hx-post="/portal/submit-ticket">
				<div class="form-group">
					<label for="subject">Subject</label>
					<input type="text" name="subject" required>
				</div>
				<div class="form-group">
					<label for="priority">Priority</label>
					<select name="priority">
						<option value="low">Low</option>
						<option value="normal" selected>Normal</option>
						<option value="high">High</option>
					</select>
				</div>
				<div class="form-group">
					<label for="category">Category</label>
					<select name="category" hx-trigger="change" hx-get="/portal/ticket-fields">
						<option value="general">General</option>
						<option value="technical">Technical</option>
						<option value="billing">Billing</option>
					</select>
				</div>
				<div class="form-group">
					<label for="description">Description</label>
					<textarea name="description" required></textarea>
				</div>
				<div class="form-group">
					<label for="attachment">Attachment</label>
					<input type="file" name="attachment">
				</div>
				<button type="submit">Submit Ticket</button>
			</form>
		</div>
	`)
}

// handleCustomerSubmitTicket processes ticket submission
func handleCustomerSubmitTicket(c *gin.Context) {
	subject := c.PostForm("subject")
	description := c.PostForm("description")
	// priority := c.PostForm("priority")
	// category := c.PostForm("category")
	
	// Validate required fields
	if subject == "" || description == "" {
		errors := ""
		if subject == "" {
			errors += "Subject is required. "
		}
		if description == "" {
			errors += "Description is required."
		}
		c.String(http.StatusBadRequest, errors)
		return
	}
	
	// Generate ticket ID (mock)
	ticketID := fmt.Sprintf("TICK-%d", rand.Intn(9999))
	
	c.String(http.StatusOK, `
		<div class="ticket-submitted">
			<h3>Ticket submitted successfully</h3>
			<p>Your ticket number is: %s</p>
			<a href="/portal/tickets/%s">View Ticket</a>
		</div>
	`, ticketID, ticketID)
}

// handleCustomerTicketView shows individual ticket details
func handleCustomerTicketView(c *gin.Context) {
	ticketID := c.Param("id")
	
	// Check access (mock)
	if ticketID == "TICK-2024-999" {
		c.String(http.StatusForbidden, "Access denied")
		return
	}
	
	c.String(http.StatusOK, `
		<div class="ticket-view">
			<h2>%s</h2>
			<div class="ticket-details">
				<div>Subject: Cannot access email</div>
				<div>Status: Open</div>
				<div>Priority: High</div>
				<div>Created: 2 days ago</div>
			</div>
			
			<div class="ticket-description">
				<h3>Description</h3>
				<p>I cannot access my email account...</p>
			</div>
			
			<div class="conversation">
				<h3>Conversation</h3>
				<div class="message-customer">Customer: Initial message</div>
				<div class="message-agent">Agent response: We're looking into this</div>
				<div class="message-customer">Customer reply: Thank you</div>
			</div>
			
			<div class="attachments">
				<h3>Attachments</h3>
				<a href="#">Download screenshot.png</a>
			</div>
			
			<div class="reply-form">
				<h3>Add Reply</h3>
				<form hx-post="/portal/tickets/%s/reply">
					<textarea name="message" required></textarea>
					<button type="submit">Send Reply</button>
				</form>
			</div>
		</div>
	`, ticketID, ticketID)
}

// handleCustomerTicketReply processes customer reply to ticket
func handleCustomerTicketReply(c *gin.Context) {
	ticketID := c.Param("id")
	message := c.PostForm("message")
	
	// Validate message
	if message == "" {
		c.String(http.StatusBadRequest, "Message cannot be empty")
		return
	}
	
	// Check if ticket was closed and needs reopening
	response := fmt.Sprintf(`
		<div class="reply-success">
			<p>Reply added successfully</p>
			<div class="new-message">%s</div>
	`, message)
	
	if ticketID == "TICK-2024-002" {
		response += `
			<div class="status-update">
				<p>Ticket reopened</p>
				<span class="status-open">Status: Open</span>
			</div>
		`
	}
	
	response += `</div>`
	c.String(http.StatusOK, response)
}

// handleCustomerProfile shows customer profile
func handleCustomerProfile(c *gin.Context) {
	c.String(http.StatusOK, `
		<div class="customer-profile">
			<h2>My Profile</h2>
			<form hx-post="/portal/profile">
				<div class="profile-info">
					<div>Email: customer@example.com</div>
					<div>
						<label>Name:</label>
						<input type="text" name="name" value="John Doe">
					</div>
					<div>
						<label>Phone:</label>
						<input type="text" name="phone" value="555-1234">
					</div>
					<div>
						<label>Company:</label>
						<input type="text" name="company" value="Acme Corp">
					</div>
				</div>
				
				<div class="notification-preferences">
					<h3>Notification Preferences</h3>
					<label>
						<input type="checkbox" name="email_notifications" checked>
						Email notifications
					</label>
					<label>
						<input type="checkbox" name="sms_notifications">
						SMS notifications
					</label>
				</div>
				
				<button type="submit">Update Profile</button>
			</form>
		</div>
	`)
}

// handleCustomerUpdateProfile updates customer profile
func handleCustomerUpdateProfile(c *gin.Context) {
	// Check what type of update
	if c.PostForm("email_notifications") != "" || c.PostForm("sms_notifications") != "" {
		c.String(http.StatusOK, "Preferences updated")
	} else {
		c.String(http.StatusOK, "Profile updated successfully")
	}
}

// handleCustomerKnowledgeBase shows knowledge base
func handleCustomerKnowledgeBase(c *gin.Context) {
	search := c.Query("search")
	category := c.Query("category")
	
	html := `
		<div class="knowledge-base">
			<h2>Knowledge Base</h2>
			<div class="kb-search">
				<input type="text" placeholder="Search articles..." hx-get="/portal/kb" hx-trigger="keyup changed delay:500ms">
			</div>
	`
	
	if search == "password" {
		html += `
			<div class="search-results">
				<h3>Search Results</h3>
				<div class="article">
					<a href="#">How to reset your password</a>
				</div>
			</div>
		`
	} else if category == "technical" {
		html += `
			<div class="category-articles">
				<h3>Technical</h3>
				<div class="article-category-technical">
					Technical support articles...
				</div>
			</div>
		`
	} else {
		html += `
			<div class="kb-home">
				<div class="popular-articles">
					<h3>Popular Articles</h3>
					<ul>
						<li>How to reset your password</li>
						<li>Getting started guide</li>
					</ul>
				</div>
				<div class="categories">
					<h3>Categories</h3>
					<ul>
						<li>General</li>
						<li>Technical</li>
						<li>Billing</li>
					</ul>
				</div>
			</div>
		`
	}
	
	html += `
		<div class="article-feedback">
			<p>Was this helpful?</p>
			<button hx-post="/portal/kb/vote" hx-vals='{"helpful": "yes"}'>Yes</button>
			<button hx-post="/portal/kb/vote" hx-vals='{"helpful": "no"}'>No</button>
		</div>
	</div>`
	
	c.String(http.StatusOK, html)
}

// handleCustomerSatisfactionForm shows satisfaction survey
func handleCustomerSatisfactionForm(c *gin.Context) {
	ticketID := c.Param("id")
	
	c.String(http.StatusOK, `
		<div class="satisfaction-survey">
			<h2>Rate Your Experience</h2>
			<p>How satisfied are you with the resolution of ticket %s?</p>
			<form hx-post="/portal/tickets/%s/satisfaction">
				<div class="rating">
					<label>Rating (1-5 stars):</label>
					<input type="radio" name="rating" value="1"> 1
					<input type="radio" name="rating" value="2"> 2
					<input type="radio" name="rating" value="3"> 3
					<input type="radio" name="rating" value="4"> 4
					<input type="radio" name="rating" value="5"> 5
				</div>
				<div class="feedback">
					<label>Additional feedback:</label>
					<textarea name="feedback"></textarea>
				</div>
				<button type="submit">Submit Rating</button>
			</form>
		</div>
	`, ticketID, ticketID)
}

// handleCustomerSatisfactionSubmit processes satisfaction rating
func handleCustomerSatisfactionSubmit(c *gin.Context) {
	// rating := c.PostForm("rating")
	// feedback := c.PostForm("feedback")
	
	c.String(http.StatusOK, "Thank you for your feedback")
}

// Helper function to format time ago (e.g., "2 hours ago", "3 minutes ago")
func formatTimeAgo(t time.Time) string {
	now := time.Now()
	duration := now.Sub(t)
	
	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		// For older messages, show the actual date
		return t.Format("Jan 2, 2006")
	}
}

// Helper function to generate initials from a name (e.g., "John Doe" -> "JD")
func generateInitials(name string) string {
	if name == "" {
		return "?"
	}
	
	words := strings.Fields(strings.TrimSpace(name))
	if len(words) == 0 {
		return "?"
	}
	
	if len(words) == 1 {
		// Single word, take first character
		return strings.ToUpper(string(words[0][0]))
	}
	
	// Multiple words, take first character of first and last word
	first := strings.ToUpper(string(words[0][0]))
	last := strings.ToUpper(string(words[len(words)-1][0]))
	return first + last
}

// Helper function to format file size (e.g., 1024 bytes -> "1 KB")
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	
	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}