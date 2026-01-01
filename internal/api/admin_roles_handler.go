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
	argIdx := 0

	if searchQuery != "" {
		query += " AND (LOWER(r.name) LIKE ? OR LOWER(r.comments) LIKE ?)"
		searchPattern := "%" + searchQuery + "%"
		args = append(args, searchPattern, searchPattern)
		argIdx += 2
	}

	if validFilter != "all" {
		if validFilter == "valid" {
			query += " AND r.valid_id = ?"
			args = append(args, 1)
		} else if validFilter == "invalid" {
			query += " AND r.valid_id = ?"
			args = append(args, 2)
		}
		argIdx++
	}

	query += " GROUP BY r.id, r.name, r.comments, r.valid_id, r.create_time, r.create_by, r.change_time, r.change_by"
	query += " ORDER BY r.name ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch roles: "+err.Error())
		return
	}
	defer func() { _ = rows.Close() }()

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
		}

		r.Permissions = []string{}

		// Set computed fields
		r.IsActive = r.ValidID == 1
		r.IsSystem = r.ID <= 3 // First 3 roles are system roles

		roles = append(roles, r)
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
		Name     string `json:"name" binding:"required"`
		Comments string `json:"comments"`
		ValidID  int    `json:"valid_id"`
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
	err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)"), input.Name).Scan(&exists)
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

	// Insert the new role using database adapter for MySQL/PostgreSQL compatibility
	var commentsPtr *string
	if input.Comments != "" {
		commentsPtr = &input.Comments
	}

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO roles (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Role created successfully",
		"data": gin.H{
			"id":   int(id64),
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
		WHERE id = $1
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

	// Permissions are stored in group_role table, not in roles table
	role.Permissions = []string{}

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

	// Prepare values for update - convert empty strings to nil for proper NULL handling
	var commentsVal interface{} = input.Comments
	if input.Comments == "" {
		commentsVal = nil
	}

	var validIDVal = input.ValidID
	if validIDVal == 0 {
		validIDVal = 1 // Default to valid if not specified
	}

	// Update the role (permissions are managed via group_role table, not here)
	// Note: For MySQL compatibility, each placeholder can only be used once
	result, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE roles 
		SET name = COALESCE(NULLIF($1, ''), name),
		    comments = $2,
		    valid_id = $3,
		    change_time = CURRENT_TIMESTAMP,
		    change_by = 1
		WHERE id = $4
	`), input.Name, commentsVal, validIDVal, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update role: " + err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Role not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Role updated successfully",
	})
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
		WHERE id = $1
	`), id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete role",
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
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
		WHERE id = $1
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
		WHERE ru.role_id = $1
		ORDER BY u.last_name, u.first_name
	`), id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch role users",
		})
		return
	}
	defer func() { _ = rows.Close() }()

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

	// Get all available users not in this role
	availableRows, err := db.Query(database.ConvertPlaceholders(`
		SELECT id, login, first_name, last_name
		FROM users
		WHERE id NOT IN (
			SELECT user_id FROM role_user WHERE role_id = $1
		)
		AND valid_id = 1
		ORDER BY last_name, first_name
	`), id)

	var availableUsers []RoleUser
	if err == nil {
		defer availableRows.Close()
		for availableRows.Next() {
			var u RoleUser
			err := availableRows.Scan(&u.UserID, &u.Login, &u.FirstName, &u.LastName)
			if err != nil {
				continue
			}
			u.Email = u.Login
			availableUsers = append(availableUsers, u)
		}
	}

	// Check if it's an API request or page render
	if c.GetHeader("Accept") == "application/json" || c.Query("format") == "json" {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"role": gin.H{
				"name": role.Name,
			},
			"members":   users,
			"available": availableUsers,
		})
		return
	}

	// Render template for browser request
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/role_users.pongo2", pongo2.Context{
		"Title":          "Role Users - " + role.Name,
		"Role":           role,
		"Users":          users,
		"AvailableUsers": availableUsers,
		"User":           getUserMapForTemplate(c),
		"ActivePage":     "admin",
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
		VALUES ($1, $2, 1, 1)
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
		WHERE role_id = $1 AND user_id = $2
	`), roleID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to remove user from role",
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
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
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid role ID",
		})
		return
	}

	if c.Request.Method == "GET" {
		// Display permissions page
		db, err := database.GetDB()
		if err != nil {
			c.String(http.StatusInternalServerError, "Database connection failed")
			return
		}

		// Get role details
		var role Role
		var comments sql.NullString
		err = db.QueryRow(database.ConvertPlaceholders(`
			SELECT id, name, comments, valid_id 
			FROM roles 
			WHERE id = $1
		`), id).Scan(&role.ID, &role.Name, &comments, &role.ValidID)

		if err == sql.ErrNoRows {
			c.String(http.StatusNotFound, "Role not found")
			return
		}

		if comments.Valid {
			role.Comments = &comments.String
		}

		// Get all groups and their permissions for this role
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
			LEFT JOIN group_role gr ON g.id = gr.group_id AND gr.role_id = $1
			WHERE g.valid_id = 1
			GROUP BY g.id, g.name
			ORDER BY g.name
		`), id)

		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to fetch permissions")
			return
		}
		defer func() { _ = rows.Close() }()

		var groups []RoleGroupPermission
		for rows.Next() {
			var g RoleGroupPermission
			var ro, moveInto, create, owner, priority, rw, note int

			err := rows.Scan(&g.GroupID, &g.GroupName, &ro, &moveInto, &create, &owner, &priority, &rw, &note)
			if err != nil {
				continue
			}

			g.Permissions = map[string]bool{
				"ro":        ro == 1,
				"move_into": moveInto == 1,
				"create":    create == 1,
				"owner":     owner == 1,
				"priority":  priority == 1,
				"rw":        rw == 1,
				"note":      note == 1,
			}

			groups = append(groups, g)
		}

		// Render permissions template
		getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/role_permissions.pongo2", pongo2.Context{
			"Title":      "Role Permissions",
			"Role":       role,
			"Groups":     groups,
			"User":       getUserMapForTemplate(c),
			"ActivePage": "admin",
		})
	} else if c.Request.Method == "PUT" {
		// Update permissions
		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Database connection failed",
			})
			return
		}

		// Parse form data
		c.Request.ParseForm()

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
		_, err = tx.Exec(database.ConvertPlaceholders("DELETE FROM group_role WHERE role_id = $1"), id)
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
				fmt.Sscanf(key[5:], "%d_%s", &groupID, &permType)

				if groupID > 0 && permType != "" && len(values) > 0 && values[0] == "1" {
					_, err = tx.Exec(database.ConvertPlaceholders(`
						INSERT INTO group_role (role_id, group_id, permission_key, permission_value, create_by, change_by)
						VALUES ($1, $2, $3, 1, 1, 1)
					`), id, groupID, permType)

					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{
							"success": false,
							"error":   "Failed to save permissions",
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

		// Redirect back to permissions page
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/roles/%d/permissions", id))
	}
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
	c.Request.ParseForm()

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
	_, err = tx.Exec(database.ConvertPlaceholders("DELETE FROM group_role WHERE role_id = $1"), id)
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
			fmt.Sscanf(key[5:], "%d_%s", &groupID, &permType)

			if groupID > 0 && permType != "" && len(values) > 0 && values[0] == "1" {
				_, err = tx.Exec(database.ConvertPlaceholders(`
					INSERT INTO group_role (role_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
					VALUES ($1, $2, $3, 1, NOW(), 1, NOW(), 1)
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
