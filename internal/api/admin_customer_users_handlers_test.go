package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestHandleAdminCustomerUsersGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		customerID     string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:       "successful get customer user",
			customerID: "1",
			setupMock: func(mock sqlmock.Sqlmock) {
				createTime := time.Now()
				rows := sqlmock.NewRows([]string{
					"id", "login", "email", "customer_id", "pw", "title",
					"first_name", "last_name", "phone", "fax", "mobile",
					"street", "zip", "city", "country", "comments", "valid_id", "create_time",
				}).AddRow(
					1, "john.doe", "john@example.com", "customer-123", "", "Mr.",
					"John", "Doe", "555-1234", "", "555-5678",
					"123 Main St", "12345", "New York", "USA", "Test customer", 1, createTime,
				)

				mock.ExpectQuery(`SELECT cu.id, cu.login, cu.email, cu.customer_id, cu.pw, cu.title`).
					WithArgs(1).
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].(map[string]interface{})
				assert.Equal(t, "john.doe", data["login"])
				assert.Equal(t, "john@example.com", data["email"])
				assert.Equal(t, "customer-123", data["customer_id"])
				assert.Equal(t, "John", data["first_name"])
				assert.Equal(t, "Doe", data["last_name"])
			},
		},
		{
			name:       "customer user not found",
			customerID: "999",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT cu.id, cu.login, cu.email, cu.customer_id`).
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Customer user not found", body["error"])
			},
		},
		{
			name:           "invalid customer ID",
			customerID:     "invalid",
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Invalid customer user ID", body["error"])
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
			router.GET("/admin/customer-users/:id", HandleAdminCustomerUsersGet)

			req, _ := http.NewRequest("GET", "/admin/customer-users/"+tt.customerID, nil)
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			tt.checkResponse(t, response)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestHandleAdminCustomerUsersCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		formData       url.Values
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "successful create customer user",
			formData: url.Values{
				"login":       {"newuser"},
				"email":       {"new@example.com"},
				"customer_id": {"customer-456"},
				"password":    {"SecurePass123"},
				"first_name":  {"New"},
				"last_name":   {"User"},
				"valid_id":    {"1"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id FROM customer_user WHERE login`).
					WithArgs("newuser").
					WillReturnError(sql.ErrNoRows)

				mock.ExpectQuery(`INSERT INTO customer_user`).
					WithArgs(
						"newuser", "new@example.com", "customer-456", "SecurePass123",
						"", "New", "User", "", "", "", "", "", "", "", "", 1,
					).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				assert.Equal(t, "Customer user created successfully", body["message"])
				assert.Equal(t, float64(5), body["id"])
			},
		},
		{
			name: "duplicate login error",
			formData: url.Values{
				"login":       {"existinguser"},
				"email":       {"new@example.com"},
				"customer_id": {"customer-456"},
				"password":    {"SecurePass123"},
				"valid_id":    {"1"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id FROM customer_user WHERE login`).
					WithArgs("existinguser").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Login already exists", body["error"])
			},
		},
		{
			name: "missing required fields",
			formData: url.Values{
				"login": {""},
				"email": {"invalid"},
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
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
			router.POST("/admin/customer-users", HandleAdminCustomerUsersCreate)

			req, _ := http.NewRequest("POST", "/admin/customer-users", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			tt.checkResponse(t, response)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestHandleAdminCustomerUsersUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		customerID     string
		formData       url.Values
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:       "successful update customer user",
			customerID: "1",
			formData: url.Values{
				"login":       {"john.doe.updated"},
				"email":       {"john.updated@example.com"},
				"customer_id": {"customer-123"},
				"first_name":  {"John"},
				"last_name":   {"Updated"},
				"valid_id":    {"1"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id FROM customer_user WHERE login .* AND id`).
					WithArgs("john.doe.updated", 1).
					WillReturnError(sql.ErrNoRows)

				mock.ExpectExec(`UPDATE customer_user SET`).
					WithArgs(
						"john.doe.updated", "john.updated@example.com", "customer-123", "",
						"John", "Updated", "", "", "", "", "", "", "", "", 1, 1,
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				assert.Equal(t, "Customer user updated successfully", body["message"])
			},
		},
		{
			name:       "update with password change",
			customerID: "1",
			formData: url.Values{
				"login":       {"john.doe"},
				"email":       {"john@example.com"},
				"customer_id": {"customer-123"},
				"password":    {"NewPassword456"},
				"first_name":  {"John"},
				"last_name":   {"Doe"},
				"valid_id":    {"1"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id FROM customer_user WHERE login .* AND id`).
					WithArgs("john.doe", 1).
					WillReturnError(sql.ErrNoRows)

				mock.ExpectExec(`UPDATE customer_user SET`).
					WithArgs(
						"john.doe", "john@example.com", "customer-123", "",
						"John", "Doe", "", "", "", "", "", "", "", "", 1, "NewPassword456", 1,
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
			},
		},
		{
			name:       "duplicate login for another user",
			customerID: "2",
			formData: url.Values{
				"login":       {"existinglogin"},
				"email":       {"test@example.com"},
				"customer_id": {"customer-123"},
				"valid_id":    {"1"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id FROM customer_user WHERE login .* AND id`).
					WithArgs("existinglogin", 2).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Login already exists for another user", body["error"])
			},
		},
		{
			name:       "customer user not found",
			customerID: "999",
			formData: url.Values{
				"login":       {"nonexistent"},
				"email":       {"test@example.com"},
				"customer_id": {"customer-123"},
				"valid_id":    {"1"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id FROM customer_user WHERE login .* AND id`).
					WithArgs("nonexistent", 999).
					WillReturnError(sql.ErrNoRows)

				mock.ExpectExec(`UPDATE customer_user SET`).
					WithArgs(
						"nonexistent", "test@example.com", "customer-123", "",
						"", "", "", "", "", "", "", "", "", "", 1, 999,
					).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Customer user not found", body["error"])
			},
		},
		{
			name:           "invalid customer ID",
			customerID:     "invalid",
			formData:       url.Values{},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Invalid customer user ID", body["error"])
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
			router.PUT("/admin/customer-users/:id", HandleAdminCustomerUsersUpdate)

			req, _ := http.NewRequest("PUT", "/admin/customer-users/"+tt.customerID, strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			tt.checkResponse(t, response)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestHandleAdminCustomerUsersDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		customerID     string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:       "successful soft delete",
			customerID: "1",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE customer_user SET valid_id = 2`).
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				assert.Equal(t, "Customer user deleted successfully", body["message"])
			},
		},
		{
			name:       "customer user not found",
			customerID: "999",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE customer_user SET valid_id = 2`).
					WithArgs(999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Customer user not found", body["error"])
			},
		},
		{
			name:           "invalid customer ID",
			customerID:     "invalid",
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Invalid customer user ID", body["error"])
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
			router.DELETE("/admin/customer-users/:id", HandleAdminCustomerUsersDelete)

			req, _ := http.NewRequest("DELETE", "/admin/customer-users/"+tt.customerID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			tt.checkResponse(t, response)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestHandleAdminCustomerUsersTickets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		customerID     string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:       "successful get tickets",
			customerID: "1",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT login FROM customer_user WHERE id`).
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"login"}).AddRow("john.doe"))

				ticketRows := sqlmock.NewRows([]string{
					"id", "title", "ticket_number", "create_time", "state", "priority", "queue",
				}).
					AddRow(100, "Test Ticket 1", "TN-001", time.Now(), "open", "3 normal", "Support").
					AddRow(101, "Test Ticket 2", "TN-002", time.Now(), "closed", "2 high", "Sales")

				mock.ExpectQuery(`SELECT t.id, t.title, t.ticket_number, t.create_time`).
					WithArgs("john.doe").
					WillReturnRows(ticketRows)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				data := body["data"].([]interface{})
				assert.Len(t, data, 2)
			},
		},
		{
			name:       "customer user not found",
			customerID: "999",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT login FROM customer_user WHERE id`).
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Customer user not found", body["error"])
			},
		},
		{
			name:       "no tickets for customer",
			customerID: "2",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT login FROM customer_user WHERE id`).
					WithArgs(2).
					WillReturnRows(sqlmock.NewRows([]string{"login"}).AddRow("jane.doe"))

				mock.ExpectQuery(`SELECT t.id, t.title, t.ticket_number, t.create_time`).
					WithArgs("jane.doe").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "title", "ticket_number", "create_time", "state", "priority", "queue",
					}))
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				assert.Nil(t, body["data"])
			},
		},
		{
			name:           "invalid customer ID",
			customerID:     "invalid",
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Invalid customer user ID", body["error"])
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
			router.GET("/admin/customer-users/:id/tickets", HandleAdminCustomerUsersTickets)

			req, _ := http.NewRequest("GET", "/admin/customer-users/"+tt.customerID+"/tickets", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			tt.checkResponse(t, response)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestHandleAdminCustomerUsersBulkAction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		body           map[string]interface{}
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		checkResponse  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "successful bulk enable",
			body: map[string]interface{}{
				"action": "enable",
				"ids":    []string{"1", "2", "3"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE customer_user SET valid_id = 1`).
					WithArgs(1, 2, 3).
					WillReturnResult(sqlmock.NewResult(0, 3))
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				assert.Equal(t, "Customer users enabled successfully", body["message"])
				assert.Equal(t, float64(3), body["rows_affected"])
			},
		},
		{
			name: "successful bulk disable",
			body: map[string]interface{}{
				"action": "disable",
				"ids":    []string{"4", "5"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE customer_user SET valid_id = 2`).
					WithArgs(4, 5).
					WillReturnResult(sqlmock.NewResult(0, 2))
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				assert.Equal(t, "Customer users disabled successfully", body["message"])
			},
		},
		{
			name: "successful bulk delete (soft delete)",
			body: map[string]interface{}{
				"action": "delete",
				"ids":    []string{"6"},
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE customer_user SET valid_id = 2`).
					WithArgs(6).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				assert.Equal(t, "Customer users deleted successfully", body["message"])
			},
		},
		{
			name: "invalid action",
			body: map[string]interface{}{
				"action": "invalid_action",
				"ids":    []string{"1"},
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Equal(t, "Invalid action", body["error"])
			},
		},
		{
			name: "empty IDs list",
			body: map[string]interface{}{
				"action": "enable",
				"ids":    []string{},
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
			},
		},
		{
			name: "invalid ID in list",
			body: map[string]interface{}{
				"action": "enable",
				"ids":    []string{"1", "invalid", "3"},
			},
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body map[string]interface{}) {
				assert.False(t, body["success"].(bool))
				assert.Contains(t, body["error"].(string), "Invalid customer user ID")
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
			router.POST("/admin/customer-users/bulk-action", HandleAdminCustomerUsersBulkAction)

			jsonBody, _ := json.Marshal(tt.body)
			req, _ := http.NewRequest("POST", "/admin/customer-users/bulk-action", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			tt.checkResponse(t, response)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestHandleAdminCustomerUsersExport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMock      func(sqlmock.Sqlmock)
		expectedStatus int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "successful export",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"login", "email", "customer_id", "title", "first_name", "last_name",
					"phone", "fax", "mobile", "street", "zip", "city", "country",
					"comments", "valid_id", "company_name",
				}).
					AddRow("john.doe", "john@example.com", "customer-1", "Mr.", "John", "Doe",
						"555-1234", "", "555-5678", "123 Main St", "12345", "New York", "USA",
						"Test comment", "1", "ACME Corp").
					AddRow("jane.smith", "jane@example.com", "customer-2", "Ms.", "Jane", "Smith",
						"555-9876", "", "", "", "", "Boston", "USA",
						"", "1", "Tech Inc")

				mock.ExpectQuery(`SELECT cu.login, cu.email, cu.customer_id`).
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
				assert.Contains(t, w.Header().Get("Content-Disposition"), "customer_users.csv")
				assert.Contains(t, w.Body.String(), "login,email,customer_id")
				assert.Contains(t, w.Body.String(), "john.doe")
				assert.Contains(t, w.Body.String(), "jane.smith")
			},
		},
		{
			name: "export empty result",
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"login", "email", "customer_id", "title", "first_name", "last_name",
					"phone", "fax", "mobile", "street", "zip", "city", "country",
					"comments", "valid_id", "company_name",
				})
				mock.ExpectQuery(`SELECT cu.login, cu.email, cu.customer_id`).
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
				lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
				assert.Equal(t, 1, len(lines)) // Only header row
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
			router.GET("/admin/customer-users/export", HandleAdminCustomerUsersExport)

			req, _ := http.NewRequest("GET", "/admin/customer-users/export", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
