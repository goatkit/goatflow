# GoatFlow Quick Start Guide

This guide gets you from `git clone` → running GoatFlow locally → creating your first ticket.

## Prerequisites

- Container runtime: Docker (Compose v2) or Podman (Compose)
- `make`
- Modern browser

## 1) Start GoatFlow (local dev)

```bash
git clone https://github.com/goatkit/goatflow.git
cd goatflow

# Local dev (safe demo credentials; matches compose defaults)
cp .env.development .env

make up
```

Useful commands:

```bash
make up-d
make backend-logs
make down
```

## 2) Where to access things

- App UI: `http://localhost` (served via the frontend container)
- API (same origin): `http://localhost/api`
- smtp4dev (email sandbox UI): `http://localhost:8025`
- Adminer (DB UI, if enabled): `http://localhost:8090`

## 3) Login

- For local dev, demo credentials come from `.env.development` (copied into `.env`).
- If `DEMO_MODE=true`, the login page shows demo accounts.
- To use **real database users** instead of demo mode, run `make synthesize` to generate secure test credentials (see [DATABASE.md](../development/DATABASE.md#test-data-generation)).

## 4) Create your first ticket (UI)

1. Login as an agent/admin.
2. Navigate to ticket creation.
3. Submit a new ticket.
4. Confirm it redirects to the ticket zoom and the number is shown.

## 5) Check the API is up

Use the built-in helper so URLs stay consistent with your `.env`:

```bash
make api-call ENDPOINT=/api/v1/status
```

## 6) Sanity checks

Run the curated Go tests inside the toolbox container:

```bash
make test
```

## Troubleshooting

- Port conflicts: set `BACKEND_PORT`/`ADMINER_PORT` in `.env` and re-run `make restart`.
- DB issues: use `make db-query QUERY="SELECT 1"` (non-interactive).

## Next docs

- Deployment: [docs/deployment/docker.md](../deployment/docker.md)
- Configuration: [docs/configuration.md](../configuration.md)
- Demo instance: [docs/DEMO.md](../DEMO.md)
- Database notes: [docs/development/DATABASE.md](../development/DATABASE.md)