# Security Scanning Guide

## Overview

GoatFlow implements automated security scanning to prevent accidental exposure of secrets, credentials, and sensitive information. We use industry-standard tools that run at multiple stages of the development lifecycle.

## Tools Used

### 1. Gitleaks
- **Purpose**: Detect and prevent secrets in git repos
- **When it runs**: Pre-commit, CI/CD, manual scans
- **Configuration**: `gitleaks.toml`
- **Docker image**: `zricethezav/gitleaks:latest`

### 2. Trivy
- **Purpose**: Vulnerability scanning and secret detection
- **When it runs**: CI/CD, manual scans
- **Configuration**: Inline in GitHub Actions
- **Docker image**: `aquasec/trivy:latest`

### 3. Git Pre-commit Hooks
- **Purpose**: Prevent secrets from being committed
- **Installation**: Bash script (no Python/pip required)
- **Configuration**: `gitleaks.toml`
- **Additional checks**: Large files, sensitive file patterns

## Quick Start

### Install Pre-commit Hooks (Recommended)

```bash
# Install pre-commit hooks (no Python/pip required)
make scan-secrets-precommit

# Or manually run the script:
bash scripts/install-git-hooks.sh

# Test the hook without committing:
.git/hooks/pre-commit
```

This will automatically scan for secrets before every commit using Docker/Podman.

### Manual Scanning

```bash
# Scan current code for secrets
make scan-secrets

# Scan entire git history
make scan-secrets-history

# Run all security scans (secrets + vulnerabilities)
make security-scan

# Scan for vulnerabilities only
make scan-vulnerabilities
```

## CI/CD Integration

### GitHub Actions

Our GitHub Actions workflow (`.github/workflows/gitleaks.yml`) runs on:
- Every push to main/develop branches
- Every pull request
- Weekly scheduled scans
- Manual workflow dispatch

The workflow:
1. Scans for secrets with Gitleaks
2. Scans for secrets and vulnerabilities with Trivy
3. Uploads results as artifacts
4. Creates GitHub issues if secrets are detected
5. Comments on PRs with findings

### GitLab CI

To integrate with GitLab CI, add to `.gitlab-ci.yml`:

```yaml
secret-scanning:
  stage: test
  image:
    name: zricethezav/gitleaks
    entrypoint: [""]
  script:
    - gitleaks detect --source . --verbose --report-path gitleaks-report.json
  artifacts:
    reports:
      secret_detection: gitleaks-report.json
```

## What Gets Scanned

### Included
- All source code files
- Configuration files
- Documentation
- Git history (when using `scan-secrets-history`)
- Container images
- Dependencies

### Excluded (via `gitleaks.toml`)
- Test files (`*_test.go`, `test/`, `tests/`)
- Migration files with bcrypt hashes
- Example environment files (`.env.example`, `.env.development`)
- Package lock files
- CI/CD configuration files

## If Secrets Are Detected

### During Pre-commit

If secrets are detected during pre-commit:

1. The commit will be blocked
2. Review the output to identify the issue
3. Remove or replace the secret with an environment variable
4. If it's a false positive, add it to `gitleaks.toml` allowlist

### In CI/CD

If secrets are detected in CI/CD:

1. The build will fail
2. A GitHub issue will be created automatically
3. Review the scan artifacts for details
4. Take immediate action (see below)

### Immediate Actions

1. **Rotate the credential immediately**
   - Even if it's in a branch or old commit
   - Assume the credential is compromised

2. **Remove from repository**
   ```bash
   # Remove from current branch
   git rm <file-with-secret>
   git commit -m "Remove exposed credential"
   
   # Remove from history (use with caution)
   git filter-branch --force --index-filter \
     "git rm --cached --ignore-unmatch <file-with-secret>" \
     --prune-empty --tag-name-filter cat -- --all
   ```

3. **Update code to use environment variables**
   ```go
   // Bad
   apiKey := "example-key-do-not-use"
   
   // Good
   apiKey := os.Getenv("API_KEY")
   ```

4. **Document the incident**
   - What was exposed
   - When it was rotated
   - What systems might be affected

## False Positives

### Inline Allowlist

For one-time exceptions, add a comment:

```go
// gitleaks:allow
testAPIKey := "example-test-key-do-not-use" // This is a test key
```

### Global Allowlist

For patterns that should always be allowed, update `gitleaks.toml`:

```toml
[[rules]]
id = "my-custom-rule"
description = "Allow test patterns"
regex = '''test_[a-z]+_key'''
allowlist = true
```

### .gitleaksignore

Create a `.gitleaksignore` file for specific findings:

```
abc123def:path/to/file.go:api-key:10
```

Format: `<commit>:<file>:<rule>:<line>`

## Best Practices

### 1. Never Commit Secrets

- Always use environment variables
- Use `.env` files (never commit them)
- Use secret management tools (Vault, AWS Secrets Manager)

### 2. Environment Variables

```bash
# .env (local development only)
DB_PASSWORD=localtestpassword
API_KEY=sk-development-key

# Production (use secret manager)
kubectl create secret generic app-secrets \
  --from-literal=db-password=$DB_PASSWORD \
  --from-literal=api-key=$API_KEY
```

### 3. Configuration Files

```yaml
# config.yaml
database:
  host: localhost
  port: 5432
  username: ${DB_USER}
  password: ${DB_PASSWORD}  # From environment
```

### 4. Docker Secrets

```dockerfile
# Bad
ENV API_KEY=example-key-do-not-use

# Good
ENV API_KEY=${API_KEY}

# Better (runtime)
# Pass via docker run -e API_KEY=$API_KEY
```

### 5. Git Configuration

```bash
# Prevent accidental commits of .env files
echo ".env*" >> .gitignore
echo "!.env.example" >> .gitignore
echo "!.env.development" >> .gitignore

# Set up pre-commit hooks
make scan-secrets-precommit
```

## Monitoring and Compliance

### Regular Scans

```bash
# Schedule weekly scans via cron
0 0 * * 0 cd /path/to/goatflow && make security-scan
```

### Audit Trail

All scan results are:
- Logged in CI/CD
- Saved as artifacts for 30 days
- Tracked in GitHub Security tab (SARIF format)

### Metrics

Track:
- Number of secrets detected per month
- Time to remediation
- False positive rate
- Scan coverage percentage

## Additional Resources

- [Gitleaks Documentation](https://github.com/gitleaks/gitleaks)
- [Trivy Documentation](https://aquasecurity.github.io/trivy/)
- [GitHub Secret Scanning](https://docs.github.com/en/code-security/secret-scanning)
- [OWASP Secrets Management](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html)

## Support

If you encounter issues with secret scanning:

1. Check the scan output for specific errors
2. Review `gitleaks.toml` configuration
3. Consult the troubleshooting section
4. Open an issue with the `security` label

Remember: **It's better to have false positives than to miss real secrets!**