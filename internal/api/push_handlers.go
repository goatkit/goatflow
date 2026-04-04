package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/push"
)

func handleGetVAPIDKey(c *gin.Context) {
	cfg := config.Get()
	pub, _, err := push.GetVAPIDKeys(cfg.Push.VAPIDPublicKey, cfg.Push.VAPIDPrivateKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get VAPID key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"publicKey": pub})
}

func handlePushSubscribe(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
		Keys     struct {
			P256dh string `json:"p256dh" binding:"required"`
			Auth   string `json:"auth" binding:"required"`
		} `json:"keys" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	userType := "agent"
	if ic, _ := c.Get("is_customer"); ic == true {
		userType = "customer"
	}

	uid, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database unavailable"})
		return
	}

	if err := push.SaveSubscription(c.Request.Context(), db, uid, userType, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func handlePushUnsubscribe(c *gin.Context) {
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database unavailable"})
		return
	}

	if err := push.DeleteSubscriptionByEndpoint(c.Request.Context(), db, req.Endpoint); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
