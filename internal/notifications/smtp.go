package notifications

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/goatkit/goatflow/internal/config"
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

func (s *SMTPProvider) Send(_ context.Context, msg EmailMessage) error {
	if s.cfg == nil || !s.cfg.Enabled {
		return nil
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	recipientsHeader := strings.Join(msg.To, ", ")
	fromHeader := s.cfg.From
	if fromHeader == "" {
		fromHeader = s.cfg.SMTP.User
	}

	var headers []string
	headers = append(headers, fmt.Sprintf("From: %s", fromHeader))
	headers = append(headers, fmt.Sprintf("To: %s", recipientsHeader))
	headers = append(headers, fmt.Sprintf("Subject: %s", msg.Subject))

	if msg.HTML {
		headers = append(headers, "MIME-Version: 1.0")
		headers = append(headers, "Content-Type: text/html; charset=UTF-8")
	} else {
		headers = append(headers, "Content-Type: text/plain; charset=UTF-8")
	}

	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + msg.Body

	client, err := s.dialSMTPClient()
	if err != nil {
		return err
	}
	defer client.Close()

	if err := s.authenticate(client); err != nil {
		return err
	}

	sender := fromHeader
	if sender == "" {
		sender = "noreply@localhost"
	}
	if err := client.Mail(sender); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	for _, to := range msg.To {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", to, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to initiate data transfer: %w", err)
	}
	if _, err := w.Write([]byte(message)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data transfer: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("failed to quit SMTP session: %w", err)
	}

	return nil
}

func (s *SMTPProvider) dialSMTPClient() (*smtp.Client, error) {
	mode := s.cfg.EffectiveTLSMode()
	addr := s.cfg.SMTP.Host + ":" + strconv.Itoa(s.cfg.SMTP.Port)
	tlsConfig := &tls.Config{
		ServerName:         s.cfg.SMTP.Host,
		InsecureSkipVerify: s.cfg.SMTP.SkipVerify,
	}

	switch mode {
	case "smtps":
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect via SMTPS: %w", err)
		}
		client, err := smtp.NewClient(conn, s.cfg.SMTP.Host)
		if err != nil {
			return nil, fmt.Errorf("failed to create SMTP client: %w", err)
		}
		return client, nil
	default:
		client, err := smtp.Dial(addr)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		if mode == "starttls" {
			if err := client.StartTLS(tlsConfig); err != nil {
				client.Close()
				return nil, fmt.Errorf("failed to start TLS: %w", err)
			}
		}
		return client, nil
	}
}

func (s *SMTPProvider) authenticate(client *smtp.Client) error {
	if s.cfg.SMTP.User == "" || s.cfg.SMTP.Password == "" {
		return nil
	}

	authType := strings.ToLower(strings.TrimSpace(s.cfg.SMTP.AuthType))
	var auth smtp.Auth
	switch authType {
	case "", "plain":
		auth = smtp.PlainAuth("", s.cfg.SMTP.User, s.cfg.SMTP.Password, s.cfg.SMTP.Host)
	case "login":
		auth = &loginAuth{username: s.cfg.SMTP.User, password: s.cfg.SMTP.Password}
	default:
		auth = smtp.PlainAuth("", s.cfg.SMTP.User, s.cfg.SMTP.Password, s.cfg.SMTP.Host)
	}

	if auth == nil {
		return nil
	}

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}
	return nil
}

// loginAuth implements SMTP LOGIN authentication.
type loginAuth struct {
	username, password string
}

func (a *loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
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
