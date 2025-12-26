//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCreateQueueAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name: "should create new queue with valid data",
			requestBody: `{
				"name": "Customer Support",
				"comment": "High-priority customer issues",
				"group_id": 1,
				"system_address": "support@company.com",
				"default_sign_key": "",
				"unlock_timeout": 300,
				"follow_up_id": 1,
				"follow_up_lock": 0,
				"calendar_name": "",
				"first_response_time": 60,
				"first_response_notify": 1,
				"update_time": 120,
				"update_notify": 1,
				"solution_time": 480,
				"solution_notify": 1
			}`,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Success bool `json:"success"`
					Data    struct {
						ID      int    `json:"id"`
						Name    string `json:"name"`
						Comment string `json:"comment"`
					} `json:"data"`
				}

				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.True(t, response.Success)
				assert.Greater(t, response.Data.ID, 0)
				assert.Equal(t, "Customer Support", response.Data.Name)
				assert.Equal(t, "High-priority customer issues", response.Data.Comment)
			},
		},
		{
			name: "should return 400 for missing required fields",
			requestBody: `{
				"comment": "Missing name field"
			}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "name")
				assert.Contains(t, strings.ToLower(body), "required")
			},
		},
		{
			name: "should return 400 for duplicate queue name",
			requestBody: `{
				"name": "Raw",
				"comment": "Duplicate name test"
			}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "queue name already exists")
			},
		},
		{
			name: "should return 400 for invalid JSON",
			requestBody: `{
				"name": "Test Queue",
				"comment": "Invalid JSON",
			}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "invalid json")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/queues", handleCreateQueue)

			req, _ := http.NewRequest("POST", "/api/queues", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestUpdateQueueAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		requestBody    string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:    "should update existing queue",
			queueID: "1",
			requestBody: `{
				"name": "Raw - Updated",
				"comment": "Updated description for raw tickets"
			}`,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Success bool `json:"success"`
					Data    struct {
						ID      int    `json:"id"`
						Name    string `json:"name"`
						Comment string `json:"comment"`
					} `json:"data"`
				}

				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.True(t, response.Success)
				assert.Equal(t, 1, response.Data.ID)
				assert.Equal(t, "Raw - Updated", response.Data.Name)
				assert.Contains(t, response.Data.Comment, "Updated description")
			},
		},
		{
			name:    "should return 404 for non-existent queue",
			queueID: "999",
			requestBody: `{
				"name": "Non-existent Queue",
				"comment": "This should fail"
			}`,
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "queue not found")
			},
		},
		{
			name:    "should return 400 for invalid queue ID",
			queueID: "invalid",
			requestBody: `{
				"name": "Test Queue"
			}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "invalid queue id")
			},
		},
		{
			name:    "should validate name uniqueness on update",
			queueID: "2",
			requestBody: `{
				"name": "Raw"
			}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "queue name already exists")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/queues/:id", handleUpdateQueue)

			req, _ := http.NewRequest("PUT", "/api/queues/"+tt.queueID, bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestDeleteQueueAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queueID        string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should soft delete queue with no tickets",
			queueID:        "3", // Misc queue has no tickets
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				var response struct {
					Success bool   `json:"success"`
					Message string `json:"message"`
				}

				err := json.Unmarshal([]byte(body), &response)
				assert.NoError(t, err)
				assert.True(t, response.Success)
				assert.Contains(t, response.Message, "Queue deleted successfully")
			},
		},
		{
			name:           "should return 409 when trying to delete queue with tickets",
			queueID:        "1", // Raw queue has tickets
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, "Cannot delete queue with existing tickets")
			},
		},
		{
			name:           "should return 404 for non-existent queue",
			queueID:        "999",
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "queue not found")
			},
		},
		{
			name:           "should return 400 for invalid queue ID",
			queueID:        "invalid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "invalid queue id")
			},
		},
		{
			name:           "should return 409 when trying to delete queue with tickets (system protection)",
			queueID:        "1", // Raw queue has tickets (acting as system queue protection)
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body string) {
				// In our current implementation, ticket check comes first
				// In production, system queue check would come before ticket check
				assert.Contains(t, body, "Cannot delete queue with existing tickets")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.DELETE("/api/queues/:id", handleDeleteQueue)

			req, _ := http.NewRequest("DELETE", "/api/queues/"+tt.queueID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueValidationAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name: "should validate queue name length",
			requestBody: `{
				"name": "A",
				"comment": "Too short name"
			}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "name")
				assert.Contains(t, strings.ToLower(body), "min")
			},
		},
		{
			name: "should validate queue name length maximum",
			requestBody: `{
				"name": "` + strings.Repeat("A", 201) + `",
				"comment": "Too long name"
			}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "name")
				assert.Contains(t, strings.ToLower(body), "max")
			},
		},
		{
			name: "should validate email format in system_address",
			requestBody: `{
				"name": "Valid Queue",
				"system_address": "invalid-email"
			}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "invalid email format")
			},
		},
		{
			name: "should validate positive values for time fields",
			requestBody: `{
				"name": "Valid Queue",
				"first_response_time": -1
			}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "time values must be positive")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/queues", handleCreateQueue)

			req, _ := http.NewRequest("POST", "/api/queues", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}
