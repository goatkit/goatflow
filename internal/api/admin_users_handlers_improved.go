package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// ImprovedHandleAdminUserGet handles GET /admin/users/:id with enhanced error handling and logging.
func ImprovedHandleAdminUserGet(c *gin.Context) {
	userID := c.Param("id")
	id, err := strconv.Atoi(userID)
	if err != nil {
		fmt.Printf("ERROR: Invalid user ID: %s\n", userID)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		fmt.Printf("ERROR: Database connection failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	fmt.Printf("INFO: Fetching user %d\n", id)

	// Get user details
	var user models.User
	query := `
		SELECT id, login, title, first_name, last_name, valid_id
		FROM users
		WHERE id = $1`

	err = db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Login,
		&user.Title,
		&user.FirstName,
		&user.LastName,
		&user.ValidID,
	)

	if err != nil {
		fmt.Printf("ERROR: User %d not found: %v\n", id, err)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	fmt.Printf("INFO: User %d found: %s %s\n", id, user.FirstName, user.LastName)

	// Get user's groups with enhanced logging
	groupQuery := `
		SELECT g.id, g.name, gu.permission_key, gu.permission_value
		FROM groups g
		JOIN group_user gu ON g.id = gu.group_id
		WHERE gu.user_id = $1 AND g.valid_id = 1
		ORDER BY g.name`

	rows, err := db.Query(groupQuery, id)
	if err != nil {
		fmt.Printf("ERROR: Failed to query groups for user %d: %v\n", id, err)
		// Don't fail completely, just return empty groups
		user.Groups = []string{}
	} else {
		defer rows.Close()
		var groupNames []string

		for rows.Next() {
			var gid int
			var gname, permKey string
			var permValue int

			if err := rows.Scan(&gid, &gname, &permKey, &permValue); err == nil {
				groupNames = append(groupNames, gname)
				// omit groupDetails until used
				fmt.Printf("INFO: User %d has group: %s (id=%d, perm=%s:%d)\n",
					id, gname, gid, permKey, permValue)
			}
		}
		_ = rows.Err() //nolint:errcheck // Iteration errors don't affect response

		user.Groups = groupNames
		fmt.Printf("INFO: User %d total groups: %d (%v)\n", id, len(groupNames), groupNames)
	}

	response := gin.H{
		"success": true,
		"data": gin.H{
			"id":         user.ID,
			"login":      user.Login,
			"title":      user.Title,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"email":      user.Login, // OTRS uses login as email
			"valid_id":   user.ValidID,
			"groups":     user.Groups,
		},
		"debug": gin.H{
			"user_id":      id,
			"groups_count": len(user.Groups),
			"timestamp":    "system_time_here",
		},
	}

	fmt.Printf("SUCCESS: Returning user %d data with %d groups\n", id, len(user.Groups))
	c.JSON(http.StatusOK, response)
}

// ImprovedHandleAdminUserUpdate handles PUT /admin/users/:id with enhanced group handling.
func ImprovedHandleAdminUserUpdate(c *gin.Context) {
	userID := c.Param("id")
	id, err := strconv.Atoi(userID)
	if err != nil {
		fmt.Printf("ERROR: Invalid user ID: %s\n", userID)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	var req struct {
		Login     string   `json:"login" form:"login"`
		FirstName string   `json:"first_name" form:"first_name"`
		LastName  string   `json:"last_name" form:"last_name"`
		Email     string   `json:"email" form:"email"`
		Password  string   `json:"password" form:"password"`
		ValidID   int      `json:"valid_id" form:"valid_id"`
		Groups    []string `json:"groups" form:"groups"`
	}

	if err := c.ShouldBind(&req); err != nil {
		fmt.Printf("ERROR: Failed to bind request for user %d: %v\n", id, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request data",
		})
		return
	}

	// Enhanced request logging
	fmt.Printf("INFO: Updating user %d:\n", id)
	fmt.Printf("  - Login: %s\n", req.Login)
	fmt.Printf("  - Name: %s %s\n", req.FirstName, req.LastName)
	fmt.Printf("  - Valid ID: %d\n", req.ValidID)
	fmt.Printf("  - Groups (%d): %v\n", len(req.Groups), req.Groups)

	db, err := database.GetDB()
	if err != nil {
		fmt.Printf("ERROR: Database connection failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Start database transaction for atomicity
	tx, err := db.Begin()
	if err != nil {
		fmt.Printf("ERROR: Failed to start transaction: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction",
		})
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Update user basic info
	if req.Password != "" {
		// Update with new password
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("ERROR: Failed to hash password: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to hash password",
			})
			return
		}

		if _, err = tx.Exec(database.ConvertPlaceholders(`
			UPDATE users 
			SET login = $1, pw = $2, first_name = $3, last_name = $4, 
			    valid_id = $5, change_time = NOW(), change_by = 1
			WHERE id = $6`),
			req.Login, string(hash), req.FirstName, req.LastName, req.ValidID, id,
		); err != nil {
			fmt.Printf("ERROR: Failed to update user %d: %v\n", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to update user: %v", err),
			})
			return
		}
	} else {
		// Update without changing password
		if _, err = tx.Exec(database.ConvertPlaceholders(`
			UPDATE users 
			SET login = $1, first_name = $2, last_name = $3, 
			    valid_id = $4, change_time = NOW(), change_by = 1
			WHERE id = $5`),
			req.Login, req.FirstName, req.LastName, req.ValidID, id); err != nil {
			fmt.Printf("ERROR: Failed to update user %d: %v\n", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Failed to update user: %v", err),
			})
			return
		}
	}

	// err handled inline above

	fmt.Printf("SUCCESS: Updated user %d basic info\n", id)

	// Update group memberships with comprehensive logging
	fmt.Printf("INFO: Updating group memberships for user %d\n", id)

	// First, get current group memberships for logging
	var currentGroups []string
	rows, err := tx.Query(database.ConvertPlaceholders(`
		SELECT g.name FROM groups g 
		JOIN group_user gu ON g.id = gu.group_id 
		WHERE gu.user_id = $1 AND g.valid_id = 1`), id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var groupName string
			if err := rows.Scan(&groupName); err == nil {
				currentGroups = append(currentGroups, groupName)
			}
		}
		_ = rows.Err() //nolint:errcheck // Iteration errors don't affect response
	}
	fmt.Printf("INFO: User %d current groups: %v\n", id, currentGroups)

	// Remove all existing group memberships
	if _, err := tx.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id = $1"), id); err != nil {
		fmt.Printf("ERROR: Failed to remove existing group memberships for user %d: %v\n", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update group memberships",
		})
		return
	}
	fmt.Printf("INFO: Removed existing group memberships for user %d\n", id)

	// Add new group memberships
	var addedGroups []string
	var failedGroups []string

	for _, groupName := range req.Groups {
		groupName = strings.TrimSpace(groupName)
		if groupName == "" {
			continue
		}

		var groupID int
		groupQuery := "SELECT id FROM groups WHERE name = $1 AND valid_id = 1"
		if err := tx.QueryRow(database.ConvertPlaceholders(groupQuery), groupName).Scan(&groupID); err != nil {
			fmt.Printf("WARNING: Group '%s' not found or invalid\n", groupName)
			failedGroups = append(failedGroups, groupName)
			continue
		}

		_, err = tx.Exec(database.ConvertPlaceholders(`
            INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
            VALUES ($1, $2, 'rw', NOW(), 1, NOW(), 1)`),
			id, groupID)

		if err != nil {
			fmt.Printf("ERROR: Failed to add user %d to group '%s' (id=%d): %v\n",
				id, groupName, groupID, err)
			failedGroups = append(failedGroups, groupName)
		} else {
			fmt.Printf("SUCCESS: Added user %d to group '%s' (id=%d)\n",
				id, groupName, groupID)
			addedGroups = append(addedGroups, groupName)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		fmt.Printf("ERROR: Failed to commit transaction for user %d: %v\n", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit changes",
		})
		return
	}

	// Final verification - query the actual groups from database
	var finalGroups []string
	rows, err = db.Query(database.ConvertPlaceholders(`
		SELECT g.name FROM groups g 
		JOIN group_user gu ON g.id = gu.group_id 
		WHERE gu.user_id = $1 AND g.valid_id = 1
		ORDER BY g.name`), id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var groupName string
			if rows.Scan(&groupName) == nil {
				finalGroups = append(finalGroups, groupName)
			}
		}
		_ = rows.Err() //nolint:errcheck // Iteration errors don't affect response
	}

	fmt.Printf("FINAL VERIFICATION: User %d now has groups: %v\n", id, finalGroups)

	response := gin.H{
		"success": true,
		"message": "User updated successfully",
		"debug": gin.H{
			"user_id":          id,
			"groups_requested": req.Groups,
			"groups_added":     addedGroups,
			"groups_failed":    failedGroups,
			"groups_final":     finalGroups,
			"previous_groups":  currentGroups,
		},
	}

	// Include warnings if some groups failed
	if len(failedGroups) > 0 {
		response["warnings"] = []string{
			fmt.Sprintf("Some groups could not be assigned: %v", failedGroups),
		}
	}

	fmt.Printf("SUCCESS: User %d update completed. Groups: %v\n", id, finalGroups)
	c.JSON(http.StatusOK, response)
}
