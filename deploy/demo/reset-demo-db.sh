#!/usr/bin/env bash
# Resets the public demo (goatflow-demo.gibbsoft.com) to its seed data.
#
# Runs from the demo server's nightly cron and from the release deploy
# (.github/workflows/build.yml, job deploy-demo). It lives next to
# docker-compose.yml, .env and goatflow_demo.sql.gz in ~/goatflow.
#
# 1. Loads goatflow_demo.sql.gz:
#    - each database in the dump is dropped first, so tables that newer
#      migrations created are not left behind next to the seed's older
#      schema_migrations version (the migrations would then fail on them);
#    - the mysql system database in the dump is skipped, so the database
#      users and passwords from .env stay as they are.
# 2. Restarts app, customer-fe and runner. Their start-up migrations bring the
#    restored (older) schema up to the running release; without the restart
#    the app would run on the seed's schema until the next reboot.
set -euo pipefail

cd "$(dirname "$(readlink -f "$0")")"

dump=goatflow_demo.sql.gz
if [ ! -f "$dump" ]; then
	echo "reset-demo-db: $dump not found in $PWD" >&2
	exit 1
fi

set -a
. ./.env
set +a

# mariadb-dump --all-databases starts each database with
# "-- Current Database: `name`" followed by CREATE DATABASE (later sections
# for views only USE it). MariaDB 11 dumps also start with a sandbox-mode
# comment the client rejects.
zcat "$dump" |
	awk 'NR == 1 && /enable the sandbox mode/ { next }
		/^-- Current Database: / { db = $4; skip = (db == "`mysql`") }
		!skip && /^CREATE DATABASE / { print "DROP DATABASE IF EXISTS " db ";" }
		!skip' |
	docker compose exec -T -e MYSQL_PWD="$DB_ROOT_PASSWORD" mariadb mariadb -uroot

docker compose restart app customer-fe runner
