package api

import (
	"encoding/csv"
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
	ContentType  string     `json:"content_type"`
	Tags         []string   `json:"tags"`
	Scope        string     `json:"scope"`
	OwnerID      int        `json:"owner_id"`
	TeamID       int        `json:"team_id,omitempty"`
	Placeholders []string   `json:"placeholders"`
	UsageCount   int        `json:"usage_count"`
	LastUsed     *time.Time `json:"last_used,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func getCannedResponseRepo(c *gin.Context) (*CannedResponseRepository, bool) {
	repo, err := NewCannedResponseRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return nil, false
	}
	return repo, true
}

// getContextInt safely extracts an int from gin context
func getContextInt(c *gin.Context, key string) int {
	if val, exists := c.Get(key); exists && val != nil {
		if v, ok := val.(int); ok {
			return v
		}
	}
	return 0
}

// getContextString safely extracts a string from gin context
func getContextString(c *gin.Context, key string) string {
	if val, exists := c.Get(key); exists && val != nil {
		if v, ok := val.(string); ok {
			return v
		}
	}
	return ""
}

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

	if req.Name == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name and content are required"})
		return
	}

	userID := getContextInt(c, "user_id")
	userRole := getContextString(c, "user_role")

	if req.Scope == "global" && userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can create global responses"})
		return
	}

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	teamIDVal := getContextInt(c, "team_id")
	if req.TeamID > 0 {
		teamIDVal = req.TeamID
	}

	exists, err := repo.CheckDuplicate(req.Name, req.Scope, userID, teamIDVal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for duplicates"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Canned response with this name already exists in this scope"})
		return
	}

	if req.ContentType == "" {
		req.ContentType = "text"
	}
	if req.Scope == "" {
		req.Scope = "personal"
	}
	if len(req.Placeholders) == 0 {
		req.Placeholders = extractPlaceholders(req.Content)
	}

	cr := &CannedResponse{
		Name:         req.Name,
		Category:     req.Category,
		Content:      req.Content,
		ContentType:  req.ContentType,
		Tags:         req.Tags,
		Scope:        req.Scope,
		OwnerID:      userID,
		TeamID:       teamIDVal,
		Placeholders: req.Placeholders,
	}

	id, err := repo.Create(cr, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create canned response"})
		return
	}

	cr.ID = id
	cr.CreatedAt = time.Now()
	cr.UpdatedAt = time.Now()

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Canned response created successfully",
		"id":       id,
		"response": cr,
	})
}

func handleGetCannedResponses(c *gin.Context) {
	userID := getContextInt(c, "user_id")
	teamID := getContextInt(c, "team_id")

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	filters := CannedResponseFilters{
		Category:  c.Query("category"),
		Scope:     c.Query("scope"),
		Search:    c.Query("search"),
		SortBy:    c.DefaultQuery("sort_by", "name"),
		SortOrder: c.DefaultQuery("sort_order", "asc"),
	}

	if tagsParam := c.Query("tags"); tagsParam != "" {
		filters.Tags = strings.Split(tagsParam, ",")
	}

	responses, err := repo.ListAccessible(userID, teamID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned responses"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"responses":   responses,
		"total_count": len(responses),
	})
}

func handleUpdateCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

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

	userID := getContextInt(c, "user_id")
	userRole := getContextString(c, "user_role")

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	existing, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned response"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	if !canModifyResponseTyped(existing, userID, userRole) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to modify this response"})
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Category != "" {
		existing.Category = req.Category
	}
	if req.Content != "" {
		existing.Content = req.Content
		if len(req.Placeholders) == 0 {
			existing.Placeholders = extractPlaceholders(req.Content)
		}
	}
	if req.ContentType != "" {
		existing.ContentType = req.ContentType
	}
	if req.Tags != nil {
		existing.Tags = req.Tags
	}
	if len(req.Placeholders) > 0 {
		existing.Placeholders = req.Placeholders
	}

	if err := repo.Update(id, existing, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update canned response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Canned response updated successfully",
		"response": existing,
	})
}

func handleDeleteCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	userID := getContextInt(c, "user_id")
	userRole := getContextString(c, "user_role")

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	existing, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned response"})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	if !canModifyResponseTyped(existing, userID, userRole) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to delete this response"})
		return
	}

	if err := repo.Delete(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete canned response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Canned response deleted successfully"})
}

func handleUseCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	userID := getContextInt(c, "user_id")
	teamID := getContextInt(c, "team_id")

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	resp, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned response"})
		return
	}
	if resp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	if !canAccessResponseTyped(resp, userID, teamID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this response"})
		return
	}

	var req struct {
		Context map[string]string `json:"context"`
	}
	_ = c.ShouldBindJSON(&req) //nolint:errcheck // Optional context

	content := resp.Content
	if req.Context != nil {
		for key, value := range req.Context {
			content = strings.ReplaceAll(content, "{{"+key+"}}", value)
		}
	}

	_ = repo.IncrementUsage(id) //nolint:errcheck // Best-effort usage tracking

	c.JSON(http.StatusOK, gin.H{
		"content":      content,
		"content_type": resp.ContentType,
		"placeholders": resp.Placeholders,
	})
}

func handleGetCannedResponseCategories(c *gin.Context) {
	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	categories, err := repo.ListCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func handleGetCannedResponseStatistics(c *gin.Context) {
	userID := getContextInt(c, "user_id")
	teamID := getContextInt(c, "team_id")

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	responses, err := repo.ListAccessible(userID, teamID, CannedResponseFilters{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load statistics"})
		return
	}

	stats := struct {
		TotalCount   int            `json:"total_count"`
		ByScope      map[string]int `json:"by_scope"`
		ByCategory   map[string]int `json:"by_category"`
		TotalUsage   int            `json:"total_usage"`
		MostUsed     []gin.H        `json:"most_used"`
		RecentlyUsed []gin.H        `json:"recently_used"`
	}{
		ByScope:    make(map[string]int),
		ByCategory: make(map[string]int),
	}

	for _, r := range responses {
		stats.TotalCount++
		stats.ByScope[r.Scope]++
		if r.Category != "" {
			stats.ByCategory[r.Category]++
		}
		stats.TotalUsage += r.UsageCount
	}

	c.JSON(http.StatusOK, stats)
}

func handleShareCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
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

	userID := getContextInt(c, "user_id")
	userRole := getContextString(c, "user_role")

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	resp, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned response"})
		return
	}
	if resp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	if !canModifyResponseTyped(resp, userID, userRole) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to share this response"})
		return
	}

	if req.Scope == "global" && userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can share globally"})
		return
	}

	resp.Scope = req.Scope
	if req.TeamID > 0 {
		resp.TeamID = req.TeamID
	}

	if err := repo.Update(id, resp, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to share canned response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Canned response shared successfully",
		"response": resp,
	})
}

func handleCopyCannedResponse(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return
	}

	userID := getContextInt(c, "user_id")
	teamID := getContextInt(c, "team_id")

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	source, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned response"})
		return
	}
	if source == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return
	}

	if !canAccessResponseTyped(source, userID, teamID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this response"})
		return
	}

	copy := &CannedResponse{
		Name:         source.Name + " (Copy)",
		Category:     source.Category,
		Content:      source.Content,
		ContentType:  source.ContentType,
		Tags:         source.Tags,
		Scope:        "personal",
		OwnerID:      userID,
		Placeholders: source.Placeholders,
	}

	newID, err := repo.Create(copy, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy canned response"})
		return
	}

	copy.ID = newID
	copy.CreatedAt = time.Now()
	copy.UpdatedAt = time.Now()

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Canned response copied successfully",
		"response": copy,
	})
}

func handleExportCannedResponses(c *gin.Context) {
	userID := getContextInt(c, "user_id")
	teamID := getContextInt(c, "team_id")

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	filters := CannedResponseFilters{
		Scope: c.Query("scope"),
	}

	responses, err := repo.ListAccessible(userID, teamID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned responses"})
		return
	}

	format := c.DefaultQuery("format", "json")

	if format == "csv" {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=canned_responses.csv")

		writer := csv.NewWriter(c.Writer)
		_ = writer.Write([]string{"Name", "Category", "Content", "Tags", "Scope"}) //nolint:errcheck // Best-effort CSV write

		for _, r := range responses {
			_ = writer.Write([]string{ //nolint:errcheck // Best-effort CSV write
				r.Name,
				r.Category,
				r.Content,
				strings.Join(r.Tags, ","),
				r.Scope,
			})
		}
		writer.Flush()
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"responses": responses,
		"count":     len(responses),
	})
}

func handleImportCannedResponses(c *gin.Context) {
	userID := getContextInt(c, "user_id")

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}
	defer file.Close()

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CSV format"})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CSV file is empty or has no data rows"})
		return
	}

	imported := 0
	skipped := 0

	for i, record := range records[1:] {
		if len(record) < 3 {
			skipped++
			continue
		}

		name := strings.TrimSpace(record[0])
		if name == "" {
			skipped++
			continue
		}

		exists, _ := repo.CheckDuplicate(name, "personal", userID, 0) //nolint:errcheck // False on error
		if exists {
			skipped++
			continue
		}

		category := ""
		if len(record) > 1 {
			category = strings.TrimSpace(record[1])
		}
		content := strings.TrimSpace(record[2])

		var tags []string
		if len(record) > 3 && record[3] != "" {
			tags = strings.Split(record[3], ",")
			for j := range tags {
				tags[j] = strings.TrimSpace(tags[j])
			}
		}

		cr := &CannedResponse{
			Name:         name,
			Category:     category,
			Content:      content,
			ContentType:  "text",
			Tags:         tags,
			Scope:        "personal",
			OwnerID:      userID,
			Placeholders: extractPlaceholders(content),
		}

		_, err := repo.Create(cr, userID)
		if err != nil {
			skipped++
			continue
		}
		imported++

		_ = i // Suppress unused variable warning
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Canned responses imported successfully",
		"imported_count": imported,
		"skipped_count":  skipped,
	})
}

func canAccessResponse(resp *CannedResponse, userID int, teamID int) bool {
	switch resp.Scope {
	case "personal":
		return resp.OwnerID == userID
	case "team":
		return resp.TeamID == teamID
	case "global":
		return true
	default:
		return false
	}
}

func canAccessResponseTyped(resp *CannedResponse, userID int, teamID int) bool {
	return canAccessResponse(resp, userID, teamID)
}

func canModifyResponse(resp *CannedResponse, userID int, userRole interface{}) bool {
	if userRole == "admin" {
		return true
	}
	if resp.Scope == "personal" {
		return resp.OwnerID == userID
	}
	return false
}

func canModifyResponseTyped(resp *CannedResponse, userID int, userRole string) bool {
	if userRole == "admin" {
		return true
	}
	if resp.Scope == "personal" {
		return resp.OwnerID == userID
	}
	return false
}

func extractPlaceholders(content string) []string {
	var placeholders []string
	seen := make(map[string]bool)

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

// RegisterCannedResponseHandlers registers all canned response API routes.
func RegisterCannedResponseHandlers(r *gin.RouterGroup) {
	cr := r.Group("/canned-responses")
	cr.POST("", handleCreateCannedResponse)
	cr.GET("", handleGetCannedResponses)
	cr.GET("/categories", handleGetCannedResponseCategories)
	cr.GET("/statistics", handleGetCannedResponseStatistics)
	cr.GET("/export", handleExportCannedResponses)
	cr.POST("/import", handleImportCannedResponses)
	cr.GET("/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
			return
		}

		repo, ok := getCannedResponseRepo(c)
		if !ok {
			return
		}

		resp, err := repo.GetByID(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned response"})
			return
		}
		if resp == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"response": resp})
	})
	cr.PUT("/:id", handleUpdateCannedResponse)
	cr.DELETE("/:id", handleDeleteCannedResponse)
	cr.POST("/:id/use", handleUseCannedResponse)
	cr.POST("/:id/share", handleShareCannedResponse)
	cr.POST("/:id/copy", handleCopyCannedResponse)
}
