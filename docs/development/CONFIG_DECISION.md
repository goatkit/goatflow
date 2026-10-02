# Configuration Management Decision

## Decision Date: 2025-08-21

## Decision
- **Configuration Format**: YAML
- **Configuration Library**: [Viper](https://github.com/spf13/viper)

## Rationale
- YAML is human-readable and widely used for configuration
- Viper provides:
  - Environment variable override support
  - Config file watching for changes
  - Default values
  - Multiple config file formats (if we need to migrate)
  - Nested configuration support
  - Type safety

## Implementation Status
**IMPLEMENTED** in `internal/platform/config/config.go`: typed config structs, defaults in
`config/default.yaml`, an optional `config/config.yaml` override, `GOATFLOW_*` environment
overrides and file watching. See [CONFIGURATION.md](CONFIGURATION.md).