#!/bin/bash
set -euo pipefail

scope=${DB_SCOPE:-primary}
cmd=${1:-print}

admin_user=""
admin_user_var=""
admin_password_var=""
host_port=""

# Derive defaults for each scope
case "$scope" in
  primary)
    driver=${DB_DRIVER:-mariadb}
    host=${DB_HOST:-mariadb}
    port=${DB_PORT:-}
    if [ -z "$port" ]; then
      if [ "${DB_DRIVER:-mariadb}" = "postgres" ]; then
        port=5432
      else
        port=3306
      fi
    fi
    host_port=$port
    name=${DB_NAME:-gotrs}
    user=${DB_USER:-gotrs_user}
    password=${DB_PASSWORD:-}
    if [ -z "$password" ]; then
      echo "ERROR: DB_PASSWORD must be set in .env" >&2
      exit 1
    fi
    admin_user_var=ADMIN_USER
    admin_password_var=ADMIN_PASSWORD
    admin_user=${ADMIN_USER:-root@localhost}
    ;;
  pg-test|postgres-test)
    driver=postgres
    host=${TEST_DB_POSTGRES_HOST:-postgres-test}
    host_port=${TEST_DB_POSTGRES_PORT:-5433}
    port=${TEST_DB_POSTGRES_INTERNAL_PORT:-5432}
    name=${TEST_DB_POSTGRES_NAME:-gotrs_test}
    user=${TEST_DB_POSTGRES_USER:-gotrs_user}
    password=${TEST_DB_POSTGRES_PASSWORD:-}
    if [ -z "$password" ]; then
      echo "ERROR: TEST_DB_POSTGRES_PASSWORD must be set in .env" >&2
      exit 1
    fi
    scope="pg-test"
    admin_user_var=TEST_PG_ADMIN_USER
    admin_password_var=TEST_PG_ADMIN_PASSWORD
    admin_user=${TEST_PG_ADMIN_USER:-root@localhost}
    ;;
  mysql-test|mariadb-test)
    driver=mysql
    host=${TEST_DB_MYSQL_HOST:-mariadb-test}
    host_port=${TEST_DB_MYSQL_PORT:-3308}
    port=${TEST_DB_MYSQL_INTERNAL_PORT:-3306}
    name=${TEST_DB_MYSQL_NAME:-${TEST_DB_NAME:-${DB_NAME:-otrs_test}}}
    user=${TEST_DB_MYSQL_USER:-${TEST_DB_USER:-otrs}}
    password=${TEST_DB_PASSWORD:-${TEST_DB_MYSQL_PASSWORD:-}}
    if [ -z "$password" ]; then
      echo "ERROR: TEST_DB_MYSQL_PASSWORD must be set in .env" >&2
      exit 1
    fi
    scope="mysql-test"
    admin_user_var=TEST_MYSQL_ADMIN_USER
    admin_password_var=TEST_MYSQL_ADMIN_PASSWORD
    admin_user=${TEST_MYSQL_ADMIN_USER:-root@localhost}
    ;;
  *)
    echo "Unknown DB_SCOPE '$scope'" >&2
    exit 1
    ;;
esac

describe() {
  printf "Using scope: %s (driver=%s, host=%s, port=%s, db=%s, user=%s, admin_user=%s" \
    "$scope" "$driver" "$host" "$port" "$name" "$user" "$admin_user"
  if [ -n "$host_port" ] && [ "$host_port" != "$port" ]; then
    printf ", host_port=%s" "$host_port"
  fi
  printf ")\n"
}

print_exports() {
  cat <<EOF
DB_CONN_SCOPE=$scope
DB_CONN_DRIVER=$driver
DB_CONN_HOST=$host
DB_CONN_PORT=$port
DB_CONN_HOST_PORT=${host_port:-$port}
DB_CONN_NAME=$name
DB_CONN_USER=$user
DB_CONN_PASSWORD=$password
DB_CONN_ADMIN_USER=$admin_user
DB_CONN_ADMIN_USER_VAR=$admin_user_var
DB_CONN_ADMIN_PASSWORD_VAR=$admin_password_var
EOF
}

case "$cmd" in
  print)
    print_exports
    ;;
  describe)
    describe
    ;;
  *)
    echo "Usage: DB_SCOPE=<scope> $0 [print|describe]" >&2
    exit 1
    ;;
esac
