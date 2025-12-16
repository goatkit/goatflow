package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

type stubGen struct{ n string }

func (g stubGen) Name() string      { return "Date" }
func (g stubGen) IsDateBased() bool { return true }
func (g stubGen) Next(ctx context.Context, store ticketnumber.CounterStore) (string, error) {
	return g.n, nil
}

type stubStore struct{}

func (stubStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) {
	return 1, nil
}

func TestTicketService_CreateRecordsHistory(t *testing.T) {
	t.Setenv("TEST_DB_DRIVER", "postgres")
	t.Setenv("DB_DRIVER", "postgres")
	database.ResetAdapterForTest()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repository.SetTicketNumberGenerator(stubGen{n: "202510050002"}, stubStore{})
	repo := repository.NewTicketRepository(db)
	svc := NewTicketService(repo)

	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM queue")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, type_id, valid_id,")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
			AddRow(1, "new", 1, 1, now, 1, now, 1))

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
		WithArgs(
			"202510050002",
			"Alpha",
			1,
			1,
			nil,
			nil,
			nil,
			1,
			1,
			nil,
			nil,
			1,
			3,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(88))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM ticket_history_type")).
		WithArgs("NewTicket").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(30))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ticket_history (")).
		WithArgs(
			"Ticket created (202510050002)",
			30,
			88,
			nil,
			1,
			1,
			1,
			3,
			1,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	result, err := svc.Create(context.Background(), CreateTicketInput{
		Title:   "Alpha",
		QueueID: 1,
		UserID:  1,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "202510050002", result.TicketNumber)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketService_CreatePersistsCustomerFields(t *testing.T) {
	t.Setenv("TEST_DB_DRIVER", "postgres")
	t.Setenv("DB_DRIVER", "postgres")
	database.ResetAdapterForTest()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repository.SetTicketNumberGenerator(stubGen{n: "202510050010"}, stubStore{})
	repo := repository.NewTicketRepository(db)
	svc := NewTicketService(repo)

	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM queue")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, type_id, valid_id,")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
			AddRow(1, "new", 1, 1, now, 1, now, 1))

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
		WithArgs(
			"202510050010",
			"Customer bound",
			1,
			1,
			nil,
			nil,
			nil,
			1,
			1,
			"cust-123",
			"user-789",
			1,
			3,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(91))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM ticket_history_type")).
		WithArgs("NewTicket").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(30))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ticket_history (")).
		WithArgs(
			"Ticket created (202510050010)",
			30,
			91,
			nil,
			1,
			1,
			1,
			3,
			1,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	result, err := svc.Create(context.Background(), CreateTicketInput{
		Title:          "Customer bound",
		QueueID:        1,
		UserID:         1,
		CustomerID:     "  cust-123  ",
		CustomerUserID: "user-789\n",
	})

	require.NoError(t, err)
	require.Equal(t, "cust-123", *result.CustomerID)
	require.Equal(t, "user-789", *result.CustomerUserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketService_CreateWritesInitialArticle(t *testing.T) {
	t.Setenv("TEST_DB_DRIVER", "postgres")
	t.Setenv("DB_DRIVER", "postgres")
	database.ResetAdapterForTest()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repository.SetTicketNumberGenerator(stubGen{n: "202510050099"}, stubStore{})
	repo := repository.NewTicketRepository(db)
	svc := NewTicketService(repo)
	articleRepo := &recordingArticleRepo{}
	svc.(*ticketService).articles = articleRepo

	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM queue")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, type_id, valid_id,")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
			AddRow(1, "new", 1, 1, now, 1, now, 1))

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
		WithArgs(
			"202510050099",
			"Article seed",
			1,
			1,
			nil,
			nil,
			nil,
			1,
			1,
			nil,
			nil,
			1,
			3,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(77))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM ticket_history_type")).
		WithArgs("NewTicket").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(30))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ticket_history (")).
		WithArgs(
			"Ticket created (202510050099)",
			30,
			77,
			nil,
			1,
			1,
			1,
			3,
			1,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	visible := false
	result, err := svc.Create(context.Background(), CreateTicketInput{
		Title:                         "Article seed",
		QueueID:                       1,
		UserID:                        1,
		Body:                          "  Hello world  ",
		ArticleIsVisibleForCustomer:   &visible,
		ArticleCommunicationChannelID: 2,
		ArticleMimeType:               "text/html",
		ArticleCharset:                "utf-16",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, articleRepo.articles, 1)
	article := articleRepo.articles[0]
	require.Equal(t, 77, article.TicketID)
	require.Equal(t, "Article seed", article.Subject)
	require.Equal(t, "Hello world", article.Body)
	require.Equal(t, constants.ArticleSenderAgent, article.SenderTypeID)
	require.Equal(t, constants.ArticleTypeEmailExternal, article.ArticleTypeID)
	require.Equal(t, 0, article.IsVisibleForCustomer)
	require.Equal(t, 2, article.CommunicationChannelID)
	require.Equal(t, "utf-16", article.Charset)
	require.Equal(t, "text/html; charset=utf-16", article.MimeType)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketService_CreateSkipsArticleWithoutBody(t *testing.T) {
	t.Setenv("TEST_DB_DRIVER", "postgres")
	t.Setenv("DB_DRIVER", "postgres")
	database.ResetAdapterForTest()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repository.SetTicketNumberGenerator(stubGen{n: "202510050123"}, stubStore{})
	repo := repository.NewTicketRepository(db)
	svc := NewTicketService(repo)
	articleRepo := &recordingArticleRepo{}
	svc.(*ticketService).articles = articleRepo

	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM queue")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, type_id, valid_id,")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
			AddRow(1, "new", 1, 1, now, 1, now, 1))

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
		WithArgs(
			"202510050123",
			"Skip article",
			1,
			1,
			nil,
			nil,
			nil,
			1,
			1,
			nil,
			nil,
			1,
			3,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(79))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM ticket_history_type")).
		WithArgs("NewTicket").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(30))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO ticket_history (")).
		WithArgs(
			"Ticket created (202510050123)",
			30,
			79,
			nil,
			1,
			1,
			1,
			3,
			1,
			sqlmock.AnyArg(),
			1,
			sqlmock.AnyArg(),
			1,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err = svc.Create(context.Background(), CreateTicketInput{
		Title:   "Skip article",
		QueueID: 1,
		UserID:  1,
		Body:    "   \n\t  ",
	})

	require.NoError(t, err)
	require.Empty(t, articleRepo.articles)
	require.NoError(t, mock.ExpectationsWereMet())
}

type recordingArticleRepo struct {
	articles []*models.Article
}

func (r *recordingArticleRepo) Create(article *models.Article) error {
	if article != nil {
		copied := *article
		r.articles = append(r.articles, &copied)
	}
	return nil
}
