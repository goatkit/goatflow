package notificationevents

import (
	"context"
	"database/sql"
	"fmt"
	"mime"
	"net/mail"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/mailqueue"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/i18n"
	"github.com/goatkit/goatflow/internal/platform/notifications"
	"github.com/goatkit/goatflow/internal/repository"
)

// defaultLanguage is the system default language (fallback for recipients
// without a language preference).
func defaultLanguage() string {
	if lang := i18n.GetInstance().GetDefaultLanguage(); lang != "" {
		return lang
	}
	return "en"
}

// queueMail puts the notification on mail_queue inside tx, from the system
// sender (email.from / email.from_name, OTRS NotificationSenderEmail).
func queueMail(ctx context.Context, tx *sql.Tx, rc recipient, subject, body string, isHTML bool) error {
	var emailCfg *config.EmailConfig
	if cfg := config.Get(); cfg != nil {
		emailCfg = &cfg.Email
	}
	envelope, from := notifications.DefaultFallbacks(emailCfg)
	to := (&mail.Address{Name: rc.fullName(), Address: rc.email}).String()
	contentType := "text/plain; charset=UTF-8"
	if isHTML {
		contentType = "text/html; charset=UTF-8"
	}
	// The subject is rendered from ticket data that customers control: fold
	// line breaks so it cannot add headers.
	subject = strings.Join(strings.Fields(subject), " ")
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"Message-ID: " + mailqueue.GenerateMessageID(notifications.DomainFromAddress(envelope)),
		"MIME-Version: 1.0",
		"Content-Type: " + contentType,
		"Content-Transfer-Encoding: 8bit",
		"Auto-Submitted: auto-generated",
	}
	raw := strings.Join(headers, "\r\n") + "\r\n\r\n" + body
	return mailqueue.InsertTx(ctx, tx, &mailqueue.MailQueueItem{
		Sender:     &envelope,
		Recipient:  rc.email,
		RawMessage: []byte(raw),
	})
}

// addHistory writes the OTRS history row of a sent notification inside tx.
func addHistory(ctx context.Context, db *sql.DB, tx *sql.Tx, t *ticketData, historyType, name string) error {
	err := repository.NewTicketRepository(db).AddTicketHistoryEntry(ctx, tx, models.TicketHistoryInsert{
		TicketID:    int(t.id),
		TypeID:      t.typeID,
		QueueID:     t.queueID,
		OwnerID:     t.ownerID,
		PriorityID:  t.priorityID,
		StateID:     t.stateID,
		CreatedBy:   1,
		HistoryType: historyType,
		Name:        name,
	})
	if err != nil {
		return fmt.Errorf("history %s: %w", historyType, err)
	}
	return nil
}
