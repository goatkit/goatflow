# Configuration (for developers)

This page is about reading configuration in Go code. The list of settings, and which ones
GoatFlow 0.10.0 really reads, is in [docs/configuration.md](../configuration.md). Use that page
for any setting name or default.

## Where the code lives

| Path | What it is |
|------|------------|
| `internal/platform/config/config.go` | The `Config` struct, `Load`, `MustLoad`, `Get`, `LoadFromFile` |
| `config/default.yaml` | Defaults for every key in `Config` (shipped with the image) |
| `config/config.yaml.example` | Example local override file |
| `config/Config.yaml` | The SysConfig registry (a different system, see [docs/configuration.md](../configuration.md)) |
| `internal/platform/dbconfig/env.go` | Database connection variables (`DB_DRIVER`, `DB_MYSQL_*`, `DB_PGSQL_*`) |

## How loading works

`config.Load(configDir)` runs once per process (it uses `sync.Once`):

1. Read `default.yaml` from `configDir`.
2. Merge `config.yaml` from the same directory, if it exists.
3. Apply environment variables with the prefix `GOATFLOW_`. The variable name is the YAML path in
   upper case with `.` replaced by `_`. Example: `email.smtp.host` is `GOATFLOW_EMAIL_SMTP_HOST`.
4. Unmarshal into `Config`.
5. Watch the config files. When one changes, the whole `Config` is unmarshalled again and
   swapped in. Code that kept an old `*Config` keeps the old values.

`cmd/goats/main.go` calls `config.Load` with `CONFIG_DIR`, or `/app/config` when `CONFIG_DIR`
is unset. If loading fails it logs a warning and carries on.

There are no command-line flags for configuration.

## Using it in code

```go
import "github.com/goatkit/goatflow/internal/platform/config"

cfg := config.Get() // nil until config.Load has run
if cfg != nil && cfg.Features.Registration {
    // ...
}
```

- `config.Get()` returns `nil` when `Load` has not run (for example in many unit tests). Check
  for `nil`.
- In tests, `config.LoadFromFile(path)` loads one YAML file. It does not apply `GOATFLOW_*`
  variables and does not watch the file.

## Values that do not come from `Config`

Some values are not in `Config` at all, or are in it only as a fallback. Do not read these from
`Config` in new code:

| Value | Read it from | Not from |
|-------|--------------|----------|
| Database connection | `dbconfig.Env("HOST")` etc. (`DB_MYSQL_*` or `DB_PGSQL_*` by `DB_DRIVER`, then flat `DB_*`) | `Config` has no database section |
| JWT signing key | `JWT_SECRET` (`internal/platform/shared/jwt_manager.go`; `cfg.Auth.JWT.Secret` is only a fallback) | `cfg.Auth.JWT.Secret` alone |
| Listen port | `APP_PORT` | `cfg.Server.Port` |
| Logging, metrics | `LOG_*`, `METRICS_*` variables (`internal/platform/logging`, `cmd/goats`) | `Config` has no logging or metrics section |

The full list is the "`default.yaml` sections" table in
[docs/configuration.md](../configuration.md#defaultyaml-sections).

## Adding a setting

1. Add the field to the right struct in `internal/platform/config/config.go`, with a
   `mapstructure:"..."` tag.
2. Add the key and its default to `config/default.yaml`. A key that is missing from the YAML
   files cannot be set with a `GOATFLOW_*` variable.
3. Read it with `config.Get()` and handle `nil`.
4. Document it in [docs/configuration.md](../configuration.md).

Never put a secret in `default.yaml`. Secrets come from environment variables.
