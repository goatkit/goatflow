package filters

import (
	"bytes"
	"context"
	"log"
	"mime"
	"net/mail"
	"strings"
)

// SubjectTokenFilter extracts OTRS-style "[Ticket#123]" tokens from the Subject header.
type SubjectTokenFilter struct {
	logger  *log.Logger
	decoder *mime.WordDecoder
}

// NewSubjectTokenFilter constructs the filter instance.
func NewSubjectTokenFilter(logger *log.Logger) *SubjectTokenFilter {
	return &SubjectTokenFilter{
		logger:  logger,
		decoder: &mime.WordDecoder{},
	}
}

// ID implements Filter.
func (f *SubjectTokenFilter) ID() string { return "followup_subject_token" }

// Apply scans the subject for OTRS-style ticket tokens and stores the ticket number annotation.
func (f *SubjectTokenFilter) Apply(ctx context.Context, m *MessageContext) error {
	if m == nil || m.Message == nil || len(m.Message.Raw) == 0 {
		return nil
	}
	subject := f.extractSubject(m.Message.Raw)
	if subject == "" {
		return nil
	}
	token := findTicketToken(subject)
	if token == "" {
		return nil
	}
	if m.Annotations == nil {
		m.Annotations = make(map[string]any)
	}
	m.Annotations[AnnotationFollowUpTicketNumber] = token
	f.logf("followup_subject_token: detected ticket %s", token)
	return nil
}

func (f *SubjectTokenFilter) extractSubject(raw []byte) string {
	reader, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		f.logf("followup_subject_token: parse failed: %v", err)
		return ""
	}
	header := strings.TrimSpace(reader.Header.Get("Subject"))
	if header == "" {
		return ""
	}
	decoded, err := f.decoder.DecodeHeader(header)
	if err != nil {
		return header
	}
	return decoded
}

func (f *SubjectTokenFilter) logf(format string, args ...any) {
	if f == nil || f.logger == nil {
		return
	}
	f.logger.Printf(format, args...)
}
