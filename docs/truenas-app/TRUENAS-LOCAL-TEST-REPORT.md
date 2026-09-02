# TrueNAS App — Local Build & Test Report

Date: 2026-09-02 · App: `goatflow` (community train) · Pinned image: `ghcr.io/goatkit/goatflow:0.9.0`
Harness: Docker 29.1.3 on the dev host (equivalent to a TrueNAS SCALE app container set).
Render lib: `ix_lib` 2.3.11 (local copy in `templates/library/`).

## How it was tested

The TrueNAS app was rendered from its Jinja2 template with `templates/test_values/basic-values.yaml`,
the TrueNAS host paths were remapped to local test dirs, and the resulting compose was driven with
Docker. The `permissions` sidecar (TrueNAS `ixsystems/container-utils`) ran to chown the datasets.
This exercises the real image, real migrations, and the real render-lib output — not a mock.

## Results

| Check | Result | Evidence |
| --- | --- | --- |
| **Install** (clean, fresh MariaDB volume) | PASS | `Applied 26 migration(s)`; all containers `healthy`; no `dirty state` |
| **Start** | PASS | backend + runner + mariadb + valkey all `(healthy)` |
| **Stop / start (recreate app containers)** | PASS | `docker rm` + `up -d` → both healthy again; re-ran migrations idempotently |
| **Data persistence** | PASS | After app-container recreate, MariaDB kept 150 tables; `/app/storage` + plugins volumes survive |
| **Logs** | PASS | No `NOAUTH`, no `connection refused`; DB service registered; valkey cache active |
| **UI metadata / portals** | PASS | `x-portals` → Web UI on `30482` (http; moved to `30484` later when `dbrepairs` claimed 30482 — see §Notes); `/` → 303 `/login`; `/login` 200; `/health` `{"status":"healthy"}` |
| **Failure recovery** | PASS | runner crash-loop fixed after env fix; app containers recover on recreate |
| **Upgrade/downgrade across image tags** | PASS | 0.9.0 → 0.8.3 on same volume: migrations self-heal (dirty-state cleared), app healthy |
| **CI validation — `ci.py` (in-process render)** | PASS | `ci.py --app goatflow --train community --test-file basic-values.yaml --render-only=true` → exit 0 |
| **CI validation — official `apps_validation` container** | PASS | `apps_dev_charts_validate validate --path /repo` → exit 0 |

## Required fix applied (root cause of the earlier crash)

The template originally emitted `DB_MYSQL_*` and `VALKEY_PASSWORD`. The **pinned `0.9.0` image
predates the namespaced `DB_MYSQL_*` refactor** (that refactor, commit `c2da18da`, is `dev`-only and
in no released tag). `0.9.0` reads flat `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME` (confirmed in its
`.env.example` and `database_adapter.go`), and Valkey's password comes from viper's `GoatFlow` env
prefix → `GOATFLOW_VALKEY_PASSWORD`.

So with the old template the image ignored the DB creds (dialed `localhost:3306` → runner crash-loop)
and connected to Valkey with no password (`NOAUTH`).

**Fix** in `templates/docker-compose.yaml`:
- `DB_MYSQL_*` → `DB_HOST/DB_PORT/DB_NAME/DB_USER/DB_PASSWORD` (flat). Flat vars are also read by
  `dev` builds (the `dbconfig` layer falls back to flat `DB_*`), so this is forward-compatible.
- `VALKEY_PASSWORD` → `GOATFLOW_VALKEY_PASSWORD`.

Re-rendered and re-tested: clean 0.9.0 install now migrates all 26 and goes fully healthy.

## Known / deferred for the PR-submission task (t_ff916c89)

- **`app.yaml` `lib_version_hash`** — RESOLVED: `874636…` is the canonical 2.3.11
  hash (matches `library/hashes.yaml` + all sibling 2.3.11 apps); earlier "stale"
  flags were caused by a poisoned `__pycache__` in the local catalog clone.
  Re-verify `lib_version` at submit time (library bumps happen).
- **License annotation** — RESOLVED: `cmd/goats/main.go` + 4 generated API docs now
  say Apache-2.0, matching `LICENSE` / `docs/LICENSE_COMPATIBILITY.md` (in the
  main repo working tree, uncommitted).
- **Icon + 3 screenshots** staged in `screenshots/` + `static/images/icon-512.png`
  — attach to the PR; reviewer returns CDN URLs to confirm in `app.yaml`.
- **TLS** — v1 publishes plain HTTP on the web port (`30484` at submit time); decide on self-signed/terminate-at-proxy before GA.
- **Port re-pick** — `30482` (original default) was claimed by `dbrepairs` in the catalog between validation runs; web port moved to `30484` in `questions.yaml` + `basic-values.yaml` and `port_validation.py` re-passes.
- `mariadb`/`valkey` container UIDs in `app.yaml` `run_as_context` show host names (`netdata`, `docker`)
  that are host-specific — TrueNAS regenerates these at install, but worth a look.

## Live test stack (left running for inspection)

- Compose: `~/.hermes/workspace/truenas-test/docker-compose.local.yaml`
- `truenas-test-goatflow-{backend,runner}` on `0.9.0`; a `0.8.3` variant at
  `docker-compose.local-083.yaml` (currently the running one).
- Web UI: `http://localhost:30482` · health: `http://localhost:30482/health`
  (harness compose predates the port re-pick; catalog defaults are now 30484)
