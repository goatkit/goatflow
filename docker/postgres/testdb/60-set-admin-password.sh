#!/bin/bash
# Sets the test admin user's password from TEST_PASSWORD, after the SQL
# seeds, so the PostgreSQL test DB has the same admin credentials as the
# MariaDB one (docker/mariadb/testdb/60-set-admin-password.sh).
set -euo pipefail

if [ -z "${TEST_PASSWORD:-}" ]; then
    echo "TEST_PASSWORD not set, skipping admin password setup"
    exit 0
fi

PASSWORD_HASH=$(echo -n "${TEST_PASSWORD}" | sha256sum | cut -d' ' -f1)
LOGIN="${TEST_USERNAME:-root@localhost}"

echo "Setting up admin user ${LOGIN} with TEST_PASSWORD from env var"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -v login="$LOGIN" -v hash="$PASSWORD_HASH" <<'SQL'
UPDATE users SET pw = :'hash', valid_id = 1 WHERE login = :'login';

INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
SELECT :'login', :'hash', 'System', 'Administrator', 1, NOW(), 1, NOW(), 1
WHERE NOT EXISTS (SELECT 1 FROM users WHERE login = :'login');

-- Admin rights in groups 1-3, as the MariaDB script grants; group_user has
-- no unique key, so skip rows that already exist.
INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
SELECT u.id, g.id, 'rw', NOW(), 1, NOW(), 1
FROM users u CROSS JOIN (VALUES (1), (2), (3)) AS g(id)
WHERE u.login = :'login'
  AND NOT EXISTS (
      SELECT 1 FROM group_user gu
      WHERE gu.user_id = u.id AND gu.group_id = g.id AND gu.permission_key = 'rw'
  );
SQL

echo "Admin user ${LOGIN} is now ready for testing"
