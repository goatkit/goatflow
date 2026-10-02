# High Availability & Production Deployment

## Overview

GoatFlow supports production deployments via Docker Compose (single-node) and Kubernetes with Helm (multi-node, auto-scaling). This document covers what's actually implemented and how to deploy for reliability.

## Architecture

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │  / Ingress      │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼─────┐
        │ GoatFlow  │ │ GoatFlow  │ │ GoatFlow  │
        │  Pod 1    │ │  Pod 2    │ │  Pod N    │
        └─────┬─────┘ └──────┬────┘ └───────┬───┘
              │              │              │
              └──────────────┼──────────────┘
                     ┌───────┴───────┐
                     │               │
               ┌─────▼──────┐  ┌─────▼─────┐
               │  Database  │  │  Valkey   │
               │ MySQL/     │  │  (Cache)  │
               │ MariaDB/   │  │           │
               │ PostgreSQL │  │           │
               └────────────┘  └───────────┘
```

## What's Implemented

| Capability | Status | Notes |
|---|---|---|
| Multiple app replicas | ✅ | Helm default: 2 backend pods (`backend.replicaCount`) |
| Horizontal Pod Autoscaler | ✅ | Off by default (`backend.autoscaling.enabled`). CPU 70% / memory 80% targets, 2-10 pods |
| Health checks (liveness/readiness) | ✅ | `GET /health` pings the database and returns 503 when it is down. The chart probes use it |
| Graceful shutdown | ✅ | On SIGTERM the server stops taking new connections and drains in-flight requests (`DRAIN_TIMEOUT`, default `5s`), then stops the scheduler and plugins |
| Scheduled jobs across replicas | ✅ | Each job tick runs on one replica only (database lock, table `gk_scheduler_job_lock`) |
| Prometheus metrics | ✅ | Separate listener on `METRICS_PORT` when `METRICS_ENABLED=true` (chart: `metrics.enabled`, Service `<fullname>-metrics`) |
| Rolling updates | ✅ | Via Kubernetes Deployments |
| Valkey cache | ✅ | Shared cache. `GET /health/detailed` reports its state |
| External database support | ✅ | RDS, Cloud SQL, managed MariaDB, etc. |
| External cache support | ✅ | ElastiCache, Memorystore, etc. (`GOATFLOW_VALKEY_*`; chart: `externalValkey.*`, see below) |
| Multi-arch images | ✅ | amd64 + arm64 |
| TLS/Ingress | ✅ | TLS ends at the ingress or proxy. GoatFlow itself serves plain HTTP |
| Connection pooling | ✅ | `database/sql` pool |
| Rate limiting | ✅ | Login, forgotten-password and sign-up forms, public plugin pages |

## What's NOT Implemented

Be honest with yourself about what you're deploying:

- ❌ Multi-region / geo-replication
- ❌ Database clustering or replication (use your cloud provider's managed service)
- ❌ Message queues (no RabbitMQ, no event bus)
- ❌ Distributed tracing (OpenTelemetry)
- ❌ Active-active clustering
- ❌ Automated disaster recovery
- ❌ Built-in backup automation

For database HA, use a managed service (RDS Multi-AZ, Cloud SQL HA, etc.) rather than trying to run your own cluster.

## Things to know before running more than one replica

- **Scheduled jobs run once per tick, on one replica.** Each backend process starts the built-in scheduler: email account polling, GenericAgent jobs, escalation checks, pending reminders, auto-close of pending tickets, and plugin jobs. When a job is due, every replica tries to claim that run in the `gk_scheduler_job_lock` table; one wins and runs it, the others skip it. The claim lasts until halfway to the job's next run, so replica clocks may differ by up to half the job interval (30 seconds for a job that runs every minute). Keep the clocks in sync (NTP). Which replica runs a given tick is not fixed. The customer-only frontend (`CUSTOMER_FE_ONLY=true`) does not run the scheduler.
- **Email poll intervals are tracked per replica.** The email poll job runs once per minute across all replicas, but the per-account poll interval (`PollIntervalSeconds`) is remembered by the replica that polled. With several replicas an account can be polled more often than its interval, at most once per minute.
- **The runner is a separate process.** `goats -mode runner` sends queued email, delivers webhooks, evaluates ticket notification rules and cleans up sessions. docker-compose runs it as the `runner` service; the Helm chart as the `<fullname>-runner` Deployment (one replica, `runner.*` values). It gets the same database settings, `JWT_SECRET` and `GOATFLOW_SECURE_KEY` as the backend (the chart keeps the key in the `<fullname>-app` Secret), so it can decrypt webhook signing secrets.
- **Migrations run once.** Every backend replica and the runner run migrations at start; a database lock lets one process migrate while the others wait and then find the schema up to date.
- **Files on disk need shared storage.** `STORAGE_PATH` holds plugin files (`<STORAGE_PATH>/plugins`) and, with `STORAGE_TYPE=fs`, attachments (`<STORAGE_PATH>/var/article`); every backend replica must see the same directory. The Helm chart mounts the `<fullname>-storage` PVC there (ReadWriteOnce by default): with replicas on several nodes, set `config.storage.persistence.accessModes` to `[ReadWriteMany]` and use an RWX storage class. The default `STORAGE_TYPE=db` keeps attachments in the database.

## Deployment Options

### Docker Compose (Single Node)

Best for: small teams, dev/staging, single-server deployments.

```bash
docker compose up -d
```

This gives you GoatFlow + database + Valkey on one machine. Simple, easy to back up, easy to restore. For many teams, this is all you need.

### Kubernetes + Helm (Multi-Node)

Best for: larger deployments, auto-scaling, cloud-native environments.

```bash
# Basic installation
helm install goatflow ./charts/goatflow

# With PostgreSQL instead of MySQL
helm install goatflow ./charts/goatflow -f charts/goatflow/values-postgresql.yaml

# Production with autoscaling
helm install goatflow ./charts/goatflow \
  --set backend.replicaCount=3 \
  --set backend.autoscaling.enabled=true \
  --set backend.autoscaling.minReplicas=3 \
  --set backend.autoscaling.maxReplicas=10
```

### External Database (Recommended for Production)

Don't run your own database in Kubernetes. Use a managed service:

```bash
helm install goatflow ./charts/goatflow \
  --set database.external.enabled=true \
  --set database.external.host=your-rds-endpoint.amazonaws.com \
  --set database.external.existingSecret=goatflow-db-credentials
```

### External Cache (Optional)

```bash
helm install goatflow ./charts/goatflow \
  --set valkey.enabled=false \
  --set externalValkey.enabled=true \
  --set externalValkey.host=your-elasticache-endpoint \
  --set externalValkey.existingSecret=goatflow-valkey   # key valkey-password; omit when no password
```

The chart passes these to the backend and the runner as `GOATFLOW_VALKEY_HOST`, `GOATFLOW_VALKEY_PORT` and `GOATFLOW_VALKEY_PASSWORD`.

## Resource Sizing

### Small (< 50 agents)

```yaml
backend:
  replicaCount: 2
  resources:
    requests:
      cpu: "250m"
      memory: "256Mi"
    limits:
      cpu: "1000m"
      memory: "1Gi"
```

### Medium (50-200 agents)

```yaml
backend:
  replicaCount: 3
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 6
  resources:
    requests:
      cpu: "500m"
      memory: "512Mi"
    limits:
      cpu: "2000m"
      memory: "2Gi"
```

### Large (200+ agents)

```yaml
backend:
  replicaCount: 5
  autoscaling:
    enabled: true
    minReplicas: 5
    maxReplicas: 10
  resources:
    requests:
      cpu: "1000m"
      memory: "1Gi"
    limits:
      cpu: "4000m"
      memory: "4Gi"
```

## Monitoring

Full details: [OBSERVABILITY.md](OBSERVABILITY.md).

### Health endpoints

| Endpoint | Login | What it checks |
|---|---|---|
| `GET /health` (and `/healthz`) | None | Pings the database (500 ms timeout). `200` with `"status": "healthy"`, or `503` with `"status": "unhealthy"` when the database is unreachable. Use it for liveness and readiness probes. |
| `GET /health/detailed` | Admin | Database and Valkey cache (`ok`, `error` or `disabled`), version and uptime. `503` when the database is down. For operators, not for probes. |

### Prometheus metrics

- `GET /metrics` on the main port needs an admin login. It is for manual checks.
- For scraping, set `METRICS_ENABLED=true`. GoatFlow then serves metrics without login on a separate port, `METRICS_PORT` (default `9090`). Keep that port on the internal network. Do not publish it.
- On Helm, add both variables with `backend.extraEnv`, and set `backend.podAnnotations` if your Prometheus uses `prometheus.io/*` annotations.

Metrics include `goatflow_up`, `goatflow_process_start_time_seconds`, the Valkey cache counters (`cache_hits_total`, `cache_misses_total`, ...) and the email poller metrics.

### Graceful shutdown

On SIGTERM or SIGINT, GoatFlow:

1. Stops accepting new connections.
2. Lets in-flight requests finish, for up to `DRAIN_TIMEOUT` (default `5s`, Go duration format). At the same time it stops the scheduler: running jobs are cancelled and get up to 5 seconds to return.
3. Shuts down plugins (capped at 30 seconds).

The longest stop is about `max(DRAIN_TIMEOUT, 5s)` + 30 seconds = 35 seconds with the defaults. The Helm chart and the compose files give the container a 45-second stop grace period (`terminationGracePeriodSeconds` / `stop_grace_period`). If you raise `DRAIN_TIMEOUT`, raise the grace period to at least `DRAIN_TIMEOUT` + 40 seconds.

The runner (`goats -mode runner`) cancels its running tasks on SIGTERM and waits up to `DRAIN_TIMEOUT` for them.

### Logs

Set `LOG_FORMAT=json` for log aggregators. `LOG_LEVEL` and `LOG_OUTPUT` are described in [configuration.md](configuration.md#logging).

### Other things to watch

- Database metrics from your managed service dashboard.
- Alerts on pod restarts, error rates and response latency.

## Backup Strategy

GoatFlow doesn't handle backups — your database does. Recommended approach:

1. **Managed database**: Enable automated backups (RDS snapshots, Cloud SQL backups)
2. **Self-hosted database**: Set up `mysqldump` / `pg_dump` on a cron schedule
3. **Attachments on disk**: with `STORAGE_TYPE=fs`, back up `<STORAGE_PATH>/var/article` too
4. **Test restores regularly** — untested backups aren't backups

## Maintenance

### Rolling Updates

```bash
# Update via Helm
helm upgrade goatflow ./charts/goatflow --set backend.image.tag=0.10.0

# Check rollout status
kubectl rollout status deployment/goatflow-backend
```

### Scaling

```bash
# Manual scaling
kubectl scale deployment/goatflow-backend --replicas=5

# Or enable autoscaling
helm upgrade goatflow ./charts/goatflow \
  --set backend.autoscaling.enabled=true
```

## Troubleshooting

```bash
# Check pod status
kubectl get pods -l app.kubernetes.io/name=goatflow

# Check logs
kubectl logs -l app.kubernetes.io/name=goatflow --tail=100

# Check resource usage
kubectl top pods -l app.kubernetes.io/name=goatflow

# Check HPA status
kubectl get hpa
```
