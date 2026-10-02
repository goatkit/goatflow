// Package errors defines the errors returned by the GoatFlow Go SDK.
package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
)

// APIError is a non-successful response from the GoatFlow API: an HTTP status
// outside 2xx, or a 2xx response whose envelope says {"success": false}.
//
// The API reports errors in two shapes; both end up here:
//
//	{"success": false, "error": "Ticket not found"}
//	{"error": {"code": "core:invalid_token", "message": "Invalid or malformed token"}}
type APIError struct {
	// StatusCode is the HTTP status of the response.
	StatusCode int
	// Code is the machine-readable error code when the API sent one
	// (e.g. "core:invalid_token"); empty otherwise.
	Code string
	// Message is the human-readable error from the response, or the HTTP
	// status text when the body carried none.
	Message string
	// Body is the raw response body when it was not a JSON error envelope.
	Body string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("goatflow: HTTP %d: %s (%s)", e.StatusCode, e.Message, e.Code)
	}
	return fmt.Sprintf("goatflow: HTTP %d: %s", e.StatusCode, e.Message)
}

// AsAPIError returns the *APIError in err's chain, if any.
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if stderrors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

func hasStatus(err error, status int) bool {
	apiErr, ok := AsAPIError(err)
	return ok && apiErr.StatusCode == status
}

// IsNotFound reports whether err is an API error with HTTP status 404.
func IsNotFound(err error) bool { return hasStatus(err, http.StatusNotFound) }

// IsUnauthorized reports whether err is an API error with HTTP status 401.
func IsUnauthorized(err error) bool { return hasStatus(err, http.StatusUnauthorized) }

// IsForbidden reports whether err is an API error with HTTP status 403.
func IsForbidden(err error) bool { return hasStatus(err, http.StatusForbidden) }

// IsRateLimited reports whether err is an API error with HTTP status 429.
func IsRateLimited(err error) bool { return hasStatus(err, http.StatusTooManyRequests) }

// NetworkError is a request that never produced an HTTP response
// (connection refused, DNS failure, timeout, cancelled context).
type NetworkError struct {
	Method string
	URL    string
	Err    error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("goatflow: %s %s: %v", e.Method, e.URL, e.Err)
}

func (e *NetworkError) Unwrap() error { return e.Err }

// DecodeError is a successful HTTP response whose body did not match the
// expected shape.
type DecodeError struct {
	StatusCode int
	Body       string
	Err        error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("goatflow: decoding HTTP %d response: %v", e.StatusCode, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }
