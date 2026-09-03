#!/usr/bin/env bash
#
# prepare-release.sh — DRY version-bump for a GoatFlow release.
#
# Usage:
#   make prepare-release VERSION=0.10.0
#   scripts/prepare-release.sh 0.10.0
#
# What it does (all mechanical, no content edits):
#   1. CHANGELOG.md   rename '## [X.Y.Z] - Unreleased' -> '## [X.Y.Z] - <date>'
#                     and insert a fresh '## [Unreleased]' section above it.
#                     If the changelog has no [Unreleased] section AND X.Y.Z
#                     already exists in the changelog, the rename is skipped
#                     and all other edits still run (per release policy).
#   2. version.go     default version constant -> X.Y.Z (dev-build fallback;
#                     CI builds always override via -ldflags from the tag)
#   3. Chart.yaml     appVersion -> X.Y.Z
#   4. TrueNAS        app.yaml app_version + ix_values.yaml image tag -> X.Y.Z
#   5. README.md      TrueNAS pin line -> X.Y.Z
#   6. ROADMAP.md     **Version** header -> X.Y.Z (theme kept) + adds a
#                     '🚧 In review' row to the Version Summary table
#
# It NEVER commits or tags. Review the diff, commit, then tag vX.Y.Z and
# let CI build. A Go test (internal/platform/version/version_consistency_test.go)
# fails CI if any of these pins drift apart afterwards.
#
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

VERSION="${1:-${VERSION:-}}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <semver>   (or: make prepare-release VERSION=<semver>)" >&2
  exit 2
fi
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "error: VERSION must be semver X.Y.Z (no 'v' prefix), got '$VERSION'" >&2
  exit 2
fi
DATE="$(date +%F)"

say()  { printf '\033[0;36m▸\033[0m %s\n' "$*"; }
warn() { printf '\033[0;33m⚠\033[0m %s\n' "$*"; }

# ---------------------------------------------------------------- changelog
say "CHANGELOG.md"
CL=CHANGELOG.md
UNREL_HEADER="## [${VERSION}] - Unreleased"
PLAIN_UNREL="## [Unreleased]"

if grep -qxF "$UNREL_HEADER" "$CL"; then
  # Rename the in-development header to a dated release header.
  python3 - "$VERSION" "$DATE" <<'PY'
import sys
version, date = sys.argv[1], sys.argv[2]
lines = open("CHANGELOG.md").read().split("\n")
for i, l in enumerate(lines):
    if l == f"## [{version}] - Unreleased":
        lines[i] = f"## [{version}] - {date}"
        lines.insert(i, "## [Unreleased]\n")  # fresh section goes ABOVE (trailing \n = blank line)
        break
open("CHANGELOG.md", "w").write("\n".join(lines))
PY
  say "  renamed '$UNREL_HEADER' -> '## [${VERSION}] - ${DATE}' + fresh [Unreleased] above it"
  warn "  review the [Unreleased] section content + roadmap theme/summary row"
elif grep -qF "## [${VERSION}]" "$CL"; then
  # Release tag already in the changelog (dated entry) — per release policy,
  # skip the changelog rename but continue with every other edit.
  warn "'${VERSION}' already exists in the changelog and there is no '$UNREL_HEADER' — skipping changelog edit (other edits continue)"
elif grep -qxF "$PLAIN_UNREL" "$CL"; then
  python3 - "$VERSION" "$DATE" <<'PY'
import sys
version, date = sys.argv[1], sys.argv[2]
lines = open("CHANGELOG.md").read().split("\n")
for i, l in enumerate(lines):
    if l == "## [Unreleased]":
        # Fresh [Unreleased] section first, then the dated release header;
        # the pending entries that follow now belong to the released version.
        lines[i] = f"## [Unreleased]\n\n## [{version}] - {date}"
        break
open("CHANGELOG.md", "w").write("\n".join(lines))
PY
  say "  renamed '[Unreleased]' -> '## [${VERSION}] - ${DATE}' + fresh [Unreleased] above it"
else
  warn "no [Unreleased] section and '${VERSION}' not in changelog — skipping changelog edit (other edits continue)"
fi

# ------------------------------------------------------------ version pins
pin_version_go() {
  say "internal/platform/version/version.go"
  python3 - "$VERSION" <<'PY'
import re, sys
version = sys.argv[1]
p = "internal/platform/version/version.go"
src = open(p).read()
new, n = re.subn(r'(Version\s*=\s*)"[^"]+"', rf'\g<1>"{version}"', src, count=1)
if n == 0: sys.exit(f"no 'Version = ...' constant found in {p}")
open(p, "w").write(new)
PY
}
pin_version_go

say "charts/goatflow/Chart.yaml"
python3 - "$VERSION" <<'PY'
import re, sys
version = sys.argv[1]
p = "charts/goatflow/Chart.yaml"
src = open(p).read()
new, n = re.subn(r'^appVersion:.*$', f'appVersion: "{version}"', src, count=1, flags=re.M)
if n == 0: sys.exit(f"no appVersion line in {p}")
open(p, "w").write(new)
PY

say "docs/truenas-app/goatflow/app.yaml"
python3 - "$VERSION" <<'PY'
import re, sys
version = sys.argv[1]
p = "docs/truenas-app/goatflow/app.yaml"
src = open(p).read()
new, n = re.subn(r'^app_version:.*$', f'app_version: {version}', src, count=1, flags=re.M)
if n == 0: sys.exit(f"no app_version line in {p}")
open(p, "w").write(new)
PY

say "docs/truenas-app/goatflow/ix_values.yaml (goatflow image tag only)"
python3 - "$VERSION" <<'PY'
import sys
version = sys.argv[1]
p = "docs/truenas-app/goatflow/ix_values.yaml"
out, in_gf = [], False
changed = 0
for line in open(p):
    if "ghcr.io/goatkit/goatflow" in line:
        in_gf = True
    if in_gf and "tag:" in line and changed == 0:
        line = line[:line.index("tag:")] + f'tag: "{version}"\n'
        changed = 1
    out.append(line)
if changed == 0: sys.exit(f"no image tag line under ghcr.io/goatkit/goatflow in {p}")
open(p, "w").write("".join(out))
PY

say "README.md (TrueNAS pin)"
python3 - "$VERSION" <<'PY'
import re, sys
version = sys.argv[1]
p = "README.md"
src = open(p).read()
if f"ghcr.io/goatkit/goatflow:{version}" in src:
    print("  already pinned")
else:
    # Tag = leading non-whitespace run of alphanumerics/dots (stops at the
    # closing backtick of `...:0.10.0`).
    new = re.sub(
        r"ghcr\.io/goatkit/goatflow:[0-9][0-9a-zA-Z.~_+-]*",
        f"ghcr.io/goatkit/goatflow:{version}",
        src, count=1,
    )
    if new == src:
        sys.exit("no ghcr.io/goatkit/goatflow:<tag> pin found in README.md TrueNAS section")
    open(p, "w").write(new)
PY

# ------------------------------------------------------------- roadmap
say "ROADMAP.md"
python3 - "$VERSION" <<'PY'
import sys, re
version = sys.argv[1]
p = "ROADMAP.md"
lines = open(p).read().split("\n")
out, header_bumped, row_added = [], False, False
for l in lines:
    # Header: bump the version number, mark the date 'Unreleased'.
    m = re.match(r"(\*\*Version\*\*:\s*)([0-9.]+)(\s*\([^)]*\)\s*-\s*)(.+)$", l)
    if m and not header_bumped:
        out.append(f"{m.group(1)}{version} (Unreleased) - {m.group(4).strip()}")
        header_bumped = True
        continue
    # Version Summary table: the 1.0.0 'Future' row sits at the top; new
    # in-development rows go directly BELOW it — unless one already exists.
    if not row_added and l.startswith("| 1.0.0 |"):
        out.append(l)
        if not any(x.startswith(f"| {version} |") for x in lines):
            out.append(f"| {version} | Unreleased | 🚧 In review | — |")
            print(f"  added Version Summary row for {version}")
        else:
            print(f"  Version Summary row for {version} already present")
        row_added = True
        continue
    out.append(l)
open(p, "w").write("\n".join(out))
if header_bumped:
    print(f"  header -> {version} (Unreleased) — review the theme text, it was kept from the previous release")
if not row_added:
    print(f"  WARNING: no '| 1.0.0 |' row found in Version Summary table — add the {version} row manually")
PY

# ------------------------------------------------------------- done
echo
say "Done. Changed files:"
git status --short
echo
say "Review the diff, commit, then tag 'v${VERSION}' — CI derives everything else from the tag."
say "Run 'go test ./internal/platform/version/...' to verify pins agree."
