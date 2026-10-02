package selfservice

import (
	"context"
	"database/sql"
	"fmt"
	"mime"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/mailqueue"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/i18n"
	"github.com/goatkit/goatflow/internal/platform/notifications"
)

// EnvBaseURL names the environment variable holding the public URL of this
// instance (scheme://host[:port][/prefix]). Links in self-service emails are
// built from it; the request Host header is attacker-controlled and would let
// anyone send a victim a reset link pointing at their own server.
const EnvBaseURL = "BASE_URL"

// publicBaseURL returns BASE_URL without a trailing slash, or an error when it
// is unset or not an absolute http(s) URL.
func publicBaseURL() (string, error) {
	raw := strings.TrimSpace(os.Getenv(EnvBaseURL))
	if raw == "" {
		return "", fmt.Errorf("%s is not set; self-service emails need the public URL of this instance", EnvBaseURL)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%s=%q is not an absolute http(s) URL", EnvBaseURL, raw)
	}
	return strings.TrimRight(raw, "/"), nil
}

// tokenLink builds the absolute link for path?token=raw.
func tokenLink(path, raw string) (string, error) {
	base, err := publicBaseURL()
	if err != nil {
		return "", err
	}
	return base + path + "?token=" + url.QueryEscape(raw), nil
}

// translate renders an i18n key in lang.
func translate(lang, key string, args ...any) string {
	return i18n.GetInstance().T(lang, key, args...)
}

// displayName is how emails greet an account holder.
func displayName(first, last, fallback string) string {
	if name := strings.TrimSpace(first + " " + last); name != "" {
		return name
	}
	return fallback
}

// queueMail puts a plain-text system email on mail_queue, from the configured
// system sender (email.from / email.from_name), for the email runner to send.
func queueMail(ctx context.Context, db *sql.DB, to, subject, body string) error {
	var emailCfg *config.EmailConfig
	if cfg := config.Get(); cfg != nil {
		emailCfg = &cfg.Email
	}
	envelope, header := notifications.DefaultFallbacks(emailCfg)
	raw := mailqueue.BuildEmailMessageWithHeaders(header, to, mime.QEncoding.Encode("utf-8", subject), body, map[string]string{
		"MIME-Version":              "1.0",
		"Content-Transfer-Encoding": "8bit",
		"Date":                      time.Now().Format(time.RFC1123Z),
		"Message-ID":                mailqueue.GenerateMessageID(notifications.DomainFromAddress(envelope)),
		"Auto-Submitted":            "auto-generated",
	})
	return mailqueue.NewMailQueueRepository(db).Insert(ctx, &mailqueue.MailQueueItem{
		Sender:     &envelope,
		Recipient:  to,
		RawMessage: raw,
		CreateTime: time.Now(),
	})
}
