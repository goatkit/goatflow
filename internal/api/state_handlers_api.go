package api

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "github.com/gotrs-io/gotrs-ce/internal/database"
)

// handleGetStates returns list of ticket states
func handleGetStates(c *gin.Context) {
    db, err := database.GetDB()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch states"})
        return
    }
    rows, err := db.Query(database.ConvertPlaceholders(`SELECT id, name, type_id, comments, valid_id FROM ticket_state`))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch states"})
        return
    }
    defer rows.Close()
    var data []gin.H
    for rows.Next() {
        var (
            id, typeID, validID int
            name string
            comments *string
        )
        if err := rows.Scan(&id, &name, &typeID, &comments, &validID); err == nil {
            row := gin.H{"id": id, "name": name, "type_id": typeID, "valid_id": validID}
            if comments != nil {
                row["comments"] = *comments
            } else {
                row["comments"] = ""
            }
            data = append(data, row)
        }
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// handleCreateState creates a new ticket state
func handleCreateState(c *gin.Context) {
    var input struct {
        Name     string  `json:"name"`
        TypeID   int     `json:"type_id"`
        Comments *string `json:"comments"`
    }
    if err := c.ShouldBindJSON(&input); err != nil || input.Name == "" || input.TypeID == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name and type_id are required"})
        return
    }
    db, err := database.GetDB()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create state"})
        return
    }
    var id int
    err = db.QueryRow(database.ConvertPlaceholders(`
        INSERT INTO ticket_state (name, type_id, comments, valid_id, create_by, change_by)
        VALUES ($1,$2,$3,1,1,1) RETURNING id
    `), input.Name, input.TypeID, input.Comments).Scan(&id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create state"})
        return
    }
    c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{
        "id": id, "name": input.Name, "type_id": input.TypeID, "comments": func() string { if input.Comments!=nil {return *input.Comments}; return "" }(), "valid_id": 1,
    }})
}

// handleUpdateState updates a ticket state
func handleUpdateState(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.Atoi(idStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid state ID"})
        return
    }
    var input struct {
        Name     *string `json:"name"`
        Comments *string `json:"comments"`
    }
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
        return
    }
    db, err := database.GetDB()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update state"})
        return
    }
    query := `UPDATE ticket_state SET change_by = $1, change_time = CURRENT_TIMESTAMP`
    args := []interface{}{1}
    argN := 2
    resp := gin.H{"id": id}
    if input.Name != nil {
        query += `, name = $` + strconv.Itoa(argN)
        args = append(args, *input.Name)
        resp["name"] = *input.Name
        argN++
    }
    if input.Comments != nil {
        query += `, comments = $` + strconv.Itoa(argN)
        args = append(args, *input.Comments)
        resp["comments"] = *input.Comments
        argN++
    }
    query += ` WHERE id = $` + strconv.Itoa(argN)
    args = append(args, id)
    result, err := db.Exec(database.ConvertPlaceholders(query), args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update state"})
        return
    }
    if rows, _ := result.RowsAffected(); rows == 0 {
        c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "State not found"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

// handleDeleteState soft deletes a ticket state
func handleDeleteState(c *gin.Context) {
    idStr := c.Param("id")
    id, err := strconv.Atoi(idStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid state ID"})
        return
    }
    db, err := database.GetDB()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete state"})
        return
    }
    // Match tests: args (id, change_by)
    result, err := db.Exec(database.ConvertPlaceholders(`UPDATE ticket_state SET valid_id = 2, change_by = $2, change_time = CURRENT_TIMESTAMP WHERE id = $1`), id, 1)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete state"})
        return
    }
    if rows, _ := result.RowsAffected(); rows == 0 {
        c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "State not found"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "State deleted successfully"})
}
