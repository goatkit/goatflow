package api

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestGetCustomerUsersForAgentReturnsPreferredQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	userQuery := database.ConvertPlaceholders(`
        SELECT login, email, first_name, last_name, customer_id
        FROM customer_user
        WHERE valid_id = 1
        ORDER BY first_name, last_name, email
    `)
	mock.ExpectQuery(regexp.QuoteMeta(userQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"login", "email", "first_name", "last_name", "customer_id"}).
			AddRow("john.customer", "john@example.com", "John", "Customer", "COMP1").
			AddRow("jane.customer", "jane@example.com", "Jane", "Customer", "COMP2"))

	queueQuery := fmt.Sprintf(`
        SELECT gc.customer_id, q.id, q.name, gc.permission_key
        FROM group_customer gc
        JOIN queue q ON q.group_id = gc.group_id
        WHERE gc.permission_value = 1
          AND q.valid_id = 1
          AND gc.customer_id IN (%s)
    `, "$1,$2")
	queueQuery = database.ConvertPlaceholders(queueQuery)
	mock.ExpectQuery(regexp.QuoteMeta(queueQuery)).
		WithArgs("COMP1", "COMP2").
		WillReturnRows(sqlmock.NewRows([]string{"customer_id", "id", "name", "permission_key"}).
			AddRow("COMP1", 90, "General", "ro").
			AddRow("COMP1", 10, "Support", "rw").
			AddRow("COMP2", 20, "Sales", "ro"))

	userQueueQuery := fmt.Sprintf(`
		SELECT gcu.user_id, q.id, q.name, gcu.permission_key
		FROM group_customer_user gcu
		JOIN queue q ON q.group_id = gcu.group_id
		WHERE gcu.permission_value = 1
		  AND q.valid_id = 1
		  AND gcu.user_id IN (%s)
	`, "$1,$2")
	userQueueQuery = database.ConvertPlaceholders(userQueueQuery)
	mock.ExpectQuery(regexp.QuoteMeta(userQueueQuery)).
		WithArgs("john.customer", "jane.customer").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "id", "name", "permission_key"}).
			AddRow("john.customer", 50, "VIP Support", "rw").
			AddRow("john.customer", 55, "Legacy", "ro").
			AddRow("jane.customer", 21, "Sales Escalations", "create"))

	users, err := getCustomerUsersForAgent(db)
	require.NoError(t, err)
	require.Len(t, users, 2)

	resultByLogin := make(map[string]gin.H)
	for _, entry := range users {
		loginVal, _ := entry["Login"].(string)
		resultByLogin[loginVal] = entry
	}

	john := resultByLogin["john.customer"]
	require.NotNil(t, john)
	require.Equal(t, "50", john["PreferredQueueID"])
	require.Equal(t, "VIP Support", john["PreferredQueueName"])

	jane := resultByLogin["jane.customer"]
	require.NotNil(t, jane)
	require.Equal(t, "21", jane["PreferredQueueID"])
	require.Equal(t, "Sales Escalations", jane["PreferredQueueName"])

	require.NoError(t, mock.ExpectationsWereMet())
}
