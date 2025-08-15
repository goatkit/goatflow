FROM nginx:alpine

# Forward request and error logs to docker log collector
RUN ln -sf /dev/stdout /var/log/nginx/access.log \
    && ln -sf /dev/stderr /var/log/nginx/error.log

# Ensure nginx can write to the log files by making sure it runs as root
# In containerized environments, this is typically acceptable
USER root