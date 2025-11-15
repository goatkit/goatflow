package tasks

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net/smtp"
	"strconv"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/mailqueue"
	"github.com/gotrs-io/gotrs-ce/internal/runner"
)

const (
	// MaxRetries is the maximum number of retry attempts for failed emails
	MaxRetries = 5
	// RetryDelayBase is the base delay for exponential backoff (in minutes)
	RetryDelayBase = 5
)

// EmailQueueTask processes emails from the mail queue
type EmailQueueTask struct {
	repo   *mailqueue.MailQueueRepository
	cfg    *config.EmailConfig
	logger *log.Logger
}

// NewEmailQueueTask creates a new email queue task
func NewEmailQueueTask(db *sql.DB, cfg *config.EmailConfig) runner.Task {
	return &EmailQueueTask{
		repo:   mailqueue.NewMailQueueRepository(db),
		cfg:    cfg,
		logger: log.New(log.Writer(), "[EMAIL-QUEUE] ", log.LstdFlags),
	}
}

// Name returns the task name
func (t *EmailQueueTask) Name() string {
	return "email-queue-processor"
}

// Schedule returns the cron schedule (every 30 seconds)
func (t *EmailQueueTask) Schedule() string {
	return "*/30 * * * * *"
}

// Timeout returns the task timeout (5 minutes)
func (t *EmailQueueTask) Timeout() time.Duration {
	return 5 * time.Minute
}

// Run processes pending emails from the queue
func (t *EmailQueueTask) Run(ctx context.Context) error {
	if !t.cfg.Enabled {
		t.logger.Println("Email notifications disabled, skipping queue processing")
		return nil
	}

	// Get pending emails (limit to 10 per run to avoid overwhelming the SMTP server)
	pendingEmails, err := t.repo.GetPending(ctx, 10)
	if err != nil {
		return fmt.Errorf("failed to get pending emails: %w", err)
	}

	if len(pendingEmails) == 0 {
		t.logger.Println("No pending emails to process")
		return nil
	}

	t.logger.Printf("Processing %d pending emails", len(pendingEmails))

	successCount := 0
	failureCount := 0

	for _, email := range pendingEmails {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := t.processEmail(ctx, email); err != nil {
			failureCount++
			t.logger.Printf("Failed to process email ID %d: %v", email.ID, err)
		} else {
			successCount++
			t.logger.Printf("Successfully sent email ID %d", email.ID)
		}
	}

	t.logger.Printf("Email queue processing complete: %d sent, %d failed", successCount, failureCount)

	// Clean up old failed emails (older than 7 days with max attempts)
	if err := t.cleanupFailedEmails(ctx); err != nil {
		t.logger.Printf("Failed to cleanup failed emails: %v", err)
	}

	return nil
}

// processEmail attempts to send a single email
func (t *EmailQueueTask) processEmail(ctx context.Context, email *mailqueue.MailQueueItem) error {
	// Send the email
	smtpCode, smtpMessage, err := t.sendEmail(ctx, email)

	if err != nil {
		// Calculate next retry time using exponential backoff
		nextDueTime := t.calculateNextRetryTime(email.Attempts + 1)

		// Update attempts and schedule retry
		updateErr := t.repo.UpdateAttempts(ctx, email.ID, smtpCode, smtpMessage, nextDueTime)
		if updateErr != nil {
			return fmt.Errorf("failed to update attempts after send failure: %w", updateErr)
		}

		return fmt.Errorf("failed to send email: %w", err)
	}

	// Email sent successfully, remove from queue
	return t.repo.Delete(ctx, email.ID)
}

// sendEmail sends an email using SMTP
func (t *EmailQueueTask) sendEmail(ctx context.Context, email *mailqueue.MailQueueItem) (*int, *string, error) {
	client, err := dialSMTPClient(t.cfg)
	if err != nil {
		return nil, stringPtr(fmt.Sprintf("Failed to connect to SMTP server: %v", err)), err
	}
	defer client.Close()

	// Authenticate if auth is set
	var auth smtp.Auth
	if t.cfg.SMTP.User != "" && t.cfg.SMTP.Password != "" {
		switch t.cfg.SMTP.AuthType {
		case "plain":
			auth = smtp.PlainAuth("", t.cfg.SMTP.User, t.cfg.SMTP.Password, t.cfg.SMTP.Host)
		case "login":
			auth = &loginAuth{username: t.cfg.SMTP.User, password: t.cfg.SMTP.Password}
		default:
			auth = smtp.PlainAuth("", t.cfg.SMTP.User, t.cfg.SMTP.Password, t.cfg.SMTP.Host)
		}
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return nil, stringPtr(fmt.Sprintf("SMTP authentication failed: %v", err)), err
		}
	}

	// Set the sender
	sender := t.cfg.From
	if email.Sender != nil && *email.Sender != "" {
		sender = *email.Sender
	}
	if err = client.Mail(sender); err != nil {
		return nil, stringPtr(fmt.Sprintf("Failed to set sender: %v", err)), err
	}

	// Set recipient
	if err = client.Rcpt(email.Recipient); err != nil {
		return nil, stringPtr(fmt.Sprintf("Failed to set recipient %s: %v", email.Recipient, err)), err
	}

	// Send the email
	w, err := client.Data()
	if err != nil {
		return nil, stringPtr(fmt.Sprintf("Failed to initiate data transfer: %v", err)), err
	}

	_, err = w.Write(email.RawMessage)
	if err != nil {
		return nil, stringPtr(fmt.Sprintf("Failed to write message: %v", err)), err
	}

	err = w.Close()
	if err != nil {
		return nil, stringPtr(fmt.Sprintf("Failed to close data transfer: %v", err)), err
	}

	// Send QUIT
	err = client.Quit()
	if err != nil {
		return nil, stringPtr(fmt.Sprintf("Failed to quit SMTP session: %v", err)), err
	}

	return nil, nil, nil // Success
}

// calculateNextRetryTime calculates the next retry time using exponential backoff
func (t *EmailQueueTask) calculateNextRetryTime(attempts int) *time.Time {
	if attempts >= MaxRetries {
		// Don't schedule further retries
		return nil
	}

	// Exponential backoff: 5min, 25min, 125min, 625min, 3125min (about 2 days)
	delay := time.Duration(RetryDelayBase) * time.Minute
	for i := 1; i < attempts; i++ {
		delay *= 5
	}

	nextTime := time.Now().Add(delay)
	return &nextTime
}

// cleanupFailedEmails removes old failed emails from the queue
func (t *EmailQueueTask) cleanupFailedEmails(ctx context.Context) error {
	failedEmails, err := t.repo.GetFailed(ctx, MaxRetries, 100)
	if err != nil {
		return fmt.Errorf("failed to get failed emails: %w", err)
	}

	if len(failedEmails) == 0 {
		return nil
	}

	t.logger.Printf("Cleaning up %d failed emails", len(failedEmails))

	for _, email := range failedEmails {
		// Only delete emails that are older than 7 days
		if time.Since(email.CreateTime) > 7*24*time.Hour {
			if err := t.repo.Delete(ctx, email.ID); err != nil {
				t.logger.Printf("Failed to delete old failed email ID %d: %v", email.ID, err)
			}
		}
	}

	return nil
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}

func dialSMTPClient(cfg *config.EmailConfig) (*smtp.Client, error) {
	mode := cfg.EffectiveTLSMode()
	addr := cfg.SMTP.Host + ":" + strconv.Itoa(cfg.SMTP.Port)
	tlsConfig := &tls.Config{
		ServerName:         cfg.SMTP.Host,
		InsecureSkipVerify: cfg.SMTP.SkipVerify,
	}

	switch mode {
	case "smtps":
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		client, err := smtp.NewClient(conn, cfg.SMTP.Host)
		if err != nil {
			return nil, err
		}
		return client, nil
	default:
		client, err := smtp.Dial(addr)
		if err != nil {
			return nil, err
		}
		if mode == "starttls" {
			if err := client.StartTLS(tlsConfig); err != nil {
				client.Close()
				return nil, err
			}
		}
		return client, nil
	}
}

// loginAuth implements SMTP LOGIN authentication
type loginAuth struct {
	username, password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, fmt.Errorf("unexpected server challenge: %s", fromServer)
		}
	}
	return nil, nil
}