#!/bin/bash
# Setup test admin user for E2E tests
# This script is called by make test-setup-admin

set -e

echo "=== Test Admin Setup ==="
echo "TEST_PASSWORD length: ${#TEST_PASSWORD}"

if [ -z "${TEST_PASSWORD:-}" ]; then
    echo "ERROR: TEST_PASSWORD not set, cannot setup admin user"
    exit 1
fi

echo "Setting up test admin user..."

# Get compose command from environment or detect it
COMPOSE_CMD="${COMPOSE_CMD:-docker compose}"

# Try goatflow CLI first (if available in backend container)
if $COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml -f docker-compose.test.yaml \
    exec -T backend-test goatflow reset-user \
    --username="${TEST_USERNAME:-root@localhost}" \
    --password="$TEST_PASSWORD" \
    --enable 2>/dev/null; then
    echo "Admin user setup via goatflow CLI"
    exit 0
fi

echo "goatflow CLI not available, using direct SQL..."

# Calculate password hash
PW_HASH=$(printf '%s' "$TEST_PASSWORD" | sha256sum | awk '{print $1}')
echo "Password hash (first 16 chars): ${PW_HASH:0:16}..."

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

# Check current user ID 1 status before update
echo "Checking current user ID 1 status..."
$COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
    mariadb --ssl=0 -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
    -e "SELECT id, login, valid_id, SUBSTRING(pw, 1, 16) as pw_prefix FROM users WHERE id = 1;" 2>/dev/null || echo "(query failed)"

# Run SQL update - restore user ID 1 with correct login, password, and valid status
# This handles the case where unit tests may have changed user ID 1's login to something else
echo "Restoring user ID 1 with login '$USERNAME', updating password and enabling..."
if ! $COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
    mariadb --ssl=0 -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
    -e "UPDATE users SET login = '$USERNAME', pw = '$PW_HASH', valid_id = 1 WHERE id = 1;"; then
    echo "ERROR: SQL update failed"
    exit 1
fi

# Verify the update
echo "Verifying user ID 1 after update..."
$COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
    mariadb --ssl=0 -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
    -e "SELECT id, login, valid_id, SUBSTRING(pw, 1, 16) as pw_prefix FROM users WHERE id = 1;" 2>/dev/null || echo "(query failed)"

VERIFY=$($COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
    mariadb --ssl=0 -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
    -N -e "SELECT valid_id FROM users WHERE id = 1 AND login = '$USERNAME';" 2>/dev/null || echo "")

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

# Restore admin user's group permissions (unit tests may have modified them)
# Admin user needs rw access to groups 1 (users), 2 (admin), 3 (stats)
echo "Restoring admin user group permissions..."
$COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
    mariadb --ssl=0 -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
    -e "
    -- Delete existing permissions for user 1 to avoid duplicates
    DELETE FROM group_user WHERE user_id = 1;

    -- Restore full admin permissions (rw on groups 1, 2, 3)
    INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) VALUES
    (1, 1, 'rw', NOW(), 1, NOW(), 1),
    (1, 2, 'rw', NOW(), 1, NOW(), 1),
    (1, 3, 'rw', NOW(), 1, NOW(), 1);
    " || echo "Warning: Failed to restore group permissions"

# Verify group permissions
echo "Verifying admin user group permissions..."
$COMPOSE_CMD -f docker-compose.yml -f docker-compose.testdb.yml exec -T mariadb-test \
    mariadb --ssl=0 -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" \
    -e "SELECT gu.user_id, gu.group_id, gu.permission_key FROM group_user gu WHERE gu.user_id = 1;" 2>/dev/null || echo "(query failed)"

echo "=== Test Admin Setup Complete ==="
