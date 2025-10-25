package api

import (
	"encoding/json"
	"fmt"
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

func TestHandleUpdateTicketStatus_PendingReminderRequiresUntil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock setup failed: %v", err)
	}
	defer mockDB.Close()

	database.SetDB(mockDB)
	t.Cleanup(func() { database.SetDB(nil) })

	stateRows := sqlmock.NewRows([]string{"id", "name", "type_id", "valid_id", "create_time", "create_by", "change_time", "change_by"}).
		AddRow(4, "pending reminder", 4, 1, time.Now(), 1, time.Now(), 1)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, type_id, valid_id,
   create_time, create_by, change_time, change_by
FROM ticket_state
WHERE id = $1`)).
		WithArgs(4).
		WillReturnRows(stateRows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", uint(9))
	c.Params = []gin.Param{{Key: "id", Value: "123"}}

	form := url.Values{}
	form.Set("status", "4")

	req := httptest.NewRequest(http.MethodPost, "/api/tickets/123/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req

	handleUpdateTicketStatus(c)

	require.Equal(t, http.StatusBadRequest, w.Code)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleUpdateTicketStatus_PendingSetsUntil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DB_DRIVER", "postgres")

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

	ticketSnapshotQuery := fmt.Sprintf(`SELECT
		t.id, t.tn, t.title, t.queue_id, t.ticket_lock_id, %s AS type_id,
		t.service_id, t.sla_id, t.user_id, t.responsible_user_id,
		t.customer_id, t.customer_user_id, t.ticket_state_id,
		t.ticket_priority_id, t.until_time, t.escalation_time,
		t.escalation_update_time, t.escalation_response_time,
		t.escalation_solution_time, t.archive_flag,
		t.create_time, t.create_by, t.change_time, t.change_by
	FROM ticket t
	WHERE t.id = $1`, database.QualifiedTicketTypeColumn("t"))

	mock.ExpectQuery(regexp.QuoteMeta(ticketSnapshotQuery)).
		WithArgs(uint(123)).
		WillReturnError(fmt.Errorf("mock pre-snapshot failure"))

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

	mock.ExpectQuery(regexp.QuoteMeta(ticketSnapshotQuery)).
		WithArgs(uint(123)).
		WillReturnError(fmt.Errorf("mock post-snapshot failure"))

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
