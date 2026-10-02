package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/storage"
)

// SimpleTicketService provides ticket and article (message) access backed by the database.
type SimpleTicketService struct {
	ticketRepo repository.ITicketRepository
	db         *sql.DB
}

// NewSimpleTicketService creates a new simple ticket service.
func NewSimpleTicketService(repo repository.ITicketRepository, db *sql.DB) *SimpleTicketService {
	return &SimpleTicketService{
		ticketRepo: repo,
		db:         db,
	}
}

var errNoTicketStore = errors.New("ticket service: database not available")

// CreateTicket creates a new ticket.
func (s *SimpleTicketService) CreateTicket(ticket *models.Ticket) error {
	if ticket == nil {
		return fmt.Errorf("ticket cannot be nil")
	}

	// Validate required fields
	if ticket.Title == "" {
		return fmt.Errorf("ticket title is required")
	}

	// Create the ticket
	return s.ticketRepo.Create(ticket)
}

// GetTicket retrieves a ticket by ID.
func (s *SimpleTicketService) GetTicket(ticketID uint) (*models.Ticket, error) {
	if s.ticketRepo == nil {
		return nil, errNoTicketStore
	}
	return s.ticketRepo.GetByID(ticketID)
}

// UpdateTicket updates an existing ticket.
func (s *SimpleTicketService) UpdateTicket(ticket *models.Ticket) error {
	if ticket == nil {
		return fmt.Errorf("ticket cannot be nil")
	}

	return s.ticketRepo.Update(ticket)
}

// DeleteTicket deletes a ticket.
func (s *SimpleTicketService) DeleteTicket(ticketID uint) error {
	return s.ticketRepo.Delete(ticketID)
}

// ListTickets returns a paginated list of tickets.
func (s *SimpleTicketService) ListTickets(req *models.TicketListRequest) (*models.TicketListResponse, error) {
	if req == nil {
		req = &models.TicketListRequest{
			Page:    1,
			PerPage: 20,
		}
	}

	return s.ticketRepo.List(req)
}

// SimpleTicketMessage is a simplified message model.
type SimpleTicketMessage struct {
	ID           uint                `json:"id"`
	TicketID     uint                `json:"ticket_id"`
	Body         string              `json:"body"`
	Subject      string              `json:"subject"`
	ContentType  string              `json:"content_type"`
	CreatedBy    uint                `json:"created_by"`
	AuthorName   string              `json:"author_name"`
	AuthorEmail  string              `json:"author_email"`
	AuthorType   string              `json:"author_type"` // "Customer", "Agent", "System"
	SenderTypeID int                 `json:"sender_type_id"`
	SenderColor  string              `json:"sender_color,omitempty"` // Hex color from article_color
	IsPublic     bool                `json:"is_public"`
	IsInternal   bool                `json:"is_internal"`
	CreatedAt    time.Time           `json:"created_at"`
	Attachments  []*SimpleAttachment `json:"attachments,omitempty"`
}

// SimpleAttachment is a simplified attachment model.
type SimpleAttachment struct {
	ID          uint      `json:"id"`
	MessageID   uint      `json:"message_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	URL         string    `json:"url"` // Download URL for the attachment
	CreatedAt   time.Time `json:"created_at"`
}

// AddMessage persists a message as an article (article + article_data_mime) on the ticket.
// On success message.ID, TicketID and CreatedAt are set from the stored row.
func (s *SimpleTicketService) AddMessage(ticketID uint, message *SimpleTicketMessage) error {
	if s.ticketRepo == nil || s.db == nil {
		return errNoTicketStore
	}
	if _, err := s.ticketRepo.GetByID(ticketID); err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	if message.Body == "" {
		return fmt.Errorf("message body is required")
	}

	senderTypeID := constants.ArticleSenderAgent
	switch message.AuthorType {
	case "Customer":
		senderTypeID = constants.ArticleSenderCustomer
	case "System":
		senderTypeID = constants.ArticleSenderSystem
	}
	visible := 1
	if message.IsInternal {
		visible = 0
	}
	contentType := message.ContentType
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}
	now := time.Now()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin add message: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	articleID, err := database.GetAdapter().InsertWithReturningTx(tx, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING id
	`), ticketID, senderTypeID, constants.CommunicationChannelInternal, visible, message.CreatedBy, message.CreatedBy)
	if err != nil {
		return fmt.Errorf("insert article: %w", err)
	}

	if _, err := tx.Exec(database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body,
			a_content_type, incoming_time, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
	`), articleID, message.AuthorEmail, message.Subject, message.Body, contentType,
		now.Unix(), message.CreatedBy, message.CreatedBy); err != nil {
		return fmt.Errorf("insert article_data_mime: %w", err)
	}

	if _, err := tx.Exec(database.ConvertPlaceholders(`
		UPDATE ticket SET change_time = CURRENT_TIMESTAMP, change_by = ? WHERE id = ?
	`), message.CreatedBy, ticketID); err != nil {
		return fmt.Errorf("touch ticket: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit add message: %w", err)
	}

	message.ID = uint(articleID)
	message.TicketID = ticketID
	message.CreatedAt = now
	message.SenderTypeID = senderTypeID
	return nil
}

// GetMessages retrieves all articles of a ticket as messages.
func (s *SimpleTicketService) GetMessages(ticketID uint) ([]*SimpleTicketMessage, error) {
	if s.ticketRepo == nil || s.db == nil {
		return nil, errNoTicketStore
	}
	if _, err := s.ticketRepo.GetByID(ticketID); err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}
	db := s.db

	// Query articles from database - join with article_data_mime for content
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT a.id,
		       COALESCE(adm.a_subject, ''),
		       COALESCE(adm.a_body, ''),
		       COALESCE(adm.a_content_type, ''),
		       a.create_time, a.create_by,
		       COALESCE(adm.a_from, ''), COALESCE(adm.a_to, ''),
		       a.article_sender_type_id, a.is_visible_for_customer
		FROM article a
		LEFT JOIN article_data_mime adm ON a.id = adm.article_id
		WHERE a.ticket_id = ?
		ORDER BY a.create_time ASC
	`), ticketID)
	if err != nil {
		return nil, fmt.Errorf("query articles of ticket %d: %w", ticketID, err)
	}
	defer rows.Close()

	dbMessages := make([]*SimpleTicketMessage, 0)

	for rows.Next() {
		var articleID int
		var subject, body, contentType, fromAddr, toAddr string
		var createTime time.Time
		var createBy, senderTypeID, isVisible int

		err := rows.Scan(&articleID, &subject, &body, &contentType, &createTime, &createBy,
			&fromAddr, &toAddr, &senderTypeID, &isVisible)
		if err != nil {
			return nil, fmt.Errorf("scan article of ticket %d: %w", ticketID, err)
		}

		// Determine author type based on sender_type_id
		authorType := "System"
		if senderTypeID == constants.ArticleSenderAgent {
			authorType = "Agent"
		} else if senderTypeID == constants.ArticleSenderCustomer {
			authorType = "Customer"
		}

		// Extract author name from email or use default
		authorName := fromAddr
		if fromAddr != "" && strings.Contains(fromAddr, "@") {
			parts := strings.Split(fromAddr, "@")
			authorName = parts[0]
		} else if authorType == "Agent" {
			authorName = "Support Agent"
		} else if authorType == "Customer" {
			authorName = "Customer"
		}

		msg := &SimpleTicketMessage{
			ID:           uint(articleID),
			TicketID:     ticketID,
			Body:         body,
			Subject:      subject,
			ContentType:  contentType,
			CreatedBy:    uint(createBy),
			AuthorName:   authorName,
			AuthorEmail:  fromAddr,
			AuthorType:   authorType,
			SenderTypeID: senderTypeID,
			IsPublic:     isVisible == 1,
			IsInternal:   isVisible == 0,
			CreatedAt:    createTime,
			Attachments:  []*SimpleAttachment{},
		}

		dbMessages = append(dbMessages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate articles of ticket %d: %w", ticketID, err)
	}

	// Attachments come from the article storage (DB or FS); HTML body parts
	// are not attachments. Each one is addressed by ticket, article and file id.
	store := storage.ForDB(db)
	for _, msg := range dbMessages {
		atts, err := store.ListAttachments(context.Background(), int64(msg.ID))
		if err != nil {
			return nil, fmt.Errorf("list attachments of article %d: %w", msg.ID, err)
		}
		for _, a := range atts {
			if storage.IsHTMLBody(a) {
				continue
			}
			msg.Attachments = append(msg.Attachments, &SimpleAttachment{
				ID:          uint(a.FileID),
				MessageID:   msg.ID,
				Filename:    a.Filename,
				ContentType: a.ContentType,
				Size:        a.Size,
				URL:         fmt.Sprintf("/api/tickets/%d/articles/%d/attachments/%d", ticketID, msg.ID, a.FileID),
				CreatedAt:   msg.CreatedAt,
			})
		}
	}

	return dbMessages, nil
}
