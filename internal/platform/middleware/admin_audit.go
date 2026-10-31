package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
)

// AdminRequestAction is the generic event for admin requests whose handler
// does not record a more specific one.
const AdminRequestAction = "admin.request"

// AuditAdminDenied records that an authenticated caller was refused an admin
// route. Unauthenticated requests are not recorded here: the auth layer
// answers those before any admin check.
func AuditAdminDenied(c *gin.Context) {
	if _, ok := c.Get("user_id"); !ok {
		if _, tok := c.Get("api_token"); !tok {
			return
		}
	}
	audit.RecordRequest(c, audit.Event{
		Category: audit.CategoryAdmin,
		Action:   AdminRequestAction,
		Outcome:  audit.OutcomeDenied,
		Details: map[string]any{
			"method": c.Request.Method,
			"route":  c.FullPath(),
			"path":   c.Request.URL.Path,
		},
	})
}

// AuditAdminWrite runs the rest of an admin request and, for writes whose
// handler recorded no event of its own, records one generic admin.request
// event with the outcome taken from the response status. Request bodies and
// query strings are never recorded: they can carry passwords and keys.
func AuditAdminWrite(c *gin.Context) {
	if !isWriteMethod(c.Request.Method) {
		c.Next()
		return
	}
	c.Next()
	if audit.Recorded(c) {
		return
	}
	status := c.Writer.Status()
	details := map[string]any{
		"method": c.Request.Method,
		"route":  c.FullPath(),
		"status": status,
	}
	if len(c.Params) > 0 {
		params := make(map[string]string, len(c.Params))
		for _, p := range c.Params {
			params[p.Key] = p.Value
		}
		details["params"] = params
	}
	audit.RecordRequest(c, audit.Event{
		Category: audit.CategoryAdmin,
		Action:   AdminRequestAction,
		Outcome:  OutcomeForStatus(status),
		Details:  details,
	})
}

// OutcomeForStatus maps an HTTP response status to an audit outcome.
func OutcomeForStatus(status int) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return audit.OutcomeDenied
	case status < http.StatusBadRequest:
		return audit.OutcomeSuccess
	default:
		return audit.OutcomeFailure
	}
}

func isWriteMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}
