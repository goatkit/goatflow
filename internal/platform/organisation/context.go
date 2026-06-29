package organisation

import (
	"context"
)

// contextKey is an unexported type for org context keys.
type contextKey struct{}

// orgContextKey stores the active org ID in request context.
var orgContextKey = contextKey{}

// WithOrgID returns a new context with the active organisation ID set.
func WithOrgID(ctx context.Context, orgID int64) context.Context {
	return context.WithValue(ctx, orgContextKey, orgID)
}

// OrgIDFromContext returns the active organisation ID from the context.
// Returns 0 if no org context is set (single-org mode).
func OrgIDFromContext(ctx context.Context) int64 {
	if id, ok := ctx.Value(orgContextKey).(int64); ok {
		return id
	}
	return 0
}

// HasOrgContext returns true if the context has an active org set.
func HasOrgContext(ctx context.Context) bool {
	_, ok := ctx.Value(orgContextKey).(int64)
	return ok
}
