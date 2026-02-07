package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// handleGlobalSearch performs a search across tickets, users, and articles.
func (router *APIRouter) handleGlobalSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		sendError(c, http.StatusBadRequest, "Search query required")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		// Return empty results if DB unavailable
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"tickets":  []interface{}{},
				"users":    []interface{}{},
				"articles": []interface{}{},
				"query":    query,
			},
		})
		return
	}

	searchPattern := "%" + query + "%"
	limit := 10

	// Search tickets
	ticketQuery := database.ConvertQuery(`
		SELECT t.id, t.tn, t.title, ts.name as state, tp.name as priority
		FROM ticket t
		LEFT JOIN ticket_state ts ON ts.id = t.ticket_state_id
		LEFT JOIN ticket_priority tp ON tp.id = t.ticket_priority_id
		WHERE (t.title LIKE ? OR t.tn LIKE ?)
		AND t.archive_flag = 0
		ORDER BY t.change_time DESC
		LIMIT ?
	`)
	ticketRows, err := db.Query(ticketQuery, searchPattern, searchPattern, limit)
	tickets := []gin.H{}
	if err == nil {
		defer ticketRows.Close()
		for ticketRows.Next() {
			var id int
			var tn, title string
			var state, priority *string
			if err := ticketRows.Scan(&id, &tn, &title, &state, &priority); err == nil {
				tickets = append(tickets, gin.H{
					"id":       id,
					"number":   tn,
					"title":    title,
					"state":    state,
					"priority": priority,
					"type":     "ticket",
				})
			}
		}
	}

	// Search users (agents)
	userQuery := database.ConvertQuery(`
		SELECT id, login, CONCAT(first_name, ' ', last_name) as name
		FROM users
		WHERE (login LIKE ? OR first_name LIKE ? OR last_name LIKE ?)
		AND valid_id = 1
		ORDER BY login
		LIMIT ?
	`)
	userRows, err := db.Query(userQuery, searchPattern, searchPattern, searchPattern, limit)
	users := []gin.H{}
	if err == nil {
		defer userRows.Close()
		for userRows.Next() {
			var id int
			var login, name string
			if err := userRows.Scan(&id, &login, &name); err == nil {
				users = append(users, gin.H{
					"id":    id,
					"login": login,
					"name":  strings.TrimSpace(name),
					"type":  "user",
				})
			}
		}
	}

	// Search articles
	articleQuery := database.ConvertQuery(`
		SELECT a.id, adm.a_subject, a.ticket_id
		FROM article a
		JOIN article_data_mime adm ON adm.article_id = a.id
		WHERE adm.a_subject LIKE ? OR adm.a_body LIKE ?
		ORDER BY a.create_time DESC
		LIMIT ?
	`)
	articleRows, err := db.Query(articleQuery, searchPattern, searchPattern, limit)
	articles := []gin.H{}
	if err == nil {
		defer articleRows.Close()
		for articleRows.Next() {
			var id, ticketID int
			var subject *string
			if err := articleRows.Scan(&id, &subject, &ticketID); err == nil {
				subj := ""
				if subject != nil {
					subj = *subject
				}
				articles = append(articles, gin.H{
					"id":        id,
					"subject":   subj,
					"ticket_id": ticketID,
					"type":      "article",
				})
			}
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"tickets":  tickets,
			"users":    users,
			"articles": articles,
			"query":    query,
			"counts": gin.H{
				"tickets":  len(tickets),
				"users":    len(users),
				"articles": len(articles),
			},
		},
	})
}

// handleSearchTickets searches for tickets matching the query.
func (router *APIRouter) handleSearchTickets(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		sendError(c, http.StatusBadRequest, "Search query required")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"tickets": []interface{}{}, "total": 0},
		})
		return
	}

	// Parse limit and offset
	limit := 25
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	searchPattern := "%" + query + "%"

	// Count total matches
	countQuery := database.ConvertQuery(`
		SELECT COUNT(*) FROM ticket t
		WHERE (t.title LIKE ? OR t.tn LIKE ?)
		AND t.archive_flag = 0
	`)
	var total int
	db.QueryRow(countQuery, searchPattern, searchPattern).Scan(&total)

	// Search tickets with full details
	ticketQuery := database.ConvertQuery(`
		SELECT t.id, t.tn, t.title, t.create_time, t.change_time,
			ts.name as state, tp.name as priority, q.name as queue,
			CONCAT(u.first_name, ' ', u.last_name) as owner
		FROM ticket t
		LEFT JOIN ticket_state ts ON ts.id = t.ticket_state_id
		LEFT JOIN ticket_priority tp ON tp.id = t.ticket_priority_id
		LEFT JOIN queue q ON q.id = t.queue_id
		LEFT JOIN users u ON u.id = t.user_id
		WHERE (t.title LIKE ? OR t.tn LIKE ?)
		AND t.archive_flag = 0
		ORDER BY t.change_time DESC
		LIMIT ? OFFSET ?
	`)

	rows, err := db.Query(ticketQuery, searchPattern, searchPattern, limit, offset)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Search failed")
		return
	}
	defer rows.Close()

	tickets := []gin.H{}
	for rows.Next() {
		var id int
		var tn, title string
		var createTime, changeTime time.Time
		var state, priority, queue, owner *string

		if err := rows.Scan(&id, &tn, &title, &createTime, &changeTime, &state, &priority, &queue, &owner); err != nil {
			continue
		}

		tickets = append(tickets, gin.H{
			"id":          id,
			"number":      tn,
			"title":       title,
			"state":       state,
			"priority":    priority,
			"queue":       queue,
			"owner":       owner,
			"created_at":  createTime,
			"updated_at":  changeTime,
		})
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"tickets": tickets,
			"total":   total,
			"limit":   limit,
			"offset":  offset,
			"query":   query,
		},
	})
}

// handleSearchUsers searches for users (agents) matching the query.
func (router *APIRouter) handleSearchUsers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		sendError(c, http.StatusBadRequest, "Search query required")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"users": []interface{}{}, "total": 0},
		})
		return
	}

	limit := 25
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	searchPattern := "%" + query + "%"

	userQuery := database.ConvertQuery(`
		SELECT id, login, first_name, last_name,
			CONCAT(first_name, ' ', last_name) as full_name
		FROM users
		WHERE (login LIKE ? OR first_name LIKE ? OR last_name LIKE ?)
		AND valid_id = 1
		ORDER BY login
		LIMIT ?
	`)

	rows, err := db.Query(userQuery, searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Search failed")
		return
	}
	defer rows.Close()

	users := []gin.H{}
	for rows.Next() {
		var id int
		var login string
		var firstName, lastName, fullName *string

		if err := rows.Scan(&id, &login, &firstName, &lastName, &fullName); err != nil {
			continue
		}

		users = append(users, gin.H{
			"id":         id,
			"login":      login,
			"first_name": firstName,
			"last_name":  lastName,
			"full_name":  strings.TrimSpace(fmt.Sprintf("%v %v", firstName, lastName)),
		})
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"users": users,
			"total": len(users),
			"query": query,
		},
	})
}

// handleSearchSuggestions returns search suggestions based on partial input.
func (router *APIRouter) handleSearchSuggestions(c *gin.Context) {
	query := c.Query("q")
	if query == "" || len(query) < 2 {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    []string{},
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    []string{query},
		})
		return
	}

	suggestions := []string{}
	searchPattern := query + "%"

	// Get ticket number suggestions
	tnQuery := database.ConvertQuery(`
		SELECT DISTINCT tn FROM ticket
		WHERE tn LIKE ?
		AND archive_flag = 0
		LIMIT 5
	`)
	tnRows, _ := db.Query(tnQuery, searchPattern)
	if tnRows != nil {
		defer tnRows.Close()
		for tnRows.Next() {
			var tn string
			if tnRows.Scan(&tn) == nil {
				suggestions = append(suggestions, tn)
			}
		}
	}

	// Get title suggestions
	titleQuery := database.ConvertQuery(`
		SELECT DISTINCT title FROM ticket
		WHERE title LIKE ?
		AND archive_flag = 0
		LIMIT 5
	`)
	titleRows, _ := db.Query(titleQuery, searchPattern)
	if titleRows != nil {
		defer titleRows.Close()
		for titleRows.Next() {
			var title string
			if titleRows.Scan(&title) == nil && len(suggestions) < 10 {
				suggestions = append(suggestions, title)
			}
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    suggestions,
	})
}

// Saved search handlers using the search_profile table.
func (router *APIRouter) handleGetSavedSearches(c *gin.Context) {
	userLogin := "admin"
	if login, exists := c.Get("user_login"); exists {
		if loginStr, ok := login.(string); ok {
			userLogin = loginStr
		}
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{Success: true, Data: gin.H{"searches": []interface{}{}, "total": 0}})
		return
	}

	// Get distinct profile names for this user
	query := database.ConvertQuery(`
		SELECT DISTINCT profile_name, profile_type
		FROM search_profile
		WHERE login = ?
		ORDER BY profile_name
	`)

	rows, err := db.Query(query, userLogin)
	searches := []gin.H{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, profileType string
			if rows.Scan(&name, &profileType) == nil {
				searches = append(searches, gin.H{
					"name": name,
					"type": profileType,
				})
			}
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"searches": searches, "total": len(searches)},
	})
}

func (router *APIRouter) handleCreateSavedSearch(c *gin.Context) {
	var req struct {
		Name       string            `json:"name" binding:"required"`
		Type       string            `json:"type"`
		Parameters map[string]string `json:"parameters"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	userLogin := "admin"
	if login, exists := c.Get("user_login"); exists {
		if loginStr, ok := login.(string); ok {
			userLogin = loginStr
		}
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	profileType := req.Type
	if profileType == "" {
		profileType = "TicketSearch"
	}

	// Insert each parameter as a key-value pair
	insertQuery := database.ConvertQuery(`
		INSERT INTO search_profile (login, profile_name, profile_type, profile_key, profile_value)
		VALUES (?, ?, ?, ?, ?)
	`)

	for key, value := range req.Parameters {
		db.Exec(insertQuery, userLogin, req.Name, profileType, key, value)
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    gin.H{"name": req.Name, "type": profileType},
	})
}

func (router *APIRouter) handleGetSavedSearch(c *gin.Context) {
	searchName := c.Param("id")

	userLogin := "admin"
	if login, exists := c.Get("user_login"); exists {
		if loginStr, ok := login.(string); ok {
			userLogin = loginStr
		}
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT profile_type, profile_key, profile_value
		FROM search_profile
		WHERE login = ? AND profile_name = ?
	`)

	rows, err := db.Query(query, userLogin, searchName)
	if err != nil {
		sendError(c, http.StatusNotFound, "Search profile not found")
		return
	}
	defer rows.Close()

	var profileType string
	parameters := map[string]string{}
	for rows.Next() {
		var pType, key string
		var value *string
		if rows.Scan(&pType, &key, &value) == nil {
			profileType = pType
			if value != nil {
				parameters[key] = *value
			}
		}
	}

	if profileType == "" {
		sendError(c, http.StatusNotFound, "Search profile not found")
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"name": searchName, "type": profileType, "parameters": parameters},
	})
}

func (router *APIRouter) handleUpdateSavedSearch(c *gin.Context) {
	searchName := c.Param("id")

	var req struct {
		Parameters map[string]string `json:"parameters"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	userLogin := "admin"
	if login, exists := c.Get("user_login"); exists {
		if loginStr, ok := login.(string); ok {
			userLogin = loginStr
		}
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Delete existing parameters
	deleteQuery := database.ConvertQuery(`
		DELETE FROM search_profile WHERE login = ? AND profile_name = ?
	`)
	db.Exec(deleteQuery, userLogin, searchName)

	// Insert new parameters
	insertQuery := database.ConvertQuery(`
		INSERT INTO search_profile (login, profile_name, profile_type, profile_key, profile_value)
		VALUES (?, ?, 'TicketSearch', ?, ?)
	`)

	for key, value := range req.Parameters {
		db.Exec(insertQuery, userLogin, searchName, key, value)
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Search profile updated",
	})
}

func (router *APIRouter) handleDeleteSavedSearch(c *gin.Context) {
	searchName := c.Param("id")

	userLogin := "admin"
	if login, exists := c.Get("user_login"); exists {
		if loginStr, ok := login.(string); ok {
			userLogin = loginStr
		}
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		DELETE FROM search_profile WHERE login = ? AND profile_name = ?
	`)

	result, err := db.Exec(query, userLogin, searchName)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to delete search profile")
		return
	}

	if affected, _ := result.RowsAffected(); affected == 0 {
		sendError(c, http.StatusNotFound, "Search profile not found")
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (router *APIRouter) handleExecuteSavedSearch(c *gin.Context) {
	searchName := c.Param("id")

	userLogin := "admin"
	if login, exists := c.Get("user_login"); exists {
		if loginStr, ok := login.(string); ok {
			userLogin = loginStr
		}
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Get search parameters
	query := database.ConvertQuery(`
		SELECT profile_key, profile_value
		FROM search_profile
		WHERE login = ? AND profile_name = ?
	`)

	rows, err := db.Query(query, userLogin, searchName)
	if err != nil {
		sendError(c, http.StatusNotFound, "Search profile not found")
		return
	}
	defer rows.Close()

	parameters := map[string]string{}
	for rows.Next() {
		var key string
		var value *string
		if rows.Scan(&key, &value) == nil && value != nil {
			parameters[key] = *value
		}
	}

	if len(parameters) == 0 {
		sendError(c, http.StatusNotFound, "Search profile not found")
		return
	}

	// Execute a basic search - would be extended to build full query from parameters
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"parameters": parameters, "message": "Execute saved search with these parameters"},
	})
}
