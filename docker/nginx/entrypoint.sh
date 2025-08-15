#!/bin/sh

# Create log directories and symlinks for container logging
# This runs during the entrypoint.d phase, before nginx starts
mkdir -p /var/log/nginx
ln -sf /dev/stdout /var/log/nginx/access.log
ln -sf /dev/stderr /var/log/nginx/error.log

# Make sure nginx user can write to the log directory
chown -R nginx:nginx /var/log/nginx

echo "Nginx logging configured: access -> stdout, error -> stderr"