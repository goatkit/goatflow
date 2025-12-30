#!/bin/bash
# Load test fixtures into the test database
set -e

echo "📦 Loading E2E test fixtures..."

mysql -u"${MYSQL_USER:-otrs}" -p"${MYSQL_PASSWORD}" "${MYSQL_DATABASE:-otrs_test}" < /docker-entrypoint-initdb.d/70-test-fixtures.sql

echo "✅ Test fixtures loaded successfully"
