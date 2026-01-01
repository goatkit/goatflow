package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestUpdateTicket_BasicFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		ticketID   string
		payload    map[string]interface{}
		wantStatus int
		checkBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:     "update title",
			ticketID: "1",
			payload: map[string]interface{}{
				"title": "Updated ticket title",
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, "Updated ticket title", data["title"])
			},
		},
		{
			name:     "update priority",
			ticketID: "1",
			payload: map[string]interface{}{
				"priority_id": 5,
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, float64(5), data["priority_id"])
			},
		},
		{
			name:     "update queue",
			ticketID: "1",
			payload: map[string]interface{}{
				"queue_id": 2,
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, float64(2), data["queue_id"])
			},
		},
		{
			name:     "update state",
			ticketID: "1",
			payload: map[string]interface{}{
				"state_id": 2, // closed successful
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, float64(2), data["state_id"])
			},
		},
		{
			name:     "update type",
			ticketID: "1",
			payload: map[string]interface{}{
				"type_id": 2,
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, float64(2), data["type_id"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/v1/tickets/:id", func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("is_authenticated", true)
				HandleUpdateTicketAPI(c)
			})

			mock := setupTicketUpdateMocks(t, tt.ticketID, tt.payload, 1, ticketMockConfig{})

			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest("PUT", "/api/v1/tickets/"+tt.ticketID, bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, response)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpdateTicket_Assignment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		ticketID   string
		payload    map[string]interface{}
		wantStatus int
		checkBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:     "assign to user",
			ticketID: "1",
			payload: map[string]interface{}{
				"responsible_user_id": 5,
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, float64(5), data["responsible_user_id"])
			},
		},
		{
			name:     "change owner",
			ticketID: "1",
			payload: map[string]interface{}{
				"user_id": 3,
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, float64(3), data["user_id"])
			},
		},
		{
			name:     "unassign ticket",
			ticketID: "1",
			payload: map[string]interface{}{
				"responsible_user_id": nil,
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Nil(t, data["responsible_user_id"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/v1/tickets/:id", func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("is_authenticated", true)
				HandleUpdateTicketAPI(c)
			})

			mock := setupTicketUpdateMocks(t, tt.ticketID, tt.payload, 1, ticketMockConfig{})

			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest("PUT", "/api/v1/tickets/"+tt.ticketID, bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, response)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpdateTicket_CustomerInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		ticketID   string
		payload    map[string]interface{}
		wantStatus int
		checkBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:     "update customer user",
			ticketID: "1",
			payload: map[string]interface{}{
				"customer_user_id": "newcustomer@example.com",
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, "newcustomer@example.com", data["customer_user_id"])
			},
		},
		{
			name:     "update customer company",
			ticketID: "1",
			payload: map[string]interface{}{
				"customer_id": "company123",
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, "company123", data["customer_id"])
			},
		},
		{
			name:     "clear customer info",
			ticketID: "1",
			payload: map[string]interface{}{
				"customer_user_id": "",
				"customer_id":      "",
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, "", data["customer_user_id"])
				assert.Equal(t, "", data["customer_id"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/v1/tickets/:id", func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("is_authenticated", true)
				HandleUpdateTicketAPI(c)
			})

			mock := setupTicketUpdateMocks(t, tt.ticketID, tt.payload, 1, ticketMockConfig{})

			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest("PUT", "/api/v1/tickets/"+tt.ticketID, bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, response)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpdateTicket_MultipleFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := map[string]interface{}{
		"title":               "Completely updated ticket",
		"priority_id":         4,
		"queue_id":            3,
		"state_id":            4, // open
		"responsible_user_id": 2,
		"customer_user_id":    "updated@customer.com",
	}

	router := gin.New()
	router.PUT("/api/v1/tickets/:id", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleUpdateTicketAPI(c)
	})

	mock := setupTicketUpdateMocks(t, "1", payload, 1, ticketMockConfig{})

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("PUT", "/api/v1/tickets/1", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "Completely updated ticket", data["title"])
	assert.Equal(t, float64(4), data["priority_id"])
	assert.Equal(t, float64(3), data["queue_id"])
	assert.Equal(t, float64(4), data["state_id"])
	assert.Equal(t, float64(2), data["responsible_user_id"])
	assert.Equal(t, "updated@customer.com", data["customer_user_id"])

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateTicket_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")
	router := gin.New()

	router.PUT("/api/v1/tickets/:id", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleUpdateTicketAPI(c)
	})

	tests := []struct {
		name       string
		ticketID   string
		payload    interface{}
		wantStatus int
		wantError  string
	}{
		{
			name:       "invalid ticket ID",
			ticketID:   "abc",
			payload:    map[string]interface{}{"title": "Test"},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid ticket ID",
		},
		{
			name:       "non-existent ticket",
			ticketID:   "999999",
			payload:    map[string]interface{}{"title": "Test"},
			wantStatus: http.StatusNotFound,
			wantError:  "Ticket not found",
		},
		{
			name:       "invalid JSON",
			ticketID:   "1",
			payload:    "not json",
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid request",
		},
		{
			name:       "empty payload",
			ticketID:   "1",
			payload:    map[string]interface{}{},
			wantStatus: http.StatusBadRequest,
			wantError:  "No fields to update",
		},
		{
			name:     "invalid queue_id",
			ticketID: "1",
			payload: map[string]interface{}{
				"queue_id": 99999,
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid queue_id",
		},
		{
			name:     "invalid state_id",
			ticketID: "1",
			payload: map[string]interface{}{
				"state_id": 99999,
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid state_id",
		},
		{
			name:     "invalid priority_id",
			ticketID: "1",
			payload: map[string]interface{}{
				"priority_id": 99999,
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid priority_id",
		},
		{
			name:     "title too long",
			ticketID: "1",
			payload: map[string]interface{}{
				"title": string(make([]byte, 256)),
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Title too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var jsonData []byte
			var err error

			if str, ok := tt.payload.(string); ok {
				jsonData = []byte(str)
			} else {
				jsonData, err = json.Marshal(tt.payload)
				require.NoError(t, err)
			}

			req := httptest.NewRequest("PUT", "/api/v1/tickets/"+tt.ticketID, bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.False(t, response["success"].(bool))
			assert.Contains(t, response["error"].(string), tt.wantError)
		})
	}
}

func TestUpdateTicket_Permissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		setupAuth  func(c *gin.Context)
		ticketID   string
		payload    map[string]interface{}
		wantStatus int
		useDB      bool
	}{
		{
			name: "authenticated user can update",
			setupAuth: func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("is_authenticated", true)
			},
			ticketID: "1",
			payload: map[string]interface{}{
				"title": "Updated by authenticated user",
			},
			wantStatus: http.StatusOK,
			useDB:      true,
		},
		{
			name: "unauthenticated user cannot update",
			setupAuth: func(c *gin.Context) {
				// No auth set
			},
			ticketID: "1",
			payload: map[string]interface{}{
				"title": "Should fail",
			},
			wantStatus: http.StatusUnauthorized,
			useDB:      false,
		},
		{
			name: "customer can only update own tickets",
			setupAuth: func(c *gin.Context) {
				c.Set("user_id", 100)
				c.Set("is_customer", true)
				c.Set("customer_email", "customer@example.com")
				c.Set("is_authenticated", true)
			},
			ticketID: "1",
			payload: map[string]interface{}{
				"title": "Customer update",
			},
			wantStatus: http.StatusForbidden,
			useDB:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database.ResetDB()
			t.Setenv("APP_ENV", "test")

			router := gin.New()
			router.PUT("/api/v1/tickets/:id", func(c *gin.Context) {
				tt.setupAuth(c)
				HandleUpdateTicketAPI(c)
			})

			var mock sqlmock.Sqlmock
			if tt.useDB {
				mock = setupTicketUpdateMocks(t, tt.ticketID, tt.payload, 1, ticketMockConfig{})
			}

			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest("PUT", "/api/v1/tickets/"+tt.ticketID, bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.useDB {
				require.NoError(t, mock.ExpectationsWereMet())
			}
		})
	}
}

func TestUpdateTicket_LockStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		ticketID   string
		payload    map[string]interface{}
		wantStatus int
		checkBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:     "lock ticket",
			ticketID: "1",
			payload: map[string]interface{}{
				"ticket_lock_id": 2, // locked
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, float64(2), data["ticket_lock_id"])
			},
		},
		{
			name:     "unlock ticket",
			ticketID: "1",
			payload: map[string]interface{}{
				"ticket_lock_id": 1, // unlocked
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, float64(1), data["ticket_lock_id"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/v1/tickets/:id", func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("is_authenticated", true)
				HandleUpdateTicketAPI(c)
			})

			mock := setupTicketUpdateMocks(t, tt.ticketID, tt.payload, 1, ticketMockConfig{})

			jsonData, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest("PUT", "/api/v1/tickets/"+tt.ticketID, bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, response)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpdateTicket_AuditFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/tickets/:id", func(c *gin.Context) {
		c.Set("user_id", 5)
		c.Set("is_authenticated", true)
		HandleUpdateTicketAPI(c)
	})

	payload := map[string]interface{}{
		"title": "Check audit fields",
	}

	mock := setupTicketUpdateMocks(t, "1", payload, 5, ticketMockConfig{})

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest("PUT", "/api/v1/tickets/1", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})

	// Check that change_by is updated to current user
	assert.Equal(t, float64(5), data["change_by"])

	// Check that change_time is updated (should be recent)
	assert.NotNil(t, data["change_time"])

	require.NoError(t, mock.ExpectationsWereMet())
}

type ticketMockConfig struct{}

type ticketRecord struct {
	id                int
	tn                string
	title             string
	queueID           int
	typeID            int
	stateID           int
	priorityID        int
	customerUserID    *string
	customerID        *string
	userID            int
	responsibleUserID *int
	ticketLockID      int
	createTime        time.Time
	createBy          int
	changeTime        time.Time
	changeBy          int
}

func setupTicketUpdateMocks(t *testing.T, ticketID string, payload map[string]interface{}, changeBy int, _ ticketMockConfig) sqlmock.Sqlmock {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("TEST_DB_DRIVER", "postgres")

	mockDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	database.SetDB(mockDB)
	t.Cleanup(func() {
		database.ResetDB()
		mockDB.Close()
	})

	id, err := strconv.Atoi(ticketID)
	require.NoError(t, err)

	customerUser := "customer@example.com"
	customerID := "ACME"
	responsible := 2
	record := ticketRecord{
		id:                id,
		tn:                fmt.Sprintf("202500%03d", id),
		title:             "Original Ticket",
		queueID:           1,
		typeID:            1,
		stateID:           1,
		priorityID:        3,
		customerUserID:    &customerUser,
		customerID:        &customerID,
		userID:            1,
		responsibleUserID: &responsible,
		ticketLockID:      1,
		createTime:        time.Now().Add(-2 * time.Hour).UTC(),
		createBy:          1,
		changeTime:        time.Now().UTC(),
		changeBy:          changeBy,
	}

	var preCustomerVal interface{}
	if record.customerUserID != nil {
		preCustomerVal = *record.customerUserID
	}
	preUserID := record.userID

	applyTicketPayload(&record, payload, changeBy)

	ticketByIDQuery := regexp.QuoteMeta(database.ConvertPlaceholders(
		"SELECT id, customer_user_id, user_id FROM ticket WHERE id = $1",
	))
	firstRows := sqlmock.NewRows([]string{"id", "customer_user_id", "user_id"}).AddRow(int64(id), preCustomerVal, preUserID)
	mock.ExpectQuery(ticketByIDQuery).WithArgs(id).WillReturnRows(firstRows)

	if queueVal, ok := payload["queue_id"]; ok && queueVal != nil {
		if queueID, ok := toInt(queueVal); ok {
			queueQuery := regexp.QuoteMeta(database.ConvertPlaceholders(
				"SELECT EXISTS(SELECT 1 FROM queue WHERE id = $1 AND valid_id = 1)",
			))
			mock.ExpectQuery(queueQuery).WithArgs(queueID).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		}
	}
	if stateVal, ok := payload["state_id"]; ok && stateVal != nil {
		if stateID, ok := toInt(stateVal); ok {
			stateQuery := regexp.QuoteMeta(database.ConvertPlaceholders(
				"SELECT EXISTS(SELECT 1 FROM ticket_state WHERE id = $1 AND valid_id = 1)",
			))
			mock.ExpectQuery(stateQuery).WithArgs(stateID).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		}
	}
	if priorityVal, ok := payload["priority_id"]; ok && priorityVal != nil {
		if priorityID, ok := toInt(priorityVal); ok {
			priorityQuery := regexp.QuoteMeta(database.ConvertPlaceholders(
				"SELECT EXISTS(SELECT 1 FROM ticket_priority WHERE id = $1 AND valid_id = 1)",
			))
			mock.ExpectQuery(priorityQuery).WithArgs(priorityID).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		}
	}
	if typeVal, ok := payload["type_id"]; ok && typeVal != nil {
		if typeID, ok := toInt(typeVal); ok {
			typeQuery := regexp.QuoteMeta(database.ConvertPlaceholders(
				"SELECT EXISTS(SELECT 1 FROM ticket_type WHERE id = $1 AND valid_id = 1)",
			))
			mock.ExpectQuery(typeQuery).WithArgs(typeID).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		}
	}

	mock.ExpectExec(`^UPDATE\s+ticket\s+SET.*`).WillReturnResult(sqlmock.NewResult(0, 1))

	typeSelect := fmt.Sprintf("%s AS type_id", database.QualifiedTicketTypeColumn("t"))
	selectQuery := database.ConvertPlaceholders(fmt.Sprintf(`
		SELECT
			t.id,
			t.tn,
			t.title,
			t.queue_id,
			%s,
			t.ticket_state_id AS state_id,
			t.ticket_priority_id AS priority_id,
			t.customer_user_id,
			t.customer_id,
			t.user_id,
			t.responsible_user_id,
			t.ticket_lock_id,
			t.create_time,
			t.create_by,
			t.change_time,
			t.change_by
		FROM ticket t
		WHERE t.id = $1
	`, typeSelect))

	finalCustomer := interface{}(nil)
	if record.customerUserID != nil {
		finalCustomer = *record.customerUserID
	}
	finalCustomerID := interface{}(nil)
	if record.customerID != nil {
		finalCustomerID = *record.customerID
	}
	finalResponsible := interface{}(nil)
	if record.responsibleUserID != nil {
		finalResponsible = *record.responsibleUserID
	}

	rows := sqlmock.NewRows([]string{
		"id",
		"tn",
		"title",
		"queue_id",
		"type_id",
		"state_id",
		"priority_id",
		"customer_user_id",
		"customer_id",
		"user_id",
		"responsible_user_id",
		"ticket_lock_id",
		"create_time",
		"create_by",
		"change_time",
		"change_by",
	}).AddRow(
		int64(record.id),
		record.tn,
		record.title,
		record.queueID,
		record.typeID,
		record.stateID,
		record.priorityID,
		finalCustomer,
		finalCustomerID,
		record.userID,
		finalResponsible,
		record.ticketLockID,
		record.createTime,
		record.createBy,
		record.changeTime,
		record.changeBy,
	)

	mock.ExpectQuery(regexp.QuoteMeta(selectQuery)).WithArgs(id).WillReturnRows(rows)

	return mock
}

func applyTicketPayload(record *ticketRecord, payload map[string]interface{}, changeBy int) {
	record.changeBy = changeBy
	record.changeTime = time.Now().UTC()

	for field, value := range payload {
		switch field {
		case "title":
			if v, ok := value.(string); ok {
				record.title = v
			}
		case "queue_id":
			if v, ok := toInt(value); ok {
				record.queueID = v
			}
		case "type_id":
			if v, ok := toInt(value); ok {
				record.typeID = v
			}
		case "state_id":
			if v, ok := toInt(value); ok {
				record.stateID = v
			}
		case "priority_id":
			if v, ok := toInt(value); ok {
				record.priorityID = v
			}
		case "customer_user_id":
			if v, ok := value.(string); ok {
				record.customerUserID = &v
			}
		case "customer_id":
			if v, ok := value.(string); ok {
				record.customerID = &v
			}
		case "user_id":
			if v, ok := toInt(value); ok {
				record.userID = v
			}
		case "responsible_user_id":
			if value == nil {
				record.responsibleUserID = nil
			} else if v, ok := toInt(value); ok {
				record.responsibleUserID = &v
			}
		case "ticket_lock_id":
			if v, ok := toInt(value); ok {
				record.ticketLockID = v
			}
		}
	}
}

func toInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}
