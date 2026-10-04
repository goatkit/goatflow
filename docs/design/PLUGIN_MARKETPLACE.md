# Plugin Marketplace (GoatKit PaaS)

## Overview

A lightweight plugin marketplace using GitHub Releases as the distribution backend. No dedicated server infrastructure — the marketplace is a curated JSON index file hosted in a GitHub repository, and the `gk` CLI handles discovery, installation, and updates.

## Architecture

```
┌─────────────────────────┐     ┌──────────────────────────┐
│  goatkit/marketplace    │     │  goatkit/inventory       │
│  (GitHub repo)          │     │  (plugin repo)           │
│                         │     │                          │
│  marketplace.json ◄─────┼─────┤  GitHub Release v1.2.0   │
│  (plugin index)         │     │  └── inventory.zip       │
│                         │     │  └── inventory.zip.sig   │
└────────┬────────────────┘     └──────────────────────────┘
         │
         │  HTTPS fetch
         ▼
┌─────────────────────────┐
│  GoatFlow instance      │
│                         │
│  gk install inventory   │
│  gk update              │
│  gk search calendar     │
│                         │
│  Admin UI → Browse      │
└─────────────────────────┘
```

## Registry Index

A single `marketplace.json` file in the `goatkit/marketplace` repository:

```json
{
  "version": 1,
  "updated_at": "2026-09-15T10:00:00Z",
  "plugins": [
    {
      "name": "inventory",
      "description": "Inventory and stock management",
      "author": "GoatKit",
      "licence": "Apache-2.0",
      "homepage": "https://github.com/goatkit/inventory",
      "repo": "goatkit/inventory",
      "category": "business",
      "tags": ["inventory", "stock", "warehouse"],
      "latest_version": "1.2.0",
      "min_host_version": "0.10.0",
      "versions": [
        { "version": "1.2.0", "min_host_version": "0.10.0", "released_at": "2026-09-15T10:00:00Z" },
        { "version": "1.1.0", "min_host_version": "0.8.0",  "released_at": "2026-06-02T09:00:00Z" },
        { "version": "1.0.0" }
      ],
      "runtime": "grpc",
      "verified": true,
      "public_key": "<ed25519 public key, hex>"
    }
  ]
}
```

### Versions

`versions` is optional and additive (the index stays `"version": 1`). Each item is one GitHub Release of the plugin:

| Field | Required | Meaning |
|-------|----------|---------|
| `version` | yes | Release version; the asset is `https://github.com/<repo>/releases/download/v<version>/<name>.zip` (+ `.zip.sig`) |
| `min_host_version` | no | Lowest GoatFlow that can run this release; empty means any |
| `released_at` | no | RFC 3339 release time, informational |

Keep `latest_version` / `min_host_version` describing the newest release so clients that predate `versions` still work. An entry without `versions` is treated as a single version: `latest_version` with the entry's `min_host_version`. If `versions` omits `latest_version`, it is still offered with the entry's `min_host_version`.

**Resolution** (shared by `gk`, the admin UI and the API). A version is *compatible* when its `min_host_version` is empty or not newer than the running GoatFlow (semver); development builds (`dev`, empty, or any non-semver version string) are compatible with everything.

- **Install** (no version given): the newest compatible version. If none is compatible the error names the oldest listed version, e.g. `inventory v1.0.0 requires GoatFlow >= 0.8.0, you have 0.7.0`.
- **Update**: the newest compatible version newer than the installed one. Newer releases that need a newer GoatFlow are not offered; if only those exist the update is refused with the same message.
- **Specific version** (`gk install inventory@1.1.0`, or `version` in the install API): must be listed and compatible, otherwise it is refused before anything is downloaded.

### Index Management

- Maintained manually or via GitHub Actions PR automation
- Plugin authors submit PRs to add/update their entry
- CI validates: repo exists, release exists, ZIP has valid manifest, signature present if `verified: true`
- Index is versioned — clients check `version` field for compatibility

## CLI Commands

### `gk install <plugin>[@version]`

```
$ gk install inventory

Fetching marketplace index...
Found: inventory v1.1.0 by GoatKit (Apache-2.0)
  Runtime: grpc
  Verified: ✓ (signed)
  Requires GoatFlow >= 0.8.0
  (v1.1.0 is the newest this GoatFlow 0.9.0 can run; latest is v1.2.0)

Installing inventory v1.1.0...
Installed inventory v1.1.0 to plugins/inventory/
Restart GoatFlow to activate.
```

**Flow:**
1. Fetch `marketplace.json` from GitHub
2. Find plugin entry by name
3. Resolve the version (see [Versions](#versions)); `@version` pins a listed one
4. Download the ZIP for that version from its GitHub Release
5. Verify the ed25519 signature if a `.sig` file exists (required when `GOATFLOW_REQUIRE_SIGNATURES=1`)
6. Extract into a staging directory under `plugins/` (`.gk-staging-*`, ignored by the plugin loader) and move it to `plugins/<name>/`
7. Print activation instructions

### `gk info <plugin>`

Lists every version with its `min_host_version` and whether this GoatFlow can run it, plus the version `gk install` would pick.

### `gk update [plugin]`

```
$ gk update

Checking for updates...
  inventory: 1.1.0 → 1.2.0 (update available)
  calendar:  2.0.0 → 2.0.0 (up to date)

Update inventory? [Y/n] y
Downloading inventory-v1.2.0.zip...
Verifying signature... ✓
Extracting... Done.
Restart GoatFlow to activate.
```

**Flow:**
1. Read installed plugin manifests from `plugins/*/plugin.yaml`
2. Fetch marketplace index
3. Pick the newest compatible version newer than the installed one (semver)
4. Download and verify it into a staging directory, then swap: the installed directory is moved aside, the new one moved in, and the old one removed. A failed download, signature check or swap leaves the installed plugin in place.

### `gk search <query>`

```
$ gk search calendar

Results:
  calendar        Calendar & appointments management    v1.0.0  GoatKit     Apache-2.0
  booking         Resource booking system               v0.3.0  Community   MIT
```

## Admin UI Integration

**Admin → Plugins → Marketplace** tab:

```
┌─────────────────────────────────────────────────────────────┐
│ Plugin Marketplace                              [Refresh]   │
├─────────────────────────────────────────────────────────────┤
│ Search: [________________________] Category: [All ▼]        │
├──────────────┬──────────────────────┬─────────┬─────────────┤
│ Plugin       │ Description          │ Version │             │
├──────────────┼──────────────────────┼─────────┼─────────────┤
│ ✓ inventory  │ Inventory management │ 1.2.0   │ [Installed] │
│   calendar   │ Calendar & appts     │ 1.0.0   │ [Install]   │
│   faq        │ Knowledge base       │ 0.9.0   │ [Install]   │
└──────────────┴──────────────────────┴─────────┴─────────────┘
```

- Fetches marketplace index via server-side HTTP (not client browser)
- Shows installed status by comparing against local plugin manifests
- Install button triggers `gk install` equivalent server-side; when a plugin lists several versions a version picker is shown (versions this GoatFlow cannot run are disabled)
- Update badge shown when a newer *compatible* version is available
- A plugin with no compatible version shows "Requires GoatFlow ≥ X" and its Install/Update button is disabled

`GET /api/v1/plugins/marketplace` returns the index annotated for the running host (`host_version` at top level). Each plugin carries the index fields plus:

| Field | Meaning |
|-------|---------|
| `versions` | Every listed version (`version`, `min_host_version`, `released_at`) with a `compatible` flag, newest first |
| `compatible` | Some listed version runs on this GoatFlow |
| `min_host_version` | Lowest GoatFlow that can run any listed version |
| `install_version` | Newest compatible version, `""` if none |
| `installed_version` | Installed version, `""` if not installed |
| `update_available` | A compatible version newer than the installed one exists |

`POST /api/v1/plugins/marketplace/install` takes `{"name": "...", "version": "..."}`; `version` is optional (new install → newest compatible, installed plugin → newest compatible update). An incompatible request returns `409` with the message above.

## Plugin Publishing

For plugin authors to list their plugin in the marketplace:

1. Create a GitHub repository with the plugin source
2. Build the plugin ZIP (via `gk build`)
3. Create a GitHub Release with the ZIP as a release asset
4. Optionally sign the ZIP (`gk sign`)
5. Submit a PR to `goatkit/marketplace` adding an entry to `marketplace.json`

### GitHub Actions Template

```yaml
name: Release Plugin
on:
  push:
    tags: ['v*']

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.10'
      - run: gk build
      - run: gk sign --key ${{ secrets.SIGNING_KEY }}
      - uses: softprops/action-gh-release@v2
        with:
          files: |
            dist/*.zip
            dist/*.zip.sig
```

## Version Compatibility

Compatibility is checked at two points:

1. **Marketplace** — `min_host_version` (per entry, or per item in `versions`) decides which releases are offered, as described under [Versions](#versions):

   ```
   Plugin requires >= 0.8.0
   GoatFlow running  0.7.0  → "inventory v1.1.0 requires GoatFlow >= 0.8.0, you have 0.7.0"
   GoatFlow running  0.8.0  → Proceeds with install
   GoatFlow running  0.9.0  → Proceeds with install
   ```

2. **Load time** — a plugin's own `GKRegistration.MinHostVersion` is enforced whenever it is registered (boot, hot reload, `POST /api/v1/plugins/upload`, marketplace install), for both gRPC and WASM plugins. A plugin that needs a newer GoatFlow is refused before `Init`, shut down, logged, and recorded in the plugin log buffer (`Plugin not loaded: … requires GoatFlow >= …`); on hot reload the running version keeps serving.

gRPC plugins receive the running GoatFlow version as `host_version` in their `Init` config.

## Localisation

Plugin metadata (name, description, tags) in `marketplace.json` is **English only** — it's developer-facing content and requiring 15 translations from every plugin author is impractical.

The **admin UI chrome** (column headers, buttons, search labels, "Install", "Installed", "Update available") uses `t()` with translations in all 15 languages, like every other GoatFlow admin page.

Plugin authors who want localised descriptions can optionally include them in their `plugin.yaml` under the existing `i18n` field. The admin UI renders the localised description if available for the user's language, falling back to English.

## Security Considerations

1. **Signature verification** — ed25519 signatures checked on install (already implemented in GoatFlow)
2. **HTTPS only** — marketplace index and downloads over HTTPS
3. **No auto-install** — marketplace is browse-only; admin explicitly triggers install
4. **No auto-update** — updates require admin confirmation
5. **Sandbox** — installed plugins run in the existing GoatKit sandbox (resource policies, permission whitelisting)
6. **Index integrity** — marketplace repo protected by GitHub branch protection; PRs require review

## Dependencies

Already implemented in GoatFlow 0.7.0/0.8.0:
- Plugin ZIP format with `plugin.yaml`
- Ed25519 signature verification
- Hot reload / blue-green plugin swap
- Plugin sandbox and resource policies
- `gk` CLI with `init`, `build`, `sign` commands

Needed for marketplace:
- [x] `gk install` command — download from GitHub Release
- [x] `gk update` command — version comparison and update
- [x] `gk search` command — index search
- [x] `marketplace.json` schema and initial index
- [x] Admin UI marketplace tab (`/admin/marketplace`: version picker, host-compatibility badges)
- [x] Per-version index (`versions`) and host-version checks on install, update and load (0.10.0)
- [x] GitHub Actions template for plugin publishing

## Hosting Costs

Zero. GitHub provides:
- Unlimited public repositories (plugin index + plugin repos)
- GitHub Releases with unlimited assets (plugin ZIPs)
- GitHub Actions CI minutes (2,000/month free for public repos)
- Raw content serving for `marketplace.json`

---

*Design: 2026-03-27*
