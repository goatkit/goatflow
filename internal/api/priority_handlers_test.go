package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestGetPriorities(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful get priorities",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "color", "valid_id"}).
					AddRow(1, "1 very low", "#03c4f0", 1).
					AddRow(2, "2 low", "#83bfc8", 1).
					AddRow(3, "3 normal", "#cdcdcd", 1).
					AddRow(4, "4 high", "#ffaaaa", 1).
					AddRow(5, "5 very high", "#ff505e", 1)
				mock.ExpectQuery("SELECT id, name, color, valid_id FROM ticket_priority").
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": []interface{}{
					map[string]interface{}{"id": float64(1), "name": "1 very low", "color": "#03c4f0", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(2), "name": "2 low", "color": "#83bfc8", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(3), "name": "3 normal", "color": "#cdcdcd", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(4), "name": "4 high", "color": "#ffaaaa", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(5), "name": "5 very high", "color": "#ff505e", "valid_id": float64(1)},
				},
			},
		},
		{
			name: "database error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, name, color, valid_id FROM ticket_priority").
					WillReturnError(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Failed to fetch priorities",
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
			router.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
			router.GET("/api/priorities", HandleListPrioritiesAPI)

			req, _ := http.NewRequest("GET", "/api/priorities", nil)
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

func TestCreatePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		body           map[string]interface{}
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful create priority",
			body: map[string]interface{}{
				"name":  "6 critical",
				"color": "#ff00ff",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ticket_priority`).
					WithArgs("6 critical").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

				if database.IsMySQL() {
					mock.ExpectExec(`INSERT INTO ticket_priority`).
						WithArgs("6 critical", "#ff00ff", 1, 1, 1).
						WillReturnResult(sqlmock.NewResult(6, 1))
				} else {
					mock.ExpectQuery(`INSERT INTO ticket_priority`).
						WithArgs("6 critical", "#ff00ff", 1, 1, 1).
						WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(6))
				}
			},
			expectedStatus: http.StatusCreated,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":       float64(6),
					"name":     "6 critical",
					"color":    "#ff00ff",
					"valid_id": float64(1),
				},
			},
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"color": "#ff00ff",
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Name is required",
			},
		},
		{
			name: "duplicate priority",
			body: map[string]interface{}{
				"name":  "3 normal",
				"color": "#cdcdcd",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ticket_priority`).
					WithArgs("3 normal").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

				if database.IsMySQL() {
					mock.ExpectExec(`INSERT INTO ticket_priority`).
						WithArgs("3 normal", "#cdcdcd", 1, 1, 1).
						WillReturnError(assert.AnError)
				} else {
					mock.ExpectQuery(`INSERT INTO ticket_priority`).
						WithArgs("3 normal", "#cdcdcd", 1, 1, 1).
						WillReturnError(assert.AnError)
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Failed to create priority",
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
			// Auth shim for handlers requiring user_id
			router.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
			router.POST("/api/priorities", HandleCreatePriorityAPI)

			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("POST", "/api/priorities", bytes.NewBuffer(body))
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

func TestUpdatePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		priorityID     string
		body           map[string]interface{}
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:       "successful update priority",
			priorityID: "3",
			body: map[string]interface{}{
				"name":  "3 medium",
				"color": "#cdcdcd",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_priority SET`).
					WithArgs("3 medium", "#cdcdcd", 1, 3).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":       float64(3),
					"name":     "3 medium",
					"color":    "#cdcdcd",
					"valid_id": float64(1),
				},
			},
		},
		{
			name:       "invalid priority ID",
			priorityID: "abc",
			body: map[string]interface{}{
				"name": "test",
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Invalid priority ID",
			},
		},
		{
			name:       "priority not found",
			priorityID: "999",
			body: map[string]interface{}{
				"name": "test",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT color FROM ticket_priority WHERE id`).
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Priority not found",
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
			router.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
			router.PUT("/api/priorities/:id", HandleUpdatePriorityAPI)

			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("PUT", "/api/priorities/"+tt.priorityID, bytes.NewBuffer(body))
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
				assert.Equal(t, expectedData["color"], responseData["color"])
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDeletePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		priorityID     string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:       "successful delete priority (soft delete)",
			priorityID: "5",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_priority SET valid_id = 2`).
					WithArgs(1, 5).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"message": "Priority deleted successfully",
			},
		},
		{
			name:           "invalid priority ID",
			priorityID:     "xyz",
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Invalid priority ID",
			},
		},
		{
			name:       "priority not found",
			priorityID: "999",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_priority SET valid_id = 2`).
					WithArgs(1, 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Priority not found",
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
			router.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
			router.DELETE("/api/priorities/:id", HandleDeletePriorityAPI)

			req, _ := http.NewRequest("DELETE", "/api/priorities/"+tt.priorityID, nil)
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
