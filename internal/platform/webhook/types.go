// Package webhook implements outbound webhooks: endpoint configuration stored in
// gk_webhook, a durable delivery log in gk_webhook_delivery, HMAC-signed HTTP
// delivery with bounded retries, and the event-source cursors product code uses
// to turn database changes into events exactly once.
package webhook

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Delivery status values stored in gk_webhook_delivery.status.
const (
	StatusPending    = "pending"
	StatusDelivering = "delivering"
	StatusDelivered  = "delivered"
	StatusFailed     = "failed"
)

// HTTP headers set on every outbound delivery. The signature header carries
// "sha256=<hex HMAC-SHA256 of the raw body keyed by the webhook secret>", the
// format GoatFlow's own inbound plugin webhooks verify.
const (
	SignatureHeader = "X-Webhook-Signature"
	EventHeader     = "X-Webhook-Event"
	DeliveryHeader  = "X-Webhook-Delivery"
	UserAgent       = "GoatFlow-Webhook/1.0"
)

// TestEvent is the event type of deliveries created by the test endpoint.
const TestEvent = "webhook.test"

// Limits and defaults for webhook configuration.
const (
	DefaultRetryCount     = 3
	MaxRetryCount         = 10
	DefaultTimeoutSeconds = 10
	MaxTimeoutSeconds     = 60
	maxNameLength         = 200
	maxURLLength          = 2000
	maxResponseBodyBytes  = 4096
	maxErrorLength        = 1000
	maxHeaders            = 20
	maxHeaderNameLength   = 256
	maxHeaderValueLength  = 1024
)

// validActive and validInactive are the OTRS valid table ids used for valid_id.
const (
	validActive   = 1
	validInactive = 2
)

// ErrNotFound is returned when a webhook or delivery does not exist.
var ErrNotFound = errors.New("not found")

// ErrDuplicateName is returned when another webhook already uses the name.
var ErrDuplicateName = errors.New("a webhook with this name already exists")

// ValidationError describes invalid webhook configuration.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func invalid(format string, args ...interface{}) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

// Webhook is a configured outbound endpoint. The secret and the custom header
// values are write-only: they are stored encrypted and only masked hints are
// ever returned.
type Webhook struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
	// Headers holds custom header values. It is only filled when values are
	// being saved or sent, never serialised.
	Headers map[string]string `json:"-"`
	// HeaderHints maps each custom header name to a masked hint of its value.
	HeaderHints    map[string]string `json:"header_hints"`
	HasSecret      bool              `json:"has_secret"`
	SecretHint     string            `json:"secret_hint,omitempty"`
	RetryCount     int               `json:"retry_count"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	IsActive       bool              `json:"is_active"`
	CreatedAt      time.Time         `json:"created_at"`
	CreatedBy      int               `json:"created_by"`
	UpdatedAt      time.Time         `json:"updated_at"`
	UpdatedBy      int               `json:"updated_by"`
}

// Subscribes reports whether the webhook wants deliveries for event.
func (w *Webhook) Subscribes(event string) bool {
	for _, e := range w.Events {
		if e == event {
			return true
		}
	}
	return false
}

// Normalize trims input, applies defaults and validates the configuration.
// knownEvent reports whether an event name may be subscribed to. Headers are
// validated when set (nil means "not being changed").
func (w *Webhook) Normalize(knownEvent func(string) bool) error {
	w.Name = strings.TrimSpace(w.Name)
	w.URL = strings.TrimSpace(w.URL)
	if w.Name == "" {
		return invalid("name is required")
	}
	if len(w.Name) > maxNameLength {
		return invalid("name must be at most %d characters", maxNameLength)
	}
	if w.URL == "" {
		return invalid("url is required")
	}
	if len(w.URL) > maxURLLength {
		return invalid("url must be at most %d characters", maxURLLength)
	}
	u, err := url.Parse(w.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return invalid("url must be an absolute http or https URL")
	}
	if err := checkTargetHost(u.Hostname()); err != nil {
		return invalid("%s", err.Error())
	}
	if len(w.Events) == 0 {
		return invalid("at least one event is required")
	}
	seen := make(map[string]bool, len(w.Events))
	events := make([]string, 0, len(w.Events))
	for _, e := range w.Events {
		e = strings.TrimSpace(e)
		if !knownEvent(e) {
			return invalid("unknown event %q", e)
		}
		if !seen[e] {
			seen[e] = true
			events = append(events, e)
		}
	}
	w.Events = events
	if err := validateHeaders(w.Headers); err != nil {
		return err
	}
	if w.RetryCount < 0 || w.RetryCount > MaxRetryCount {
		return invalid("retry_count must be between 0 and %d", MaxRetryCount)
	}
	if w.TimeoutSeconds < 1 || w.TimeoutSeconds > MaxTimeoutSeconds {
		return invalid("timeout_seconds must be between 1 and %d", MaxTimeoutSeconds)
	}
	return nil
}

func validateHeaders(headers map[string]string) error {
	if len(headers) > maxHeaders {
		return invalid("at most %d custom headers are allowed", maxHeaders)
	}
	lower := make(map[string]bool, len(headers))
	for k, v := range headers {
		if !validHeaderName(k) {
			return invalid("invalid header name %q", k)
		}
		if isReservedHeader(k) {
			return invalid("header %q is set by GoatFlow and cannot be overridden", k)
		}
		if lower[strings.ToLower(k)] {
			return invalid("header %q is given more than once", k)
		}
		lower[strings.ToLower(k)] = true
		if len(v) > maxHeaderValueLength {
			return invalid("value of header %q must be at most %d characters", k, maxHeaderValueLength)
		}
		if !validHeaderValue(v) {
			return invalid("value of header %q contains a control character", k)
		}
	}
	return nil
}

func validHeaderName(name string) bool {
	if name == "" || len(name) > maxHeaderNameLength {
		return false
	}
	for _, r := range name {
		if r > 126 || r <= 32 || strings.ContainsRune("()<>@,;:\\\"/[]?={}", r) {
			return false
		}
	}
	return true
}

// validHeaderValue rejects control characters (CR and LF would split the
// header); horizontal tab is allowed.
func validHeaderValue(v string) bool {
	for _, r := range v {
		if (r < 0x20 && r != '\t') || r == 0x7f {
			return false
		}
	}
	return true
}

func isReservedHeader(name string) bool {
	switch strings.ToLower(name) {
	case "content-type", "content-length", "host", "user-agent",
		strings.ToLower(SignatureHeader), strings.ToLower(EventHeader), strings.ToLower(DeliveryHeader):
		return true
	}
	return false
}

// Delivery is one event sent (or to be sent) to one webhook.
type Delivery struct {
	ID            int64      `json:"id"`
	WebhookID     int64      `json:"webhook_id"`
	Event         string     `json:"event"`
	Payload       string     `json:"payload,omitempty"`
	Status        string     `json:"status"`
	Success       bool       `json:"success"`
	Attempts      int        `json:"attempts"`
	StatusCode    *int       `json:"status_code"`
	Response      string     `json:"response,omitempty"`
	Error         string     `json:"error,omitempty"`
	DurationMS    *int       `json:"duration_ms"`
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Envelope is the JSON body POSTed to webhook endpoints.
type Envelope struct {
	Event      string      `json:"event"`
	OccurredAt time.Time   `json:"occurred_at"`
	Data       interface{} `json:"data"`
}
