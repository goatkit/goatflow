package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/require"
)

func TestHandleUpdateTicketStatus_PendingRequiresUntil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	defer mockDB.Close()

	database.SetDB(mockDB)
	t.Cleanup(func() { database.SetDB(nil) })

	stateRows := sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(7, "pending auto close+", 5, 1, time.Now(), 1, time.Now(), 1)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, type_id, valid_id,
	   create_time, create_by, change_time, change_by
FROM ticket_state
WHERE id = $1`)).
		WithArgs(7).
		WillReturnRows(stateRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", uint(24))
	c.Params = []gin.Param{{Key: "id", Value: "123"}}

	form := url.Values{}
	form.Set("status", "7")

	req := httptest.NewRequest(http.MethodPost, "/api/tickets/123/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req

	handleUpdateTicketStatus(c)

	require.Equal(t, http.StatusBadRequest, w.Code)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleUpdateTicketStatus_PendingSetsUntil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	defer mockDB.Close()

	database.SetDB(mockDB)
	t.Cleanup(func() { database.SetDB(nil) })

	stateRows := sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(7, "pending auto close+", 5, 1, time.Now(), 1, time.Now(), 1)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, type_id, valid_id,
	   create_time, create_by, change_time, change_by
FROM ticket_state
WHERE id = $1`)).
		WithArgs(7).
		WillReturnRows(stateRows)

	pendingUntil := "2025-10-18T15:30"
	pendingUnix := parsePendingUntil(pendingUntil)

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE ticket
SET ticket_state_id = $1,
	until_time = $2,
	change_time = CURRENT_TIMESTAMP,
	change_by = $3
WHERE id = $4`)).
		WithArgs(7, pendingUnix, 42, 123).
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", uint(42))
	c.Params = []gin.Param{{Key: "id", Value: "123"}}

	form := url.Values{}
	form.Set("status", "7")
	form.Set("pending_until", pendingUntil)

	req := httptest.NewRequest(http.MethodPost, "/api/tickets/123/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req

	handleUpdateTicketStatus(c)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, float64(7), resp["status"])
	require.Equal(t, pendingUntil, resp["pending_until"])

	require.NoError(t, mock.ExpectationsWereMet())
}
