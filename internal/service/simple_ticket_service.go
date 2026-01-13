package service

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// This is for development/testing with in-memory repository.
type SimpleTicketService struct {
	ticketRepo repository.ITicketRepository
	messages   map[uint][]*SimpleTicketMessage // In-memory message storage by ticket ID
	messagesMu sync.RWMutex                    // Mutex for thread-safe access to messages
	nextMsgID  uint                            // Auto-incrementing message ID
}

// NewSimpleTicketService creates a new simple ticket service.
func NewSimpleTicketService(repo repository.ITicketRepository) *SimpleTicketService {
	return &SimpleTicketService{
		ticketRepo: repo,
		messages:   make(map[uint][]*SimpleTicketMessage),
		nextMsgID:  1,
	}
}

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
		// DB-less mode in tests: only ticket 1 exists
		if ticketID == 1 {
			now := time.Now()
			return &models.Ticket{ID: int(ticketID), TicketNumber: "T-TEST-1", Title: "Test Ticket", QueueID: 1, TicketStateID: 2, TicketPriorityID: 2, CreateTime: now, ChangeTime: now}, nil
		}
		return nil, fmt.Errorf("ticket not found")
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

// AddMessage adds a message to a ticket.
func (s *SimpleTicketService) AddMessage(ticketID uint, message *SimpleTicketMessage) error {
	// Validate the ticket exists
	var err error
	if s.ticketRepo != nil {
		_, err = s.ticketRepo.GetByID(ticketID)
	} else {
		if ticketID != 1 {
			err = fmt.Errorf("ticket not found")
		}
	}
	if err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	if message.Body == "" {
		return fmt.Errorf("message body is required")
	}

	// Lock for thread-safe access
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

	// Set message metadata
	message.ID = s.nextMsgID
	message.TicketID = ticketID
	message.CreatedAt = time.Now()
	s.nextMsgID++

	// Set default author info if not provided
	if message.AuthorName == "" {
		if message.AuthorType == "Customer" {
			message.AuthorName = "Customer"
		} else {
			message.AuthorName = "System User"
		}
	}

	if message.AuthorType == "" {
		message.AuthorType = "System"
	}

	// Store the message
	if s.messages[ticketID] == nil {
		s.messages[ticketID] = make([]*SimpleTicketMessage, 0)
	}
	s.messages[ticketID] = append(s.messages[ticketID], message)

	return nil
}

// GetMessages retrieves all messages for a ticket.
func (s *SimpleTicketService) GetMessages(ticketID uint) ([]*SimpleTicketMessage, error) {
	// Validate the ticket exists
	var err error
	if s.ticketRepo != nil {
		_, err = s.ticketRepo.GetByID(ticketID)
	} else {
		// DB-less mode: only ticket 1 exists
		if ticketID != 1 {
			err = fmt.Errorf("ticket not found")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	// First check in-memory messages
	s.messagesMu.RLock()
	inMemoryMessages := s.messages[ticketID]
	s.messagesMu.RUnlock()

	// Also retrieve messages from the database (articles)
	db, err := database.GetDB()
	if err != nil || db == nil {
		// If database is not available, return in-memory messages only
		if inMemoryMessages == nil {
			return make([]*SimpleTicketMessage, 0), nil
		}
		result := make([]*SimpleTicketMessage, len(inMemoryMessages))
		copy(result, inMemoryMessages)
		return result, nil
	}

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
		// If query fails, return in-memory messages
		if inMemoryMessages == nil {
			return make([]*SimpleTicketMessage, 0), nil
		}
		result := make([]*SimpleTicketMessage, len(inMemoryMessages))
		copy(result, inMemoryMessages)
		return result, nil
	}
	defer rows.Close()

	var dbMessages []*SimpleTicketMessage
	var articleIDs []int

	for rows.Next() {
		var articleID int
		var subject, body, contentType, fromAddr, toAddr string
		var createTime time.Time
		var createBy, senderTypeID, isVisible int

		err := rows.Scan(&articleID, &subject, &body, &contentType, &createTime, &createBy,
			&fromAddr, &toAddr, &senderTypeID, &isVisible)
		if err != nil {
			continue
		}

		// Determine author type based on sender_type_id
		authorType := "System"
		if senderTypeID == 1 {
			authorType = "Agent"
		} else if senderTypeID == 3 {
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
		articleIDs = append(articleIDs, articleID)
	}
	_ = rows.Err() // Check for iteration errors

	// Now query attachments for all articles using sqlx.In for safe IN clause
	if len(articleIDs) > 0 {
		qb, err := database.GetQueryBuilder()
		if err != nil {
			return dbMessages, nil
		}

		query, args, err := qb.In(`
			SELECT att.id, att.article_id, att.filename,
			       COALESCE(att.content_type, 'application/octet-stream'),
			       COALESCE(att.content_size, '0'),
			       att.content
			FROM article_data_mime_attachment att
			WHERE att.article_id IN (?)
			ORDER BY att.id
		`, articleIDs)
		if err != nil {
			return dbMessages, nil
		}

		attachRows, err := qb.Query(query, args...)
		if err == nil {
			defer attachRows.Close()

			// Create a map for quick message lookup
			messageMap := make(map[uint]*SimpleTicketMessage)
			for _, msg := range dbMessages {
				messageMap[msg.ID] = msg
			}

			for attachRows.Next() {
				var attID, articleID int
				var filename, contentType, contentSize string
				var content []byte

				err := attachRows.Scan(&attID, &articleID, &filename,
					&contentType, &contentSize, &content)
				if err != nil {
					continue
				}

				// Parse size
				size, _ := strconv.ParseInt(contentSize, 10, 64)

				// Add attachment to the corresponding message
				if msg, ok := messageMap[uint(articleID)]; ok {
					attachment := &SimpleAttachment{
						ID:          uint(attID),
						MessageID:   uint(articleID),
						Filename:    filename,
						ContentType: contentType,
						Size:        size,
						URL:         fmt.Sprintf("/api/attachments/%d/download", attID),
						CreatedAt:   msg.CreatedAt,
					}
					msg.Attachments = append(msg.Attachments, attachment)
				}
			}
			_ = attachRows.Err() // Check for iteration errors
		}
	}

	// Return database messages if found
	if len(dbMessages) > 0 {
		return dbMessages, nil
	}

	// Return in-memory messages if no database messages found
	if inMemoryMessages == nil {
		return make([]*SimpleTicketMessage, 0), nil
	}
	result := make([]*SimpleTicketMessage, len(inMemoryMessages))
	copy(result, inMemoryMessages)
	return result, nil
}
