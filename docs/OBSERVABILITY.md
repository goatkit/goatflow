# Observability

GoatFlow 0.10.0 has health check endpoints, Prometheus metrics, structured logs and graceful shutdown.
This page covers how each one works, how to set it up with environment variables, and which deployment files use them.

## Environment variables

| Variable | Default | Accepted values | What it does |
|----------|---------|-----------------|--------------|
| `LOG_FORMAT` | `text` | `json`, `text` | Log line format. Unknown values fall back to `text` and print a warning to stderr. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` (or `warning`), `error` | Lowest level written for `slog` lines. Unknown values fall back to `info`. |
| `LOG_OUTPUT` | `stdout` | `stdout`, `stderr`, or a file path | Where logs are written. |
| `LOG_FILE_PATH` | not set | a file path | Old name for the log file. Used only when `LOG_OUTPUT` is empty. |
| `GOATFLOW_PLUGIN_LOG_ECHO` | off | `1`, `true`, `yes`, `on` | Also writes plugin log calls as plain log lines (see [Plugin logs](#plugin-logs)). |
| `METRICS_ENABLED` | off | `true` (any case) or `1` | Starts a separate metrics listener. |
| `METRICS_PORT` | `9090` | a port number | Port for the separate metrics listener. |
| `DRAIN_TIMEOUT` | `5s` | a positive Go duration, for example `5s`, `30s`, `1m` | How long shutdown waits for open HTTP requests (server) or cancelled tasks (runner). |
| `APP_PORT` | `8080` | a port number | Main HTTP port. `/health` and `/metrics` are served here. |
| `CUSTOMER_FE_ONLY` | off | `true`, `1`, `yes` or `on` (any case) | Customer-only instance. Changes which paths answer (see [Customer-only instances](#customer-only-instances)). |

Notes:

- These are read from the environment only. `config/default.yaml` has no logging or metrics section.
- `.env.example` sets `LOG_FORMAT=json`, `LOG_OUTPUT=stdout` and `METRICS_ENABLED=true`. These differ from the code defaults above. If you copy that file, you get JSON logs and the metrics listener.

## Health checks

### Endpoints

| Path | Who can call it | What it checks |
|------|-----------------|----------------|
| `GET /health` | Anyone. No login. | Database ping. |
| `GET /healthz` | Anyone. No login. | Same handler as `/health`. |
| `GET /health/detailed` | Logged-in admins only. | Database ping, Valkey cache, version, uptime. |

### `GET /health`

GoatFlow pings the database with a 500 ms timeout.

| Database ping | HTTP status | `status` field |
|---------------|-------------|----------------|
| Works | `200` | `healthy` |
| Fails or times out | `503` | `unhealthy` |

Response when healthy:

```json
{
  "components": { "database": "ok" },
  "status": "healthy",
  "version": "0.10.0"
}
```

Response when the database is down (HTTP 503):

```json
{
  "components": { "database": "error" },
  "status": "unhealthy",
  "version": "0.10.0"
}
```

`version` is the plain release number. It has no git commit.

### `GET /health/detailed`

This is for operators, not for probes. The route uses the `auth` and `admin` middleware.

| Caller | Result |
|--------|--------|
| Not logged in, browser | `303` redirect to `/login` |
| Not logged in, API client | `401` `{"error":"Missing authorization token"}` |
| API token (`gf_...`) | `401` `{"error":"API tokens are not accepted on this route"}` |
| Logged in, not an admin | `403` `{"error":"Admin access required"}` |
| Logged in admin | Health report |

The health report:

```json
{
  "components": { "cache": "ok", "database": "ok" },
  "status": "healthy",
  "uptime": "3h12m5.402118s",
  "version": "0.10.0"
}
```

| Field | Values | Meaning |
|-------|--------|---------|
| `components.database` | `ok`, `error` | Database ping, 500 ms timeout. |
| `components.cache` | `ok`, `error`, `disabled` | Valkey `INFO` call, 2 s timeout. `disabled` means no Valkey cache was set up at startup. |
| `status` | `healthy`, `unhealthy` | `unhealthy` only when the database fails. A cache error does not change it. |
| `uptime` | Go duration text | Time since the process started. |
| `version` | release number | Same as `/health`. |

HTTP status is `200`, or `503` when the database fails.

### Which probes use which endpoint

All probes use the public `/health`. None use `/health/detailed`.

| Where | File | Check |
|-------|------|-------|
| Docker image | `Dockerfile` (`runtime` stage) | `wget --spider http://localhost:8080/health` every 30s, timeout 3s, start period 5s, 3 retries. |
| Docker Compose (dev) customer-fe | `docker/frontend/healthcheck.sh` | `wget --spider http://localhost:${APP_PORT:-8080}/health` |
| TrueNAS app, main container | `docs/truenas-app/goatflow/templates/docker-compose.yaml` | `wget` on `/health`, start period 30s, 5 retries. |
| TrueNAS app, customer FE container | same file | `wget` on `/health`, start period 30s, 5 retries. |
| Helm chart backend | `charts/goatflow/values.yaml` (`backend.probes`) | Liveness and readiness both `GET /health` on port `http` (8080). |

Things to know:

- The `Dockerfile` check uses port `8080`. If you change `APP_PORT`, override the health check too.
- Because `/health` pings the database, a backend with a dead database fails both liveness and readiness in the Helm chart. Kubernetes will restart it.
- The Helm chart's frontend (nginx) pod has its own `/health`. It returns a fixed `healthy` text and does not call the backend.
- The runner has no HTTP server. The TrueNAS template checks `pidof goats`. The dev `docker-compose.yml` checks that PID 1 is `./goats -mode runner`.

Default Helm probe timings (`charts/goatflow/values.yaml`):

| Probe | `initialDelaySeconds` | `periodSeconds` | `timeoutSeconds` |
|-------|-----------------------|-----------------|------------------|
| liveness | 30 | 10 | 5 |
| readiness | 5 | 5 | 3 |

## Metrics

GoatFlow serves Prometheus metrics in two places. Both serve the same data from the Prometheus default registry.

| Where | Login needed | Turned on by |
|-------|--------------|--------------|
| `GET /metrics` on the app port (`APP_PORT`, default 8080) | Yes. Logged-in admin. API tokens get `401`. | Always on. |
| Separate listener on `METRICS_PORT` (default 9090) | No. | `METRICS_ENABLED=true` or `METRICS_ENABLED=1` |

### Separate metrics listener

Use this for Prometheus. Prometheus cannot log in to the app port, and API tokens are refused there.

- It listens on all network interfaces (`:9090`), not only localhost.
- It has no authentication.
- It answers on every path. Use `/metrics`.
- If the port is already in use, GoatFlow logs a warning and keeps running without it.
- Startup log line: `Prometheus metrics exposed on :9090 (METRICS_ENABLED)`

> **Security:** anyone who can reach this port can read the metrics. Keep it on an internal network. Do not publish it to the internet or map it to a host port that is reachable from outside.

None of the shipped deployment files turn it on:

- `deploy/docker-compose.yml` does not set `METRICS_ENABLED` and only exposes port 8080.
- The Helm chart has no metrics settings. It has no metrics container port, no metrics Service port and no ServiceMonitor. The backend Service only has port `8080`.

To use it with Helm, set the variable with `backend.extraEnv`. `values.yaml` shows Prometheus pod annotations as a commented example under `backend.podAnnotations`:

```yaml
backend:
  extraEnv:
    - name: METRICS_ENABLED
      value: "true"
  podAnnotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9090"
```

The annotations only help if your Prometheus is set up to discover pods by annotation.

### Example Prometheus scrape config

For `deploy/docker-compose.yml`, where the main app service is called `app`, after you add `METRICS_ENABLED: "true"` to it:

```yaml
scrape_configs:
  - job_name: goatflow
    metrics_path: /metrics
    static_configs:
      - targets: ["app:9090"]
```

Prometheus must be on the same Docker network as `app`.

### Metric names

GoatFlow metrics:

| Metric | Type | When it appears |
|--------|------|-----------------|
| `goatflow_up` | gauge | Always. Value is `1`. |
| `goatflow_process_start_time_seconds` | gauge | Always. Unix time the process started. |
| `cache_hits_total` | counter | When a Valkey cache is set up. |
| `cache_misses_total` | counter | When a Valkey cache is set up. |
| `cache_errors_total` | counter | When a Valkey cache is set up. |
| `cache_sets_total` | counter | When a Valkey cache is set up. |
| `cache_deletes_total` | counter | When a Valkey cache is set up. |
| `cache_operation_duration_seconds` | histogram | When a Valkey cache is set up. |
| `goatflow_scheduler_email_poll_runs_total` | counter | When the scheduler runs (main app only). |
| `goatflow_scheduler_email_poll_active_accounts` | gauge | When the scheduler runs. |
| `goatflow_scheduler_email_poll_accounts_total` | counter, labels `status` (`success`, `failure`) and `connector` | When the scheduler runs. |
| `goatflow_scheduler_email_poll_duration_seconds` | histogram | When the scheduler runs. |

The scheduler does not run on customer-only instances or when the database is unavailable at startup. With several backend replicas, each job tick runs on one replica only (see [HIGH_AVAILABILITY.md](HIGH_AVAILABILITY.md#things-to-know-before-running-more-than-one-replica)), so the email poll metrics of one replica cover only the ticks it ran. Sum them across replicas.

The default registry also includes the standard collectors from the Prometheus Go client:

- `go_*`: Go runtime, for example `go_goroutines`, `go_gc_duration_seconds`, `go_info`.
- `process_*`: process stats, for example `process_cpu_seconds_total`, `process_resident_memory_bytes`, `process_start_time_seconds`.
- `promhttp_metric_handler_requests_total` and `promhttp_metric_handler_requests_in_flight`: requests to the metrics handler itself.

### Runner

The runner (`goats -mode runner`) exposes no metrics. It returns before any HTTP server or metrics listener starts, so `METRICS_ENABLED` has no effect on it.

## Customer-only instances

An instance with `CUSTOMER_FE_ONLY=true` answers only an allowlist of paths (see [docs/CUSTOMER_PORTAL.md](CUSTOMER_PORTAL.md)). Other paths get `404`.

| Path | On a customer-only instance |
|------|-----------------------------|
| `/health`, `/healthz` | Work as normal. |
| `/metrics` on the app port | `404`. |
| `/health/detailed` | `404`. Only the exact `/health` and `/healthz` paths are allowed. |
| Separate listener on `METRICS_PORT` | Works if `METRICS_ENABLED` is set. It is a separate server and the allowlist does not apply to it. |

## Logging

Logging is set up first thing at startup, from `LOG_FORMAT`, `LOG_LEVEL` and `LOG_OUTPUT`.

### Two kinds of log line

GoatFlow code writes logs in two ways. Both go to the same place and use the same format.

| Kind | Used by | Filtered by `LOG_LEVEL`? |
|------|---------|--------------------------|
| `slog` lines | Newer code, for example plugin health, plugin log calls and the runner. These have a level and extra fields. | Yes. |
| Standard `log` lines | Most of the code base (`log.Printf`). These have no level; they are written as `INFO`. | No. Always written. |

So `LOG_LEVEL=warn` hides `slog` info and debug lines only. Standard `log` lines still appear.

### `LOG_FORMAT=json`

Both kinds use the Go `slog` JSON layout: `time` (RFC 3339 with nanoseconds), `level` (upper case: `DEBUG`, `INFO`, `WARN`, `ERROR`), `msg`, then any extra fields.

```json
{"time":"2026-10-02T14:40:02.192173466Z","level":"WARN","msg":"auto-restarting unhealthy plugin","plugin":"example","attempt":1}
```

Standard `log` lines have level `INFO` and no extra fields:

```json
{"time":"2026-10-02T14:40:02.192234316Z","level":"INFO","msg":"Shutting down: received terminated — draining connections (timeout 5s)"}
```

### `LOG_FORMAT=text` (default)

```text
time=2026-10-02T14:40:02.192Z level=WARN msg="auto-restarting unhealthy plugin" plugin=example attempt=1
2026/10/02 14:40:02 Shutting down: received terminated — draining connections (timeout 5s)
```

Runner lines are `slog` lines with `component=runner`, for example `level=INFO msg="executing task" component=runner task=email_queue`.

### Lines that do not follow `LOG_FORMAT`

Some output is written straight to stdout and is never JSON:

- The startup route list printed by `cmd/goats/main.go` (`Starting GoatFlow HTMX server on port ...`).
- Gin's own debug output when `APP_ENV` is not `production`.

### `LOG_OUTPUT` and `LOG_FILE_PATH`

Which destination is used:

1. `LOG_OUTPUT`, if set.
2. Otherwise `LOG_FILE_PATH`, if set.
3. Otherwise stdout.

| Value | Result |
|-------|--------|
| empty or `stdout` (any case) | stdout |
| `stderr` (any case) | stderr |
| any other text | treated as a file path |

For a file path:

- A relative path is joined to the working directory.
- Missing directories are created (mode `0750`).
- The file is opened for append and created if missing (mode `0600`, owner only: logs can hold user and ticket data). An existing file keeps its mode.
- If the directory or file cannot be created, logs go to **stdout** and one line saying why is printed to stderr (`logging: cannot open log file ...`). GoatFlow keeps running.
- The file is closed when the process exits normally.

GoatFlow does not rotate log files.

The Helm chart runs the backend with a read-only root file system, with only `/tmp` writable. A file path outside `/tmp` falls back to stdout there. Keep `LOG_OUTPUT=stdout` on Kubernetes.

### Shipped defaults

| Where | `LOG_LEVEL` | `LOG_FORMAT` |
|-------|-------------|--------------|
| Code default | `info` | `text` |
| `.env.example` | `info` | `json` |
| `deploy/docker-compose.yml` (app, customer-fe) | `warn` (`${LOG_LEVEL:-warn}`) | not set |
| Helm chart | `config.logLevel`, default `info` | not set |
| `make synthesize` generated `.env` | `info` | `json` |

### Plugin logs

Plugin log calls go through `slog`, so `LOG_LEVEL` filters them. They are also kept in the plugin log buffer for the admin viewer.

Set `GOATFLOW_PLUGIN_LOG_ECHO=true` to also write each plugin log call as a standard `log` line, for example `[plugin:example] INFO: message`. These lines skip the `LOG_LEVEL` filter.

## Graceful shutdown

This applies to the server (the default mode).

### What happens

On `SIGTERM` or `SIGINT`:

1. GoatFlow logs `Shutting down: received terminated — draining connections (timeout 5s)`. For `SIGINT` the text is `received interrupt`.
2. It stops accepting new connections and waits for open HTTP requests to finish, for up to `DRAIN_TIMEOUT`. The separate metrics listener, if running, is drained at the same time with the same limit.
3. If requests are still open after that, it logs `Connection drain incomplete` and closes them.
4. In parallel with steps 2-3 it stops the scheduler: running jobs are cancelled and get up to 5 seconds to return. Shutdown waits for this before going on.
5. It stops the plugin hot-reload watcher.
6. It shuts down plugins. Each plugin has its own limit from its resource policy. The whole plugin step is capped at 30 seconds.
7. It closes the log file, if one is open, and exits.

If the main port cannot be opened at startup (for example it is already in use), GoatFlow stops the scheduler and plugins and exits with an error straight away.

### `DRAIN_TIMEOUT`

- Parsed as a Go duration: a number with a unit, for example `500ms`, `5s`, `1m`.
- Unset means the default, `5s`.
- A value that is not a positive duration (a plain number such as `10`, `0s`, `-1s`, a typo) is an error: GoatFlow logs `invalid DRAIN_TIMEOUT "10": want a positive Go duration such as 5s or 1m; using the default 5s` at startup and uses `5s`.
- The server and the runner both use it.

### Stop time to allow

The longest server shutdown is about `max(DRAIN_TIMEOUT, 5s)` (HTTP drain and scheduler stop, in parallel) + 30 seconds (plugins). With the defaults that is 35 seconds.

The shipped deployment files give the container 45 seconds:

- `docker-compose.yml`, `deploy/docker-compose.yml` and the TrueNAS template set `stop_grace_period: 45s` (Docker's own default is 10 seconds).
- The Helm chart sets `terminationGracePeriodSeconds: 45` (Kubernetes' own default is 30 seconds).

If you raise `DRAIN_TIMEOUT`, raise the stop grace period to at least `DRAIN_TIMEOUT` + 40 seconds, or the container runtime kills GoatFlow before plugins have shut down.

### Runner

On `SIGTERM` or `SIGINT` the runner (`goats -mode runner`):

1. Logs `stopping task runner` (`component=runner`).
2. Stops starting new tasks and cancels the context of every running task (email queue, session cleanup, webhook dispatch).
3. Waits up to `DRAIN_TIMEOUT` for the running tasks to return, logs `task runner stopped` and exits. A task that does not return in time is abandoned: the runner logs `tasks still running at stop timeout; exiting without them` at `ERROR` and exits.

Mail the email queue task had not sent yet stays in `mail_queue`: a row is deleted only after it was sent, so the next run picks it up.

## Related docs

- [Configuration](configuration.md)
- [Docker deployment](deployment/docker.md)
- [Kubernetes deployment](deployment/kubernetes.md)
- [Helm chart](../charts/goatflow/README.md)
- [Webhooks](WEBHOOKS.md)
- [Customer portal](CUSTOMER_PORTAL.md)
