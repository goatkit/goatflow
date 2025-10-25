#!/bin/sh
set -eu
exec mariadb --protocol=TCP --user="${MARIADB_USER}" --password="${MARIADB_PASSWORD}" "${MARIADB_DATABASE}" < /docker-entrypoint-initdb.d/testdb-init.sql
