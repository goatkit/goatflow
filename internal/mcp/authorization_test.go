package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/database"
)

// =============================================================================
// MCP AUTHORIZATION TEST FIXTURES
// =============================================================================

// MCPTestFixtures holds test data for MCP authorization tests
type MCPTestFixtures struct {
	db *sql.DB
	mu sync.Mutex

	// Groups
	GroupAdmin   int // Admin group (ID 2 in standard GoatFlow)
	GroupSupport int
	GroupBilling int

	// Queues
	QueueSupport int
	QueueBilling int

	// Agents
	AgentAdmin    int // In admin group - can use execute_sql
	AgentSupport  int // rw on Support only - cannot use execute_sql
	AgentBilling  int // rw on Billing only
	AgentReadOnly int // ro on Support only

	// Customer Companies
	CompanyAcme     string
	CompanyNovaBank string

	// Customer Users
	CustomerAcme     string
	CustomerNovaBank string

	// Tickets
	TicketAcmeSupport      int64
	TicketNovaBankSupport  int64
	TicketAcmeBilling      int64
	TicketNovaBankBilling  int64
}

var (
	mcpFixtures     *MCPTestFixtures
	mcpFixturesOnce sync.Once
	mcpFixturesErr  error
)

// getMCPFixtures returns shared test fixtures, creating them once
func getMCPFixtures(t *testing.T) *MCPTestFixtures {
	t.Helper()
	requireDatabase(t)

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skip("Database not available, skipping MCP authorization tests")
	}

	mcpFixturesOnce.Do(func() {
		mcpFixtures = &MCPTestFixtures{db: db}
		mcpFixturesErr = mcpFixtures.setup()
	})

	if mcpFixturesErr != nil {
		t.Skipf("Failed to setup MCP fixtures: %v", mcpFixturesErr)
	}

	return mcpFixtures
}

func (f *MCPTestFixtures) setup() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()

	// Helper for exec operations
	exec := func(query string, args ...interface{}) error {
		_, err := f.db.Exec(database.ConvertPlaceholders(query), args...)
		return err
	}

	// Use high IDs to avoid conflicts (80000+ range for MCP tests)
	f.GroupAdmin = 2 // Standard admin group in GoatFlow
	f.GroupSupport = 80001
	f.GroupBilling = 80002
	f.QueueSupport = 80001
	f.QueueBilling = 80002

	// Clean up any existing test data
	_, _ = f.db.Exec("SET FOREIGN_KEY_CHECKS=0")
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id >= 80000 AND id < 90000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM queue WHERE id >= 80000 AND id < 90000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id >= 80000 AND user_id < 90000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM group_customer WHERE customer_id LIKE 'mcptest-%'"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login LIKE '%mcptest%'"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM customer_company WHERE customer_id LIKE 'mcptest-%'"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM `groups` WHERE id >= 80000 AND id < 90000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id >= 80000 AND id < 90000"))
	_, _ = f.db.Exec("SET FOREIGN_KEY_CHECKS=1")

	suffix := fmt.Sprintf("_%d", time.Now().UnixNano()%100000)

	// -------------------------------------------------------------------------
	// 1. Create Groups
	// -------------------------------------------------------------------------
	groups := []struct {
		id   int
		name string
	}{
		{f.GroupSupport, "MCPTest-Support" + suffix},
		{f.GroupBilling, "MCPTest-Billing" + suffix},
	}

	for _, g := range groups {
		if err := exec(
			"INSERT INTO `groups` (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (?, ?, 'MCP authorization test group', 1, ?, 1, ?, 1)",
			g.id, g.name, now, now,
		); err != nil {
			return fmt.Errorf("failed to create group %s: %w", g.name, err)
		}
	}

	// -------------------------------------------------------------------------
	// 2. Create Queues
	// -------------------------------------------------------------------------
	queues := []struct {
		id      int
		name    string
		groupID int
	}{
		{f.QueueSupport, "MCPTest-Support-Queue" + suffix, f.GroupSupport},
		{f.QueueBilling, "MCPTest-Billing-Queue" + suffix, f.GroupBilling},
	}

	for _, q := range queues {
		if err := exec(
			`INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, 
				first_response_notify, update_time, update_notify, solution_time, solution_notify,
				system_address_id, calendar_name, default_sign_key, salutation_id, signature_id,
				follow_up_id, follow_up_lock, comments, valid_id, 
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, 0, 0, 0, 0, 0, 0, 0, 1, NULL, NULL, 1, 1, 1, 0, 'MCP test queue', 1, ?, 1, ?, 1)`,
			q.id, q.name, q.groupID, now, now,
		); err != nil {
			return fmt.Errorf("failed to create queue %s: %w", q.name, err)
		}
	}

	// -------------------------------------------------------------------------
	// 3. Create Agents
	// -------------------------------------------------------------------------
	f.AgentAdmin = 80001
	f.AgentSupport = 80002
	f.AgentBilling = 80003
	f.AgentReadOnly = 80004

	agents := []struct {
		id    int
		login string
	}{
		{f.AgentAdmin, "mcptest-admin" + suffix},
		{f.AgentSupport, "mcptest-agent-support" + suffix},
		{f.AgentBilling, "mcptest-agent-billing" + suffix},
		{f.AgentReadOnly, "mcptest-agent-readonly" + suffix},
	}

	for _, a := range agents {
		if err := exec(
			`INSERT INTO users (id, login, pw, first_name, last_name, valid_id, 
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'test', 'Test', 'Agent', 1, ?, 1, ?, 1)`,
			a.id, a.login, now, now,
		); err != nil {
			return fmt.Errorf("failed to create agent %s: %w", a.login, err)
		}
	}

	// -------------------------------------------------------------------------
	// 4. Assign Agent Group Permissions
	// -------------------------------------------------------------------------
	agentPerms := []struct {
		userID  int
		groupID int
		perm    string
	}{
		// Admin gets admin group membership + rw on all test queues
		{f.AgentAdmin, f.GroupAdmin, "rw"},     // Admin group membership!
		{f.AgentAdmin, f.GroupSupport, "rw"},
		{f.AgentAdmin, f.GroupBilling, "rw"},

		// Support agent: rw on Support only, no admin group
		{f.AgentSupport, f.GroupSupport, "rw"},

		// Billing agent: rw on Billing only, no admin group
		{f.AgentBilling, f.GroupBilling, "rw"},

		// ReadOnly agent: ro on Support only
		{f.AgentReadOnly, f.GroupSupport, "ro"},
	}

	for _, ap := range agentPerms {
		if err := exec(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, ?, 1)
		`, ap.userID, ap.groupID, ap.perm, now, now); err != nil && !strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("failed to assign permission: %w", err)
		}
	}

	// -------------------------------------------------------------------------
	// 5. Create Customer Companies
	// -------------------------------------------------------------------------
	f.CompanyAcme = "mcptest-acme-corp"
	f.CompanyNovaBank = "mcptest-novabank"

	for _, c := range []string{f.CompanyAcme, f.CompanyNovaBank} {
		if err := exec(`
			INSERT INTO customer_company (customer_id, name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, ?, 1, ?, 1)
		`, c, c, now, now); err != nil && !strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("failed to create company %s: %w", c, err)
		}
	}

	// -------------------------------------------------------------------------
	// 6. Create Customer Users
	// -------------------------------------------------------------------------
	f.CustomerAcme = "alice@mcptest-acme.com"
	f.CustomerNovaBank = "bob@mcptest-novabank.com"

	customers := []struct {
		login   string
		company string
	}{
		{f.CustomerAcme, f.CompanyAcme},
		{f.CustomerNovaBank, f.CompanyNovaBank},
	}

	for _, cu := range customers {
		if err := exec(`
			INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id,
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, 'test', 'Test', 'Customer', 1, ?, 1, ?, 1)
		`, cu.login, cu.login, cu.company, now, now); err != nil && !strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("failed to create customer %s: %w", cu.login, err)
		}
	}

	// -------------------------------------------------------------------------
	// 7. Assign Customer Company Group Permissions
	// -------------------------------------------------------------------------
	// Acme -> Support (ro)
	_ = exec(`
		INSERT INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'ro', 1, 'Ticket', ?, 1, ?, 1)
	`, f.CompanyAcme, f.GroupSupport, now, now)

	// NovaBank -> Billing (ro)
	_ = exec(`
		INSERT INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'ro', 1, 'Ticket', ?, 1, ?, 1)
	`, f.CompanyNovaBank, f.GroupBilling, now, now)

	// -------------------------------------------------------------------------
	// 8. Create Test Tickets
	// -------------------------------------------------------------------------
	f.TicketAcmeSupport = 80001
	f.TicketNovaBankSupport = 80002
	f.TicketAcmeBilling = 80003
	f.TicketNovaBankBilling = 80004

	tickets := []struct {
		id         int64
		tn         string
		title      string
		queueID    int
		customerID string
	}{
		{f.TicketAcmeSupport, "MCP80001", "Acme Support Ticket", f.QueueSupport, f.CompanyAcme},
		{f.TicketNovaBankSupport, "MCP80002", "NovaBank Support Ticket", f.QueueSupport, f.CompanyNovaBank},
		{f.TicketAcmeBilling, "MCP80003", "Acme Billing Ticket", f.QueueBilling, f.CompanyAcme},
		{f.TicketNovaBankBilling, "MCP80004", "NovaBank Billing Ticket", f.QueueBilling, f.CompanyNovaBank},
	}

	for _, tk := range tickets {
		if err := exec(
			`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, 
				ticket_state_id, ticket_priority_id, customer_id, customer_user_id,
				user_id, responsible_user_id, timeout, until_time, escalation_time,
				escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, 1, 1, 3, ?, '', 1, 1, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)`,
			tk.id, tk.tn, tk.title, tk.queueID, tk.customerID, now, now,
		); err != nil {
			return fmt.Errorf("failed to create ticket %s: %w", tk.title, err)
		}
	}

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// callMCPTool executes an MCP tool call and returns the result
func callMCPTool(t *testing.T, server *Server, toolName string, args map[string]any) (*ToolCallResult, error) {
	t.Helper()

	params := ToolCallParams{
		Name:      toolName,
		Arguments: args,
	}
	paramsJSON, _ := json.Marshal(params)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	reqBytes, _ := json.Marshal(req)
	respBytes, err := server.HandleMessage(context.Background(), reqBytes)
	if err != nil {
		return nil, err
	}

	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", resp.Error.Message)
	}

	// Parse result
	resultBytes, _ := json.Marshal(resp.Result)
	var result ToolCallResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return &result, nil
}

// getResultText extracts text content from a ToolCallResult
func getResultText(result *ToolCallResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text
		}
	}
	return ""
}

// =============================================================================
// EXECUTE_SQL ADMIN GATE TESTS
// =============================================================================

func TestExecuteSQL_AdminOnly(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("admin user can execute SQL", func(t *testing.T) {
		// Create server as admin user (in admin group)
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		result, err := callMCPTool(t, server, "execute_sql", map[string]any{
			"query": "SELECT 1 as test",
		})

		require.NoError(t, err, "Admin should be able to execute SQL")
		assert.False(t, result.IsError, "Result should not be an error")

		text := getResultText(result)
		assert.Contains(t, text, "test", "Should return query result")
	})

	t.Run("non-admin agent cannot execute SQL", func(t *testing.T) {
		// Create server as support agent (not in admin group)
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")

		result, err := callMCPTool(t, server, "execute_sql", map[string]any{
			"query": "SELECT 1 as test",
		})

		// The error comes back as a result with IsError=true
		require.NoError(t, err, "Should not return protocol error")
		assert.True(t, result.IsError, "Result should be an error")

		text := getResultText(result)
		assert.Contains(t, text, "admin group membership", "Should mention admin requirement")
	})

	t.Run("billing agent cannot execute SQL", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentBilling, "mcptest-agent-billing")

		result, err := callMCPTool(t, server, "execute_sql", map[string]any{
			"query": "SELECT 1 as test",
		})

		require.NoError(t, err)
		assert.True(t, result.IsError, "Billing agent should not be able to execute SQL")
		assert.Contains(t, getResultText(result), "admin group membership")
	})

	t.Run("readonly agent cannot execute SQL", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentReadOnly, "mcptest-agent-readonly")

		result, err := callMCPTool(t, server, "execute_sql", map[string]any{
			"query": "SELECT 1 as test",
		})

		require.NoError(t, err)
		assert.True(t, result.IsError, "ReadOnly agent should not be able to execute SQL")
		assert.Contains(t, getResultText(result), "admin group membership")
	})
}

func TestExecuteSQL_StillRejectsMutations(t *testing.T) {
	fixtures := getMCPFixtures(t)

	// Even admin cannot run non-SELECT queries
	server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

	tests := []struct {
		name  string
		query string
	}{
		{"INSERT blocked", "INSERT INTO ticket (title) VALUES ('test')"},
		{"UPDATE blocked", "UPDATE ticket SET title = 'hacked'"},
		{"DELETE blocked", "DELETE FROM ticket WHERE id = 1"},
		{"DROP blocked", "DROP TABLE ticket"},
		{"TRUNCATE blocked", "TRUNCATE TABLE ticket"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := callMCPTool(t, server, "execute_sql", map[string]any{
				"query": tt.query,
			})

			require.NoError(t, err)
			assert.True(t, result.IsError, "Mutation should be blocked")
			assert.Contains(t, getResultText(result), "SELECT", "Should mention only SELECT allowed")
		})
	}
}

// =============================================================================
// AGENT QUEUE ACCESS TESTS
// =============================================================================

func TestListTickets_AgentQueueAccess(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("admin sees tickets in all queues", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		result, err := callMCPTool(t, server, "list_tickets", map[string]any{
			"limit": 100,
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := getResultText(result)
		// Admin should see all test tickets
		assert.Contains(t, text, "MCP80001", "Admin should see Acme Support ticket")
		assert.Contains(t, text, "MCP80002", "Admin should see NovaBank Support ticket")
		assert.Contains(t, text, "MCP80003", "Admin should see Acme Billing ticket")
		assert.Contains(t, text, "MCP80004", "Admin should see NovaBank Billing ticket")
	})

	t.Run("support agent sees only support queue tickets", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")

		result, err := callMCPTool(t, server, "list_tickets", map[string]any{
			"limit": 100,
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := getResultText(result)
		// Support agent should only see Support queue tickets
		assert.Contains(t, text, "MCP80001", "Support agent should see Acme Support ticket")
		assert.Contains(t, text, "MCP80002", "Support agent should see NovaBank Support ticket")
		// Should NOT see Billing queue tickets
		assert.NotContains(t, text, "MCP80003", "Support agent should NOT see Acme Billing ticket")
		assert.NotContains(t, text, "MCP80004", "Support agent should NOT see NovaBank Billing ticket")
	})

	t.Run("billing agent sees only billing queue tickets", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentBilling, "mcptest-agent-billing")

		result, err := callMCPTool(t, server, "list_tickets", map[string]any{
			"limit": 100,
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := getResultText(result)
		// Billing agent should only see Billing queue tickets
		assert.NotContains(t, text, "MCP80001", "Billing agent should NOT see Acme Support ticket")
		assert.NotContains(t, text, "MCP80002", "Billing agent should NOT see NovaBank Support ticket")
		assert.Contains(t, text, "MCP80003", "Billing agent should see Acme Billing ticket")
		assert.Contains(t, text, "MCP80004", "Billing agent should see NovaBank Billing ticket")
	})
}

func TestGetTicket_AgentQueueAccess(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("agent can get ticket in accessible queue", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")

		result, err := callMCPTool(t, server, "get_ticket", map[string]any{
			"ticket_id": fixtures.TicketAcmeSupport,
		})

		require.NoError(t, err)
		assert.False(t, result.IsError, "Support agent should access Support queue ticket")
		assert.Contains(t, getResultText(result), "Acme Support Ticket")
	})

	t.Run("agent cannot get ticket in inaccessible queue", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")

		result, err := callMCPTool(t, server, "get_ticket", map[string]any{
			"ticket_id": fixtures.TicketAcmeBilling, // Billing queue - no access
		})

		require.NoError(t, err)
		// Should return error or not found
		assert.True(t, result.IsError, "Support agent should NOT access Billing queue ticket")
	})
}

func TestSearchTickets_AgentQueueAccess(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("search results filtered by queue access", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")

		result, err := callMCPTool(t, server, "search_tickets", map[string]any{
			"query": "Acme", // Would match both Support and Billing tickets
			"limit": 100,
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := getResultText(result)
		// Should find Acme Support but NOT Acme Billing
		assert.Contains(t, text, "Acme Support", "Should find Acme Support ticket")
		assert.NotContains(t, text, "Acme Billing", "Should NOT find Acme Billing ticket")
	})
}

// =============================================================================
// CUSTOMER COMPANY ISOLATION TESTS  
// =============================================================================

// Note: These tests require MCP to support customer tokens.
// Currently MCP is designed for agent tokens only.
// These are placeholder tests for when customer MCP access is implemented.

func TestListTickets_CustomerCompanyIsolation(t *testing.T) {
	t.Skip("Customer MCP access not yet implemented - MCP is currently agent-only")

	// When implemented:
	// - Customer should only see tickets from their own company
	// - Customer should NOT see tickets from other companies
}

// =============================================================================
// MULTI-USER PROXY BEHAVIOR TESTS
// =============================================================================

func TestMCPServer_MultiUserProxy(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("different users see different results", func(t *testing.T) {
		// Same tool call, different users, different results

		// Admin user
		adminServer := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")
		adminResult, _ := callMCPTool(t, adminServer, "list_tickets", map[string]any{"limit": 100})
		adminText := getResultText(adminResult)

		// Support user
		supportServer := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")
		supportResult, _ := callMCPTool(t, supportServer, "list_tickets", map[string]any{"limit": 100})
		supportText := getResultText(supportResult)

		// Admin should see more tickets than support agent
		// (Both see Support tickets, only admin sees Billing tickets)
		assert.Contains(t, adminText, "MCP80003", "Admin sees Billing ticket")
		assert.NotContains(t, supportText, "MCP80003", "Support does NOT see Billing ticket")
	})

	t.Run("server instances are isolated", func(t *testing.T) {
		// Creating a server for one user doesn't affect another

		server1 := NewServer(fixtures.db, fixtures.AgentAdmin, "admin")
		server2 := NewServer(fixtures.db, fixtures.AgentSupport, "support")

		// Verify they have different user contexts
		assert.Equal(t, fixtures.AgentAdmin, server1.userID)
		assert.Equal(t, fixtures.AgentSupport, server2.userID)
	})
}

// =============================================================================
// PERMISSION SERVICE INTEGRATION TESTS
// =============================================================================

func TestPermissionService_IsInGroup(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("admin is in admin group", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "admin")
		isAdmin, err := server.permissions.IsInGroup(fixtures.AgentAdmin, "admin")
		require.NoError(t, err)
		assert.True(t, isAdmin, "Admin agent should be in admin group")
	})

	t.Run("support agent is not in admin group", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "support")
		isAdmin, err := server.permissions.IsInGroup(fixtures.AgentSupport, "admin")
		require.NoError(t, err)
		assert.False(t, isAdmin, "Support agent should NOT be in admin group")
	})

	t.Run("billing agent is not in admin group", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentBilling, "billing")
		isAdmin, err := server.permissions.IsInGroup(fixtures.AgentBilling, "admin")
		require.NoError(t, err)
		assert.False(t, isAdmin, "Billing agent should NOT be in admin group")
	})
}

// =============================================================================
// TOOL VISIBILITY TESTS
// =============================================================================

func TestToolsList_AllUsersSeeSameTools(t *testing.T) {
	fixtures := getMCPFixtures(t)

	// All users should see the same tool list (access control is at execution time)

	users := []struct {
		id    int
		login string
	}{
		{fixtures.AgentAdmin, "admin"},
		{fixtures.AgentSupport, "support"},
		{fixtures.AgentBilling, "billing"},
	}

	var lastToolCount int
	for _, u := range users {
		server := NewServer(fixtures.db, u.id, u.login)

		req := Request{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/list",
		}
		reqBytes, _ := json.Marshal(req)
		respBytes, _ := server.HandleMessage(context.Background(), reqBytes)

		var resp Response
		_ = json.Unmarshal(respBytes, &resp)

		result := resp.Result.(map[string]any)
		tools := result["tools"].([]any)

		if lastToolCount == 0 {
			lastToolCount = len(tools)
		} else {
			assert.Equal(t, lastToolCount, len(tools), "All users should see same number of tools")
		}
	}
}

// =============================================================================
// CREATE/UPDATE/ARTICLE PERMISSION TESTS
// =============================================================================

func TestCreateTicket_QueuePermissions(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("agent with create permission can create ticket", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		result, err := callMCPTool(t, server, "create_ticket", map[string]any{
			"title":    "Test ticket from MCP",
			"queue_id": fixtures.QueueSupport,
			"body":     "This is a test ticket created via MCP",
		})

		require.NoError(t, err)
		assert.False(t, result.IsError, "Admin should create ticket in Support queue")
		
		// Verify ticket was actually created with correct values
		text := getResultText(result)
		assert.Contains(t, text, "ticket_id")
		assert.Contains(t, text, "ticket_number")
		assert.Contains(t, text, "article_id")
		
		// Parse result to get ticket ID and verify in DB
		var resultMap map[string]any
		json.Unmarshal([]byte(text), &resultMap)
		ticketID := int64(resultMap["ticket_id"].(float64))
		
		// Verify ticket exists with correct title and queue
		var title string
		var queueID int
		err = fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT title, queue_id FROM ticket WHERE id = ?"),
			ticketID).Scan(&title, &queueID)
		require.NoError(t, err, "Ticket should exist in database")
		assert.Equal(t, "Test ticket from MCP", title)
		assert.Equal(t, fixtures.QueueSupport, queueID)
		
		// Verify article was created with correct body
		articleID := int64(resultMap["article_id"].(float64))
		var body string
		err = fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT a_body FROM article_data_mime WHERE article_id = ?"),
			articleID).Scan(&body)
		require.NoError(t, err, "Article should exist in database")
		assert.Equal(t, "This is a test ticket created via MCP", body)
	})

	t.Run("agent without create permission cannot create ticket", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")

		// Count tickets before attempt
		var countBefore int
		fixtures.db.QueryRow("SELECT COUNT(*) FROM ticket WHERE title = 'Should fail - no permission'").Scan(&countBefore)

		result, err := callMCPTool(t, server, "create_ticket", map[string]any{
			"title":    "Should fail - no permission",
			"queue_id": fixtures.QueueBilling, // No permission here
			"body":     "This should be blocked",
		})

		require.NoError(t, err)
		assert.True(t, result.IsError, "Support agent should NOT create in Billing queue")
		assert.Contains(t, getResultText(result), "no permission")
		
		// Verify ticket was NOT created
		var countAfter int
		fixtures.db.QueryRow("SELECT COUNT(*) FROM ticket WHERE title = 'Should fail - no permission'").Scan(&countAfter)
		assert.Equal(t, countBefore, countAfter, "No ticket should have been created")
	})

	t.Run("missing required fields returns error", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		// Missing title
		result, _ := callMCPTool(t, server, "create_ticket", map[string]any{
			"queue_id": fixtures.QueueSupport,
			"body":     "Test body",
		})
		assert.True(t, result.IsError)
		assert.Contains(t, getResultText(result), "required")

		// Missing queue_id
		result, _ = callMCPTool(t, server, "create_ticket", map[string]any{
			"title": "Test",
			"body":  "Test body",
		})
		assert.True(t, result.IsError)
		assert.Contains(t, getResultText(result), "required")

		// Missing body
		result, _ = callMCPTool(t, server, "create_ticket", map[string]any{
			"title":    "Test",
			"queue_id": fixtures.QueueSupport,
		})
		assert.True(t, result.IsError)
		assert.Contains(t, getResultText(result), "required")
	})
}

func TestUpdateTicket_Permissions(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("agent can update ticket title and verify change", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")
		
		// Get original title
		var originalTitle string
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT title FROM ticket WHERE id = ?"),
			fixtures.TicketAcmeSupport).Scan(&originalTitle)

		newTitle := fmt.Sprintf("Updated title via MCP %d", time.Now().UnixNano())
		result, err := callMCPTool(t, server, "update_ticket", map[string]any{
			"ticket_id": fixtures.TicketAcmeSupport,
			"title":     newTitle,
		})

		require.NoError(t, err)
		assert.False(t, result.IsError, "Support agent should update ticket in Support queue")
		assert.Contains(t, getResultText(result), "updated")
		
		// Verify title was actually changed in database
		var updatedTitle string
		err = fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT title FROM ticket WHERE id = ?"),
			fixtures.TicketAcmeSupport).Scan(&updatedTitle)
		require.NoError(t, err)
		assert.Equal(t, newTitle, updatedTitle, "Title should be updated in database")
		assert.NotEqual(t, originalTitle, updatedTitle, "Title should have changed")
	})

	t.Run("agent cannot update ticket in inaccessible queue", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")
		
		// Get original title to verify it doesn't change
		var originalTitle string
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT title FROM ticket WHERE id = ?"),
			fixtures.TicketAcmeBilling).Scan(&originalTitle)

		result, err := callMCPTool(t, server, "update_ticket", map[string]any{
			"ticket_id": fixtures.TicketAcmeBilling, // Billing queue - no access
			"title":     "Should fail - unauthorized",
		})

		require.NoError(t, err)
		assert.True(t, result.IsError, "Support agent should NOT update Billing ticket")
		assert.Contains(t, getResultText(result), "not found") // Security: 404 not 403
		
		// Verify title was NOT changed
		var unchangedTitle string
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT title FROM ticket WHERE id = ?"),
			fixtures.TicketAcmeBilling).Scan(&unchangedTitle)
		assert.Equal(t, originalTitle, unchangedTitle, "Title should NOT have changed")
	})

	t.Run("agent cannot move ticket to queue without move_into permission", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")
		
		// Verify ticket is in Support queue before
		var queueBefore int
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT queue_id FROM ticket WHERE id = ?"),
			fixtures.TicketAcmeSupport).Scan(&queueBefore)
		assert.Equal(t, fixtures.QueueSupport, queueBefore)

		result, err := callMCPTool(t, server, "update_ticket", map[string]any{
			"ticket_id": fixtures.TicketAcmeSupport,
			"queue_id":  fixtures.QueueBilling, // No move_into permission
		})

		require.NoError(t, err)
		assert.True(t, result.IsError, "Support agent should NOT move ticket to Billing")
		assert.Contains(t, getResultText(result), "no permission")
		
		// Verify ticket is still in Support queue
		var queueAfter int
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT queue_id FROM ticket WHERE id = ?"),
			fixtures.TicketAcmeSupport).Scan(&queueAfter)
		assert.Equal(t, fixtures.QueueSupport, queueAfter, "Ticket should NOT have moved")
	})

	t.Run("admin can move ticket between queues", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")
		
		// Create a ticket we can move (don't mess with fixture tickets)
		createResult, _ := callMCPTool(t, server, "create_ticket", map[string]any{
			"title":    "Ticket to move",
			"queue_id": fixtures.QueueSupport,
			"body":     "Testing move_into permission",
		})
		var createMap map[string]any
		json.Unmarshal([]byte(getResultText(createResult)), &createMap)
		ticketID := int(createMap["ticket_id"].(float64))

		// Move to Billing queue
		result, err := callMCPTool(t, server, "update_ticket", map[string]any{
			"ticket_id": ticketID,
			"queue_id":  fixtures.QueueBilling,
		})

		require.NoError(t, err)
		assert.False(t, result.IsError, "Admin should move ticket")
		
		// Verify ticket moved
		var newQueueID int
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT queue_id FROM ticket WHERE id = ?"),
			ticketID).Scan(&newQueueID)
		assert.Equal(t, fixtures.QueueBilling, newQueueID, "Ticket should be in Billing queue")
	})

	t.Run("missing ticket_id returns error", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		result, _ := callMCPTool(t, server, "update_ticket", map[string]any{
			"title": "Test",
		})
		assert.True(t, result.IsError)
		assert.Contains(t, getResultText(result), "ticket_id")
	})

	t.Run("no fields to update returns error", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		result, _ := callMCPTool(t, server, "update_ticket", map[string]any{
			"ticket_id": fixtures.TicketAcmeSupport,
		})
		assert.True(t, result.IsError)
		assert.Contains(t, getResultText(result), "no fields")
	})

	t.Run("nonexistent ticket returns not found", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		result, _ := callMCPTool(t, server, "update_ticket", map[string]any{
			"ticket_id": 99999999,
			"title":     "Test",
		})
		assert.True(t, result.IsError)
		assert.Contains(t, getResultText(result), "not found")
	})
}

func TestAddArticle_Permissions(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("agent can add article and verify content", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")
		
		uniqueBody := fmt.Sprintf("Test note added via MCP at %d", time.Now().UnixNano())
		result, err := callMCPTool(t, server, "add_article", map[string]any{
			"ticket_id": fixtures.TicketAcmeSupport,
			"subject":   "MCP Test Subject",
			"body":      uniqueBody,
		})

		require.NoError(t, err)
		assert.False(t, result.IsError, "Admin should add article to Support ticket")
		
		// Parse result
		text := getResultText(result)
		assert.Contains(t, text, "article_id")
		
		var resultMap map[string]any
		json.Unmarshal([]byte(text), &resultMap)
		articleID := int64(resultMap["article_id"].(float64))
		
		// Verify article exists with correct content
		var subject, body string
		err = fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT a_subject, a_body FROM article_data_mime WHERE article_id = ?"),
			articleID).Scan(&subject, &body)
		require.NoError(t, err, "Article should exist in database")
		assert.Equal(t, "MCP Test Subject", subject)
		assert.Equal(t, uniqueBody, body)
		
		// Verify article is linked to correct ticket
		var ticketID int64
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT ticket_id FROM article WHERE id = ?"),
			articleID).Scan(&ticketID)
		assert.Equal(t, int64(fixtures.TicketAcmeSupport), ticketID)
	})

	t.Run("article defaults subject to ticket title if not provided", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")
		
		// Get ticket title
		var ticketTitle string
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT title FROM ticket WHERE id = ?"),
			fixtures.TicketAcmeSupport).Scan(&ticketTitle)
		
		result, err := callMCPTool(t, server, "add_article", map[string]any{
			"ticket_id": fixtures.TicketAcmeSupport,
			"body":      "Article without explicit subject",
			// No subject provided
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)
		
		var resultMap map[string]any
		json.Unmarshal([]byte(getResultText(result)), &resultMap)
		articleID := int64(resultMap["article_id"].(float64))
		
		// Verify subject defaulted to ticket title
		var subject string
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT a_subject FROM article_data_mime WHERE article_id = ?"),
			articleID).Scan(&subject)
		assert.Equal(t, ticketTitle, subject, "Subject should default to ticket title")
	})

	t.Run("internal article is not visible to customer", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")
		
		result, err := callMCPTool(t, server, "add_article", map[string]any{
			"ticket_id":    fixtures.TicketAcmeSupport,
			"body":         "Internal note - not for customers",
			"article_type": "note-internal",
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)
		
		var resultMap map[string]any
		json.Unmarshal([]byte(getResultText(result)), &resultMap)
		articleID := int64(resultMap["article_id"].(float64))
		
		// Verify visibility
		var isVisible int
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT is_visible_for_customer FROM article WHERE id = ?"),
			articleID).Scan(&isVisible)
		assert.Equal(t, 0, isVisible, "Internal article should NOT be visible to customer")
	})

	t.Run("external article is visible to customer", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")
		
		result, err := callMCPTool(t, server, "add_article", map[string]any{
			"ticket_id":    fixtures.TicketAcmeSupport,
			"body":         "External note - visible to customers",
			"article_type": "note-external",
		})

		require.NoError(t, err)
		assert.False(t, result.IsError)
		
		var resultMap map[string]any
		json.Unmarshal([]byte(getResultText(result)), &resultMap)
		articleID := int64(resultMap["article_id"].(float64))
		
		// Verify visibility
		var isVisible int
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT is_visible_for_customer FROM article WHERE id = ?"),
			articleID).Scan(&isVisible)
		assert.Equal(t, 1, isVisible, "External article SHOULD be visible to customer")
	})

	t.Run("agent cannot add article to inaccessible ticket", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")
		
		// Count articles before
		var countBefore int
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT COUNT(*) FROM article WHERE ticket_id = ?"),
			fixtures.TicketAcmeBilling).Scan(&countBefore)

		result, err := callMCPTool(t, server, "add_article", map[string]any{
			"ticket_id": fixtures.TicketAcmeBilling, // Billing queue - no access
			"body":      "Should fail",
		})

		require.NoError(t, err)
		assert.True(t, result.IsError, "Support agent should NOT add article to Billing ticket")
		assert.Contains(t, getResultText(result), "not found") // Security: 404 not 403
		
		// Verify no article was created
		var countAfter int
		fixtures.db.QueryRow(
			database.ConvertPlaceholders("SELECT COUNT(*) FROM article WHERE ticket_id = ?"),
			fixtures.TicketAcmeBilling).Scan(&countAfter)
		assert.Equal(t, countBefore, countAfter, "No article should have been created")
	})

	t.Run("missing required fields returns error", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		// Missing ticket_id
		result, _ := callMCPTool(t, server, "add_article", map[string]any{
			"body": "Test",
		})
		assert.True(t, result.IsError)
		assert.Contains(t, getResultText(result), "required")

		// Missing body
		result, _ = callMCPTool(t, server, "add_article", map[string]any{
			"ticket_id": fixtures.TicketAcmeSupport,
		})
		assert.True(t, result.IsError)
		assert.Contains(t, getResultText(result), "required")
	})

	t.Run("nonexistent ticket returns not found", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		result, _ := callMCPTool(t, server, "add_article", map[string]any{
			"ticket_id": 99999999,
			"body":      "Test",
		})
		assert.True(t, result.IsError)
		assert.Contains(t, getResultText(result), "not found")
	})
}

// =============================================================================
// INFORMATION DISCLOSURE VULNERABILITY TESTS
// =============================================================================
// These tests verify that tools don't leak information about resources
// the user doesn't have access to.

func TestListQueues_OnlyShowsAccessibleQueues(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("support agent only sees queues they have access to", func(t *testing.T) {
		// Support agent has access to Support queue only, NOT Billing
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")

		result, err := callMCPTool(t, server, "list_queues", map[string]any{})

		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := getResultText(result)
		// Should see Support queue
		assert.Contains(t, text, "MCPTest-Support", "Support agent should see Support queue")
		// Should NOT see Billing queue - this is the vulnerability
		assert.NotContains(t, text, "MCPTest-Billing", "Support agent should NOT see Billing queue")
	})

	t.Run("billing agent only sees queues they have access to", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentBilling, "mcptest-agent-billing")

		result, err := callMCPTool(t, server, "list_queues", map[string]any{})

		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := getResultText(result)
		// Should see Billing queue
		assert.Contains(t, text, "MCPTest-Billing", "Billing agent should see Billing queue")
		// Should NOT see Support queue
		assert.NotContains(t, text, "MCPTest-Support", "Billing agent should NOT see Support queue")
	})

	t.Run("admin sees all queues", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")

		result, err := callMCPTool(t, server, "list_queues", map[string]any{})

		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := getResultText(result)
		assert.Contains(t, text, "MCPTest-Support", "Admin should see Support queue")
		assert.Contains(t, text, "MCPTest-Billing", "Admin should see Billing queue")
	})
}

func TestGetStatistics_OnlyShowsAccessibleData(t *testing.T) {
	fixtures := getMCPFixtures(t)

	t.Run("support agent stats only include accessible queues", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")

		result, err := callMCPTool(t, server, "get_statistics", map[string]any{})

		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := getResultText(result)
		// tickets_by_queue should NOT contain Billing queue
		assert.NotContains(t, text, "MCPTest-Billing", "Stats should NOT show Billing queue data")
		// But should contain Support if there are tickets there
		// (May or may not contain Support depending on test data)
	})

	t.Run("billing agent stats only include accessible queues", func(t *testing.T) {
		server := NewServer(fixtures.db, fixtures.AgentBilling, "mcptest-agent-billing")

		result, err := callMCPTool(t, server, "get_statistics", map[string]any{})

		require.NoError(t, err)
		assert.False(t, result.IsError)

		text := getResultText(result)
		// tickets_by_queue should NOT contain Support queue
		assert.NotContains(t, text, "MCPTest-Support", "Stats should NOT show Support queue data")
	})

	t.Run("different users see different ticket counts", func(t *testing.T) {
		// Admin sees all tickets
		adminServer := NewServer(fixtures.db, fixtures.AgentAdmin, "mcptest-admin")
		adminResult, _ := callMCPTool(t, adminServer, "get_statistics", map[string]any{})
		
		// Support agent sees only Support queue tickets  
		supportServer := NewServer(fixtures.db, fixtures.AgentSupport, "mcptest-agent-support")
		supportResult, _ := callMCPTool(t, supportServer, "get_statistics", map[string]any{})

		// Parse total_tickets from both
		var adminStats, supportStats map[string]any
		json.Unmarshal([]byte(getResultText(adminResult)), &adminStats)
		json.Unmarshal([]byte(getResultText(supportResult)), &supportStats)

		adminTotal := int(adminStats["total_tickets"].(float64))
		supportTotal := int(supportStats["total_tickets"].(float64))

		// Support agent should see fewer tickets than admin
		// (Admin sees Support + Billing, Support agent sees only Support)
		assert.Less(t, supportTotal, adminTotal, 
			"Support agent should see fewer tickets than admin (got admin=%d, support=%d)", 
			adminTotal, supportTotal)
	})
}
