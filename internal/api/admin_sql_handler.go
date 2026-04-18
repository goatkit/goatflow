package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
)

// adminSQLRequest is the request body for the admin SQL endpoint.
type adminSQLRequest struct {
	Query string `json:"query" binding:"required"`
	Args  []any  `json:"args,omitempty"`
}

// isAllowedStatement checks if the SQL statement type is in the allowlist.
// Allowed: SELECT, DESCRIBE/DESC, EXPLAIN, SHOW TABLES, SHOW COLUMNS.
func isAllowedStatement(query string) bool {
	trimmed := strings.TrimSpace(query)
	upper := strings.ToUpper(trimmed)

	switch {
	case strings.HasPrefix(upper, "SELECT"):
		return true
	case strings.HasPrefix(upper, "DESCRIBE"):
		return true
	case strings.HasPrefix(upper, "DESC "):
		return true
	case strings.HasPrefix(upper, "EXPLAIN"):
		return true
	case strings.HasPrefix(upper, "SHOW TABLES"):
		return true
	case strings.HasPrefix(upper, "SHOW COLUMNS"):
		return true
	default:
		return false
	}
}

// HandleAdminExecuteSQL executes a read-only SQL query.
// Requires admin middleware. Only allowlisted statement types are permitted:
// SELECT, DESCRIBE, EXPLAIN, SHOW TABLES, SHOW COLUMNS.
//
//	@Summary		Execute read-only SQL query
//	@Description	Execute a read-only SQL query (admin only). Allowed statements: SELECT, DESCRIBE, EXPLAIN, SHOW TABLES, SHOW COLUMNS.
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			request	body		adminSQLRequest	true	"SQL query and optional arguments"
//	@Success		200		{object}	map[string]interface{}	"Query results"
//	@Failure		400		{object}	map[string]interface{}	"Bad request"
//	@Failure		403		{object}	map[string]interface{}	"Forbidden"
//	@Security		BearerAuth
//	@Router			/api/v1/admin/sql [post]
func HandleAdminExecuteSQL(c *gin.Context) {
	var req adminSQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: query field is required",
		})
		return
	}

	if !isAllowedStatement(req.Query) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Only SELECT, DESCRIBE, EXPLAIN, SHOW TABLES, and SHOW COLUMNS statements are allowed",
		})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database unavailable",
		})
		return
	}

	query := database.ConvertPlaceholders(req.Query)
	rows, err := db.QueryContext(c.Request.Context(), query, req.Args...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Query failed: " + err.Error(),
		})
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get columns: " + err.Error(),
		})
		return
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"columns":    columns,
		"rows":       results,
		"rows_count": len(results),
	})
}
