package api

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleListPrioritiesAPI handles GET /api/v1/priorities
func HandleListPrioritiesAPI(c *gin.Context) {
    // Require auth
    if _, exists := c.Get("user_id"); !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }

    db, err := database.GetDB()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
        return
    }

    // Build query based on filters
    query := database.ConvertPlaceholders(`
        SELECT id, name, color, valid_id
        FROM ticket_priority
        WHERE 1=1
    `)
    args := []interface{}{}
    paramCount := 0

    if validFilter := c.Query("valid"); validFilter == "true" {
        paramCount++
        query += database.ConvertPlaceholders(` AND valid_id = $` + strconv.Itoa(paramCount))
        args = append(args, 1)
    }

    query += ` ORDER BY id`

    rows, err := db.Query(query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch priorities"})
        return
    }
    defer rows.Close()

    var items []gin.H
    for rows.Next() {
        var id, validID int
        var name, color string
        if err := rows.Scan(&id, &name, &color, &validID); err != nil {
            continue
        }
        items = append(items, gin.H{"id": id, "name": name, "color": color, "valid_id": validID})
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}