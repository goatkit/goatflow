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
      "min_host_version": "0.8.0",
      "runtime": "grpc",
      "verified": true,
      "downloads": 0
    }
  ]
}
```

### Index Management

- Maintained manually or via GitHub Actions PR automation
- Plugin authors submit PRs to add/update their entry
- CI validates: repo exists, release exists, ZIP has valid manifest, signature present if `verified: true`
- Index is versioned — clients check `version` field for compatibility

## CLI Commands

### `gk install <plugin>`

```
$ gk install inventory

Fetching marketplace index...
Found: inventory v1.2.0 by GoatKit (Apache-2.0)
  Requires GoatFlow >= 0.8.0 ✓
  Runtime: gRPC
  Verified: ✓ (signed)

Downloading inventory-v1.2.0.zip from goatkit/inventory...
Verifying signature... ✓
Extracting to plugins/inventory/...
Done. Restart GoatFlow to activate.
```

**Flow:**
1. Fetch `marketplace.json` from GitHub (raw URL, cached for 1 hour)
2. Find plugin entry by name
3. Check `min_host_version` against running GoatFlow version
4. Download ZIP from GitHub Release (`gh release download` or direct URL)
5. Verify ed25519 signature if `.sig` file exists
6. Extract to `plugins/<name>/` directory
7. Print activation instructions

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
1. Read installed plugin manifests from `plugins/*/manifest.yaml`
2. Fetch marketplace index
3. Compare versions (semver)
4. Download and replace if newer version available

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
- Install button triggers `gk install` equivalent server-side
- Update badge shown when newer version available

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
          go-version: '1.25'
      - run: gk build
      - run: gk sign --key ${{ secrets.SIGNING_KEY }}
      - uses: softprops/action-gh-release@v2
        with:
          files: |
            dist/*.zip
            dist/*.zip.sig
```

## Version Compatibility

The `min_host_version` field in the marketplace index ensures plugins aren't installed on incompatible GoatFlow versions:

```
Plugin requires >= 0.8.0
GoatFlow running  0.7.0  → "Requires GoatFlow 0.8.0 or later. Please upgrade."
GoatFlow running  0.8.0  → Proceeds with install
GoatFlow running  0.9.0  → Proceeds with install
```

## Localisation

Plugin metadata (name, description, tags) in `marketplace.json` is **English only** — it's developer-facing content and requiring 15 translations from every plugin author is impractical.

The **admin UI chrome** (column headers, buttons, search labels, "Install", "Installed", "Update available") uses `t()` with translations in all 15 languages, like every other GoatFlow admin page.

Plugin authors who want localised descriptions can optionally include them in their `manifest.yaml` under the existing `i18n` field. The admin UI renders the localised description if available for the user's language, falling back to English.

## Security Considerations

1. **Signature verification** — ed25519 signatures checked on install (already implemented in GoatFlow)
2. **HTTPS only** — marketplace index and downloads over HTTPS
3. **No auto-install** — marketplace is browse-only; admin explicitly triggers install
4. **No auto-update** — updates require admin confirmation
5. **Sandbox** — installed plugins run in the existing GoatKit sandbox (resource policies, permission whitelisting)
6. **Index integrity** — marketplace repo protected by GitHub branch protection; PRs require review

## Dependencies

Already implemented in GoatFlow 0.7.0/0.8.0:
- Plugin ZIP format with `manifest.yaml`
- Ed25519 signature verification
- Hot reload / blue-green plugin swap
- Plugin sandbox and resource policies
- `gk` CLI with `init`, `build`, `sign` commands

Needed for marketplace:
- [ ] `gk install` command — download from GitHub Release
- [ ] `gk update` command — version comparison and update
- [ ] `gk search` command — index search
- [ ] `marketplace.json` schema and initial index
- [ ] Admin UI marketplace tab
- [ ] GitHub Actions template for plugin publishing

## Hosting Costs

Zero. GitHub provides:
- Unlimited public repositories (plugin index + plugin repos)
- GitHub Releases with unlimited assets (plugin ZIPs)
- GitHub Actions CI minutes (2,000/month free for public repos)
- Raw content serving for `marketplace.json`

---

*Design: 2026-03-27*
