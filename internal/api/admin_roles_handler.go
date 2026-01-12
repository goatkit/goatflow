package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// Role represents a role in the system.
type Role struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Comments    *string   `json:"comments"`
	Description string    `json:"description"` // Computed from Comments for template
	ValidID     int       `json:"valid_id"`
	CreateTime  time.Time `json:"create_time"`
	CreateBy    int       `json:"create_by"`
	ChangeTime  time.Time `json:"change_time"`
	ChangeBy    int       `json:"change_by"`
	UserCount   int       `json:"user_count"`
	GroupCount  int       `json:"group_count"`
	IsActive    bool      `json:"is_active"`   // Computed from ValidID
	IsSystem    bool      `json:"is_system"`   // True for built-in roles
	Permissions []string  `json:"permissions"` // Simple permissions list
}

// RoleUser represents a user assigned to a role.
type RoleUser struct {
	UserID    int    `json:"user_id"`
	Login     string `json:"login"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// RoleGroup represents group permissions for a role.
type RoleGroupPermission struct {
	GroupID     int             `json:"group_id"`
	GroupName   string          `json:"group_name"`
	Permissions map[string]bool `json:"permissions"`
}

// handleAdminRoles displays the admin roles management page.
func handleAdminRoles(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get search and filter parameters
	searchQuery := c.Query("search")
	validFilter := c.DefaultQuery("valid", "all")

	// Build query - uses MariaDB compatible syntax
	query := `
		SELECT
			r.id, r.name, r.comments, r.valid_id,
			r.create_time, r.create_by, r.change_time, r.change_by,
			COUNT(DISTINCT ru.user_id) as user_count,
			COUNT(DISTINCT gr.group_id) as group_count
		FROM roles r
		LEFT JOIN role_user ru ON r.id = ru.role_id
		LEFT JOIN group_role gr ON r.id = gr.role_id
		WHERE 1=1
	`

	var args []interface{}

	if searchQuery != "" {
		query += " AND (LOWER(r.name) LIKE ? OR LOWER(r.comments) LIKE ?)"
		searchPattern := "%" + searchQuery + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if validFilter != "all" {
		if validFilter == "valid" {
			query += " AND r.valid_id = ?"
			args = append(args, 1)
		} else if validFilter == "invalid" {
			query += " AND r.valid_id = ?"
			args = append(args, 2)
		}
	}

	query += " GROUP BY r.id, r.name, r.comments, r.valid_id, r.create_time, r.create_by, r.change_time, r.change_by"
	query += " ORDER BY r.name ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch roles: "+err.Error())
		return
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		var r Role
		var comments sql.NullString

		err := rows.Scan(
			&r.ID, &r.Name, &comments, &r.ValidID,
			&r.CreateTime, &r.CreateBy, &r.ChangeTime, &r.ChangeBy,
			&r.UserCount, &r.GroupCount,
		)
		if err != nil {
			continue
		}

		if comments.Valid {
			r.Comments = &comments.String
			r.Description = comments.String
		}

		// Fetch permissions from group_role table
		r.Permissions, _ = fetchRolePermissions(db, r.ID)

		// Set computed fields
		r.IsActive = r.ValidID == 1
		r.IsSystem = r.ID <= 3 // First 3 roles are system roles

		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		c.String(http.StatusInternalServerError, "Error iterating roles: "+err.Error())
		return
	}

	// Render the template
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/roles.pongo2", pongo2.Context{
		"Title":       "Role Management",
		"Roles":       roles,
		"SearchQuery": searchQuery,
		"ValidFilter": validFilter,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}

// handleAdminRoleCreate creates a new role.
func handleAdminRoleCreate(c *gin.Context) {
	var input struct {
		Name        string   `json:"name" binding:"required"`
		Comments    string   `json:"comments"`
		Description string   `json:"description"`
		ValidID     int      `json:"valid_id"`
		Permissions []string `json:"permissions"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input: " + err.Error(),
		})
		return
	}

	if input.ValidID == 0 {
		input.ValidID = 1
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
	err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM roles WHERE name = ?)"), input.Name).Scan(&exists)
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
			"error":   "Role with this name already exists",
		})
		return
	}

	// Use Comments or Description (Description from frontend)
	var comments string
	if input.Comments != "" {
		comments = input.Comments
	} else if input.Description != "" {
		comments = input.Description
	}

	// Insert the new role using database adapter for MySQL/PostgreSQL compatibility
	var commentsPtr *string
	if comments != "" {
		commentsPtr = &comments
	}

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO roles (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id
	`)

	adapter := database.GetAdapter()
	id64, err := adapter.InsertWithReturning(db, insertQuery, input.Name, commentsPtr, input.ValidID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create role: " + err.Error(),
		})
		return
	}

	roleID := int(id64)

	// Save permissions to group_role table
	if len(input.Permissions) > 0 {
		if err := saveRolePermissions(db, roleID, input.Permissions); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to save permissions: " + err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Role created successfully",
		"data": gin.H{
			"id":   roleID,
			"name": input.Name,
		},
	})
}

// handleAdminRoleGet retrieves a single role by ID.
func handleAdminRoleGet(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid role ID",
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

	// Get the role details
	var role Role
	var comments sql.NullString
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id
		FROM roles
		WHERE id = ?
	`), id).Scan(&role.ID, &role.Name, &comments, &role.ValidID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Role not found",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch role",
		})
		return
	}

	if comments.Valid {
		role.Comments = &comments.String
	}

	// Fetch permissions from group_role table
	role.Permissions, err = fetchRolePermissions(db, id)
	if err != nil {
		role.Permissions = []string{}
	}

	// Return role data in format expected by JavaScript
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"role": gin.H{
			"ID":          role.ID,
			"Name":        role.Name,
			"Description": role.Comments,
			"IsActive":    role.ValidID == 1,
			"Permissions": role.Permissions,
		},
	})
}

// handleAdminRoleUpdate updates an existing role.
func handleAdminRoleUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid role ID",
		})
		return
	}

	var input struct {
		Name        string   `json:"name"`
		Comments    string   `json:"comments"`
		Description string   `json:"description"`
		ValidID     int      `json:"valid_id"`
		Permissions []string `json:"permissions"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input: " + err.Error(),
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

	// Use Comments or Description (Description from frontend)
	var comments string
	if input.Comments != "" {
		comments = input.Comments
	} else if input.Description != "" {
		comments = input.Description
	}

	var commentsVal interface{} = comments
	if comments == "" {
		commentsVal = nil
	}

	var validIDVal = input.ValidID
	if validIDVal == 0 {
		validIDVal = 1 // Default to valid if not specified
	}

	// Update the role
	result, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE roles
		SET name = COALESCE(NULLIF(?, ''), name),
		    comments = ?,
		    valid_id = ?,
		    change_time = CURRENT_TIMESTAMP,
		    change_by = 1
		WHERE id = ?
	`), input.Name, commentsVal, validIDVal, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update role: " + err.Error(),
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
			"error":   "Role not found",
		})
		return
	}

	// Update permissions if provided
	if len(input.Permissions) > 0 {
		if err := saveRolePermissions(db, id, input.Permissions); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to update permissions: " + err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Role updated successfully",
	})
}

// saveRolePermissions saves role permissions to the group_role table.
// Maps simple permission strings to group_role permission keys.
func saveRolePermissions(db *sql.DB, roleID int, permissions []string) error {
	if len(permissions) == 0 {
		return nil
	}

	// Check if "*" (all permissions) is in the list
	hasAll := false
	for _, p := range permissions {
		if p == "*" {
			hasAll = true
			break
		}
	}

	// Get all groups to assign permissions to
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id FROM groups WHERE valid_id = 1 ORDER BY id
	`))
	if err != nil {
		return err
	}
	defer rows.Close()

	var groupIDs []int
	for rows.Next() {
		var groupID int
		if err := rows.Scan(&groupID); err != nil {
			continue
		}
		groupIDs = append(groupIDs, groupID)
	}

	if len(groupIDs) == 0 {
		return nil
	}

	// Define mapping from simple permissions to group_role permission keys
	permMapping := map[string]string{
		"view_tickets":     "ro",
		"create_tickets":   "create",
		"edit_tickets":     "note",
		"delete_tickets":   "owner",
		"assign_tickets":   "move_into",
		"manage_users":     "rw",
		"manage_settings":  "rw",
		"manage_priorities": "priority",
	}

	var permKeys []string
	if hasAll {
		// All permissions means full rw access on all groups
		permKeys = []string{"ro", "move_into", "create", "owner", "priority", "rw", "note"}
	} else {
		// Map each frontend permission to its database equivalent
		seen := make(map[string]bool)
		for _, p := range permissions {
			if key, ok := permMapping[p]; ok && !seen[key] {
				permKeys = append(permKeys, key)
				seen[key] = true
			}
		}
	}

	if len(permKeys) == 0 {
		return nil
	}

	// Clear existing permissions for this role first
	if _, err := db.Exec(database.ConvertPlaceholders(`
		DELETE FROM group_role WHERE role_id = ?
	`), roleID); err != nil {
		return err
	}

	// Insert new permissions for each group
	for _, groupID := range groupIDs {
		for _, permKey := range permKeys {
			if _, err := db.Exec(database.ConvertPlaceholders(`
				INSERT INTO group_role (role_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
				VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
			`), roleID, groupID, permKey); err != nil {
				return err
			}
		}
	}

	return nil
}

// frontendPermMapping maps raw database permission keys to frontend-friendly names.
// Some database permissions map to multiple frontend permissions.
var frontendPermMapping = map[string]string{
	"ro":         "view_tickets",
	"move_into":  "assign_tickets",
	"create":     "create_tickets",
	"note":       "edit_tickets",
}

func mapDatabaseToFrontendPermissions(rawPermissions []string) []string {
	var permissions []string

	// Handle special cases where one DB permission maps to multiple frontend permissions
	permissionMap := make(map[string]bool)

	for _, perm := range rawPermissions {
		switch perm {
		case "rw":
			// "rw" permission enables both manage_users and manage_settings
			permissionMap["manage_users"] = true
			permissionMap["manage_settings"] = true
		case "owner":
			// "owner" enables delete_tickets
			permissionMap["delete_tickets"] = true
		case "priority":
			// "priority" enables manage_priorities
			permissionMap["manage_priorities"] = true
		default:
			// Use the standard mapping for other permissions
			if name, ok := frontendPermMapping[perm]; ok {
				permissionMap[name] = true
			}
		}
	}

	// Convert map to slice
	for perm := range permissionMap {
		permissions = append(permissions, perm)
	}

	return permissions
}

// allRawPermissions contains all possible raw permission keys for full access.
var allRawPermissions = []string{"ro", "move_into", "create", "owner", "priority", "rw", "note"}

// fetchRolePermissions fetches unique permission keys from group_role table for a role.
func fetchRolePermissions(db *sql.DB, roleID int) ([]string, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT DISTINCT permission_key
		FROM group_role
		WHERE role_id = ?
		ORDER BY permission_key
	`), roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rawPermissions []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			continue
		}
		rawPermissions = append(rawPermissions, perm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Check if role has all permissions (full access)
	hasAll := true
	for _, required := range allRawPermissions {
		found := false
		for _, p := range rawPermissions {
			if p == required {
				found = true
				break
			}
		}
		if !found {
			hasAll = false
			break
		}
	}

	// If all permissions, return wildcard
	if hasAll && len(rawPermissions) > 0 {
		return []string{"*"}, nil
	}

	// Map to frontend-friendly names using the new mapping function
	permissions := mapDatabaseToFrontendPermissions(rawPermissions)

	return permissions, nil
}

// handleAdminRoleDelete soft deletes a role (sets valid_id = 2).
func handleAdminRoleDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid role ID",
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

	// Soft delete by setting valid_id = 2
	result, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE roles
		SET valid_id = 2, change_time = CURRENT_TIMESTAMP, change_by = 1
		WHERE id = ?
	`), id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete role",
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
			"error":   "Role not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Role deleted successfully",
	})
}

// handleAdminRoleUsers displays and manages users assigned to a role.
// For scalability with thousands of users, this endpoint:
// - Returns current role members (usually a small list)
// - Does NOT return all available users (use search endpoint instead)
func handleAdminRoleUsers(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid role ID",
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

	// Get role details
	var role Role
	var comments sql.NullString
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id
		FROM roles
		WHERE id = ?
	`), id).Scan(&role.ID, &role.Name, &comments, &role.ValidID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Role not found",
		})
		return
	}

	if comments.Valid {
		role.Comments = &comments.String
	}

	// Permissions are stored in group_role table
	role.Permissions = []string{}

	// Get users assigned to this role
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT u.id, u.login, u.first_name, u.last_name
		FROM users u
		JOIN role_user ru ON u.id = ru.user_id
		WHERE ru.role_id = ?
		ORDER BY u.last_name, u.first_name
	`), id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch role users",
		})
		return
	}
	defer rows.Close()

	var users []RoleUser
	for rows.Next() {
		var u RoleUser
		err := rows.Scan(&u.UserID, &u.Login, &u.FirstName, &u.LastName)
		if err != nil {
			continue
		}
		u.Email = u.Login // Use login as email for display
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Error iterating role users",
		})
		return
	}

	// Check if it's an API request or page render
	if c.GetHeader("Accept") == "application/json" || c.Query("format") == "json" {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"role": gin.H{
				"id":   role.ID,
				"name": role.Name,
			},
			"members": users,
			// Note: 'available' users are NOT returned here for scalability
			// Use /admin/roles/:id/users/search?q=xxx to search for users to add
		})
		return
	}

	// Render template for browser request
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/role_users.pongo2", pongo2.Context{
		"Title":      "Role Users - " + role.Name,
		"Role":       role,
		"Users":      users,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

// handleAdminRoleUsersSearch searches for users that can be added to a role.
// Supports pagination and search for scalability with large user bases.
func handleAdminRoleUsersSearch(c *gin.Context) {
	idStr := c.Param("id")
	roleID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid role ID",
		})
		return
	}

	// Get search query
	query := c.Query("q")
	if len(query) < 2 {
		// Require at least 2 characters to search
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"users":   []RoleUser{},
			"message": "Enter at least 2 characters to search",
		})
		return
	}

	// Limit results for performance
	limit := 20

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Search for users not already in this role
	searchPattern := "%" + query + "%"
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, login, first_name, last_name
		FROM users
		WHERE id NOT IN (
			SELECT user_id FROM role_user WHERE role_id = ?
		)
		AND valid_id = 1
		AND (
			LOWER(login) LIKE LOWER(?) OR
			LOWER(first_name) LIKE LOWER(?) OR
			LOWER(last_name) LIKE LOWER(?) OR
			LOWER(CONCAT(first_name, ' ', last_name)) LIKE LOWER(?)
		)
		ORDER BY last_name, first_name
		LIMIT ?
	`), roleID, searchPattern, searchPattern, searchPattern, searchPattern, limit)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Search failed: " + err.Error(),
		})
		return
	}
	defer rows.Close()

	var users []RoleUser
	for rows.Next() {
		var u RoleUser
		if err := rows.Scan(&u.UserID, &u.Login, &u.FirstName, &u.LastName); err != nil {
			continue
		}
		u.Email = u.Login
		users = append(users, u)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"users":   users,
	})
}

// handleAdminRoleUserAdd adds a user to a role.
func handleAdminRoleUserAdd(c *gin.Context) {
	roleIDStr := c.Param("id")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid role ID",
		})
		return
	}

	var input struct {
		UserID int `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid input",
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

	// Add user to role (use INSERT IGNORE for MySQL compatibility)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT IGNORE INTO role_user (role_id, user_id, create_by, change_by)
		VALUES (?, ?, 1, 1)
	`), roleID, input.UserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to add user to role",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User added to role successfully",
	})
}

// handleAdminRoleUserRemove removes a user from a role.
func handleAdminRoleUserRemove(c *gin.Context) {
	roleIDStr := c.Param("id")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid role ID",
		})
		return
	}

	userIDStr := c.Param("userId")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
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

	// Remove user from role
	result, err := db.Exec(database.ConvertPlaceholders(`
		DELETE FROM role_user
		WHERE role_id = ? AND user_id = ?
	`), roleID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to remove user from role",
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
			"error":   "User not found in role",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User removed from role successfully",
	})
}

// handleAdminRolePermissions manages group permissions for a role.
func handleAdminRolePermissions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid role ID"})
		return
	}

	switch c.Request.Method {
	case "GET":
		handleAdminRolePermissionsGET(c, id)
	case "PUT":
		handleAdminRolePermissionsPUT(c, id)
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
	}
}

func handleAdminRolePermissionsGET(c *gin.Context, id int) {
	db, err := database.GetDB()
	if err != nil {
		c.String(http.StatusInternalServerError, "Database connection failed")
		return
	}

	role, err := loadRoleByID(db, id)
	if err == sql.ErrNoRows {
		c.String(http.StatusNotFound, "Role not found")
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load role")
		return
	}

	groups, err := loadRoleGroupPermissions(db, id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch permissions")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/role_permissions.pongo2", pongo2.Context{
		"Title":      "Role Permissions",
		"Role":       role,
		"Groups":     groups,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

func loadRoleByID(db *sql.DB, id int) (*Role, error) {
	var role Role
	var comments sql.NullString
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id FROM roles WHERE id = ?`), id).Scan(&role.ID, &role.Name, &comments, &role.ValidID)
	if err != nil {
		return nil, err
	}
	if comments.Valid {
		role.Comments = &comments.String
	}
	return &role, nil
}

func loadRoleGroupPermissions(db *sql.DB, roleID int) ([]RoleGroupPermission, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT
			g.id, g.name,
			MAX(CASE WHEN gr.permission_key = 'ro' THEN gr.permission_value ELSE 0 END) as ro,
			MAX(CASE WHEN gr.permission_key = 'move_into' THEN gr.permission_value ELSE 0 END) as move_into,
			MAX(CASE WHEN gr.permission_key = 'create' THEN gr.permission_value ELSE 0 END) as create_perm,
			MAX(CASE WHEN gr.permission_key = 'owner' THEN gr.permission_value ELSE 0 END) as owner,
			MAX(CASE WHEN gr.permission_key = 'priority' THEN gr.permission_value ELSE 0 END) as priority,
			MAX(CASE WHEN gr.permission_key = 'rw' THEN gr.permission_value ELSE 0 END) as rw,
			MAX(CASE WHEN gr.permission_key = 'note' THEN gr.permission_value ELSE 0 END) as note
		FROM groups g
		LEFT JOIN group_role gr ON g.id = gr.group_id AND gr.role_id = ?
		WHERE g.valid_id = 1
		GROUP BY g.id, g.name
		ORDER BY g.name`), roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]RoleGroupPermission, 0)
	for rows.Next() {
		var g RoleGroupPermission
		var ro, moveInto, create, owner, priority, rw, note int
		if err := rows.Scan(&g.GroupID, &g.GroupName, &ro, &moveInto, &create, &owner, &priority, &rw, &note); err != nil {
			continue
		}
		g.Permissions = map[string]bool{
			"ro": ro == 1, "move_into": moveInto == 1, "create": create == 1,
			"owner": owner == 1, "priority": priority == 1, "rw": rw == 1, "note": note == 1,
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func handleAdminRolePermissionsPUT(c *gin.Context, id int) {
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid form data"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to start transaction"})
		return
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.Exec(database.ConvertPlaceholders("DELETE FROM group_role WHERE role_id = ?"), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to clear permissions"})
		return
	}

	if err := insertRolePermissionsFromForm(tx, id, c.Request.PostForm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save permissions"})
		return
	}

	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to commit changes"})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/roles/%d/permissions", id))
}

func insertRolePermissionsFromForm(tx *sql.Tx, roleID int, form map[string][]string) error {
	for key, values := range form {
		if len(key) <= 5 || key[:5] != "perm_" {
			continue
		}
		var groupID int
		var permType string
		_, _ = fmt.Sscanf(key[5:], "%d_%s", &groupID, &permType) //nolint:errcheck // Parse errors handled by validation
		if groupID > 0 && permType != "" && len(values) > 0 && values[0] == "1" {
			_, err := tx.Exec(database.ConvertPlaceholders(`
				INSERT INTO group_role (role_id, group_id, permission_key, permission_value, create_by, change_by)
				VALUES (?, ?, ?, 1, 1, 1)`), roleID, groupID, permType)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// handleAdminRolePermissionsUpdate updates role-group permissions.
func handleAdminRolePermissionsUpdate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid role ID",
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

	// Parse form data
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid form data",
		})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction",
		})
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing permissions for this role
	_, err = tx.Exec(database.ConvertPlaceholders("DELETE FROM group_role WHERE role_id = ?"), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to clear permissions",
		})
		return
	}

	// Insert new permissions
	for key, values := range c.Request.PostForm {
		if len(key) > 5 && key[:5] == "perm_" {
			// Parse permission key: perm_groupID_permissionType
			var groupID int
			var permType string
			_, _ = fmt.Sscanf(key[5:], "%d_%s", &groupID, &permType) //nolint:errcheck // Parse errors handled by validation

			if groupID > 0 && permType != "" && len(values) > 0 && values[0] == "1" {
				_, err = tx.Exec(database.ConvertPlaceholders(`
				INSERT INTO group_role (
					role_id, group_id, permission_key, permission_value,
					create_time, create_by, change_time, change_by)
				VALUES (?, ?, ?, 1, NOW(), 1, NOW(), 1)
				`), id, groupID, permType)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"error":   "Failed to save permissions: " + err.Error(),
					})
					return
				}
			}
		}
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit changes",
		})
		return
	}

	// Return success for HTMX or redirect for standard form
	if c.GetHeader("HX-Request") != "" {
		c.Header("HX-Trigger", "rolePermissionsUpdated")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Permissions updated successfully",
		})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/roles/%d/permissions", id))
}
