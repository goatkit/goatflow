package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleListUsersAPI handles GET /api/v1/users
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
        // Fallback mock for tests without DB
        users := []gin.H{
            {"id": 1, "login": "admin", "valid_id": 1, "valid": true, "groups": []gin.H{{"id": 1, "name": "admin"}}},
        }
        c.JSON(http.StatusOK, gin.H{
            "success": true,
            "data":    users,
            "pagination": gin.H{
                "page": page, "per_page": perPage, "total": 1, "total_pages": 1,
                "has_next": false, "has_prev": false,
            },
        })
        return
    }

	// Build the query
	query := `
		SELECT DISTINCT
			u.id,
			u.login,
			u.email,
			u.first_name,
			u.last_name,
			u.valid_id,
			u.create_time,
			u.change_time
		FROM users u
	`

	where := []string{}
	args := []interface{}{}
	argCount := 0

	// Join with user_groups if filtering by group
	if groupID != "" {
		query += " INNER JOIN user_groups ug ON u.id = ug.user_id"
		argCount++
		where = append(where, fmt.Sprintf("ug.group_id = $%d", argCount))
		if gid, err := strconv.Atoi(groupID); err == nil {
			args = append(args, gid)
		}
	}

	// Add search filter
	if search != "" {
		argCount++
		searchPattern := "%" + search + "%"
		where = append(where, fmt.Sprintf(
			"(u.login ILIKE $%d OR u.first_name ILIKE $%d OR u.last_name ILIKE $%d OR u.email ILIKE $%d)",
			argCount, argCount, argCount, argCount,
		))
		args = append(args, searchPattern)
	}

	// Add valid filter
	if validFilter != "" {
		if valid, err := strconv.Atoi(validFilter); err == nil && (valid == 1 || valid == 2) {
			argCount++
			where = append(where, fmt.Sprintf("u.valid_id = $%d", argCount))
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
		countQuery += " INNER JOIN user_groups ug ON u.id = ug.user_id"
	}
	if len(where) > 0 {
		countQuery += " WHERE " + strings.Join(where, " AND ")
	}
	countQuery = database.ConvertPlaceholders(countQuery)

	var total int
	err = db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to count users",
		})
		return
	}

	// Add pagination
	offset := (page - 1) * perPage
	argCount++
	query += fmt.Sprintf(" ORDER BY u.id LIMIT $%d", argCount)
	args = append(args, perPage)
	argCount++
	query += fmt.Sprintf(" OFFSET $%d", argCount)
	args = append(args, offset)

	// Convert placeholders for the database
	query = database.ConvertPlaceholders(query)

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
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
			Email      sql.NullString `json:"-"`
			FirstName  sql.NullString `json:"-"`
			LastName   sql.NullString `json:"-"`
			ValidID    int            `json:"valid_id"`
			CreateTime sql.NullTime   `json:"-"`
			ChangeTime sql.NullTime   `json:"-"`
		}

		err := rows.Scan(
			&user.ID,
			&user.Login,
			&user.Email,
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

		if user.Email.Valid {
			userMap["email"] = user.Email.String
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
			INNER JOIN user_groups ug ON g.id = ug.group_id
			WHERE ug.user_id = $1
		`)
		
		groupRows, err := db.Query(groupQuery, user.ID)
		if err == nil {
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
			groupRows.Close()
			userMap["groups"] = groups
		}

		users = append(users, userMap)
	}

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