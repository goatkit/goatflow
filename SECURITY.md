# Security Policy

## Supported versions

Until 1.0.0, security fixes go into the latest minor release only. A fix ships as the next
patch or minor release; older minors are not patched, so upgrade to receive it.

| Version | Supported          |
| ------- | ------------------ |
| 0.10.x  | :white_check_mark: |
| < 0.10  | :x:                |

## Reporting a vulnerability

Do not report security issues in public GitHub issues, discussions or pull requests.

Report privately through GitHub:
**[Report a vulnerability](https://github.com/goatkit/goatflow/security/advisories/new)**
(the repository's Security tab → "Report a vulnerability"). The report stays private between you
and the maintainers, and becomes the draft advisory for the fix.

If you cannot use GitHub, email **security@goatflow.io**.

Please include as much as you can:

* the type of issue (for example SQL injection, cross-site scripting, broken access control)
* the affected version, tag or commit, and the source files involved
* any configuration needed to reproduce it
* step-by-step reproduction, and a proof of concept if you have one
* the impact: what an attacker gains and what they need first

English, please.

## Scope

In scope: this repository and the container images and Helm chart published from it. Plugins live
in their own repositories: report a plugin vulnerability there, or by email when the repository has
no private reporting.

Out of scope: issues that need an already compromised server or administrator account, and
missing hardening that is documented as the operator's job in [docs/SECURITY.md](docs/SECURITY.md)
(for example TLS and HSTS, which are set at the reverse proxy).

When testing, use your own installation. On the public demo, do not degrade the service, and do
not access or change data that is not yours. We will not pursue or support legal action against
research done in good faith within this policy.

## What happens next

We aim to reply to a report within:

| Severity | First response |
| -------- | -------------- |
| Critical | 24 hours       |
| High     | 48 hours       |
| Medium   | 1 week         |
| Low      | 1 week         |

Severity is rated with CVSS v3.1 when we confirm the report, and we tell you the rating.

1. **Triage.** We reproduce the issue, confirm the affected versions and the severity, and accept
   or decline the report with our reasons.
2. **Private fix.** The fix is developed in the advisory's temporary private fork, and you are
   invited to review it.
3. **CVE.** For accepted vulnerabilities in released versions we request a CVE through GitHub,
   which is a CVE Numbering Authority, from the draft advisory.
4. **Release.** The fix ships in a release. Critical fixes are released as soon as they are ready,
   without waiting for the next planned release.
5. **Disclosure.** We publish the GitHub Security Advisory with the CVE, affected and patched
   versions, and upgrade steps, and list the fix under "Security" in [CHANGELOG.md](CHANGELOG.md).
   We ask reporters to keep the issue private until the advisory is published, or 90 days after
   the report, whichever comes first.
6. **Credit.** You are credited in the advisory and the changelog, unless you ask us not to.

We do not run a paid bug bounty.

## Security updates

Published advisories are listed at
<https://github.com/goatkit/goatflow/security/advisories>. To be notified of security releases,
watch the repository on GitHub with Custom → Releases enabled.

## Security controls

The security controls GoatFlow has today, and what it leaves to the operator, are described in
[docs/SECURITY.md](docs/SECURITY.md).
