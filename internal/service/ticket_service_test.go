package service

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTicketServiceMocks(t *testing.T) (*TicketService, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	userRepo := repository.NewUserRepository(db)
	queueRepo := repository.NewQueueRepository(db)
	stateRepo := repository.NewTicketStateRepository(db)
	priorityRepo := repository.NewTicketPriorityRepository(db)

	service := NewTicketService(
		ticketRepo,
		articleRepo,
		userRepo,
		queueRepo,
		stateRepo,
		priorityRepo,
		db,
	)

	return service, mock
}

func TestTicketService_CreateTicket(t *testing.T) {
	t.Run("successful ticket creation with initial article", func(t *testing.T) {
		service, mock := setupTicketServiceMocks(t)
		defer mock.ExpectClose()

		// Mock transaction
		mock.ExpectBegin()

		// Mock getting default queue
		mock.ExpectQuery("SELECT .* FROM queues WHERE name = \\$1").
			WithArgs("General").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
				AddRow(1, "General"))

		// Mock getting default state
		mock.ExpectQuery("SELECT .* FROM ticket_states WHERE name = \\$1").
			WithArgs("new").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
				AddRow(1, "new"))

		// Mock getting default priority
		mock.ExpectQuery("SELECT .* FROM ticket_priorities WHERE name = \\$1").
			WithArgs("normal").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
				AddRow(3, "normal"))

		// Mock ticket creation
		mock.ExpectQuery("INSERT INTO tickets").
			WithArgs(sqlmock.AnyArg(), "Test Ticket", 1, 1, 3, 100, "CUST001", 10, sqlmock.AnyArg(), 
				sqlmock.AnyArg(), 1, sqlmock.AnyArg(), 1, sqlmock.AnyArg(), sqlmock.AnyArg(), 1, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		// Mock article creation
		mock.ExpectQuery("INSERT INTO articles").
			WithArgs(1, 1, 1, sqlmock.AnyArg(), sqlmock.AnyArg(), "Initial Article", 
				"This is the initial article", sqlmock.AnyArg(), sqlmock.AnyArg(), 
				sqlmock.AnyArg(), sqlmock.AnyArg(), 1, sqlmock.AnyArg(), 1, sqlmock.AnyArg(), 1).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		// Mock updating ticket change time
		mock.ExpectExec("UPDATE tickets SET change_time = \\$2").
			WithArgs(1, sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Mock commit
		mock.ExpectCommit()

		// Mock reload ticket
		mock.ExpectQuery("SELECT .* FROM tickets WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tn", "title", "queue_id", "ticket_state_id", "ticket_priority_id",
				"customer_user_id", "customer_id", "user_id", "responsible_user_id",
				"ticket_lock_id", "ticket_type_id", "escalation_time", "escalation_update_time",
				"escalation_response_time", "escalation_solution_time", "unlock_timeout",
				"archive_flag", "valid_id", "tenant_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				1, "202501150001", "Test Ticket", 1, 1, 3,
				100, "CUST001", 10, nil,
				1, nil, nil, nil,
				nil, nil, nil,
				0, 1, 1, time.Now(), 1,
				time.Now(), 1,
			))

		req := &models.CreateTicketRequest{
			Title:          "Test Ticket",
			CustomerUserID: 100,
			CustomerID:     "CUST001",
			OwnerID:        10,
			TenantID:       1,
			CreateBy:       1,
			InitialArticle: &models.CreateArticleRequest{
				ArticleTypeID: 1,
				SenderTypeID:  1,
				Subject:       "Initial Article",
				Body:          "This is the initial article",
				CreateBy:      1,
			},
		}

		ticket, err := service.CreateTicket(req)

		assert.NoError(t, err)
		assert.NotNil(t, ticket)
		assert.Equal(t, uint(1), ticket.ID)
		assert.Equal(t, "Test Ticket", ticket.Title)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("validation error - missing title", func(t *testing.T) {
		service, _ := setupTicketServiceMocks(t)

		req := &models.CreateTicketRequest{
			Title:    "",
			TenantID: 1,
			CreateBy: 1,
		}

		ticket, err := service.CreateTicket(req)

		assert.Error(t, err)
		assert.Nil(t, ticket)
		assert.Contains(t, err.Error(), "title is required")
	})

	t.Run("validation error - missing customer info", func(t *testing.T) {
		service, _ := setupTicketServiceMocks(t)

		req := &models.CreateTicketRequest{
			Title:    "Test Ticket",
			TenantID: 1,
			CreateBy: 1,
		}

		ticket, err := service.CreateTicket(req)

		assert.Error(t, err)
		assert.Nil(t, ticket)
		assert.Contains(t, err.Error(), "customer information is required")
	})
}

func TestTicketService_UpdateTicket(t *testing.T) {
	t.Run("successful ticket update", func(t *testing.T) {
		service, mock := setupTicketServiceMocks(t)
		defer mock.ExpectClose()

		// Mock get ticket by ID
		mock.ExpectQuery("SELECT .* FROM tickets WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tn", "title", "queue_id", "ticket_state_id", "ticket_priority_id",
				"customer_user_id", "customer_id", "user_id", "responsible_user_id",
				"ticket_lock_id", "ticket_type_id", "escalation_time", "escalation_update_time",
				"escalation_response_time", "escalation_solution_time", "unlock_timeout",
				"archive_flag", "valid_id", "tenant_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				1, "202501150001", "Old Title", 1, 1, 3,
				100, "CUST001", 10, nil,
				1, nil, nil, nil,
				nil, nil, nil,
				0, 1, 1, time.Now(), 1,
				time.Now(), 1,
			))

		// Mock update ticket
		mock.ExpectExec("UPDATE tickets SET").
			WithArgs(1, "202501150001", "New Title", 2, 2, 4, 100, "CUST001", 20, 
				30, 1, nil, nil, nil, nil, nil, nil, 0, 1, 1, sqlmock.AnyArg(), 2).
			WillReturnResult(sqlmock.NewResult(0, 1))

		req := &models.UpdateTicketRequest{
			Title:             "New Title",
			QueueID:           2,
			StateID:           2,
			PriorityID:        4,
			OwnerID:           20,
			ResponsibleUserID: 30,
			UpdateBy:          2,
		}

		ticket, err := service.UpdateTicket(1, req)

		assert.NoError(t, err)
		assert.NotNil(t, ticket)
		assert.Equal(t, "New Title", ticket.Title)
		assert.Equal(t, uint(2), ticket.QueueID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTicketService_AssignTicket(t *testing.T) {
	t.Run("successful ticket assignment", func(t *testing.T) {
		service, mock := setupTicketServiceMocks(t)
		defer mock.ExpectClose()

		// Mock get ticket
		mock.ExpectQuery("SELECT .* FROM tickets WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tn", "title", "queue_id", "ticket_state_id", "ticket_priority_id",
				"customer_user_id", "customer_id", "user_id", "responsible_user_id",
				"ticket_lock_id", "ticket_type_id", "escalation_time", "escalation_update_time",
				"escalation_response_time", "escalation_solution_time", "unlock_timeout",
				"archive_flag", "valid_id", "tenant_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				1, "202501150001", "Test Ticket", 1, 1, 3,
				100, "CUST001", 10, nil,
				1, nil, nil, nil,
				nil, nil, nil,
				0, 1, 1, time.Now(), 1,
				time.Now(), 1,
			))

		// Mock get agent
		mock.ExpectQuery("SELECT .* FROM users WHERE id = \\$1").
			WithArgs(20).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tenant_id", "login", "email", "title", "first_name", "last_name",
				"password", "role", "customer_id", "mobile", "failed_login_attempts",
				"account_locked", "account_locked_until", "password_reset_token",
				"password_reset_expires", "email_verified", "email_verification_token",
				"preferences", "last_login", "valid_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				20, 1, "agent1", "agent@example.com", nil, "Agent", "One",
				"hash", "Agent", nil, nil, 0,
				false, nil, nil,
				nil, true, nil,
				nil, nil, 1, time.Now(), 1,
				time.Now(), 1,
			))

		// Mock update ticket
		mock.ExpectExec("UPDATE tickets SET").
			WithArgs(1, "202501150001", "Test Ticket", 1, 1, 3, 100, "CUST001", 20,
				nil, 1, nil, nil, nil, nil, nil, nil, 0, 1, 1, sqlmock.AnyArg(), 2).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := service.AssignTicket(1, 20, 2)

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("error - user is not an agent", func(t *testing.T) {
		service, mock := setupTicketServiceMocks(t)
		defer mock.ExpectClose()

		// Mock get ticket
		mock.ExpectQuery("SELECT .* FROM tickets WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tn", "title", "queue_id", "ticket_state_id", "ticket_priority_id",
				"customer_user_id", "customer_id", "user_id", "responsible_user_id",
				"ticket_lock_id", "ticket_type_id", "escalation_time", "escalation_update_time",
				"escalation_response_time", "escalation_solution_time", "unlock_timeout",
				"archive_flag", "valid_id", "tenant_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				1, "202501150001", "Test Ticket", 1, 1, 3,
				100, "CUST001", 10, nil,
				1, nil, nil, nil,
				nil, nil, nil,
				0, 1, 1, time.Now(), 1,
				time.Now(), 1,
			))

		// Mock get user (customer, not agent)
		mock.ExpectQuery("SELECT .* FROM users WHERE id = \\$1").
			WithArgs(30).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tenant_id", "login", "email", "title", "first_name", "last_name",
				"password", "role", "customer_id", "mobile", "failed_login_attempts",
				"account_locked", "account_locked_until", "password_reset_token",
				"password_reset_expires", "email_verified", "email_verification_token",
				"preferences", "last_login", "valid_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				30, 1, "customer1", "customer@example.com", nil, "Customer", "One",
				"hash", "Customer", nil, nil, 0,
				false, nil, nil,
				nil, true, nil,
				nil, nil, 1, time.Now(), 1,
				time.Now(), 1,
			))

		err := service.AssignTicket(1, 30, 2)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user is not an agent")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTicketService_MergeTickets(t *testing.T) {
	t.Run("successful ticket merge", func(t *testing.T) {
		service, mock := setupTicketServiceMocks(t)
		defer mock.ExpectClose()

		// Mock get target ticket
		mock.ExpectQuery("SELECT .* FROM tickets WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tn", "title", "queue_id", "ticket_state_id", "ticket_priority_id",
				"customer_user_id", "customer_id", "user_id", "responsible_user_id",
				"ticket_lock_id", "ticket_type_id", "escalation_time", "escalation_update_time",
				"escalation_response_time", "escalation_solution_time", "unlock_timeout",
				"archive_flag", "valid_id", "tenant_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				1, "202501150001", "Target Ticket", 1, 1, 3,
				100, "CUST001", 10, nil,
				1, nil, nil, nil,
				nil, nil, nil,
				0, 1, 1, time.Now(), 1,
				time.Now(), 1,
			))

		// Mock get source ticket
		mock.ExpectQuery("SELECT .* FROM tickets WHERE id = \\$1").
			WithArgs(2).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tn", "title", "queue_id", "ticket_state_id", "ticket_priority_id",
				"customer_user_id", "customer_id", "user_id", "responsible_user_id",
				"ticket_lock_id", "ticket_type_id", "escalation_time", "escalation_update_time",
				"escalation_response_time", "escalation_solution_time", "unlock_timeout",
				"archive_flag", "valid_id", "tenant_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				2, "202501150002", "Source Ticket", 1, 1, 3,
				100, "CUST001", 10, nil,
				1, nil, nil, nil,
				nil, nil, nil,
				0, 1, 1, time.Now(), 1,
				time.Now(), 1,
			))

		// Mock get merging user
		mock.ExpectQuery("SELECT .* FROM users WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tenant_id", "login", "email", "title", "first_name", "last_name",
				"password", "role", "customer_id", "mobile", "failed_login_attempts",
				"account_locked", "account_locked_until", "password_reset_token",
				"password_reset_expires", "email_verified", "email_verification_token",
				"preferences", "last_login", "valid_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				1, 1, "admin", "admin@example.com", nil, "Admin", "User",
				"hash", "Admin", nil, nil, 0,
				false, nil, nil,
				nil, true, nil,
				nil, nil, 1, time.Now(), 1,
				time.Now(), 1,
			))

		// Mock transaction
		mock.ExpectBegin()

		// Mock move articles
		mock.ExpectExec("UPDATE articles SET ticket_id = \\$1 WHERE ticket_id = \\$2").
			WithArgs(1, 2).
			WillReturnResult(sqlmock.NewResult(0, 3))

		// Mock create merge note
		mock.ExpectQuery("INSERT INTO articles").
			WithArgs(1, 1, 1, sqlmock.AnyArg(), sqlmock.AnyArg(), "Merged ticket 202501150002",
				"Ticket 202501150002 has been merged into this ticket.", sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 1,
				sqlmock.AnyArg(), 1, sqlmock.AnyArg(), 1).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))

		// Mock update merge note's ticket change time
		mock.ExpectExec("UPDATE tickets SET change_time = \\$2").
			WithArgs(1, sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Mock get merged state
		mock.ExpectQuery("SELECT .* FROM ticket_states WHERE name = \\$1").
			WithArgs("merged").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
				AddRow(9, "merged"))

		// Mock update source ticket to merged
		mock.ExpectExec("UPDATE tickets SET").
			WithArgs(2, "202501150002", "Source Ticket", 1, 9, 3, 100, "CUST001", 10,
				nil, 1, nil, nil, nil, nil, nil, nil, 0, 1, 1, sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Mock update target ticket
		mock.ExpectExec("UPDATE tickets SET").
			WithArgs(1, "202501150001", "Target Ticket", 1, 1, 3, 100, "CUST001", 10,
				nil, 1, nil, nil, nil, nil, nil, nil, 0, 1, 1, sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Mock commit
		mock.ExpectCommit()

		err := service.MergeTickets(1, 2, 1)

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestTicketService_EscalateTicket(t *testing.T) {
	t.Run("successful ticket escalation", func(t *testing.T) {
		service, mock := setupTicketServiceMocks(t)
		defer mock.ExpectClose()

		// Mock get ticket
		mock.ExpectQuery("SELECT .* FROM tickets WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tn", "title", "queue_id", "ticket_state_id", "ticket_priority_id",
				"customer_user_id", "customer_id", "user_id", "responsible_user_id",
				"ticket_lock_id", "ticket_type_id", "escalation_time", "escalation_update_time",
				"escalation_response_time", "escalation_solution_time", "unlock_timeout",
				"archive_flag", "valid_id", "tenant_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				1, "202501150001", "Test Ticket", 1, 1, 3,
				100, "CUST001", 10, nil,
				1, nil, nil, nil,
				nil, nil, nil,
				0, 1, 1, time.Now(), 1,
				time.Now(), 1,
			))

		// Mock get escalating user
		mock.ExpectQuery("SELECT .* FROM users WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "tenant_id", "login", "email", "title", "first_name", "last_name",
				"password", "role", "customer_id", "mobile", "failed_login_attempts",
				"account_locked", "account_locked_until", "password_reset_token",
				"password_reset_expires", "email_verified", "email_verification_token",
				"preferences", "last_login", "valid_id", "create_time", "create_by",
				"change_time", "change_by",
			}).AddRow(
				1, 1, "agent", "agent@example.com", nil, "Agent", "User",
				"hash", "Agent", nil, nil, 0,
				false, nil, nil,
				nil, true, nil,
				nil, nil, 1, time.Now(), 1,
				time.Now(), 1,
			))

		// Mock transaction
		mock.ExpectBegin()

		// Mock get old priority
		mock.ExpectQuery("SELECT .* FROM ticket_priorities WHERE id = \\$1").
			WithArgs(3).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
				AddRow(3, "normal"))

		// Mock get new priority
		mock.ExpectQuery("SELECT .* FROM ticket_priorities WHERE id = \\$1").
			WithArgs(5).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
				AddRow(5, "very high"))

		// Mock update ticket priority
		mock.ExpectExec("UPDATE tickets SET").
			WithArgs(1, "202501150001", "Test Ticket", 1, 1, 5, 100, "CUST001", 10,
				nil, 1, nil, nil, nil, nil, nil, nil, 0, 1, 1, sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Mock create escalation note
		mock.ExpectQuery("INSERT INTO articles").
			WithArgs(1, 1, 1, sqlmock.AnyArg(), sqlmock.AnyArg(), "Ticket Escalated",
				"Priority changed from 'normal' to 'very high'. Reason: Critical customer issue",
				sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 1,
				sqlmock.AnyArg(), 1, sqlmock.AnyArg(), 1).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(20))

		// Mock update ticket change time for escalation note
		mock.ExpectExec("UPDATE tickets SET change_time = \\$2").
			WithArgs(1, sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// Mock commit
		mock.ExpectCommit()

		err := service.EscalateTicket(1, 5, 1, "Critical customer issue")

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}