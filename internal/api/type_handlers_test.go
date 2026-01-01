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

func TestGetTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
		validate       func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful get types",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "comments", "valid_id"}).
					AddRow(1, "Incident", "Service incident", 1).
					AddRow(2, "Problem", "Underlying problem", 1).
					AddRow(3, "Request", "Service request", 1).
					AddRow(4, "Change", "Change request", 1)
				mock.ExpectQuery("SELECT id, name, comments, valid_id FROM ticket_type").
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": []interface{}{
					map[string]interface{}{"id": float64(1), "name": "Incident", "comments": "Service incident", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(2), "name": "Problem", "comments": "Underlying problem", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(3), "name": "Request", "comments": "Service request", "valid_id": float64(1)},
					map[string]interface{}{"id": float64(4), "name": "Change", "comments": "Change request", "valid_id": float64(1)},
				},
			},
		},
		{
			name: "database error falls back to lookup data",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, name, comments, valid_id FROM ticket_type").
					WillReturnError(assert.AnError)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
			},
			validate: func(t *testing.T, response map[string]interface{}) {
				data, ok := response["data"].([]interface{})
				require.True(t, ok, "data should be an array")
				require.NotEmpty(t, data)
				first := data[0].(map[string]interface{})
				assert.NotEmpty(t, first["value"])
				assert.NotEmpty(t, first["label"])
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
			router.GET("/api/types", HandleGetTypes)

			req, _ := http.NewRequest("GET", "/api/types", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			if tt.expectedBody != nil {
				if expectedSuccess, ok := tt.expectedBody["success"]; ok {
					assert.Equal(t, expectedSuccess, response["success"])
				}

				if expectedError, ok := tt.expectedBody["error"]; ok && expectedError != nil {
					assert.Equal(t, expectedError, response["error"])
				} else if _, hasError := response["error"]; hasError && tt.expectedBody["error"] == nil {
					assert.Nil(t, response["error"])
				}

				if expectedData, ok := tt.expectedBody["data"]; ok && expectedData != nil {
					assert.Equal(t, expectedData, response["data"])
				}
			}

			if tt.validate != nil {
				tt := tt
				tt.validate(t, response)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCreateType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")

	tests := []struct {
		name           string
		body           map[string]interface{}
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful create type",
			body: map[string]interface{}{
				"name":     "Enhancement",
				"comments": "Feature enhancement request",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO ticket_type`).
					WithArgs("Enhancement", "Feature enhancement request", 1, 1, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
			},
			expectedStatus: http.StatusCreated,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":       float64(5),
					"name":     "Enhancement",
					"comments": "Feature enhancement request",
					"valid_id": float64(1),
				},
			},
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"comments": "Test",
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Name is required",
			},
		},
		{
			name: "duplicate type",
			body: map[string]interface{}{
				"name": "Incident",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO ticket_type`).
					WithArgs("Incident", sqlmock.AnyArg(), 1, 1, 1).
					WillReturnError(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Failed to create type",
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
			router.POST("/api/types", handleCreateType)

			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("POST", "/api/types", bytes.NewBuffer(body))
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

func TestUpdateType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")

	tests := []struct {
		name           string
		typeID         string
		body           map[string]interface{}
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:   "successful update type",
			typeID: "3",
			body: map[string]interface{}{
				"name":     "Service Request",
				"comments": "General service request",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_type SET`).
					WithArgs(1, "Service Request", "General service request", 3).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":       float64(3),
					"name":     "Service Request",
					"comments": "General service request",
					"valid_id": float64(1),
				},
			},
		},
		{
			name:   "invalid type ID",
			typeID: "abc",
			body: map[string]interface{}{
				"name": "test",
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Invalid type ID",
			},
		},
		{
			name:   "type not found",
			typeID: "999",
			body: map[string]interface{}{
				"name": "test",
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_type SET`).
					WithArgs(1, "test", 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Type not found",
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
			router.PUT("/api/types/:id", handleUpdateType)

			body, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("PUT", "/api/types/"+tt.typeID, bytes.NewBuffer(body))
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
				assert.Equal(t, expectedData["comments"], responseData["comments"])
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDeleteType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "test")

	tests := []struct {
		name           string
		typeID         string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name:   "successful delete type (soft delete)",
			typeID: "4",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_type SET valid_id = 2`).
					WithArgs(4, 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
				"message": "Type deleted successfully",
			},
		},
		{
			name:           "invalid type ID",
			typeID:         "xyz",
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Invalid type ID",
			},
		},
		{
			name:   "type not found",
			typeID: "999",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE ticket_type SET valid_id = 2`).
					WithArgs(999, 1).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			expectedBody: map[string]interface{}{
				"success": false,
				"error":   "Type not found",
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
			router.DELETE("/api/types/:id", handleDeleteType)

			req, _ := http.NewRequest("DELETE", "/api/types/"+tt.typeID, nil)
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
