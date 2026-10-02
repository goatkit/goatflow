package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

// Customer portal tickets get their number from the configured ticket number
// generator, like agent tickets, so two submissions in quick succession both
// succeed with distinct numbers.
func TestCustomerCreateTicketUsesConfiguredTicketNumberGenerator(t *testing.T) {
	db := getTestDB(t)
	gen, err := ticketnumber.Resolve("DateChecksum", "10", nil)
	require.NoError(t, err)
	repository.SetTicketNumberGenerator(gen, ticketnumber.NewDBStore(db, "10"))
	t.Cleanup(func() { require.NoError(t, initTestTicketNumberGenerator()) })

	login := fmt.Sprintf("tn-gen-%d", time.Now().UnixNano())
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, first_name, last_name, valid_id,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'tn-gen-co', 'Tn', 'Gen', 1, NOW(), 1, NOW(), 1)`), login, login+"@example.com")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login = ?"), login)
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/customer/tickets/create", func(c *gin.Context) {
		c.Set("username", login)
		c.Set("user_role", "Customer")
	}, handleCustomerCreateTicket(db))

	create := func(title string) string {
		t.Helper()
		form := url.Values{"title": {title}, "message": {"Printer on floor 2 is offline."}}
		req := httptest.NewRequest(http.MethodPost, "/customer/tickets/create", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := serve(router, req)
		require.Equal(t, http.StatusSeeOther, w.Code, w.Body.String())

		ticketID, err := strconv.ParseInt(strings.TrimPrefix(w.Header().Get("Location"), "/customer/tickets/"), 10, 64)
		require.NoError(t, err, w.Header().Get("Location"))
		deleteTicketRows(t, db, ticketID)

		var tn, customerUserID string
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT tn, customer_user_id FROM ticket WHERE id = ?"), ticketID).Scan(&tn, &customerUserID))
		assert.Equal(t, login, customerUserID)
		return tn
	}

	first := create("First printer ticket")
	second := create("Second printer ticket")

	// DateChecksum with SystemID 10: yyyymmdd + "10" + counter (min 5 digits) + check digit.
	dateChecksum := `^\d{8}10\d{5,}\d$`
	assert.Regexp(t, dateChecksum, first)
	assert.Regexp(t, dateChecksum, second)
	assert.NotEqual(t, first, second)
}
