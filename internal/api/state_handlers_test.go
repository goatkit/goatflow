package api

import (
	"bytes"
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

func TestGetStates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful get states",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "type_id", "comments", "valid_id"}).
					AddRow(1, "new", 1, "New ticket", 1).
					AddRow(2, "open", 2, "Open ticket", 1).
					AddRow(3, "pending reminder", 3, "Pending reminder", 1).
					AddRow(4, "closed successful", 4, "Closed successfully", 1)
				mock.ExpectQuery("SELECT id, name, type_id, comments, valid_id FROM ticket_state").
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": []interface{}{
					map[string]interface{}{"id": float64(1), "name": "new", "type_id": float64(1), "comments": "New ticket", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(2), "name": "open", "type_id": float64(2), "comments": "Open ticket", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(3), "name": "pending reminder", "type_id": float64(3), "comments": "Pending reminder", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(4), "name": "closed successful", "type_id": float64(4), "comments": "Closed successfully", "valid_id": float64(1)},
				},
			},
		},
		{
			name: "database error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, name, type_id, comments, valid_id FROM ticket_state").
					WillReturnError(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Failed to fetch states",
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
			router.GET("/api/states", handleGetStates)

			req, _ := http.NewRequest("GET", "/api/states", nil)
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

func TestCreateState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		body           map[string]interface{}
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful create state",
			body: map[string]interface{}{
				"name":     "pending approval",
				"type_id":  3,
				"comments": "Waiting for approval",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO ticket_state`).
					WithArgs("pending approval", 3, "Waiting for approval", 1, 1, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
			},
			expectedStatus: http.StatusCreated,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":       float64(5),
					"name":     "pending approval",
					"type_id":  float64(3),
					"comments": "Waiting for approval",
					"valid_id": float64(1),
				},
			},
		},
		{
			name: "missing required fields",
			body: map[string]interface{}{
				"comments": "Test",
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Name and type_id are required",
			},
		},
		{
			name: "duplicate state",
			body: map[string]interface{}{
				"name":    "open",
				"type_id": 2,
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO ticket_state`).
					WithArgs("open", 2, sqlmock.AnyArg(), 1, 1, 1).
					WillReturnError(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Failed to create state",
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
			router.POST("/api/states", handleCreateState)

			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("POST", "/api/states", bytes.NewBuffer(body))
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

func TestUpdateState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		stateID        string
		body           map[string]interface{}
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:    "successful update state",
			stateID: "2",
			body: map[string]interface{}{
				"name":     "in progress",
				"comments": "Work in progress",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_state SET`).
					WithArgs(1, "in progress", "Work in progress", 2).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":       float64(2),
					"name":     "in progress",
					"comments": "Work in progress",
				},
			},
		},
		{
			name:    "invalid state ID",
			stateID: "abc",
			body: map[string]interface{}{
				"name": "test",
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Invalid state ID",
			},
		},
		{
			name:    "state not found",
			stateID: "999",
			body: map[string]interface{}{
				"name": "test",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_state SET`).
					WithArgs(1, "test", 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "State not found",
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
			router.PUT("/api/states/:id", handleUpdateState)

			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("PUT", "/api/states/"+tt.stateID, bytes.NewBuffer(body))
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

func TestDeleteState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		stateID        string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:    "successful delete state (soft delete)",
			stateID: "3",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_state SET valid_id = 2`).
					WithArgs(3, 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"message": "State deleted successfully",
			},
		},
		{
			name:           "invalid state ID",
			stateID:        "xyz",
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Invalid state ID",
			},
		},
		{
			name:    "state not found",
			stateID: "999",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_state SET valid_id = 2`).
					WithArgs(999, 1).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "State not found",
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
			router.DELETE("/api/states/:id", handleDeleteState)

			req, _ := http.NewRequest("DELETE", "/api/states/"+tt.stateID, nil)
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
