package api

import (
	"net/http"
	
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
)

// handleAdminLookups shows the admin page for managing lookup values
func handleAdminLookups(c *gin.Context) {
	// Check if user has admin role
	// In production, this should check actual user permissions
	
	// Get dynamic form data from lookup service
	lookupService := GetLookupService()
	formData := lookupService.GetTicketFormData()
	
	tmpl, err := loadTemplate(
		"templates/layouts/base.html",
		"templates/pages/admin/lookups.html",
	)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(c.Writer, "lookups.html", gin.H{
		"Title":      "Manage Lookups - GOTRS Admin",
		"Queues":     formData.Queues,
		"Priorities": formData.Priorities,
		"Types":      formData.Types,
		"Statuses":   formData.Statuses,
		"User":       getUserFromContext(c),
		"ActivePage": "admin",
	}); err != nil {
		c.String(http.StatusInternalServerError, "Render error: %v", err)
	}
}

// handleGetQueues returns list of queues as JSON
func handleGetQueues(c *gin.Context) {
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formData.Queues,
	})
}

// handleGetPriorities returns list of priorities as JSON
func handleGetPriorities(c *gin.Context) {
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formData.Priorities,
	})
}

// handleGetTypes returns list of ticket types as JSON
func handleGetTypes(c *gin.Context) {
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formData.Types,
	})
}

// handleGetStatuses returns list of ticket statuses as JSON
func handleGetStatuses(c *gin.Context) {
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formData.Statuses,
	})
}

// handleGetFormData returns all form data (queues, priorities, types, statuses) as JSON
func handleGetFormData(c *gin.Context) {
	lookupService := GetLookupService()
	lang := middleware.GetLanguage(c)
	formData := lookupService.GetTicketFormDataWithLang(lang)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formData,
	})
}

// handleInvalidateLookupCache forces a refresh of the lookup cache
func handleInvalidateLookupCache(c *gin.Context) {
	// Check if user has admin role
	// In production, this should check actual user permissions
	userRole := c.GetString("user_role")
	if userRole != "Admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Admin access required",
		})
		return
	}
	
	lookupService := GetLookupService()
	lookupService.InvalidateCache()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Lookup cache invalidated successfully",
	})
}