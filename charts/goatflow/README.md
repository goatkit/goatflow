# GoatFlow Helm Chart

Helm chart for deploying GoatFlow on Kubernetes.

> **Read [Known limitations (0.10.0 chart)](#known-limitations-0100-chart) before you install.**
> A default install of this chart does not give a working GoatFlow yet.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.12+
- Ingress controller (nginx-ingress recommended)
- PV provisioner support in the cluster

## Installation

### Install from OCI Registry

CI publishes the chart to `oci://ghcr.io/goatkit/charts/goatflow`
(`.github/workflows/build.yml`):

| Chart version | Published when | Deploys image tag |
|---------------|----------------|-------------------|
| `0.10.0` | the `v0.10.0` release tag is pushed | `0.10.0` |
| `0.1.0-main` | a commit is pushed to `main` | `main` |

```bash
# Release (recommended)
helm install goatflow oci://ghcr.io/goatkit/charts/goatflow --version 0.10.0

# Latest build of the main branch
helm install goatflow oci://ghcr.io/goatkit/charts/goatflow --version 0.1.0-main
```

The chart's `appVersion` is set to the image tag, so `--version 0.10.0` deploys
`ghcr.io/goatkit/goatflow:0.10.0`. Helm chart versions must be semantic versions, so there is
no `latest` chart version.

### Install from GitHub Release

Each release has the packaged chart attached:

```bash
helm install goatflow https://github.com/goatkit/goatflow/releases/download/v0.10.0/goatflow-helm-chart-0.10.0.tgz

# Always the newest release
curl -LO https://github.com/goatkit/goatflow/releases/latest/download/goatflow-helm-chart-latest.tgz
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
make helm ARGS="lint charts/goatflow"

# Render templates (dry-run)
make helm ARGS="template goatflow charts/goatflow"

# Update chart dependencies (valkey subchart)
make helm ARGS="dependency update charts/goatflow"
```

## What the Chart Deploys

| Component | Image | Notes |
|-----------|-------|-------|
| Backend Deployment | `ghcr.io/goatkit/goatflow` | Agent UI, customer portal, REST API. Runs migrations at start. Also runs the in-process scheduler (email polling, escalation checks, pending reminders, auto close, GenericAgent jobs). Probes use `GET /health`, which pings the database. |
| Frontend Deployment | `nginx:1.25-alpine` | The Ingress sends traffic here. It is not a separate customer portal. |
| Database StatefulSet | `mariadb:11` or `postgres:16-alpine` | Skipped when `database.external.enabled` is true. |
| Valkey | valkey-helm subchart | Redis-compatible cache. |
| Ingress | — | Optional (`ingress.enabled`). |

## Known limitations (0.10.0 chart)

The 0.10.0 chart does not yet match the Docker Compose stack (`deploy/docker-compose.yml`) or
the TrueNAS app. Each row below was checked against `charts/goatflow/templates/` and the
GoatFlow code.

### What does not work

| Limitation | What happens | Work-around |
|------------|--------------|-------------|
| **No background runner** | The runner (`./goats -mode runner`, image `ghcr.io/goatkit/goatflow-runner`) is the only process that runs the email queue, webhook dispatch and session cleanup tasks. On a Helm install no outgoing email is sent (customer ticket emails, password-reset and sign-up links stay in the `mail_queue` table), webhooks are never delivered, and expired sessions are not cleaned up. | Run a runner Deployment yourself, for example through `extraResources`. Give it the same database settings, `JWT_SECRET` and `GOATFLOW_SECURE_KEY` as the backend, plus SMTP settings (`GOATFLOW_EMAIL_SMTP_HOST`, `GOATFLOW_EMAIL_SMTP_PORT`, `GOATFLOW_EMAIL_SMTP_USER`, `GOATFLOW_EMAIL_SMTP_PASSWORD`, `GOATFLOW_EMAIL_FROM`). |
| **Ingress does not reach the UI** | The Ingress sends every path to the frontend nginx. That nginx proxies only `/api/` and `/ws` to the backend and answers everything else from its own files. `/login`, `/customer`, `/admin` and the other pages never reach the backend. nginx also answers `/health` itself with a fixed 200. | Set `ingress.enabled: false` and create your own Ingress to the Service `<fullname>-backend`, port 8080 (for example through `extraResources`). |
| **`GOATFLOW_SECURE_KEY` is not set** | Each backend pod makes its own random key at start and writes it to the log. Encrypted values (plugin secure settings, webhook signing secrets) cannot be read after a restart or by another replica. | Create a Secret with 64 hex characters (`openssl rand -hex 32`) and set `GOATFLOW_SECURE_KEY` from it in `backend.extraEnv`. Never change it later. |
| **Environment names GoatFlow does not read** | The backend gets `DB_TYPE` (GoatFlow reads `DB_DRIVER`, default `mysql`), `APP_SECRET` (GoatFlow reads `JWT_SECRET`), `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` and `CACHE_ENABLED` (GoatFlow reads `GOATFLOW_VALKEY_HOST`, `GOATFLOW_VALKEY_PORT`, `GOATFLOW_VALKEY_PASSWORD`), `SERVER_PORT` (GoatFlow reads `APP_PORT`, default 8080) and `SESSION_TIMEOUT` (not read). Results: PostgreSQL installs try to connect as MySQL; login tokens are signed with the placeholder secret from `config/default.yaml`; the cache is turned off. | Set `DB_DRIVER`, `JWT_SECRET` and the `GOATFLOW_VALKEY_*` variables in `backend.extraEnv` (example below). |
| **`backend-config` ConfigMap is not mounted** | The chart writes `STORAGE_TYPE`, `STORAGE_PATH` and the values above into the `<fullname>-backend-config` ConfigMap, but the Deployment does not load it. `config.storage.*` has no effect; attachments stay in the database (`db`). `config.email.*` is not used anywhere. | Keep `db` storage. Email needs the runner (first row). |
| **Bundled MariaDB never becomes ready** | The MariaDB probes run `mysqladmin`, which the `mariadb:11` image does not include. | Use PostgreSQL (with `DB_DRIVER`) or an external database. |
| **Valkey password Secret is missing** | With `valkey.enabled: true` the backend reads `REDIS_PASSWORD` from the Secret `<release>-valkey`, key `valkey-password`. The Valkey subchart does not create that Secret, so the backend pod cannot start. | Create that Secret yourself (the password is the one in `valkey.auth.aclConfig`, default `changeme`), or set `valkey.auth.existingSecret`. |
| **Secrets change on upgrade** | Database and app secrets left empty in values are generated again on every `helm upgrade`. The database volume keeps the old password, so the backend can no longer log in. | Set `database.*.password` and `secrets.appSecretKey`, or use existing Secrets with `secrets.create: false`. |
| **No customer-only instance** | Docker Compose and TrueNAS can run a second, customer-only instance (`CUSTOMER_FE_ONLY=true`). The chart does not. | The backend serves the customer portal at `/customer`. |
| **Metrics listener off** | `/metrics` on the app port needs an admin login. The unauthenticated listener on `METRICS_PORT` (default 9090) only starts with `METRICS_ENABLED=true`. | Set `METRICS_ENABLED=true` in `backend.extraEnv` before using the scrape annotations below. |

### Example work-around values

For a release named `goatflow` (the chart's app Secret is then `goatflow-app` and the Valkey
Service is `goatflow-valkey`):

```yaml
ingress:
  enabled: false            # add your own Ingress to goatflow-backend:8080
backend:
  extraEnv:
    - name: JWT_SECRET
      valueFrom:
        secretKeyRef:
          name: goatflow-app
          key: app-secret-key
    - name: GOATFLOW_SECURE_KEY
      valueFrom:
        secretKeyRef:
          name: goatflow-secure-key   # create this Secret yourself
          key: key
    - name: GOATFLOW_VALKEY_HOST
      value: goatflow-valkey
    - name: GOATFLOW_VALKEY_PASSWORD
      value: changeme                 # match valkey.auth.aclConfig
    - name: DB_DRIVER                 # PostgreSQL only
      value: postgres
```

## Configuration

### Database Selection

The chart supports MySQL/MariaDB (default) or PostgreSQL:

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
    existingSecret: "goatflow-db-credentials"
```

The backend reads the user name and password from `existingSecret` with these keys:

| `database.type` | User key | Password key |
|-----------------|----------|--------------|
| `mysql` | `mysql-user` | `mysql-password` |
| `postgresql` | `postgres-user` | `postgres-password` |

### Public URL and Attachment Storage

```yaml
config:
  # BASE_URL. Password-reset and customer sign-up emails link to it.
  # While it is empty, those emails are not sent.
  baseUrl: "https://helpdesk.example.com"
  storage:
    type: db            # db (attachments in the database) or fs
    path: /data/storage
```

`config.storage.type: fs` needs a persistent volume at `<path>/var/article`, which the chart does
not create. Keep `db` unless you add one. See [docs/ARTICLE_STORAGE.md](../../docs/ARTICLE_STORAGE.md).

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

### LDAP / Active Directory Login

Agents can log in with directory credentials. Every `config.ldap.*` value maps to an `LDAP_*`
variable documented in [docs/LDAP.md](../../docs/LDAP.md); empty values keep GoatFlow's defaults.
The backend refuses to start when the LDAP settings are invalid.

```yaml
config:
  authProviders: ["ldap", "database"]   # AUTH_PROVIDERS
  ldap:
    enabled: true
    type: active_directory
    host: dc01.corp.example.com
    useTLS: false
    useSSL: true
    caCert: |                           # mounted, passed as LDAP_TLS_CA_FILE
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    bindDN: CN=svc-goatflow,OU=Service Accounts,DC=corp,DC=example,DC=com
    existingSecret: goatflow-ldap       # key: ldap-bind-password
    baseDN: DC=corp,DC=example,DC=com
    domain: corp.example.com
    groupBaseDN: OU=Groups,DC=corp,DC=example,DC=com
    agentGroups: ["GoatFlow Agents"]
    adminGroups: ["GoatFlow Admins"]
    autoCreateUsers: true
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
```

**Note:** Strings containing `{{` must be quoted in YAML.

## Values Reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.commonAnnotations` | Annotations applied to all resources | `{}` |
| `global.commonLabels` | Labels applied to all resources | `{}` |
| `backend.enabled` | Enable backend deployment | `true` |
| `backend.replicaCount` | Number of backend replicas | `2` |
| `backend.image.repository` | Backend image repository | `ghcr.io/goatkit/goatflow` |
| `backend.image.tag` | Backend image tag | `""` (uses appVersion) |
| `backend.podAnnotations` | Annotations for backend pods | `{}` |
| `backend.podLabels` | Labels for backend pods | `{}` |
| `backend.serviceAnnotations` | Annotations for backend service | `{}` |
| `frontend.enabled` | Enable frontend deployment | `true` |
| `frontend.replicaCount` | Number of frontend replicas | `2` |
| `frontend.podAnnotations` | Annotations for frontend pods | `{}` |
| `frontend.serviceAnnotations` | Annotations for frontend service | `{}` |
| `database.type` | Database type: `mysql` or `postgresql` | `mysql` |
| `database.external.enabled` | Use external database | `false` |
| `database.external.existingSecret` | Secret with the database user and password (keys in [Using External Database](#using-external-database)) | `""` |
| `serviceAccount.annotations` | ServiceAccount annotations (IRSA, WI) | `{}` |
| `valkey.enabled` | Deploy Valkey subchart | `true` |
| `ingress.enabled` | Enable ingress | `false` |
| `secrets.create` | Create database and app secrets from values | `true` |
| `config.logLevel` | Application log level (`LOG_LEVEL`) | `info` |
| `config.baseUrl` | Public URL (`BASE_URL`); reset and sign-up emails are not sent while empty | `""` |
| `config.session.lifetime` | Session lifetime in seconds (`SESSION_TIMEOUT`) | `28800` |
| `config.storage.type` | Attachment storage: `db` or `fs` (`STORAGE_TYPE`) | `db` |
| `config.storage.path` | Storage root for `fs` (`STORAGE_PATH`) | `/data/storage` |
| `config.authProviders` | Auth provider order (`AUTH_PROVIDERS`), e.g. `["ldap", "database"]` | `[]` (Config.yaml) |
| `config.ldap.enabled` | Enable LDAP agent login (`LDAP_ENABLED`) | `false` |
| `config.ldap.type` | `openldap`, `389ds` or `active_directory` (`LDAP_TYPE`) | `""` |
| `config.ldap.host` / `port` | Directory server (`LDAP_HOST`, `LDAP_PORT`); host is required when LDAP is enabled | `""` |
| `config.ldap.useTLS` / `useSSL` | StartTLS or LDAPS (`LDAP_USE_TLS`, `LDAP_USE_SSL`) | `true` / `false` |
| `config.ldap.caCert` | PEM CA certificate, mounted and passed as `LDAP_TLS_CA_FILE` | `""` |
| `config.ldap.skipTLSVerify` | Testing only: skip server certificate check | `false` |
| `config.ldap.timeout` | Seconds per connect or LDAP operation (`LDAP_TIMEOUT`) | `""` |
| `config.ldap.bindDN` | Service account for searches; empty = anonymous search | `""` |
| `config.ldap.bindPassword` | Bind password; the chart creates a Secret when `secrets.create` is true | `""` |
| `config.ldap.existingSecret` / `existingSecretKey` | Existing Secret and key holding the bind password | `""` / `ldap-bind-password` |
| `config.ldap.baseDN` | Search base (`LDAP_BASE_DN`); required when LDAP is enabled | `""` |
| `config.ldap.userBaseDN` / `userFilter` | User search base and filter (filter has exactly one `%s`) | `""` |
| `config.ldap.isActiveDirectory` / `domain` | Active Directory mode and `<login>@<domain>` matching | `""` |
| `config.ldap.*Attribute` | Attribute names: `usernameAttribute`, `emailAttribute`, `firstNameAttribute`, `lastNameAttribute`, `displayNameAttribute`, `groupAttribute` | `""` |
| `config.ldap.groupBaseDN` / `groupFilter` / `groupMemberValue` | Group search (`groupMemberValue`: `dn` or `username`) | `""` |
| `config.ldap.agentGroups` | Only members of these groups (or `adminGroups`) may log in | `[]` |
| `config.ldap.adminGroups` | GoatFlow `admin` group membership follows these groups | `[]` |
| `config.ldap.autoCreateUsers` / `autoUpdateUsers` | Create the agent on first login / sync name and email on each login | `false` |
| `config.ldap.initialGroups` | GoatFlow groups for auto-created agents | `[]` |
| `backend.extraEnv` | Extra environment variables for the backend container | `[]` |
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

The backend runs as UID 1000, the nginx frontend as UID 101 and MariaDB as UID 999, all with
`runAsNonRoot: true`. The PostgreSQL StatefulSet sets only `fsGroup: 999` and runs as the image's
default user.

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
