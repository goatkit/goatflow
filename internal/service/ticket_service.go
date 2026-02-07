package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/constants"
	"github.com/goatkit/goatflow/internal/history"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
)

type TicketService interface {
	Create(ctx context.Context, req CreateTicketInput) (*models.Ticket, error)
}

type ticketService struct {
	repo     *repository.TicketRepository
	articles articleCreator
}

type articleCreator interface {
	Create(article *models.Article) error
}

type TicketServiceOption func(*ticketService)

// WithArticleRepository wires the optional article repository used for the initial article.
func WithArticleRepository(repo *repository.ArticleRepository) TicketServiceOption {
	return func(ts *ticketService) {
		if ts != nil && repo != nil {
			ts.articles = repo
		}
	}
}

func NewTicketService(repo *repository.TicketRepository, opts ...TicketServiceOption) TicketService {
	ts := &ticketService{repo: repo}
	for _, opt := range opts {
		if opt != nil {
			opt(ts)
		}
	}
	return ts
}

type CreateTicketInput struct {
	Title                         string
	QueueID                       int
	PriorityID                    int
	StateID                       int
	UserID                        int
	Body                          string // optional initial article body
	ArticleSubject                string
	ArticleSenderTypeID           int
	ArticleTypeID                 int
	ArticleIsVisibleForCustomer   *bool
	ArticleMimeType               string
	ArticleCharset                string
	ArticleCommunicationChannelID int
	PendingUntil                  int // unix seconds when pending should elapse; 0 = none
	TypeID                        int // optional ticket type to set on create (0 = none)
	CustomerID                    string
	CustomerUserID                string
}

func (s *ticketService) Create(ctx context.Context, in CreateTicketInput) (*models.Ticket, error) {
	if in.Title == "" {
		return nil, errors.New("title required")
	}
	if len(in.Title) > 255 {
		return nil, errors.New("title too long")
	}
	if in.QueueID <= 0 {
		return nil, errors.New("invalid queue")
	}
	if in.PriorityID == 0 {
		in.PriorityID = 3
	}
	if in.StateID == 0 {
		in.StateID = models.TicketStateNew
	}

	// Ensure queue exists before attempting insert
	if ok, err := s.repo.QueueExists(in.QueueID); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("invalid queue")
	}

	var stateTypeID int
	if in.StateID > 0 {
		state, err := s.repo.GetTicketStateByID(in.StateID)
		if err != nil {
			return nil, err
		}
		if state == nil || state.ValidID != 1 {
			return nil, errors.New("invalid ticket state")
		}
		stateTypeID = state.TypeID
	}

	ticket := &models.Ticket{
		Title:             in.Title,
		QueueID:           in.QueueID,
		TicketLockID:      models.TicketUnlocked,
		TicketStateID:     in.StateID,
		TicketPriorityID:  in.PriorityID,
		CreateBy:          in.UserID,
		ChangeBy:          in.UserID,
		UserID:            &in.UserID,
		ResponsibleUserID: &in.UserID,
		CreateTime:        time.Now(),
		ChangeTime:        time.Now(),
	}
	if trimmed := strings.TrimSpace(in.CustomerID); trimmed != "" {
		cid := trimmed
		ticket.CustomerID = &cid
	}
	if trimmed := strings.TrimSpace(in.CustomerUserID); trimmed != "" {
		cuid := trimmed
		ticket.CustomerUserID = &cuid
	}
	if in.TypeID > 0 {
		tid := in.TypeID
		ticket.TypeID = &tid
	}
	if (stateTypeID == 4 || stateTypeID == 5) && in.PendingUntil > 0 {
		ticket.UntilTime = in.PendingUntil
	} else if (stateTypeID == 4 || stateTypeID == 5) && in.PendingUntil <= 0 {
		return nil, errors.New("pending state requires pending until")
	}
	if err := s.repo.Create(ticket); err != nil {
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if ticket.ChangeTime.IsZero() {
		ticket.ChangeTime = time.Now()
	}

	message := fmt.Sprintf("Ticket created (%s)", ticket.TicketNumber)
	recorder := history.NewRecorder(s.repo)
	if recorder != nil {
		if err := recorder.Record(ctx, nil, ticket, nil, history.TypeNewTicket, message, in.UserID); err != nil {
			log.Printf("ticket history record (create) failed: %v", err)
		}
	}
	s.createInitialArticle(ticket, in)
	return ticket, nil
}

func (s *ticketService) createInitialArticle(ticket *models.Ticket, in CreateTicketInput) {
	if s == nil || s.articles == nil {
		return
	}
	body := strings.TrimSpace(in.Body)
	if body == "" || ticket == nil {
		return
	}
	subject := strings.TrimSpace(in.ArticleSubject)
	if subject == "" {
		subject = ticket.Title
	}
	senderType := in.ArticleSenderTypeID
	if senderType == 0 {
		senderType = constants.ArticleSenderAgent
	}
	articleType := in.ArticleTypeID
	if articleType == 0 {
		articleType = constants.ArticleTypeEmailExternal
	}
	visible := 1
	if in.ArticleIsVisibleForCustomer != nil {
		if !*in.ArticleIsVisibleForCustomer {
			visible = 0
		}
	}
	channelID := in.ArticleCommunicationChannelID
	if channelID <= 0 {
		channelID = 1
	}
	charset := strings.TrimSpace(in.ArticleCharset)
	if charset == "" {
		charset = "utf-8"
	}
	mimeType := strings.TrimSpace(in.ArticleMimeType)
	if mimeType == "" {
		mimeType = "text/plain"
	}
	if !strings.Contains(strings.ToLower(mimeType), "charset=") {
		mimeType = fmt.Sprintf("%s; charset=%s", mimeType, charset)
	}
	article := &models.Article{
		TicketID:               ticket.ID,
		Subject:                subject,
		Body:                   body,
		SenderTypeID:           senderType,
		ArticleTypeID:          articleType,
		CommunicationChannelID: channelID,
		IsVisibleForCustomer:   visible,
		Charset:                charset,
		MimeType:               mimeType,
		CreateBy:               in.UserID,
		ChangeBy:               in.UserID,
	}
	if err := s.articles.Create(article); err != nil {
		log.Printf("ticket article create failed for ticket %d: %v", ticket.ID, err)
	}
}
