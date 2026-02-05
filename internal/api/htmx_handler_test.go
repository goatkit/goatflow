package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// MockHTMXService for testing.
type MockHTMXService struct {
	LoginFunc      func(email, password string) (*models.User, string, string, error)
	GetTicketsFunc func(userID uint, filters map[string]interface{}) ([]*models.Ticket, error)
}

func (m *MockHTMXService) Login(email, password string) (*models.User, string, string, error) {
	if m.LoginFunc != nil {
		return m.LoginFunc(email, password)
	}
	return nil, "", "", errors.New("not implemented")
}

func (m *MockHTMXService) GetTickets(userID uint, filters map[string]interface{}) ([]*models.Ticket, error) {
	if m.GetTicketsFunc != nil {
		return m.GetTicketsFunc(userID, filters)
	}
	return nil, errors.New("not implemented")
}

func TestHTMXLoginHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    map[string]string
		setupMock      func(*MockHTMXService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful login returns JSON with HTMX request",
			requestBody: map[string]string{
				"email":    "test@example.com",
				"password": "password123",
			},
			setupMock: func(m *MockHTMXService) {
				m.LoginFunc = func(email, password string) (*models.User, string, string, error) {
					if email == "test@example.com" && password == "password123" {
						user := &models.User{
							ID:        1,
							Email:     "test@example.com",
							FirstName: "Test",
							LastName:  "User",
						}
						return user, "access_token", "refresh_token", nil
					}
					return nil, "", "", errors.New("invalid credentials")
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "access_token", response["access_token"])
				assert.Equal(t, "refresh_token", response["refresh_token"])
			},
		},
		{
			name: "failed login returns 401",
			requestBody: map[string]string{
				"email":    "test@example.com",
				"password": "wrongpassword",
			},
			setupMock: func(m *MockHTMXService) {
				m.LoginFunc = func(email, password string) (*models.User, string, string, error) {
					return nil, "", "", errors.New("Invalid credentials")
				}
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Invalid credentials")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockHTMXService)
			tt.setupMock(mockService)

			router := gin.New()
			// Add HTMX login endpoint
			router.POST("/api/auth/login", func(c *gin.Context) {
				var req map[string]string
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				user, accessToken, refreshToken, err := mockService.Login(req["email"], req["password"])
				if err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"access_token":  accessToken,
					"refresh_token": refreshToken,
					"user":          user,
				})
			})

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("HX-Request", "true") // HTMX header

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestHTMXTicketListHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    string
		setupMock      func(*MockHTMXService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "returns HTML fragment for ticket list",
			queryParams: "?status=open&priority=3",
			setupMock: func(m *MockHTMXService) {
				m.GetTicketsFunc = func(userID uint, filters map[string]interface{}) ([]*models.Ticket, error) {
					tickets := []*models.Ticket{
						{
							ID:               1,
							TicketNumber:     "TICKET-001",
							Title:            "Test Ticket",
							TicketStateID:    2, // open
							TicketPriorityID: 3,
						},
					}
					return tickets, nil
				}
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				body := w.Body.String()
				assert.Contains(t, body, "TICKET-001")
				assert.Contains(t, body, "Test Ticket")
				assert.Contains(t, body, "open")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockHTMXService)
			tt.setupMock(mockService)

			router := gin.New()
			router.GET("/api/tickets", func(c *gin.Context) {
				// In real implementation, would get user from context
				userID := uint(1)

				filters := make(map[string]interface{})
				if status := c.Query("status"); status != "" {
					filters["status"] = status
				}
				if priority := c.Query("priority"); priority != "" {
					filters["priority"] = priority
				}

				tickets, err := mockService.GetTickets(userID, filters)
				if err != nil {
					c.String(http.StatusInternalServerError, "Error loading tickets")
					return
				}

				// Return HTML fragment for HTMX
				var html strings.Builder
				for _, ticket := range tickets {
					html.WriteString(`<div class="ticket-item">`)
					html.WriteString(`<span class="ticket-number">` + ticket.TicketNumber + `</span>`)
					html.WriteString(`<span class="ticket-title">` + ticket.Title + `</span>`)
					html.WriteString(`<span class="ticket-status">open</span>`)
					html.WriteString(`</div>`)
				}

				c.Header("Content-Type", "text/html")
				c.String(http.StatusOK, html.String())
			})

			req := httptest.NewRequest("GET", "/api/tickets"+tt.queryParams, nil)
			req.Header.Set("HX-Request", "true")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestSSEEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/tickets/stream", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		// Send a test event
		c.SSEvent("ticket-update", gin.H{
			"ticketId": 1,
			"status":   "updated",
		})
		c.Writer.Flush()
	})

	req := httptest.NewRequest("GET", "/api/tickets/stream", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	// Gin's SSEvent doesn't add space after colon
	assert.Contains(t, w.Body.String(), "event:ticket-update")
}
