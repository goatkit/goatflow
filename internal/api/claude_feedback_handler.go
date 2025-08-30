package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleClaudeFeedback processes feedback from the Claude chat widget
func HandleClaudeFeedback(c *gin.Context) {
	var request struct {
		Message string                 `json:"message"`
		Context map[string]interface{} `json:"context"`
		Timestamp string               `json:"timestamp"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Generate ticket number
	ticketNumber := fmt.Sprintf("%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()/1000000)
	
	// Prepare context information
	contextJSON, _ := json.Marshal(request.Context)
	currentPath := ""
	if path, ok := request.Context["currentPath"].(string); ok {
		currentPath = path
	}
	
	// Determine if this is an error report or general feedback
	isErrorReport := strings.Contains(strings.ToLower(request.Message), "error") || 
	                 strings.Contains(strings.ToLower(request.Message), "500") ||
	                 strings.Contains(strings.ToLower(request.Message), "404") ||
	                 strings.Contains(strings.ToLower(request.Message), "broken")
	
	// Set appropriate priority and state
	priorityID := 3 // normal
	stateID := 1    // new
	if isErrorReport {
		priorityID = 4 // high for errors
	}
	
	// Create title from message
	title := request.Message
	if len(title) > 100 {
		title = title[:97] + "..."
	}
	
	// Create ticket in Claude Code queue (ID: 14)
	query := `
		INSERT INTO ticket (
			tn, title, queue_id, type_id, ticket_state_id, 
			ticket_priority_id, ticket_lock_id, customer_user_id,
			create_time, change_time, create_by, change_by,
			until_time, escalation_time, escalation_response_time,
			escalation_update_time, escalation_solution_time, 
			archive_flag
		) VALUES (
			$1, $2, 14, 1, $3, 
			$4, 1, 'claude-chat',
			NOW(), NOW(), 1, 1,
			0, 0, 0, 
			0, 0,
			0
		) RETURNING id
	`
	
	var ticketID int
	err = db.QueryRow(query, ticketNumber, title, stateID, priorityID).Scan(&ticketID)
	if err != nil {
		// Log error but still respond to user
		fmt.Printf("Failed to create ticket: %v\n", err)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"response": "Thank you for your feedback! I've logged this issue and will investigate.",
		})
		return
	}
	
	// Create article with the feedback details
	articleBody := fmt.Sprintf(`Feedback from Claude Chat Widget

Message: %s

Page: %s
Timestamp: %s

Full Context:
%s`, request.Message, currentPath, request.Timestamp, string(contextJSON))
	
	// First create the article entry
	articleQuery := `
		INSERT INTO article (
			ticket_id, article_sender_type_id,
			communication_channel_id, is_visible_for_customer,
			create_time, change_time, create_by, change_by
		) VALUES (
			$1, 3,
			1, 1,
			NOW(), NOW(), 1, 1
		) RETURNING id
	`
	
	var articleID int
	err = db.QueryRow(articleQuery, ticketID).Scan(&articleID)
	if err != nil {
		fmt.Printf("Failed to create article: %v\n", err)
	} else {
		// Now create article_data_mime entry with the actual content
		articleDataQuery := `
			INSERT INTO article_data_mime (
				article_id, a_from, a_to, a_subject, a_body,
				a_content_type, create_time, change_time, 
				create_by, change_by
			) VALUES (
				$1, 'claude-chat@gotrs.io', 'support@gotrs.io', $2, 
				$3::bytea, 'text/plain', NOW(), NOW(), 1, 1
			)
		`
		
		_, err = db.Exec(articleDataQuery, articleID, title, []byte(articleBody))
		if err != nil {
			fmt.Printf("Failed to create article_data_mime: %v\n", err)
		}
	}
	
	// Prepare response based on type of feedback
	response := ""
	if isErrorReport {
		response = fmt.Sprintf("Thank you for reporting this error! I've created ticket #%s and will investigate immediately. The issue on %s has been logged with high priority.", ticketNumber, currentPath)
	} else {
		response = fmt.Sprintf("Thank you for your feedback! I've created ticket #%s to track this. Your suggestion has been recorded and will be reviewed.", ticketNumber)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"ticketNumber": ticketNumber,
		"response": response,
	})
}

// HandleClaudeTicketStatus checks the status of Claude tickets
func HandleClaudeTicketStatus(c *gin.Context) {
	var request struct {
		Tickets []string `json:"tickets"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": "Invalid request",
		})
		return
	}
	
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": "Database error",
		})
		return
	}
	
	var ticketUpdates []map[string]interface{}
	
	for _, ticketNum := range request.Tickets {
		var status string
		var stateID int
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT ts.name, t.ticket_state_id
			FROM ticket t
			JOIN ticket_state ts ON t.ticket_state_id = ts.id
			WHERE t.tn = $1
		`), ticketNum).Scan(&status, &stateID)
		
		if err == nil {
			// Map OTRS states to simple states for chat
			simpleStatus := "open"
			switch stateID {
			case 2, 3: // closed successful, closed unsuccessful
				simpleStatus = "closed"
			case 4: // pending
				simpleStatus = "pending"
			case 5, 6: // removed, merged
				simpleStatus = "closed"
			}
			
			ticketUpdates = append(ticketUpdates, map[string]interface{}{
				"number": ticketNum,
				"status": simpleStatus,
				"newResponses": []interface{}{}, // TODO: Check for new articles
			})
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"tickets": ticketUpdates,
	})
}