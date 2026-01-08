#!/bin/sh
# MariaDB healthcheck script
# Uses environment variables MYSQL_USER and MYSQL_PASSWORD set by container

mariadb-admin ping -h 127.0.0.1 -u"${MYSQL_USER}" -p"${MYSQL_PASSWORD}"
