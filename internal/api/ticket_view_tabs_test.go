package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTicketHistoryFragmentRendersData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	templateDir := filepath.Join(filepath.Dir(file), "..", "..", "templates")
	InitPongo2Renderer(templateDir)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	database.SetDB(db)
	defer database.ResetDB()

	rows := sqlmock.NewRows([]string{
		"id", "history_type", "name", "create_time", "login", "first_name", "last_name", "subject", "queue", "state", "priority",
	}).AddRow(
		10,
		"TicketCreate",
		"Ticket created",
		time.Date(2024, time.December, 24, 9, 30, 0, 0, time.UTC),
		"agent1",
		"Alice",
		"Agent",
		"Initial contact",
		"Raw",
		"open",
		"3 normal",
	)

	mock.ExpectQuery(`(?s)SELECT\s+th\.id.*FROM\s+ticket_history`).
		WithArgs(uint(1), 25).
		WillReturnRows(rows)

	router := gin.New()
	router.GET("/agent/tickets/:id/history", HandleTicketHistoryFragment)

	req, _ := http.NewRequest("GET", "/agent/tickets/1/history", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "TicketCreate — Ticket created")
	assert.Contains(t, body, "Alice Agent")
	assert.Contains(t, body, "Priority 3 normal")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketLinksFragmentRendersData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	templateDir := filepath.Join(filepath.Dir(file), "..", "..", "templates")
	InitPongo2Renderer(templateDir)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	database.SetDB(db)
	defer database.ResetDB()

	rows := sqlmock.NewRows([]string{
		"source_key", "target_key", "type_name", "state_name", "create_time", "login", "first_name", "last_name",
		"src_id", "src_tn", "src_title", "dst_id", "dst_tn", "dst_title",
	}).AddRow(
		"1",
		"2",
		"ParentChild",
		"Valid",
		time.Date(2024, time.December, 25, 8, 0, 0, 0, time.UTC),
		"agent1",
		"Alice",
		"Agent",
		int64(1),
		"10001",
		"Parent ticket",
		int64(2),
		"10002",
		"Child ticket",
	)

	mock.ExpectQuery(`(?s)SELECT\s+lr\.source_key.*FROM\s+link_relation`).
		WithArgs("1", 25).
		WillReturnRows(rows)

	router := gin.New()
	router.GET("/agent/tickets/:id/links", HandleTicketLinksFragment)

	req, _ := http.NewRequest("GET", "/agent/tickets/1/links", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "10002 — Child ticket")
	assert.Contains(t, body, "ParentChild (Outbound)")
	assert.Contains(t, body, "Valid • Alice Agent")

	assert.NoError(t, mock.ExpectationsWereMet())
}
