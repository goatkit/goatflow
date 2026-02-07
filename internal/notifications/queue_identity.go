package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"net/mail"
	"strings"

	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/database"
)

const (
	defaultSenderEmail  = "support@goatflow.local"
	defaultSenderDomain = "goatflow.local"
)

// Snippet describes a reusable text block such as a salutation or signature.
type Snippet struct {
	Text        string
	ContentType string
}

// QueueIdentity captures outbound email metadata tied to a queue.
type QueueIdentity struct {
	QueueID               int
	Email                 string
	DisplayName           string
	SalutationText        string
	SalutationContentType string
	SignatureText         string
	SignatureContentType  string
}

// ResolveQueueIdentity loads queue-linked sender, salutation, and signature data.
func ResolveQueueIdentity(ctx context.Context, db *sql.DB, queueID int) (*QueueIdentity, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if queueID <= 0 {
		return nil, fmt.Errorf("queue id must be positive")
	}

	query := `
		SELECT
			q.id,
			sa.value0,
			sa.value1,
			sal.text,
			sal.content_type,
			sig.text,
			sig.content_type
		FROM queue q
		LEFT JOIN system_address sa ON sa.id = q.system_address_id
		LEFT JOIN salutation sal ON sal.id = q.salutation_id
		LEFT JOIN signature sig ON sig.id = q.signature_id
		WHERE q.id = ?
	`

	row := db.QueryRowContext(ctx, database.ConvertPlaceholders(query), queueID)
	var (
		identity                              QueueIdentity
		email, displayName                    sql.NullString
		salutationText, salutationContentType sql.NullString
		signatureText, signatureContentType   sql.NullString
	)
	if err := row.Scan(
		&identity.QueueID,
		&email,
		&displayName,
		&salutationText,
		&salutationContentType,
		&signatureText,
		&signatureContentType,
	); err != nil {
		return nil, err
	}

	if email.Valid {
		identity.Email = strings.TrimSpace(email.String)
	}
	if displayName.Valid {
		identity.DisplayName = strings.TrimSpace(displayName.String)
	}
	if salutationText.Valid {
		identity.SalutationText = strings.TrimSpace(salutationText.String)
	}
	if salutationContentType.Valid {
		identity.SalutationContentType = strings.TrimSpace(salutationContentType.String)
	}
	if signatureText.Valid {
		identity.SignatureText = strings.TrimSpace(signatureText.String)
	}
	if signatureContentType.Valid {
		identity.SignatureContentType = strings.TrimSpace(signatureContentType.String)
	}

	return &identity, nil
}

// SalutationSnippet exposes the queue salutation as a snippet when present.
func (qi *QueueIdentity) SalutationSnippet() *Snippet {
	if qi == nil || strings.TrimSpace(qi.SalutationText) == "" {
		return nil
	}
	return &Snippet{
		Text:        qi.SalutationText,
		ContentType: normalizeContentType(qi.SalutationContentType),
	}
}

// SignatureSnippet exposes the queue signature as a snippet when present.
func (qi *QueueIdentity) SignatureSnippet() *Snippet {
	if qi == nil || strings.TrimSpace(qi.SignatureText) == "" {
		return nil
	}
	return &Snippet{
		Text:        qi.SignatureText,
		ContentType: normalizeContentType(qi.SignatureContentType),
	}
}

// DefaultFallbacks returns sane defaults derived from the email config.
func DefaultFallbacks(cfg *config.EmailConfig) (string, string) {
	envelope := defaultSenderEmail
	header := defaultSenderEmail
	if cfg != nil && strings.TrimSpace(cfg.From) != "" {
		envelope = strings.TrimSpace(cfg.From)
		header = envelope
		if strings.TrimSpace(cfg.FromName) != "" {
			header = (&mail.Address{Name: strings.TrimSpace(cfg.FromName), Address: envelope}).String()
		}
	}
	return sanitizeAddress(envelope), strings.TrimSpace(header)
}

// EnvelopeAddress picks the SMTP MAIL FROM value with fallback.
func EnvelopeAddress(identity *QueueIdentity, fallback string) string {
	if identity != nil && strings.TrimSpace(identity.Email) != "" {
		return strings.TrimSpace(identity.Email)
	}
	return sanitizeAddress(fallback)
}

// HeaderAddress builds the From header with display name when present.
func HeaderAddress(identity *QueueIdentity, fallback string) string {
	if identity != nil && strings.TrimSpace(identity.Email) != "" {
		if strings.TrimSpace(identity.DisplayName) != "" {
			return (&mail.Address{Name: identity.DisplayName, Address: identity.Email}).String()
		}
		return strings.TrimSpace(identity.Email)
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return defaultSenderEmail
}

// DomainFromAddress extracts the domain portion of an email address.
func DomainFromAddress(address string) string {
	addr := sanitizeAddress(address)
	parts := strings.Split(addr, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func normalizeContentType(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "text/plain"
	}
	return trimmed
}

func sanitizeAddress(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultSenderEmail
	}
	if parsed, err := mail.ParseAddress(trimmed); err == nil && parsed.Address != "" {
		return strings.TrimSpace(parsed.Address)
	}
	if start := strings.Index(trimmed, "<"); start >= 0 {
		if end := strings.Index(trimmed, ">"); end > start {
			email := strings.TrimSpace(trimmed[start+1 : end])
			if email != "" {
				return email
			}
		}
	}
	return trimmed
}
