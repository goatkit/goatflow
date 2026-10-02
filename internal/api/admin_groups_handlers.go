package api

// Admin groups, permissions, and user management handlers.
// Split from admin_htmx_handlers.go for maintainability.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

func init() {
	routing.RegisterHandler("handleAdminGroups", handleAdminGroups)
	routing.RegisterHandler("handleCreateGroup", handleCreateGroup)
	routing.RegisterHandler("handleGetGroup", handleGetGroup)
	routing.RegisterHandler("handleUpdateGroup", handleUpdateGroup)
	routing.RegisterHandler("handleDeleteGroup", handleDeleteGroup)
	routing.RegisterHandler("handleAdminPermissions", handleAdminPermissions)
	routing.RegisterHandler("handleGetUserPermissionMatrix", handleGetUserPermissionMatrix)
	routing.RegisterHandler("handleUpdateUserPermissions", handleUpdateUserPermissions)
	routing.RegisterHandler("handleClonePermissions", handleClonePermissions)
	routing.RegisterHandler("handleAddUserToGroup", handleAddUserToGroup)
	routing.RegisterHandler("handleRemoveUserFromGroup", handleRemoveUserFromGroup)
	routing.RegisterHandler("handleGroupPermissions", handleGroupPermissions)
	routing.RegisterHandler("handleSaveGroupPermissions", handleSaveGroupPermissions)
	routing.RegisterHandler("handleGetGroups", handleGetGroups)
	routing.RegisterHandler("handleGetGroupMembers", handleGetGroupMembers)
	routing.RegisterHandler("handleGetGroupAPI", handleGetGroupAPI)
}

type groupPermissionAssignment struct {
	UserID      uint            `json:"user_id"`
	Permissions map[string]bool `json:"permissions"`
}

type saveGroupPermissionsRequest struct {
	Assignments []groupPermissionAssignment `json:"assignments"`
}

// handleAdminGroups shows the admin groups page.
func handleAdminGroups(c *gin.Context) {
	saveState := strings.EqualFold(strings.TrimSpace(c.Query("save_state")), "true") || strings.TrimSpace(c.Query("save_state")) == "1"
	searchTerm := strings.TrimSpace(c.Query("search"))
	statusTerm := strings.TrimSpace(c.Query("status"))

	if saveState {
		state := map[string]string{
			"search": searchTerm,
			"status": statusTerm,
		}
		if payload, err := json.Marshal(state); err == nil {
			encoded := url.QueryEscape(string(payload))
			c.SetCookie("group_filters", encoded, 86400, "/admin/groups", "", false, true)
		}
	}

	// Restore filters from cookie if no query params provided
	if searchTerm == "" && statusTerm == "" {
		if cookie, err := c.Cookie("group_filters"); err == nil {
			if decoded, err := url.QueryUnescape(cookie); err == nil {
				var state map[string]string
				if err := json.Unmarshal([]byte(decoded), &state); err == nil {
					searchTerm = state["search"]
					statusTerm = state["status"]
				}
			}
		}
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	groupRepo := repository.NewGroupRepository(db)
	groups, err := groupRepo.List()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch groups")
		return
	}

	groupList := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		groupIDUint, ok := group.ID.(uint)
		memberCount := 0
		if ok {
			if members, err := groupRepo.GetGroupMembers(groupIDUint); err == nil {
				memberCount = len(members)
			}
		}

		groupList = append(groupList, makeAdminGroupEntry(group, memberCount))
	}

	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Template renderer unavailable")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/groups.pongo2", pongo2.Context{
		"Groups":     groupList,
		"User":       getUserMapForTemplate(c),
		"ActivePage": "admin",
	})
}

func makeAdminGroupEntry(group *models.Group, memberCount int) gin.H {
	isSystem := group.Name == "admin" || group.Name == "users" || group.Name == "stats"
	isActive := group.ValidID == 1
	return gin.H{
		"ID":          group.ID,
		"Name":        group.Name,
		"Description": group.Comments,
		"Comments":    group.Comments,
		"MemberCount": memberCount,
		"ValidID":     group.ValidID,
		"IsActive":    isActive,
		"IsSystem":    isSystem,
		"CreateTime":  group.CreateTime,
	}
}

// handleCreateGroup creates a new group.
func handleCreateGroup(c *gin.Context) {
	var groupForm struct {
		Name     string `form:"name" json:"name" binding:"required"`
		Comments string `form:"comments" json:"comments"`
		ValidID  int    `form:"valid_id" json:"valid_id"`
	}

	if err := c.ShouldBind(&groupForm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get current user for audit fields
	userID := 1 // Default to system user
	if userCtx, ok := c.Get("user"); ok {
		if userData, ok := userCtx.(*models.User); ok && userData != nil {
			userID = int(userData.ID)
		}
	}

	validID := groupForm.ValidID
	if validID <= 0 {
		validID = 1 // Active unless a status was chosen
	}

	groupRepo := repository.NewGroupRepository(db)
	group := &models.Group{
		Name:     groupForm.Name,
		Comments: groupForm.Comments,
		ValidID:  validID,
		CreateBy: userID,
		ChangeBy: userID,
	}

	if err := groupRepo.Create(group); err != nil {
		// Duplicate detection for UX/tests
		errLower := strings.ToLower(err.Error())
		if strings.Contains(errLower, "duplicate") ||
			strings.Contains(errLower, "exists") ||
			strings.Contains(errLower, "unique") {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": "Group with this name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"group":   group,
	})
}

// handleGetGroup returns group details.
func handleGetGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)
	group, err := groupRepo.GetByID(uint(groupID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	// Get group members
	var members []*models.User
	if groupIDUint, ok := group.ID.(uint); ok {
		if m, err := groupRepo.GetGroupMembers(groupIDUint); err == nil {
			members = m
		}
	}

	// Format response to match frontend expectations
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"role": gin.H{
			"ID":          group.ID,
			"Name":        group.Name,
			"Description": group.Comments,
			"IsActive":    group.ValidID == 1,
			"Permissions": []string{}, // Groups don't have permissions in OTRS
		},
		"members": members,
	})
}

// handleUpdateGroup updates a group.
func handleUpdateGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	var groupForm struct {
		Name     string `form:"name" json:"name"`
		Comments string `form:"comments" json:"comments"`
		ValidID  int    `form:"valid_id" json:"valid_id"`
	}

	if err := c.ShouldBind(&groupForm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)
	group, err := groupRepo.GetByID(uint(groupID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	// Get current user for audit fields
	userID := 1 // Default to system user
	if userCtx, ok := c.Get("user"); ok {
		if userData, ok := userCtx.(*models.User); ok && userData != nil {
			userID = int(userData.ID)
		}
	}

	// Update group fields
	if groupForm.Name != "" {
		group.Name = groupForm.Name
	}
	if groupForm.Comments != "" {
		group.Comments = groupForm.Comments
	}
	if groupForm.ValidID > 0 {
		group.ValidID = groupForm.ValidID
	}
	group.ChangeBy = userID

	if err := groupRepo.Update(group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"group":   group,
	})
}

// handleDeleteGroup deletes a group.
func handleDeleteGroup(c *gin.Context) {
	groupID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)
	group, err := groupRepo.GetByID(uint(groupID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	// Don't delete system groups
	if group.Name == "admin" || group.Name == "users" || group.Name == "stats" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete system groups"})
		return
	}

	// Get current user for audit fields
	userID := 1 // Default to system user
	if userCtx, ok := c.Get("user"); ok {
		if userData, ok := userCtx.(*models.User); ok && userData != nil {
			userID = int(userData.ID)
		}
	}

	// In OTRS style, we mark groups as invalid rather than deleting them
	group.ValidID = 2 // Mark as invalid
	group.ChangeBy = userID

	if err := groupRepo.Update(group); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Group deleted successfully",
	})
}

// Advanced search handlers are defined in ticket_advanced_search_handler.go

// Ticket merge handlers are defined in ticket_merge_handler.go

// Permission Management handlers

// handleAdminPermissions displays the permission management page.
func handleAdminPermissions(c *gin.Context) {
	// Prevent caching of this page
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	db, err := database.GetDB()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Database connection failed")
		return
	}

	// Get all users
	userRepo := repository.NewUserRepository(db)
	users, err := userRepo.List()
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, "Failed to fetch users")
		return
	}

	// Get selected user ID from query param
	selectedUserIDStr := c.Query("user")
	var selectedUserID uint
	if selectedUserIDStr != "" {
		if id, err := strconv.ParseUint(selectedUserIDStr, 10, 32); err == nil {
			selectedUserID = uint(id)
		}
	}

	// If a user is selected, get their permission matrix
	var permissionMatrix *service.PermissionMatrix
	if selectedUserID > 0 {
		permService := service.NewPermissionService(db)
		permissionMatrix, err = permService.GetUserPermissionMatrix(selectedUserID)
		if err != nil {
			// Log error but don't fail the page
			log.Printf("Failed to get permission matrix for user %d: %v", selectedUserID, err)
		} else if permissionMatrix != nil {
			log.Printf("Got permission matrix for user %d: %d groups", selectedUserID, len(permissionMatrix.Groups))
		} else {
			log.Printf("Permission matrix is nil for user %d", selectedUserID)
		}
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/permissions.pongo2", pongo2.Context{
		"Users":            users,
		"SelectedUserID":   selectedUserID,
		"PermissionMatrix": permissionMatrix,
		"User":             getUserMapForTemplate(c),
		"ActivePage":       "admin",
	})
}

// handleGetUserPermissionMatrix returns the permission matrix for a user.
func handleGetUserPermissionMatrix(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	permService := service.NewPermissionService(db)
	matrix, err := permService.GetUserPermissionMatrix(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    matrix,
	})
}

// handleUpdateUserPermissions updates all permissions for a user.
func handleUpdateUserPermissions(c *gin.Context) {
	userIDStr := c.Param("userId")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	// Parse permission data from form
	permissions := make(map[uint]map[string]bool)

	// Parse form data - handle both multipart and urlencoded
	var formValues map[string][]string

	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		// Parse multipart form
		if err := c.Request.ParseMultipartForm(128 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid multipart form data"})
			return
		}
		formValues = c.Request.MultipartForm.Value
	} else {
		// Parse URL-encoded form
		if err := c.Request.ParseForm(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid form data"})
			return
		}
		formValues = c.Request.PostForm
	}

	// First, collect all groups that have checkboxes
	groupsWithCheckboxes := make(map[uint]bool)

	// Process each permission checkbox
	// Format: perm_<groupID>_<permissionKey>
	for key, values := range formValues {
		if strings.HasPrefix(key, "perm_") && len(values) > 0 {
			// Split into exactly 3 parts to handle permission keys with underscores (e.g., "move_into")
			parts := strings.SplitN(key, "_", 3)
			if len(parts) == 3 {
				groupID, err := strconv.ParseUint(parts[1], 10, 32)
				if err != nil {
					continue // Skip invalid group IDs
				}
				permKey := parts[2]

				groupsWithCheckboxes[uint(groupID)] = true

				if permissions[uint(groupID)] == nil {
					permissions[uint(groupID)] = make(map[string]bool)
				}
				permissions[uint(groupID)][permKey] = (values[0] == "1" || values[0] == "on")
			}
		}
	}

	// Ensure all groups with checkboxes have all permission keys
	for groupID := range groupsWithCheckboxes {
		if permissions[groupID] == nil {
			permissions[groupID] = make(map[string]bool)
		}
		// Ensure all permission keys exist (default to false if not set)
		for _, key := range []string{"ro", "move_into", "create", "note", "owner", "priority", "rw"} {
			if _, exists := permissions[groupID][key]; !exists {
				permissions[groupID][key] = false
			}
		}
	}

	// Debug log
	log.Printf("DEBUG: Updating permissions for user %d, received %d groups with checkboxes", userID, len(groupsWithCheckboxes))
	for gid, perms := range permissions {
		hasAny := false
		for _, v := range perms {
			if v {
				hasAny = true
				break
			}
		}
		log.Printf("  Group %d: has permissions=%v", gid, hasAny)
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	permService := service.NewPermissionService(db)
	if err := permService.UpdateUserPermissions(uint(userID), permissions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update permissions"})
		return
	}

	// Always return JSON for this endpoint since it's called via AJAX
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Permissions updated successfully",
	})
}

// handleClonePermissions replaces the target agent's group permissions with a
// copy of the source agent's (the "Clone Permissions" modal on /admin/permissions).
func handleClonePermissions(c *gin.Context) {
	parseID := func(field string) (uint, bool) {
		id, err := strconv.ParseUint(strings.TrimSpace(c.PostForm(field)), 10, 32)
		return uint(id), err == nil && id > 0
	}
	sourceID, okSource := parseID("source_user_id")
	targetID, okTarget := parseID("target_user_id")
	if !okSource || !okTarget {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "source_user_id and target_user_id must be user IDs"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	actorID := GetUserIDFromCtxUint(c, 1)
	err = service.NewPermissionService(db).CloneUserPermissions(sourceID, targetID, actorID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Permissions cloned successfully"})
	case errors.Is(err, service.ErrCloneSameUser):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Source and target users must be different"})
	case errors.Is(err, repository.ErrPermissionUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "User not found"})
	default:
		log.Printf("clone permissions %d -> %d: %v", sourceID, targetID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to clone permissions"})
	}
}

// handleAddUserToGroup assigns a user to a group.
func handleAddUserToGroup(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid group ID"})
		return
	}

	var req struct {
		UserID uint `form:"user_id" json:"user_id" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request data"})

		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)

	// Add user to group
	err = groupRepo.AddUserToGroup(req.UserID, uint(groupID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to add user to group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User assigned to group successfully",
	})
}

// handleRemoveUserFromGroup removes a user from a group.
func handleRemoveUserFromGroup(c *gin.Context) {
	groupIDStr := c.Param("id")
	userIDStr := c.Param("userId")

	groupID, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid group ID"})
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid user ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	groupRepo := repository.NewGroupRepository(db)

	// Remove user from group
	err = groupRepo.RemoveUserFromGroup(uint(userID), uint(groupID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to remove user from group"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User removed from group successfully",
	})
}

// handleGroupPermissions shows a queue-centric matrix for a group's assignments.
func handleGroupPermissions(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupIDValue, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid group ID"})
		return
	}
	groupID := uint(groupIDValue)

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	data, err := fetchGroupPermissionsData(db, groupID)
	if err != nil {
		errMsg := err.Error()
		status := http.StatusInternalServerError
		if strings.Contains(strings.ToLower(errMsg), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"success": false, "error": errMsg})
		return
	}

	if wantsJSONResponse(c) {
		respondWithGroupPermissionsJSON(c, data)
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/group_permissions.pongo2", pongo2.Context{
		"Group":          data.Group,
		"Members":        data.Members,
		"Queues":         data.Queues,
		"PermissionKeys": groupPermissionDefinitions,
		"User":           getUserMapForTemplate(c),
		"ActivePage":     "admin",
	})
}

// handleSaveGroupPermissions updates permission flags for members in a group.
func handleSaveGroupPermissions(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupIDValue, err := strconv.ParseUint(groupIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid group ID"})
		return
	}
	groupID := uint(groupIDValue)

	var payload saveGroupPermissionsRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid permission payload"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	permService := service.NewPermissionService(db)
	for _, assignment := range payload.Assignments {
		if assignment.UserID == 0 {
			continue
		}
		normalized := normalizeGroupPermissionMap(assignment.Permissions)
		if err := permService.UpdateUserPermissions(assignment.UserID, map[uint]map[string]bool{
			groupID: normalized,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update permissions"})
			return
		}
	}

	data, err := fetchGroupPermissionsData(db, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to refresh permissions"})
		return
	}

	respondWithGroupPermissionsJSON(c, data)
}

// handleGetGroups returns all groups as JSON for API requests.
func handleGetGroups(c *gin.Context) {
	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Query for all groups
	query := database.ConvertPlaceholders(`
		SELECT id, name, valid_id
		FROM groups
		WHERE valid_id = 1
		ORDER BY name`)

	rows, err := db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch groups",
		})
		return
	}
	defer rows.Close()

	groups := []map[string]interface{}{}
	for rows.Next() {
		var id, validID int
		var name string
		err := rows.Scan(&id, &name, &validID)
		if err != nil {
			continue
		}

		group := map[string]interface{}{
			"id":       id,
			"name":     name,
			"valid_id": validID,
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating groups: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"groups":  groups,
	})
}

// handleGetGroupMembers returns users assigned to a group.
func handleGetGroupMembers(c *gin.Context) {
	groupID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Query for group members
	query := database.ConvertPlaceholders(`
		SELECT DISTINCT u.id, u.login, u.first_name, u.last_name
		FROM users u
		INNER JOIN group_user gu ON u.id = gu.user_id
		WHERE gu.group_id = ? AND u.valid_id = 1
		ORDER BY u.id`)

	rows, err := db.Query(query, groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch group members",
		})
		return
	}
	defer rows.Close()

	members := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var login, firstName, lastName sql.NullString
		err := rows.Scan(&id, &login, &firstName, &lastName)
		if err != nil {
			continue
		}

		member := map[string]interface{}{
			"id":         id,
			"login":      login.String,
			"first_name": firstName.String,
			"last_name":  lastName.String,
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating group members: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    members,
		"members": members,
		"count":   len(members),
	})
}

// handleGetGroupAPI returns group details as JSON for API requests.
func handleGetGroupAPI(c *gin.Context) {
	groupID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Query for group details
	var id int
	var name, comments sql.NullString
	var validID sql.NullInt32

	query := database.ConvertPlaceholders(`SELECT id, name, comments, valid_id FROM groups WHERE id = ?`)
	err = db.QueryRow(query, groupID).Scan(&id, &name, &comments, &validID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Group not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch group",
			})
		}
		return
	}

	group := map[string]interface{}{
		"id":       id,
		"name":     name.String,
		"comments": comments.String,
		"valid_id": validID.Int32,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    group,
	})
}
