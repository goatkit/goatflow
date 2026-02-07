# GoatFlow Helm Chart

Helm chart for deploying GoatFlow (Go Ticket Request System) on Kubernetes.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.12+
- Ingress controller (nginx-ingress recommended)
- PV provisioner support in the cluster

## Installation

### Install from OCI Registry

The chart is published with tags that mirror the container image tags:

```bash
# Install latest release
helm install goatflow oci://ghcr.io/goatkit/charts/goatflow --version latest

# Install specific version (deploys matching container images)
helm install goatflow oci://ghcr.io/goatkit/charts/goatflow --version v0.5.0

# Install from main branch (latest development)
helm install goatflow oci://ghcr.io/goatkit/charts/goatflow --version main

# Install from dev branch
helm install goatflow oci://ghcr.io/goatkit/charts/goatflow --version dev
```

**Tag Mirroring:** The chart's `appVersion` matches the tag, so `--version v0.5.0` automatically deploys `:v0.5.0` container images.

### Install from GitHub Release

```bash
# Download and install from release assets
helm install goatflow https://github.com/goatkit/goatflow/releases/download/v1.0.0/goatflow-0.1.0.tgz
```

### Install from Cloned Repository

```bash
# Clone the repository
git clone https://github.com/goatkit/goatflow.git
cd goatflow

# Update dependencies
helm dependency update charts/goatflow

# Install with default values (MySQL)
helm install goatflow ./charts/goatflow

# Install with PostgreSQL
helm install goatflow ./charts/goatflow -f charts/goatflow/values-postgresql.yaml

# Install with custom values
helm install goatflow ./charts/goatflow --set backend.replicaCount=3
```

### Development with Make

```bash
# Lint the chart
make helm-lint

# Render templates (dry-run)
make helm-template

# Render with PostgreSQL values
make helm-template-pg

# Package for distribution
make helm-package
```

## Configuration

### Database Selection

The chart supports MySQL (default) or PostgreSQL:

```yaml
# MySQL (default)
database:
  type: mysql

# PostgreSQL
database:
  type: postgresql
```

### Using External Database

```yaml
database:
  external:
    enabled: true
    host: "your-rds-endpoint.amazonaws.com"
    port: "3306"
    database: "goatflow"
    existingSecret: "goatflow-db-credentials"  # Secret with keys: username, password
```

### Valkey (Redis-compatible Cache)

The chart uses the official [valkey-helm](https://github.com/valkey-io/valkey-helm) subchart:

```yaml
valkey:
  enabled: true
  auth:
    enabled: true
    aclConfig: "user default on >your-secure-password ~* &* +@all"
  persistence:
    enabled: true
    size: 1Gi
```

For external Redis/Valkey (ElastiCache, etc.):

```yaml
valkey:
  enabled: false

externalValkey:
  enabled: true
  host: "your-elasticache-endpoint"
  port: 6379
  existingSecret: "goatflow-valkey-credentials"
```

### Ingress

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  hosts:
    - host: goatflow.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: goatflow-tls
      hosts:
        - goatflow.example.com
```

### Resource Limits

```yaml
backend:
  resources:
    requests:
      cpu: "250m"
      memory: "256Mi"
    limits:
      cpu: "1"
      memory: "1Gi"
```

### Autoscaling

```yaml
backend:
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70
    targetMemoryUtilizationPercentage: 80
```

### Annotations & Labels

Inject custom annotations and labels into resources for cloud integrations:

```yaml
# Global annotations/labels applied to ALL resources
global:
  commonAnnotations:
    company.io/team: "platform"
  commonLabels:
    environment: "production"

# AWS EKS with IRSA (IAM Roles for Service Accounts)
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::123456789:role/goatflow"

# GKE Workload Identity
serviceAccount:
  annotations:
    iam.gke.io/gcp-service-account: "goatflow@project.iam.gserviceaccount.com"

# Prometheus scraping
backend:
  podAnnotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9090"
    prometheus.io/path: "/metrics"

# Istio sidecar injection
backend:
  podAnnotations:
    sidecar.istio.io/inject: "true"
  podLabels:
    app: goatflow
    version: v1

# AWS Load Balancer configuration
backend:
  serviceAnnotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-scheme: "internet-facing"
```

### Extra Resources

Define arbitrary Kubernetes resources with full Helm templating support:

```yaml
extraResources:
  # Custom ConfigMap
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: "{{ .Release.Name }}-custom-config"
      namespace: "{{ .Release.Namespace }}"
      labels:
        {{- include "goatflow.labels" . | nindent 8 }}
    data:
      backend-url: "http://{{ .Release.Name }}-backend:{{ .Values.backend.service.port }}"
      app-version: "{{ .Chart.AppVersion }}"

  # PodDisruptionBudget
  - apiVersion: policy/v1
    kind: PodDisruptionBudget
    metadata:
      name: "{{ .Release.Name }}-backend-pdb"
    spec:
      minAvailable: 1
      selector:
        matchLabels:
          app.kubernetes.io/name: goatflow
          app.kubernetes.io/component: backend

  # CronJob for scheduled tasks
  - apiVersion: batch/v1
    kind: CronJob
    metadata:
      name: "{{ .Release.Name }}-cleanup"
    spec:
      schedule: "0 2 * * *"
      jobTemplate:
        spec:
          template:
            spec:
              containers:
                - name: cleanup
                  image: "{{ .Values.backend.image.repository }}:{{ .Values.backend.image.tag | default .Chart.AppVersion }}"
                  command: ["/app/goatflow", "cleanup"]
              restartPolicy: OnFailure
```

**Note:** Strings containing `{{` must be quoted in YAML.

## Values Reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.commonAnnotations` | Annotations applied to all resources | `{}` |
| `global.commonLabels` | Labels applied to all resources | `{}` |
| `backend.enabled` | Enable backend deployment | `true` |
| `backend.replicaCount` | Number of backend replicas | `2` |
| `backend.image.repository` | Backend image repository | `goatflow/backend` |
| `backend.image.tag` | Backend image tag | `""` (uses appVersion) |
| `backend.podAnnotations` | Annotations for backend pods | `{}` |
| `backend.podLabels` | Labels for backend pods | `{}` |
| `backend.serviceAnnotations` | Annotations for backend service | `{}` |
| `frontend.enabled` | Enable frontend deployment | `true` |
| `frontend.replicaCount` | Number of frontend replicas | `2` |
| `frontend.podAnnotations` | Annotations for frontend pods | `{}` |
| `frontend.serviceAnnotations` | Annotations for frontend service | `{}` |
| `database.type` | Database type: mysql or postgresql | `mysql` |
| `database.external.enabled` | Use external database | `false` |
| `serviceAccount.annotations` | ServiceAccount annotations (IRSA, WI) | `{}` |
| `valkey.enabled` | Deploy Valkey subchart | `true` |
| `ingress.enabled` | Enable ingress | `false` |
| `config.logLevel` | Application log level | `info` |
| `extraResources` | Additional K8s resources (templated) | `[]` |

See `values.yaml` for full configuration options.

## Security

### Secrets Management

For production, use external secrets management:

```yaml
secrets:
  create: false  # Don't create secrets from values

database:
  mysql:
    existingSecret: "goatflow-mysql-secret"  # Pre-created secret
```

### Network Policies

The chart does not include NetworkPolicies by default. Add them based on your cluster's security requirements.

### Pod Security

All pods run as non-root (UID 1000) by default.

## Upgrading

```bash
helm upgrade goatflow ./charts/goatflow
```

## Uninstalling

```bash
helm uninstall goatflow
```

**Note**: PVCs are not deleted by default. To remove persistent data:

```bash
kubectl delete pvc -l app.kubernetes.io/instance=goatflow
```

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -l app.kubernetes.io/instance=goatflow
kubectl describe pod <pod-name>
kubectl logs <pod-name>
```

### Database Connection Issues

```bash
# Check database pod
kubectl logs -l app.kubernetes.io/component=database

# Test connectivity from backend
kubectl exec -it <backend-pod> -- nc -zv <db-service> 3306
```

### Valkey Connection Issues

```bash
# Check valkey pods
kubectl get pods -l app.kubernetes.io/name=valkey

# Test connectivity
kubectl exec -it <backend-pod> -- nc -zv <valkey-service> 6379
```

## License

Apache 2.0 - See [LICENSE](../../LICENSE) for details.
