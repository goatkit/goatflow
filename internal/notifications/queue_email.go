package notifications

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/config"
)

// EmailBranding bundles outbound email metadata for queue-based messages.
type EmailBranding struct {
	EnvelopeFrom string
	HeaderFrom   string
	Body         string
	Domain       string
}

// PrepareQueueEmail injects queue identity details into the outgoing body and headers.
func PrepareQueueEmail(ctx context.Context, db *sql.DB, queueID int, baseBody string, baseIsHTML bool, cfg *config.EmailConfig, renderCtx *RenderContext) (*EmailBranding, error) {
	fallbackEnvelope, fallbackHeader := DefaultFallbacks(cfg)
	var (
		identity *QueueIdentity
		err      error
	)
	if db != nil && queueID > 0 {
		identity, err = ResolveQueueIdentity(ctx, db, queueID)
	}

	finalBody := strings.TrimSpace(applyPlaceholders(baseBody, renderCtx))
	if identity != nil {
		finalBody = ApplyBranding(baseBody, baseIsHTML, identity, renderCtx)
	}

	envelope := EnvelopeAddress(identity, fallbackEnvelope)
	header := HeaderAddress(identity, fallbackHeader)
	domain := DomainFromAddress(envelope)
	if strings.TrimSpace(domain) == "" {
		domain = defaultSenderDomain
	}

	branding := &EmailBranding{
		EnvelopeFrom: envelope,
		HeaderFrom:   header,
		Body:         finalBody,
		Domain:       domain,
	}

	return branding, err
}
