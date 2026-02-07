package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
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

// TestDataGenerator generates test data SQL and CSV files.
type TestDataGenerator struct {
	credentials []TestCredential
	synthesizer *Synthesizer
	sqlPath     string
	csvPath     string
}

// NewTestDataGenerator creates a new test data generator.
func NewTestDataGenerator(synthesizer *Synthesizer) *TestDataGenerator {
	return &TestDataGenerator{
		synthesizer: synthesizer,
		credentials: make([]TestCredential, 0),
		sqlPath:     "migrations/postgres/000004_generated_test_data.up.sql",
		csvPath:     "test_credentials.csv",
	}
}

// SetPaths sets custom paths for output files.
func (g *TestDataGenerator) SetPaths(sqlPath, csvPath string) {
	g.sqlPath = sqlPath
	g.csvPath = csvPath
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

	// Generate SQL file
	if err := g.generateSQL(); err != nil {
		return fmt.Errorf("failed to generate SQL: %w", err)
	}

	// Generate CSV file
	if err := g.generateCSV(); err != nil {
		return fmt.Errorf("failed to generate CSV: %w", err)
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

// hashPassword creates a bcrypt hash of the password.
func (g *TestDataGenerator) hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// generateSQL creates the SQL migration file.
func (g *TestDataGenerator) generateSQL() error {
	// Ensure migrations directory exists
	dir := filepath.Dir(g.sqlPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	file, err := os.Create(g.sqlPath)
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
	fmt.Fprintf(file, "ON CONFLICT (login) DO NOTHING;\n\n")

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
	fmt.Fprintf(file, "ON CONFLICT (login) DO NOTHING;\n\n")

	// Add remaining test data (groups, tickets, etc.)
	fmt.Fprintf(file, `-- Add agents to users group
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) 
SELECT id, 2, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1 FROM users WHERE login IN ('root@localhost', 'agent.smith', 'agent.jones')
ON CONFLICT DO NOTHING;

-- Sample tickets (use subqueries for user_id references)
INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, user_id, ticket_priority_id, ticket_state_id, customer_id, customer_user_id, create_by, change_by) VALUES
('2025010000001', 'Cannot login to system', 4, 1, (SELECT id FROM users WHERE login = 'agent.smith'), 3, 1, 'COMP1', 'john.customer', 1, 1),
('2025010000002', 'Request for new feature', 4, 1, (SELECT id FROM users WHERE login = 'agent.jones'), 2, 2, 'COMP2', 'jane.customer', 1, 1),
('2025010000003', 'System running slow', 4, 1, (SELECT id FROM users WHERE login = 'agent.smith'), 4, 2, 'COMP1', 'john.customer', 1, 1),
('2025010000004', 'Password reset needed', 4, 1, (SELECT id FROM users WHERE login = 'agent.jones'), 3, 1, 'COMP3', 'bob.customer', 1, 1),
('2025010000005', 'API documentation request', 4, 1, (SELECT id FROM users WHERE login = 'agent.smith'), 2, 3, 'COMP2', 'jane.customer', 1, 1)
ON CONFLICT (tn) DO NOTHING;

-- Sample articles for tickets
INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer, create_by, change_by)
SELECT id, 3, 1, 1, 1, 1 FROM ticket
ON CONFLICT DO NOTHING;

-- Add article content (a_body is bytea, incoming_time is integer Unix timestamp)
INSERT INTO article_data_mime (article_id, a_from, a_to, a_subject, a_body, incoming_time, create_by, change_by)
SELECT 
    a.id,
    cu.email,
    'support@example.com',
    t.title,
    CAST(('Initial ticket description for: ' || t.title) AS bytea),
    EXTRACT(EPOCH FROM t.create_time)::integer,
    1,
    1
FROM article a
JOIN ticket t ON a.ticket_id = t.id
JOIN customer_user cu ON t.customer_user_id = cu.login
ON CONFLICT DO NOTHING;
`)

	return nil
}

// DEPRECATED: Use 'make show-dev-creds' to extract from SQL comments instead.
func (g *TestDataGenerator) generateCSV() error {
	// No longer output CSV since credentials are in SQL comments
	// Use: grep "^-- ||" migrations/postgres/000004_generated_test_data.up.sql
	return nil
}

// GetCredentials returns the generated credentials.
func (g *TestDataGenerator) GetCredentials() []TestCredential {
	return g.credentials
}

// GetCredentialByUsername returns a specific credential.
func (g *TestDataGenerator) GetCredentialByUsername(username string) *TestCredential {
	for _, cred := range g.credentials {
		if cred.Username == username {
			return &cred
		}
	}
	return nil
}
