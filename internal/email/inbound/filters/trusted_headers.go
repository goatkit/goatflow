package filters

import (
	"bytes"
	"context"
	"log"
	"mime"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
)

// TrustedHeadersFilter captures X-GoatFlow-* overrides when the mailbox allows trusted headers.
// It mirrors the OTRS PostMaster behavior where select headers may override routing metadata.
type TrustedHeadersFilter struct {
	logger       *log.Logger
	extraHeaders []string
}

// NewTrustedHeadersFilter constructs a filter instance.
func NewTrustedHeadersFilter(logger *log.Logger, extraHeaders ...string) *TrustedHeadersFilter {
	return &TrustedHeadersFilter{logger: logger, extraHeaders: normalizeHeaderList(extraHeaders)}
}

// ID returns the filter identifier.
func (f *TrustedHeadersFilter) ID() string { return "trusted_headers" }

// Apply inspects trusted headers and stores overrides inside the annotations map.
func (f *TrustedHeadersFilter) Apply(ctx context.Context, m *MessageContext) error {
	if m == nil || m.Message == nil || len(m.Message.Raw) == 0 {
		return nil
	}
	if !m.Account.AllowTrustedHeaders {
		return nil
	}
	reader, err := mail.ReadMessage(bytes.NewReader(m.Message.Raw))
	if err != nil {
		f.logf("trusted_headers: parse failed: %v", err)
		return nil
	}
	dec := mime.WordDecoder{}
	setStr := func(key, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if m.Annotations == nil {
			m.Annotations = make(map[string]any)
		}
		m.Annotations[key] = value
	}
	setInt := func(key, raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if id, err := strconv.Atoi(raw); err == nil && id > 0 {
			if m.Annotations == nil {
				m.Annotations = make(map[string]any)
			}
			m.Annotations[key] = id
		}
	}
	setBool := func(key, raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		switch strings.ToLower(raw) {
		case "1", "true", "yes", "y":
			if m.Annotations == nil {
				m.Annotations = make(map[string]any)
			}
			m.Annotations[key] = true
		case "0", "false", "no", "n":
			if m.Annotations == nil {
				m.Annotations = make(map[string]any)
			}
			m.Annotations[key] = false
		}
	}

	decode := func(v string) string {
		v = strings.TrimSpace(v)
		if v == "" {
			return ""
		}
		decoded, err := dec.DecodeHeader(v)
		if err != nil {
			return v
		}
		return decoded
	}

	setInt(AnnotationQueueIDOverride, firstHeaderValue(reader.Header, queueIDHeaders))
	queueName := decode(firstHeaderValue(reader.Header, queueNameHeaders))
	if queueName != "" {
		setStr(AnnotationQueueNameOverride, queueName)
	}

	setInt(AnnotationPriorityIDOverride, firstHeaderValue(reader.Header, priorityIDHeaders))
	setStr(AnnotationTitleOverride, decode(firstHeaderValue(reader.Header, titleHeaders)))

	customerID := decode(firstHeaderValue(reader.Header, customerIDHeaders))
	if customerID != "" {
		setStr(AnnotationCustomerIDOverride, customerID)
	}
	customerUser := decode(firstHeaderValue(reader.Header, customerUserHeaders))
	if customerUser != "" {
		setStr(AnnotationCustomerUserOverride, customerUser)
	}
	setBool(AnnotationIgnoreMessage, firstHeaderValue(reader.Header, ignoreHeaders))

	if len(f.extraHeaders) > 0 {
		for _, headerName := range f.extraHeaders {
			raw := reader.Header.Get(headerName)
			if raw == "" {
				continue
			}
			decoded := decode(raw)
			if decoded == "" {
				continue
			}
			setStr(annotationTrustedHeaderKey(headerName), decoded)
		}
	}

	return nil
}

func (f *TrustedHeadersFilter) logf(format string, args ...any) {
	if f == nil || f.logger == nil {
		return
	}
	f.logger.Printf(format, args...)
}

var (
	queueIDHeaders      = canonicalHeaderList("X-GoatFlow-QueueID", "X-OTRS-QueueID")
	queueNameHeaders    = canonicalHeaderList("X-GoatFlow-Queue", "X-GoatFlow-QueueName", "X-OTRS-Queue", "X-OTRS-QueueName")
	priorityIDHeaders   = canonicalHeaderList("X-GoatFlow-PriorityID", "X-OTRS-PriorityID")
	titleHeaders        = canonicalHeaderList("X-GoatFlow-Title", "X-OTRS-Title")
	customerIDHeaders   = canonicalHeaderList("X-GoatFlow-CustomerID", "X-OTRS-CustomerID")
	customerUserHeaders = canonicalHeaderList("X-GoatFlow-CustomerUser", "X-GoatFlow-CustomerUserID", "X-OTRS-CustomerUser", "X-OTRS-CustomerUserID")
	ignoreHeaders       = canonicalHeaderList("X-GoatFlow-Ignore", "X-OTRS-Ignore")
)

func firstHeaderValue(header mail.Header, names []string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}

func canonicalHeaderList(values ...string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		canonical := textproto.CanonicalMIMEHeaderKey(value)
		key := strings.ToLower(canonical)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, canonical)
	}
	return out
}

func normalizeHeaderList(values []string) []string {
	return canonicalHeaderList(values...)
}

func annotationTrustedHeaderKey(headerName string) string {
	return AnnotationTrustedHeaderPrefix + strings.ToLower(headerName)
}
