# Database Safety Documentation

## Overview

GoatFlow implements multiple layers of safety to prevent accidental data loss, especially in production environments. This document explains the safety mechanisms in place.

## Database Structure

When you run `make up` or `docker compose up`, the following databases are created:

1. **Main Database** (`goatflow`) - For development work
2. **Test Database** (`goatflow_test`) - For running tests (dev/test environments only)
3. **User Database** (matches DB_USER) - Prevents "database does not exist" errors

## Safety Mechanisms

### 1. Environment-Based Database Creation

The PostgreSQL init script (`docker/postgres/01-init-databases.sql`) checks the `APP_ENV` variable:

- **Development/Test** (`APP_ENV=development` or `APP_ENV=test`):
  - Creates main database
  - Creates test database with `_test` suffix
  - Creates user-named database

- **Production** (`APP_ENV=production`):
  - Creates main database only
  - **NEVER** creates test database
  - Skips test-related initialization

### 2. Test Execution Safety

When running tests with `make test`:

1. **Automatic Test Database Selection**:
   - Tests automatically use `${DB_NAME}_test` database
   - Never touches the main development database
   - Example: If `DB_NAME=goatflow`, tests use `goatflow_test`

2. **Environment Override**:
   - `APP_ENV` is set to `test` during test execution
   - Prevents accidental production testing

3. **Safe Test Script** (`scripts/test-db-safe.sh`):
   - Verifies environment is not production
   - Ensures database name ends with `_test`
   - Only allows localhost connections
   - Confirms before cleaning test data

### 3. Production Safety

The `docker-compose.prod.yml` file provides additional production safeguards:

- Forces `APP_ENV=production`
- Removes init script mounting
- Disables development tools (mailhog, adminer)
- Uses specific production data directories

### 4. Username Flexibility

The system handles ANY username gracefully:

- `DB_USER=goatflow` ✅
- `DB_USER=goatflow_user` ✅
- `DB_USER=admin` ✅
- `DB_USER=custom_name` ✅

No more "FATAL: database 'username' does not exist" errors!

## Safety Commands

### Check Safety Status
```bash
make debug-env              # Check environment configuration
make test-safe              # Run tests with all safety checks
./scripts/test-db-safe.sh check  # Verify safety without running tests
```

### Test Database Management
```bash
make test                   # Run tests (uses test database automatically)
make test-clean            # Clean test database (with confirmation)
```

### Production Deployment
```bash
# Use production compose file
docker compose -f docker-compose.yml -f docker-compose.prod.yml up

# Or set environment
APP_ENV=production docker compose up
```

## Safety Checklist

Before running tests:
- ✅ `APP_ENV` is not `production`
- ✅ Database name contains `_test`
- ✅ Database host is localhost/local container
- ✅ Not connected to remote database

Before deploying to production:
- ✅ Set `APP_ENV=production`
- ✅ Use `docker-compose.prod.yml`
- ✅ Remove test databases
- ✅ Disable development tools

## Environment Variables

### Critical Safety Variables

| Variable | Development | Test | Production |
|----------|------------|------|------------|
| `APP_ENV` | `development` | `test` | `production` |
| `DB_NAME` | `goatflow` | `goatflow_test` | `goatflow` |
| `GIN_MODE` | `debug` | `test` | `release` |

### Database Configuration

| Variable | Description | Example | Safety Impact |
|----------|-------------|---------|---------------|
| `DB_NAME` | Main database name | `goatflow` | Tests use `${DB_NAME}_test` |
| `DB_USER` | Database username | `goatflow_user` | Any value works |
| `DB_HOST` | Database hostname | `postgres` | Tests restricted to localhost |
| `APP_ENV` | Environment mode | `development` | Controls test DB creation |

## Common Scenarios

### Scenario 1: Fresh Development Setup
```bash
cp .env.example .env
make up                    # Creates: goatflow, goatflow_test, goatflow_user
make test                  # Uses: goatflow_test only
```

### Scenario 2: Custom Username
```bash
# .env
DB_USER=my_custom_user
DB_NAME=myapp

make up                    # Creates: myapp, myapp_test, my_custom_user
make test                  # Uses: myapp_test only
```

### Scenario 3: Production Deployment
```bash
APP_ENV=production docker compose up
# Only creates main database, no test database
# Test commands will fail with safety error
```

## Troubleshooting

### "database does not exist" Error
**Solution**: The init script now handles this automatically by creating a database matching the username.

### Tests Affecting Development Data
**Solution**: Tests automatically use `_test` database. Check that `make test` shows "Using test database: goatflow_test"

### Can't Run Tests in Production
**Solution**: This is intentional! Tests should never run against production data. Use a staging environment for production-like testing.

### Test Database Not Created
**Check**:
1. `APP_ENV` is not set to `production`
2. PostgreSQL container has the init script mounted
3. Clear volumes if needed: `make clean`

## Best Practices

1. **Always use make commands** - They include safety checks
2. **Never manually set DB_NAME to production values during tests**
3. **Use docker-compose.prod.yml for production deployments**
4. **Keep test data separate from development data**
5. **Run `make test-clean` periodically to reset test database**

## Safety Architecture

```
┌─────────────────────────────────────┐
│         User Configuration          │
│         (any DB_USER value)         │
└────────────┬────────────────────────┘
             │
             ▼
┌─────────────────────────────────────┐
│    PostgreSQL Init Script           │
│  (01-init-databases.sql)            │
├─────────────────────────────────────┤
│ • Creates user-named database       │
│ • Creates main database (DB_NAME)   │
│ • Creates test database if not prod │
└────────────┬────────────────────────┘
             │
    ┌────────┴────────┐
    │                 │
    ▼                 ▼
┌──────────┐    ┌──────────┐
│   Dev    │    │   Test   │
│  goatflow   │    │goatflow_test│
└──────────┘    └──────────┘
    │                 │
    │                 │
    ▼                 ▼
Development        Testing
   Work            Only
```

## Summary

The safety system ensures:
- ✅ Any username works without errors
- ✅ Test data is isolated from development data
- ✅ Production data cannot be accidentally deleted by tests
- ✅ Clear separation between environments
- ✅ Automatic safety checks before dangerous operations

This multi-layered approach prevents data loss while maintaining flexibility for developers.