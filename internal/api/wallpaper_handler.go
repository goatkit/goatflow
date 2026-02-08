package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/service"
)

// HandleSetWallpaper saves the user's wallpaper preference (on/off).
func HandleSetWallpaper(c *gin.Context) {
	userID := GetUserIDFromCtx(c, 0)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}

	// In demo mode, cookie is enough - don't persist
	if isDemo, _ := c.Get("is_demo"); isDemo == true {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	prefService := service.NewUserPreferencesService(db)
	_ = prefService.SetPreference(userID, "wallpaper_enabled", req.Value)

	c.JSON(http.StatusOK, gin.H{"success": true})
}
