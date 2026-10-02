# Kubernetes Deployment Guide

GoatFlow ships a Helm chart at `charts/goatflow/`. Full settings:
[charts/goatflow/README.md](../../charts/goatflow/README.md).

The chart deploys the backend (agent UI, customer portal, API), the background runner
(outgoing email, webhooks, notification rules, session cleanup), MariaDB or PostgreSQL, Valkey,
and optionally an Ingress and a metrics Service.

## Install

The chart is published to the GitHub container registry. A release tag `v0.10.0` publishes chart
version `0.10.0`; pushes to `main` publish `0.1.0-main`.

```bash
# MySQL/MariaDB (default)
helm install goatflow oci://ghcr.io/goatkit/charts/goatflow --version 0.10.0 \
  --namespace goatflow --create-namespace

# PostgreSQL
helm install goatflow oci://ghcr.io/goatkit/charts/goatflow --version 0.10.0 \
  --namespace goatflow --create-namespace \
  --set database.type=postgresql
```

From a checkout of the repository:

```bash
git clone https://github.com/goatkit/goatflow.git
cd goatflow
helm dependency update charts/goatflow
helm install goatflow ./charts/goatflow --namespace goatflow --create-namespace
# PostgreSQL: add -f charts/goatflow/values-postgresql.yaml
```

The images are `ghcr.io/goatkit/goatflow` (backend) and `ghcr.io/goatkit/goatflow-runner`
(runner). By default their tag is the chart's `appVersion` (`0.10.0`); set `backend.image.tag`
and `runner.image.tag` to override it.

## First login

Log in as `root@localhost`. The chart generates the first-boot password (or uses
`secrets.adminPassword`):

```bash
kubectl -n goatflow get secret goatflow-app -o jsonpath='{.data.admin-password}' | base64 -d
```

The backend sets `Secure` login cookies, so serve it over HTTPS (Ingress with TLS).

## Settings to check

| Value | Why |
|-------|-----|
| `ingress.*` | Every path goes to the backend Service. Add `tls` for HTTPS. |
| `config.baseUrl` | Public URL (`BASE_URL`). Password-reset and customer sign-up emails are not sent while it is empty. |
| `config.email.*` | SMTP for the runner and the backend. While disabled, mail stays in the `mail_queue` table. |
| `secrets.*` | Database passwords, `JWT_SECRET`, `GOATFLOW_SECURE_KEY` and the admin password are generated on install and kept on upgrade. Back up the `<fullname>-app` Secret: stored webhook and plugin secrets cannot be decrypted without its `secure-key`. |
| `config.ldap.*`, `config.authProviders` | LDAP / Active Directory agent login. See [docs/LDAP.md](../LDAP.md). |

## Health and metrics

- Liveness and readiness probes call `GET /health` on port 8080. It pings the database and
  answers 503 when the database is unreachable.
- `GET /health/detailed` and `GET /metrics` on port 8080 need an admin login.
- For Prometheus, set `metrics.enabled=true`. The backend then serves `/metrics` without login
  on port 9090 (`metrics.port`), exposed by the Service `<fullname>-metrics`.

## See Also

- [Helm chart README](../../charts/goatflow/README.md)
- [Docker Deployment](docker.md)
- [Architecture Overview](../ARCHITECTURE.md)
