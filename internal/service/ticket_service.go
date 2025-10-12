package service

import (
    "context"
    "errors"
    "time"

    "github.com/gotrs-io/gotrs-ce/internal/models"
    "github.com/gotrs-io/gotrs-ce/internal/repository"
)

type TicketService interface {
    Create(ctx context.Context, req CreateTicketInput) (*models.Ticket, error)
}

type ticketService struct { repo *repository.TicketRepository }

func NewTicketService(repo *repository.TicketRepository) TicketService { return &ticketService{repo: repo} }

type CreateTicketInput struct {
    Title     string
    QueueID   int
    PriorityID int
    StateID   int
    UserID    int
    Body      string // reserved for future article create
}

func (s *ticketService) Create(ctx context.Context, in CreateTicketInput) (*models.Ticket, error) {
    if in.Title == "" { return nil, errors.New("title required") }
    if len(in.Title) > 255 { return nil, errors.New("title too long") }
    if in.QueueID <= 0 { return nil, errors.New("invalid queue") }
    if in.PriorityID == 0 { in.PriorityID = 3 }
    if in.StateID == 0 { in.StateID = models.TicketStateNew }

    // Ensure queue exists before attempting insert (aligns with tests' sqlmock expectation)
    if ok, err := s.repo.QueueExists(in.QueueID); err != nil {
        return nil, err
    } else if !ok {
        return nil, errors.New("invalid queue")
    }

    ticket := &models.Ticket{
        Title:            in.Title,
        QueueID:          in.QueueID,
        TicketLockID:     models.TicketUnlocked,
        TicketStateID:    in.StateID,
        TicketPriorityID: in.PriorityID,
        CreateBy:         in.UserID,
        ChangeBy:         in.UserID,
        UserID:           &in.UserID,
        ResponsibleUserID: &in.UserID,
        CreateTime:       time.Now(),
        ChangeTime:       time.Now(),
    }
    if err := s.repo.Create(ticket); err != nil { return nil, err }
    return ticket, nil
}
