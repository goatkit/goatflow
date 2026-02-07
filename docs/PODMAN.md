# Podman Support Documentation

## Overview

GoatFlow has first-class support for Podman, including rootless containers and SELinux compatibility. All containers run as non-root users for enhanced security.

## Container Security Principles

1. **Non-root by default**: All containers run as UID 1000 (appuser)
2. **Alpine-first**: Minimal attack surface with Alpine Linux
3. **SELinux labels**: `:Z` labels for Fedora/RHEL compatibility
4. **Rootless operation**: Full support for rootless Podman

## Quick Start with Podman

```bash
# The Makefile auto-detects podman-compose
make setup
make up

# Or explicitly:
podman-compose up
```

## Fedora Kinoite/Silverblue Setup

```bash
# Install podman-compose (if not present)
sudo rpm-ostree install podman-compose
systemctl reboot  # Required for rpm-ostree

# Clone and start GoatFlow
git clone https://github.com/goatflow/goatflow
cd goatflow
make setup
make up
```

## Rootless Podman Configuration

```bash
# Check subuid/subgid mappings
podman unshare cat /proc/self/uid_map

# Enable lingering for systemd user services
loginctl enable-linger $USER

# Start rootless containers
podman-compose up
```

## SELinux Considerations

All volume mounts include `:Z` labels for proper SELinux contexts:

```yaml
volumes:
  - ./:/app:Z  # Relabels for container access
```

## Systemd Integration

Generate systemd unit files for production:

```bash
# Generate unit files
make podman-systemd

# Install user units
mkdir -p ~/.config/systemd/user/
cp container-*.service ~/.config/systemd/user/

# Enable and start
systemctl --user daemon-reload
systemctl --user enable container-goatflow-backend.service
systemctl --user start container-goatflow-backend.service
```

## Network Configuration

### Rootless Networking
```bash
# Check available port range
cat /proc/sys/net/ipv4/ip_unprivileged_port_start

# For ports below 1024, use higher ports and map:
podman-compose -p goatflow up
```

### Pod Networking
```bash
# Create a pod for shared networking
podman pod create --name goatflow -p 80:80 -p 8025:8025

# Run containers in the pod
podman run -d --pod goatflow --name goatflow-backend goatflow-backend
```

## Storage Optimization

### Using Podman Volumes
```bash
# Create named volumes
podman volume create goatflow-postgres
podman volume create goatflow-valkey

# List volumes
podman volume ls

# Inspect volume
podman volume inspect goatflow-postgres
```

### Overlay Storage Driver
```bash
# Check storage driver
podman info | grep graphDriverName

# Configure overlay for better performance
# Edit ~/.config/containers/storage.conf
[storage]
driver = "overlay"
```

## Troubleshooting

### Permission Issues
```bash
# Fix volume permissions
podman unshare chown -R 1000:1000 ./

# Check container user
podman exec goatflow-backend id
```

### SELinux Denials
```bash
# Check for denials
sudo ausearch -m avc -ts recent

# Temporarily set permissive (debugging only)
sudo setenforce 0
```

### Rootless Limitations
- Cannot bind to ports < 1024 without sysctl changes
- Some network features unavailable
- Resource limits apply to user slice

### DNS Resolution
```bash
# If containers can't resolve DNS
podman run --dns 8.8.8.8 alpine nslookup google.com

# Add to compose file:
dns:
  - 8.8.8.8
  - 8.8.4.4
```

## Performance Tuning

### User Namespace Remapping
```bash
# Check current mappings
cat /etc/subuid
cat /etc/subgid

# Increase if needed (as root)
usermod --add-subuids 100000-165535 $USER
usermod --add-subgids 100000-165535 $USER
```

### Resource Limits
```bash
# Set CPU and memory limits
podman run --cpus="2" --memory="2g" goatflow-backend

# In compose file:
deploy:
  resources:
    limits:
      cpus: '2'
      memory: 2G
```

## Podman vs Docker Differences

| Feature | Docker | Podman |
|---------|--------|--------|
| Daemon | Required | Daemonless |
| Root by default | Yes | No |
| Systemd integration | Limited | Native |
| SELinux | Manual | Automatic |
| Rootless | Complex | Native |

## Security Best Practices

1. **Always run rootless** when possible
2. **Use non-root users** in containers (UID 1000)
3. **Enable SELinux** (default on Fedora/RHEL)
4. **Minimize capabilities** - drop unnecessary caps
5. **Use read-only root filesystem** where possible

```yaml
# Example secure container
security_opt:
  - no-new-privileges:true
  - seccomp=unconfined
cap_drop:
  - ALL
cap_add:
  - NET_BIND_SERVICE
read_only: true
```

## Additional Resources

- [Podman Documentation](https://docs.podman.io/)
- [Rootless Containers](https://rootlesscontaine.rs/)
- [Fedora Silverblue](https://silverblue.fedoraproject.org/)
- [SELinux and Containers](https://www.redhat.com/en/blog/selinux-and-containers)