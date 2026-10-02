package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// The ticket-create form loads /tickets/customer-info/:login into a side panel.
// Its "open tickets" figure must count the customer's tickets whose state type
// is new/open/pending, and nothing else.
func TestCustomerInfoPanelCountsOpenTickets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	token := GetTestAuthToken(t)

	suffix := time.Now().UnixNano()
	login := fmt.Sprintf("cipanel%d", suffix)
	other := login + "x"
	createTestCustomerUser(t, db, login)
	createTestCustomerUser(t, db, other)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket WHERE customer_user_id IN (?, ?)`), login, other)
		cleanupTestCustomerUser(t, db, login)
		cleanupTestCustomerUser(t, db, other)
	})

	insert := func(customer string, stateType string, n int) {
		t.Helper()
		var stateID int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
			SELECT MIN(ts.id) FROM ticket_state ts
			JOIN ticket_state_type tst ON tst.id = ts.type_id
			WHERE tst.name = ?`), stateType).Scan(&stateID))
		_, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
				ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
				escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, 'customer panel', 1, 1, 1, 1, 1, 3, ?, 'test-company', ?, 0, 0, 0, 0, 0, 0, 0,
				CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
			RETURNING id`), fmt.Sprintf("CIP%d%02d", suffix%1e12, n), stateID, customer)
		require.NoError(t, err)
	}
	insert(login, "new", 1)
	insert(login, "open", 2)
	insert(login, "pending reminder", 3)
	insert(login, "closed", 4)
	insert(other, "open", 5)

	router := NewSimpleRouterWithDB(db)
	req := httptest.NewRequest(http.MethodGet, "/tickets/customer-info/"+login, nil)
	AddTestAuthCookie(req, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	m := regexp.MustCompile(`Open tickets \(customer\)</dt>\s*<dd[^>]*>(\d+)</dd>`).FindStringSubmatch(w.Body.String())
	require.NotNil(t, m, "open tickets row missing: %s", w.Body.String())
	require.Equal(t, "3", m[1])
}
