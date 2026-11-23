FROM docker.io/mariadb:13

USER 999:999

COPY --chown=999:999 testdb-init.sql /docker-entrypoint-initdb.d/50-testdb-init.sql

