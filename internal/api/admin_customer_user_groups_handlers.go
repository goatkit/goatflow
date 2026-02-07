package api

// Admin customer user group handlers - manages individual customer user to group permissions.
// Matches OTRS AdminCustomerUserGroup functionality.
// Different from AdminCustomerGroups which manages customer COMPANY permissions.

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/routing"
)

func init() {
	routing.RegisterHandler("handleAdminCustomerUserGroups", handleAdminCustomerUserGroups)
	routing.RegisterHandler("handleAdminCustomerUserGroupEdit", handleAdminCustomerUserGroupEdit)
	routing.RegisterHandler("handleAdminCustomerUserGroupUpdate", handleAdminCustomerUserGroupUpdate)
	routing.RegisterHandler("handleAdminCustomerUserGroupByGroup", handleAdminCustomerUserGroupByGroup)
	routing.RegisterHandler("handleAdminCustomerUserGroupByGroupUpdate", handleAdminCustomerUserGroupByGroupUpdate)
	routing.RegisterHandler("handleGetCustomerUserGroupPermissions", handleGetCustomerUserGroupPermissions)
}

// CustomerUserInfo holds customer user data for display
type CustomerUserInfo struct {
	Login     string
	FirstName string
	LastName  string
	Email     string
	ValidID   int
	ValidName string
}

// GroupWithCustomerUserPermissions holds group data with permission flags for customer user templates
type GroupWithCustomerUserPermissions struct {
	ID          int
	Name        string
	ValidID     int
	ValidName   string
	Permissions map[string]bool // permission_key -> enabled
}

// CustomerUserWithPermissions holds customer user data with permission flags for templates
type CustomerUserWithPermissions struct {
	Login       string
	FirstName   string
	LastName    string
	Email       string
	ValidID     int
	ValidName   string
	Permissions map[string]bool // permission_key -> enabled
}

// Permission types supported for customer users (matching OTRS)
var customerUserGroupPermissionTypes = []string{"ro", "rw"}

// handleAdminCustomerUserGroups shows the customer user groups overview page
func handleAdminCustomerUserGroups(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Load customer users
	customerUsers, err := loadCustomerUsers(db, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load customer users: " + err.Error()})
		return
	}

	// Load groups (reuse from customer groups)
	groups, err := loadGroups(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load groups"})
		return
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"customerUsers": customerUsers,
			"groups":        groups,
		})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_user_groups.pongo2", pongo2.Context{
		"Title":           "Customer User Groups",
		"ActivePage":      "admin",
		"ActiveAdminPage": "customer-user-groups",
		"User":            getUserMapForTemplate(c),
		"CustomerUsers":   customerUsers,
		"Groups":          groups,
		"Search":          search,
		"PermissionTypes": customerUserGroupPermissionTypes,
	})
}

// handleAdminCustomerUserGroupEdit shows permissions for a specific customer user
func handleAdminCustomerUserGroupEdit(c *gin.Context) {
	userLogin := c.Param("id")
	if userLogin == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Customer user login required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Load customer user info
	customerUser, err := loadCustomerUser(db, userLogin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer user not found"})
		return
	}

	// Load all valid groups
	groups, err := loadGroups(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load groups"})
		return
	}

	// Load current permissions for this customer user
	permissions, err := loadCustomerUserGroupPermissions(db, userLogin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load permissions"})
		return
	}

	// Build groups with embedded permissions for template
	groupsWithPerms := make([]GroupWithCustomerUserPermissions, 0, len(groups))
	for _, g := range groups {
		gwp := GroupWithCustomerUserPermissions{
			ID:          g.ID,
			Name:        g.Name,
			ValidID:     g.ValidID,
			ValidName:   g.ValidName,
			Permissions: make(map[string]bool),
		}
		if perms, ok := permissions[g.ID]; ok {
			gwp.Permissions = perms
		}
		groupsWithPerms = append(groupsWithPerms, gwp)
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"customerUser": customerUser,
			"groups":       groupsWithPerms,
			"permissions":  permissions,
		})
		return
	}

	displayName := customerUser.FirstName + " " + customerUser.LastName
	if displayName == " " {
		displayName = customerUser.Login
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_user_group_edit.pongo2", pongo2.Context{
		"Title":           fmt.Sprintf("Customer User Group Permissions: %s", displayName),
		"ActivePage":      "admin",
		"ActiveAdminPage": "customer-user-groups",
		"User":            getUserMapForTemplate(c),
		"CustomerUser":    customerUser,
		"Groups":          groupsWithPerms,
		"PermissionTypes": customerUserGroupPermissionTypes,
	})
}

// handleAdminCustomerUserGroupUpdate updates permissions for a customer user
func handleAdminCustomerUserGroupUpdate(c *gin.Context) {
	userLogin := c.Param("id")
	if userLogin == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Customer user login required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get user ID from context
	userID := 1 // Default to admin
	if u, exists := c.Get("userID"); exists {
		if uid, ok := u.(int); ok {
			userID = uid
		}
	}

	// Parse form data - format: permissions[group_id][permission_type] = "1" or absent
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	// Build permissions map from form
	newPermissions := make(map[int]map[string]bool)
	for key, values := range c.Request.PostForm {
		if strings.HasPrefix(key, "permissions[") {
			// Parse permissions[123][ro] format
			parts := strings.Split(strings.TrimPrefix(key, "permissions["), "][")
			if len(parts) == 2 {
				groupIDStr := strings.TrimSuffix(parts[0], "]")
				permType := strings.TrimSuffix(parts[1], "]")
				groupID, err := strconv.Atoi(groupIDStr)
				if err != nil {
					continue
				}
				if newPermissions[groupID] == nil {
					newPermissions[groupID] = make(map[string]bool)
				}
				newPermissions[groupID][permType] = len(values) > 0 && values[0] == "1"
			}
		}
	}

	// Update permissions in database
	if err := updateCustomerUserGroupPermissions(db, userLogin, newPermissions, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update permissions: " + err.Error()})
		return
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Permissions updated successfully"})
		return
	}

	c.Redirect(http.StatusFound, "/admin/customer-user-groups")
}

// handleAdminCustomerUserGroupByGroup shows customer users for a specific group
func handleAdminCustomerUserGroupByGroup(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	search := strings.TrimSpace(c.Query("search"))

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Load group info
	group, err := loadGroup(db, groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	// Load customer users
	customerUsers, err := loadCustomerUsers(db, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load customer users"})
		return
	}

	// Load current permissions for this group
	permissions, err := loadGroupCustomerUserPermissions(db, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load permissions"})
		return
	}

	// Build customer users with embedded permissions for template
	usersWithPerms := make([]CustomerUserWithPermissions, 0, len(customerUsers))
	for _, cu := range customerUsers {
		uwp := CustomerUserWithPermissions{
			Login:       cu.Login,
			FirstName:   cu.FirstName,
			LastName:    cu.LastName,
			Email:       cu.Email,
			ValidID:     cu.ValidID,
			ValidName:   cu.ValidName,
			Permissions: make(map[string]bool),
		}
		if perms, ok := permissions[cu.Login]; ok {
			uwp.Permissions = perms
		}
		usersWithPerms = append(usersWithPerms, uwp)
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"group":         group,
			"customerUsers": usersWithPerms,
			"permissions":   permissions,
		})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_user_group_by_group.pongo2", pongo2.Context{
		"Title":           fmt.Sprintf("Group Customer User Permissions: %s", group.Name),
		"ActivePage":      "admin",
		"ActiveAdminPage": "customer-user-groups",
		"User":            getUserMapForTemplate(c),
		"Group":           group,
		"CustomerUsers":   usersWithPerms,
		"PermissionTypes": customerUserGroupPermissionTypes,
		"Search":          search,
	})
}

// handleAdminCustomerUserGroupByGroupUpdate updates permissions for a group's customer users
func handleAdminCustomerUserGroupByGroupUpdate(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get user ID from context
	userID := 1
	if u, exists := c.Get("userID"); exists {
		if uid, ok := u.(int); ok {
			userID = uid
		}
	}

	// Parse form data
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	// Build permissions map from form - format: permissions[user_login][permission_type]
	newPermissions := make(map[string]map[string]bool)
	for key, values := range c.Request.PostForm {
		if strings.HasPrefix(key, "permissions[") {
			parts := strings.Split(strings.TrimPrefix(key, "permissions["), "][")
			if len(parts) == 2 {
				userLogin := strings.TrimSuffix(parts[0], "]")
				permType := strings.TrimSuffix(parts[1], "]")
				if newPermissions[userLogin] == nil {
					newPermissions[userLogin] = make(map[string]bool)
				}
				newPermissions[userLogin][permType] = len(values) > 0 && values[0] == "1"
			}
		}
	}

	// Update permissions in database
	if err := updateGroupCustomerUserPermissions(db, groupID, newPermissions, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update permissions: " + err.Error()})
		return
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Permissions updated successfully"})
		return
	}

	c.Redirect(http.StatusFound, "/admin/customer-user-groups")
}

// handleGetCustomerUserGroupPermissions returns permissions as JSON for API use
func handleGetCustomerUserGroupPermissions(c *gin.Context) {
	userLogin := c.Query("user_login")
	groupIDStr := c.Query("group_id")

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	if userLogin != "" {
		permissions, err := loadCustomerUserGroupPermissions(db, userLogin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load permissions"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "permissions": permissions})
		return
	}

	if groupIDStr != "" {
		groupID, err := strconv.Atoi(groupIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
			return
		}
		permissions, err := loadGroupCustomerUserPermissions(db, groupID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load permissions"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "permissions": permissions})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "Either user_login or group_id required"})
}

// Database helper functions for customer users

func loadCustomerUsers(db *sql.DB, search string) ([]CustomerUserInfo, error) {
	query := `
		SELECT cu.login, COALESCE(cu.first_name, ''), COALESCE(cu.last_name, ''),
		       COALESCE(cu.email, ''), cu.valid_id, COALESCE(v.name, 'valid') as valid_name
		FROM customer_user cu
		LEFT JOIN valid v ON cu.valid_id = v.id
		WHERE cu.valid_id = 1
	`
	args := []interface{}{}

	if search != "" && search != "*" {
		query += ` AND (LOWER(cu.login) LIKE LOWER(?)
		           OR LOWER(cu.first_name) LIKE LOWER(?)
		           OR LOWER(cu.last_name) LIKE LOWER(?)
		           OR LOWER(cu.email) LIKE LOWER(?))`
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	query += " ORDER BY cu.last_name, cu.first_name, cu.login"
	query = database.ConvertPlaceholders(query)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []CustomerUserInfo
	for rows.Next() {
		var u CustomerUserInfo
		if err := rows.Scan(&u.Login, &u.FirstName, &u.LastName, &u.Email, &u.ValidID, &u.ValidName); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func loadCustomerUser(db *sql.DB, login string) (*CustomerUserInfo, error) {
	query := `
		SELECT cu.login, COALESCE(cu.first_name, ''), COALESCE(cu.last_name, ''),
		       COALESCE(cu.email, ''), cu.valid_id, COALESCE(v.name, 'valid') as valid_name
		FROM customer_user cu
		LEFT JOIN valid v ON cu.valid_id = v.id
		WHERE cu.login = ?
	`
	query = database.ConvertPlaceholders(query)

	var u CustomerUserInfo
	err := db.QueryRow(query, login).Scan(&u.Login, &u.FirstName, &u.LastName, &u.Email, &u.ValidID, &u.ValidName)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func loadCustomerUserGroupPermissions(db *sql.DB, userLogin string) (map[int]map[string]bool, error) {
	query := `
		SELECT group_id, permission_key, permission_value
		FROM group_customer_user
		WHERE user_id = ?
	`
	query = database.ConvertPlaceholders(query)

	rows, err := db.Query(query, userLogin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make(map[int]map[string]bool)
	for rows.Next() {
		var groupID int
		var permKey string
		var permValue int
		if err := rows.Scan(&groupID, &permKey, &permValue); err != nil {
			return nil, err
		}
		if permissions[groupID] == nil {
			permissions[groupID] = make(map[string]bool)
		}
		permissions[groupID][permKey] = permValue > 0
	}
	return permissions, rows.Err()
}

func loadGroupCustomerUserPermissions(db *sql.DB, groupID int) (map[string]map[string]bool, error) {
	query := `
		SELECT user_id, permission_key, permission_value
		FROM group_customer_user
		WHERE group_id = ?
	`
	query = database.ConvertPlaceholders(query)

	rows, err := db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make(map[string]map[string]bool)
	for rows.Next() {
		var userLogin string
		var permKey string
		var permValue int
		if err := rows.Scan(&userLogin, &permKey, &permValue); err != nil {
			return nil, err
		}
		if permissions[userLogin] == nil {
			permissions[userLogin] = make(map[string]bool)
		}
		permissions[userLogin][permKey] = permValue > 0
	}
	return permissions, rows.Err()
}

func updateCustomerUserGroupPermissions(db *sql.DB, userLogin string, permissions map[int]map[string]bool, userID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing permissions for this customer user
	deleteQuery := database.ConvertPlaceholders("DELETE FROM group_customer_user WHERE user_id = ?")
	if _, err := tx.Exec(deleteQuery, userLogin); err != nil {
		return err
	}

	// Insert new permissions
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO group_customer_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)

	now := time.Now()
	for groupID, perms := range permissions {
		for permKey, enabled := range perms {
			if enabled {
				permValue := 1
				if _, err := tx.Exec(insertQuery, userLogin, groupID, permKey, permValue, now, userID, now, userID); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

func updateGroupCustomerUserPermissions(db *sql.DB, groupID int, permissions map[string]map[string]bool, userID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing permissions for this group
	deleteQuery := database.ConvertPlaceholders("DELETE FROM group_customer_user WHERE group_id = ?")
	if _, err := tx.Exec(deleteQuery, groupID); err != nil {
		return err
	}

	// Insert new permissions
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO group_customer_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)

	now := time.Now()
	for userLogin, perms := range permissions {
		for permKey, enabled := range perms {
			if enabled {
				permValue := 1
				if _, err := tx.Exec(insertQuery, userLogin, groupID, permKey, permValue, now, userID, now, userID); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}
