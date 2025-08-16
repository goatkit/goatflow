package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-Driven Development for Ticket Template Feature
// Templates allow users to create reusable ticket formats

func TestCreateTicketTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name: "Create basic template successfully",
			formData: url.Values{
				"name":        {"Password Reset"},
				"description": {"Template for password reset requests"},
				"subject":     {"Password Reset Request"},
				"body":        {"Please reset my password for account: [ACCOUNT_EMAIL]"},
				"priority":    {"3 normal"},
				"queue_id":    {"1"},
				"type_id":     {"1"},
				"tags":        {"password,reset,account"},
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Template created successfully", resp["message"])
				assert.Contains(t, resp, "template_id")
				assert.Equal(t, "Password Reset", resp["name"])
			},
		},
		{
			name: "Create template with placeholders",
			formData: url.Values{
				"name":        {"New Employee Onboarding"},
				"description": {"Template for onboarding new employees"},
				"subject":     {"New Employee Setup - {{EMPLOYEE_NAME}}"},
				"body":        {"Please set up accounts for:\nName: {{EMPLOYEE_NAME}}\nDepartment: {{DEPARTMENT}}\nStart Date: {{START_DATE}}\nManager: {{MANAGER_NAME}}"},
				"priority":    {"4 high"},
				"queue_id":    {"2"},
				"type_id":     {"2"},
				"placeholders": {"EMPLOYEE_NAME,DEPARTMENT,START_DATE,MANAGER_NAME"},
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "template_id")
				if placeholders, ok := resp["placeholders"].([]interface{}); ok {
					assert.Len(t, placeholders, 4)
				} else if placeholders, ok := resp["placeholders"].([]string); ok {
					assert.Len(t, placeholders, 4)
				}
			},
		},
		{
			name: "Missing required name",
			formData: url.Values{
				"description": {"Template without name"},
				"subject":     {"Test Subject"},
				"body":        {"Test Body"},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "name is required")
			},
		},
		{
			name: "Duplicate template name",
			formData: url.Values{
				"name":        {"Password Reset"}, // Already exists from first test
				"description": {"Duplicate template"},
				"subject":     {"Test"},
				"body":        {"Test"},
			},
			wantStatus: http.StatusConflict,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Template with this name already exists")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/ticket-templates", handleCreateTicketTemplate)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/ticket-templates", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestGetTicketTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Get all templates",
			query:      "",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				templates := resp["templates"].([]interface{})
				assert.Greater(t, len(templates), 0)
			},
		},
		{
			name:       "Filter by queue",
			query:      "?queue_id=1",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				templates := resp["templates"].([]interface{})
				for _, tmpl := range templates {
					template := tmpl.(map[string]interface{})
					assert.Equal(t, float64(1), template["queue_id"])
				}
			},
		},
		{
			name:       "Search templates",
			query:      "?search=password",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				templates := resp["templates"].([]interface{})
				assert.Greater(t, len(templates), 0)
				// Should find templates with "password" in name or description
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/ticket-templates", handleGetTicketTemplates)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/ticket-templates"+tt.query, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestGetTicketTemplateByID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		templateID string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Get existing template",
			templateID: "1",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "id")
				assert.Contains(t, resp, "name")
				assert.Contains(t, resp, "subject")
				assert.Contains(t, resp, "body")
			},
		},
		{
			name:       "Template not found",
			templateID: "99999",
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Template not found")
			},
		},
		{
			name:       "Invalid template ID",
			templateID: "invalid",
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Invalid template ID")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/ticket-templates/:id", handleGetTicketTemplateByID)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/ticket-templates/"+tt.templateID, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestUpdateTicketTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		templateID string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Update template successfully",
			templateID: "1",
			formData: url.Values{
				"name":        {"Updated Password Reset"},
				"description": {"Updated template for password reset"},
				"subject":     {"Password Reset Request - Updated"},
				"body":        {"Updated body text"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Template updated successfully", resp["message"])
				assert.Equal(t, "Updated Password Reset", resp["name"])
			},
		},
		{
			name:       "Partial update",
			templateID: "2",
			formData: url.Values{
				"description": {"Only updating description"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Template updated successfully", resp["message"])
			},
		},
		{
			name:       "Template not found",
			templateID: "99999",
			formData: url.Values{
				"name": {"Test"},
			},
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Template not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.PUT("/api/ticket-templates/:id", handleUpdateTicketTemplate)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/ticket-templates/"+tt.templateID, strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestDeleteTicketTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		templateID string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Delete template successfully",
			templateID: "3",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Template deleted successfully", resp["message"])
			},
		},
		{
			name:       "Template not found",
			templateID: "99999",
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Template not found")
			},
		},
		{
			name:       "Cannot delete system template",
			templateID: "1", // Assuming ID 1 is a system template
			wantStatus: http.StatusForbidden,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Cannot delete system template")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.DELETE("/api/ticket-templates/:id", handleDeleteTicketTemplate)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", "/api/ticket-templates/"+tt.templateID, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestCreateTicketFromTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		templateID string
		formData   url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Create ticket from template",
			templateID: "1",
			formData: url.Values{
				"customer_email": {"user@example.com"},
				"customer_name":  {"John Doe"},
				"placeholders":   {`{"ACCOUNT_EMAIL": "john@example.com"}`},
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "ticket_id")
				assert.Equal(t, "Ticket created from template", resp["message"])
				assert.Equal(t, "Password Reset Request", resp["subject"])
			},
		},
		{
			name:       "Create ticket with placeholder substitution",
			templateID: "2",
			formData: url.Values{
				"customer_email": {"hr@example.com"},
				"customer_name":  {"HR Department"},
				"placeholders": {`{
					"EMPLOYEE_NAME": "Jane Smith",
					"DEPARTMENT": "Engineering",
					"START_DATE": "2024-01-15",
					"MANAGER_NAME": "Bob Wilson"
				}`},
			},
			wantStatus: http.StatusCreated,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp, "ticket_id")
				// Check that placeholders were replaced
				body := resp["body"].(string)
				assert.Contains(t, body, "Jane Smith")
				assert.Contains(t, body, "Engineering")
				assert.NotContains(t, body, "{{EMPLOYEE_NAME}}")
			},
		},
		{
			name:       "Missing required placeholders",
			templateID: "2",
			formData: url.Values{
				"customer_email": {"hr@example.com"},
				"customer_name":  {"HR Department"},
				"placeholders":   {`{"EMPLOYEE_NAME": "Jane Smith"}`}, // Missing other required placeholders
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Missing required placeholders")
			},
		},
		{
			name:       "Template not found",
			templateID: "99999",
			formData: url.Values{
				"customer_email": {"test@example.com"},
			},
			wantStatus: http.StatusNotFound,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Template not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/api/ticket-templates/:id/create-ticket", handleCreateTicketFromTemplate)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/ticket-templates/"+tt.templateID+"/create-ticket", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestTemplatePermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		userRole   string
		operation  string
		wantStatus int
	}{
		{
			name:       "Admin can create templates",
			userRole:   "admin",
			operation:  "create",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "Agent can view templates",
			userRole:   "agent",
			operation:  "view",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Agent cannot create templates",
			userRole:   "agent",
			operation:  "create",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "Customer can use templates",
			userRole:   "customer",
			operation:  "use",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "Customer cannot modify templates",
			userRole:   "customer",
			operation:  "update",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			
			// Add middleware to set user role
			router.Use(func(c *gin.Context) {
				c.Set("user_role", tt.userRole)
				c.Set("user_id", 1)
				c.Next()
			})
			
			// Register routes based on operation
			switch tt.operation {
			case "create":
				router.POST("/api/ticket-templates", handleCreateTicketTemplate)
				formData := url.Values{
					"name":    {"Test Template"},
					"subject": {"Test"},
					"body":    {"Test"},
				}
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("POST", "/api/ticket-templates", strings.NewReader(formData.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				router.ServeHTTP(w, req)
				assert.Equal(t, tt.wantStatus, w.Code)
				
			case "view":
				router.GET("/api/ticket-templates", handleGetTicketTemplates)
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("GET", "/api/ticket-templates", nil)
				router.ServeHTTP(w, req)
				assert.Equal(t, tt.wantStatus, w.Code)
				
			case "update":
				router.PUT("/api/ticket-templates/:id", handleUpdateTicketTemplate)
				formData := url.Values{"name": {"Updated"}}
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("PUT", "/api/ticket-templates/1", strings.NewReader(formData.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				router.ServeHTTP(w, req)
				assert.Equal(t, tt.wantStatus, w.Code)
				
			case "use":
				router.POST("/api/ticket-templates/:id/create-ticket", handleCreateTicketFromTemplate)
				formData := url.Values{
					"customer_email": {"test@example.com"},
					"customer_name":  {"Test User"},
				}
				w := httptest.NewRecorder()
				req, _ := http.NewRequest("POST", "/api/ticket-templates/1/create-ticket", strings.NewReader(formData.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				router.ServeHTTP(w, req)
				assert.Equal(t, tt.wantStatus, w.Code)
			}
		})
	}
}

func TestTemplatePlaceholderValidation(t *testing.T) {
	tests := []struct {
		name              string
		template          string
		placeholders      map[string]string
		wantResult        string
		wantMissingPlaceholders []string
	}{
		{
			name:     "All placeholders replaced",
			template: "Hello {{NAME}}, your order {{ORDER_ID}} is ready",
			placeholders: map[string]string{
				"NAME":     "John",
				"ORDER_ID": "12345",
			},
			wantResult:        "Hello John, your order 12345 is ready",
			wantMissingPlaceholders: nil,
		},
		{
			name:     "Missing placeholders detected",
			template: "Hello {{NAME}}, your order {{ORDER_ID}} is ready",
			placeholders: map[string]string{
				"NAME": "John",
			},
			wantResult:        "",
			wantMissingPlaceholders: []string{"ORDER_ID"},
		},
		{
			name:     "Multiple placeholder formats",
			template: "User: [USER], Email: {{EMAIL}}, ID: {ID}",
			placeholders: map[string]string{
				"USER":  "John",
				"EMAIL": "john@example.com",
				"ID":    "123",
			},
			wantResult:        "User: John, Email: john@example.com, ID: 123",
			wantMissingPlaceholders: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, missing := replacePlaceholders(tt.template, tt.placeholders)
			
			if tt.wantMissingPlaceholders == nil {
				assert.Empty(t, missing)
				assert.Equal(t, tt.wantResult, result)
			} else {
				assert.Equal(t, tt.wantMissingPlaceholders, missing)
			}
		})
	}
}