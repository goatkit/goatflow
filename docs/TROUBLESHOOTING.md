# GoatFlow Troubleshooting Guide

> **Reminder:** All operational commands should flow through the Makefile or provided helper scripts. Avoid running raw `docker`/`podman` commands on the host—use the container-first wrappers instead.

## Common Issues and Solutions

### 1. Database Connection Error: "database goatflow_user does not exist"

**Problem:** You see an error like:
```
FATAL: database "goatflow_user" does not exist
```

**Cause:** The `.env` file has incorrect database user configuration that doesn't match the database name.

**Solution:**

1. **Run the fix script:**
   ```bash
   ./scripts/fix-database.sh
   ```

2. **Or manually fix (Makefile workflow):**
   ```bash
   # Stop and clean everything
   make down
   make clean

   # Edit .env file and change:
   # DB_USER=goatflow_user  → DB_USER=goatflow

   # Start fresh
   make up
   ```

3. **Verify the fix:**
   - Check `.env` has `DB_USER=goatflow`
   - Check `.env` has `DB_NAME=goatflow`
   - Both should match for PostgreSQL to initialize correctly

### 2. Compose Command Not Found

**Problem:** You see:
```
make: docker-compose: No such file or directory
```

**Cause:** Your host is missing the Compose plugin/binary that the Makefile expects.

**Solution:**

1. Run `make debug-env` to see which compose variant the tooling detected.
2. Prefer `make up`, `make down`, `make restart`, etc.—they automatically use the correct compose command.
3. If you must run compose manually, call the wrapper: `./scripts/compose.sh <args>`.
4. Install the hinted compose package if `make debug-env` reports it as missing.

### 3. Container Won't Start

**Problem:** Containers fail to start or keep restarting.

**Solution:**

1. **Check logs:**
   ```bash
   make logs
   # Follow everything continuously
   make logs-follow
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
   ./scripts/compose.sh ps
   # All services should show as "running"
   ```

2. **Wait for services to be healthy:**
   ```bash
   ./scripts/compose.sh ps
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
make toolbox-exec ARGS="go mod download"
make toolbox-exec ARGS="go mod tidy"
```

### 8. Frontend Asset Issues

**Problem:** CSS or static assets look stale after changes.

**Solution:**
```bash
# Rebuild Tailwind/CSS assets
make css-build

# For iterative work, run the watcher
make css-watch
```

### 9. Database Migrations Fail

**Problem:** Migrations won't run or fail with errors.

**Solution:**
```bash
# Reset database completely
make clean
make up
make db-migrate

# Or manually via compose wrapper:
./scripts/compose.sh exec backend \
   sh -lc 'PGPASSWORD=$$DB_PASSWORD psql -h $$DB_HOST -U $$DB_USER -d $$DB_NAME < migrations/postgres/000001_schema_alignment.up.sql'
```

### 10. Tests Fail to Run

**Problem:** `make test` doesn't work.

**Solution:**

1. **Ensure testify is installed:**
   ```bash
   make toolbox-exec ARGS="go get github.com/stretchr/testify"
   ```

2. **Run tests directly:**
   ```bash
   make toolbox-exec ARGS="go test -v ./..."
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
./scripts/compose.sh ps

# Check logs
make logs

# Shell into container
./scripts/compose.sh exec backend sh
# For database access (MariaDB default)
./scripts/compose.sh exec mariadb mysql -u $$DB_USER -p$$DB_PASSWORD $$DB_NAME
```

### Clean Slate Reset
```bash
# Complete cleanup and restart
./scripts/fix-database.sh
make clean
make setup
make up
```

### Environment Verification
```bash
# Check .env values
grep DB_ .env

# Verify compose file is valid
./scripts/compose.sh config

# Test database connection
./scripts/compose.sh exec mariadb mysqladmin ping -u $$DB_USER -p$$DB_PASSWORD || true
# If you're running Postgres instead of MariaDB
./scripts/compose.sh exec postgres pg_isready -U $$DB_USER || true
```

## Getting Help

If you continue to experience issues:

1. Check the logs: `make logs`
2. Review your `.env` file configuration
3. Ensure Docker/Podman daemon is running
4. Check the [GitHub Issues](https://github.com/goatkit/goatflow/issues)
5. Run `make debug-env` to see your environment setup

## Prevention Tips

1. **Always use the Makefile commands** - They handle configuration correctly
2. **Keep .env and .env.example in sync** - Update both when changing config
3. **Use make clean before major changes** - Ensures fresh state
4. **Check logs early** - Don't wait for multiple errors
5. **Use the wrapper scripts** - `./scripts/compose.sh` and `./scripts/fix-database.sh` handle edge cases