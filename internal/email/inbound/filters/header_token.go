package filters

import (
	"bytes"
	"context"
	"log"
	"net/mail"
	"strings"
)

var headerTokenKeys = []string{
	"X-GoatFlow-TicketNumber",
	"X-GoatFlow-ExternalTicketNumber",
	"X-OTRS-TicketNumber",
	"X-OTRS-FollowUp-TicketNumber",
	"X-Znuny-TicketNumber",
}

// HeaderTokenFilter inspects whitelisted headers for ticket numbers.
type HeaderTokenFilter struct {
	logger *log.Logger
}

// NewHeaderTokenFilter returns a header-based follow-up detector.
func NewHeaderTokenFilter(logger *log.Logger) *HeaderTokenFilter {
	return &HeaderTokenFilter{logger: logger}
}

// ID implements Filter.
func (f *HeaderTokenFilter) ID() string { return "followup_header_token" }

// Apply extracts ticket numbers from known headers.
func (f *HeaderTokenFilter) Apply(ctx context.Context, m *MessageContext) error {
	if m == nil || m.Message == nil || len(m.Message.Raw) == 0 {
		return nil
	}
	if hasFollowUpAnnotation(m) {
		return nil
	}
	reader, err := mail.ReadMessage(bytes.NewReader(m.Message.Raw))
	if err != nil {
		f.logf("followup_header_token: parse failed: %v", err)
		return nil
	}
	for _, key := range headerTokenKeys {
		value := strings.TrimSpace(reader.Header.Get(key))
		if value == "" {
			continue
		}
		if token := parseHeaderTicketNumber(value); token != "" {
			if m.Annotations == nil {
				m.Annotations = make(map[string]any)
			}
			m.Annotations[AnnotationFollowUpTicketNumber] = token
			f.logf("followup_header_token: detected ticket %s via %s", token, key)
			break
		}
	}
	return nil
}

func parseHeaderTicketNumber(value string) string {
	if token := findTicketToken(value); token != "" {
		return token
	}
	clean := strings.TrimSpace(value)
	clean = strings.Trim(clean, "<>")
	clean = strings.Trim(clean, "[]")
	clean = strings.TrimPrefix(strings.ToLower(clean), "ticket#")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return ""
	}
	if isDigits(clean) {
		return clean
	}
	for i := len(clean); i > 0; i-- {
		candidate := clean[:i]
		candidate = strings.TrimSpace(candidate)
		if isDigits(candidate) {
			return candidate
		}
	}
	return ""
}

func isDigits(input string) bool {
	if input == "" {
		return false
	}
	for _, r := range input {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (f *HeaderTokenFilter) logf(format string, args ...any) {
	if f == nil || f.logger == nil {
		return
	}
	f.logger.Printf(format, args...)
}
