package errors

import (
	"fmt"
	"net/http"
)

// APIError represents an error from the GoatFlow API
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Code       string `json:"code"`
	Details    string `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("GoatFlow API error (%d): %s - %s", e.StatusCode, e.Message, e.Details)
	}
	return fmt.Sprintf("GoatFlow API error (%d): %s", e.StatusCode, e.Message)
}

// NewAPIError creates a new API error
func NewAPIError(statusCode int, message, code, details string) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
		Code:       code,
		Details:    details,
	}
}

// Common error types
var (
	// ErrUnauthorized represents a 401 Unauthorized error
	ErrUnauthorized = &APIError{
		StatusCode: http.StatusUnauthorized,
		Message:    "Unauthorized",
		Code:       "UNAUTHORIZED",
	}

	// ErrForbidden represents a 403 Forbidden error
	ErrForbidden = &APIError{
		StatusCode: http.StatusForbidden,
		Message:    "Forbidden",
		Code:       "FORBIDDEN",
	}

	// ErrNotFound represents a 404 Not Found error
	ErrNotFound = &APIError{
		StatusCode: http.StatusNotFound,
		Message:    "Resource not found",
		Code:       "NOT_FOUND",
	}

	// ErrBadRequest represents a 400 Bad Request error
	ErrBadRequest = &APIError{
		StatusCode: http.StatusBadRequest,
		Message:    "Bad request",
		Code:       "BAD_REQUEST",
	}

	// ErrInternalServer represents a 500 Internal Server Error
	ErrInternalServer = &APIError{
		StatusCode: http.StatusInternalServerError,
		Message:    "Internal server error",
		Code:       "INTERNAL_SERVER_ERROR",
	}

	// ErrRateLimited represents a 429 Too Many Requests error
	ErrRateLimited = &APIError{
		StatusCode: http.StatusTooManyRequests,
		Message:    "Rate limit exceeded",
		Code:       "RATE_LIMITED",
	}
)

// IsAPIError checks if an error is an API error
func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsUnauthorized checks if an error is an unauthorized error
func IsUnauthorized(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// IsForbidden checks if an error is a forbidden error
func IsForbidden(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusForbidden
	}
	return false
}

// IsRateLimited checks if an error is a rate limit error
func IsRateLimited(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusTooManyRequests
	}
	return false
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("validation failed with %d errors", len(e.Errors))
}

// NetworkError represents a network-related error
type NetworkError struct {
	Operation string `json:"operation"`
	URL       string `json:"url"`
	Err       error  `json:"error"`
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s to %s: %v", e.Operation, e.URL, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// TimeoutError represents a timeout error
type TimeoutError struct {
	Operation string `json:"operation"`
	Timeout   string `json:"timeout"`
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout error during %s after %s", e.Operation, e.Timeout)
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("configuration error for %s: %s", e.Field, e.Message)
}
