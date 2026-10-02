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

# Local dev settings (matches the compose defaults)
cp .env.development .env

make up-d
```

Useful commands:

```bash
make up-d
make backend-logs
make down
```

## 2) Where to access things

| What | URL |
|------|-----|
| Agent and admin UI (HTMX, served by the backend) | `http://localhost:8080` |
| API (same origin) | `http://localhost:8080/api` |
| Customer portal (a second backend container with `CUSTOMER_FE_ONLY=true`) | `http://localhost:8083/customer` |
| smtp4dev (email sandbox UI) | `http://localhost:8025` |
| Adminer (database UI, compose profile `tools`) | `http://localhost:8090` |

`make` picks host port 18080 for the backend when 8080 is already in use, unless you set
`BACKEND_PORT`. The customer portal port is `CUSTOMER_FE_PORT` (default 8083).

## 3) Login

The seeded admin `root@localhost` starts **disabled** (`valid_id = 2`) with a random password
nobody knows. `.env.development` does not change this. After the stack is up, run:

```bash
make reset-password
```

It asks for a username (`root@localhost`) and a new password, and enables the user.

`make synthesize` writes a new `.env` with generated secrets when no `.env` exists yet (see
[DATABASE.md](../development/DATABASE.md#generated-test-credentials)).

Demo mode (`app.demo_mode: true` or `GOATFLOW_APP_DEMO_MODE=true`) only stops non-admin users
changing passwords/MFA and keeps preference changes session-only. It never logs anyone in.

## 4) Create your first ticket (UI)

1. Log in as `root@localhost`.
2. Go to `/ticket/new` (or the new ticket button on the ticket list, `/tickets`).
3. Submit a new ticket.
4. Check that it opens the ticket zoom page and shows the ticket number.

## 5) Check the API is up

Use the built-in helper so URLs stay consistent with your `.env`:

```bash
make api-call ENDPOINT=/health
```

## 6) Sanity checks

Run the full test run (static checks, test stack, unit tests and browser tests, all in
containers). See [TESTING.md](../development/TESTING.md) for smaller targets.

```bash
make test
```

## Troubleshooting

- Port conflicts: set `BACKEND_PORT`/`ADMINER_PORT` in `.env` and re-run `make restart`.
- DB issues: use `make db-query QUERY="SELECT 1"` (non-interactive).

## Next docs

- Deployment: [docs/deployment/docker.md](../deployment/docker.md)
- Configuration: [docs/configuration.md](../configuration.md)
- Database notes: [docs/development/DATABASE.md](../development/DATABASE.md)