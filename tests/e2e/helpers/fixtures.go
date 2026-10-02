package helpers

// Rows seeded into both test databases by schema/seed/test_integration.sql
// (PostgreSQL) and schema/seed/test_integration_mysql.sql (MariaDB). The e2e
// suites rely on them; a missing row is a broken test stack, not a skip.
const (
	// SeedTemplateName is a valid standard_template row.
	SeedTemplateName = "E2E Seed Answer"

	// SeedCustomerLogin / SeedCustomerPassword: customer_user of company COMP1,
	// pw stored as the unsalted SHA-256 the customer login verifies.
	SeedCustomerLogin    = "e2e.customer"
	SeedCustomerPassword = "E2eCustomer!Seed1"

	// Seed2FAAgentLogin / Seed2FAAgentPassword: agent in group users (rw) with
	// 2FA disabled, dedicated to the TOTP flow so it never touches the admin.
	Seed2FAAgentLogin    = "e2e-2fa-agent"
	Seed2FAAgentPassword = "E2eTwoFactor!Seed1"

	// SeedDynamicFieldName is the Ticket Text dynamic field with id 1.
	SeedDynamicFieldName = "TestTextField"

	// SeedQueueName is the queue with id 5.
	SeedQueueName = "Support"
)
