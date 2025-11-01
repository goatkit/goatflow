package notifications

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strconv"

	"github.com/gotrs-io/gotrs-ce/internal/config"
)

type EmailMessage struct {
	To      []string
	Subject string
	Body    string
	HTML    bool
}

type EmailProvider interface {
	Send(ctx context.Context, msg EmailMessage) error
}

type SMTPProvider struct {
	cfg *config.EmailConfig
}

func NewSMTPProvider(cfg *config.EmailConfig) EmailProvider {
	return &SMTPProvider{cfg: cfg}
}

func (s *SMTPProvider) Send(ctx context.Context, msg EmailMessage) error {
	if !s.cfg.Enabled {
		return nil // Silently skip if email is disabled
	}

	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	// Build the message
	var message string
	if msg.HTML {
		message = fmt.Sprintf("To: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
			msg.To[0], msg.Subject, msg.Body)
	} else {
		message = fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", msg.To[0], msg.Subject, msg.Body)
	}

	// Set up authentication
	var auth smtp.Auth
	if s.cfg.SMTP.User != "" && s.cfg.SMTP.Password != "" {
		switch s.cfg.SMTP.AuthType {
		case "plain":
			auth = smtp.PlainAuth("", s.cfg.SMTP.User, s.cfg.SMTP.Password, s.cfg.SMTP.Host)
		case "login":
			auth = &loginAuth{username: s.cfg.SMTP.User, password: s.cfg.SMTP.Password}
		default:
			auth = smtp.PlainAuth("", s.cfg.SMTP.User, s.cfg.SMTP.Password, s.cfg.SMTP.Host)
		}
	}

	// Set up TLS config
	tlsConfig := &tls.Config{
		ServerName:         s.cfg.SMTP.Host,
		InsecureSkipVerify: s.cfg.SMTP.SkipVerify,
	}

	// Create SMTP client
	addr := s.cfg.SMTP.Host + ":" + strconv.Itoa(s.cfg.SMTP.Port)
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Start TLS if required
	if s.cfg.SMTP.TLS {
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Authenticate if auth is set
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set the sender
	if err = client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, to := range msg.To {
		if err = client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", to, err)
		}
	}

	// Send the email
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to initiate data transfer: %w", err)
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data transfer: %w", err)
	}

	// Send QUIT
	err = client.Quit()
	if err != nil {
		return fmt.Errorf("failed to quit SMTP session: %w", err)
	}

	return nil
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