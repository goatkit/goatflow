package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// Get or create the singleton lookup repository
var lookupRepo repository.LookupRepository

func GetLookupRepository() repository.LookupRepository {
	if lookupRepo == nil {
		lookupRepo = repository.NewMemoryLookupRepository()
	}
	return lookupRepo
}

// Queue CRUD handlers
func handleCreateLookupQueue(c *gin.Context) {
	// Check admin permission
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	var queue models.QueueInfo
	if err := c.ShouldBindJSON(&queue); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	repo := GetLookupRepository()
	if err := repo.CreateQueue(c.Request.Context(), &queue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Log the change
	logLookupChange(c, "queue", queue.ID, "create", "", toJSON(queue))
	
	// Invalidate cache
	GetLookupService().InvalidateCache()
	
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    queue,
	})
}

func handleUpdateLookupQueue(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}
	
	repo := GetLookupRepository()
	
	// Get existing for audit log
	existing, err := repo.GetQueueByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}
	
	var queue models.QueueInfo
	if err := c.ShouldBindJSON(&queue); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	queue.ID = id
	if err := repo.UpdateQueue(c.Request.Context(), &queue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Log the change
	logLookupChange(c, "queue", id, "update", toJSON(existing), toJSON(queue))
	
	// Invalidate cache
	GetLookupService().InvalidateCache()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    queue,
	})
}

func handleDeleteLookupQueue(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
		return
	}
	
	repo := GetLookupRepository()
	
	// Get existing for audit log
	existing, err := repo.GetQueueByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}
	
	if err := repo.DeleteQueue(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Log the change
	logLookupChange(c, "queue", id, "delete", toJSON(existing), "")
	
	// Invalidate cache
	GetLookupService().InvalidateCache()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Queue deleted successfully",
	})
}

// Type CRUD handlers
func handleCreateType(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	var typ models.LookupItem
	if err := c.ShouldBindJSON(&typ); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	repo := GetLookupRepository()
	if err := repo.CreateType(c.Request.Context(), &typ); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Log the change
	logLookupChange(c, "type", typ.ID, "create", "", toJSON(typ))
	
	// Invalidate cache
	GetLookupService().InvalidateCache()
	
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    typ,
	})
}

func handleUpdateType(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid type ID"})
		return
	}
	
	repo := GetLookupRepository()
	
	// Get existing for audit log
	existing, err := repo.GetTypeByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Type not found"})
		return
	}
	
	var typ models.LookupItem
	if err := c.ShouldBindJSON(&typ); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	typ.ID = id
	if err := repo.UpdateType(c.Request.Context(), &typ); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Log the change
	logLookupChange(c, "type", id, "update", toJSON(existing), toJSON(typ))
	
	// Invalidate cache
	GetLookupService().InvalidateCache()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    typ,
	})
}

func handleDeleteType(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid type ID"})
		return
	}
	
	repo := GetLookupRepository()
	
	// Get existing for audit log
	existing, err := repo.GetTypeByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Type not found"})
		return
	}
	
	if err := repo.DeleteType(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Log the change
	logLookupChange(c, "type", id, "delete", toJSON(existing), "")
	
	// Invalidate cache
	GetLookupService().InvalidateCache()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Type deleted successfully",
	})
}

// Priority update handler (no create/delete for system-defined priorities)
func handleUpdatePriority(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid priority ID"})
		return
	}
	
	repo := GetLookupRepository()
	
	// Get existing for audit log
	existing, err := repo.GetPriorityByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Priority not found"})
		return
	}
	
	var priority models.LookupItem
	if err := c.ShouldBindJSON(&priority); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	priority.ID = id
	if err := repo.UpdatePriority(c.Request.Context(), &priority); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Log the change
	logLookupChange(c, "priority", id, "update", toJSON(existing), toJSON(priority))
	
	// Invalidate cache
	GetLookupService().InvalidateCache()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    priority,
		"message": "Priority label updated (value is system-defined)",
	})
}

// Status update handler (no create/delete for system-defined statuses)
func handleUpdateStatus(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status ID"})
		return
	}
	
	repo := GetLookupRepository()
	
	// Get existing for audit log
	existing, err := repo.GetStatusByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Status not found"})
		return
	}
	
	var status models.LookupItem
	if err := c.ShouldBindJSON(&status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	status.ID = id
	if err := repo.UpdateStatus(c.Request.Context(), &status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Log the change
	logLookupChange(c, "status", id, "update", toJSON(existing), toJSON(status))
	
	// Invalidate cache
	GetLookupService().InvalidateCache()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
		"message": "Status label updated (value is system-defined)",
	})
}

// Audit log handler
func handleGetAuditLogs(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	entityType := c.Query("entity_type")
	entityIDStr := c.Query("entity_id")
	limitStr := c.DefaultQuery("limit", "50")
	
	if entityType == "" || entityIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "entity_type and entity_id are required"})
		return
	}
	
	entityID, err := strconv.Atoi(entityIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity_id"})
		return
	}
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}
	
	repo := GetLookupRepository()
	logs, err := repo.GetAuditLogs(c.Request.Context(), entityType, entityID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
	})
}

// Export/Import handlers
func handleExportConfiguration(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	repo := GetLookupRepository()
	config, err := repo.ExportConfiguration(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Set export metadata
	if userEmail := c.GetString("user_email"); userEmail != "" {
		config.ExportedBy = userEmail
	}
	
	c.JSON(http.StatusOK, config)
}

func handleImportConfiguration(c *gin.Context) {
	if !checkAdminPermission(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	
	var config repository.LookupConfiguration
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	repo := GetLookupRepository()
	if err := repo.ImportConfiguration(c.Request.Context(), &config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Log the import
	logLookupChange(c, "system", 0, "import", "", "Imported configuration")
	
	// Invalidate cache
	GetLookupService().InvalidateCache()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Configuration imported successfully",
	})
}

// Helper functions
func checkAdminPermission(c *gin.Context) bool {
	// In production, check actual user permissions from JWT/session
	userRole := c.GetString("user_role")
	return userRole == "Admin"
}

func logLookupChange(c *gin.Context, entityType string, entityID int, action, oldValue, newValue string) {
	repo := GetLookupRepository()
	
	log := &repository.LookupAuditLog{
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		OldValue:   oldValue,
		NewValue:   newValue,
		UserID:     c.GetInt("user_id"),
		UserEmail:  c.GetString("user_email"),
		IPAddress:  c.ClientIP(),
	}
	
	// Fire and forget - don't fail the request if logging fails
	go repo.LogChange(c.Request.Context(), log)
}

func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}