package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
    "github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
)

type stubGen struct{ n string }
func (g stubGen) Name() string { return "Date" }
func (g stubGen) IsDateBased() bool { return true }
func (g stubGen) Next(ctx context.Context, store ticketnumber.CounterStore) (string, error) { return g.n, nil }

// minimal counter store implementing ticketnumber.CounterStore
type stubStore struct{}
func (stubStore) Add(ctx context.Context, dateScoped bool, offset int64) (int64, error) { return 1, nil }

// prepareRouter minimal for handler
func setupCreateRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// auth shim for tests
	r.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	r.POST("/api/tickets", HandleCreateTicketAPI)
	return r
}

func injectGenerator() {
	repository.SetTicketNumberGenerator(stubGen{n: "202510050001"}, stubStore{})
}

func TestCreateTicketAPI_HappyPath(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil { t.Fatalf("sqlmock: %v", err) }
	defer mockDB.Close()
	database.SetDB(mockDB)
	injectGenerator()

	// queue existence check
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM queue")).
		WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	// insert ticket returning id
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO ticket (")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(77))

	payload := map[string]interface{}{"title":"Alpha","queue_id":1}
	b,_ := json.Marshal(payload)
	r := setupCreateRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/tickets", bytes.NewReader(b))
	req.Header.Set("Content-Type","application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated { t.Fatalf("expected 201 got %d body=%s", w.Code, w.Body.String()) }
	if !regexp.MustCompile(`"tn":"202510050001"`).Match(w.Body.Bytes()) { t.Fatalf("expected tn present body=%s", w.Body.String()) }
	if err := mock.ExpectationsWereMet(); err != nil { t.Fatalf("unmet expectations: %v", err) }
}

func TestCreateTicketAPI_InvalidQueue(t *testing.T) {
	mockDB, mock, _ := sqlmock.New(); defer mockDB.Close(); database.SetDB(mockDB); injectGenerator()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM queue")).
		WithArgs(999).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	payload := map[string]interface{}{"title":"Alpha","queue_id":999}
	b,_ := json.Marshal(payload)
	r := setupCreateRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tickets", bytes.NewReader(b))
	req.Header.Set("Content-Type","application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest { t.Fatalf("expected 400 got %d", w.Code) }
}

func TestCreateTicketAPI_MissingTitle(t *testing.T) {
	mockDB, _, _ := sqlmock.New(); defer mockDB.Close(); database.SetDB(mockDB)
	injectGenerator()
	payload := map[string]interface{}{"queue_id":1}
	b,_ := json.Marshal(payload)
	r := setupCreateRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tickets", bytes.NewReader(b))
	req.Header.Set("Content-Type","application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest { t.Fatalf("expected 400 got %d body=%s", w.Code, w.Body.String()) }
}

func TestCreateTicketAPI_TitleTooLong(t *testing.T) {
	mockDB, _, _ := sqlmock.New(); defer mockDB.Close(); database.SetDB(mockDB); injectGenerator()
	long := make([]byte, 256)
	for i := range long { long[i] = 'a' }
	payload := map[string]interface{}{"title":string(long),"queue_id":1}
	b,_ := json.Marshal(payload)
	r := setupCreateRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tickets", bytes.NewReader(b))
	req.Header.Set("Content-Type","application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest { t.Fatalf("expected 400 got %d", w.Code) }
}

func TestCreateTicketAPI_DBUnavailable(t *testing.T) {
	// Ensure DB nil triggers 503 path
	database.ResetDB() // nil DB
	injectGenerator()
	payload := map[string]interface{}{"title":"Alpha","queue_id":1}
	b,_ := json.Marshal(payload)
	r := setupCreateRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tickets", bytes.NewReader(b))
	req.Header.Set("Content-Type","application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable { t.Fatalf("expected 503 got %d", w.Code) }
}
