# High Availability Setup

## Overview

GOTRS high availability (HA) setup ensures zero downtime, automatic failover, and horizontal scalability. The architecture includes:

- **Multi-region deployment** with geo-replication
- **Database clustering** with automatic failover
- **Message queue clustering** for reliable event processing
- **Load balancing** across multiple application instances
- **Auto-scaling** based on load metrics
- **Disaster recovery** with automated backups

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                      Load Balancer                        │
│                    (AWS NLB / GCP LB)                     │
└────────────┬──────────────────────────┬──────────────────┘
             │                          │
    ┌────────▼────────┐        ┌───────▼────────┐
    │   Region 1      │        │    Region 2     │
    │                 │        │                 │
    │  ┌───────────┐  │        │  ┌───────────┐  │
    │  │  GOTRS    │  │        │  │  GOTRS    │  │
    │  │ Instances │  │        │  │ Instances │  │
    │  │  (3-20)   │  │        │  │  (3-20)   │  │
    │  └─────┬─────┘  │        │  └─────┬─────┘  │
    │        │        │        │        │        │
    │  ┌─────▼─────┐  │        │  ┌─────▼─────┐  │
    │  │PostgreSQL │  │        │  │PostgreSQL │  │
    │  │  Cluster  │◄─┼────────┼─►│  Cluster  │  │
    │  │    (3)    │  │        │  │    (3)    │  │
    │  └───────────┘  │        │  └───────────┘  │
    │                 │        │                 │
    │  ┌───────────┐  │        │  ┌───────────┐  │
    │  │   Redis   │◄─┼────────┼─►│   Redis   │  │
    │  │  Cluster  │  │        │  │  Cluster  │  │
    │  │    (6)    │  │        │  │    (6)    │  │
    │  └───────────┘  │        │  └───────────┘  │
    │                 │        │                 │
    │  ┌───────────┐  │        │  ┌───────────┐  │
    │  │ RabbitMQ  │◄─┼────────┼─►│ RabbitMQ  │  │
    │  │  Cluster  │  │        │  │  Cluster  │  │
    │  │    (3)    │  │        │  │    (3)    │  │
    │  └───────────┘  │        │  └───────────┘  │
    └─────────────────┘        └─────────────────┘
```

## Components

### 1. PostgreSQL High Availability

**Features:**
- 3-node cluster with automatic failover
- Synchronous replication for zero data loss
- Read replicas for load distribution
- Point-in-time recovery (PITR)
- Automated backups to S3

**Configuration:**
```yaml
spec:
  instances: 3
  primaryUpdateStrategy: unsupervised
  postgresql:
    parameters:
      synchronous_commit: "on"
      synchronous_standby_names: "*"
      max_connections: 200
```

**Connection Strings:**
- Write operations: `postgres-cluster-rw:5432`
- Read operations: `postgres-cluster-ro:5432`
- Read replicas only: `postgres-cluster-r:5432`

### 2. Redis Cluster

**Features:**
- 6-node cluster (3 masters, 3 replicas)
- Automatic sharding across nodes
- Automatic failover
- Data persistence with AOF
- Configurable eviction policies

**Configuration:**
```yaml
spec:
  replicas: 6
  cluster-enabled: yes
  cluster-replicas: 1
  maxmemory-policy: allkeys-lru
```

### 3. RabbitMQ Cluster

**Features:**
- 3-node cluster with quorum queues
- Message persistence
- Automatic partition healing
- Dead letter exchanges
- Management interface

**Queue Types:**
- **Quorum Queues**: For critical messages requiring consistency
- **Classic Mirrored**: For high-throughput scenarios

### 4. Application Layer

**Features:**
- Horizontal auto-scaling (3-20 pods)
- Rolling updates with zero downtime
- Health checks and automatic restarts
- Session affinity for WebSocket connections
- Distributed tracing

**Auto-scaling Triggers:**
- CPU > 70%
- Memory > 80%
- Request rate > 1000 req/s

## Deployment

### Prerequisites

1. Kubernetes cluster (1.25+)
2. Helm 3.12+
3. Ingress controller (nginx-ingress recommended)
4. Storage class with SSD support
5. (Optional) Prometheus operator for monitoring

### Installation with Helm

Deploy GOTRS using the Helm chart:

```bash
# Basic installation
helm install gotrs ./charts/gotrs

# With PostgreSQL instead of MySQL
helm install gotrs ./charts/gotrs -f charts/gotrs/values-postgresql.yaml

# Production configuration with autoscaling
helm install gotrs ./charts/gotrs \
  --set backend.replicaCount=3 \
  --set backend.autoscaling.enabled=true \
  --set backend.autoscaling.minReplicas=3 \
  --set backend.autoscaling.maxReplicas=10
```

### External Database (Production)

For production HA, use a managed database service:

```bash
helm install gotrs ./charts/gotrs \
  --set database.external.enabled=true \
  --set database.external.host=your-rds-endpoint.amazonaws.com \
  --set database.external.existingSecret=gotrs-db-credentials
```

### External Cache (Production)

For production HA, use a managed Redis/Valkey service:

```bash
helm install gotrs ./charts/gotrs \
  --set valkey.enabled=false \
  --set externalValkey.enabled=true \
  --set externalValkey.host=your-elasticache-endpoint
```

## Monitoring

### Key Metrics

**Application:**
- Request rate and latency
- Error rate
- Active connections
- Cache hit ratio

**Database:**
- Replication lag
- Connection pool usage
- Query performance
- Transaction rate

**Message Queue:**
- Queue depth
- Message rate
- Consumer lag
- Connection count

### Dashboards

Access monitoring dashboards:
- Grafana: `http://monitoring.gotrs.local`
- RabbitMQ: `http://rabbitmq.gotrs.local`
- PostgreSQL: Custom Grafana dashboard

## Failover Scenarios

### 1. Application Pod Failure

- **Detection**: Liveness probe failure (3 consecutive failures)
- **Recovery**: Automatic pod restart
- **Time to Recovery**: < 30 seconds
- **Data Loss**: None

### 2. PostgreSQL Primary Failure

- **Detection**: Patroni health check
- **Recovery**: Automatic promotion of synchronous standby
- **Time to Recovery**: < 60 seconds
- **Data Loss**: None (synchronous replication)

### 3. Redis Node Failure

- **Detection**: Cluster health check
- **Recovery**: Automatic failover to replica
- **Time to Recovery**: < 10 seconds
- **Data Loss**: Minimal (depends on replication lag)

### 4. Zone/Region Failure

- **Detection**: Health check failures across zone
- **Recovery**: Traffic redirect to healthy zone
- **Time to Recovery**: < 2 minutes
- **Data Loss**: None (multi-region replication)

## Backup and Recovery

### Backup Schedule

- **PostgreSQL**: Daily full backup, hourly WAL archiving
- **Redis**: Hourly RDB snapshots
- **Configuration**: Version controlled in Git

### Recovery Procedures

**Point-in-Time Recovery:**
```bash
# Restore PostgreSQL to specific time
kubectl cnpg recover postgres-cluster \
  --target-time "2024-01-15 14:30:00" \
  --backup-name postgres-backup-20240115
```

**Disaster Recovery:**
```bash
# Full cluster restore from backup
./scripts/disaster-recovery.sh --region us-west-2 --backup-date 2024-01-15
```

## Performance Tuning

### Database Optimization

```sql
-- Connection pooling
max_connections = 200
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 4MB

-- Write performance
checkpoint_completion_target = 0.9
wal_buffers = 16MB
max_wal_size = 4GB

-- Query performance
random_page_cost = 1.1
effective_io_concurrency = 200
```

### Application Tuning

```yaml
resources:
  requests:
    memory: 512Mi
    cpu: 500m
  limits:
    memory: 2Gi
    cpu: 2000m
```

### Cache Configuration

```yaml
cache:
  default_ttl: 5m
  query_cache_ttl: 1m
  compression_enabled: true
  compression_threshold: 1024
```

## Security Considerations

### Network Policies

- Restrict pod-to-pod communication
- Enforce TLS for all connections
- Implement network segmentation

### Secrets Management

- Use Kubernetes secrets for credentials
- Enable encryption at rest
- Rotate credentials regularly

### Access Control

- RBAC for Kubernetes resources
- Database user permissions
- API rate limiting

## Maintenance

### Rolling Updates

```bash
# Update application with zero downtime
kubectl set image deployment/gotrs-ha gotrs=gotrs/gotrs:v2.0.0
kubectl rollout status deployment/gotrs-ha
```

### Scaling Operations

```bash
# Manual scaling
kubectl scale deployment/gotrs-ha --replicas=10

# Update auto-scaling limits
kubectl patch hpa gotrs-ha-hpa --patch '{"spec":{"maxReplicas":30}}'
```

### Health Checks

```bash
# Check cluster health
kubectl get pods -l app=gotrs
kubectl get pods -l postgresql=postgres-cluster
kubectl get pods -l app.kubernetes.io/name=rabbitmq-cluster

# Check replication status
kubectl exec -it postgres-cluster-1 -- psql -c "SELECT * FROM pg_stat_replication;"
```

## Troubleshooting

### Common Issues

**High Memory Usage:**
```bash
# Check memory usage
kubectl top pods -l app=gotrs

# Increase memory limits
kubectl patch deployment gotrs-ha --patch '{"spec":{"template":{"spec":{"containers":[{"name":"gotrs","resources":{"limits":{"memory":"4Gi"}}}]}}}}'
```

**Slow Queries:**
```bash
# Check slow query log
kubectl exec -it postgres-cluster-1 -- psql -c "SELECT * FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;"
```

**Message Queue Backlog:**
```bash
# Check queue depth
kubectl exec -it rabbitmq-cluster-0 -- rabbitmqctl list_queues name messages consumers
```

## Best Practices

1. **Regular Testing**: Conduct monthly failover tests
2. **Capacity Planning**: Monitor growth trends and scale proactively
3. **Documentation**: Keep runbooks updated
4. **Monitoring**: Set up comprehensive alerting
5. **Backup Verification**: Test restore procedures regularly
6. **Security Updates**: Apply patches promptly
7. **Performance Baselines**: Establish and monitor against baselines

## Cost Optimization

- Use spot instances for non-critical workloads
- Implement pod autoscaling based on actual load
- Right-size resource requests and limits
- Use reserved instances for predictable workloads
- Implement data lifecycle policies for backups