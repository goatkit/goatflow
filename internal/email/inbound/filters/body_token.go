package filters

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"strings"

	gomail "github.com/emersion/go-message/mail"
)

const defaultBodyTokenLimit = 256 * 1024

// BodyTokenFilter scans decoded text parts for OTRS-style ticket tokens.
type BodyTokenFilter struct {
	logger *log.Logger
	limit  int64
}

// NewBodyTokenFilter constructs the body token detector.
func NewBodyTokenFilter(logger *log.Logger) *BodyTokenFilter {
	return &BodyTokenFilter{logger: logger, limit: defaultBodyTokenLimit}
}

// ID implements Filter.
func (f *BodyTokenFilter) ID() string { return "followup_body_token" }

// Apply extracts the first text part and scans it for a ticket token.
func (f *BodyTokenFilter) Apply(ctx context.Context, m *MessageContext) error {
	if m == nil || m.Message == nil || len(m.Message.Raw) == 0 {
		return nil
	}
	if m.Annotations != nil {
		if _, exists := m.Annotations[AnnotationFollowUpTicketNumber]; exists {
			return nil
		}
	}
	body := f.extractBody(m.Message.Raw)
	if body == "" {
		return nil
	}
	if token := findTicketToken(body); token != "" {
		if m.Annotations == nil {
			m.Annotations = make(map[string]any)
		}
		m.Annotations[AnnotationFollowUpTicketNumber] = token
		f.logf("followup_body_token: detected ticket %s", token)
	}
	return nil
}

func (f *BodyTokenFilter) extractBody(raw []byte) string {
	reader, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		f.logf("followup_body_token: parse failed: %v", err)
		return f.truncate(string(raw))
	}
	var plain, html string
	for {
		part, perr := reader.NextPart()
		if errors.Is(perr, io.EOF) {
			break
		}
		if perr != nil {
			f.logf("followup_body_token: read part failed: %v", perr)
			break
		}
		inline, ok := part.Header.(*gomail.InlineHeader)
		if !ok {
			continue
		}
		body, mimeType := f.readInlineBody(part, inline)
		if body == "" {
			continue
		}
		if strings.HasPrefix(mimeType, "text/plain") && plain == "" {
			plain = body
		}
		if strings.HasPrefix(mimeType, "text/html") && html == "" {
			html = stripHTML(body)
		}
		if plain != "" {
			break
		}
	}
	if plain != "" {
		return plain
	}
	if html != "" {
		return html
	}
	return f.truncate(string(raw))
}

func (f *BodyTokenFilter) readInlineBody(part *gomail.Part, header *gomail.InlineHeader) (string, string) {
	if part == nil || header == nil {
		return "", ""
	}
	mimeType, _, err := header.ContentType()
	if err != nil || strings.TrimSpace(mimeType) == "" {
		mimeType = header.Get("Content-Type")
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if mimeType == "" {
		mimeType = "text/plain"
	}
	limit := f.bodyLimit()
	reader := io.LimitReader(part.Body, limit)
	buf, readErr := io.ReadAll(reader)
	if readErr != nil {
		f.logf("followup_body_token: read body failed: %v", readErr)
		return "", ""
	}
	return string(buf), mimeType
}

func (f *BodyTokenFilter) truncate(in string) string {
	limit := f.bodyLimit()
	if limit <= 0 || int64(len(in)) <= limit {
		return in
	}
	return in[:limit]
}

func (f *BodyTokenFilter) bodyLimit() int64 {
	if f == nil || f.limit <= 0 {
		return defaultBodyTokenLimit
	}
	return f.limit
}

func (f *BodyTokenFilter) logf(format string, args ...any) {
	if f == nil || f.logger == nil {
		return
	}
	f.logger.Printf(format, args...)
}
