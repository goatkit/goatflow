#!/bin/bash
set -euo pipefail

mariadb_cli() {
    mariadb \
        --ssl=0 \
        --init-command="SET SESSION foreign_key_checks = 0" \
        -h "${MARIADB_HOST:-localhost}" \
        -u "${MARIADB_USER}" \
        -p"${MARIADB_PASSWORD}" \
        "${MARIADB_DATABASE}"
}

mariadb_root_cli() {
    if [ -z "${MARIADB_ROOT_PASSWORD:-}" ]; then
        echo "MARIADB_ROOT_PASSWORD not set; cannot perform privileged operations" >&2
        return 1
    fi

    mariadb \
        --ssl=0 \
        -h "${MARIADB_HOST:-localhost}" \
        -u root \
        -p"${MARIADB_ROOT_PASSWORD}" \
        "${MARIADB_DATABASE}"
}

# Apply every up migration in version order, so the test DB has the same
# schema as a migrated install. The data dir is tmpfs: each container
# (re)creation starts empty and relies on this script alone. A hard-coded
# list here went stale and left later tables (canned_response,
# user_api_tokens, gk_identity_provider, ...) missing.
MIGRATIONS_DIR="/docker-entrypoint-initdb.d/migrations"
shopt -s nullglob
MIGRATION_FILES=("${MIGRATIONS_DIR}"/*.up.sql)
if [ "${#MIGRATION_FILES[@]}" -eq 0 ]; then
    echo "No migrations found in ${MIGRATIONS_DIR}" >&2
    exit 1
fi

for path in "${MIGRATION_FILES[@]}"; do
    echo "Applying migration: $(basename "$path")"
    mariadb_cli < "$path"
done

echo "Ensuring '${MARIADB_USER}' has remote access"
if mariadb_root_cli >/dev/null 2>&1 <<SQL
GRANT ALL PRIVILEGES ON \`${MARIADB_DATABASE}\`.* TO '${MARIADB_USER}'@'%' IDENTIFIED BY '${MARIADB_PASSWORD}';
FLUSH PRIVILEGES;
SQL
then
    true
else
    echo "Warning: failed to grant remote access for '${MARIADB_USER}', proceeding without modification" >&2
fi

echo "Re-enabling foreign key checks"
mariadb_cli <<'EOSQL'
SET foreign_key_checks = 1;
EOSQL

echo "MariaDB test migrations applied successfully."
