# Configuration Management

GOTRS uses a layered YAML configuration system with Viper for flexible configuration management with hot reload support.

## Configuration Hierarchy

Configuration is loaded in the following order (later sources override earlier ones):

1. **default.yaml** - Base configuration with all default values (committed to repo)
2. **config.yaml** - Local environment overrides (gitignored)
3. **Environment variables** - Runtime overrides with `GOTRS_` prefix
4. **Command-line flags** - Highest priority (when implemented)

## File Structure

```
config/
├── default.yaml           # Default configuration (version controlled)
├── config.yaml.example    # Example local override (version controlled)
└── config.yaml           # Local overrides (gitignored)
```

## Configuration Files

### default.yaml
Contains all configuration options with sensible defaults. This file is committed to version control and serves as the single source of truth for available configuration options.

### config.yaml
Local override file for development. Copy `config.yaml.example` to `config.yaml` and modify as needed. This file is gitignored to prevent committing secrets.

## Environment Variables

All configuration values can be overridden using environment variables with the `GOTRS_` prefix. Nested keys use underscores:

```bash
# Override app.debug
export GOTRS_APP_DEBUG=false

# Override database.host
export GOTRS_DATABASE_HOST=postgres.example.com

# Override auth.jwt.secret
export GOTRS_AUTH_JWT_SECRET="production-secret-key"
```

## Hot Reload

The configuration system supports hot reload without restarting the application:

1. Edit `config.yaml` or `default.yaml`
2. The application detects changes automatically
3. New configuration is loaded and applied
4. Console logs confirm successful reload

**Note**: Some configuration changes (like port binding) require a restart.

## Usage in Code

```go
import "github.com/gotrs-io/gotrs-ce/internal/config"

// Load configuration on startup
config.MustLoad("./config")

// Get configuration anywhere in the app
cfg := config.Get()

// Access specific values
dbHost := cfg.Database.Host
jwtSecret := cfg.Auth.JWT.Secret
isDebug := cfg.App.Debug
```

## Configuration Sections

### App Configuration
- Application metadata (name, version)
- Environment (development, staging, production)
- Debug mode toggle
- Timezone settings

### Server Configuration
- Host and port binding
- Timeout settings
- CORS configuration

### Database Configuration
- PostgreSQL connection settings
- Connection pool configuration
- Migration settings

### Redis Configuration
- Connection settings
- Session storage configuration
- Cache settings

### Authentication
- JWT token configuration
- Session settings
- Password requirements

### Email Configuration
- SMTP settings
- Template paths
- Queue configuration

### Storage Configuration
- Local or S3 storage
- Attachment settings
- File size limits

### Ticket System
- ID generation format
- Default values
- SLA settings
- Notification preferences

### Logging
- Log levels and formats
- Output destinations
- File rotation settings

### Metrics & Monitoring
- Prometheus endpoint
- OpenTelemetry configuration

### Rate Limiting
- Request limits
- Burst settings
- Path exclusions

### Feature Flags
- Enable/disable features
- Integration toggles

## Kubernetes/OpenShift Integration

The YAML configuration integrates seamlessly with Kubernetes:

### ConfigMaps
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gotrs-config
data:
  config.yaml: |
    app:
      env: production
      debug: false
    database:
      host: postgres-service
```

### Secrets
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gotrs-secrets
stringData:
  GOTRS_DATABASE_PASSWORD: "secure-password"
  GOTRS_AUTH_JWT_SECRET: "production-secret"
```

### Deployment
```yaml
spec:
  containers:
  - name: gotrs
    envFrom:
    - secretRef:
        name: gotrs-secrets
    volumeMounts:
    - name: config
      mountPath: /app/config
  volumes:
  - name: config
    configMap:
      name: gotrs-config
```

## Security Best Practices

1. **Never commit secrets** - Use config.yaml or environment variables
2. **Use strong JWT secrets** - Generate with `openssl rand -base64 32`
3. **Rotate secrets regularly** - Especially in production
4. **Limit config file permissions** - chmod 600 for config.yaml
5. **Use Kubernetes Secrets** - For production deployments

## Development Tips

1. **Copy the example file**: `cp config/config.yaml.example config/config.yaml`
2. **Use meaningful overrides**: Only override what you need
3. **Test hot reload**: Edit config.yaml and watch the console
4. **Environment-specific files**: Consider config.dev.yaml, config.prod.yaml patterns
5. **Validate configuration**: Implement validation in config.go

## Troubleshooting

### Configuration not loading
- Check file paths and permissions
- Verify YAML syntax with a linter
- Check console for error messages

### Environment variables not working
- Ensure `GOTRS_` prefix is used
- Use underscores for nested keys
- Check for typos in variable names

### Hot reload not working
- Verify fsnotify is working on your OS
- Check file system events aren't blocked
- Ensure config files aren't symlinks

## Example: Local Development Setup

```yaml
# config/config.yaml (local overrides)
app:
  debug: true
  env: development

database:
  host: localhost
  password: localpassword

redis:
  host: localhost

email:
  enabled: false  # Disable for local dev

auth:
  jwt:
    secret: "local-development-secret-key"
  session:
    secure: false  # Allow HTTP in development

logging:
  level: debug
  format: text  # Human-readable logs
```

## Example: Production Setup

```bash
# Production environment variables
export GOTRS_APP_ENV=production
export GOTRS_APP_DEBUG=false
export GOTRS_DATABASE_HOST=postgres.internal
export GOTRS_DATABASE_PASSWORD="${DB_PASSWORD}"
export GOTRS_REDIS_HOST=redis.internal
export GOTRS_REDIS_PASSWORD="${REDIS_PASSWORD}"
export GOTRS_AUTH_JWT_SECRET="${JWT_SECRET}"
export GOTRS_AUTH_SESSION_SECURE=true
export GOTRS_EMAIL_SMTP_HOST=smtp.sendgrid.net
export GOTRS_EMAIL_SMTP_USER=apikey
export GOTRS_EMAIL_SMTP_PASSWORD="${SENDGRID_API_KEY}"
```