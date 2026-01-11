package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// ACL represents an Access Control List entry.
type ACL struct {
	ID             int              `json:"id"`
	Name           string           `json:"name"`
	Comments       *string          `json:"comments,omitempty"`
	Description    *string          `json:"description,omitempty"`
	ValidID        int              `json:"valid_id"`
	StopAfterMatch *int             `json:"stop_after_match,omitempty"`
	ConfigMatch    *ACLConfigMatch  `json:"config_match,omitempty"`
	ConfigChange   *ACLConfigChange `json:"config_change,omitempty"`
	CreateTime     time.Time        `json:"create_time"`
	CreateBy       int              `json:"create_by"`
	ChangeTime     time.Time        `json:"change_time"`
	ChangeBy       int              `json:"change_by"`
}

// ACLConfigMatch defines the conditions for when an ACL applies.
type ACLConfigMatch struct {
	Properties map[string]interface{} `json:"Properties,omitempty"`
}

// ACLConfigChange defines what changes the ACL makes.
type ACLConfigChange struct {
	Possible    map[string]interface{} `json:"Possible,omitempty"`
	PossibleNot map[string]interface{} `json:"PossibleNot,omitempty"`
	PossibleAdd map[string]interface{} `json:"PossibleAdd,omitempty"`
}

// handleAdminACL renders the admin ACL management page.
func handleAdminACL(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get search and filter parameters
	searchQuery := c.Query("search")
	validFilter := c.DefaultQuery("valid", "all")
	sortBy := c.DefaultQuery("sort", "name")
	sortOrder := c.DefaultQuery("order", "asc")

	// Build query with filters
	query := `
		SELECT
			id, name, comments, description, valid_id,
			stop_after_match, config_match, config_change,
			create_time, create_by, change_time, change_by
		FROM acl
		WHERE 1=1
	`

	var args []interface{}
	argCount := 1

	if searchQuery != "" {
		query += fmt.Sprintf(" AND (LOWER(name) LIKE $%d OR LOWER(description) LIKE $%d OR LOWER(comments) LIKE $%d)", argCount, argCount+1, argCount+2)
		searchPattern := "%" + strings.ToLower(searchQuery) + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
		argCount += 3
	}

	if validFilter != "all" {
		if validFilter == "valid" {
			query += fmt.Sprintf(" AND valid_id = $%d", argCount)
			args = append(args, 1)
			argCount++
		} else if validFilter == "invalid" {
			query += fmt.Sprintf(" AND valid_id = $%d", argCount)
			args = append(args, 2)
			argCount++
		}
	}

	// Add sorting
	validSortColumns := map[string]bool{
		"id": true, "name": true, "valid_id": true, "change_time": true,
	}
	if !validSortColumns[sortBy] {
		sortBy = "name"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	rows, err := db.Query(query, args...)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch ACLs")
		return
	}
	defer rows.Close()

	var acls []ACL
	for rows.Next() {
		var a ACL
		var comments, description sql.NullString
		var stopAfterMatch sql.NullInt32
		var configMatch, configChange []byte

		err := rows.Scan(
			&a.ID, &a.Name, &comments, &description, &a.ValidID,
			&stopAfterMatch, &configMatch, &configChange,
			&a.CreateTime, &a.CreateBy, &a.ChangeTime, &a.ChangeBy,
		)
		if err != nil {
			continue
		}

		if comments.Valid {
			a.Comments = &comments.String
		}
		if description.Valid {
			a.Description = &description.String
		}
		if stopAfterMatch.Valid {
			val := int(stopAfterMatch.Int32)
			a.StopAfterMatch = &val
		}

		// Parse config_match JSON
		if len(configMatch) > 0 {
			var cm ACLConfigMatch
			if err := json.Unmarshal(configMatch, &cm); err == nil {
				a.ConfigMatch = &cm
			}
		}

		// Parse config_change JSON
		if len(configChange) > 0 {
			var cc ACLConfigChange
			if err := json.Unmarshal(configChange, &cc); err == nil {
				a.ConfigChange = &cc
			}
		}

		acls = append(acls, a)
	}
	_ = rows.Err()

	// Check if JSON response is requested
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    acls,
		})
		return
	}

	// Render the template
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/acl.pongo2", pongo2.Context{
		"Title":       "ACL Management",
		"ACLs":        acls,
		"SearchQuery": searchQuery,
		"ValidFilter": validFilter,
		"SortBy":      sortBy,
		"SortOrder":   sortOrder,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}

// handleAdminACLCreate creates a new ACL.
func handleAdminACLCreate(c *gin.Context) {
	var input struct {
		Name           string                  `json:"name" binding:"required"`
		Comments       *string                 `json:"comments"`
		Description    *string                 `json:"description"`
		ValidID        int                     `json:"valid_id"`
		StopAfterMatch *int                    `json:"stop_after_match"`
		ConfigMatch    *map[string]interface{} `json:"config_match"`
		ConfigChange   *map[string]interface{} `json:"config_change"`
	}

	// Default valid_id to 1 if not provided
	input.ValidID = 1

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name is required",
		})
		return
	}

	// Validate name is not empty
	if strings.TrimSpace(input.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Name is required",
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

	// Check for duplicate name
	var exists bool
	err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM acl WHERE name = ?)"), input.Name).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check for duplicate",
		})
		return
	}

	if exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ACL with this name already exists",
		})
		return
	}

	// Serialize config_match and config_change to JSON bytes
	var configMatchBytes, configChangeBytes []byte
	if input.ConfigMatch != nil {
		configMatchBytes, _ = json.Marshal(input.ConfigMatch)
	}
	if input.ConfigChange != nil {
		configChangeBytes, _ = json.Marshal(input.ConfigChange)
	}

	// Default stop_after_match to 0 if not provided
	stopAfterMatch := 0
	if input.StopAfterMatch != nil {
		stopAfterMatch = *input.StopAfterMatch
	}

	// Insert the new ACL
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO acl (
			name, comments, description, valid_id, stop_after_match,
			config_match, config_change,
			create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?,
			CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
		) RETURNING id
	`)

	adapter := database.GetAdapter()
	id64, err := adapter.InsertWithReturning(db, insertQuery,
		input.Name, input.Comments, input.Description,
		input.ValidID, stopAfterMatch,
		configMatchBytes, configChangeBytes)
	id := int(id64)

	if err != nil {
		// Check for duplicate key error
		errStr := err.Error()
		if strings.Contains(errStr, "Duplicate entry") || strings.Contains(errStr, "duplicate key") {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "ACL with this name already exists",
			})
			return
		}
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "ACL with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create ACL: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "ACL created successfully",
		"data": gin.H{
			"id":   id,
			"name": input.Name,
		},
	})
}

// handleAdminACLUpdate updates an existing ACL.
func handleAdminACLUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ACL ID",
		})
		return
	}

	var input struct {
		Name           *string                 `json:"name"`
		Comments       *string                 `json:"comments"`
		Description    *string                 `json:"description"`
		ValidID        *int                    `json:"valid_id"`
		StopAfterMatch *int                    `json:"stop_after_match"`
		ConfigMatch    *map[string]interface{} `json:"config_match"`
		ConfigChange   *map[string]interface{} `json:"config_change"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input",
		})
		return
	}

	// Build update query dynamically
	updates := []string{"change_time = CURRENT_TIMESTAMP", "change_by = 1"}
	args := []interface{}{}

	if input.Name != nil && *input.Name != "" {
		updates = append(updates, "name = ?")
		args = append(args, *input.Name)
	}

	if input.Comments != nil {
		updates = append(updates, "comments = ?")
		args = append(args, *input.Comments)
	}

	if input.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *input.Description)
	}

	if input.ValidID != nil {
		updates = append(updates, "valid_id = ?")
		args = append(args, *input.ValidID)
	}

	if input.StopAfterMatch != nil {
		updates = append(updates, "stop_after_match = ?")
		args = append(args, *input.StopAfterMatch)
	}

	if input.ConfigMatch != nil {
		configMatchBytes, _ := json.Marshal(input.ConfigMatch)
		updates = append(updates, "config_match = ?")
		args = append(args, configMatchBytes)
	}

	if input.ConfigChange != nil {
		configChangeBytes, _ := json.Marshal(input.ConfigChange)
		updates = append(updates, "config_change = ?")
		args = append(args, configChangeBytes)
	}

	args = append(args, id)

	qb, err := database.GetQueryBuilder()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	query := database.ConvertPlaceholders("UPDATE acl SET " + strings.Join(updates, ", ") + " WHERE id = ?")
	result, err := qb.DB().Exec(query, args...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "ACL with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update ACL",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "ACL not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "ACL updated successfully",
	})
}

// handleAdminACLDelete soft deletes an ACL.
func handleAdminACLDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ACL ID",
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

	// Soft delete (mark as invalid)
	result, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE acl
		SET valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = 1
		WHERE id = ?
	`), id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete ACL",
		})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "ACL not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "ACL deleted successfully",
	})
}

// handleAdminACLGet returns a single ACL by ID.
func handleAdminACLGet(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ACL ID",
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

	query := database.ConvertPlaceholders(`
		SELECT
			id, name, comments, description, valid_id,
			stop_after_match, config_match, config_change,
			create_time, create_by, change_time, change_by
		FROM acl
		WHERE id = ?
	`)

	var a ACL
	var comments, description sql.NullString
	var stopAfterMatch sql.NullInt32
	var configMatch, configChange []byte

	err = db.QueryRow(query, id).Scan(
		&a.ID, &a.Name, &comments, &description, &a.ValidID,
		&stopAfterMatch, &configMatch, &configChange,
		&a.CreateTime, &a.CreateBy, &a.ChangeTime, &a.ChangeBy,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "ACL not found",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch ACL",
		})
		return
	}

	if comments.Valid {
		a.Comments = &comments.String
	}
	if description.Valid {
		a.Description = &description.String
	}
	if stopAfterMatch.Valid {
		val := int(stopAfterMatch.Int32)
		a.StopAfterMatch = &val
	}

	// Parse config_match JSON
	if len(configMatch) > 0 {
		var cm ACLConfigMatch
		if err := json.Unmarshal(configMatch, &cm); err == nil {
			a.ConfigMatch = &cm
		}
	}

	// Parse config_change JSON
	if len(configChange) > 0 {
		var cc ACLConfigChange
		if err := json.Unmarshal(configChange, &cc); err == nil {
			a.ConfigChange = &cc
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    a,
	})
}
