//go:build !graphql
package api

import (
	"net/http"
	
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
)

// handleAdminLookups is already defined in htmx_routes.go for templates

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

    // If DB is not available, LookupService returns empty defaults safely
    // But guard against nil repo path panics by ensuring service exists
    if lookupService == nil {
        c.JSON(http.StatusOK, gin.H{"success": true, "data": []any{}})
        return
    }

    formData := lookupService.GetTicketFormDataWithLang(lang)
    if formData == nil {
        c.JSON(http.StatusOK, gin.H{"success": true, "data": []any{}})
        return
    }

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
