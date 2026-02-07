package filters

import (
	"bytes"
	"context"
	"log"
	"mime"
	"net/mail"
	"strings"
)

// ExternalTicketNumberFilter applies custom regex rules to extract GoatFlow ticket numbers.
type ExternalTicketNumberFilter struct {
	logger     *log.Logger
	rules      []ExternalTicketRule
	needBody   bool
	bodyHelper *BodyTokenFilter
}

// NewExternalTicketNumberFilter constructs the filter.
func NewExternalTicketNumberFilter(rules []ExternalTicketRule, logger *log.Logger) *ExternalTicketNumberFilter {
	if len(rules) == 0 {
		return nil
	}
	filter := &ExternalTicketNumberFilter{
		logger:     logger,
		rules:      make([]ExternalTicketRule, len(rules)),
		bodyHelper: NewBodyTokenFilter(logger),
	}
	copy(filter.rules, rules)
	for _, rule := range rules {
		if rule.SearchBody {
			filter.needBody = true
			break
		}
	}
	return filter
}

// ID implements Filter.
func (f *ExternalTicketNumberFilter) ID() string { return "followup_external_ticket" }

// Apply evaluates the configured rules.
func (f *ExternalTicketNumberFilter) Apply(ctx context.Context, m *MessageContext) error {
	if f == nil || len(f.rules) == 0 || m == nil || m.Message == nil || len(m.Message.Raw) == 0 {
		return nil
	}
	if hasFollowUpAnnotation(m) {
		return nil
	}
	headers, subject := f.parseHeaders(m.Message.Raw)
	body := ""
	if f.needBody {
		body = f.bodyHelper.extractBody(m.Message.Raw)
	}
	for _, rule := range f.rules {
		if ticket := f.applyRule(rule, subject, body, headers); ticket != "" {
			if m.Annotations == nil {
				m.Annotations = make(map[string]any)
			}
			m.Annotations[AnnotationFollowUpTicketNumber] = ticket
			f.logf("followup_external_ticket: %s detected ticket %s", rule.Name, ticket)
			break
		}
	}
	return nil
}

func (f *ExternalTicketNumberFilter) applyRule(rule ExternalTicketRule, subject, body string, headers mail.Header) string {
	if rule.SearchSubject {
		if ticket := f.extract(rule, subject); ticket != "" {
			return ticket
		}
	}
	if rule.SearchBody {
		if ticket := f.extract(rule, body); ticket != "" {
			return ticket
		}
	}
	for _, name := range rule.Headers {
		if value := headers.Get(name); value != "" {
			if ticket := f.extract(rule, value); ticket != "" {
				return ticket
			}
		}
	}
	return ""
}

func (f *ExternalTicketNumberFilter) extract(rule ExternalTicketRule, input string) string {
	input = strings.TrimSpace(input)
	if input == "" || rule.Pattern == nil {
		return ""
	}
	matches := rule.Pattern.FindStringSubmatch(input)
	if len(matches) < 2 {
		return ""
	}
	return sanitizeTicketNumber(matches[1])
}

func (f *ExternalTicketNumberFilter) parseHeaders(raw []byte) (mail.Header, string) {
	reader, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		f.logf("followup_external_ticket: header parse failed: %v", err)
		return mail.Header{}, ""
	}
	return reader.Header, decodeSubject(reader.Header)
}

func decodeSubject(header mail.Header) string {
	if header == nil {
		return ""
	}
	value := header.Get("Subject")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var dec mime.WordDecoder
	decoded, err := dec.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

func sanitizeTicketNumber(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "<>[]()\"'")
	if value == "" {
		return ""
	}
	return value
}

func (f *ExternalTicketNumberFilter) logf(format string, args ...any) {
	if f == nil || f.logger == nil {
		return
	}
	f.logger.Printf(format, args...)
}
