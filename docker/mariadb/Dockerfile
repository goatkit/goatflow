# Secure non-root MariaDB container
FROM mariadb:11-alpine

# Create non-root user and group (UID/GID 1000 for consistency)
RUN addgroup -g 1000 -S mysql && \
    adduser -u 1000 -D -S -G mysql mysql

# Create necessary directories with proper ownership
RUN mkdir -p /var/lib/mysql /var/run/mysqld /docker-entrypoint-initdb.d && \
    chown -R mysql:mysql /var/lib/mysql /var/run/mysqld /docker-entrypoint-initdb.d && \
    chmod 755 /var/run/mysqld

# Copy init script if needed (init.sql is optional)
# NOTE: If init.sql doesn't exist, build will fail - create an empty one or remove this line
COPY --chown=mysql:mysql docker/mariadb/init.sql /docker-entrypoint-initdb.d/

# Set proper permissions for entrypoint
RUN chmod +x /usr/local/bin/docker-entrypoint.sh || true

# Switch to non-root user
USER mysql

# Expose MariaDB port
EXPOSE 3306

# Use the official entrypoint
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["mysqld"]