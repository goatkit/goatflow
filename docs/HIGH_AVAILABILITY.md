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
| Graceful shutdown | ✅ | On SIGTERM the server stops taking new connections and drains in-flight requests (`DRAIN_TIMEOUT`, default `10s`) |
| Prometheus metrics | ✅ | Separate listener on `METRICS_PORT` when `METRICS_ENABLED=true` |
| Rolling updates | ✅ | Via Kubernetes Deployments |
| Valkey cache | ✅ | Shared cache. `GET /health/detailed` reports its state |
| External database support | ✅ | RDS, Cloud SQL, managed MariaDB, etc. |
| External cache support | ✅ | ElastiCache, Memorystore, etc. (set `GOATFLOW_VALKEY_HOST`, see below) |
| Multi-arch images | ✅ | amd64 + arm64 |
| TLS/Ingress | ✅ | TLS ends at the ingress or proxy. GoatFlow itself serves plain HTTP |
| Connection pooling | ✅ | `database/sql` pool |
| Rate limiting | ✅ | Login, forgotten-password and sign-up forms, public plugin pages |

## Helm chart: environment variables to add (0.10.0)

Some variables that the chart sets are not read by GoatFlow 0.10.0. Add the real ones with `backend.extraEnv` until the chart is fixed.

| Chart sets | GoatFlow reads | What happens without a fix |
|---|---|---|
| `DB_TYPE` | `DB_DRIVER` (`mysql`, `mariadb` or `postgres`) | GoatFlow always uses the MySQL driver. PostgreSQL installs do not connect. |
| `APP_SECRET` | `JWT_SECRET` | Login tokens are signed with the public placeholder in `default.yaml`. |
| `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` | `GOATFLOW_VALKEY_HOST`, `GOATFLOW_VALKEY_PORT`, `GOATFLOW_VALKEY_PASSWORD` | GoatFlow looks for Valkey at host `valkey`. |
| `SERVER_PORT`, `SESSION_TIMEOUT`, `CACHE_ENABLED` | not read | Nothing. The port is `APP_PORT` (default `8080`). |

The `-backend-config` ConfigMap (`STORAGE_TYPE`, `STORAGE_PATH`) is not mounted into the pod, so the storage values do not reach GoatFlow either. The default (`db`) works without it.

Example values file:

```yaml
backend:
  extraEnv:
    - name: DB_DRIVER
      value: postgres            # or mysql
    - name: JWT_SECRET
      valueFrom:
        secretKeyRef:
          name: goatflow-jwt     # a Secret you create, 32+ characters
          key: jwt-secret
    - name: GOATFLOW_VALKEY_HOST
      value: my-valkey.example.internal
    - name: GOATFLOW_SECURE_KEY
      valueFrom:
        secretKeyRef:
          name: goatflow-secure-key   # 64 hex characters
          key: key
```

See [configuration.md](configuration.md) for every variable.

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

- **Scheduled jobs run on every replica.** Each backend process starts the built-in scheduler: email account polling, GenericAgent jobs, escalation checks, pending reminders, auto-close of pending tickets, and plugin jobs. There is no lock between replicas, so with 3 replicas each job runs 3 times. The customer-only frontend (`CUSTOMER_FE_ONLY=true`) does not run the scheduler.
- **The runner is a separate process.** `goats -mode runner` sends queued email, cleans up sessions and delivers webhooks. docker-compose runs it as the `runner` service. The Helm chart has no runner Deployment, so on Kubernetes you must add one yourself (same image and environment, command `./goats -mode runner`). Give it the same `GOATFLOW_SECURE_KEY` as the backend, or it cannot decrypt webhook signing secrets.
- **Attachments on disk need shared storage.** With `STORAGE_TYPE=fs`, every replica must see the same `<STORAGE_PATH>/var/article` (a ReadWriteMany volume). The chart does not provision one. The default `STORAGE_TYPE=db` keeps attachments in the database and needs nothing extra.

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
# (also set DB_DRIVER=postgres in backend.extraEnv, see the table above)
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

The chart's `externalValkey.*` values only set `REDIS_*` variables, which GoatFlow does not read. Set the host with `backend.extraEnv` as well:

```bash
helm install goatflow ./charts/goatflow \
  --set valkey.enabled=false \
  --set externalValkey.enabled=true \
  --set externalValkey.host=your-elasticache-endpoint \
  --set 'backend.extraEnv[0].name=GOATFLOW_VALKEY_HOST' \
  --set 'backend.extraEnv[0].value=your-elasticache-endpoint'
```

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
2. Lets in-flight requests finish, for up to `DRAIN_TIMEOUT` (default `10s`, Go duration format).
3. Stops the scheduler and plugins (plugin shutdown is capped at 30 seconds).

Give the pod enough stop time for both steps. Kubernetes waits `terminationGracePeriodSeconds` (30 seconds unless you change it) before it kills the container.

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
