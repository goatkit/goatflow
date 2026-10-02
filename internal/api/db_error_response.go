package api

import (
	"log"

	"github.com/gin-gonic/gin"
)

// genericInternalError is the only text a client sees when a database call fails.
// Raw driver errors leak schema, SQL and server details, so they stay in the log.
const genericInternalError = "Internal server error"

// internalDBError logs a failed database call with its request context and returns
// the generic message to send to the client instead of err.Error().
func internalDBError(c *gin.Context, op string, err error) string {
	log.Printf("database error: %s %s: %s: %v", c.Request.Method, c.Request.URL.Path, op, err)
	return genericInternalError
}
