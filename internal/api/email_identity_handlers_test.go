package api

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func setupEmailIdentityTestRouter(handler gin.HandlerFunc, method, path string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", 1)
	})
	router.Handle(method, path, handler)
	return router
}

func expectInsertReturning(mock sqlmock.Sqlmock, table string, id int64, args ...driver.Value) {
	pattern := fmt.Sprintf("INSERT INTO %s", table)
	if database.IsMySQL() {
		mock.ExpectExec(pattern).
			WithArgs(args...).
			WillReturnResult(sqlmock.NewResult(id, 1))
		return
	}
	mock.ExpectQuery(pattern).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
}

func TestHandleCreateSystemAddressAPI(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	database.SetDB(db)
	defer database.ResetDB()

	expectInsertReturning(mock, "system_address", 7, "support@example.com", "Support", 1, sqlmock.AnyArg(), 1, 1, 1)

	router := setupEmailIdentityTestRouter(HandleCreateSystemAddressAPI, http.MethodPost, "/api/v1/system-addresses")

	body := map[string]interface{}{
		"email":        "support@example.com",
		"display_name": "Support",
		"queue_id":     1,
		"comments":     "Primary",
		"valid_id":     1,
	}

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/system-addresses", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleCreateSystemAddressAPI_Validation(t *testing.T) {
	router := setupEmailIdentityTestRouter(HandleCreateSystemAddressAPI, http.MethodPost, "/api/v1/system-addresses")

	reqBody := map[string]interface{}{
		"display_name": "Support",
		"queue_id":     1,
	}

	payload, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/system-addresses", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleUpdateSystemAddressAPI(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	database.SetDB(db)
	defer database.ResetDB()

	mock.ExpectExec("UPDATE system_address").
		WithArgs("support@example.com", "Support", 1, sqlmock.AnyArg(), 1, 1, 9).
		WillReturnResult(sqlmock.NewResult(0, 1))

	router := setupEmailIdentityTestRouter(HandleUpdateSystemAddressAPI, http.MethodPut, "/api/v1/system-addresses/:id")

	body := map[string]interface{}{
		"email":        "support@example.com",
		"display_name": "Support",
		"queue_id":     1,
		"comments":     "Primary",
		"valid_id":     1,
	}

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPut, "/api/v1/system-addresses/9", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleCreateSalutationAPI(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	database.SetDB(db)
	defer database.ResetDB()

	expectInsertReturning(mock, "salutation", 3, "Default", "Hello", "text/plain", sqlmock.AnyArg(), 1, 1, 1)

	router := setupEmailIdentityTestRouter(HandleCreateSalutationAPI, http.MethodPost, "/api/v1/salutations")

	body := map[string]interface{}{
		"name":     "Default",
		"text":     "Hello",
		"comments": "Greeting",
	}

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/salutations", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleUpdateSalutationAPI_Validation(t *testing.T) {
	router := setupEmailIdentityTestRouter(HandleUpdateSalutationAPI, http.MethodPut, "/api/v1/salutations/:id")

	body := map[string]interface{}{
		"text": "Hello",
	}

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPut, "/api/v1/salutations/5", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCreateSignatureAPI(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	database.SetDB(db)
	defer database.ResetDB()

	expectInsertReturning(mock, "signature", 4, "Default", "Thanks", "text/plain", sqlmock.AnyArg(), 1, 1, 1)

	router := setupEmailIdentityTestRouter(HandleCreateSignatureAPI, http.MethodPost, "/api/v1/signatures")

	body := map[string]interface{}{
		"name":     "Default",
		"text":     "Thanks",
		"comments": "Footer",
	}

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/signatures", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
