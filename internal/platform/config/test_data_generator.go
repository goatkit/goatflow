package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/auth"
)

// TestCredential represents a test user credential.
type TestCredential struct {
	Username  string
	Password  string
	Email     string
	FirstName string
	LastName  string
	Role      string
	Type      string // "agent" or "customer"
}

// GeneratedTestDataPath is where `goatflow synthesize` writes the development test data.
// It is PostgreSQL SQL loaded by `make db-apply-test-data`, not a migration, so it must stay
// outside migrations/ (golang-migrate would treat it as a duplicate version).
const GeneratedTestDataPath = "schema/seed/generated_test_data.postgres.sql"

// TestDataGenerator generates test data SQL.
type TestDataGenerator struct {
	credentials []TestCredential
	synthesizer *Synthesizer
	sqlPath     string
}

// NewTestDataGenerator creates a new test data generator.
func NewTestDataGenerator(synthesizer *Synthesizer) *TestDataGenerator {
	return &TestDataGenerator{
		synthesizer: synthesizer,
		credentials: make([]TestCredential, 0),
		sqlPath:     GeneratedTestDataPath,
	}
}

// Generate creates test data with secure passwords.
func (g *TestDataGenerator) Generate() error {
	// Generate credentials for test users
	g.credentials = []TestCredential{
		// Admin user (OTRS-compatible root@localhost)
		{
			Username:  "root@localhost",
			Password:  g.generatePassword(),
			Email:     "root@localhost",
			FirstName: "Admin",
			LastName:  "OTRS",
			Role:      "admin",
			Type:      "agent",
		},
		// Test agents
		{
			Username:  "agent.smith",
			Password:  g.generatePassword(),
			Email:     "smith@goatflow.local",
			FirstName: "Agent",
			LastName:  "Smith",
			Role:      "agent",
			Type:      "agent",
		},
		{
			Username:  "agent.jones",
			Password:  g.generatePassword(),
			Email:     "jones@goatflow.local",
			FirstName: "Agent",
			LastName:  "Jones",
			Role:      "agent",
			Type:      "agent",
		},
		// Test customers
		{
			Username:  "john.customer",
			Password:  g.generatePassword(),
			Email:     "john@acme.com",
			FirstName: "John",
			LastName:  "Customer",
			Role:      "customer",
			Type:      "customer",
		},
		{
			Username:  "jane.customer",
			Password:  g.generatePassword(),
			Email:     "jane@techstart.com",
			FirstName: "Jane",
			LastName:  "Customer",
			Role:      "customer",
			Type:      "customer",
		},
		{
			Username:  "bob.customer",
			Password:  g.generatePassword(),
			Email:     "bob@global.com",
			FirstName: "Bob",
			LastName:  "Customer",
			Role:      "customer",
			Type:      "customer",
		},
	}

	if err := g.generateSQL(); err != nil {
		return fmt.Errorf("failed to generate SQL: %w", err)
	}

	return nil
}

// generatePassword creates a secure password.
func (g *TestDataGenerator) generatePassword() string {
	// Generate a secure but memorable password
	password, _ := g.synthesizer.GenerateSecret(SecretTypePassword, 16, "", "")
	// Add special character to ensure complexity
	return password + "!1"
}

// hashPassword hashes the password with the configured auth.PasswordHasher.
func (g *TestDataGenerator) hashPassword(password string) (string, error) {
	return auth.NewPasswordHasher().HashPassword(password)
}

// generateSQL writes the test data SQL file. It holds plaintext passwords in comments,
// so only the owner can read it.
func (g *TestDataGenerator) generateSQL() error {
	dir := filepath.Dir(g.sqlPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create %s: %w", dir, err)
	}

	file, err := os.OpenFile(g.sqlPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Write header
	fmt.Fprintf(file, "-- Auto-generated test data - DO NOT COMMIT TO GIT\n")
	fmt.Fprintf(file, "-- Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "-- This file is gitignored and should never be committed\n\n")

	// Add production check
	fmt.Fprintf(file, "DO $$\n")
	fmt.Fprintf(file, "BEGIN\n")
	fmt.Fprintf(file, "    IF current_setting('app.env', true) = 'production' THEN\n")
	fmt.Fprintf(file, "        RAISE EXCEPTION 'Test data migration cannot be run in production';\n")
	fmt.Fprintf(file, "    END IF;\n")
	fmt.Fprintf(file, "END $$;\n\n")

	// Write test companies
	fmt.Fprintf(file, "-- Test customer companies\n")
	fmt.Fprintf(file, "INSERT INTO customer_company (customer_id, name, street, city, country, valid_id, create_time, create_by, change_time, change_by) VALUES\n")
	fmt.Fprintf(file, "('COMP1', 'Acme Corporation', '123 Main St', 'New York', 'USA', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),\n")
	fmt.Fprintf(file, "('COMP2', 'TechStart Inc', '456 Tech Ave', 'San Francisco', 'USA', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1),\n")
	fmt.Fprintf(file, "('COMP3', 'Global Services Ltd', '789 Business Park', 'London', 'UK', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)\n")
	fmt.Fprintf(file, "ON CONFLICT (customer_id) DO NOTHING;\n\n")

	// Write agents
	fmt.Fprintf(file, "-- Test agents (dynamically generated passwords)\n")
	for _, cred := range g.credentials {
		if cred.Type == "agent" {
			fmt.Fprintf(file, "-- || %s / %s\n", cred.Username, cred.Password)
		}
	}
	fmt.Fprintf(file, "INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by) VALUES\n")

	agents := []string{}
	for _, cred := range g.credentials {
		if cred.Type == "agent" {
			hash, err := g.hashPassword(cred.Password)
			if err != nil {
				return fmt.Errorf("failed to hash password for %s: %w", cred.Username, err)
			}
			agents = append(agents, fmt.Sprintf("('%s', '%s', '%s', '%s', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)",
				cred.Username, hash, cred.FirstName, cred.LastName))
		}
	}
	fmt.Fprintf(file, "%s\n", strings.Join(agents, ",\n"))
	// Existing logins (root@localhost from migration 000002) take the generated password
	// and are enabled, so the "-- ||" credentials above always work.
	fmt.Fprintf(file, "ON CONFLICT (login) DO UPDATE SET pw = EXCLUDED.pw, valid_id = 1, change_time = CURRENT_TIMESTAMP;\n\n")

	// Write customers
	fmt.Fprintf(file, "-- Test customers (dynamically generated passwords)\n")
	for _, cred := range g.credentials {
		if cred.Type == "customer" {
			fmt.Fprintf(file, "-- || %s / %s\n", cred.Username, cred.Password)
		}
	}
	fmt.Fprintf(file, "INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, phone, valid_id, create_time, create_by, change_time, change_by) VALUES\n")

	customers := []string{}
	companyMap := map[string]string{
		"john@acme.com":      "COMP1",
		"jane@techstart.com": "COMP2",
		"bob@global.com":     "COMP3",
	}
	phoneNum := 101

	for _, cred := range g.credentials {
		if cred.Type == "customer" {
			hash, err := g.hashPassword(cred.Password)
			if err != nil {
				return fmt.Errorf("failed to hash password for %s: %w", cred.Username, err)
			}
			company := companyMap[cred.Email]
			customers = append(customers, fmt.Sprintf("('%s', '%s', '%s', '%s', '%s', '%s', '555-0%d', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)",
				cred.Username, cred.Email, company, hash, cred.FirstName, cred.LastName, phoneNum))
			phoneNum++
		}
	}
	fmt.Fprintf(file, "%s\n", strings.Join(customers, ",\n"))
	fmt.Fprintf(file, "ON CONFLICT (login) DO UPDATE SET pw = EXCLUDED.pw, valid_id = 1, change_time = CURRENT_TIMESTAMP;\n\n")

	// Add remaining test data (groups, tickets, articles). Every statement is
	// re-runnable: group_user and article have no unique key, so they are guarded.
	fmt.Fprintf(file, `-- Add agents to the users group
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
SELECT u.id, g.id, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM users u JOIN groups g ON g.name = 'users'
WHERE u.login IN ('root@localhost', 'agent.smith', 'agent.jones')
  AND NOT EXISTS (
    SELECT 1 FROM group_user gu
    WHERE gu.user_id = u.id AND gu.group_id = g.id AND gu.permission_key = 'rw'
  );

-- Sample tickets in the Misc queue (owner and responsible agent looked up by login)
INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, user_id, responsible_user_id, ticket_priority_id, ticket_state_id,
    customer_id, customer_user_id, timeout, until_time, escalation_time, escalation_update_time,
    escalation_response_time, escalation_solution_time, create_time, create_by, change_time, change_by)
SELECT v.tn, v.title, q.id, 1, u.id, u.id, v.priority_id, s.id, v.customer_id, v.customer_user_id,
    0, 0, 0, 0, 0, 0, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM (VALUES
    ('2025010000001', 'Cannot login to system', 'agent.smith', 3, 'new', 'COMP1', 'john.customer'),
    ('2025010000002', 'Request for new feature', 'agent.jones', 2, 'open', 'COMP2', 'jane.customer'),
    ('2025010000003', 'System running slow', 'agent.smith', 4, 'open', 'COMP1', 'john.customer'),
    ('2025010000004', 'Password reset needed', 'agent.jones', 3, 'new', 'COMP3', 'bob.customer'),
    ('2025010000005', 'API documentation request', 'agent.smith', 2, 'closed successful', 'COMP2', 'jane.customer')
) AS v(tn, title, agent_login, priority_id, state_name, customer_id, customer_user_id)
JOIN users u ON u.login = v.agent_login
JOIN ticket_state s ON s.name = v.state_name
JOIN queue q ON q.name = 'Misc'
ON CONFLICT (tn) DO NOTHING;

-- One customer email article per sample ticket
INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
    create_time, create_by, change_time, change_by)
SELECT t.id, 3, 1, 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
FROM ticket t
WHERE t.tn IN ('2025010000001', '2025010000002', '2025010000003', '2025010000004', '2025010000005')
  AND NOT EXISTS (SELECT 1 FROM article a WHERE a.ticket_id = t.id);

-- Article content (a_body is bytea, incoming_time is integer Unix timestamp)
INSERT INTO article_data_mime (article_id, a_from, a_to, a_subject, a_body, incoming_time,
    create_time, create_by, change_time, change_by)
SELECT
    a.id,
    cu.email,
    'support@example.com',
    t.title,
    CAST(('Initial ticket description for: ' || t.title) AS bytea),
    EXTRACT(EPOCH FROM t.create_time)::integer,
    CURRENT_TIMESTAMP,
    1,
    CURRENT_TIMESTAMP,
    1
FROM article a
JOIN ticket t ON a.ticket_id = t.id
JOIN customer_user cu ON t.customer_user_id = cu.login
WHERE t.tn IN ('2025010000001', '2025010000002', '2025010000003', '2025010000004', '2025010000005')
  AND NOT EXISTS (SELECT 1 FROM article_data_mime m WHERE m.article_id = a.id);
`)

	return nil
}
