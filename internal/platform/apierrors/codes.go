// Package apierrors provides structured API error codes and responses.
// All codes are namespaced (e.g., "core:unauthorized", "stats:export_failed").
package apierrors

import "net/http"

// Core error codes - registered automatically at init
const (
	// Authentication & Authorization
	CodeUnauthorized = "core:unauthorized"
	CodeForbidden    = "core:forbidden"
	CodeInvalidToken = "core:invalid_token"
	CodeTokenExpired = "core:token_expired"
	CodeTokenRevoked = "core:token_revoked"

	// Request errors
	CodeInvalidRequest    = "core:invalid_request"
	CodeValidationFailed  = "core:validation_failed"
	CodeInvalidScope      = "core:invalid_scope"
	CodeInvalidExpiration = "core:invalid_expiration"
	CodeInvalidID         = "core:invalid_id"

	// Resource errors
	CodeNotFound      = "core:not_found"
	CodeTokenNotFound = "core:token_not_found"
	CodeConflict      = "core:conflict"

	// Rate limiting
	CodeRateLimited = "core:rate_limited"

	// Server errors
	CodeInternalError      = "core:internal_error"
	CodeServiceUnavailable = "core:service_unavailable"
)

// coreErrors defines all core error codes with their default messages and HTTP status
var coreErrors = []ErrorCode{
	// Authentication & Authorization
	{Code: CodeUnauthorized, Message: "Authentication required", HTTPStatus: http.StatusUnauthorized},
	{Code: CodeForbidden, Message: "Permission denied", HTTPStatus: http.StatusForbidden},
	{Code: CodeInvalidToken, Message: "Invalid or malformed token", HTTPStatus: http.StatusUnauthorized},
	{Code: CodeTokenExpired, Message: "Token has expired", HTTPStatus: http.StatusUnauthorized},
	{Code: CodeTokenRevoked, Message: "Token has been revoked", HTTPStatus: http.StatusUnauthorized},

	// Request errors
	{Code: CodeInvalidRequest, Message: "Invalid request body", HTTPStatus: http.StatusBadRequest},
	{Code: CodeValidationFailed, Message: "Request validation failed", HTTPStatus: http.StatusBadRequest},
	{Code: CodeInvalidScope, Message: "Invalid scope value", HTTPStatus: http.StatusBadRequest},
	{Code: CodeInvalidExpiration, Message: "Invalid expiration format", HTTPStatus: http.StatusBadRequest},
	{Code: CodeInvalidID, Message: "Invalid ID format", HTTPStatus: http.StatusBadRequest},

	// Resource errors
	{Code: CodeNotFound, Message: "Resource not found", HTTPStatus: http.StatusNotFound},
	{Code: CodeTokenNotFound, Message: "Token not found", HTTPStatus: http.StatusNotFound},
	{Code: CodeConflict, Message: "Resource conflict", HTTPStatus: http.StatusConflict},

	// Rate limiting
	{Code: CodeRateLimited, Message: "Too many requests", HTTPStatus: http.StatusTooManyRequests},

	// Server errors
	{Code: CodeInternalError, Message: "Internal server error", HTTPStatus: http.StatusInternalServerError},
	{Code: CodeServiceUnavailable, Message: "Service temporarily unavailable", HTTPStatus: http.StatusServiceUnavailable},
}

func init() {
	// Register all core error codes
	for _, e := range coreErrors {
		Registry.Register(e)
	}
}
