package deletion

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// HandleAdminRecycleBinList handles GET /admin/recycle-bin — lists recycle bin entries.
func HandleAdminRecycleBinList(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		entityType := c.Query("entity_type")
		entries, err := svc.RecycleBinList(c.Request.Context(), entityType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"entries": entries})
	}
}

// HandleAdminRestore handles POST /admin/api/recycle-bin/restore.
func HandleAdminRestore(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			EntityType string `json:"entity_type"`
			EntityID   int64  `json:"entity_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.EntityType == "" || req.EntityID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "entity_type and entity_id are required"})
			return
		}

		userID := getAdminUserID(c)
		if err := svc.Restore(c.Request.Context(), req.EntityType, req.EntityID, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "restored", "entity_type": req.EntityType, "entity_id": req.EntityID})
	}
}

// HandleAdminHardDelete handles POST /admin/api/recycle-bin/purge.
func HandleAdminHardDelete(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			EntityType string `json:"entity_type"`
			EntityID   int64  `json:"entity_id"`
			Reason     string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.EntityType == "" || req.EntityID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "entity_type and entity_id are required"})
			return
		}

		userID := getAdminUserID(c)
		if err := svc.HardDelete(c.Request.Context(), req.EntityType, req.EntityID, userID, req.Reason); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "purged", "entity_type": req.EntityType, "entity_id": req.EntityID})
	}
}

// HandleAdminBatchSoftDelete handles POST /admin/api/recycle-bin/batch-delete.
func HandleAdminBatchSoftDelete(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			EntityType string  `json:"entity_type"`
			EntityIDs  []int64 `json:"entity_ids"`
			Reason     string  `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.EntityType == "" || len(req.EntityIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "entity_type and entity_ids are required"})
			return
		}

		userID := getAdminUserID(c)
		deleted, err := svc.ScopeSoftDelete(c.Request.Context(), req.EntityType, req.EntityIDs, userID, req.Reason)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "deleted", "count": deleted})
	}
}

// HandleAdminBatchHardDelete handles POST /admin/api/recycle-bin/batch-purge.
func HandleAdminBatchHardDelete(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			EntityType string  `json:"entity_type"`
			EntityIDs  []int64 `json:"entity_ids"`
			Reason     string  `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.EntityType == "" || len(req.EntityIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "entity_type and entity_ids are required"})
			return
		}

		userID := getAdminUserID(c)
		deleted, err := svc.ScopeHardDelete(c.Request.Context(), req.EntityType, req.EntityIDs, userID, req.Reason)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "purged", "count": deleted})
	}
}

// HandleAdminDeletionLog handles GET /admin/api/recycle-bin/log/:entity_type/:entity_id.
func HandleAdminDeletionLog(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		entityType := c.Param("entity_type")
		entityID, err := strconv.ParseInt(c.Param("entity_id"), 10, 64)
		if err != nil || entityType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid entity_type or entity_id"})
			return
		}

		logs, err := svc.repo.GetDeletionLog(entityType, entityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"logs": logs})
	}
}

func getAdminUserID(c *gin.Context) int {
	if id, exists := c.Get("user_id"); exists {
		switch v := id.(type) {
		case int:
			return v
		case int64:
			return int(v)
		}
	}
	return 1
}
