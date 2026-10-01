#!/bin/bash
set -euo pipefail

psql=(psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB")

MIGRATIONS_DIR="/docker-entrypoint-initdb.d/migrations"

apply_migration() {
    local file="$1"
    echo "Applying migration: $file"
    "${psql[@]}" -f "$MIGRATIONS_DIR/$file"
}

# Apply every up migration in version order, so the test DB has the same
# schema as a migrated install. The data dir is tmpfs: each container
# (re)creation starts empty and relies on this script alone. A hard-coded
# list here went stale and left every table after migration 1 missing.
shopt -s nullglob
MIGRATION_FILES=("$MIGRATIONS_DIR"/*.up.sql)
if [ "${#MIGRATION_FILES[@]}" -eq 0 ]; then
    echo "No migrations found in $MIGRATIONS_DIR" >&2
    exit 1
fi

for path in "${MIGRATION_FILES[@]}"; do
    apply_migration "$(basename "$path")"
done

echo "All migrations applied successfully."
