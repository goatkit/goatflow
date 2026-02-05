# API Error Codes Design

## Overview

All API errors return structured responses with stable error codes. Messages are English; clients are responsible for translating codes to local languages.

This follows the pattern used by Stripe, GitHub, AWS, and other major APIs.

## Response Format

```json
{
  "error": {
    "code": "core:token_not_found",
    "message": "Token not found or has been revoked"
  }
}
```

| Field | Description |
|-------|-------------|
| `code` | Stable, namespaced identifier. Safe to use for programmatic handling and client-side i18n. |
| `message` | Human-readable English description. May change between versions. |

HTTP status codes indicate the error category (4xx client error, 5xx server error).

## Namespacing

All error codes are namespaced to prevent collisions and clarify ownership:

| Prefix | Owner | Example |
|--------|-------|---------|
| `core:` | GoatKit core | `core:unauthorized`, `core:invalid_request` |
| `{plugin}:` | Plugin | `stats:export_failed`, `faq:article_not_found` |

## Core Error Codes

Defined in `internal/errors/codes.go`:

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `core:unauthorized` | 401 | Authentication required |
| `core:forbidden` | 403 | Permission denied |
| `core:not_found` | 404 | Resource not found |
| `core:invalid_request` | 400 | Malformed request body |
| `core:validation_failed` | 400 | Request validation failed |
| `core:conflict` | 409 | Resource conflict (e.g., duplicate) |
| `core:rate_limited` | 429 | Too many requests |
| `core:internal_error` | 500 | Server-side failure |
| `core:service_unavailable` | 503 | Service temporarily unavailable |

### Token-specific codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `core:token_not_found` | 404 | Token doesn't exist or not owned by user |
| `core:token_expired` | 401 | Token has expired |
| `core:token_revoked` | 401 | Token has been revoked |
| `core:invalid_token` | 401 | Token format invalid or verification failed |
| `core:invalid_scope` | 400 | Invalid scope value |
| `core:invalid_expiration` | 400 | Invalid expiration format |

## Plugin Error Codes

Plugins declare their error codes in `GKRegistration` alongside other capabilities:

```go
type GKRegistration struct {
    // ... other fields ...
    ErrorCodes []ErrorCodeSpec `json:"error_codes,omitempty"`
}

type ErrorCodeSpec struct {
    Code       string `json:"code"`        // "export_failed" (prefix added by host)
    Message    string `json:"message"`     // Default English message
    HTTPStatus int    `json:"http_status"` // Suggested HTTP status
}
```

Example plugin registration:

```go
func (p *StatsPlugin) GKRegister() plugin.GKRegistration {
    return plugin.GKRegistration{
        Name:    "stats",
        Version: "1.0.0",
        // ... other fields ...
        ErrorCodes: []plugin.ErrorCodeSpec{
            {Code: "export_failed", Message: "Failed to export report", HTTPStatus: 500},
            {Code: "invalid_date_range", Message: "Invalid date range specified", HTTPStatus: 400},
            {Code: "query_timeout", Message: "Report query timed out", HTTPStatus: 504},
        },
    }
}
```

The host automatically prefixes codes with the plugin name: `stats:export_failed`.

## Error Registry

The error registry (`internal/apierrors/registry.go`) provides:

1. **Registration** - Core codes registered at init, plugin codes registered on plugin load
2. **Enumeration** - List all known error codes (for docs, client SDKs)
3. **Lookup** - Get HTTP status and default message for any code

```go
// List all error codes
codes := apierrors.Registry.All()

// List codes by namespace
coreCodes := apierrors.Registry.ByNamespace("core")
pluginCodes := apierrors.Registry.ByNamespace("stats")

// List all namespaces
namespaces := apierrors.Registry.Namespaces()

// Lookup
status := apierrors.Registry.HTTPStatus("stats:export_failed")
message := apierrors.Registry.Message("stats:export_failed")
```

## Client Usage

Clients should:

1. Use `code` for programmatic error handling
2. Translate codes to local language for end users
3. Fall back to `message` if code is unknown

Example (JavaScript):

```javascript
const errorMessages = {
  'core:unauthorized': 'Bitte melden Sie sich an',
  'core:token_expired': 'Ihre Sitzung ist abgelaufen',
  // ...
};

function getErrorMessage(error) {
  return errorMessages[error.code] || error.message;
}
```

## Future Extensions

The enumeration pattern can be extended to other plugin capabilities:

- `EnumerateScopes()` - Available OAuth scopes
- `EnumeratePermissions()` - Required permissions
- `EnumerateWebhooks()` - Webhook event types
- `EnumerateMetrics()` - Exposed metrics

## Implementation

- Core package: `internal/apierrors/`
  - `codes.go` - Core error code constants and definitions
  - `registry.go` - Error code registry with namespacing
  - `response.go` - Gin response helpers
- Plugin interface: `internal/plugin/types.go`
- HTTP helpers:
  - `apierrors.Error(c, code)` - looks up code, sets HTTP status, returns JSON
  - `apierrors.ErrorWithMessage(c, code, msg)` - custom message
  - `apierrors.ErrorWithStatus(c, status, code, msg)` - custom status
