# TrueNAS App Store — Requirements Checklist (GoatFlow)

Cross-checked 2026-09-02 against `docs/truenas-app-catalog-requirements.md`
(task t_fd5702ef) and the `trueapps-catalog-packaging` skill. App dir:
`docs/truenas-app/goatflow/`. Train: `community`. Re-validated on 2026-09-02
(evening retry) on a synced `origin/master` — all gates below re-executed.
Note: web port `30482 → 30484` after `dbrepairs` claimed 30482 upstream.

## 1. Required files (per requirements §2)

| File | Required? | Present | Notes |
| --- | --- | --- | --- |
| `app.yaml` | yes | ✅ | steady-state; `generate_metadata.py` no-op |
| `ix_values.yaml` | yes | ✅ | image `ghcr.io/goatkit/goatflow:0.9.0`, consts |
| `questions.yaml` | yes | ✅ | 6 groups; secrets required/private; ports 30484/30483 |
| `templates/docker-compose.yaml` | yes | ✅ | Jinja2 render-lib; flat `DB_*` + `GOATFLOW_VALKEY_PASSWORD` |
| `README.md` | yes | ✅ | services table + upgrade/rollback section |
| `templates/test_values/basic-values.yaml` | yes | ✅ | CI fixture, secrets + `/opt/tests` host paths |
| `item.yaml` | auto-generated | ✅ | deterministic, stable, committed (as siblings do) |
| `templates/library/base_v2_3_11/` | needed for CI | ✅ | byte-identical to catalog `library/2.3.11` |
| `app_migrations.yaml` | optional | n/a | no breaking config changes yet |

## 2. app.yaml fields (per requirements §3)

| Field | Value | OK |
| --- | --- | --- |
| `app_version` | 0.9.0 (== image tag) | ✅ |
| `capabilities` | [] (none required) | ✅ |
| `categories` | [productivity] (single) | ✅ |
| `date_added` | 2026-09-02 | ✅ |
| `description` | one-liner | ✅ |
| `home` | github.com/goatkit/goatflow | ✅ |
| `host_mounts` | [] (none) | ✅ |
| `icon` | media.sys.truenas.net/…/icon.png | ✅ placeholder; reviewer confirms CDN |
| `lib_version` | 2.3.11 | ✅ re-check at submit time (library bumps) |
| `lib_version_hash` | 874636… (matches hashes.yaml + 408 siblings) | ✅ verified correct, not stale |
| `maintainers` | truenas default | ✅ add `hello@goatflow.io` if desired |
| `name` | goatflow (== dir, lowercase) | ✅ |
| `run_as_context` | 4 always-running containers | ✅ permissions sidecar excluded (short-lived) |
| `screenshots` | 1 placeholder URL | ✅ attach 3 in PR; reviewer returns CDN URLs |
| `sources` | includes apps.truenas.com catalog URL | ✅ auto-added, stable |
| `train` | community | ✅ |
| `version` | 1.0.0 | ✅ stable fixed point |

## 3. Compose / templating (per requirements §5)

- ✅ `tpl.add_container`, `.deps.mariadb`, `.deps.redis` (valkey), `.deps.perms`
- ✅ `tpl.portals.add` → Web UI 30484 (confirmed in rendered `x-portals`)
- ✅ health checks use in-image binaries (`wget` for backend, `mariadb-admin` for
  db, `redis-cli` for valkey, `pidof goats` for runner) — all present in the images
- ✅ non-root via `.set_user(run_as.user, run_as.group)`; `cap_drop: [ALL]` +
  `security_opt: no-new-privileges` on all app containers
- ✅ image keys end in `image`; ghcr preferred

## 4. Ports (per requirements §6)

- ✅ `port_validation.py` exit 0; 30484/30483 don't collide with any catalog app
  (minio 30000 was the original clash — fixed in t_436cbb6c; 30482 was then
  claimed by `dbrepairs`, so web port moved to 30484 on 2026-09-02)

## 5. Licensing & support (per requirements §7)

- ✅ Catalog files governed by repo LGPL-3.0; app code is Apache-2.0 (LICENSE).
- ✅ Upstream image public/pullable: `ghcr.io/goatkit/goatflow`.
- ✅ License discrepancy resolved: `cmd/goats/main.go` + 4 generated API docs
  now say Apache-2.0 (were AGPL-3.0). `docs/LICENSE_COMPATIBILITY.md` confirms
  Apache-2.0 is the real license.
- ⚠️ Ongoing: community-train maintenance — bump `app_version`/image on upstream
  releases (renovate bot tracks the tag).

## 6. Icons / screenshots / UI metadata (per requirements §8)

- ✅ Icon source: `static/images/icon-512.png` (attach to PR; reviewer → CDN).
- ✅ 3 screenshots captured from the live app: `screenshots/screenshot1-dashboard.png`,
  `screenshot2-tickets.png`, `screenshot3-queues.png` (attach to PR; reviewer → CDN).
- ⚠️ `app.yaml` `icon:`/`screenshots:` carry the expected CDN URLs as placeholders —
  confirm against the URLs the reviewer returns after upload.

## 7. CI validation (per requirements §10.4 + skill)

| Check | Command | Result |
| --- | --- | --- |
| Render parity | `ci.py --app goatflow --train community --test-file basic-values.yaml --render-only=true` | ✅ exit 0 |
| Port collision | `port_validation.py ix-dev/community/goatflow` | ✅ exit 0 |
| Metadata regen | `generate_metadata.py --app goatflow --train community` | ✅ exit 0, zero diff |
| Official structure | `ghcr.io/truenas/apps_validation` → `apps_dev_charts_validate validate` | ✅ exit 0 |

All run from a clean synced `truenas/apps` checkout (catalog clone `/tmp/truenas-apps`).

## 8. Local live-app testing (task t_1422f175)

- ✅ install / start / stop-start / data persistence / logs / portals / failure
  recovery / 0.9.0→0.8.3 upgrade-downgrade — all PASS. See
  `TRUENAS-LOCAL-TEST-REPORT.md`.

## 9. Open items for Nige (human decisions, per skill "what you need from the human")

- [ ] Confirm `lib_version` 2.3.11 is still current at PR time (re-run hash check).
- [ ] Attach icon + 3 screenshots to the PR; reviewer returns CDN URLs.
- [ ] Decide TLS posture (v1 = HTTP on published port; note reverse-proxy TLS).
- [ ] Optional: add `hello@goatflow.io` to `maintainers`.
- [ ] Open the discussion issue first, then the PR (requirements §10.1).

## Verdict

**Package is ready for review.** Every automated CI gate passes on a clean
checkout; all required files are present and in steady-state; the license is
reconciled; screenshots + icon are staged. Only the human PR mechanics
(fork, attach media, open issue/PR) and the two soft decisions (lib_version
re-check, TLS note) remain.
