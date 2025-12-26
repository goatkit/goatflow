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

MIGRATIONS_DIR="/docker-entrypoint-initdb.d/migrations"
MIGRATION_FILES=(
    000001_schema_alignment.up.sql
    000002_minimal_data.up.sql
    000003_queue_email_address.up.sql
    000004_dynamic_field_screen_config.up.sql
)

for file in "${MIGRATION_FILES[@]}"; do
    path="${MIGRATIONS_DIR}/${file}"
    if [ -f "$path" ]; then
        echo "Applying migration: ${file}"
        mariadb_cli < "$path"
    else
        echo "Skipping missing migration: ${file}"
    fi
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
