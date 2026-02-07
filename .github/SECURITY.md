# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          | Notes                           |
| ------- | ------------------ | ------------------------------- |
| 1.x.x   | :white_check_mark: | Current stable release          |
| 0.x.x   | :x:                | Pre-release/development version |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

If you believe you have found a security vulnerability in GoatFlow, please report it to us through coordinated disclosure.

### Where to Report

Please report security vulnerabilities to: **security@goatflow.io**

Please include the following information (as much as you can provide) to help us better understand the nature and scope of the possible issue:

* Type of issue (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
* Full paths of source file(s) related to the manifestation of the issue
* The location of the affected source code (tag/branch/commit or direct URL)
* Any special configuration required to reproduce the issue
* Step-by-step instructions to reproduce the issue
* Proof-of-concept or exploit code (if possible)
* Impact of the issue, including how an attacker might exploit the issue

### What to Expect

* **Initial Response**: We will acknowledge receipt of your vulnerability report within 48 hours
* **Assessment**: Our security team will investigate and validate the report
* **Fix Development**: We will develop and test appropriate fixes
* **Disclosure Timeline**: We aim to release patches within 30-90 days depending on severity
* **Credit**: We will credit reporters who follow coordinated disclosure

### Preferred Languages

We prefer all communications to be in English.

## Security Best Practices

When deploying GoatFlow:

1. **Keep Updated**: Always run the latest stable version
2. **Use HTTPS**: Enable TLS/SSL for all connections
3. **Strong Passwords**: Enforce strong password policies
4. **Principle of Least Privilege**: Grant minimum necessary permissions
5. **Regular Backups**: Maintain regular backups of your data
6. **Monitor Logs**: Regularly review security and audit logs
7. **Network Security**: Use firewalls and network segmentation
8. **Container Security**: Run containers as non-root users

## Security Features

GoatFlow includes several security features:

* **Authentication**: Multi-factor authentication support
* **Authorization**: Role-based access control (RBAC)
* **Encryption**: Data encryption at rest and in transit
* **Audit Logging**: Comprehensive audit trail
* **Session Management**: Secure session handling
* **Input Validation**: Protection against injection attacks
* **Rate Limiting**: API rate limiting to prevent abuse
* **CORS**: Configurable CORS policies

## Bug Bounty Program

We currently do not offer a paid bug bounty program. However, we deeply appreciate security researchers who:

1. Follow responsible disclosure practices
2. Give us reasonable time to address issues
3. Don't access or modify other users' data
4. Don't perform actions that could harm our services or users

We recognize researchers in our Hall of Fame and provide public acknowledgment.

## Security Updates

Security updates will be announced through:

* Security mailing list: hello@goatflow.io
* GitHub Security Advisories
* Release notes
* Official blog: https://goatflow.io/blog

## Contact

* Security issues: security@goatflow.io
* General support: hello@goatflow.io
* PGP Key: Available at https://goatflow.io/security/pgp-key.asc

## Commitment

Gibbsoft Ltd and the GoatFlow team are committed to ensuring the security of GoatFlow and its users. We appreciate your help in keeping GoatFlow secure.

---

Last Updated: August 2025