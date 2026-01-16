#!/bin/bash
# Setup test admin user for E2E tests
# This script is called by make test-setup-admin

set -e

if [ -z "${TEST_PASSWORD:-}" ]; then
    echo "TEST_PASSWORD not set, skipping admin setup"
    exit 0
fi

echo "Setting up test admin user..."

# Get compose command from environment or detect it
COMPOSE_CMD="${COMPOSE_CMD:-docker compose}"

# Try gotrs CLI first (if available in backend container)
if $COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml -f docker-compose.test.yaml \
    exec -T backend-test gotrs reset-user \
    --username="${TEST_USERNAME:-root@localhost}" \
    --password="$TEST_PASSWORD" \
    --enable 2>/dev/null; then
    echo "Admin user setup via gotrs CLI"
    exit 0
fi

echo "gotrs CLI not available, using direct SQL..."

# Calculate password hash
PW_HASH=$(printf '%s' "$TEST_PASSWORD" | sha256sum | awk '{print $1}')
echo "Password hash: ${PW_HASH:0:16}..."

# Set variables with defaults
DB_USER="${TEST_DB_MYSQL_USER:-otrs}"
DB_PASS="${TEST_DB_MYSQL_PASSWORD}"
DB_NAME="${TEST_DB_MYSQL_NAME:-otrs_test}"
USERNAME="${TEST_USERNAME:-root@localhost}"

if [ -z "$DB_PASS" ]; then
    echo "ERROR: TEST_DB_MYSQL_PASSWORD not set"
    exit 1
fi

echo "Connecting to database as user: $DB_USER"

# Run SQL update
if ! $COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
    mariadb --ssl=0 -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
    -e "UPDATE users SET pw = '$PW_HASH', valid_id = 1 WHERE login = '$USERNAME';"; then
    echo "ERROR: SQL update failed"
    exit 1
fi

# Verify the update
VERIFY=$($COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
    mariadb --ssl=0 -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
    -N -e "SELECT valid_id FROM users WHERE login = '$USERNAME';" 2>/dev/null || echo "")

if [ "$VERIFY" = "1" ]; then
    echo "Admin user verified (valid_id=1)"
else
    echo "ERROR: Admin user verification failed (valid_id='$VERIFY')"
    # Show what is in the users table
    echo "Users table contents:"
    $COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
        mariadb --ssl=0 -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
        -e "SELECT id, login, valid_id FROM users;" 2>/dev/null || echo "(query failed)"
    exit 1
fi

echo "Test admin user ready"
