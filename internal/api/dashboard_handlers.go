package api

import (
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)

// Unexported concrete handlers; exported vars in exports.go point here.
var (
    handleLoginPage = func(c *gin.Context) {
        renderer := GetPongo2Renderer()
        if renderer == nil {
            c.String(http.StatusInternalServerError, "template renderer unavailable")
            return
        }
        renderer.HTML(c, http.StatusOK, "pages/login.pongo2", map[string]any{
            "Title": "Login",
        })
    }

    handleDashboard = func(c *gin.Context) {
        renderer := GetPongo2Renderer()
        if renderer == nil {
            c.String(http.StatusInternalServerError, "template renderer unavailable")
            return
        }
        userID, _ := c.Get("user_id")
        userRole, _ := c.Get("user_role")
        renderer.HTML(c, http.StatusOK, "pages/dashboard.pongo2", map[string]any{
            "UserID":  userID,
            "UserRole": userRole,
            "Now":     time.Now(),
        })
    }

    handleDashboardStats = func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "open_tickets":   0,
            "pending_tickets": 0,
            "updated_at":     time.Now().UTC(),
        })
    }

    handleRecentTickets = func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"tickets": []gin.H{}})
    }

    dashboard_queue_status = func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"queues": []gin.H{}})
    }

    handleActivityStream = func(c *gin.Context) {
        c.Writer.Header().Set("Content-Type", "text/event-stream")
        c.Writer.Header().Set("Cache-Control", "no-cache")
        c.Writer.Header().Set("Connection", "keep-alive")
        c.Writer.Flush()
        _, err := c.Writer.Write([]byte("event: heartbeat\n" + "data: ok\n\n"))
        if err != nil {
            log.Printf("activity stream write error: %v", err)
        }
    }
)
