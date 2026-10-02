# Kubernetes Deployment Guide

GoatFlow ships a Helm chart at `charts/goatflow/`. Full settings:
[charts/goatflow/README.md](../../charts/goatflow/README.md).

## Known limitations (0.10.0 chart)

A default install of the 0.10.0 chart does not give a working GoatFlow. Read the
[full list with work-arounds](../../charts/goatflow/README.md#known-limitations-0100-chart)
first. In short:

| Limitation | Effect |
|------------|--------|
| No background runner | No outgoing email (the `mail_queue` is never sent), no webhook delivery, no session cleanup. |
| Ingress goes to the nginx frontend | nginx proxies only `/api/` and `/ws` to the backend, so `/login`, `/customer` and `/admin` do not work through the Ingress. |
| `GOATFLOW_SECURE_KEY` not set | Each pod makes its own random key; encrypted plugin and webhook secrets cannot be read after a restart or by another replica. |
| Environment names GoatFlow does not read | `DB_TYPE` (needs `DB_DRIVER`), `APP_SECRET` (needs `JWT_SECRET`), `REDIS_*` / `CACHE_ENABLED` (needs `GOATFLOW_VALKEY_*`), `SERVER_PORT`, `SESSION_TIMEOUT`. |
| `backend-config` ConfigMap not mounted | `config.storage.*` has no effect. |

For a working install today, use Docker Compose ([docker.md](docker.md)) or the TrueNAS app,
which both run the runner.

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

The image is `ghcr.io/goatkit/goatflow`. By default its tag is the chart's `appVersion`
(`0.10.0`); set `backend.image.tag` to override it.

## Settings to check

| Value | Why |
|-------|-----|
| `config.baseUrl` | Public URL (`BASE_URL`). Password-reset and customer sign-up emails are not sent while it is empty. |
| `database.*.password`, `secrets.appSecretKey` | Empty values are generated again on every `helm upgrade`. Set them, or use existing Secrets. |
| `config.ldap.*`, `config.authProviders` | LDAP / Active Directory agent login. See [docs/LDAP.md](../LDAP.md). |

## Health and metrics

- Liveness and readiness probes call `GET /health` on port 8080. It pings the database and
  answers 503 when the database is unreachable.
- `GET /health/detailed` and `GET /metrics` on port 8080 need an admin login.
- For Prometheus, set `METRICS_ENABLED=true` (with `backend.extraEnv`). The backend then serves
  `/metrics` without login on `METRICS_PORT` (default 9090).

## See Also

- [Helm chart README](../../charts/goatflow/README.md)
- [Docker Deployment](docker.md)
- [Architecture Overview](../ARCHITECTURE.md)
