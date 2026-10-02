package api

import (
	"encoding/csv"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/shared"
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

// cannedResponseCaller identifies the agent calling a canned-response
// endpoint. Canned responses are agent tooling: customers are rejected.
type cannedResponseCaller struct {
	userID  int
	teamID  int
	isAdmin bool
}

func getCannedResponseCaller(c *gin.Context) (cannedResponseCaller, bool) {
	role, _ := c.Get("user_role") //nolint:errcheck // absent role handled below
	roleStr := shared.ToString(role, "")
	if strings.EqualFold(roleStr, "customer") || c.GetBool("is_customer") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Canned responses are available to agents only"})
		return cannedResponseCaller{}, false
	}
	userIDVal, _ := c.Get("user_id") //nolint:errcheck // absent id handled below
	userID := shared.ToInt(userIDVal, 0)
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return cannedResponseCaller{}, false
	}
	teamIDVal, _ := c.Get("team_id") //nolint:errcheck // optional
	return cannedResponseCaller{
		userID:  userID,
		teamID:  shared.ToInt(teamIDVal, 0),
		isAdmin: c.GetBool("isInAdminGroup") || strings.EqualFold(roleStr, "admin"),
	}, true
}

func getCannedResponseRepo(c *gin.Context) (*CannedResponseRepository, bool) {
	repo, err := NewCannedResponseRepository()
	if err != nil {
		log.Printf("canned responses: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return nil, false
	}
	return repo, true
}

// loadAccessibleCannedResponse resolves :id to a valid response the caller may read.
func loadAccessibleCannedResponse(c *gin.Context, repo *CannedResponseRepository, caller cannedResponseCaller) (*CannedResponse, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid response ID"})
		return nil, false
	}
	resp, err := repo.GetByID(id)
	if err != nil {
		log.Printf("canned responses: load %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned response"})
		return nil, false
	}
	if resp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Canned response not found"})
		return nil, false
	}
	if !caller.isAdmin && !canAccessResponse(resp, caller.userID, caller.teamID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this response"})
		return nil, false
	}
	return resp, true
}

func handleCreateCannedResponse(c *gin.Context) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
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

	if req.Name == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name and content are required"})
		return
	}
	if req.Scope == "" {
		req.Scope = "personal"
	}
	if req.Scope != "personal" && req.Scope != "team" && req.Scope != "global" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Scope must be personal, team or global"})
		return
	}
	if req.Scope == "global" && !caller.isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can create global responses"})
		return
	}

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	teamIDVal := caller.teamID
	if req.TeamID > 0 {
		teamIDVal = req.TeamID
	}

	exists, err := repo.CheckDuplicate(req.Name, req.Scope, caller.userID, teamIDVal)
	if err != nil {
		log.Printf("canned responses: duplicate check: %v", err)
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
		OwnerID:      caller.userID,
		TeamID:       teamIDVal,
		Placeholders: req.Placeholders,
	}

	id, err := repo.Create(cr, caller.userID)
	if err != nil {
		log.Printf("canned responses: create: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create canned response"})
		return
	}

	cr.ID = id
	cr.CreatedAt = time.Now()
	cr.UpdatedAt = cr.CreatedAt

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Canned response created successfully",
		"id":       id,
		"response": cr,
	})
}

// listCannedResponses lists the responses visible to the caller, narrowed by
// the shared query filters plus any route-specific overrides.
func listCannedResponses(c *gin.Context, override func(*CannedResponseFilters)) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
		return
	}
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
	if override != nil {
		override(&filters)
	}

	responses, err := repo.ListAccessible(caller.userID, caller.teamID, filters)
	if err != nil {
		log.Printf("canned responses: list: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned responses"})
		return
	}
	if responses == nil {
		responses = []*CannedResponse{}
	}
	if limit, err := strconv.Atoi(c.Query("limit")); err == nil && limit > 0 && limit < len(responses) {
		responses = responses[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"responses":   responses,
		"total_count": len(responses),
	})
}

func handleGetCannedResponses(c *gin.Context) {
	listCannedResponses(c, nil)
}

func handleGetCannedResponsesByCategory(c *gin.Context) {
	category := c.Param("category")
	listCannedResponses(c, func(f *CannedResponseFilters) { f.Category = category })
}

func handleSearchCannedResponses(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query 'q' is required"})
		return
	}
	listCannedResponses(c, func(f *CannedResponseFilters) { f.Search = q })
}

func handleGetPopularCannedResponses(c *gin.Context) {
	listCannedResponses(c, func(f *CannedResponseFilters) { f.SortBy = "usage" })
}

func handleGetCannedResponse(c *gin.Context) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
		return
	}
	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}
	resp, ok := loadAccessibleCannedResponse(c, repo, caller)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"response": resp})
}

func handleUpdateCannedResponse(c *gin.Context) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
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

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}
	existing, ok := loadAccessibleCannedResponse(c, repo, caller)
	if !ok {
		return
	}
	if !canModifyResponse(existing, caller) {
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

	if err := repo.Update(existing.ID, existing, caller.userID); err != nil {
		log.Printf("canned responses: update %d: %v", existing.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update canned response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Canned response updated successfully",
		"response": existing,
	})
}

func handleDeleteCannedResponse(c *gin.Context) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
		return
	}
	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}
	existing, ok := loadAccessibleCannedResponse(c, repo, caller)
	if !ok {
		return
	}
	if !canModifyResponse(existing, caller) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to delete this response"})
		return
	}

	if err := repo.Delete(existing.ID, caller.userID); err != nil {
		log.Printf("canned responses: delete %d: %v", existing.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete canned response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Canned response deleted successfully"})
}

func handleUseCannedResponse(c *gin.Context) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
		return
	}
	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}
	resp, ok := loadAccessibleCannedResponse(c, repo, caller)
	if !ok {
		return
	}

	var req struct {
		Context map[string]string `json:"context"`
	}
	_ = c.ShouldBindJSON(&req) //nolint:errcheck // Optional context

	content := resp.Content
	for key, value := range req.Context {
		content = strings.ReplaceAll(content, "{{"+key+"}}", value)
	}

	if err := repo.IncrementUsage(resp.ID); err != nil {
		log.Printf("canned responses: usage count %d: %v", resp.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record usage"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content":      content,
		"content_type": resp.ContentType,
		"placeholders": resp.Placeholders,
	})
}

func handleGetCannedResponseCategories(c *gin.Context) {
	if _, ok := getCannedResponseCaller(c); !ok {
		return
	}
	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	categories, err := repo.ListCategories()
	if err != nil {
		log.Printf("canned responses: categories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load categories"})
		return
	}
	if categories == nil {
		categories = []string{}
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func handleGetCannedResponseStatistics(c *gin.Context) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
		return
	}
	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	responses, err := repo.ListAccessible(caller.userID, caller.teamID, CannedResponseFilters{})
	if err != nil {
		log.Printf("canned responses: statistics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load statistics"})
		return
	}

	stats := struct {
		TotalCount int            `json:"total_count"`
		ByScope    map[string]int `json:"by_scope"`
		ByCategory map[string]int `json:"by_category"`
		TotalUsage int            `json:"total_usage"`
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
	caller, ok := getCannedResponseCaller(c)
	if !ok {
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
	if req.Scope != "personal" && req.Scope != "team" && req.Scope != "global" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Scope must be personal, team or global"})
		return
	}
	if req.Scope == "global" && !caller.isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can share globally"})
		return
	}

	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}
	resp, ok := loadAccessibleCannedResponse(c, repo, caller)
	if !ok {
		return
	}
	if !canModifyResponse(resp, caller) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to share this response"})
		return
	}

	resp.Scope = req.Scope
	if req.TeamID > 0 {
		resp.TeamID = req.TeamID
	}

	if err := repo.Update(resp.ID, resp, caller.userID); err != nil {
		log.Printf("canned responses: share %d: %v", resp.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to share canned response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Canned response shared successfully",
		"response": resp,
	})
}

func handleCopyCannedResponse(c *gin.Context) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
		return
	}
	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}
	source, ok := loadAccessibleCannedResponse(c, repo, caller)
	if !ok {
		return
	}

	dup := &CannedResponse{
		Name:         source.Name + " (Copy)",
		Category:     source.Category,
		Content:      source.Content,
		ContentType:  source.ContentType,
		Tags:         source.Tags,
		Scope:        "personal",
		OwnerID:      caller.userID,
		Placeholders: source.Placeholders,
	}

	newID, err := repo.Create(dup, caller.userID)
	if err != nil {
		log.Printf("canned responses: copy %d: %v", source.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy canned response"})
		return
	}

	dup.ID = newID
	dup.CreatedAt = time.Now()
	dup.UpdatedAt = dup.CreatedAt

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Canned response copied successfully",
		"response": dup,
	})
}

func handleExportCannedResponses(c *gin.Context) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
		return
	}
	repo, ok := getCannedResponseRepo(c)
	if !ok {
		return
	}

	responses, err := repo.ListAccessible(caller.userID, caller.teamID, CannedResponseFilters{Scope: c.Query("scope")})
	if err != nil {
		log.Printf("canned responses: export: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load canned responses"})
		return
	}

	if c.DefaultQuery("format", "json") == "csv" {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=canned_responses.csv")

		writer := csv.NewWriter(c.Writer)
		_ = writer.Write([]string{"Name", "Category", "Content", "Tags", "Scope"}) //nolint:errcheck // flushed below
		for _, r := range responses {
			_ = writer.Write([]string{r.Name, r.Category, r.Content, strings.Join(r.Tags, ","), r.Scope}) //nolint:errcheck // flushed below
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			log.Printf("canned responses: csv export: %v", err)
		}
		return
	}

	if responses == nil {
		responses = []*CannedResponse{}
	}
	c.JSON(http.StatusOK, gin.H{
		"responses": responses,
		"count":     len(responses),
	})
}

func handleImportCannedResponses(c *gin.Context) {
	caller, ok := getCannedResponseCaller(c)
	if !ok {
		return
	}

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

	records, err := csv.NewReader(file).ReadAll()
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
	for _, record := range records[1:] {
		if len(record) < 3 {
			skipped++
			continue
		}
		name := strings.TrimSpace(record[0])
		if name == "" {
			skipped++
			continue
		}

		exists, err := repo.CheckDuplicate(name, "personal", caller.userID, 0)
		if err != nil {
			log.Printf("canned responses: import duplicate check: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for duplicates"})
			return
		}
		if exists {
			skipped++
			continue
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
			Category:     strings.TrimSpace(record[1]),
			Content:      content,
			ContentType:  "text",
			Tags:         tags,
			Scope:        "personal",
			OwnerID:      caller.userID,
			Placeholders: extractPlaceholders(content),
		}
		if _, err := repo.Create(cr, caller.userID); err != nil {
			log.Printf("canned responses: import %q: %v", name, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import canned responses", "imported_count": imported})
			return
		}
		imported++
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
		return teamID > 0 && resp.TeamID == teamID
	case "global":
		return true
	default:
		return false
	}
}

func canModifyResponse(resp *CannedResponse, caller cannedResponseCaller) bool {
	if caller.isAdmin {
		return true
	}
	return resp.Scope == "personal" && resp.OwnerID == caller.userID
}

func extractPlaceholders(content string) []string {
	var placeholders []string
	seen := make(map[string]bool)

	for i := range len(content) - 3 {
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
