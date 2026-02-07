package filters

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
)

func TestAttachmentTokenFilterDetectsFilenameToken(t *testing.T) {
	filter := NewAttachmentTokenFilter(nil)
	raw := buildAttachmentMessage(
		"text/plain; name=\"summary-[Ticket#20257001].txt\"",
		"attachment; filename=\"summary-[Ticket#20257001].txt\"",
		"",
		"Attachment body",
	)
	ctx := &MessageContext{Message: &connector.FetchedMessage{Raw: []byte(raw)}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "20257001" {
		t.Fatalf("expected ticket 20257001, got %+v", ctx.Annotations)
	}
}

func TestAttachmentTokenFilterUsesContentDescription(t *testing.T) {
	filter := NewAttachmentTokenFilter(nil)
	ctx := &MessageContext{Message: &connector.FetchedMessage{Raw: []byte(strings.Join([]string{
		"Subject: Example",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=xyz",
		"",
		"--xyz",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Body",
		"--xyz",
		"Content-Type: application/octet-stream",
		"Content-Transfer-Encoding: base64",
		"Content-Description: Refs [Ticket#777]",
		"Content-Disposition: attachment; filename=ref.txt",
		"",
		base64.StdEncoding.EncodeToString([]byte("content")),
		"--xyz--",
	}, "\r\n"))}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations[AnnotationFollowUpTicketNumber] != "777" {
		t.Fatalf("expected ticket 777, got %+v", ctx.Annotations)
	}
}

func TestAttachmentTokenFilterSkipsWhenNoMatches(t *testing.T) {
	filter := NewAttachmentTokenFilter(nil)
	raw := buildAttachmentMessage(
		"text/plain; name=\"notes.txt\"",
		"attachment; filename=\"notes.txt\"",
		"",
		"No tokens",
	)
	ctx := &MessageContext{Message: &connector.FetchedMessage{Raw: []byte(raw)}}
	if err := filter.Apply(context.Background(), ctx); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ctx.Annotations != nil {
		t.Fatalf("expected no annotations, got %+v", ctx.Annotations)
	}
}

func buildAttachmentMessage(contentType, disposition, extraHeader, body string) string {
	boundary := "mix-" + base64.StdEncoding.EncodeToString([]byte(body))[:6]
	attachment := base64.StdEncoding.EncodeToString([]byte(body))
	sections := []string{
		"Subject: Attachment",
		"MIME-Version: 1.0",
		fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s", boundary),
		"",
		fmt.Sprintf("--%s", boundary),
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Body",
		fmt.Sprintf("--%s", boundary),
		fmt.Sprintf("Content-Type: %s", contentType),
		"Content-Transfer-Encoding: base64",
		fmt.Sprintf("Content-Disposition: %s", disposition),
	}
	if strings.TrimSpace(extraHeader) != "" {
		sections = append(sections, extraHeader)
	}
	sections = append(sections,
		"",
		attachment,
		fmt.Sprintf("--%s--", boundary),
	)
	return strings.Join(sections, "\r\n")
}
