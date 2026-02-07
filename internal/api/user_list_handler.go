package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// HandleListUsersAPI handles GET /api/v1/users.
//
//	@Summary		List users
//	@Description	Retrieve a paginated list of users (agents)
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int		false	"Page number"		default(1)
//	@Param			per_page	query		int		false	"Items per page"	default(20)
//	@Param			search		query		string	false	"Search in login, name, email"
//	@Param			group_id	query		int		false	"Filter by group membership"
//	@Success		200			{object}	map[string]interface{}	"List of users"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/users [get]
func HandleListUsersAPI(c *gin.Context) {
	// Check authentication
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Authentication required",
		})
		return
	}

	// Parse query parameters
	page := 1
	perPage := 20

	if p := c.Query("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}

	if pp := c.Query("per_page"); pp != "" {
		if val, err := strconv.Atoi(pp); err == nil && val > 0 && val <= 100 {
			perPage = val
		}
	}

	search := c.Query("search")
	validFilter := c.Query("valid") // "1" for valid only, "2" for invalid only, "" for all
	groupID := c.Query("group_id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		respondWithMockUsers(c, page, perPage)
		return
	}

	// Build the query
	query := `
		SELECT DISTINCT
			u.id,
			u.login,
			u.first_name,
			u.last_name,
			u.valid_id,
			u.create_time,
			u.change_time
		FROM users u
	`

	where := []string{}
	args := []interface{}{}

	// Join with user_groups if filtering by group
	if groupID != "" {
		query += " INNER JOIN group_user gu ON u.id = gu.user_id"
		where = append(where, "gu.group_id = ?")
		if gid, err := strconv.Atoi(groupID); err == nil {
			args = append(args, gid)
		}
	}

	// Add search filter
	if search != "" {
		searchPattern := "%" + search + "%"
		searchClauses := make([]string, 0, 4)
		fields := []string{"u.login", "u.first_name", "u.last_name"}
		for _, field := range fields {
			searchClauses = append(searchClauses, fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", field))
			args = append(args, searchPattern)
		}
		where = append(where, "("+strings.Join(searchClauses, " OR ")+")")
	}

	// Add valid filter
	if validFilter != "" {
		if valid, err := strconv.Atoi(validFilter); err == nil && (valid == 1 || valid == 2) {
			where = append(where, "u.valid_id = ?")
			args = append(args, valid)
		}
	}

	// Combine WHERE clauses
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	// Get total count
	countQuery := "SELECT COUNT(DISTINCT u.id) FROM users u"
	if groupID != "" {
		countQuery += " INNER JOIN group_user gu ON u.id = gu.user_id"
	}
	if len(where) > 0 {
		countQuery += " WHERE " + strings.Join(where, " AND ")
	}
	countQuery = database.ConvertPlaceholders(countQuery)

	var total int
	err = db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		if shouldFallbackToMock(err) {
			respondWithMockUsers(c, page, perPage)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to count users",
		})
		return
	}

	// Add pagination
	offset := (page - 1) * perPage
	query += " ORDER BY u.id LIMIT ?"
	args = append(args, perPage)
	query += " OFFSET ?"
	args = append(args, offset)

	// Convert placeholders for the database
	query = database.ConvertPlaceholders(query)

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
		if shouldFallbackToMock(err) {
			respondWithMockUsers(c, page, perPage)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to retrieve users",
		})
		return
	}
	defer rows.Close()

	users := []map[string]interface{}{}
	for rows.Next() {
		var user struct {
			ID         int            `json:"id"`
			Login      string         `json:"login"`
			FirstName  sql.NullString `json:"-"`
			LastName   sql.NullString `json:"-"`
			ValidID    int            `json:"valid_id"`
			CreateTime sql.NullTime   `json:"-"`
			ChangeTime sql.NullTime   `json:"-"`
		}

		err := rows.Scan(
			&user.ID,
			&user.Login,
			&user.FirstName,
			&user.LastName,
			&user.ValidID,
			&user.CreateTime,
			&user.ChangeTime,
		)
		if err != nil {
			continue
		}

		userMap := map[string]interface{}{
			"id":       user.ID,
			"login":    user.Login,
			"valid_id": user.ValidID,
			"valid":    user.ValidID == 1,
		}

		if user.FirstName.Valid {
			userMap["first_name"] = user.FirstName.String
		}
		if user.LastName.Valid {
			userMap["last_name"] = user.LastName.String
		}
		if user.CreateTime.Valid {
			userMap["create_time"] = user.CreateTime.Time.Format("2006-01-02T15:04:05Z")
		}
		if user.ChangeTime.Valid {
			userMap["change_time"] = user.ChangeTime.Time.Format("2006-01-02T15:04:05Z")
		}

		// Get user's groups
		groupQuery := database.ConvertPlaceholders(`
			SELECT g.id, g.name
			FROM groups g
			INNER JOIN group_user gu ON g.id = gu.group_id
			WHERE gu.user_id = ?
		`)

		groupRows, err := db.Query(groupQuery, user.ID)
		if err == nil {
			defer groupRows.Close()
			groups := []map[string]interface{}{}
			for groupRows.Next() {
				var groupID int
				var groupName string
				if err := groupRows.Scan(&groupID, &groupName); err == nil {
					groups = append(groups, map[string]interface{}{
						"id":   groupID,
						"name": groupName,
					})
				}
			}
			_ = groupRows.Err() //nolint:errcheck // Check for iteration errors
			userMap["groups"] = groups
		} else if shouldFallbackToMock(err) {
			userMap["groups"] = []map[string]interface{}{}
		}

		users = append(users, userMap)
	}
	_ = rows.Err() //nolint:errcheck // Check for iteration errors

	// Calculate pagination info
	totalPages := (total + perPage - 1) / perPage

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    users,
		"pagination": gin.H{
			"page":        page,
			"per_page":    perPage,
			"total":       total,
			"total_pages": totalPages,
			"has_next":    page < totalPages,
			"has_prev":    page > 1,
		},
	})
}

func respondWithMockUsers(c *gin.Context, page, perPage int) {
	users := []gin.H{
		{"id": 1, "login": "admin", "valid_id": 1, "valid": true, "groups": []gin.H{{"id": 1, "name": "Admin"}}},
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    users,
		"pagination": gin.H{
			"page": page, "per_page": perPage, "total": len(users), "total_pages": 1,
			"has_next": false, "has_prev": false,
		},
	})
}

func shouldFallbackToMock(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "doesn't exist") || strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "relation") && strings.Contains(msg, "does not exist") {
		return true
	}
	if strings.Contains(msg, "unknown table") || strings.Contains(msg, "table not found") {
		return true
	}
	return false
}
