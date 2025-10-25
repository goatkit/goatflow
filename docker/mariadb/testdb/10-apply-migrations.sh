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

MIGRATIONS_DIR="/docker-entrypoint-initdb.d/migrations"
MIGRATION_FILES=(
    000001_schema_alignment.up.sql
    000002_minimal_data.up.sql
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

echo "Re-enabling foreign key checks"
mariadb_cli <<'EOSQL'
SET foreign_key_checks = 1;
EOSQL

echo "MariaDB test migrations applied successfully."
