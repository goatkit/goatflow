package apierrors

import (
	"github.com/gin-gonic/gin"
)

// APIError represents the JSON error response structure
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error sends an error response using a registered error code
// It looks up the code in the registry for HTTP status and default message
func Error(c *gin.Context, code string) {
	status := Registry.HTTPStatus(code)
	message := Registry.Message(code)
	c.JSON(status, gin.H{"error": APIError{Code: code, Message: message}})
}

// ErrorWithMessage sends an error response with a custom message
// Useful when the message needs dynamic content (e.g., validation details)
func ErrorWithMessage(c *gin.Context, code, message string) {
	status := Registry.HTTPStatus(code)
	c.JSON(status, gin.H{"error": APIError{Code: code, Message: message}})
}

// ErrorWithStatus sends an error response with a custom HTTP status
// Use when the registered status isn't appropriate for the context
func ErrorWithStatus(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": APIError{Code: code, Message: message}})
}

// New creates an APIError without sending a response
// Useful for building error responses manually
func New(code string) APIError {
	return APIError{
		Code:    code,
		Message: Registry.Message(code),
	}
}

// NewWithMessage creates an APIError with a custom message
func NewWithMessage(code, message string) APIError {
	return APIError{
		Code:    code,
		Message: message,
	}
}
