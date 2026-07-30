package api

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
)

// setupSvcTestDB returns a test DB and the setup service, skipping the test when
// no test database is available (the suite must not hard-fail in environments
// without `make test-db-up`).
func setupSvcTestDB(t *testing.T) (*service.SetupAssistantService, string) {
	t.Helper()
	if err := database.InitTestDB(); err != nil {
		t.Skipf("test database not available: %v", err)
	}
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	svc := service.NewSetupAssistantService(db, nil)
	return svc, fmt.Sprintf("_setup_%d", time.Now().UnixNano()%100000)
}

func TestSetupAssistant_Recce(t *testing.T) {
	svc, _ := setupSvcTestDB(t)
	snap, err := svc.Recce(context.Background())
	require.NoError(t, err)
	require.NotNil(t, snap)
	// Counts are non-negative; the seeded test DB always has some groups/queues.
	assert.GreaterOrEqual(t, snap.Groups, 0)
	assert.GreaterOrEqual(t, snap.Queues, 0)
	assert.GreaterOrEqual(t, snap.Agents, 0)
}

func TestSetupAssistant_CreateGroup(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	name := "SetupTestTeam" + sfx
	t.Cleanup(func() {
		_, _ = database.GetDB() //nolint:errcheck
	})

	id, err := svc.CreateGroup(ctx, name, "created by setup test", 1)
	require.NoError(t, err)
	assert.Greater(t, id, 0)

	// Duplicate name is rejected with a descriptive error.
	_, err = svc.CreateGroup(ctx, name, "", 1)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "exists")

	// Validation: empty name.
	_, err = svc.CreateGroup(ctx, "   ", "", 1)
	require.Error(t, err)
}

func TestSetupAssistant_ExecuteWizard(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	team := "WizardTeam" + sfx
	queue := "WizardQueue" + sfx

	res := svc.ExecuteWizard(ctx, service.WizardRequest{
		Groups: []service.GroupInput{{Name: team}},
		// group_ids: [1] is a 1-based index into the just-created groups → resolves to the team above.
		Queues: []service.QueueInput{{Name: queue, GroupIDs: []int{1}}},
	})
	require.True(t, res.Success, "wizard should succeed, got error: %v", res.Error)
	require.NotEmpty(t, res.Created)

	// Dependency order: the group is created before the queue that references it.
	kinds := make([]string, 0, len(res.Created))
	for _, c := range res.Created {
		kinds = append(kinds, c.Kind)
	}
	assert.Equal(t, "group", kinds[0])
	assert.Contains(t, kinds, "queue")

	// The queue exists in the DB and references the created group.
	db, _ := database.GetDB()
	var qID, qGroup int
	err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT id, group_id FROM queue WHERE name = ?"), queue).Scan(&qID, &qGroup)
	require.NoError(t, err)
	assert.Greater(t, qID, 0)
	assert.Greater(t, qGroup, 0, "queue.group_id must be the created team's id")
}

func TestSetupAssistant_ExecuteWizard_PartialFailure(t *testing.T) {
	svc, _ := setupSvcTestDB(t)
	ctx := context.Background()

	// First group valid, second invalid → wizard stops at the second, reports
	// the error, and lists the first as created.
	res := svc.ExecuteWizard(ctx, service.WizardRequest{
		Groups: []service.GroupInput{
			{Name: "PartialOK_" + fmt.Sprint(time.Now().UnixNano()%100000)},
			{Name: "   "}, // invalid
		},
	})
	assert.False(t, res.Success)
	assert.NotEmpty(t, res.Error)
	require.NotEmpty(t, res.Created, "the valid group must be reported as created")
	assert.Equal(t, "group", res.Created[0].Kind)
}

func TestSetupAssistant_TasksCatalog(t *testing.T) {
	// No DB needed: catalog is deterministic and plugin discovery with a nil
	// manager returns an empty plugin list.
	svc := service.NewSetupAssistantService(nil, nil)
	core := svc.GetCoreTasks()
	assert.Len(t, core, 8, "core task catalog must have 8 built-in tasks")
	plugins := svc.GetPluginTasks()
	assert.Empty(t, plugins, "nil plugin manager → no plugin tasks")
	all := svc.GetAllTasks()
	assert.Len(t, all, len(core)+len(plugins))

	// Core task ids are unique.
	seen := make(map[string]bool, len(core))
	for _, t2 := range core {
		assert.False(t, seen[t2.ID], "duplicate core task id: %s", t2.ID)
		seen[t2.ID] = true
		assert.Equal(t, "setup-assistant", t2.Plugin)
	}
}

func TestSuggestCustomerID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Acme Corporation", "ACME-CORPORATION"},
		{"Smith & Co.", "SMITH-CO"},
		{"  multiple   spaces  ", "MULTIPLE-SPACES"},
		{"Tech/IT_Solutions", "TECH-IT-SOLUTIONS"},
		{"Ünïcödé Ltd", "NCD-LTD"}, // non-ASCII dropped, separators collapse
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, service.SuggestCustomerID(tc.in), "input %q", tc.in)
	}
	// Capped at 50 chars.
	long := strings.Repeat("A", 80)
	assert.LessOrEqual(t, len(service.SuggestCustomerID(long)), 50)
}

func TestSetupAssistant_OnboardCustomer(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	cid := "ONBOARD" + sfx

	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID: cid,
		Name:       "Onboard Test Co " + sfx,
		Country:    "AU",
		Users: []service.CustomerUserInput{
			{Login: "alice" + sfx, Email: "alice" + sfx + "@example.com", FirstName: "Alice", LastName: "A"},
			{Login: "bob" + sfx, Email: "bob" + sfx + "@example.com", FirstName: "Bob", LastName: "B"},
		},
	})
	require.True(t, res.Success, "onboarding should succeed, got error: %v", res.Error)
	assert.Equal(t, cid, res.CustomerID)
	require.Len(t, res.UsersCreated, 2)
	// Each provisioned user has a non-empty temporary password.
	for _, u := range res.UsersCreated {
		assert.NotEmpty(t, u.Password)
	}

	// Company + users exist in the DB and are linked.
	db, _ := database.GetDB()
	var compName string
	err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT name FROM customer_company WHERE customer_id = ?"), cid).Scan(&compName)
	require.NoError(t, err)
	assert.Contains(t, compName, "Onboard Test Co")

	var nUsers int
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM customer_user WHERE customer_id = ?"), cid).Scan(&nUsers)
	require.NoError(t, err)
	assert.Equal(t, 2, nUsers, "two portal users should be linked to the company")

	// Duplicate company id is rejected.
	res2 := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID: cid,
		Name:       "Dup Co",
	})
	assert.False(t, res2.Success)
	assert.Contains(t, strings.ToLower(res2.Error), "exists")
}

func TestSetupAssistant_OnboardCustomer_SuggestsID(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	// No CustomerID provided — should be derived from the name.
	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		Name: "Suggested ID Co " + sfx,
	})
	require.True(t, res.Success, res.Error)
	assert.Equal(t, service.SuggestCustomerID("Suggested ID Co "+sfx), res.CustomerID)
	assert.NotEmpty(t, res.CustomerID)
}

func TestSetupAssistant_OnboardCustomer_WithMailAndGroups(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	cid := "MCOB" + sfx

	// Find a real queue id to target.
	db, _ := database.GetDB()
	var queueID int
	_ = db.QueryRow(database.ConvertPlaceholders("SELECT id FROM queue WHERE valid_id = 1 LIMIT 1")).Scan(&queueID)
	require.Greater(t, queueID, 0, "test DB needs at least one active queue")

	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID: cid,
		Name:       "Mail Co " + sfx,
		ManagingGroupIDs: []int{1},
		MailAccount: &service.MailAccountInput{
			Login: "support" + sfx + "@example.com", Password: "mailboxpass",
			Host: "imap.example.com", AccountType: "IMAP", QueueID: queueID,
		},
	})
	require.True(t, res.Success, "onboarding should succeed, got: %v", res.Error)
	assert.Greater(t, res.MailAccountID, 0)
	assert.Equal(t, []int{1}, res.ManagingGroupIDs)

	// The mailbox exists and is wired to the chosen queue.
	var acctQueue int
	err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT queue_id FROM mail_account WHERE id = ?"), res.MailAccountID).Scan(&acctQueue)
	require.NoError(t, err)
	assert.Equal(t, queueID, acctQueue)

	// group_customer row grants the team access.
	var n int
	_ = db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM group_customer WHERE customer_id = ? AND group_id = 1"), cid).Scan(&n)
	assert.Greater(t, n, 0, "managing team should be linked to the customer")
}

func TestSetupAssistant_OnboardCustomer_CreateQueueForMailbox(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	cid := "QC" + sfx

	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID: cid,
		Name:       "Queue Create Co " + sfx,
		MailAccount: &service.MailAccountInput{
			Login: "support" + sfx + "@example.com", Password: "mailboxpass",
			Host: "imap.example.com", AccountType: "IMAP",
			CreateQueueName: "Auto Queue " + sfx, CreateQueueGroupID: 1,
		},
	})
	require.True(t, res.Success, "onboarding should succeed, got: %v", res.Error)
	assert.Greater(t, res.MailAccountID, 0)
	assert.Greater(t, res.CreatedQueueID, 0, "a new queue should have been created")

	// The mailbox is wired to the newly-created queue.
	db, _ := database.GetDB()
	var acctQueue int
	err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT queue_id FROM mail_account WHERE id = ?"), res.MailAccountID).Scan(&acctQueue)
	require.NoError(t, err)
	assert.Equal(t, res.CreatedQueueID, acctQueue)

	// The created queue exists and is owned by group 1.
	var qGroup int
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT group_id FROM queue WHERE id = ?"), res.CreatedQueueID).Scan(&qGroup)
	require.NoError(t, err)
	assert.Equal(t, 1, qGroup)
}

func TestSetupAssistant_OnboardCustomer_CreateManagingTeam(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()
	cid := "MGT" + sfx
	teamName := "MGT Support " + sfx

	res := svc.OnboardCustomer(ctx, service.OnboardCustomerRequest{
		CustomerID:             cid,
		Name:                   "MGT Co " + sfx,
		CreateManagingGroupName: teamName,
	})
	require.True(t, res.Success, "onboarding should succeed, got: %v", res.Error)
	require.Greater(t, res.CreatedGroupID, 0, "a new team should have been created")
	assert.Equal(t, teamName, res.CreatedGroupName)
	require.Contains(t, res.ManagingGroupIDs, res.CreatedGroupID, "new team should be in managing groups")

	// The team exists and is linked to the customer via group_customer.
	db, _ := database.GetDB()
	var n int
	_ = db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM group_customer WHERE customer_id = ? AND group_id = ?"),
		cid, res.CreatedGroupID).Scan(&n)
	assert.Greater(t, n, 0, "created team should be linked to the customer")
}
