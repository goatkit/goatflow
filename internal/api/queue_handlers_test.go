package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetQueues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful get queues",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "comments", "valid_id", "ticket_count",
				}).
					AddRow(1, "Support", "General support queue", 1, 5).
					AddRow(2, "Sales", "Sales inquiries", 2, 0)

				mock.ExpectQuery(`SELECT q.id, q.name, q.comments, q.valid_id, COALESCE\(tc.ticket_count, 0\)`).
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": []interface{}{
					map[string]interface{}{
						"id":           float64(1),
						"name":         "Support",
						"comment":      "General support queue",
						"ticket_count": float64(5),
						"status":       "active",
					},
					map[string]interface{}{
						"id":           float64(2),
						"name":         "Sales",
						"comment":      "Sales inquiries",
						"ticket_count": float64(0),
						"status":       "inactive",
					},
				},
			},
		},
		{
			name: "database error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT q.id, q.name, q.comments, q.valid_id, COALESCE\(tc.ticket_count, 0\)`).
					WillReturnError(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Failed to fetch queues",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			database.SetDB(db)
			defer database.ResetDB()

			tt.setupMock(mock)

			router := gin.New()
			router.GET("/api/queues", handleGetQueuesAPI)

			req, _ := http.NewRequest("GET", "/api/queues", nil)
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody["success"], response["success"])

			if tt.expectedBody["error"] != nil {
				assert.Equal(t, tt.expectedBody["error"], response["error"])
			}

			if tt.expectedBody["data"] != nil {
				assert.Equal(t, tt.expectedBody["data"], response["data"])
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCreateQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		body           map[string]interface{}
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful create queue",
			body: map[string]interface{}{
				"name":              "Technical Support",
				"group_id":          1,
				"comments":          "Technical support queue",
				"unlock_timeout":    45,
				"follow_up_id":      1,
				"follow_up_lock":    0,
				"system_address_id": 1,
				"salutation_id":     1,
				"signature_id":      1,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO queue`).
					WithArgs(
						"Technical Support", 1, sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), 45, 1, 0, "Technical support queue", 1, 1, 1,
					).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
			},
			expectedStatus: http.StatusCreated,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":                float64(3),
					"name":              "Technical Support",
					"group_id":          float64(1),
					"comments":          "Technical support queue",
					"unlock_timeout":    float64(45),
					"follow_up_id":      float64(1),
					"follow_up_lock":    float64(0),
					"system_address_id": float64(1),
					"salutation_id":     float64(1),
					"signature_id":      float64(1),
					"valid_id":          float64(1),
				},
			},
		},
		{
			name: "missing required fields",
			body: map[string]interface{}{
				"comments": "Test queue",
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Name and group_id are required",
			},
		},
		{
			name: "duplicate queue name",
			body: map[string]interface{}{
				"name":     "Support",
				"group_id": 1,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO queue`).
					WithArgs(
						"Support", 1, sqlmock.AnyArg(), sqlmock.AnyArg(),
						sqlmock.AnyArg(), 0, 1, 0, sqlmock.AnyArg(), 1, 1, 1,
					).
					WillReturnError(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Failed to create queue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			database.SetDB(db)
			defer database.ResetDB()

			tt.setupMock(mock)

			router := gin.New()
			router.POST("/api/queues", handleCreateQueue)

			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("POST", "/api/queues", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody["success"], response["success"])

			if tt.expectedBody["error"] != nil {
				assert.Equal(t, tt.expectedBody["error"], response["error"])
			}

			if tt.expectedBody["data"] != nil {
				assert.Equal(t, tt.expectedBody["data"], response["data"])
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpdateQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		body           map[string]interface{}
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:    "successful update queue",
			queueID: "1",
			body: map[string]interface{}{
				"name":           "Support Team",
				"comments":       "Updated support queue",
				"unlock_timeout": 60,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE queue SET`).
					WithArgs(1, "Support Team", "Updated support queue", 60, 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":             float64(1),
					"name":           "Support Team",
					"comments":       "Updated support queue",
					"unlock_timeout": float64(60),
				},
			},
		},
		{
			name:    "invalid queue ID",
			queueID: "abc",
			body: map[string]interface{}{
				"name": "Test",
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Invalid queue ID",
			},
		},
		{
			name:    "queue not found",
			queueID: "999",
			body: map[string]interface{}{
				"name": "Test Queue",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE queue SET`).
					WithArgs(1, "Test Queue", 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Queue not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			database.SetDB(db)
			defer database.ResetDB()

			tt.setupMock(mock)

			router := gin.New()
			router.PUT("/api/queues/:id", handleUpdateQueue)

			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("PUT", "/api/queues/"+tt.queueID, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody["success"], response["success"])

			if tt.expectedBody["error"] != nil {
				assert.Equal(t, tt.expectedBody["error"], response["error"])
			}

			if tt.expectedBody["data"] != nil {
				expectedData := tt.expectedBody["data"].(map[string]interface{})
				responseData := response["data"].(map[string]interface{})
				assert.Equal(t, expectedData["id"], responseData["id"])
				assert.Equal(t, expectedData["name"], responseData["name"])
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDeleteQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:    "successful delete queue (soft delete)",
			queueID: "2",
			setupMock: func(mock sqlmock.Sqlmock) {
				// Check for tickets in queue
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ticket WHERE queue_id = \$1`).
					WithArgs(2).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

				// Soft delete
				mock.ExpectExec(`UPDATE queue SET valid_id = 2`).
					WithArgs(2, 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"message": "Queue deleted successfully",
			},
		},
		{
			name:    "queue has tickets",
			queueID: "1",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ticket WHERE queue_id = \$1`).
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Cannot delete queue with existing tickets",
			},
		},
		{
			name:           "invalid queue ID",
			queueID:        "xyz",
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Invalid queue ID",
			},
		},
		{
			name:    "queue not found",
			queueID: "999",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ticket WHERE queue_id = \$1`).
					WithArgs(999).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

				mock.ExpectExec(`UPDATE queue SET valid_id = 2`).
					WithArgs(999, 1).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Queue not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			database.SetDB(db)
			defer database.ResetDB()

			tt.setupMock(mock)

			router := gin.New()
			router.DELETE("/api/queues/:id", handleDeleteQueue)

			req, _ := http.NewRequest("DELETE", "/api/queues/"+tt.queueID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody["success"], response["success"])

			if tt.expectedBody["error"] != nil {
				assert.Equal(t, tt.expectedBody["error"], response["error"])
			}

			if tt.expectedBody["message"] != nil {
				assert.Equal(t, tt.expectedBody["message"], response["message"])
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetQueueDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		hasData        bool
	}{
		{
			name:    "successful get queue details",
			queueID: "1",
			setupMock: func(mock sqlmock.Sqlmock) {
				// Get queue details
				queueRows := sqlmock.NewRows([]string{
					"id", "name", "group_id", "system_address_id", "salutation_id",
					"signature_id", "unlock_timeout", "follow_up_id", "follow_up_lock",
					"comments", "valid_id", "group_name",
				}).AddRow(1, "Support", 1, 1, 1, 1, 30, 1, 0, "General support", 1, "users")

				mock.ExpectQuery(`SELECT q.*, g.name as group_name FROM queue q`).
					WithArgs(1).
					WillReturnRows(queueRows)

				// Get ticket count
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ticket WHERE queue_id = \$1`).
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))

				// Get open tickets
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ticket WHERE queue_id = \$1 AND ticket_state_id IN`).
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

				// Get agent count
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT user_id\) FROM user_groups WHERE group_id = \$1`).
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
			},
			expectedStatus: http.StatusOK,
			hasData:        true,
		},
		{
			name:    "queue not found",
			queueID: "999",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT q.*, g.name as group_name FROM queue q`).
					WithArgs(999).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "name", "group_id", "system_address_id", "salutation_id",
						"signature_id", "unlock_timeout", "follow_up_id", "follow_up_lock",
						"comments", "valid_id", "group_name",
					}))
			},
			expectedStatus: http.StatusNotFound,
			hasData:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			database.SetDB(db)
			defer database.ResetDB()

			tt.setupMock(mock)

			router := gin.New()
			router.GET("/api/queues/:id/details", handleGetQueueDetails)

			req, _ := http.NewRequest("GET", "/api/queues/"+tt.queueID+"/details", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.hasData {
				assert.True(t, response["success"].(bool))
				assert.NotNil(t, response["data"])
			} else {
				assert.False(t, response["success"].(bool))
				assert.NotNil(t, response["error"])
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
