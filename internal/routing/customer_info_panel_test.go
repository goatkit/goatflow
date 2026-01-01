package routing

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

func TestHandleCustomerInfoPanel_RendersCustomerDetails(t *testing.T) {
	t.Setenv("TEST_DB_DRIVER", "postgres")
	gin.SetMode(gin.TestMode)

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	templateDir := filepath.Join(filepath.Dir(file), "..", "..", "templates")
	renderer, err := shared.NewTemplateRenderer(templateDir)
	require.NoError(t, err)

	// Set the global renderer
	shared.SetGlobalRenderer(renderer)

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	database.SetDB(mockDB)
	t.Cleanup(database.ResetDB)
	database.ResetAdapterForTest()
	database.SetAdapter(&database.PostgreSQLAdapter{})
	t.Cleanup(database.ResetAdapterForTest)

	login := "customer@example.com"

	query := `SELECT cu.login, cu.title, cu.first_name, cu.last_name, cu.email, cu.phone, cu.mobile,
                 cu.street, cu.zip, cu.city, cu.country, cu.customer_id, cu.comments,
                 cc.name, cc.street, cc.zip, cc.city, cc.country, cc.url, cc.comments
          FROM customer_user cu
          LEFT JOIN customer_company cc ON cc.customer_id = cu.customer_id
          WHERE cu.login = $1 LIMIT 1`

	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(login).
		WillReturnRows(sqlmock.NewRows([]string{
			"login", "title", "first_name", "last_name", "email", "phone", "mobile",
			"street", "zip", "city", "country", "customer_id", "comments",
			"company_name", "company_street", "company_zip", "company_city", "company_country", "company_url", "company_comment",
		}).AddRow(
			login,
			"Ms.",
			"Jane",
			"Doe",
			login,
			"123-456-7890",
			"",
			"123 Main St",
			"75000",
			"Paris",
			"FR",
			"CUST123",
			"VIP customer",
			"Acme Corp",
			"456 Market St",
			"75001",
			"Paris",
			"FR",
			"https://acme.test",
			"Important client",
		))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM tickets WHERE customer_user_id = $1 AND state NOT IN ('closed','resolved')`)).
		WithArgs(login).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	router := gin.New()
	router.GET("/customer-info/:login", HandleCustomerInfoPanel)

	req := httptest.NewRequest(http.MethodGet, "/customer-info/"+login, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	require.Contains(t, body, "customer@example.com")
	require.Contains(t, body, "Acme Corp")
	require.Contains(t, body, ">2<")

	require.NoError(t, mock.ExpectationsWereMet())
}
