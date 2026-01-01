package api

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleListGroupsAPI handles GET /api/v1/groups.
func HandleListGroupsAPI(c *gin.Context) {
	if _, ok := c.Get("user_id"); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	validParam := strings.TrimSpace(strings.ToLower(c.Query("valid")))

	query := `
		SELECT g.id, g.name, g.comments, g.valid_id,
		       g.create_time, g.change_time,
		       COUNT(gu.user_id) AS member_count
		FROM groups g
		LEFT JOIN group_user gu ON g.id = gu.group_id
	`
	var args []interface{}
	switch validParam {
	case "", "true", "1", "active":
		query += " WHERE g.valid_id = ?"
		args = append(args, 1)
	case "false", "0", "inactive":
		query += " WHERE g.valid_id <> ?"
		args = append(args, 1)
	case "all":
		// no filter
	default:
		query += " WHERE g.valid_id = ?"
		args = append(args, 1)
	}
	query += " GROUP BY g.id, g.name, g.comments, g.valid_id, g.create_time, g.change_time"
	query += " ORDER BY g.name"

	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to fetch groups"})
		return
	}
	defer func() { _ = rows.Close() }()

	type groupRow struct {
		ID         int
		Name       string
		Comments   sql.NullString
		ValidID    int
		CreateTime sql.NullTime
		ChangeTime sql.NullTime
		Members    int
	}

	var groups []gin.H
	for rows.Next() {
		var row groupRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Comments, &row.ValidID, &row.CreateTime, &row.ChangeTime, &row.Members); err != nil {
			continue
		}
		entry := gin.H{
			"id":           row.ID,
			"name":         row.Name,
			"valid_id":     row.ValidID,
			"active":       row.ValidID == 1,
			"member_count": row.Members,
		}
		if row.Comments.Valid {
			entry["comments"] = row.Comments.String
		}
		if row.CreateTime.Valid {
			entry["create_time"] = formatAPITime(row.CreateTime.Time)
		}
		if row.ChangeTime.Valid {
			entry["change_time"] = formatAPITime(row.ChangeTime.Time)
		}
		groups = append(groups, entry)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to read groups"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": groups})
}

func formatAPITime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
