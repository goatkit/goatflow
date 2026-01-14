#!/bin/bash
# Sets admin user password from TEST_PASSWORD env var
# This runs after SQL seeds and ensures test admin credentials work
set -euo pipefail

if [ -z "${TEST_PASSWORD:-}" ]; then
    echo "TEST_PASSWORD not set, skipping admin password setup"
    exit 0
fi

# Calculate SHA256 hash of the password
PASSWORD_HASH=$(echo -n "${TEST_PASSWORD}" | sha256sum | cut -d' ' -f1)

LOGIN="${TEST_USERNAME:-root@localhost}"

echo "Setting up admin user ${LOGIN} with TEST_PASSWORD from env var"

# First check if the user exists
USER_EXISTS=$(mariadb \
    --ssl=0 \
    -h "${MARIADB_HOST:-localhost}" \
    -u "${MARIADB_USER}" \
    -p"${MARIADB_PASSWORD}" \
    "${MARIADB_DATABASE}" \
    -N -e "SELECT COUNT(*) FROM users WHERE login = '${LOGIN}';" 2>/dev/null || echo "0")

if [ "${USER_EXISTS}" = "0" ]; then
    echo "User ${LOGIN} does not exist, creating it..."
    # Insert the admin user with the hashed password
    # Use a high ID to avoid conflicts with existing data
    mariadb \
        --ssl=0 \
        -h "${MARIADB_HOST:-localhost}" \
        -u "${MARIADB_USER}" \
        -p"${MARIADB_PASSWORD}" \
        "${MARIADB_DATABASE}" \
        -e "INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
            VALUES ('${LOGIN}', '${PASSWORD_HASH}', 'System', 'Administrator', 1, NOW(), 1, NOW(), 1);"

    # Get the new user's ID
    NEW_USER_ID=$(mariadb \
        --ssl=0 \
        -h "${MARIADB_HOST:-localhost}" \
        -u "${MARIADB_USER}" \
        -p"${MARIADB_PASSWORD}" \
        "${MARIADB_DATABASE}" \
        -N -e "SELECT id FROM users WHERE login = '${LOGIN}';" 2>/dev/null)

    echo "Created user ${LOGIN} with ID ${NEW_USER_ID}"

    # Grant admin group permissions (group_id 2 = admin)
    mariadb \
        --ssl=0 \
        -h "${MARIADB_HOST:-localhost}" \
        -u "${MARIADB_USER}" \
        -p"${MARIADB_PASSWORD}" \
        "${MARIADB_DATABASE}" \
        -e "INSERT IGNORE INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
            VALUES (${NEW_USER_ID}, 1, 'rw', NOW(), 1, NOW(), 1),
                   (${NEW_USER_ID}, 2, 'rw', NOW(), 1, NOW(), 1),
                   (${NEW_USER_ID}, 3, 'rw', NOW(), 1, NOW(), 1);"

    echo "Granted admin permissions to user ${LOGIN}"
else
    echo "User ${LOGIN} exists, updating password and valid_id..."
    mariadb \
        --ssl=0 \
        -h "${MARIADB_HOST:-localhost}" \
        -u "${MARIADB_USER}" \
        -p"${MARIADB_PASSWORD}" \
        "${MARIADB_DATABASE}" \
        -e "UPDATE users SET pw = '${PASSWORD_HASH}', valid_id = 1 WHERE login = '${LOGIN}';"
fi

echo "Admin user ${LOGIN} is now ready for testing"
