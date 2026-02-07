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
| Multiple app replicas | ✅ | Default: 2 backend pods |
| Horizontal Pod Autoscaler | ✅ | CPU 70% / Memory 80% targets, 2-10 pods |
| Health checks (liveness/readiness) | ✅ | Built-in probes |
| Rolling updates | ✅ | Zero-downtime deploys via Kubernetes |
| Valkey/Redis caching | ✅ | Session + query cache |
| External database support | ✅ | RDS, Cloud SQL, managed MariaDB, etc. |
| External cache support | ✅ | ElastiCache, Memorystore, etc. |
| Multi-arch images | ✅ | amd64 + arm64 |
| TLS/Ingress | ✅ | Via Kubernetes ingress controller |
| Connection pooling | ✅ | Configurable MaxOpenConns/MaxIdleConns |
| Rate limiting | ✅ | Login + API token rate limiting |

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
  --set externalValkey.host=your-elasticache-endpoint
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

GoatFlow exposes a `/health` endpoint for liveness and readiness probes. For production monitoring:

- Use your cloud provider's monitoring (CloudWatch, Stackdriver, etc.)
- Monitor database metrics via your managed service dashboard
- Set up alerts on pod restarts, error rates, and response latency

## Backup Strategy

GoatFlow doesn't handle backups — your database does. Recommended approach:

1. **Managed database**: Enable automated backups (RDS snapshots, Cloud SQL backups)
2. **Self-hosted database**: Set up `mysqldump` / `pg_dump` on a cron schedule
3. **Test restores regularly** — untested backups aren't backups

## Maintenance

### Rolling Updates

```bash
# Update via Helm
helm upgrade goatflow ./charts/goatflow --set backend.image.tag=v0.6.5

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
