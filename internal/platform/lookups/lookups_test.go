package lookups_test

import (
	"context"
	"database/sql"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/lookups/lookupstest"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("lookup tests need a test database (TEST_DB_HOST)")
	}
	require.NoError(t, database.InitTestDB())
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

// otrsDefaults is the OTRS/Znuny default lookup data every install must carry
// (000002 + 000029 on fresh installs; the OTRS initial insert on imports).
var otrsDefaults = map[lookups.Table][]string{
	lookups.StateType: {"new", "open", "closed", "pending reminder", "pending auto", "removed", "merged"},
	lookups.StateLookup: {"new", "open", "closed successful", "closed unsuccessful", "pending reminder",
		"pending auto close+", "pending auto close-", "removed", "merged"},
	lookups.HistoryType: {"NewTicket", "FollowUp", "SendAutoReject", "SendAutoReply", "SendAutoFollowUp",
		"Forward", "Bounce", "SendAnswer", "SendAgentNotification", "SendCustomerNotification", "EmailAgent",
		"EmailCustomer", "PhoneCallAgent", "PhoneCallCustomer", "AddNote", "Move", "Lock", "Unlock", "Remove",
		"TimeAccounting", "CustomerUpdate", "PriorityUpdate", "OwnerUpdate", "LoopProtection", "Misc",
		"SetPendingTime", "StateUpdate", "TicketDynamicFieldUpdate", "WebRequestCustomer", "TicketLinkAdd",
		"TicketLinkDelete", "SystemRequest", "Merged", "ResponsibleUpdate", "Subscribe", "Unsubscribe",
		"TypeUpdate", "ServiceUpdate", "SLAUpdate", "ArchiveFlagUpdate", "EscalationSolutionTimeStop",
		"EscalationResponseTimeStart", "EscalationUpdateTimeStart", "EscalationSolutionTimeStart",
		"EscalationResponseTimeNotifyBefore", "EscalationUpdateTimeNotifyBefore",
		"EscalationSolutionTimeNotifyBefore", "EscalationResponseTimeStop", "EscalationUpdateTimeStop",
		"TitleUpdate", "EmailResend"},
	lookups.SenderType:   {"agent", "system", "customer"},
	lookups.Channel:      {"Email", "Phone", "Internal", "Chat"},
	lookups.LockType:     {"unlock", "lock", "tmp_lock"},
	lookups.LinkType:     {"Normal", "ParentChild"},
	lookups.LinkState:    {"Valid", "Temporary"},
	lookups.LinkObject:   {"Ticket"},
	lookups.AutoResponse: {"auto reply", "auto reject", "auto follow up", "auto reply/new ticket", "auto remove"},
	lookups.FollowUp:     {"possible", "reject", "new ticket"},
	lookups.ValidLookup:  {"valid", "invalid", "invalid-temporarily"},
}

func TestOTRSDefaultLookupRowsExist(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	for table, names := range otrsDefaults {
		for _, name := range names {
			_, err := lookups.ID(ctx, db, table, name)
			assert.NoError(t, err, "%s row %q missing", table, name)
		}
	}
	// Default states carry the OTRS state type.
	stateTypes := map[string]string{
		"pending auto close+": "pending auto", "pending auto close-": "pending auto",
		"removed": "removed", "merged": "merged",
	}
	for state, typ := range stateTypes {
		id, err := lookups.ID(ctx, db, lookups.StateLookup, state)
		require.NoError(t, err)
		got, err := lookups.StateTypeNameOfState(ctx, db, id)
		require.NoError(t, err)
		assert.Equal(t, typ, got, "state %q", state)
	}
}

func stateNames(t *testing.T, db *sql.DB, subquery string) []string {
	t.Helper()
	rows, err := db.Query(database.ConvertPlaceholders(
		"SELECT name FROM ticket_state WHERE id IN (" + subquery + ") AND name IN " +
			"('new','open','closed successful','closed unsuccessful','pending reminder'," +
			"'pending auto close+','pending auto close-','removed','merged')"))
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		out = append(out, n)
	}
	require.NoError(t, rows.Err())
	sort.Strings(out)
	return out
}

func TestResolutionIndependentOfNumbering(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	for _, n := range lookupstest.StateNumberings {
		t.Run(n.Name, func(t *testing.T) {
			lookupstest.UseStateNumbering(t, db, n)

			for name, id := range n.StateTypes {
				got, err := lookups.ID(ctx, db, lookups.StateType, name)
				require.NoError(t, err)
				assert.Equal(t, id, got, "state type %q", name)
				gotName, err := lookups.Name(ctx, db, lookups.StateType, id)
				require.NoError(t, err)
				assert.Equal(t, name, gotName)
			}
			typeName, err := lookups.StateTypeNameOfState(ctx, db, n.States["pending reminder"])
			require.NoError(t, err)
			assert.Equal(t, "pending reminder", typeName)
			assert.True(t, lookups.IsPendingStateType(typeName))

			assert.Equal(t, []string{"closed successful", "closed unsuccessful"}, stateNames(t, db, lookups.ClosedStateIDsSQL))
			assert.Equal(t, []string{"new", "open"}, stateNames(t, db, lookups.NewOpenStateIDsSQL))
			assert.Equal(t, []string{"pending auto close+", "pending auto close-", "pending reminder"}, stateNames(t, db, lookups.PendingStateIDsSQL))
			assert.Equal(t, []string{"pending reminder"}, stateNames(t, db, lookups.PendingReminderStateIDsSQL))
			assert.Equal(t, []string{"closed successful", "closed unsuccessful", "merged", "removed"}, stateNames(t, db, lookups.FinishedStateIDsSQL))
			assert.Equal(t, []string{"new", "open", "pending auto close+", "pending auto close-", "pending reminder"}, stateNames(t, db, lookups.ViewableStateIDsSQL))
		})
	}
}
