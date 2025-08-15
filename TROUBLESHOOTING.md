# GOTRS Troubleshooting Guide

## Common Issues and Solutions

### 1. Database Connection Error: "database gotrs_user does not exist"

**Problem:** You see an error like:
```
FATAL: database "gotrs_user" does not exist
```

**Cause:** The `.env` file has incorrect database user configuration that doesn't match the database name.

**Solution:**

1. **Run the fix script:**
   ```bash
   ./fix-database.sh
   ```

2. **Or manually fix:**
   ```bash
   # Stop and clean everything
   docker compose down -v  # or docker-compose down -v
   
   # Edit .env file and change:
   # DB_USER=gotrs_user  → DB_USER=gotrs
   
   # Start fresh
   docker compose up  # or make up
   ```

3. **Verify the fix:**
   - Check `.env` has `DB_USER=gotrs`
   - Check `.env` has `DB_NAME=gotrs`
   - Both should match for PostgreSQL to initialize correctly

### 2. Docker Compose Command Not Found

**Problem:** You see:
```
make: docker-compose: No such file or directory
```

**Cause:** Modern Docker uses `docker compose` (with space) instead of `docker-compose` (with hyphen).

**Solution:**

The Makefile now auto-detects the correct command. To check what's available:
```bash
make debug-env
```

You can also use the wrapper script:
```bash
./compose.sh up     # Auto-detects the right command
```

### 3. Container Won't Start

**Problem:** Containers fail to start or keep restarting.

**Solution:**

1. **Check logs:**
   ```bash
   make logs
   # or for specific service:
   docker compose logs postgres
   ```

2. **Clean everything and restart:**
   ```bash
   make clean
   make up
   ```

3. **Check port conflicts:**
   ```bash
   # Check if ports are in use
   lsof -i :5432  # PostgreSQL
   lsof -i :8080  # Backend
   lsof -i :3000  # Frontend
   ```

### 4. Tests Can't Connect to Database

**Problem:** Running `make test` fails with connection errors.

**Solution:**

1. **Ensure services are running:**
   ```bash
   docker compose ps
   # All services should show as "running"
   ```

2. **Wait for services to be healthy:**
   ```bash
   # Check health status
   docker compose ps
   # Look for (healthy) status
   ```

3. **Run migrations if needed:**
   ```bash
   make db-migrate
   ```

### 5. Permission Denied Errors

**Problem:** You see permission errors when accessing files or volumes.

**Solution for Docker:**
```bash
# Add your user to docker group
sudo usermod -aG docker $USER
# Log out and back in
```

**Solution for Podman (rootless):**
```bash
# Podman runs rootless by default, check SELinux labels
# Volumes should have :Z flag in docker-compose.yml
```

### 6. Out of Disk Space

**Problem:** Containers fail to start due to disk space.

**Solution:**
```bash
# Clean up Docker/Podman
docker system prune -a --volumes
# or
podman system prune -a --volumes

# Check disk usage
df -h
docker system df
```

### 7. Backend Can't Find Dependencies

**Problem:** Go modules not found or build fails.

**Solution:**
```bash
# The Makefile will handle dependencies, but if needed:
docker compose exec backend go mod download
docker compose exec backend go mod tidy
```

### 8. Frontend Build Errors

**Problem:** Frontend fails to compile or start.

**Solution:**
```bash
# Rebuild frontend
docker compose build frontend --no-cache
docker compose up frontend
```

### 9. Database Migrations Fail

**Problem:** Migrations won't run or fail with errors.

**Solution:**
```bash
# Reset database completely
make clean
make up
make db-migrate

# Or manually:
docker compose exec -e PGPASSWORD=gotrs_password backend \
  psql -h postgres -U gotrs -d gotrs < migrations/000001_initial_schema.up.sql
```

### 10. Tests Fail to Run

**Problem:** `make test` doesn't work.

**Solution:**

1. **Ensure testify is installed:**
   ```bash
   docker compose exec backend go get github.com/stretchr/testify
   ```

2. **Run tests directly:**
   ```bash
   docker compose exec backend go test -v ./...
   ```

3. **Check test coverage:**
   ```bash
   make test-coverage
   ```

## Debugging Commands

### Check System Status
```bash
# Show what compose command is being used
make debug-env

# Check running containers
docker compose ps

# Check logs
make logs

# Shell into container
docker compose exec backend sh
docker compose exec postgres psql -U gotrs -d gotrs
```

### Clean Slate Reset
```bash
# Complete cleanup and restart
./fix-database.sh
make clean
make setup
make up
```

### Environment Verification
```bash
# Check .env values
grep DB_ .env

# Verify compose file is valid
docker compose config

# Test database connection
docker compose exec postgres pg_isready -U gotrs
```

## Getting Help

If you continue to experience issues:

1. Check the logs: `make logs`
2. Review your `.env` file configuration
3. Ensure Docker/Podman daemon is running
4. Check the [GitHub Issues](https://github.com/gotrs-io/gotrs-ce/issues)
5. Run `make debug-env` to see your environment setup

## Prevention Tips

1. **Always use the Makefile commands** - They handle configuration correctly
2. **Keep .env and .env.example in sync** - Update both when changing config
3. **Use make clean before major changes** - Ensures fresh state
4. **Check logs early** - Don't wait for multiple errors
5. **Use the wrapper scripts** - `./compose.sh` and `./fix-database.sh` handle edge cases