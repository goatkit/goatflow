package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/service"
)

const coachmarkPrefKey = "dismissed_coachmarks"

// HandleDismissCoachmark records a coachmark as dismissed for the current user.
func HandleDismissCoachmark(c *gin.Context) {
	userID := GetUserIDFromCtx(c, 0)
	if userID == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true}) // silently succeed for unauthenticated
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing id"})
		return
	}

	// In demo mode, don't persist (localStorage is enough)
	if isDemo, _ := c.Get("is_demo"); isDemo == true {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true}) // degrade gracefully
		return
	}

	prefService := service.NewUserPreferencesService(db)

	// Reset all coachmarks
	if req.ID == "__reset_all__" {
		_ = prefService.SetPreference(userID, coachmarkPrefKey, "[]")
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	// Get current dismissed list
	existing, _ := prefService.GetPreference(userID, coachmarkPrefKey)
	var dismissed []string
	if existing != "" {
		_ = json.Unmarshal([]byte(existing), &dismissed)
	}

	// Check if already dismissed
	for _, d := range dismissed {
		if d == req.ID {
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}
	}

	// Add and save
	dismissed = append(dismissed, req.ID)
	data, _ := json.Marshal(dismissed)
	_ = prefService.SetPreference(userID, coachmarkPrefKey, string(data))

	c.JSON(http.StatusOK, gin.H{"success": true})
}
