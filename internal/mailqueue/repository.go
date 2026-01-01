// Package mailqueue provides email queue storage and management.
package mailqueue

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// MailQueueItem represents an email in the queue.
type MailQueueItem struct {
	ID                int64
	InsertFingerprint *string
	ArticleID         *int64
	Attempts          int
	Sender            *string
	Recipient         string
	RawMessage        []byte
	DueTime           *time.Time
	LastSMTPCode      *int
	LastSMTPMessage   *string
	CreateTime        time.Time
}

// MailQueueRepository handles database operations for the mail queue.
type MailQueueRepository struct {
	db *sql.DB
}

// NewMailQueueRepository creates a new mail queue repository.
func NewMailQueueRepository(db *sql.DB) *MailQueueRepository {
	return &MailQueueRepository{db: db}
}

// Insert adds a new email to the queue.
func (r *MailQueueRepository) Insert(ctx context.Context, item *MailQueueItem) error {
	query := `
		INSERT INTO mail_queue (
			insert_fingerprint, article_id, attempts, sender, recipient,
			raw_message, due_time, create_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, NOW())
	`

	_, err := r.db.ExecContext(ctx, query,
		item.InsertFingerprint,
		item.ArticleID,
		item.Attempts,
		item.Sender,
		item.Recipient,
		item.RawMessage,
		item.DueTime,
	)

	if err != nil {
		// Handle duplicate key errors (article_id or insert_fingerprint already exists)
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			return fmt.Errorf("email already queued: %w", err)
		}
		return fmt.Errorf("failed to insert mail queue item: %w", err)
	}

	return nil
}

// GetPending retrieves emails that are ready to be sent (due_time is null or past).
func (r *MailQueueRepository) GetPending(ctx context.Context, limit int) ([]*MailQueueItem, error) {
	query := `
		SELECT id, insert_fingerprint, article_id, attempts, sender, recipient,
			   raw_message, due_time, last_smtp_code, last_smtp_message, create_time
		FROM mail_queue
		WHERE (due_time IS NULL OR due_time <= NOW())
		ORDER BY create_time ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending emails: %w", err)
	}
	defer rows.Close()

	var items []*MailQueueItem
	for rows.Next() {
		var item MailQueueItem
		err := rows.Scan(
			&item.ID,
			&item.InsertFingerprint,
			&item.ArticleID,
			&item.Attempts,
			&item.Sender,
			&item.Recipient,
			&item.RawMessage,
			&item.DueTime,
			&item.LastSMTPCode,
			&item.LastSMTPMessage,
			&item.CreateTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mail queue item: %w", err)
		}
		items = append(items, &item)
	}

	return items, rows.Err()
}

// UpdateAttempts increments the attempt count and sets the next due time.
func (r *MailQueueRepository) UpdateAttempts(ctx context.Context, id int64, smtpCode *int, smtpMessage *string, nextDueTime *time.Time) error {
	query := `
		UPDATE mail_queue
		SET attempts = attempts + 1,
			last_smtp_code = ?,
			last_smtp_message = ?,
			due_time = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query, smtpCode, smtpMessage, nextDueTime, id)
	if err != nil {
		return fmt.Errorf("failed to update mail queue attempts: %w", err)
	}

	return nil
}

// Delete removes a successfully sent email from the queue.
func (r *MailQueueRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM mail_queue WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete mail queue item: %w", err)
	}

	return nil
}

// GetFailed retrieves emails that have exceeded max attempts.
func (r *MailQueueRepository) GetFailed(ctx context.Context, maxAttempts int, limit int) ([]*MailQueueItem, error) {
	query := `
		SELECT id, insert_fingerprint, article_id, attempts, sender, recipient,
			   raw_message, due_time, last_smtp_code, last_smtp_message, create_time
		FROM mail_queue
		WHERE attempts >= ?
		ORDER BY create_time ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, maxAttempts, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query failed emails: %w", err)
	}
	defer rows.Close()

	var items []*MailQueueItem
	for rows.Next() {
		var item MailQueueItem
		err := rows.Scan(
			&item.ID,
			&item.InsertFingerprint,
			&item.ArticleID,
			&item.Attempts,
			&item.Sender,
			&item.Recipient,
			&item.RawMessage,
			&item.DueTime,
			&item.LastSMTPCode,
			&item.LastSMTPMessage,
			&item.CreateTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan failed mail queue item: %w", err)
		}
		items = append(items, &item)
	}

	return items, rows.Err()
}

// BuildHTMLEmailMessage constructs a properly formatted HTML email message.
func BuildHTMLEmailMessage(from, to, subject, htmlBody string) []byte {
	return BuildEmailMessageWithHeaders(from, to, subject, htmlBody, map[string]string{
		"Content-Type": "text/html; charset=UTF-8",
	})
}

// BuildTextEmailMessage constructs a properly formatted plain text email message.
func BuildTextEmailMessage(from, to, subject, textBody string) []byte {
	return BuildEmailMessageWithHeaders(from, to, subject, textBody, map[string]string{
		"Content-Type": "text/plain; charset=UTF-8",
	})
}

// BuildEmailMessageWithHeaders builds an email message with optional headers for threading.
func BuildEmailMessageWithHeaders(from, to, subject, body string, headers map[string]string) []byte {
	var headerLines []string

	// Add standard headers
	headerLines = append(headerLines, fmt.Sprintf("From: %s", from))
	headerLines = append(headerLines, fmt.Sprintf("To: %s", to))
	headerLines = append(headerLines, fmt.Sprintf("Subject: %s", subject))

	// Add custom headers
	for key, value := range headers {
		headerLines = append(headerLines, fmt.Sprintf("%s: %s", key, value))
	}

	// Determine content type
	var contentType string
	if containsHTML(body) {
		contentType = "text/html; charset=UTF-8"
	} else {
		contentType = "text/plain; charset=UTF-8"
	}
	headerLines = append(headerLines, fmt.Sprintf("Content-Type: %s", contentType))

	// Build the complete message
	message := strings.Join(headerLines, "\r\n") + "\r\n\r\n" + body
	return []byte(message)
}

// BuildEmailMessageWithThreading builds an email message with threading headers.
func BuildEmailMessageWithThreading(from, to, subject, body, domain string, inReplyTo, references string) []byte {
	headers := make(map[string]string)

	// Generate Message-ID
	messageID := GenerateMessageID(domain)
	headers["Message-ID"] = messageID

	// Add threading headers if provided
	if inReplyTo != "" {
		headers["In-Reply-To"] = inReplyTo
	}
	if references != "" {
		headers["References"] = references
	}

	return BuildEmailMessageWithHeaders(from, to, subject, body, headers)
}

// BuildEmailMessage automatically detects content type and builds appropriate email message.
func BuildEmailMessage(from, to, subject, body string) []byte {
	return BuildEmailMessageWithHeaders(from, to, subject, body, nil)
}

// containsHTML checks if the content contains HTML tags.
func containsHTML(content string) bool {
	htmlTags := []string{"<p>", "<br", "<div>", "<span>", "<strong>", "<em>", "<b>", "<i>", "<h1>", "<h2>", "<h3>", "<ul>", "<ol>", "<li>", "<table>", "<a ", "<blockquote>", "<img "}
	for _, tag := range htmlTags {
		if strings.Contains(content, tag) {
			return true
		}
	}
	return false
}

// isMarkdownContent checks if the content appears to be markdown.
func isMarkdownContent(content string) bool {
	markdownPatterns := []string{"**", "*", "_", "`", "# ", "## ", "### ", "- ", "* ", "+ ", "1. ", "[", "](", "![", "](", "\n"}
	markdownCount := 0
	for _, pattern := range markdownPatterns {
		if strings.Contains(content, pattern) {
			markdownCount++
		}
	}
	// If we have multiple markdown patterns, it's likely rich text
	return markdownCount >= 2
}

// markdownToHTML converts markdown content to HTML.
func markdownToHTML(markdown string) string {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML in markdown
		),
	)

	var buf strings.Builder
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		// If conversion fails, return original content
		return markdown
	}

	return buf.String()
}

// GenerateMessageID creates a unique Message-ID header for email threading.
func GenerateMessageID(domain string) string {
	// Generate a random component
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)

	// Use timestamp for uniqueness
	timestamp := time.Now().Unix()

	return fmt.Sprintf("<%d.%s@%s>", timestamp, randomHex, domain)
}

// ExtractMessageIDFromRawMessage extracts the Message-ID header from a raw email message.
func ExtractMessageIDFromRawMessage(rawMessage []byte) string {
	message := string(rawMessage)
	lines := strings.Split(message, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "message-id:") {
			// Extract the Message-ID value
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				messageID := strings.TrimSpace(parts[1])
				// Remove angle brackets if present
				messageID = strings.Trim(messageID, "<>")
				return messageID
			}
		}
		// Stop at the first empty line (end of headers)
		if line == "" {
			break
		}
	}

	return ""
}
