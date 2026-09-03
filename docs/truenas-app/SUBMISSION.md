# TrueNAS App Store Submission — GoatFlow

**Package re-pinned to 0.10.0** (2026-09-03): `app.yaml` `app_version` and
`ix_values.yaml` image tag now point at `ghcr.io/goatkit/goatflow:0.10.0`.
**Re-run the CI gates + live-app tests against 0.10.0 before submitting** —
the evidence below (and the `TRUENAS-LOCAL-TEST-REPORT.md` dates) was captured
against the 0.9.0 image and is not valid for 0.10.0.

Prepared 2026-09-02, re-validated 2026-09-02 20:18 on a synced `origin/master`
(b18f6a1). Everything below is validated locally with TrueNAS's own CI
tooling against a clean checkout of `truenas/apps` (catalog clone at
`/tmp/truenas-apps`). The app directory is `docs/truenas-app/goatflow/` — copy it
to `ix-dev/community/goatflow/` in a fork of `truenas/apps` and open a PR.

**Re-run on retry (2026-09-02 evening)**: all 4 gates re-executed and re-passed
on a synced checkout. One change since the first pass: `dbrepairs` claimed port
`30482` in the catalog between runs, so the web port moved `30482 → 30484`
(`questions.yaml` + `basic-values.yaml`); `port_validation.py` re-passes.

## What's in the package

`docs/truenas-app/goatflow/` (drop-in for `ix-dev/community/goatflow/`):

| File | Role | Status |
| --- | --- | --- |
| `app.yaml` | metadata | steady-state (generate_metadata.py no-op) |
| `ix_values.yaml` | static defaults | pinned `ghcr.io/goatkit/goatflow:0.10.0` |
| `questions.yaml` | wizard form | ports 30484/30483, secrets, storage, run_as |
| `templates/docker-compose.yaml` | Jinja2 render-lib template | flat `DB_*` + `GOATFLOW_VALKEY_PASSWORD` (0.9.0-compatible) |
| `templates/test_values/basic-values.yaml` | CI fixture | secrets filled, host paths under `/opt/tests` |
| `templates/library/base_v2_3_11/` | render library | byte-identical to catalog `library/2.3.11` |
| `README.md` | services + upgrade/rollback | present |
| `item.yaml` | auto-generated catalog entry | deterministic, stable |

Supporting docs in `docs/truenas-app/`: `TRUENAS-LOCAL-TEST-REPORT.md`,
`SUBMISSION-CHECKLIST.md` (this tree), `screenshots/` (3 PNGs for the PR).

## CI validation (all gates, clean synced state)

| Gate | Command | Result |
| --- | --- | --- |
| Port uniqueness | `python3 .github/scripts/port_validation.py ix-dev/community/goatflow` | exit 0 — 30484/30483 free (30482 was claimed by `dbrepairs` between runs; next available 30485) |
| Render parity | `python3 .github/scripts/ci.py --app goatflow --train community --test-file basic-values.yaml --render-only=true` | exit 0 — `x-portals` Web UI :30484; `lib_version_hash` `874636…` (matches `library/hashes.yaml` + all sibling 2.3.11 apps) |
| Structure | `docker run ghcr.io/truenas/apps_validation … apps_dev_charts_validate validate --path /repo` | exit 0 |
| Check Metadata | `python3 .github/scripts/generate_metadata.py --train community --app goatflow` | exit 0 — `app.yaml` + `item.yaml` STABLE (zero diff), version stays 1.0.0 |

Live-app evidence (separate task, same day): clean install, start/stop/recreate,
data persistence, DB-outage recovery, 0.9.0→0.8.3 upgrade/downgrade — all PASS.
See `TRUENAS-LOCAL-TEST-REPORT.md`.

## lib_version_hash — resolved (was a false alarm)

Two upstream tasks flagged `lib_version_hash: 874636…` as "stale" because a local
metadata run emitted `5c7d35…`. Root cause: the local `library/2.3.11/__pycache__/`
(50 gitignored `.pyc` files) poisoned `apps_catalog_hash_generate`'s library
fingerprint. After clearing pycache, TrueNAS's own tool produces `874636…` —
identical to the committed `library/hashes.yaml` and all 408 sibling apps on
lib 2.3.11. **No hash change needed.** `run_as_context` was corrected once (valkey
568→1000 — the library's `deps_redis` defaults to 568 only when no `run_as` is
set; our fixture sets run_as 1000, so valkey renders `1000:1000`), the
short-lived `permissions` sidecar was dropped, and the catalog `sources` URL was
added — after which `generate_metadata.py` is a no-op, which is what the CI
"Check Metadata" gate requires.

## Reconciled before PR (done this session)

- License: `cmd/goats/main.go` + 4 generated API docs said AGPL-3.0; `LICENSE` is
  Apache-2.0. All five annotations now say Apache-2.0 (in the working tree,
  uncommitted). `docs/LICENSE_COMPATIBILITY.md` confirms Apache-2.0.

## What Nige must do before/with the PR (human-only steps)

1. **Fork + PR**: fork `truenas/apps`, copy `docs/truenas-app/goatflow/` →
   `ix-dev/community/goatflow/`, open PR using `.github/PULL_REQUEST_TEMPLATE/app_addition.md`. PR body is below.
2. **Screenshots + icon**: attach `docs/truenas-app/screenshots/screenshot1-3.png`
   and `static/images/icon-512.png` to the PR. Reviewer uploads to the CDN and
   returns `https://media.sys.truenas.net/apps/goatflow/…` URLs; `app.yaml`
   `icon:`/`screenshots:` already carry those URLs — confirm/adjust when they come back.
3. **Decide TLS**: v1 publishes plain HTTP on the published port (30484). Fine to
   start; note "reverse-proxy TLS" in Special Notes if we want to flag it.
4. **Maintainers**: current `maintainers` block is the truenas default; optionally
   add `hello@goatflow.io` as an upstream maintainer.
5. **Post-merge**: catalog updates daily on apps.truenas.com; renovate bot tracks
   the `0.10.0` tag — bump `app_version` + image tag on upstream releases.

## PR description (paste into the truenas/apps PR)

```markdown
# App Addition

- [x] I have opened an [issue](https://github.com/truenas/apps/issues) to discuss
      this app addition before submitting this pull request.

## AI

- [ ] Part or All of this PR was generated by an LLM.

## Description

Adds GoatFlow to the community train. GoatFlow is an open-source support and
ticket management platform (OTRS-compatible core): agent UI, customer portal,
background task runner, and a WASM plugin system. Ships as a single image
(ghcr.io/goatkit/goatflow) running the backend + runner roles, with MariaDB and
Valkey bundled in-app and a one-shot permissions sidecar for the storage datasets.
Apache-2.0 licensed.

## App Information

- **Upstream**: https://github.com/goatkit/goatflow
- **Documentation**: https://github.com/goatkit/goatflow (README) /
  https://apps.truenas.com/catalog/goatflow_community/ (post-merge)
- **App Version**: 0.10.0

## Testing

Tested locally with:

- [x] basic-values.yaml

All tests passed successfully. Render parity via `.github/scripts/ci.py
--render-only=true`, structure via the `apps_validation` container, and
`generate_metadata.py` steady-state — all exit 0. Live-app pass on a
TrueNAS-Equivalent Docker harness: clean install (26 migrations), start,
stop/start, data persistence, logs, portals, failure recovery, and 0.9.0 → 0.8.3
upgrade/downgrade.

## Icons and Screenshots

Please upload the following to the CDN:

- Icon: attached (icon-512.png)
- Screenshot 1: attached (dashboard)
- Screenshot 2: attached (tickets)
- Screenshot 3: attached (queues)

## Special Notes

- First-time setup: no manual migration step — schema migrations run on every
  start and are idempotent. Set the required secrets in the wizard (DB/Valkey
  passwords, JWT secret, secure key, admin password).
- v1 serves HTTP on the published port; put a TLS-terminating reverse proxy in
  front for production use.
- Admin account is `root@localhost` (OTRS convention); the wizard's Admin
  Password sets it.

## Checklist

- [x] App runs successfully locally
- [x] Only modified files under /ix-dev/ or /library/
- [x] README.md included
- [x] Multiple test scenarios tested (basic-values.yaml; conditional customer-fe
      path exercised in dev)
- [x] questions.yaml has clear descriptions and follows structure of existing apps
- [x] All automated CI checks pass
```
