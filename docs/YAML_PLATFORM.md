# GoatFlow Unified YAML-as-a-Service Platform

## Overview

The GoatFlow YAML Platform provides enterprise-grade configuration management for all YAML-based configurations in the system. It brings Git-like version control, schema validation, hot reload capabilities, and comprehensive tooling to every configuration file.

## Architecture

```
┌─────────────────────────────────────────────┐
│         Unified YAML Platform               │
├─────────────────────────────────────────────┤
│  Version Control │ Hot Reload │ Validation  │
├─────────────────────────────────────────────┤
│     Version      │   Schema   │   Linter    │
│     Manager      │  Registry  │             │
├─────────────────────────────────────────────┤
│ Routes │ Config │ Dashboards │ Compose │... │
└─────────────────────────────────────────────┘
```

## Core Components

### 1. Version Manager (`internal/yamlmgmt/version_manager.go`)
- Git-like version control for any YAML document
- SHA-based content hashing
- Parent tracking for version history
- Atomic rollback capabilities
- Change tracking and diff generation

### 2. Hot Reload Manager (`internal/yamlmgmt/hot_reload.go`)
- File system watching with fsnotify
- Debounced change detection
- Automatic version creation on changes
- Type-specific reload handlers
- Event notification system

### 3. Schema Registry (`internal/yamlmgmt/schema_registry.go`)
- JSON Schema validation
- Type-specific schema definitions
- Extensible schema registration
- Comprehensive validation reporting

### 4. Universal Linter (`internal/yamlmgmt/linter.go`)
- Best practice enforcement
- Security vulnerability detection
- Performance anti-pattern identification
- Customizable rules per YAML type

### 5. Config Adapter (`internal/yamlmgmt/config_adapter.go`)
- Bridges existing Config.yaml format
- Import/export capabilities
- Setting management functions
- Backward compatibility

### 6. CLI Tool (`cmd/goatflow-config/main.go`)
- Unified interface for all operations
- Container-based execution
- Zero host dependencies

## Supported YAML Types

### Routes
```yaml
apiVersion: goatflow.io/v1
kind: Route
metadata:
  name: api-endpoints
  namespace: core
spec:
  prefix: /api/v1
  routes:
    - path: /health
      method: GET
      handler: healthCheck
```

### Configuration
```yaml
apiVersion: goatflow.io/v1
kind: Config
metadata:
  name: system-config
  version: "1.0"
data:
  settings:
    - name: SystemID
      type: integer
      default: 10
```

### Dashboards
```yaml
apiVersion: goatflow.io/v1
kind: Dashboard
metadata:
  name: admin-dashboard
spec:
  dashboard:
    title: "Admin Dashboard"
    tiles:
      - name: "Users"
        url: "/admin/users"
```

## CLI Usage

### Basic Commands

```bash
# List all configurations
goatflow-config list

# Show specific configuration
goatflow-config show config system-config

# Validate a YAML file
goatflow-config validate config.yaml

# Lint for best practices
goatflow-config lint ./configs/

# View version history
goatflow-config version list config system-config

# Rollback to previous version
goatflow-config rollback config system-config v1.0

# Show differences between versions
goatflow-config diff config system-config v1.0 v1.1

# Apply a configuration
goatflow-config apply new-config.yaml

# Watch for changes (hot reload)
goatflow-config watch

# Export configurations
goatflow-config export config ./backup/

# Import configurations
goatflow-config import ./configs/
```

### Container Usage

```bash
# Run with Docker
docker run --rm -v $(pwd):/app goatflow-config-manager <command>

# Run with Podman
podman run --rm -v $(pwd):/app:Z goatflow-config-manager <command>

# With persistent storage
docker run --rm \
  -v config-data:/app/.versions \
  -v $(pwd):/app \
  goatflow-config-manager <command>
```

## Version Management

### Version Creation
Every configuration change creates a new version automatically:
- SHA-256 hash for content identification
- Timestamp for chronological ordering
- Author tracking from environment
- Parent hash for history chain
- Change description

### Version Format
```
v2025.1.15-1430 (3a2f1b9c)
│    │  │   │     └─ Short hash
│    │  │   └─ Hour and minute
│    │  └─ Day
│    └─ Month
└─ Year
```

### Rollback Safety
- Instant rollback to any previous version
- No data loss - all versions preserved
- Atomic operations prevent partial updates
- Automatic validation before rollback

## Hot Reload

### How It Works
1. File watcher monitors configuration directories
2. Changes trigger debounced reload (500ms default)
3. Validation runs before applying changes
4. New version created automatically
5. Handlers notified of configuration change
6. Services update without restart

### Configuration
```go
hotReload.WatchDirectory("./routes", KindRoute)
hotReload.RegisterHandler(KindRoute, routeReloadHandler)
```

## Validation & Linting

### Schema Validation
- JSON Schema Draft-07 support
- Type-specific schemas
- Custom validation rules
- Detailed error reporting

### Linting Rules

#### Universal Rules
- Naming conventions (kebab-case)
- Required metadata fields
- API version format
- Description presence

#### Type-Specific Rules
- **Routes**: Authentication on admin paths, RESTful conventions
- **Config**: Sensitive data protection, validation rules
- **Dashboards**: Performance limits, color consistency

## Integration

### With Existing Systems

```go
// Import existing Config.yaml
adapter := yamlmgmt.NewConfigAdapter(versionMgr)
err := adapter.ImportConfigYAML("./config/Config.yaml")

// Get configuration value
value, err := adapter.GetConfigValue("SystemID")

// Update configuration
err = adapter.ApplyConfigChanges("SystemID", 20)
```

### With Hot Reload

```go
// Set up hot reload
hotReload, _ := yamlmgmt.NewHotReloadManager(versionMgr, validator)
hotReload.WatchDirectory("./config", yamlmgmt.KindConfig)

// Register reload handler
hotReload.RegisterHandler(yamlmgmt.KindConfig, func(doc *yamlmgmt.YAMLDocument) error {
    // Apply configuration changes
    return applyConfig(doc)
})
```

## Benefits

### Safety
- **Version Control**: Every change tracked with rollback capability
- **Validation**: Schema validation prevents broken configurations
- **Atomic Updates**: All-or-nothing configuration changes
- **Audit Trail**: Complete history of who changed what when

### Performance
- **Hot Reload**: Zero-downtime configuration updates
- **Debouncing**: Prevents reload storms from rapid changes
- **Caching**: Version cache for quick retrieval
- **Efficient Storage**: Content-addressed storage with deduplication

### Developer Experience
- **Unified Interface**: One tool for all configuration types
- **GitOps Ready**: Version control built into the platform
- **Container-First**: Everything runs in containers
- **Extensible**: Easy to add new YAML types

### Operations
- **Rollback Safety**: Instant recovery from bad configurations
- **Change Tracking**: Detailed diff between versions
- **Compliance**: Complete audit trail for regulations
- **Disaster Recovery**: Export/import for backup and restore

## Extending the Platform

### Adding New YAML Types

1. Define the type constant:
```go
const KindMyType YAMLKind = "MyType"
```

2. Register schema:
```go
schemaRegistry.RegisterSchema(KindMyType, &Schema{...})
```

3. Add linting rules:
```go
linter.RegisterRule(KindMyType, LintRule{...})
```

4. Register reload handler:
```go
hotReload.RegisterHandler(KindMyType, myReloadHandler)
```

## Migration Guide

### Phase 1: Preparation
1. Install the config manager container
2. Test with non-critical configurations
3. Set up persistent storage for versions

### Phase 2: Import
1. Import existing routes: `goatflow-config import ./routes`
2. Import system config: `goatflow-config import ./config`
3. Verify imports: `goatflow-config list`

### Phase 3: Integration
1. Enable hot reload: `goatflow-config watch &`
2. Update services to use versioned configs
3. Train team on new tooling

### Phase 4: GitOps
1. Store configurations in Git
2. Set up CI/CD pipelines
3. Automate validation and deployment

## Best Practices

### Configuration Structure
- Use consistent naming (kebab-case)
- Always include descriptions
- Group related settings
- Use semantic versioning

### Version Management
- Create meaningful commit messages
- Review diffs before applying
- Test in staging before production
- Keep version history clean (auto-cleanup after 50 versions)

### Hot Reload
- Validate locally before saving
- Use debouncing for rapid changes
- Monitor reload events
- Have rollback plan ready

### Security
- Mark sensitive settings as readonly
- Use validation rules for inputs
- Audit configuration changes
- Encrypt sensitive values

## Troubleshooting

### Common Issues

**Container can't access files**
```bash
# Use absolute paths or proper volume mounts
docker run -v $(pwd):/app:Z goatflow-config-manager list
```

**Validation failures**
```bash
# Check schema compliance
goatflow-config validate config.yaml
# Fix based on error messages
```

**Hot reload not working**
```bash
# Check file permissions
# Verify watch directory is correct
# Check event logs for errors
```

**Version history lost**
```bash
# Use persistent volume for .versions directory
docker volume create goatflow-config-data
docker run -v goatflow-config-data:/app/.versions ...
```

## Future Enhancements

### Planned Features
- [ ] Web UI for configuration management
- [ ] Kubernetes CRD support
- [ ] Encryption at rest for sensitive configs
- [ ] Multi-environment configuration overlays
- [ ] Configuration templates
- [ ] Webhook notifications on changes
- [ ] Configuration drift detection
- [ ] A/B testing for configurations

### Integration Opportunities
- Temporal workflows for configuration deployment
- Zinc search for configuration discovery
- Grafana dashboards for configuration metrics
- Git integration for full GitOps

## Conclusion

The GoatFlow Unified YAML Platform transforms configuration management from a risky, manual process into a safe, automated, version-controlled system. With hot reload, validation, and comprehensive tooling, it provides enterprise-grade configuration management while maintaining the simplicity of YAML files.

The platform's container-first architecture ensures it works consistently across all environments, from local development to production Kubernetes clusters. By treating configuration as code with full version control, the platform enables GitOps workflows and provides the safety net of instant rollback for any configuration change.