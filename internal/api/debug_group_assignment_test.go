//go:build debug
// +build debug

package api

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/require"
)

// TestDebugGroupAssignmentLogic tests the actual group assignment logic step by step
func TestDebugGroupAssignmentLogic(t *testing.T) {
	// Ensure test DB is initialized
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping debug test")
	}
	defer database.CloseTestDB()

	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping debug test")
	}

	t.Run("Debug group assignment logic step by step", func(t *testing.T) {
		userID := 15
		requestedGroups := []string{"admin", "OBC", "Support"}

		log.Printf("=== DEBUGGING GROUP ASSIGNMENT FOR USER %d ===", userID)

		// STEP 1: Check current state
		log.Println("STEP 1: Current group assignments")
		currentGroups := getCurrentGroups(t, db, userID)
		log.Printf("Current groups: %v", currentGroups)

		// STEP 2: Simulate the DELETE operation
		log.Println("STEP 2: Deleting existing group memberships")
		result, err := db.Exec(database.ConvertPlaceholders(`
            DELETE FROM group_user WHERE user_id = ?
        `), userID)
		require.NoError(t, err)

		rowsAffected, _ := result.RowsAffected()
		log.Printf("Deleted %d existing group memberships", rowsAffected)

		// Verify deletion worked
		afterDeleteGroups := getCurrentGroups(t, db, userID)
		log.Printf("Groups after delete: %v", afterDeleteGroups)
		require.Empty(t, afterDeleteGroups, "Should have no groups after deletion")

		// STEP 3: Check that requested groups exist in groups table
		log.Println("STEP 3: Checking if requested groups exist in groups table")
		for _, groupName := range requestedGroups {
			var groupID int
			var validID int
			err := db.QueryRow(database.ConvertPlaceholders(`
                SELECT id, valid_id FROM groups WHERE name = ?
            `), groupName).Scan(&groupID, &validID)
			if err == sql.ErrNoRows {
				log.Printf("❌ Group '%s' does not exist in database", groupName)
			} else if err != nil {
				log.Printf("❌ Error querying group '%s': %v", groupName, err)
			} else {
				log.Printf("✅ Group '%s' exists with ID %d, valid_id %d", groupName, groupID, validID)
			}
		}

		// STEP 4: Simulate the INSERT operations with detailed logging
		log.Println("STEP 4: Adding new group memberships")
		for _, groupName := range requestedGroups {
			groupName = strings.TrimSpace(groupName)
			if groupName == "" {
				log.Printf("⚠️ Skipping empty group name")
				continue
			}

			log.Printf("Processing group: '%s'", groupName)

			var groupID int
			err = db.QueryRow(database.ConvertPlaceholders(`
                SELECT id FROM groups WHERE name = ? AND valid_id = 1
            `), groupName).Scan(&groupID)
			if err == sql.ErrNoRows {
				log.Printf("❌ Group '%s' not found or not valid", groupName)
				continue
			} else if err != nil {
				log.Printf("❌ Error looking up group '%s': %v", groupName, err)
				continue
			}

			log.Printf("Found group '%s' with ID %d", groupName, groupID)

			_, err = db.Exec(database.ConvertPlaceholders(`
				INSERT INTO group_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
                VALUES (?, ?, 'rw', 1, NOW(), 1, NOW(), 1)`),
				userID, groupID,
			)

			if err != nil {
				log.Printf("❌ Error inserting group membership for '%s': %v", groupName, err)
			} else {
				log.Printf("✅ Successfully inserted group membership for '%s'", groupName)
			}
		}

		// STEP 5: Check final state
		log.Println("STEP 5: Final group assignments")
		finalGroups := getCurrentGroups(t, db, userID)
		log.Printf("Final groups: %v", finalGroups)

		// Compare expected vs actual
		log.Println("STEP 6: Comparison")
		log.Printf("Requested: %v", requestedGroups)
		log.Printf("Actual:    %v", finalGroups)

		// Check which groups are missing
		for _, expected := range requestedGroups {
			found := false
			for _, actual := range finalGroups {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				log.Printf("❌ MISSING: Group '%s' was requested but not assigned", expected)
			} else {
				log.Printf("✅ OK: Group '%s' was correctly assigned", expected)
			}
		}

		log.Println("=== END DEBUG ===")
	})
}

// Helper function to get current groups for a user
func getCurrentGroups(t *testing.T, db *sql.DB, userID int) []string {
	var groups []string

	query := database.ConvertPlaceholders(`
		SELECT g.name
		FROM groups g
		JOIN group_user gu ON g.id = gu.group_id
		WHERE gu.user_id = ? AND g.valid_id = 1
		ORDER BY g.name`)

	rows, err := db.Query(query, userID)
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var groupName string
		err := rows.Scan(&groupName)
		require.NoError(t, err)
		groups = append(groups, groupName)
	}

	return groups
}

// TestGroupExistenceAndValidity checks if the groups we're trying to assign actually exist
func TestGroupExistenceAndValidity(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping debug test")
	}
	defer database.CloseTestDB()

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping debug test")
	}

	t.Run("Check existence of groups we're trying to assign", func(t *testing.T) {
		testGroups := []string{"admin", "OBC", "Support", "Sales", "IT", "Management"}

		fmt.Println("=== GROUP EXISTENCE CHECK ===")

		for _, groupName := range testGroups {
			var groupID int
			var validID int
			var comments string

			err := db.QueryRow(database.ConvertPlaceholders(`
				SELECT id, valid_id, COALESCE(comments, '')
                FROM groups
                WHERE name = ?`), groupName).Scan(&groupID, &validID, &comments)

			if err == sql.ErrNoRows {
				fmt.Printf("❌ Group '%s': NOT FOUND\n", groupName)
			} else if err != nil {
				fmt.Printf("❌ Group '%s': ERROR - %v\n", groupName, err)
			} else {
				status := "VALID"
				if validID != 1 {
					status = "INVALID"
				}
				fmt.Printf("✅ Group '%s': ID=%d, Status=%s, Comments='%s'\n",
					groupName, groupID, status, comments)
			}
		}

		fmt.Println("=== END GROUP CHECK ===")
	})
}
