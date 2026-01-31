# GOTRS Secrets Management Guide

## Overview

GOTRS uses a strict secrets management policy to ensure sensitive credentials never appear in code and are properly secured in all environments. All secrets are managed exclusively through environment variables.

## Quick Start

### Generate Secure Configuration

```bash
# First time setup - generates .env with secure random secrets
gotrs synthesize

# Or using make
make synthesize
```

This command:
- Generates cryptographically secure random values for all secrets
- Creates a `.env` file with proper configuration
- Installs git hooks to prevent accidental secret commits
- Backs up existing configuration if present

### Rotate Secrets

```bash
# Rotate only secret values, keeping other settings
gotrs synthesize --rotate-secrets

# Or using make
make rotate-secrets
```

## Architecture

### Single Source of Truth

All secrets are stored in environment variables:
- **Development**: `.env` file (gitignored)
- **Testing**: Centralized test constants with obvious prefixes
- **Production**: Platform secret management (Kubernetes Secrets, AWS Secrets Manager, etc.)

### No Secrets in Code

The codebase contains:
- ❌ No hardcoded credentials
- ❌ No real API keys
- ❌ No production passwords
- ✅ Only references to environment variables
- ✅ Test values with obvious prefixes (`test-`, `mock-`, `dummy-`)

## Secret Generation

### Automatic Generation

The `gotrs synthesize` command generates:

| Secret | Type | Length | Format |
|--------|------|--------|--------|
| JWT_SECRET | Hex | 64 chars | Random hex string |
| SESSION_SECRET | Hex | 48 chars | Random hex string |
| DB_PASSWORD | Mixed | 24 chars | Letters, numbers, symbols |
| API_KEY_INTERNAL | API Key | 32 chars | `gtr-internal-{random}` |
| WEBHOOK_SECRET | Hex | 32 chars | Random hex string |
| ZINC_PASSWORD | Password | 20 chars | Strong password |
| LDAP_BIND_PASSWORD | Password | 16 chars | Strong password |

### Manual Generation

If you need to generate secrets manually:

```bash
# Generate a secure random secret (Linux/Mac)
openssl rand -hex 32

# Generate a strong password
openssl rand -base64 24

# Generate UUID-based API key
uuidgen | tr '[:upper:]' '[:lower:]' | tr -d '-'
```

## Validation

### Startup Validation

GOTRS validates secrets at startup and will:
- **Development**: Warn about weak/default values
- **Production**: Fail to start with insecure secrets

### Validation Rules

1. **JWT_SECRET**: Must be at least 32 characters
2. **Passwords**: Should be at least 12 characters
3. **API Keys**: Should be at least 16 characters
4. **No Default Values**: Rejects common defaults like "changeme", "password", "admin"

## Security Scanning

### Pre-commit Hooks

Automatically installed by `gotrs synthesize`:

```bash
# Manual installation
make scan-secrets-precommit
```

The pre-commit hook:
- Scans for secrets before each commit
- Blocks commits containing credentials
- Uses gitleaks for detection

### CI/CD Scanning

GitHub Actions automatically:
- Scans all commits for secrets
- Runs on PRs and main branch
- Creates security issues if secrets detected
- Uses both Gitleaks and Trivy

### Manual Scanning

```bash
# Scan current code
make scan-secrets

# Scan git history
make scan-secrets-history

# Full security scan
make security-scan
```

## Test Secrets

### Centralized Test Utilities

Use the provided test utilities in `internal/testutil/secrets.go`:

```go
import "github.com/gotrs-io/gotrs-ce/internal/testutil"

func TestSomething(t *testing.T) {
    // Setup test environment with safe test secrets
    testutil.SetupTestEnvironment(t)
    
    // Use predefined test constants
    apiKey := testutil.TestAPIKey
    jwtSecret := testutil.GetTestJWTSecret()
}
```

### Acceptable Test Patterns

Test files may contain:
- Values prefixed with: `test-`, `mock-`, `dummy-`, `example-`
- Obvious fake values: `test-api-key-123`
- Values from `testutil` package

## Production Deployment

### Environment Variable Sources

#### Kubernetes

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gotrs-secrets
type: Opaque
data:
  JWT_SECRET: <base64-encoded-secret>
  DB_PASSWORD: <base64-encoded-password>
```

#### Docker Compose

```yaml
services:
  gotrs:
    env_file:
      - .env.production  # Never commit this file
    environment:
      - JWT_SECRET=${JWT_SECRET}
```

#### AWS Secrets Manager

```bash
aws secretsmanager create-secret \
  --name gotrs/production/jwt-secret \
  --secret-string "$(openssl rand -hex 32)"
```

### Secret Rotation

#### Scheduled Rotation

```bash
# Rotate all secrets quarterly
0 0 1 */3 * gotrs synthesize --rotate-secrets --output .env.production
```

#### Manual Rotation

1. Generate new secrets: `gotrs synthesize --rotate-secrets`
2. Update production secret store
3. Deploy with new secrets
4. Monitor for issues
5. Remove old secrets after verification

## Best Practices

### DO ✅

- Use `gotrs synthesize` for initial setup
- Store production secrets in dedicated secret management systems
- Rotate secrets regularly (quarterly minimum)
- Use different secrets for each environment
- Monitor for exposed secrets in CI/CD
- Use strong, randomly generated values

### DON'T ❌

- Commit `.env` files to version control
- Use default or example values in production
- Share secrets via email, Slack, or tickets
- Log secret values (even in debug mode)
- Hardcode secrets anywhere in code
- Use weak or predictable patterns

## Troubleshooting

### Common Issues

#### "Secret validation failed" on startup

**Cause**: Insecure or default secret detected
**Solution**: Run `gotrs synthesize` to generate secure values

#### Pre-commit hook blocking commits

**Cause**: Potential secret detected in code
**Solution**: 
1. Review the detected pattern
2. If it's a false positive, add to `.gitleaksignore`
3. If it's a real secret, remove it and use environment variable

#### Can't find .env file

**Cause**: File not created yet
**Solution**: Run `gotrs synthesize` to create initial configuration

### False Positives

Add false positives to `.gitleaksignore`:

```
# Format: fingerprint:file:rule:line
abc123def456:docs/example.md:generic-api-key:42
```

## Emergency Response

If secrets are exposed:

1. **Immediate Actions**:
   - Rotate affected secrets immediately
   - Run `gotrs synthesize --rotate-secrets`
   - Deploy new secrets to production

2. **Investigation**:
   - Check access logs for unauthorized use
   - Review git history: `git log -p -S "secret_value"`
   - Scan for other exposures: `make scan-secrets-history`

3. **Remediation**:
   - Remove from git history if needed (BFG Repo-Cleaner)
   - Update `.gitleaksignore` for false positives
   - Review and strengthen secret handling procedures

4. **Prevention**:
   - Ensure pre-commit hooks are installed
   - Enable secret scanning in CI/CD
   - Regular security training for team

## Integration with AGENT_GUIDE.md

The synthesize command respects settings in AGENT_GUIDE.md:
- Follows project-specific secret patterns
- Applies custom validation rules
- Integrates with project workflows

## References

- [NIST Guidelines for Password/Secret Management](https://pages.nist.gov/800-63-3/)
- [OWASP Secrets Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html)
- [12 Factor App - Config](https://12factor.net/config)