package api

// Admin customer group handlers - manages customer company to group permissions.
// Matches OTRS AdminCustomerGroup functionality.

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
	routing.RegisterHandler("handleAdminCustomerGroups", handleAdminCustomerGroups)
	routing.RegisterHandler("handleAdminCustomerGroupEdit", handleAdminCustomerGroupEdit)
	routing.RegisterHandler("handleAdminCustomerGroupUpdate", handleAdminCustomerGroupUpdate)
	routing.RegisterHandler("handleAdminCustomerGroupByGroup", handleAdminCustomerGroupByGroup)
	routing.RegisterHandler("handleAdminCustomerGroupByGroupUpdate", handleAdminCustomerGroupByGroupUpdate)
	routing.RegisterHandler("handleGetCustomerGroupPermissions", handleGetCustomerGroupPermissions)
}

// CustomerGroupPermission represents a permission assignment
type CustomerGroupPermission struct {
	CustomerID        string
	GroupID           int
	PermissionKey     string
	PermissionValue   int
	PermissionContext string
}

// CustomerCompanyInfo holds customer company data for display
type CustomerCompanyInfo struct {
	CustomerID string
	Name       string
	ValidID    int
	ValidName  string
}

// GroupInfo holds group data for display
type GroupInfo struct {
	ID        int
	Name      string
	ValidID   int
	ValidName string
}

// GroupWithPermissions holds group data with permission flags for templates
type GroupWithPermissions struct {
	ID          int
	Name        string
	ValidID     int
	ValidName   string
	Permissions map[string]bool // permission_key -> enabled
}

// CustomerWithPermissions holds customer data with permission flags for templates
type CustomerWithPermissions struct {
	CustomerID  string
	Name        string
	ValidID     int
	ValidName   string
	Permissions map[string]bool // permission_key -> enabled
}

// CustomerGroupAssignment holds the permission matrix for a customer
type CustomerGroupAssignment struct {
	Customer    CustomerCompanyInfo
	Permissions map[int]map[string]bool // group_id -> permission_key -> enabled
}

// GroupCustomerAssignment holds the permission matrix for a group
type GroupCustomerAssignment struct {
	Group       GroupInfo
	Permissions map[string]map[string]bool // customer_id -> permission_key -> enabled
}

// Permission types supported (matching OTRS)
var customerGroupPermissionTypes = []string{"ro", "rw"}

// handleAdminCustomerGroups shows the customer groups overview page
func handleAdminCustomerGroups(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Load customer companies
	customers, err := loadCustomerCompanies(db, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load customer companies"})
		return
	}

	// Load groups
	groups, err := loadGroups(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load groups"})
		return
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"customers": customers,
			"groups":    groups,
		})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_groups.pongo2", pongo2.Context{
		"Title":           "Customer Groups",
		"ActivePage":      "admin",
		"ActiveAdminPage": "customer-groups",
		"User":            getUserMapForTemplate(c),
		"Customers":       customers,
		"Groups":          groups,
		"Search":          search,
		"PermissionTypes": customerGroupPermissionTypes,
	})
}

// handleAdminCustomerGroupEdit shows permissions for a specific customer company
func handleAdminCustomerGroupEdit(c *gin.Context) {
	customerID := c.Param("id")
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Customer ID required"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Load customer info
	customer, err := loadCustomerCompany(db, customerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}

	// Load all valid groups
	groups, err := loadGroups(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load groups"})
		return
	}

	// Load current permissions for this customer
	permissions, err := loadCustomerGroupPermissions(db, customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load permissions"})
		return
	}

	// Build groups with embedded permissions for template
	groupsWithPerms := make([]GroupWithPermissions, 0, len(groups))
	for _, g := range groups {
		gwp := GroupWithPermissions{
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
			"success":     true,
			"customer":    customer,
			"groups":      groupsWithPerms,
			"permissions": permissions,
		})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_group_edit.pongo2", pongo2.Context{
		"Title":           fmt.Sprintf("Customer Group Permissions: %s", customer.Name),
		"ActivePage":      "admin",
		"ActiveAdminPage": "customer-groups",
		"User":            getUserMapForTemplate(c),
		"Customer":        customer,
		"Groups":          groupsWithPerms,
		"PermissionTypes": customerGroupPermissionTypes,
	})
}

// handleAdminCustomerGroupUpdate updates permissions for a customer company
func handleAdminCustomerGroupUpdate(c *gin.Context) {
	customerID := c.Param("id")
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Customer ID required"})
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
	if err := updateCustomerGroupPermissions(db, customerID, newPermissions, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update permissions"})
		return
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Permissions updated successfully"})
		return
	}

	c.Redirect(http.StatusFound, "/admin/customer-groups")
}

// handleAdminCustomerGroupByGroup shows customers for a specific group
func handleAdminCustomerGroupByGroup(c *gin.Context) {
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

	// Load customer companies
	customers, err := loadCustomerCompanies(db, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load customers"})
		return
	}

	// Load current permissions for this group
	permissions, err := loadGroupCustomerPermissions(db, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load permissions"})
		return
	}

	// Build customers with embedded permissions for template
	customersWithPerms := make([]CustomerWithPermissions, 0, len(customers))
	for _, cust := range customers {
		cwp := CustomerWithPermissions{
			CustomerID:  cust.CustomerID,
			Name:        cust.Name,
			ValidID:     cust.ValidID,
			ValidName:   cust.ValidName,
			Permissions: make(map[string]bool),
		}
		if perms, ok := permissions[cust.CustomerID]; ok {
			cwp.Permissions = perms
		}
		customersWithPerms = append(customersWithPerms, cwp)
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"group":       group,
			"customers":   customersWithPerms,
			"permissions": permissions,
		})
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/customer_group_by_group.pongo2", pongo2.Context{
		"Title":           fmt.Sprintf("Group Customer Permissions: %s", group.Name),
		"ActivePage":      "admin",
		"ActiveAdminPage": "customer-groups",
		"User":            getUserMapForTemplate(c),
		"Group":           group,
		"Customers":       customersWithPerms,
		"PermissionTypes": customerGroupPermissionTypes,
		"Search":          search,
	})
}

// handleAdminCustomerGroupByGroupUpdate updates permissions for a group's customers
func handleAdminCustomerGroupByGroupUpdate(c *gin.Context) {
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

	// Build permissions map from form - format: permissions[customer_id][permission_type]
	newPermissions := make(map[string]map[string]bool)
	for key, values := range c.Request.PostForm {
		if strings.HasPrefix(key, "permissions[") {
			parts := strings.Split(strings.TrimPrefix(key, "permissions["), "][")
			if len(parts) == 2 {
				custID := strings.TrimSuffix(parts[0], "]")
				permType := strings.TrimSuffix(parts[1], "]")
				if newPermissions[custID] == nil {
					newPermissions[custID] = make(map[string]bool)
				}
				newPermissions[custID][permType] = len(values) > 0 && values[0] == "1"
			}
		}
	}

	// Update permissions in database
	if err := updateGroupCustomerPermissions(db, groupID, newPermissions, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update permissions"})
		return
	}

	if wantsJSONResponse(c) {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Permissions updated successfully"})
		return
	}

	c.Redirect(http.StatusFound, "/admin/customer-groups")
}

// handleGetCustomerGroupPermissions returns permissions as JSON for API use
func handleGetCustomerGroupPermissions(c *gin.Context) {
	customerID := c.Query("customer_id")
	groupIDStr := c.Query("group_id")

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	if customerID != "" {
		permissions, err := loadCustomerGroupPermissions(db, customerID)
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
		permissions, err := loadGroupCustomerPermissions(db, groupID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load permissions"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "permissions": permissions})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "Either customer_id or group_id required"})
}

// Database helper functions

func loadCustomerCompanies(db *sql.DB, search string) ([]CustomerCompanyInfo, error) {
	query := `
		SELECT cc.customer_id, cc.name, cc.valid_id, COALESCE(v.name, 'valid') as valid_name
		FROM customer_company cc
		LEFT JOIN valid v ON cc.valid_id = v.id
		WHERE cc.valid_id = 1
	`
	args := []interface{}{}

	if search != "" && search != "*" {
		query += " AND (LOWER(cc.customer_id) LIKE LOWER(?) OR LOWER(cc.name) LIKE LOWER(?))"
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	query += " ORDER BY cc.name"
	query = database.ConvertPlaceholders(query)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []CustomerCompanyInfo
	for rows.Next() {
		var c CustomerCompanyInfo
		if err := rows.Scan(&c.CustomerID, &c.Name, &c.ValidID, &c.ValidName); err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	return customers, rows.Err()
}

func loadCustomerCompany(db *sql.DB, customerID string) (*CustomerCompanyInfo, error) {
	query := `
		SELECT cc.customer_id, cc.name, cc.valid_id, COALESCE(v.name, 'valid') as valid_name
		FROM customer_company cc
		LEFT JOIN valid v ON cc.valid_id = v.id
		WHERE cc.customer_id = ?
	`
	query = database.ConvertPlaceholders(query)

	var c CustomerCompanyInfo
	err := db.QueryRow(query, customerID).Scan(&c.CustomerID, &c.Name, &c.ValidID, &c.ValidName)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func loadGroups(db *sql.DB) ([]GroupInfo, error) {
	query := `
		SELECT g.id, g.name, g.valid_id, COALESCE(v.name, 'valid') as valid_name
		FROM groups g
		LEFT JOIN valid v ON g.valid_id = v.id
		WHERE g.valid_id = 1
		ORDER BY g.name
	`
	query = database.ConvertPlaceholders(query)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []GroupInfo
	for rows.Next() {
		var g GroupInfo
		if err := rows.Scan(&g.ID, &g.Name, &g.ValidID, &g.ValidName); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func loadGroup(db *sql.DB, groupID int) (*GroupInfo, error) {
	query := `
		SELECT g.id, g.name, g.valid_id, COALESCE(v.name, 'valid') as valid_name
		FROM groups g
		LEFT JOIN valid v ON g.valid_id = v.id
		WHERE g.id = ?
	`
	query = database.ConvertPlaceholders(query)

	var g GroupInfo
	err := db.QueryRow(query, groupID).Scan(&g.ID, &g.Name, &g.ValidID, &g.ValidName)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func loadCustomerGroupPermissions(db *sql.DB, customerID string) (map[int]map[string]bool, error) {
	query := `
		SELECT group_id, permission_key, permission_value
		FROM group_customer
		WHERE customer_id = ?
	`
	query = database.ConvertPlaceholders(query)

	rows, err := db.Query(query, customerID)
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

func loadGroupCustomerPermissions(db *sql.DB, groupID int) (map[string]map[string]bool, error) {
	query := `
		SELECT customer_id, permission_key, permission_value
		FROM group_customer
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
		var customerID string
		var permKey string
		var permValue int
		if err := rows.Scan(&customerID, &permKey, &permValue); err != nil {
			return nil, err
		}
		if permissions[customerID] == nil {
			permissions[customerID] = make(map[string]bool)
		}
		permissions[customerID][permKey] = permValue > 0
	}
	return permissions, rows.Err()
}

func updateCustomerGroupPermissions(db *sql.DB, customerID string, permissions map[int]map[string]bool, userID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing permissions for this customer
	deleteQuery := database.ConvertPlaceholders("DELETE FROM group_customer WHERE customer_id = ?")
	if _, err := tx.Exec(deleteQuery, customerID); err != nil {
		return err
	}

	// Insert new permissions
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	now := time.Now()
	for groupID, perms := range permissions {
		for permKey, enabled := range perms {
			if enabled {
				permValue := 1
				if _, err := tx.Exec(insertQuery, customerID, groupID, permKey, permValue, "customer", now, userID, now, userID); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

func updateGroupCustomerPermissions(db *sql.DB, groupID int, permissions map[string]map[string]bool, userID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing permissions for this group
	deleteQuery := database.ConvertPlaceholders("DELETE FROM group_customer WHERE group_id = ?")
	if _, err := tx.Exec(deleteQuery, groupID); err != nil {
		return err
	}

	// Insert new permissions
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)

	now := time.Now()
	for customerID, perms := range permissions {
		for permKey, enabled := range perms {
			if enabled {
				permValue := 1
				if _, err := tx.Exec(insertQuery, customerID, groupID, permKey, permValue, "customer", now, userID, now, userID); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}
