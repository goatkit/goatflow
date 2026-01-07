package api

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// SignatureForAgent represents a signature available to agents.
type SignatureForAgent struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Text        string `json:"text"`
	ContentType string `json:"content_type"`
}

// GetQueueSignature returns the signature assigned to a specific queue.
func GetQueueSignature(queueID int) (*SignatureForAgent, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	query := `
		SELECT s.id, s.name, s.text, s.content_type
		FROM signature s
		INNER JOIN queue q ON q.signature_id = s.id
		WHERE q.id = ? AND s.valid_id = 1
	`

	var sig SignatureForAgent
	var text, contentType sql.NullString

	err = db.QueryRow(database.ConvertPlaceholders(query), queueID).Scan(
		&sig.ID, &sig.Name, &text, &contentType,
	)
	if err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil
	}
	if err != nil {
		return nil, err
	}

	sig.Text = text.String
	sig.ContentType = contentType.String

	return &sig, nil
}

// GET /agent/api/signatures/queue/:queue_id?ticket_id=X.
func handleGetQueueSignature(c *gin.Context) {
	queueIDStr := c.Param("queue_id")
	queueID, err := strconv.Atoi(queueIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid queue_id",
		})
		return
	}

	signature, err := GetQueueSignature(queueID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to fetch signature",
		})
		return
	}

	if signature == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    nil,
		})
		return
	}

	// Get ticket context for variable substitution if provided
	ticketIDStr := c.Query("ticket_id")
	if ticketIDStr != "" {
		ticketID, err := strconv.Atoi(ticketIDStr)
		if err == nil {
			vars := GetTicketTemplateVariables(ticketID)
			// Add agent context from session
			if agentName, exists := c.Get("agent_fullname"); exists {
				if name, ok := agentName.(string); ok {
					vars["CURRENT_UserFullname"] = name
				}
			}
			if agentFirst, exists := c.Get("agent_firstname"); exists {
				if first, ok := agentFirst.(string); ok {
					vars["CURRENT_UserFirstname"] = first
				}
			}
			if agentLast, exists := c.Get("agent_lastname"); exists {
				if last, ok := agentLast.(string); ok {
					vars["CURRENT_UserLastname"] = last
				}
			}
			signature.Text = SubstituteTemplateVariables(signature.Text, vars)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           signature.ID,
			"name":         signature.Name,
			"text":         signature.Text,
			"content_type": signature.ContentType,
		},
	})
}
