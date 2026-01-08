#!/bin/sh
# MariaDB healthcheck script
# Uses MYSQL_PWD environment variable to avoid exposing password in process list

export MYSQL_PWD="${MYSQL_PASSWORD}"
mariadb-admin ping -h 127.0.0.1 -u"${MYSQL_USER}"
exit $?
