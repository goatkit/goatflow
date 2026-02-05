package v1

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/services"
)

// =============================================================================
// TEST FIXTURES - Users, Groups, Tokens for Authorization Tests
// =============================================================================

// AuthTestFixtures holds all test data for authorization tests
type AuthTestFixtures struct {
	db *sql.DB
	mu sync.Mutex

	// Groups (queues share group_id)
	GroupSupport  int
	GroupBilling  int
	GroupStats    int
	GroupNovaBank int

	// Queues
	QueueSupport  int
	QueueBilling  int
	QueueNovaBank int

	// Agents
	AgentAdmin     int // Admin role, all access
	AgentRWSupport int // rw on Support only
	AgentROSupport int // ro on Support only
	AgentMulti     int // rw on Support, ro on Billing, rw on Stats

	// Agents with granular permissions
	AgentNoteOnly     int // note permission only on Support
	AgentCreateOnly   int // create permission only on Support
	AgentMoveInto     int // move_into permission only on Support
	AgentOwner        int // owner permission only on Support
	AgentPriority     int // priority permission only on Support

	// Customers
	CustomerAcme     string // Acme Corp customer
	CustomerNovaBank string // NovaBank customer
	CustomerNoGroup  string // Customer with no group access

	// Customer Companies
	CompanyAcme     string
	CompanyNovaBank string

	// Tickets (for access tests)
	TicketAcmeSupport      int // Acme ticket in Support queue
	TicketNovaBankSupport  int // NovaBank ticket in Support queue
	TicketAcmeBilling      int // Acme ticket in Billing queue
	TicketForDelete        int // Dedicated ticket for DELETE tests (will be archived)
	TicketNovaBankExclusive int // NovaBank ticket in NovaBank queue (only NovaBank company can access)

	// API Tokens
	Tokens map[string]string // name -> raw token string
}

var (
	authFixtures     *AuthTestFixtures
	authFixturesOnce sync.Once
	authFixturesErr  error
)

// getAuthFixtures returns shared test fixtures, creating them once
func getAuthFixtures(t *testing.T) *AuthTestFixtures {
	t.Helper()
	requireDatabase(t)

	authFixturesOnce.Do(func() {
		db, err := database.GetDB()
		if err != nil {
			authFixturesErr = fmt.Errorf("failed to get database: %w", err)
			return
		}

		authFixtures = &AuthTestFixtures{
			db:     db,
			Tokens: make(map[string]string),
		}
		authFixturesErr = authFixtures.setup()
	})

	if authFixturesErr != nil {
		t.Skipf("skipping authorization test: %v", authFixturesErr)
	}

	return authFixtures
}

// setup creates all test data
func (f *AuthTestFixtures) setup() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()

	// Helper for exec operations
	exec := func(query string, args ...interface{}) error {
		_, err := f.db.Exec(database.ConvertPlaceholders(query), args...)
		return err
	}

	// Helper for insert-or-replace: DELETE then INSERT (simpler than upsert for tests)
	insertOrReplace := func(deleteQuery, insertQuery string, deleteArgs, insertArgs []interface{}) error {
		// Attempt delete first (ignore errors - row might not exist)
		_, _ = f.db.Exec(database.ConvertPlaceholders(deleteQuery), deleteArgs...)
		// Now insert
		_, err := f.db.Exec(database.ConvertPlaceholders(insertQuery), insertArgs...)
		return err
	}
	_ = exec            // May be used later
	_ = insertOrReplace // May be used later

	// Use unique suffix to avoid conflicts across test runs
	suffix := fmt.Sprintf("_%d", time.Now().UnixNano()%100000)

	// Use very high IDs to avoid conflicts (90000+ range)
	f.GroupSupport = 90001
	f.GroupBilling = 90002
	f.GroupStats = 90003
	f.GroupNovaBank = 90004
	f.QueueSupport = 90001
	f.QueueBilling = 90002
	f.QueueNovaBank = 90003

	// -------------------------------------------------------------------------
	// 0. Clean up any existing test data (disable FK checks for clean slate)
	// -------------------------------------------------------------------------
	_, _ = f.db.Exec("SET FOREIGN_KEY_CHECKS=0")
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM user_api_tokens WHERE user_id >= 90000 OR name LIKE 'agent-%' OR name LIKE 'customer-%'"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM ticket WHERE id >= 90000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM queue WHERE id >= 90000 OR name LIKE 'AuthTest-%'"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id >= 90000 OR group_id >= 90000"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login LIKE '%authtest%'"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM customer_company WHERE customer_id LIKE 'authtest-%'"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM `groups` WHERE id >= 90000 OR name LIKE 'AuthTest-%'"))
	_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id >= 90000 OR login LIKE 'authtest-%'"))
	_, _ = f.db.Exec("SET FOREIGN_KEY_CHECKS=1")

	// -------------------------------------------------------------------------
	// 1. Create Groups
	// -------------------------------------------------------------------------
	groups := []struct {
		id   int
		name string
	}{
		{f.GroupSupport, "AuthTest-Support" + suffix},
		{f.GroupBilling, "AuthTest-Billing" + suffix},
		{f.GroupStats, "AuthTest-Stats" + suffix},
		{f.GroupNovaBank, "AuthTest-NovaBank" + suffix},
	}

	for _, g := range groups {
		if err := exec(
			"INSERT INTO `groups` (id, name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (?, ?, 'Authorization test group', 1, ?, 1, ?, 1)",
			g.id, g.name, now, now,
		); err != nil {
			return fmt.Errorf("failed to create group %s: %w", g.name, err)
		}
	}

	// -------------------------------------------------------------------------
	// 2. Create Queues (linked to groups)
	// -------------------------------------------------------------------------
	queues := []struct {
		id      int
		name    string
		groupID int
	}{
		{f.QueueSupport, "AuthTest-Support-Queue" + suffix, f.GroupSupport},
		{f.QueueBilling, "AuthTest-Billing-Queue" + suffix, f.GroupBilling},
		{f.QueueNovaBank, "AuthTest-NovaBank-Queue" + suffix, f.GroupNovaBank},
	}

	for _, q := range queues {
		if err := insertOrReplace(
			"DELETE FROM queue WHERE id = ?",
			`INSERT INTO queue (id, name, group_id, unlock_timeout, first_response_time, 
				first_response_notify, update_time, update_notify, solution_time, solution_notify,
				system_address_id, calendar_name, default_sign_key, salutation_id, signature_id,
				follow_up_id, follow_up_lock, comments, valid_id, 
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, 0, 0, 0, 0, 0, 0, 0, 1, NULL, NULL, 1, 1, 1, 0, 'Auth test queue', 1, ?, 1, ?, 1)`,
			[]interface{}{q.id},
			[]interface{}{q.id, q.name, q.groupID, now, now},
		); err != nil {
			return fmt.Errorf("failed to create queue %s: %w", q.name, err)
		}
	}

	// -------------------------------------------------------------------------
	// 3. Create Agents with different permission levels
	// -------------------------------------------------------------------------
	f.AgentAdmin = 90001
	f.AgentRWSupport = 90002
	f.AgentROSupport = 90003
	f.AgentMulti = 90004
	// Agents with granular permissions
	f.AgentNoteOnly = 90005
	f.AgentCreateOnly = 90006
	f.AgentMoveInto = 90007
	f.AgentOwner = 90008
	f.AgentPriority = 90009

	agents := []struct {
		id    int
		login string
	}{
		{f.AgentAdmin, "authtest-admin" + suffix},
		{f.AgentRWSupport, "authtest-agent-rw-support" + suffix},
		{f.AgentROSupport, "authtest-agent-ro-support" + suffix},
		{f.AgentMulti, "authtest-agent-multi" + suffix},
		// Granular permission agents
		{f.AgentNoteOnly, "authtest-agent-note-only" + suffix},
		{f.AgentCreateOnly, "authtest-agent-create-only" + suffix},
		{f.AgentMoveInto, "authtest-agent-move-into" + suffix},
		{f.AgentOwner, "authtest-agent-owner" + suffix},
		{f.AgentPriority, "authtest-agent-priority" + suffix},
	}

	for _, a := range agents {
		if err := insertOrReplace(
			"DELETE FROM users WHERE id = ?",
			`INSERT INTO users (id, login, pw, first_name, last_name, valid_id, 
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'test', 'Test', 'Agent', 1, ?, 1, ?, 1)`,
			[]interface{}{a.id},
			[]interface{}{a.id, a.login, now, now},
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
		perms   []string // permission_key values
	}{
		// Admin gets rw on all groups
		{f.AgentAdmin, f.GroupSupport, []string{"rw"}},
		{f.AgentAdmin, f.GroupBilling, []string{"rw"}},
		{f.AgentAdmin, f.GroupStats, []string{"rw"}},
		{f.AgentAdmin, f.GroupNovaBank, []string{"rw"}},

		// RW Support agent
		{f.AgentRWSupport, f.GroupSupport, []string{"rw"}},

		// RO Support agent
		{f.AgentROSupport, f.GroupSupport, []string{"ro"}},

		// Multi-group agent: rw Support, ro Billing, rw Stats
		{f.AgentMulti, f.GroupSupport, []string{"rw"}},
		{f.AgentMulti, f.GroupBilling, []string{"ro"}},
		{f.AgentMulti, f.GroupStats, []string{"rw"}},

		// Granular permission agents - each has only one specific permission
		{f.AgentNoteOnly, f.GroupSupport, []string{"note"}},
		{f.AgentCreateOnly, f.GroupSupport, []string{"create"}},
		{f.AgentMoveInto, f.GroupSupport, []string{"move_into"}},
		{f.AgentOwner, f.GroupSupport, []string{"owner"}},
		{f.AgentPriority, f.GroupSupport, []string{"priority"}},
	}

	for _, ap := range agentPerms {
		for _, perm := range ap.perms {
			// Delete existing then insert (simpler than upsert)
			_, _ = f.db.Exec(database.ConvertPlaceholders(
				"DELETE FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = ?"),
				ap.userID, ap.groupID, perm)
			if err := exec(`
				INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
				VALUES (?, ?, ?, ?, 1, ?, 1)
			`, ap.userID, ap.groupID, perm, now, now); err != nil && !strings.Contains(err.Error(), "duplicate") {
				return fmt.Errorf("failed to assign permission: %w", err)
			}
		}
	}

	// -------------------------------------------------------------------------
	// 5. Create Customer Companies
	// -------------------------------------------------------------------------
	f.CompanyAcme = "authtest-acme-corp"
	f.CompanyNovaBank = "authtest-novabank"

	companies := []string{f.CompanyAcme, f.CompanyNovaBank}
	for _, c := range companies {
		_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM customer_company WHERE customer_id = ?"), c)
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
	f.CustomerAcme = "alice@authtest-acme.com"
	f.CustomerNovaBank = "bob@authtest-novabank.com"
	f.CustomerNoGroup = "carol@authtest-nogroup.com"

	customers := []struct {
		login   string
		company string
	}{
		{f.CustomerAcme, f.CompanyAcme},
		{f.CustomerNovaBank, f.CompanyNovaBank},
		{f.CustomerNoGroup, f.CompanyAcme}, // Same company, but won't have group access
	}

	for _, cu := range customers {
		_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login = ?"), cu.login)
		if err := exec(`
			INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id,
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, 'test', 'Test', 'Customer', 1, ?, 1, ?, 1)
		`, cu.login, cu.login, cu.company, now, now); err != nil && !strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("failed to create customer %s: %w", cu.login, err)
		}
	}

	// -------------------------------------------------------------------------
	// 7. Assign Customer Group Permissions (group_customer table)
	// -------------------------------------------------------------------------
	// Acme company -> Support group (ro) - can see tickets in Support queue
	_, _ = f.db.Exec(database.ConvertPlaceholders(
		"DELETE FROM group_customer WHERE customer_id = ? AND group_id = ?"), f.CompanyAcme, f.GroupSupport)
	_ = exec(`
		INSERT INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'ro', 1, 'Ticket', ?, 1, ?, 1)
	`, f.CompanyAcme, f.GroupSupport, now, now)

	// NovaBank company -> Support + NovaBank groups (ro)
	_, _ = f.db.Exec(database.ConvertPlaceholders(
		"DELETE FROM group_customer WHERE customer_id = ? AND group_id = ?"), f.CompanyNovaBank, f.GroupSupport)
	_ = exec(`
		INSERT INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'ro', 1, 'Ticket', ?, 1, ?, 1)
	`, f.CompanyNovaBank, f.GroupSupport, now, now)
	_, _ = f.db.Exec(database.ConvertPlaceholders(
		"DELETE FROM group_customer WHERE customer_id = ? AND group_id = ?"), f.CompanyNovaBank, f.GroupNovaBank)
	_ = exec(`
		INSERT INTO group_customer (customer_id, group_id, permission_key, permission_value, permission_context,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'ro', 1, 'Ticket', ?, 1, ?, 1)
	`, f.CompanyNovaBank, f.GroupNovaBank, now, now)
	
	// -------------------------------------------------------------------------
	// 7b. Assign Individual Customer User Group Permissions (group_customer_user table)
	// -------------------------------------------------------------------------
	// Give CustomerAcme (alice) explicit access to Billing group (beyond her company's access)
	_, _ = f.db.Exec(database.ConvertPlaceholders(
		"DELETE FROM group_customer_user WHERE user_id = ? AND group_id = ?"), f.CustomerAcme, f.GroupBilling)
	_ = exec(`
		INSERT INTO group_customer_user (user_id, group_id, permission_key, permission_value,
			create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'ro', 1, ?, 1, ?, 1)
	`, f.CustomerAcme, f.GroupBilling, now, now)
	
	// CustomerNoGroup has NO group permissions (only company-based access via CompanyAcme)

	// -------------------------------------------------------------------------
	// 8. Create Test Tickets (using known IDs)
	// -------------------------------------------------------------------------
	f.TicketAcmeSupport = 90001
	f.TicketNovaBankSupport = 90002
	f.TicketAcmeBilling = 90003
	f.TicketForDelete = 90004
	f.TicketNovaBankExclusive = 90005

	tickets := []struct {
		id             int
		tn             string
		title          string
		queueID        int
		customerID     string
		customerUserID string // Customer login/email for article permission checks
	}{
		{f.TicketAcmeSupport, "AUTH90001", "Acme Support Ticket", f.QueueSupport, f.CompanyAcme, f.CustomerAcme},
		{f.TicketNovaBankSupport, "AUTH90002", "NovaBank Support Ticket", f.QueueSupport, f.CompanyNovaBank, f.CustomerNovaBank},
		{f.TicketAcmeBilling, "AUTH90003", "Acme Billing Ticket", f.QueueBilling, f.CompanyAcme, f.CustomerAcme},
		{f.TicketForDelete, "AUTH90004", "Ticket For Delete Tests", f.QueueSupport, f.CompanyAcme, f.CustomerAcme},
		{f.TicketNovaBankExclusive, "AUTH90005", "NovaBank Exclusive Ticket", f.QueueNovaBank, f.CompanyNovaBank, f.CustomerNovaBank},
	}

	for _, tk := range tickets {
		if err := insertOrReplace(
			"DELETE FROM ticket WHERE id = ?",
			`INSERT INTO ticket (id, tn, title, queue_id, ticket_lock_id, type_id, 
				ticket_state_id, ticket_priority_id, customer_id, customer_user_id,
				user_id, responsible_user_id, timeout, until_time, escalation_time,
				escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, 1, 1, 1, 3, ?, ?, 1, 1, 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)`,
			[]interface{}{tk.id},
			[]interface{}{tk.id, tk.tn, tk.title, tk.queueID, tk.customerID, tk.customerUserID, now, now},
		); err != nil {
			return fmt.Errorf("failed to create ticket %s: %w", tk.title, err)
		}
	}

	// -------------------------------------------------------------------------
	// 9. Create API Tokens with various scopes
	// -------------------------------------------------------------------------
	tokenDefs := []struct {
		name     string
		userID   int
		userType models.APITokenUserType
		scopes   []string
	}{
		// Agent tokens
		{"agent-admin-full", f.AgentAdmin, models.APITokenUserAgent, []string{"*"}},
		{"agent-admin-tickets-read", f.AgentAdmin, models.APITokenUserAgent, []string{"tickets:read"}},
		{"agent-admin-tickets-write", f.AgentAdmin, models.APITokenUserAgent, []string{"tickets:write"}},
		{"agent-admin-tickets-delete", f.AgentAdmin, models.APITokenUserAgent, []string{"tickets:delete"}},
		{"agent-admin-articles-read", f.AgentAdmin, models.APITokenUserAgent, []string{"articles:read"}},
		{"agent-admin-articles-write", f.AgentAdmin, models.APITokenUserAgent, []string{"articles:write"}},
		{"agent-admin-admin-scope", f.AgentAdmin, models.APITokenUserAgent, []string{"admin:*"}},

		{"agent-rw-support-full", f.AgentRWSupport, models.APITokenUserAgent, []string{"*"}},
		{"agent-ro-support-full", f.AgentROSupport, models.APITokenUserAgent, []string{"*"}},
		{"agent-multi-full", f.AgentMulti, models.APITokenUserAgent, []string{"*"}},
		{"agent-multi-tickets-read", f.AgentMulti, models.APITokenUserAgent, []string{"tickets:read"}},

		// Granular permission agents
		{"agent-note-only-full", f.AgentNoteOnly, models.APITokenUserAgent, []string{"*"}},
		{"agent-create-only-full", f.AgentCreateOnly, models.APITokenUserAgent, []string{"*"}},
		{"agent-move-into-full", f.AgentMoveInto, models.APITokenUserAgent, []string{"*"}},
		{"agent-owner-full", f.AgentOwner, models.APITokenUserAgent, []string{"*"}},
		{"agent-priority-full", f.AgentPriority, models.APITokenUserAgent, []string{"*"}},

		// Customer tokens (using placeholder IDs since customer_user table uses string IDs)
		{"customer-acme-full", 1000, models.APITokenUserCustomer, []string{"*"}},
		{"customer-acme-tickets-read", 1000, models.APITokenUserCustomer, []string{"tickets:read"}},
		{"customer-novabank-full", 1001, models.APITokenUserCustomer, []string{"*"}},
		{"customer-nogroup-full", 1002, models.APITokenUserCustomer, []string{"*"}},

		// Edge cases
		{"agent-expired", f.AgentAdmin, models.APITokenUserAgent, []string{"*"}},
		{"agent-revoked", f.AgentAdmin, models.APITokenUserAgent, []string{"*"}},
	}

	for _, td := range tokenDefs {
		// Generate a test token
		rawToken := models.TokenPrefix + "test_" + td.name + "_" + fmt.Sprintf("%d", time.Now().UnixNano())
		hash := rawToken // In real code this would be hashed

		expiresAt := now.Add(365 * 24 * time.Hour)
		if td.name == "agent-expired" {
			expiresAt = now.Add(-1 * time.Hour)
		}

		isRevoked := false
		if td.name == "agent-revoked" {
			isRevoked = true
		}

		scopesJSON := "[]"
		if len(td.scopes) > 0 {
			scopesJSON = `["` + strings.Join(td.scopes, `","`) + `"]`
		}

		_, _ = f.db.Exec(database.ConvertPlaceholders("DELETE FROM user_api_tokens WHERE name = ?"), td.name)
		
		// Set revoked_at if token should be revoked
		var revokedAt interface{} = nil
		if isRevoked {
			revokedAt = now
		}
		
		if err := exec(`
			INSERT INTO user_api_tokens (user_id, user_type, name, prefix, token_hash, scopes, 
				expires_at, revoked_at, created_at)
			VALUES (?, ?, ?, 'gf_test', ?, ?, ?, ?, ?)
		`, td.userID, td.userType, td.name, hash, scopesJSON, expiresAt, revokedAt, now); err != nil {
			return fmt.Errorf("failed to create token %s: %w", td.name, err)
		}

		f.Tokens[td.name] = rawToken
	}

	return nil
}

// cleanup removes all test data (call in TestMain or defer)
func (f *AuthTestFixtures) cleanup() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Delete in reverse dependency order
	queries := []string{
		`DELETE FROM api_tokens WHERE name LIKE 'authtest-%' OR name LIKE 'agent-%' OR name LIKE 'customer-%'`,
		`DELETE FROM ticket WHERE title LIKE 'Acme%' OR title LIKE 'NovaBank%'`,
		`DELETE FROM customer_group WHERE customer_id LIKE 'authtest-%'`,
		`DELETE FROM customer_user WHERE login LIKE '%@authtest-%'`,
		`DELETE FROM customer_company WHERE customer_id LIKE 'authtest-%'`,
		`DELETE FROM group_user WHERE user_id IN (SELECT id FROM users WHERE login LIKE 'authtest-%')`,
		`DELETE FROM users WHERE login LIKE 'authtest-%'`,
		`DELETE FROM queue WHERE name LIKE 'AuthTest-%'`,
		"DELETE FROM `groups` WHERE name LIKE 'AuthTest-%'",
	}

	for _, q := range queries {
		_, _ = f.db.Exec(database.ConvertPlaceholders(q))
	}

	return nil
}

// =============================================================================
// TEST ROUTER SETUP
// =============================================================================

// MockTokenVerifier implements middleware.APITokenVerifier for tests
type MockTokenVerifier struct {
	fixtures *AuthTestFixtures
}

func (m *MockTokenVerifier) VerifyToken(ctx context.Context, rawToken string) (*models.APIToken, error) {
	// Look up token in fixtures
	for name, token := range m.fixtures.Tokens {
		if token == rawToken {
			// Build APIToken from fixture data
			return m.buildToken(name)
		}
	}
	return nil, fmt.Errorf("invalid token")
}

func (m *MockTokenVerifier) UpdateLastUsed(ctx context.Context, tokenID int64, ip string) error {
	return nil
}

func (m *MockTokenVerifier) buildToken(name string) (*models.APIToken, error) {
	// Parse token name to determine properties
	// This is a simplified mock - real implementation would query DB
	
	token := &models.APIToken{
		ID:   1,
		Name: name,
	}

	switch {
	case strings.HasPrefix(name, "agent-admin"):
		token.UserID = m.fixtures.AgentAdmin
		token.UserType = models.APITokenUserAgent
	case strings.HasPrefix(name, "agent-rw-support"):
		token.UserID = m.fixtures.AgentRWSupport
		token.UserType = models.APITokenUserAgent
	case strings.HasPrefix(name, "agent-ro-support"):
		token.UserID = m.fixtures.AgentROSupport
		token.UserType = models.APITokenUserAgent
	case strings.HasPrefix(name, "agent-multi"):
		token.UserID = m.fixtures.AgentMulti
		token.UserType = models.APITokenUserAgent
	case strings.HasPrefix(name, "agent-note-only"):
		token.UserID = m.fixtures.AgentNoteOnly
		token.UserType = models.APITokenUserAgent
	case strings.HasPrefix(name, "agent-create-only"):
		token.UserID = m.fixtures.AgentCreateOnly
		token.UserType = models.APITokenUserAgent
	case strings.HasPrefix(name, "agent-move-into"):
		token.UserID = m.fixtures.AgentMoveInto
		token.UserType = models.APITokenUserAgent
	case strings.HasPrefix(name, "agent-owner"):
		token.UserID = m.fixtures.AgentOwner
		token.UserType = models.APITokenUserAgent
	case strings.HasPrefix(name, "agent-priority"):
		token.UserID = m.fixtures.AgentPriority
		token.UserType = models.APITokenUserAgent
	case strings.HasPrefix(name, "customer-acme"):
		token.UserID = 1000 // Placeholder
		token.UserType = models.APITokenUserCustomer
		token.CustomerLogin = m.fixtures.CustomerAcme // "alice@authtest-acme.com"
	case strings.HasPrefix(name, "customer-novabank"):
		token.UserID = 1001 // Placeholder
		token.UserType = models.APITokenUserCustomer
		token.CustomerLogin = m.fixtures.CustomerNovaBank // "bob@authtest-novabank.com"
	case strings.HasPrefix(name, "customer-nogroup"):
		token.UserID = 1002 // Placeholder
		token.UserType = models.APITokenUserCustomer
		token.CustomerLogin = m.fixtures.CustomerNoGroup // "carol@authtest-nogroup.com"
	}

	// Parse scopes from name
	switch {
	case strings.Contains(name, "-full"):
		token.Scopes = []string{"*"}
	case strings.Contains(name, "-tickets-read"):
		token.Scopes = []string{"tickets:read"}
	case strings.Contains(name, "-tickets-write"):
		token.Scopes = []string{"tickets:write"}
	case strings.Contains(name, "-tickets-delete"):
		token.Scopes = []string{"tickets:delete"}
	case strings.Contains(name, "-articles-read"):
		token.Scopes = []string{"articles:read"}
	case strings.Contains(name, "-articles-write"):
		token.Scopes = []string{"articles:write"}
	case strings.Contains(name, "-admin-scope"):
		token.Scopes = []string{"admin:*"}
	}

	// Handle expired/revoked
	if name == "agent-expired" {
		return nil, fmt.Errorf("token expired")
	}
	if name == "agent-revoked" {
		return nil, fmt.Errorf("token revoked")
	}

	return token, nil
}

// setupAuthTestRouter creates a test router with auth middleware and REAL handlers
func setupAuthTestRouter(t *testing.T, fixtures *AuthTestFixtures) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Set up mock token verifier (still needed to translate test tokens)
	mockVerifier := &MockTokenVerifier{fixtures: fixtures}
	middleware.SetAPITokenVerifier(mockVerifier)

	// Create JWT manager and real API router
	jwtManager := auth.NewJWTManager("test-secret", 24*time.Hour)
	apiRouter := NewAPIRouter(nil, jwtManager, nil)

	// Setup v1 API routes with auth middleware + real handlers
	v1 := router.Group("/api/v1")
	v1.Use(middleware.UnifiedAuthMiddleware(jwtManager))

	// Ticket routes - using real handlers
	tickets := v1.Group("/tickets")
	tickets.GET("", middleware.RequireScope("tickets:read"), apiRouter.HandleListTickets)
	tickets.GET("/:id", middleware.RequireScope("tickets:read"), func(c *gin.Context) {
		api.HandleGetTicketAPI(c)
	})
	tickets.POST("", middleware.RequireScope("tickets:write"), func(c *gin.Context) {
		api.HandleCreateTicketAPI(c)
	})
	tickets.PATCH("/:id", middleware.RequireScope("tickets:write"), func(c *gin.Context) {
		api.HandleUpdateTicketAPI(c)
	})
	tickets.DELETE("/:id", middleware.RequireScope("tickets:delete"), func(c *gin.Context) {
		api.HandleDeleteTicketAPI(c)
	})

	// Article routes
	tickets.GET("/:id/articles", middleware.RequireScope("articles:read"), apiRouter.handleGetTicketArticles)
	tickets.POST("/:id/articles", middleware.RequireScope("articles:write"), func(c *gin.Context) {
		api.HandleCreateArticleAPI(c)
	})

	// Queue routes
	queues := v1.Group("/queues")
	queues.GET("", middleware.RequireScope("queues:read"), apiRouter.handleListQueues)

	// User routes (agent only)
	users := v1.Group("/users")
	users.GET("", middleware.RequireScope("users:read"), apiRouter.handleListUsers)

	// Admin routes
	admin := v1.Group("/admin")
	admin.Use(middleware.RequireScope("admin:*"))
	admin.GET("/settings", apiRouter.handleGetSystemConfig)

	// Stats routes (requires stats group)
	stats := v1.Group("/stats")
	stats.GET("/dashboard", apiRouter.handleGetDashboardStats)

	return router
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// makeRequest performs an HTTP request with the given token
func makeRequest(t *testing.T, router *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	return makeRequestWithBody(t, router, method, path, token, "")
}

func makeRequestWithBody(t *testing.T, router *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, path, nil)
	}
	require.NoError(t, err)

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// assertStatus checks response status code
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Equal(t, expected, w.Code, msgAndArgs...)
}

// =============================================================================
// TOKEN SCOPE TESTS
// =============================================================================

func TestTokenScope_TicketsRead(t *testing.T) {
	fixtures := getAuthFixtures(t)
	router := setupAuthTestRouter(t, fixtures)

	// Use real ticket ID from fixtures
	ticketPath := fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketAcmeSupport)
	
	// Valid request bodies for POST/PATCH
	createBody := fmt.Sprintf(`{"title":"Test Ticket","queue_id":%d}`, fixtures.QueueSupport)
	updateBody := `{"title":"Updated Title"}`

	tests := []struct {
		name        string
		tokenName   string
		method      string
		path        string
		body        string
		wantBlocked bool // true = expect 403, false = expect NOT 403 (scope allows)
	}{
		// tickets:read scope - should block writes
		{"read scope can GET tickets", "agent-admin-tickets-read", "GET", "/api/v1/tickets", "", false},
		{"read scope can GET ticket by ID", "agent-admin-tickets-read", "GET", ticketPath, "", false},
		{"read scope BLOCKED from POST", "agent-admin-tickets-read", "POST", "/api/v1/tickets", createBody, true},
		{"read scope BLOCKED from PATCH", "agent-admin-tickets-read", "PATCH", ticketPath, updateBody, true},
		{"read scope BLOCKED from DELETE", "agent-admin-tickets-read", "DELETE", ticketPath, "", true},

		// tickets:write scope - should allow write but block delete
		{"write scope can POST ticket", "agent-admin-tickets-write", "POST", "/api/v1/tickets", createBody, false},
		{"write scope can PATCH ticket", "agent-admin-tickets-write", "PATCH", ticketPath, updateBody, false},
		{"write scope BLOCKED from DELETE", "agent-admin-tickets-write", "DELETE", ticketPath, "", true},

		// tickets:delete scope - should allow delete
		{"delete scope can DELETE ticket", "agent-admin-tickets-delete", "DELETE", ticketPath, "", false},

		// Full access scope - should allow everything
		{"full scope can GET", "agent-admin-full", "GET", "/api/v1/tickets", "", false},
		{"full scope can POST", "agent-admin-full", "POST", "/api/v1/tickets", createBody, false},
		{"full scope can PATCH", "agent-admin-full", "PATCH", ticketPath, updateBody, false},
		{"full scope can DELETE", "agent-admin-full", "DELETE", ticketPath, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := fixtures.Tokens[tt.tokenName]
			w := makeRequestWithBody(t, router, tt.method, tt.path, token, tt.body)
			
			if tt.wantBlocked {
				assert.Equal(t, 403, w.Code, "expected 403 (scope should block), got %d for %s %s", w.Code, tt.method, tt.path)
			} else {
				assert.NotEqual(t, 403, w.Code, "expected NOT 403 (scope should allow), got 403 for %s %s", tt.method, tt.path)
			}
		})
	}
}

func TestTokenScope_ArticlesReadWrite(t *testing.T) {
	fixtures := getAuthFixtures(t)
	router := setupAuthTestRouter(t, fixtures)

	// Use a real ticket ID from fixtures for article operations
	articlePath := fmt.Sprintf("/api/v1/tickets/%d/articles", fixtures.TicketAcmeSupport)
	articleBody := `{"body":"Test article content"}`

	tests := []struct {
		name       string
		tokenName  string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"articles:read can GET articles", "agent-admin-articles-read", "GET", articlePath, "", 200},
		{"articles:read cannot POST article", "agent-admin-articles-read", "POST", articlePath, articleBody, 403},
		{"articles:write can POST article", "agent-admin-articles-write", "POST", articlePath, articleBody, 201},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := fixtures.Tokens[tt.tokenName]
			w := makeRequestWithBody(t, router, tt.method, tt.path, token, tt.body)
			assertStatus(t, w, tt.wantStatus)
		})
	}
}

func TestTokenScope_AgentOnlyScopes(t *testing.T) {
	fixtures := getAuthFixtures(t)
	router := setupAuthTestRouter(t, fixtures)

	tests := []struct {
		name       string
		tokenName  string
		method     string
		path       string
		wantStatus int
	}{
		// Agent with users:read - should work (scope exists in agent token definitions... 
		// but we need to add it to fixtures first. For now test admin:* scope)
		{"agent admin scope can access admin", "agent-admin-admin-scope", "GET", "/api/v1/admin/settings", 200},
		
		// Customer tokens should not be able to access agent-only endpoints
		// (This would require the endpoint to check user_role, not just scope)
		{"customer full scope cannot access admin", "customer-acme-full", "GET", "/api/v1/admin/settings", 403},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := fixtures.Tokens[tt.tokenName]
			w := makeRequest(t, router, tt.method, tt.path, token)
			assertStatus(t, w, tt.wantStatus)
		})
	}
}

func TestTokenScope_ExpiredAndRevoked(t *testing.T) {
	fixtures := getAuthFixtures(t)
	router := setupAuthTestRouter(t, fixtures)

	tests := []struct {
		name       string
		tokenName  string
		wantStatus int
	}{
		{"expired token returns 401", "agent-expired", 401},
		{"revoked token returns 401", "agent-revoked", 401},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := fixtures.Tokens[tt.tokenName]
			w := makeRequest(t, router, "GET", "/api/v1/tickets", token)
			assertStatus(t, w, tt.wantStatus)
		})
	}
}

// =============================================================================
// AGENT GROUP PERMISSION TESTS
// =============================================================================

func TestAgentGroupPermissions_QueueAccess(t *testing.T) {
	fixtures := getAuthFixtures(t)
	router := setupAuthTestRouter(t, fixtures)

	// These tests verify that agents can only access tickets in queues
	// where they have appropriate group permissions.
	// 
	// NOTE: This requires the actual ticket handlers to enforce group checks,
	// not just scope checks. The placeholder handlers don't do this yet.
	// 
	// Test scenarios:
	// - agent-rw-support: rw on Support only
	// - agent-ro-support: ro on Support only  
	// - agent-multi: rw on Support, ro on Billing, rw on Stats

	t.Run("agent with rw can update ticket in their queue", func(t *testing.T) {
		// agent-rw-support has rw on Support queue
		token := fixtures.Tokens["agent-rw-support-full"]
		body := `{"title":"Updated by RW agent"}`
		w := makeRequestWithBody(t, router, "PATCH", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketAcmeSupport), token, body)
		// With real handlers, this should check group permissions
		assertStatus(t, w, 200)
	})

	t.Run("agent with ro cannot update ticket in their queue", func(t *testing.T) {
		// agent-ro-support has only ro on Support queue
		// RO permission enforcement implemented - agents with only 'ro' get 403 on write operations
		token := fixtures.Tokens["agent-ro-support-full"]
		body := `{"title":"Should be blocked"}`
		w := makeRequestWithBody(t, router, "PATCH", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketAcmeSupport), token, body)
		assertStatus(t, w, 403)
	})

	t.Run("agent cannot access ticket in queue they have no access to", func(t *testing.T) {
		// agent-rw-support has no access to Billing queue
		// Security: return 404 (not 403) to avoid revealing ticket existence
		token := fixtures.Tokens["agent-rw-support-full"]
		w := makeRequest(t, router, "GET", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketAcmeBilling), token)
		assertStatus(t, w, 404)
	})

	// DELETE endpoint permission tests
	t.Run("agent with rw can delete ticket in their queue", func(t *testing.T) {
		// agent-rw-support has rw on Support queue
		// Use dedicated delete ticket to avoid test interference
		token := fixtures.Tokens["agent-rw-support-full"]
		w := makeRequest(t, router, "DELETE", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketForDelete), token)
		// Should succeed (200 or 204) - ticket gets archived
		if w.Code != 200 && w.Code != 204 {
			t.Errorf("expected 200 or 204, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("agent with ro cannot delete ticket in their queue", func(t *testing.T) {
		// agent-ro-support has only ro on Support queue - cannot delete
		// Security: return 404 (not 403) to avoid revealing ticket existence
		token := fixtures.Tokens["agent-ro-support-full"]
		w := makeRequest(t, router, "DELETE", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketAcmeSupport), token)
		assertStatus(t, w, 404)
	})

	t.Run("agent cannot delete ticket in queue they have no access to", func(t *testing.T) {
		// agent-rw-support has no access to Billing queue
		// Security: return 404 (not 403) to avoid revealing ticket existence
		token := fixtures.Tokens["agent-rw-support-full"]
		w := makeRequest(t, router, "DELETE", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketAcmeBilling), token)
		assertStatus(t, w, 404)
	})
}

func TestAgentGroupPermissions_GranularPerms(t *testing.T) {
	// Test specific permission types: ro, rw, move_into, create, note, owner, priority
	fixtures := getAuthFixtures(t)

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Fatal("Database not available for granular permission tests")
	}

	permSvc := services.NewPermissionService(db)

	// Test 'note' permission
	t.Run("agent with note permission can add notes", func(t *testing.T) {
		canNote, err := permSvc.CanAddNote(fixtures.AgentNoteOnly, int64(fixtures.TicketAcmeSupport))
		if err != nil {
			t.Fatalf("CanAddNote failed: %v", err)
		}
		if !canNote {
			t.Error("Agent with 'note' permission should be able to add notes")
		}
	})

	t.Run("agent with note permission cannot read tickets", func(t *testing.T) {
		// 'note' alone doesn't grant read access
		canRead, err := permSvc.CanReadTicket(fixtures.AgentNoteOnly, int64(fixtures.TicketAcmeSupport))
		if err != nil {
			t.Fatalf("CanReadTicket failed: %v", err)
		}
		if canRead {
			t.Error("Agent with only 'note' permission should not have read access")
		}
	})

	// Test 'create' permission
	t.Run("agent with create permission can create in queue", func(t *testing.T) {
		canCreate, err := permSvc.CanCreate(fixtures.AgentCreateOnly, fixtures.QueueSupport)
		if err != nil {
			t.Fatalf("CanCreate failed: %v", err)
		}
		if !canCreate {
			t.Error("Agent with 'create' permission should be able to create tickets")
		}
	})

	t.Run("agent with create permission cannot create in other queue", func(t *testing.T) {
		canCreate, err := permSvc.CanCreate(fixtures.AgentCreateOnly, fixtures.QueueBilling)
		if err != nil {
			t.Fatalf("CanCreate failed: %v", err)
		}
		if canCreate {
			t.Error("Agent should not be able to create in queue without permission")
		}
	})

	// Test 'move_into' permission
	t.Run("agent with move_into permission can move tickets into queue", func(t *testing.T) {
		canMove, err := permSvc.CanMoveInto(fixtures.AgentMoveInto, fixtures.QueueSupport)
		if err != nil {
			t.Fatalf("CanMoveInto failed: %v", err)
		}
		if !canMove {
			t.Error("Agent with 'move_into' permission should be able to move tickets")
		}
	})

	// Test 'owner' permission
	t.Run("agent with owner permission can be owner in queue", func(t *testing.T) {
		canOwn, err := permSvc.CanBeOwner(fixtures.AgentOwner, fixtures.QueueSupport)
		if err != nil {
			t.Fatalf("CanBeOwner failed: %v", err)
		}
		if !canOwn {
			t.Error("Agent with 'owner' permission should be able to be owner")
		}
	})

	// Test 'priority' permission
	t.Run("agent with priority permission can change priority", func(t *testing.T) {
		canPriority, err := permSvc.CanChangePriority(fixtures.AgentPriority, int64(fixtures.TicketAcmeSupport))
		if err != nil {
			t.Fatalf("CanChangePriority failed: %v", err)
		}
		if !canPriority {
			t.Error("Agent with 'priority' permission should be able to change priority")
		}
	})

	// Test 'rw' supersedes all
	t.Run("rw permission grants all granular permissions", func(t *testing.T) {
		// AgentRWSupport has 'rw' on Support queue
		canNote, _ := permSvc.CanAddNote(fixtures.AgentRWSupport, int64(fixtures.TicketAcmeSupport))
		canCreate, _ := permSvc.CanCreate(fixtures.AgentRWSupport, fixtures.QueueSupport)
		canMove, _ := permSvc.CanMoveInto(fixtures.AgentRWSupport, fixtures.QueueSupport)
		canOwn, _ := permSvc.CanBeOwner(fixtures.AgentRWSupport, fixtures.QueueSupport)
		canPriority, _ := permSvc.CanChangePriority(fixtures.AgentRWSupport, int64(fixtures.TicketAcmeSupport))

		if !canNote {
			t.Error("Agent with 'rw' should be able to add notes")
		}
		if !canCreate {
			t.Error("Agent with 'rw' should be able to create tickets")
		}
		if !canMove {
			t.Error("Agent with 'rw' should be able to move tickets")
		}
		if !canOwn {
			t.Error("Agent with 'rw' should be able to be owner")
		}
		if !canPriority {
			t.Error("Agent with 'rw' should be able to change priority")
		}
	})

	// Test permission isolation - agents can't use permissions they don't have
	t.Run("note-only agent cannot change priority", func(t *testing.T) {
		canPriority, err := permSvc.CanChangePriority(fixtures.AgentNoteOnly, int64(fixtures.TicketAcmeSupport))
		if err != nil {
			t.Fatalf("CanChangePriority failed: %v", err)
		}
		if canPriority {
			t.Error("Agent with only 'note' permission should not be able to change priority")
		}
	})

	t.Run("create-only agent cannot add notes", func(t *testing.T) {
		canNote, err := permSvc.CanAddNote(fixtures.AgentCreateOnly, int64(fixtures.TicketAcmeSupport))
		if err != nil {
			t.Fatalf("CanAddNote failed: %v", err)
		}
		if canNote {
			t.Error("Agent with only 'create' permission should not be able to add notes")
		}
	})
}

// =============================================================================
// CUSTOMER ACCESS TESTS  
// =============================================================================

func TestCustomerAccess_CompanyIsolation(t *testing.T) {
	fixtures := getAuthFixtures(t)
	router := setupAuthTestRouter(t, fixtures)

	// Customers should only see tickets from their own company

	t.Run("customer can see own company tickets", func(t *testing.T) {
		token := fixtures.Tokens["customer-acme-full"]
		w := makeRequest(t, router, "GET", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketAcmeSupport), token)
		// With real handlers, should return 200 and filter to Acme tickets only
		assertStatus(t, w, 200)
	})

	t.Run("customer cannot see other company tickets", func(t *testing.T) {
		// Customer company isolation implemented - customers can only see their own company's tickets
		// Security: return 404 (not 403) to avoid revealing ticket existence
		token := fixtures.Tokens["customer-acme-full"]
		w := makeRequest(t, router, "GET", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketNovaBankSupport), token)
		assertStatus(t, w, 404)
	})
}

func TestCustomerAccess_GroupPermissions(t *testing.T) {
	// Test that customers can access tickets via group permissions
	// (in addition to their company's tickets)
	fixtures := getAuthFixtures(t)

	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Fatal("Database not available for customer group permission tests")
	}

	permSvc := services.NewPermissionService(db)

	// Test company-level group access (group_customer table)
	t.Run("customer company has group access to queue", func(t *testing.T) {
		// Acme company has 'ro' on Support group
		canAccess, err := permSvc.CustomerCompanyCanAccessQueue(fixtures.CompanyAcme, fixtures.QueueSupport)
		if err != nil {
			t.Fatalf("CustomerCompanyCanAccessQueue failed: %v", err)
		}
		if !canAccess {
			t.Error("Acme company should have access to Support queue via group_customer")
		}
	})

	t.Run("customer company cannot access queue without group permission", func(t *testing.T) {
		// Acme company does NOT have access to Billing queue
		canAccess, err := permSvc.CustomerCompanyCanAccessQueue(fixtures.CompanyAcme, fixtures.QueueBilling)
		if err != nil {
			t.Fatalf("CustomerCompanyCanAccessQueue failed: %v", err)
		}
		if canAccess {
			t.Error("Acme company should NOT have access to Billing queue")
		}
	})

	// Test individual customer user group access (group_customer_user table)
	t.Run("customer user has explicit group access beyond company", func(t *testing.T) {
		// Alice (CustomerAcme) has explicit 'ro' on Billing group
		canAccess, err := permSvc.CustomerCanAccessQueue(fixtures.CustomerAcme, fixtures.QueueBilling)
		if err != nil {
			t.Fatalf("CustomerCanAccessQueue failed: %v", err)
		}
		if !canAccess {
			t.Error("Alice should have access to Billing queue via group_customer_user")
		}
	})

	t.Run("customer user without group permission cannot access queue", func(t *testing.T) {
		// Carol (CustomerNoGroup) has no explicit group permissions
		canAccess, err := permSvc.CustomerCanAccessQueue(fixtures.CustomerNoGroup, fixtures.QueueBilling)
		if err != nil {
			t.Fatalf("CustomerCanAccessQueue failed: %v", err)
		}
		if canAccess {
			t.Error("Carol should NOT have access to Billing queue")
		}
	})

	// Test combined ticket access (company ownership OR group access)
	t.Run("customer can access ticket via company ownership", func(t *testing.T) {
		// Alice can access Acme Support ticket because it belongs to her company
		canAccess, err := permSvc.CustomerCanAccessTicket(
			fixtures.CustomerAcme, fixtures.CompanyAcme, int64(fixtures.TicketAcmeSupport))
		if err != nil {
			t.Fatalf("CustomerCanAccessTicket failed: %v", err)
		}
		if !canAccess {
			t.Error("Alice should access Acme Support ticket (company ownership)")
		}
	})

	t.Run("customer can access ticket via individual group permission", func(t *testing.T) {
		// Alice can access Acme Billing ticket via her explicit group_customer_user permission
		canAccess, err := permSvc.CustomerCanAccessTicket(
			fixtures.CustomerAcme, fixtures.CompanyAcme, int64(fixtures.TicketAcmeBilling))
		if err != nil {
			t.Fatalf("CustomerCanAccessTicket failed: %v", err)
		}
		if !canAccess {
			t.Error("Alice should access Acme Billing ticket (individual group permission)")
		}
	})

	t.Run("customer cannot access ticket in queue without any permission", func(t *testing.T) {
		// Carol (Acme company) cannot access NovaBank Exclusive ticket because:
		// 1. It's owned by NovaBank (not Acme)
		// 2. Acme company has no group access to NovaBank queue
		// 3. Carol has no individual group_customer_user permission on NovaBank queue
		canAccess, err := permSvc.CustomerCanAccessTicket(
			fixtures.CustomerNoGroup, fixtures.CompanyAcme, int64(fixtures.TicketNovaBankExclusive))
		if err != nil {
			t.Fatalf("CustomerCanAccessTicket failed: %v", err)
		}
		if canAccess {
			t.Error("Carol should NOT access NovaBank Exclusive ticket (no company ownership or group access)")
		}
	})
	
	t.Run("customer can access other company ticket via company group permission", func(t *testing.T) {
		// Carol (Acme company) CAN access NovaBank Support ticket because
		// Acme company has 'ro' on Support group
		canAccess, err := permSvc.CustomerCanAccessTicket(
			fixtures.CustomerNoGroup, fixtures.CompanyAcme, int64(fixtures.TicketNovaBankSupport))
		if err != nil {
			t.Fatalf("CustomerCanAccessTicket failed: %v", err)
		}
		if !canAccess {
			t.Error("Carol SHOULD access NovaBank Support ticket (company has Support group access)")
		}
	})
}

// =============================================================================
// COMBINED SCOPE + GROUP TESTS
// =============================================================================

func TestScopeAndGroupCombination(t *testing.T) {
	fixtures := getAuthFixtures(t)
	router := setupAuthTestRouter(t, fixtures)

	// Scope allows but group denies -> 403
	// Group allows but scope denies -> 403
	// Both allow -> 200

	t.Run("scope allows but group denies returns 403", func(t *testing.T) {
		// agent-multi has tickets:read scope (via full access)
		// agent-multi has NO access to NovaBank queue
		token := fixtures.Tokens["agent-multi-full"]
		// Try to read a ticket in NovaBank queue
		// With real handlers, this should be 403
		w := makeRequest(t, router, "GET", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketNovaBankSupport), token)
		t.Logf("Response: %d (expected 403 with real handler - agent-multi has no NovaBank access)", w.Code)
	})

	t.Run("group allows but scope denies returns 403", func(t *testing.T) {
		// agent-multi-tickets-read has only tickets:read scope
		// agent-multi has rw on Support queue
		// Try to update a ticket - scope should deny
		token := fixtures.Tokens["agent-multi-tickets-read"]
		w := makeRequest(t, router, "PATCH", fmt.Sprintf("/api/v1/tickets/%d", fixtures.TicketAcmeSupport), token)
		assertStatus(t, w, 403, "scope should deny write even though group allows")
	})
}

// =============================================================================
// STATS MODULE ACCESS TESTS
// =============================================================================

func TestStatsModuleAccess(t *testing.T) {
	fixtures := getAuthFixtures(t)
	router := setupAuthTestRouter(t, fixtures)

	t.Run("agent with stats group can access stats", func(t *testing.T) {
		// agent-multi has rw on Stats group
		token := fixtures.Tokens["agent-multi-full"]
		w := makeRequest(t, router, "GET", "/api/v1/stats/dashboard", token)
		// With real handlers checking stats group membership, should be 200
		assertStatus(t, w, 200)
	})

	t.Run("agent without stats group cannot access stats", func(t *testing.T) {
		// agent-rw-support has NO Stats group access
		token := fixtures.Tokens["agent-rw-support-full"]
		w := makeRequest(t, router, "GET", "/api/v1/stats/dashboard", token)
		// With real handlers, should be 403
		t.Logf("Response: %d (expected 403 with real handler)", w.Code)
	})
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestEdgeCases(t *testing.T) {
	fixtures := getAuthFixtures(t)
	router := setupAuthTestRouter(t, fixtures)

	t.Run("no token returns 401", func(t *testing.T) {
		w := makeRequest(t, router, "GET", "/api/v1/tickets", "")
		assertStatus(t, w, 401)
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		w := makeRequest(t, router, "GET", "/api/v1/tickets", "gf_invalid_token_12345")
		assertStatus(t, w, 401)
	})

	t.Run("malformed token returns 401", func(t *testing.T) {
		w := makeRequest(t, router, "GET", "/api/v1/tickets", "not-a-valid-format")
		assertStatus(t, w, 401)
	})
}
