package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/cache"
)

// valkeyCache holds the cache client injected from main for status lookups.
var valkeyCache *cache.RedisCache

// SetValkeyCache allows main to provide the shared Redis/Valkey cache.
func SetValkeyCache(c *cache.RedisCache) {
	valkeyCache = c
}

type mailPollStatus struct {
	AccountID       int        `json:"account_id"`
	LastPollAt      *time.Time `json:"last_poll_at"`
	LastStatus      string     `json:"last_status"`
	LastError       string     `json:"last_error"`
	MessagesFetched int        `json:"messages_fetched"`
	NextPollETA     *time.Time `json:"next_poll_eta"`
}

// HandleMailAccountPollStatus returns the last poll status for a mail account from Valkey.
func HandleMailAccountPollStatus(c *gin.Context) {
	if valkeyCache == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "status cache unavailable"})
		return
	}

	idParam := c.Param("id")
	accountID, err := strconv.Atoi(idParam)
	if err != nil || accountID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid account id"})
		return
	}

	key := fmt.Sprintf("mail_poll_status:%d", accountID)
	data, err := valkeyCache.Get(c, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if data == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": nil})
		return
	}

	raw, ok := data.([]byte)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "unexpected cache payload"})
		return
	}

	var status mailPollStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}
