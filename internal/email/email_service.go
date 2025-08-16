package email

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

type EmailService struct {
	db               *sql.DB
	smtpHost         string
	smtpPort         string
	smtpUsername     string
	smtpPassword     string
	smtpFrom         string
	useTLS           bool
	emailAccountRepo *repository.EmailAccountRepository
	templateRepo     *repository.EmailTemplateRepository
}

func NewEmailService(db *sql.DB, config EmailConfig) *EmailService {
	return &EmailService{
		db:               db,
		smtpHost:         config.SMTPHost,
		smtpPort:         config.SMTPPort,
		smtpUsername:     config.SMTPUsername,
		smtpPassword:     config.SMTPPassword,
		smtpFrom:         config.SMTPFrom,
		useTLS:           config.UseTLS,
		emailAccountRepo: repository.NewEmailAccountRepository(db),
		templateRepo:     repository.NewEmailTemplateRepository(db),
	}
}

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	UseTLS       bool
}

type EmailMessage struct {
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        string
	ContentType string
	Attachments []EmailAttachment
}

type EmailAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

func (s *EmailService) SendEmail(message *EmailMessage) error {
	if message.ContentType == "" {
		message.ContentType = "text/plain"
	}

	auth := smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)

	var headers bytes.Buffer
	headers.WriteString(fmt.Sprintf("From: %s\r\n", s.smtpFrom))
	headers.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(message.To, ", ")))
	if len(message.CC) > 0 {
		headers.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(message.CC, ", ")))
	}
	headers.WriteString(fmt.Sprintf("Subject: %s\r\n", message.Subject))
	headers.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	headers.WriteString(fmt.Sprintf("Content-Type: %s; charset=UTF-8\r\n", message.ContentType))
	headers.WriteString("\r\n")
	headers.WriteString(message.Body)

	recipients := append(message.To, message.CC...)
	recipients = append(recipients, message.BCC...)

	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)

	if s.useTLS {
		tlsConfig := &tls.Config{
			ServerName: s.smtpHost,
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, s.smtpHost)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer client.Close()

		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}

		if err := client.Mail(s.smtpFrom); err != nil {
			return fmt.Errorf("failed to set sender: %w", err)
		}

		for _, recipient := range recipients {
			if err := client.Rcpt(recipient); err != nil {
				log.Printf("Failed to add recipient %s: %v", recipient, err)
			}
		}

		writer, err := client.Data()
		if err != nil {
			return fmt.Errorf("failed to start data transfer: %w", err)
		}

		if _, err := writer.Write(headers.Bytes()); err != nil {
			return fmt.Errorf("failed to write email data: %w", err)
		}

		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to close data transfer: %w", err)
		}

		return client.Quit()
	}

	return smtp.SendMail(addr, auth, s.smtpFrom, recipients, headers.Bytes())
}

func (s *EmailService) SendTicketNotification(ticket *models.Ticket, notificationType string, recipients []string) error {
	templateName := fmt.Sprintf("ticket_%s", notificationType)
	tmpl, err := s.templateRepo.GetByName(templateName)
	if err != nil {
		log.Printf("Template %s not found, using default", templateName)
		return s.sendDefaultTicketNotification(ticket, notificationType, recipients)
	}

	subject, err := s.processTemplate(tmpl.SubjectTemplate, ticket)
	if err != nil {
		return fmt.Errorf("failed to process subject template: %w", err)
	}

	body, err := s.processTemplate(tmpl.BodyTemplate, ticket)
	if err != nil {
		return fmt.Errorf("failed to process body template: %w", err)
	}

	message := &EmailMessage{
		To:          recipients,
		Subject:     subject,
		Body:        body,
		ContentType: "text/html",
	}

	return s.SendEmail(message)
}

func (s *EmailService) sendDefaultTicketNotification(ticket *models.Ticket, notificationType string, recipients []string) error {
	var subject, body string

	switch notificationType {
	case "created":
		subject = fmt.Sprintf("[Ticket #%s] New ticket created: %s", ticket.TicketNumber, ticket.Title)
		body = fmt.Sprintf("A new ticket has been created.\n\nTicket Number: %s\nTitle: %s\nPriority: %d\nQueue: %d\n",
			ticket.TicketNumber, ticket.Title, ticket.TicketPriorityID, ticket.QueueID)
	case "updated":
		subject = fmt.Sprintf("[Ticket #%s] Ticket updated: %s", ticket.TicketNumber, ticket.Title)
		body = fmt.Sprintf("Ticket #%s has been updated.\n\nTitle: %s\nStatus: %d\nPriority: %d\n",
			ticket.TicketNumber, ticket.Title, ticket.TicketStateID, ticket.TicketPriorityID)
	case "assigned":
		subject = fmt.Sprintf("[Ticket #%s] Ticket assigned to you: %s", ticket.TicketNumber, ticket.Title)
		body = fmt.Sprintf("Ticket #%s has been assigned to you.\n\nTitle: %s\nPriority: %d\n",
			ticket.TicketNumber, ticket.Title, ticket.TicketPriorityID)
	case "closed":
		subject = fmt.Sprintf("[Ticket #%s] Ticket closed: %s", ticket.TicketNumber, ticket.Title)
		body = fmt.Sprintf("Ticket #%s has been closed.\n\nTitle: %s\n", ticket.TicketNumber, ticket.Title)
	default:
		subject = fmt.Sprintf("[Ticket #%s] %s", ticket.TicketNumber, ticket.Title)
		body = fmt.Sprintf("Ticket #%s notification.\n\nTitle: %s\n", ticket.TicketNumber, ticket.Title)
	}

	message := &EmailMessage{
		To:          recipients,
		Subject:     subject,
		Body:        body,
		ContentType: "text/plain",
	}

	return s.SendEmail(message)
}

func (s *EmailService) processTemplate(templateStr string, data interface{}) (string, error) {
	tmpl, err := template.New("email").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (s *EmailService) SendArticleNotification(article *models.Article, ticket *models.Ticket, recipients []string) error {
	subject := fmt.Sprintf("Re: [Ticket #%s] %s", ticket.TicketNumber, ticket.Title)
	
	body := fmt.Sprintf(`
New message in ticket #%s

From: %s
Subject: %s

%s

---
View ticket: [link to ticket]
`, ticket.TicketNumber, "Agent", article.Subject, article.Body)

	message := &EmailMessage{
		To:          recipients,
		Subject:     subject,
		Body:        body,
		ContentType: "text/plain",
	}

	return s.SendEmail(message)
}

func (s *EmailService) ProcessIncomingEmail(from string, subject string, body string, attachments []EmailAttachment) (*models.Ticket, error) {
	ticketNumberPattern := `\[Ticket #(\w+)\]`
	if matches := strings.Contains(subject, "[Ticket #"); matches {
		return s.addArticleToExistingTicket(from, subject, body, attachments)
	}

	return s.createTicketFromEmail(from, subject, body, attachments)
}

func (s *EmailService) createTicketFromEmail(from string, subject string, body string, attachments []EmailAttachment) (*models.Ticket, error) {
	now := time.Now()
	ticket := &models.Ticket{
		TicketNumber:     s.generateTicketNumber(),
		Title:            subject,
		QueueID:          1,
		TicketStateID:    1,
		TicketPriorityID: 3,
		CustomerUserID:   &from,
		CreateTime:       now,
		CreateBy:         1,
		ChangeTime:       now,
		ChangeBy:         1,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	ticketRepo := repository.NewTicketRepository(s.db)
	ticketID, err := ticketRepo.Create(ticket)
	if err != nil {
		return nil, err
	}
	ticket.ID = ticketID

	article := &models.Article{
		TicketID:               ticket.ID,
		ArticleTypeID:          1,
		SenderTypeID:           3,
		CommunicationChannelID: 1,
		IsVisibleForCustomer:   1,
		Subject:                subject,
		Body:                   body,
		BodyType:               "text/plain",
		CreateTime:             now,
		CreateBy:               1,
		ChangeTime:             now,
		ChangeBy:               1,
	}

	articleRepo := repository.NewArticleRepository(s.db)
	articleID, err := articleRepo.Create(article)
	if err != nil {
		return nil, err
	}

	for _, attachment := range attachments {
		att := &models.ArticleAttachment{
			ArticleID:   articleID,
			Filename:    attachment.Filename,
			ContentType: attachment.ContentType,
			ContentSize: len(attachment.Content),
			Content:     string(attachment.Content),
			CreateTime:  now,
			CreateBy:    1,
			ChangeTime:  now,
			ChangeBy:    1,
		}
		if err := articleRepo.CreateAttachment(att); err != nil {
			log.Printf("Failed to create attachment: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return ticket, nil
}

func (s *EmailService) addArticleToExistingTicket(from string, subject string, body string, attachments []EmailAttachment) (*models.Ticket, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (s *EmailService) generateTicketNumber() string {
	return fmt.Sprintf("%d%09d", time.Now().Year(), time.Now().UnixNano()%1000000000)
}

func (s *EmailService) TestConnection() error {
	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)
	
	if s.useTLS {
		tlsConfig := &tls.Config{
			ServerName: s.smtpHost,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		conn.Close()
	} else {
		conn, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		conn.Close()
	}
	
	return nil
}