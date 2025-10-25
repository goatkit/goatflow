#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DRIVER="${DB_CONN_DRIVER:-${DB_DRIVER:-postgres}}"

case "$DRIVER" in
	postgres|pgsql)
		exec "$SCRIPT_DIR/db/postgres/reset-user-password.sh" "$@"
		;;
	mysql|mariadb)
		exec "$SCRIPT_DIR/db/mysql/reset-user-password.sh" "$@"
		;;
	*)
		echo "Unsupported DB driver '$DRIVER'" >&2
		exit 1
		;;
esac
