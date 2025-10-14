package api

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleListPrioritiesAPI handles GET /api/v1/priorities
func HandleListPrioritiesAPI(c *gin.Context) {
    db, err := database.GetDB()
    if err != nil || db == nil {
        c.Header("X-Guru-Error", "Priorities lookup failed: database unavailable")
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "priorities lookup failed: database unavailable"})
        return
    }

    rows, err := db.Query(database.ConvertPlaceholders(`
        SELECT id, name, valid_id
        FROM ticket_priority
        WHERE valid_id = $1
        ORDER BY id
    `), 1)
    if err != nil {
        c.Header("X-Guru-Error", "Priorities lookup failed: query error")
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to fetch priorities"})
        return
    }
    defer rows.Close()

    var items []gin.H
    for rows.Next() {
        var id, validID int
        var name string
        if err := rows.Scan(&id, &name, &validID); err != nil {
            continue
        }
        items = append(items, gin.H{"id": id, "name": name, "valid_id": validID})
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}