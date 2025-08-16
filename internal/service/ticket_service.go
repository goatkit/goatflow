package service

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// TicketService handles business logic for tickets
type TicketService struct {
	ticketRepo   *repository.TicketRepository
	articleRepo  *repository.ArticleRepository
	userRepo     *repository.UserRepository
	queueRepo    *repository.QueueRepository
	stateRepo    *repository.TicketStateRepository
	priorityRepo *repository.TicketPriorityRepository
	db           *sql.DB
}

// NewTicketService creates a new ticket service
func NewTicketService(
	ticketRepo *repository.TicketRepository,
	articleRepo *repository.ArticleRepository,
	userRepo *repository.UserRepository,
	queueRepo *repository.QueueRepository,
	stateRepo *repository.TicketStateRepository,
	priorityRepo *repository.TicketPriorityRepository,
	db *sql.DB,
) *TicketService {
	return &TicketService{
		ticketRepo:   ticketRepo,
		articleRepo:  articleRepo,
		userRepo:     userRepo,
		queueRepo:    queueRepo,
		stateRepo:    stateRepo,
		priorityRepo: priorityRepo,
		db:           db,
	}
}

// CreateTicket creates a new ticket with initial article
func (s *TicketService) CreateTicket(req *models.CreateTicketRequest) (*models.Ticket, error) {
	// Validate request
	if err := s.validateCreateTicketRequest(req); err != nil {
		return nil, err
	}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Generate ticket number
	tn := s.generateTicketNumber()

	// Get default queue if not specified
	queueID := req.QueueID
	if queueID == 0 {
		queue, err := s.queueRepo.GetByName("General")
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to get default queue: %w", err)
		}
		queueID = queue.ID
	}

	// Get default state (new)
	stateID := req.StateID
	if stateID == 0 {
		state, err := s.stateRepo.GetByName("new")
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to get default state: %w", err)
		}
		stateID = state.ID
	}

	// Get default priority (normal)
	priorityID := req.PriorityID
	if priorityID == 0 {
		priority, err := s.priorityRepo.GetByName("normal")
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to get default priority: %w", err)
		}
		priorityID = priority.ID
	}

	// Create ticket
	ticket := &models.Ticket{
		TN:               tn,
		Title:            req.Title,
		QueueID:          queueID,
		TicketStateID:    stateID,
		TicketPriorityID: priorityID,
		CustomerUserID:   req.CustomerUserID,
		CustomerID:       &req.CustomerID,
		UserID:           &req.OwnerID,
		ResponsibleUserID: &req.ResponsibleUserID,
		TypeID:           int(req.TypeID),
		TenantID:         req.TenantID,
		CreateTime:       time.Now(),
		CreateBy:         req.CreateBy,
		ChangeTime:       time.Now(),
		ChangeBy:         req.CreateBy,
	}

	// Save ticket
	if err := s.ticketRepo.Create(ticket); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	// Create initial article if provided
	if req.InitialArticle != nil {
		article := &models.Article{
			TicketID:       ticket.ID,
			ArticleTypeID:  1, // Default to 'note' type
			SenderTypeID:   1, // Default to 'agent' sender
			Subject:        &req.InitialArticle.Subject,
			Body:           req.InitialArticle.Body,
			BodyType:       "text/plain",
			Charset:        "utf-8",
			MimeType:       "text/plain",
			CreateTime:     time.Now(),
			CreateBy:       req.CreateBy,
			ChangeTime:     time.Now(),
			ChangeBy:       req.CreateBy,
			ValidID:        1,
		}

		if req.InitialArticle.ContentType != "" {
			article.BodyType = req.InitialArticle.ContentType
			article.MimeType = req.InitialArticle.ContentType
		}

		if err := s.articleRepo.Create(article); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create initial article: %w", err)
		}

		// Handle attachments if any
		for _, attachment := range req.InitialArticle.Attachments {
			att := &models.Attachment{
				ArticleID:    article.ID,
				Filename:     attachment.Filename,
				ContentSize:  len(attachment.Content),
				ContentType:  attachment.ContentType,
				Content:      string(attachment.Content),
				CreateTime:   time.Now(),
				CreateBy:     req.CreateBy,
				ChangeTime:   time.Now(),
				ChangeBy:     req.CreateBy,
			}

			if err := s.articleRepo.CreateAttachment(att); err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create attachment: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Reload ticket with associations
	fullTicket, err := s.ticketRepo.GetByID(ticket.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload ticket: %w", err)
	}

	return fullTicket, nil
}

// UpdateTicket updates an existing ticket
func (s *TicketService) UpdateTicket(id uint, req *models.UpdateTicketRequest) (*models.Ticket, error) {
	// Get existing ticket
	ticket, err := s.ticketRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Check if user has permission to update (implement based on RBAC)
	if !s.canUpdateTicket(ticket, req.UpdateBy) {
		return nil, errors.New("permission denied")
	}

	// Update fields if provided
	if req.Title != "" {
		ticket.Title = req.Title
	}
	if req.QueueID != 0 {
		ticket.QueueID = req.QueueID
	}
	if req.StateID != 0 {
		ticket.TicketStateID = req.StateID
	}
	if req.PriorityID != 0 {
		ticket.TicketPriorityID = req.PriorityID
	}
	if req.OwnerID != 0 {
		ownerID := req.OwnerID
		ticket.UserID = &ownerID
	}
	if req.ResponsibleUserID != 0 {
		respID := req.ResponsibleUserID
		ticket.ResponsibleUserID = &respID
	}

	ticket.ChangeTime = time.Now()
	ticket.ChangeBy = req.UpdateBy

	// Save updates
	if err := s.ticketRepo.Update(ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

// AddArticle adds a new article to a ticket
func (s *TicketService) AddArticle(ticketID uint, req *models.CreateArticleRequest) (*models.Article, error) {
	// Verify ticket exists
	ticket, err := s.ticketRepo.GetByID(ticketID)
	if err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	// Check if user has permission to add article
	if !s.canAddArticle(ticket, req.CreateBy) {
		return nil, errors.New("permission denied")
	}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create article
	article := &models.Article{
		TicketID:      ticketID,
		ArticleTypeID: req.ArticleTypeID,
		SenderTypeID:  req.SenderTypeID,
		Subject:       &req.Subject,
		Body:          req.Body,
		BodyType:      req.ContentType,
		Charset:       "utf-8",
		MimeType:      req.ContentType,
		CreateTime:    time.Now(),
		CreateBy:      req.CreateBy,
		ChangeTime:    time.Now(),
		ChangeBy:      req.CreateBy,
		ValidID:       1,
	}

	if req.ContentType == "" {
		article.BodyType = "text/plain"
		article.MimeType = "text/plain"
	}

	if err := s.articleRepo.Create(article); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create article: %w", err)
	}

	// Handle attachments
	for _, attachment := range req.Attachments {
		att := &models.Attachment{
			ArticleID:    article.ID,
			Filename:     attachment.Filename,
			ContentSize:  len(attachment.Content),
			ContentType:  attachment.ContentType,
			Content:      attachment.Content,
			CreateTime:   time.Now(),
			CreateBy:     req.CreateBy,
			ChangeTime:   time.Now(),
			ChangeBy:     req.CreateBy,
		}

		if err := s.articleRepo.CreateAttachment(att); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create attachment: %w", err)
		}
	}

	// Update ticket change time
	ticket.ChangeTime = time.Now()
	ticket.ChangeBy = req.CreateBy
	if err := s.ticketRepo.Update(ticket); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return article, nil
}

// MergeTickets merges source ticket into target ticket
func (s *TicketService) MergeTickets(targetID, sourceID uint, userID uint) error {
	// Get both tickets
	targetTicket, err := s.ticketRepo.GetByID(targetID)
	if err != nil {
		return fmt.Errorf("target ticket not found: %w", err)
	}

	sourceTicket, err := s.ticketRepo.GetByID(sourceID)
	if err != nil {
		return fmt.Errorf("source ticket not found: %w", err)
	}

	// Check permissions
	if !s.canMergeTickets(targetTicket, sourceTicket, userID) {
		return errors.New("permission denied")
	}

	// Begin transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Move all articles from source to target
	query := `UPDATE articles SET ticket_id = $1 WHERE ticket_id = $2`
	if _, err := tx.Exec(query, targetID, sourceID); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to move articles: %w", err)
	}

	// Create merge note in target ticket
	mergeNote := &models.Article{
		TicketID:      targetID,
		ArticleTypeID: 1, // note
		SenderTypeID:  1, // agent
		Subject:       fmt.Sprintf("Merged ticket %s", sourceTicket.TN),
		Body:          fmt.Sprintf("Ticket %s has been merged into this ticket.", sourceTicket.TN),
		BodyType:      "text/plain",
		Charset:       "utf-8",
		MimeType:      "text/plain",
		CreateTime:    time.Now(),
		CreateBy:      userID,
		ChangeTime:    time.Now(),
		ChangeBy:      userID,
		ValidID:       1,
	}

	if err := s.articleRepo.Create(mergeNote); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create merge note: %w", err)
	}

	// Mark source ticket as merged
	mergedState, err := s.stateRepo.GetByName("merged")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to get merged state: %w", err)
	}

	sourceTicket.TicketStateID = mergedState.ID
	sourceTicket.ChangeTime = time.Now()
	sourceTicket.ChangeBy = userID

	if err := s.ticketRepo.Update(sourceTicket); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update source ticket: %w", err)
	}

	// Update target ticket
	targetTicket.ChangeTime = time.Now()
	targetTicket.ChangeBy = userID

	if err := s.ticketRepo.Update(targetTicket); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update target ticket: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// AssignTicket assigns a ticket to an agent
func (s *TicketService) AssignTicket(ticketID uint, agentID uint, assignedBy uint) error {
	ticket, err := s.ticketRepo.GetByID(ticketID)
	if err != nil {
		return err
	}

	// Check if assignedBy has permission
	if !s.canAssignTicket(ticket, assignedBy) {
		return errors.New("permission denied")
	}

	// Verify agent exists and has appropriate role
	agent, err := s.userRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	if agent.Role != "Agent" && agent.Role != "Admin" {
		return errors.New("user is not an agent")
	}

	// Update ticket
	ticket.UserID = &agentID
	ticket.ChangeTime = time.Now()
	ticket.ChangeBy = assignedBy

	return s.ticketRepo.Update(ticket)
}

// EscalateTicket escalates a ticket priority
func (s *TicketService) EscalateTicket(ticketID uint, newPriorityID uint, escalatedBy uint, reason string) error {
	ticket, err := s.ticketRepo.GetByID(ticketID)
	if err != nil {
		return err
	}

	// Check permission
	if !s.canEscalateTicket(ticket, escalatedBy) {
		return errors.New("permission denied")
	}

	// Begin transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Get priority names for note
	oldPriority, _ := s.priorityRepo.GetByID(ticket.TicketPriorityID)
	newPriority, err := s.priorityRepo.GetByID(newPriorityID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("invalid priority: %w", err)
	}

	// Update ticket priority
	ticket.TicketPriorityID = newPriorityID
	ticket.ChangeTime = time.Now()
	ticket.ChangeBy = escalatedBy

	if err := s.ticketRepo.Update(ticket); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update ticket: %w", err)
	}

	// Add escalation note
	note := &models.Article{
		TicketID:      ticketID,
		ArticleTypeID: 1, // note
		SenderTypeID:  1, // agent
		Subject:       "Ticket Escalated",
		Body:          fmt.Sprintf("Priority changed from '%s' to '%s'. Reason: %s", oldPriority.Name, newPriority.Name, reason),
		BodyType:      "text/plain",
		Charset:       "utf-8",
		MimeType:      "text/plain",
		CreateTime:    time.Now(),
		CreateBy:      escalatedBy,
		ChangeTime:    time.Now(),
		ChangeBy:      escalatedBy,
		ValidID:       1,
	}

	if err := s.articleRepo.Create(note); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create escalation note: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTicketHistory returns the history of changes for a ticket
func (s *TicketService) GetTicketHistory(ticketID uint) ([]*models.TicketHistory, error) {
	// This would typically query an audit log table
	// For now, return articles as history
	articles, err := s.articleRepo.GetByTicketID(ticketID, true)
	if err != nil {
		return nil, err
	}

	history := make([]*models.TicketHistory, 0, len(articles))
	for i := range articles {
		articleID := articles[i].ID
		history = append(history, &models.TicketHistory{
			TicketID:    ticketID,
			ArticleID:   &articleID,
			Name:        articles[i].Subject,
			Value:       articles[i].Body,
			CreateTime:  articles[i].CreateTime,
			CreateBy:    articles[i].CreateBy,
			ChangeTime:  articles[i].ChangeTime,
			ChangeBy:    articles[i].ChangeBy,
		})
	}

	return history, nil
}

// Helper methods

func (s *TicketService) generateTicketNumber() string {
	// Generate unique ticket number
	// Format: YYYYMMDDHHMMSS + 6 random digits
	return fmt.Sprintf("%s%06d", time.Now().Format("20060102150405"), time.Now().UnixNano()%1000000)
}

func (s *TicketService) validateCreateTicketRequest(req *models.CreateTicketRequest) error {
	if req.Title == "" {
		return errors.New("title is required")
	}
	if req.CustomerUserID == 0 && req.CustomerID == "" {
		return errors.New("customer information is required")
	}
	if req.TenantID == 0 {
		return errors.New("tenant ID is required")
	}
	return nil
}

func (s *TicketService) canUpdateTicket(ticket *models.Ticket, userID uint) bool {
	// Implement based on RBAC
	// For now, allow all authenticated users
	return userID > 0
}

func (s *TicketService) canAddArticle(ticket *models.Ticket, userID uint) bool {
	// Implement based on RBAC
	// For now, allow all authenticated users
	return userID > 0
}

func (s *TicketService) canMergeTickets(target, source *models.Ticket, userID uint) bool {
	// Implement based on RBAC
	// For now, only allow agents and admins
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return false
	}
	return user.Role == "Agent" || user.Role == "Admin"
}

func (s *TicketService) canAssignTicket(ticket *models.Ticket, userID uint) bool {
	// Implement based on RBAC
	// For now, only allow agents and admins
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return false
	}
	return user.Role == "Agent" || user.Role == "Admin"
}

func (s *TicketService) canEscalateTicket(ticket *models.Ticket, userID uint) bool {
	// Implement based on RBAC
	// For now, only allow agents and admins
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return false
	}
	return user.Role == "Agent" || user.Role == "Admin"
}