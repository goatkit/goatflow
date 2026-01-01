package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CannedResponse represents a pre-written response.
type CannedResponse struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	Category     string     `json:"category"`
	Content      string     `json:"content"`
	ContentType  string     `json:"content_type"` // text or html
	Tags         []string   `json:"tags"`
	Scope        string     `json:"scope"` // personal, team, global
	OwnerID      int        `json:"owner_id"`
	TeamID       int        `json:"team_id,omitempty"`
	Placeholders []string   `json:"placeholders"`
	UsageCount   int        `json:"usage_count"`
	LastUsed     *time.Time `json:"last_used,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Mock storage for canned responses.
var cannedResponses = map[int]*CannedResponse{
	1: {
		ID:          1,
		Name:        "Thank You",
		Category:    "General",
		Content:     "Thank you for contacting support. We'll review your request and respond shortly.",
		ContentType: "text",
		Tags:        []string{"greeting", "acknowledgment"},
		Scope:       "personal",
		OwnerID:     1,
		UsageCount:  5,
		CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-30 * 24 * time.Hour),
	},
	2: {
		ID:           2,
		Name:         "Password Reset Instructions",
		Category:     "Account",
		Content:      "Hello {{customer_name}}, thank you for ticket #{{ticket_id}}",
		ContentType:  "text",
		Tags:         []string{"password", "account"},
		Scope:        "team",
		OwnerID:      1,
		TeamID:       1,
		Placeholders: []string{"customer_name", "ticket_id"},
		UsageCount:   10,
		CreatedAt:    time.Now().Add(-20 * 24 * time.Hour),
		UpdatedAt:    time.Now().Add(-20 * 24 * time.Hour),
	},
	3: {
		ID:          3,
		Name:        "Another User Response",
		Category:    "General",
		Content:     "This belongs to another user",
		ContentType: "text",
		Scope:       "personal",
		OwnerID:     2, // Different user
		CreatedAt:   time.Now().Add(-10 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-10 * 24 * time.Hour),
	},
	4: {
		ID:          4,
		Name:        "Service Maintenance",
		Category:    "System",
		Content:     "We are currently performing scheduled maintenance.",
		ContentType: "text",
		Scope:       "global",
		OwnerID:     1,
		UsageCount:  20,
		CreatedAt:   time.Now().Add(-15 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-15 * 24 * time.Hour),
	},
}

var nextCannedResponseID = 5
var cannedResponsesByName = map[string][]int{
	"Thank You":                   {1},
	"Password Reset Instructions": {2},
	"Another User Response":       {3},
	"Service Maintenance":         {4},
}

// handleCreateCannedResponse creates a new canned response.
func handleCreateCannedResponse(c *gin.Context) {
	var req struct {
		Name         string   `json:"name"`
		Category     string   `json:"category"`
		Content      string   `json:"content"`
		ContentType  string   `json:"content_type"`
		Tags         []string `json:"tags"`
		Scope        string   `json:"scope"`
		TeamID       int      `json:"team_id"`
		Placeholders []string `json:"placeholders"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.Name == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name and content are required"})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	// Check permissions for global scope
	if req.Scope == "global" && userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can create global responses"})
		return
	}

	// Check for duplicates in same scope
	if existingIDs, exists := cannedResponsesByName[req.Name]; exists {
		for _, id := range existingIDs {
			if resp, ok := cannedResponses[id]; ok {
				if resp.Scope == req.Scope {
					if req.Scope == "personal" && resp.OwnerID == userID.(int) {
						c.JSON(http.StatusConflict, gin.H{"error": "Canned response with this name already exists in your personal responses"})
						return
					} else if req.Scope == "team" && resp.TeamID == req.TeamID {
						c.JSON(http.StatusConflict, gin.H{"error": "Canned response with this name already exists in team responses"})
						return
					} else if req.Scope == "global" {
						c.JSON(http.StatusConflict, gin.H{"error": "Canned response with this name already exists in global responses"})
						return
					}
				}
			}
		}
	}

	// Set defaults
	if req.ContentType == "" {
		req.ContentType = "text"
	}
	if req.Scope == "" {
		req.Scope = "personal"
	}

	// Extract placeholders from content if not provided
	if len(req.Placeholders) == 0 {
		req.Placeholders = extractPlaceholders(req.Content)
	}

	// Create response
	response := &CannedResponse{
		ID:           nextCannedResponseID,
		Name:         req.Name,
		Category:     req.Category,
		Content:      req.Content,
		ContentType:  req.ContentType,
		Tags:         req.Tags,
		Scope:        req.Scope,
		OwnerID:      userID.(int),
		TeamID:       req.TeamID,
		Placeholders: req.Placeholders,
		UsageCount:   0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	cannedResponses[nextCannedResponseID] = response

	// Update name index
	if cannedResponsesByName[req.Name] == nil {
		cannedResponsesByName[req.Name] = []int{}
	}
	cannedResponsesByName[req.Name] = append(cannedResponsesByName[req.Name], nextCannedResponseID)

	nextCannedResponseID++

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Canned response created successfully",
		"id":       response.ID,
		"response": response,
	})
}

// handleGetCannedResponses returns accessible canned responses.
func handleGetCannedResponses(c *gin.Context) {
	userID, _ := c.Get("user_id")
	teamID, _ := c.Get("team_id")

	// Parse filters
	category := c.Query("category")
	scope := c.Query("scope")
	search := c.Query("search")
	tags := c.Query("tags")

	var results []CannedResponse

	for _, resp := range cannedResponses {
		// Check access permissions
		if !canAccessResponse(resp, userID.(int), teamID) {
			continue
		}

		// Apply filters
		if category != "" && resp.Category != category {
			continue
		}

		if scope != "" && resp.Scope != scope {
			continue
		}

		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(resp.Name), searchLower) &&
				!strings.Contains(strings.ToLower(resp.Content), searchLower) &&
				!strings.Contains(strings.ToLower(resp.Category), searchLower) {
				continue
			}
		}

		if tags != "" {
			tagList := strings.Split(tags, ",")
			found := false
			for _, tag := range tagList {
				for _, respTag := range resp.Tags {
					if respTag == strings.TrimSpace(tag) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
		}

		results = append(results, *resp)
	}

	c.JSON(http.StatusOK, gin.H{
		"responses": results,
		"total":     len(results),
	})
}

// handleUpdateCannedResponse updates a canned response.
func handleUpdateCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	response, exists := cannedResponses[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	// Check permissions
	if response.Scope == "personal" && response.OwnerID != userID.(int) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only edit your own personal responses"})
		return
	}
	if response.Scope == "global" && userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can edit global responses"})
		return
	}

	var req struct {
		Name         string   `json:"name"`
		Category     string   `json:"category"`
		Content      string   `json:"content"`
		ContentType  string   `json:"content_type"`
		Tags         []string `json:"tags"`
		Placeholders []string `json:"placeholders"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields if provided
	if req.Name != "" {
		response.Name = req.Name
	}
	if req.Category != "" {
		response.Category = req.Category
	}
	if req.Content != "" {
		response.Content = req.Content
		// Re-extract placeholders if content changed
		if len(req.Placeholders) == 0 {
			response.Placeholders = extractPlaceholders(req.Content)
		}
	}
	if req.ContentType != "" {
		response.ContentType = req.ContentType
	}
	if len(req.Tags) > 0 {
		response.Tags = req.Tags
	}
	if len(req.Placeholders) > 0 {
		response.Placeholders = req.Placeholders
	}

	response.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"message":  "Canned response updated successfully",
		"response": response,
	})
}

// handleDeleteCannedResponse deletes a canned response.
func handleDeleteCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	response, exists := cannedResponses[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	// Check permissions
	if response.Scope == "personal" && response.OwnerID != userID.(int) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own personal responses"})
		return
	}
	if response.Scope == "global" && userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can delete global responses"})
		return
	}

	delete(cannedResponses, id)

	c.JSON(http.StatusOK, gin.H{
		"message": "Canned response deleted successfully",
	})
}

// handleUseCannedResponse applies a canned response with placeholder substitution.
func handleUseCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	response, exists := cannedResponses[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	var req struct {
		Placeholders    map[string]string `json:"placeholders"`
		IncludeMetadata bool              `json:"include_metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply placeholders
	content := response.Content

	// Check for required placeholders
	for _, placeholder := range response.Placeholders {
		if value, ok := req.Placeholders[placeholder]; ok {
			content = strings.ReplaceAll(content, "{{"+placeholder+"}}", value)
		} else if strings.Contains(content, "{{"+placeholder+"}}") {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Missing required placeholder: %s", placeholder)})
			return
		}
	}

	// Update usage statistics
	response.UsageCount++
	now := time.Now()
	response.LastUsed = &now

	result := gin.H{
		"content": content,
	}

	if req.IncludeMetadata {
		result["used_at"] = now
		result["used_count"] = response.UsageCount
	}

	c.JSON(http.StatusOK, result)
}

// handleGetCannedResponseCategories returns all available categories.
func handleGetCannedResponseCategories(c *gin.Context) {
	categoryMap := make(map[string]int)
	colorMap := map[string]string{
		"General":    "#4A90E2",
		"Account":    "#7B68EE",
		"System":     "#50C878",
		"Support":    "#FFB347",
		"Imported":   "#FF6B6B",
		"Onboarding": "#4ECDC4",
	}

	for _, resp := range cannedResponses {
		if resp.Category != "" {
			categoryMap[resp.Category]++
		}
	}

	var categories []map[string]interface{}
	for name, count := range categoryMap {
		color := colorMap[name]
		if color == "" {
			color = "#999999"
		}
		categories = append(categories, map[string]interface{}{
			"name":  name,
			"count": count,
			"color": color,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
	})
}

// handleGetCannedResponseStatistics returns usage statistics.
func handleGetCannedResponseStatistics(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var personalCount, teamCount, globalCount int
	var mostUsed []map[string]interface{}
	var recentlyUsed []map[string]interface{}
	usageByCategory := make(map[string]int)

	for _, resp := range cannedResponses {
		// Count by scope
		switch resp.Scope {
		case "personal":
			if resp.OwnerID == userID.(int) {
				personalCount++
			}
		case "team":
			teamCount++
		case "global":
			globalCount++
		}

		// Track usage by category
		if resp.Category != "" {
			usageByCategory[resp.Category] += resp.UsageCount
		}

		// Track most used
		if resp.UsageCount > 0 {
			mostUsed = append(mostUsed, map[string]interface{}{
				"id":          resp.ID,
				"name":        resp.Name,
				"usage_count": resp.UsageCount,
			})
		}

		// Track recently used
		if resp.LastUsed != nil {
			recentlyUsed = append(recentlyUsed, map[string]interface{}{
				"id":        resp.ID,
				"name":      resp.Name,
				"last_used": resp.LastUsed,
			})
		}
	}

	// Sort and limit most used (simplified)
	if len(mostUsed) > 5 {
		mostUsed = mostUsed[:5]
	}
	if len(recentlyUsed) > 5 {
		recentlyUsed = recentlyUsed[:5]
	}

	c.JSON(http.StatusOK, gin.H{
		"statistics": gin.H{
			"total_responses":   len(cannedResponses),
			"personal_count":    personalCount,
			"team_count":        teamCount,
			"global_count":      globalCount,
			"most_used":         mostUsed,
			"recently_used":     recentlyUsed,
			"usage_by_category": usageByCategory,
		},
	})
}

// handleShareCannedResponse shares a personal response with team or global.
func handleShareCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	response, exists := cannedResponses[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	userID, _ := c.Get("user_id")

	// Check ownership
	if response.OwnerID != userID.(int) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only share your own responses"})
		return
	}

	var req struct {
		Scope  string `json:"scope"`
		TeamID int    `json:"team_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update scope
	response.Scope = req.Scope
	if req.Scope == "team" {
		response.TeamID = req.TeamID
	}
	response.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"message": "Canned response shared successfully",
	})
}

// handleCopyCannedResponse copies a shared response to personal.
func handleCopyCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	response, exists := cannedResponses[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	userID, _ := c.Get("user_id")

	// Create a copy
	newResponse := &CannedResponse{
		ID:           nextCannedResponseID,
		Name:         response.Name + " (Copy)",
		Category:     response.Category,
		Content:      response.Content,
		ContentType:  response.ContentType,
		Tags:         response.Tags,
		Scope:        "personal",
		OwnerID:      userID.(int),
		Placeholders: response.Placeholders,
		UsageCount:   0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	cannedResponses[nextCannedResponseID] = newResponse
	nextCannedResponseID++

	c.JSON(http.StatusCreated, gin.H{
		"message": "Canned response copied successfully",
		"id":      newResponse.ID,
	})
}

// handleExportCannedResponses exports user's canned responses.
func handleExportCannedResponses(c *gin.Context) {
	userID, _ := c.Get("user_id")
	format := c.Query("format")

	var userResponses []CannedResponse
	for _, resp := range cannedResponses {
		if resp.OwnerID == userID.(int) && resp.Scope == "personal" {
			userResponses = append(userResponses, *resp)
		}
	}

	timestamp := time.Now().Format("20060102-150405")

	switch format {
	case "json":
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"canned-responses-%s.json\"", timestamp))

		c.JSON(http.StatusOK, gin.H{
			"responses":   userResponses,
			"exported_at": time.Now(),
		})

	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"canned-responses-%s.csv\"", timestamp))

		writer := csv.NewWriter(c.Writer)
		writer.Write([]string{"Name", "Category", "Content", "Tags", "Created"})

		for _, resp := range userResponses {
			writer.Write([]string{
				resp.Name,
				resp.Category,
				resp.Content,
				strings.Join(resp.Tags, ","),
				resp.CreatedAt.Format("2006-01-02"),
			})
		}
		writer.Flush()

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Supported: json, csv"})
	}
}

// handleImportCannedResponses imports canned responses.
func handleImportCannedResponses(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		Responses []struct {
			Name     string   `json:"name"`
			Category string   `json:"category"`
			Content  string   `json:"content"`
			Tags     []string `json:"tags"`
			Scope    string   `json:"scope"`
		} `json:"responses"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	imported := 0
	skipped := 0

	for _, respData := range req.Responses {
		// Check for duplicates
		duplicate := false
		if existingIDs, exists := cannedResponsesByName[respData.Name]; exists {
			for _, id := range existingIDs {
				if resp, ok := cannedResponses[id]; ok {
					if resp.OwnerID == userID.(int) && resp.Scope == "personal" {
						duplicate = true
						skipped++
						break
					}
				}
			}
		}

		if !duplicate {
			response := &CannedResponse{
				ID:         nextCannedResponseID,
				Name:       respData.Name,
				Category:   respData.Category,
				Content:    respData.Content,
				Tags:       respData.Tags,
				Scope:      "personal",
				OwnerID:    userID.(int),
				UsageCount: 0,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			cannedResponses[nextCannedResponseID] = response

			if cannedResponsesByName[respData.Name] == nil {
				cannedResponsesByName[respData.Name] = []int{}
			}
			cannedResponsesByName[respData.Name] = append(cannedResponsesByName[respData.Name], nextCannedResponseID)

			nextCannedResponseID++
			imported++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Canned responses imported successfully",
		"imported_count": imported,
		"skipped_count":  skipped,
	})
}

// Helper functions

func canAccessResponse(resp *CannedResponse, userID int, teamID interface{}) bool {
	switch resp.Scope {
	case "personal":
		return resp.OwnerID == userID
	case "team":
		if teamID != nil {
			return resp.TeamID == teamID.(int)
		}
		return false
	case "global":
		return true
	default:
		return false
	}
}

func extractPlaceholders(content string) []string {
	var placeholders []string
	seen := make(map[string]bool)

	// Find {{placeholder}} patterns
	for i := 0; i < len(content)-3; i++ {
		if content[i:i+2] == "{{" {
			end := strings.Index(content[i+2:], "}}")
			if end > 0 {
				placeholder := content[i+2 : i+2+end]
				if !seen[placeholder] {
					placeholders = append(placeholders, placeholder)
					seen[placeholder] = true
				}
			}
		}
	}

	return placeholders
}
