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

echo "Setting admin password for ${TEST_USERNAME:-root@localhost} from TEST_PASSWORD env var"

LOGIN="${TEST_USERNAME:-root@localhost}"

mariadb \
    --ssl=0 \
    -h "${MARIADB_HOST:-localhost}" \
    -u "${MARIADB_USER}" \
    -p"${MARIADB_PASSWORD}" \
    "${MARIADB_DATABASE}" \
    -e "UPDATE users SET pw = '${PASSWORD_HASH}', valid_id = 1 WHERE login = '${LOGIN}';"

echo "Admin password hash and valid_id updated successfully for ${LOGIN}"
