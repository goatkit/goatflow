# GoatFlow Helm Chart

Helm chart for deploying GoatFlow on Kubernetes.

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
| Backend Deployment | `ghcr.io/goatkit/goatflow` | Agent UI, customer portal (`/customer`), REST API. Runs migrations at start (replicas take turns through a database lock). Also runs the in-process scheduler (email polling, escalation checks, pending reminders, auto close, GenericAgent jobs). Probes use `GET /health`, which pings the database. |
| Runner Deployment | `ghcr.io/goatkit/goatflow-runner` | `./goats -mode runner`: sends the outgoing email queue, delivers webhooks, evaluates ticket notification rules, cleans up expired sessions. Same database, secrets and email settings as the backend; no Service. `runner.enabled`, one replica. |
| Database StatefulSet | `mariadb:11` or `postgres:16-alpine` | Skipped when `database.external.enabled` is true. Backend and runner wait for it (`busybox` init container). |
| Valkey | valkey-helm subchart | Redis-compatible cache, Service `<release>-valkey`. |
| Ingress | — | Optional (`ingress.enabled`). Every path goes to the backend Service. |
| Metrics Service | — | Optional (`metrics.enabled`): `<fullname>-metrics` on port 9090. |
| Storage PVC | — | `<fullname>-storage`, mounted on the backend at `config.storage.path`: fs attachments and plugin files. Kept on `helm uninstall`. |

The chart has no separate customer-only instance (Docker Compose and TrueNAS can run one with
`CUSTOMER_FE_ONLY=true`); the backend serves the customer portal at `/customer`.

### Secrets

With `secrets.create: true` (default) the chart creates two Secrets. Values left empty are
generated on install and kept on every `helm upgrade` (the chart reads the live Secret). Both
carry `helm.sh/resource-policy: keep`, so `helm uninstall` leaves them in place with the
volumes, and a reinstall under the same release name and namespace reuses them:

| Secret | Keys |
|--------|------|
| `<fullname>-database` | `mysql-root-password`, `mysql-user`, `mysql-password` (or `postgres-user`, `postgres-password`) |
| `<fullname>-app` | `app-secret-key` (`JWT_SECRET`), `secure-key` (`GOATFLOW_SECURE_KEY`, 64 hex characters), `admin-password` (`GOATFLOW_ADMIN_PASSWORD`), `smtp-password` when `config.email.smtp.password` is set |

`GOATFLOW_SECURE_KEY` encrypts stored webhook signing secrets, identity provider secrets (OIDC
client secret, SAML private key) and plugin secure settings. The backend and the runner read
the same key. Back it up and never change it: stored secrets cannot be decrypted with a
different key. To bring your own, set `secrets.secureKey` (`openssl rand -hex 32`) or point
`secrets.existingSecret` at a Secret with the keys above.

Generated values need a live cluster to read back: `helm template` (and GitOps tools that
render with it, such as Argo CD) cannot look up the existing Secret and would generate new
values on every render. In that setup set `secrets.existingSecret`,
`database.mysql.existingSecret` / `database.postgresql.existingSecret`, or the values
themselves.

### First login

The seeded admin account `root@localhost` is disabled until the first backend start applies
`admin-password` to it (once; later changes to the Secret are ignored). Read it with:

```bash
kubectl get secret <fullname>-app -o jsonpath='{.data.admin-password}' | base64 -d
```

Set `secrets.adminPassword` to choose it yourself. Change it in GoatFlow after the first login.

The backend runs with `APP_ENV=production`, so its login cookies are `Secure`: serve GoatFlow
over HTTPS (Ingress TLS or a TLS-terminating proxy), or browsers drop the session cookie.

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

### Public URL and File Storage

```yaml
config:
  # BASE_URL. Password-reset and customer sign-up emails link to it.
  # While it is empty, those emails are not sent.
  baseUrl: "https://helpdesk.example.com"
  storage:
    type: db            # db (attachments in the database) or fs
    path: /data/storage # STORAGE_PATH
    persistence:
      enabled: true
      existingClaim: "" # use your own PVC instead of <fullname>-storage
      size: 10Gi
      storageClass: ""  # global.storageClass when empty
      accessModes: [ReadWriteOnce]
```

The backend's root filesystem is read-only, so `config.storage.path` is a volume: a PVC
(`<fullname>-storage`, 10Gi, ReadWriteOnce) by default. It holds plugin files (`<path>/plugins`)
and, with `type: fs`, article attachments (`<path>/var/article`, see
[docs/ARTICLE_STORAGE.md](../../docs/ARTICLE_STORAGE.md)). The runner does not use it.

- The PVC carries `helm.sh/resource-policy: keep`, like the database volume, so `helm uninstall`
  leaves it; delete it yourself to drop the files.
- A ReadWriteOnce volume attaches to one node. With more than one backend replica
  (`backend.replicaCount`, default 2, or autoscaling) on several nodes, use a ReadWriteMany
  storage class (NFS, CephFS, EFS, Azure Files) and set `accessModes: [ReadWriteMany]`;
  otherwise pods on other nodes stay in `ContainerCreating` (Multi-Attach error).
- `persistence.enabled: false` mounts an emptyDir instead: plugin files are lost when the pod
  restarts, and `type: fs` is refused.

### Valkey (Redis-compatible Cache)

The chart uses the official [valkey-helm](https://github.com/valkey-io/valkey-helm) subchart.
GoatFlow connects to the `<release>-valkey` Service (`GOATFLOW_VALKEY_HOST`, `GOATFLOW_VALKEY_PORT`).
Authentication is off by default (Valkey is only reachable inside the cluster). To require a
password, define the `default` ACL user; the backend and runner read its password from the
subchart Secret `<release>-valkey-auth` (key `default-password`):

```yaml
valkey:
  enabled: true
  auth:
    enabled: true
    aclUsers:
      default:
        permissions: "~* &* +@all"
        password: "a-long-random-password"
    # Or keep the password in your own Secret:
    # usersExistingSecret: valkey-users     # key: aclUsers.default.passwordKey (default "default")
```

`auth.aclConfig` alone is not supported: GoatFlow needs the `default` user's password.

For external Redis/Valkey (ElastiCache, etc.):

```yaml
valkey:
  enabled: false

externalValkey:
  enabled: true
  host: "your-elasticache-endpoint"
  port: 6379
  existingSecret: "goatflow-valkey-credentials"   # key: existingSecretPasswordKey (valkey-password)
  # or: password: "..." (the chart stores it in <fullname>-valkey-external)
```

### Outgoing Email

The runner sends the `mail_queue` (ticket emails, password-reset and sign-up links); the backend
sends a few messages (for example two-factor codes) directly. Both get these settings as
`GOATFLOW_EMAIL_*`:

```yaml
config:
  baseUrl: "https://helpdesk.example.com"   # links in reset / sign-up emails
  email:
    enabled: true
    from: helpdesk@example.com
    fromName: "Example Helpdesk"
    smtp:
      host: smtp.example.com
      port: 587
      username: helpdesk@example.com
      existingSecret: goatflow-smtp       # key: existingSecretKey (smtp-password)
      # or: password: "..." (stored in the app Secret)
      startTLS: true
      authType: plain                     # plain or login
```

While `config.email.enabled` is false, mail stays in the `mail_queue` table.

### Metrics

```yaml
metrics:
  enabled: true      # METRICS_ENABLED=true, METRICS_PORT=9090 on the backend
  port: 9090
  service:
    annotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "9090"
```

This adds an unauthenticated `/metrics` listener on its own port and the Service
`<fullname>-metrics`. `/metrics` on the app port always exists but needs an admin login.

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

# Prometheus scraping (pod annotations; needs metrics.enabled)
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
| `backend.terminationGracePeriodSeconds` | Stop grace period (keep >= `DRAIN_TIMEOUT` + 40s) | `45` |
| `runner.enabled` | Deploy the background runner | `true` |
| `runner.image.repository` | Runner image repository | `ghcr.io/goatkit/goatflow-runner` |
| `runner.image.tag` | Runner image tag | `""` (uses appVersion) |
| `runner.replicaCount` | Runner replicas: 0 or 1 (runner tasks take no lock; more is refused) | `1` |
| `runner.extraEnv` | Extra environment variables for the runner | `[]` |
| `metrics.enabled` | Prometheus listener + `<fullname>-metrics` Service | `false` |
| `metrics.port` | Metrics port (`METRICS_PORT`) | `9090` |
| `database.type` | Database type: `mysql` or `postgresql` | `mysql` |
| `database.external.enabled` | Use external database | `false` |
| `database.external.existingSecret` | Secret with the database user and password (keys in [Using External Database](#using-external-database)) | `""` |
| `serviceAccount.annotations` | ServiceAccount annotations (IRSA, WI) | `{}` |
| `valkey.enabled` | Deploy Valkey subchart | `true` |
| `ingress.enabled` | Enable ingress | `false` |
| `secrets.create` | Create database and app secrets (empty values generated once, kept on upgrade) | `true` |
| `secrets.existingSecret` | Existing app Secret (keys in [Secrets](#secrets)) | `""` |
| `secrets.appSecretKey` | `JWT_SECRET`, at least 32 characters | generated |
| `secrets.secureKey` | `GOATFLOW_SECURE_KEY`, exactly 64 hex characters | generated |
| `secrets.adminPassword` | First-boot password of `root@localhost` | generated |
| `valkey.auth.enabled` | Valkey ACL auth (needs `valkey.auth.aclUsers.default`) | `false` |
| `externalValkey.*` | External Valkey/Redis host, port, password or `existingSecret` | disabled |
| `config.email.*` | Outgoing email (`GOATFLOW_EMAIL_*`), see [Outgoing Email](#outgoing-email) | disabled |
| `config.logLevel` | Application log level (`LOG_LEVEL`) | `info` |
| `config.baseUrl` | Public URL (`BASE_URL`); reset and sign-up emails are not sent while empty | `""` |
| `config.session.lifetime` | Maximum session lifetime in seconds (`GOATFLOW_AUTH_SESSION_SESSIONMAXTIME`) | `28800` |
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

The backend and runner run as UID 1000 with a read-only root filesystem, MariaDB as UID 999,
all with `runAsNonRoot: true`. The PostgreSQL StatefulSet sets only `fsGroup: 999` and runs as the image's
default user.

## Upgrading

```bash
helm upgrade goatflow ./charts/goatflow
```

Generated passwords and keys are read back from the existing Secrets, so an upgrade does not
change them.

## Uninstalling

```bash
helm uninstall goatflow
```

**Note**: the PVCs and the generated Secrets (`<fullname>-database`, `<fullname>-app`) are kept
(`helm.sh/resource-policy: keep`): a reinstall finds its data and the passwords and keys that
open it. To remove everything, including the data:

```bash
kubectl delete pvc -l app.kubernetes.io/instance=goatflow
kubectl delete secret goatflow-database goatflow-app
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
