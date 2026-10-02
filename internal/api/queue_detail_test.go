package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQueueDetailErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	getTestDB(t)

	tests := []struct {
		name           string
		queueID        string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should handle non-existent queue gracefully",
			queueID:        "999",
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "queue not found")
			},
		},
		{
			name:           "should handle invalid queue ID gracefully",
			queueID:        "invalid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "invalid queue id")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/queues/:id", handleQueueDetail)

			req, _ := http.NewRequest("GET", "/api/queues/"+tt.queueID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}
