# TOTP/2FA Threat Model

## Overview

This document describes the threat model for GoatFlow Two-Factor Authentication (TOTP-based) for both agents and customers.

**Last Updated:** 2026-02-06

## Test Coverage Summary

| Category | Count | Status |
|----------|-------|--------|
| Go Unit Tests | 15 | ✅ All pass |
| Go Security/Contract Tests | 35 | ✅ All pass |
| Go E2E Tests | 2 | ✅ All pass |
| Playwright Behavioral Tests | 23 | ✅ All pass |
| **Total** | **75** | ✅ |

## Assets to Protect

1. **User accounts** - Agent and customer accounts
2. **TOTP secrets** - Base32-encoded shared secrets
3. **Recovery codes** - One-time backup codes (128-bit entropy)
4. **Session tokens** - JWT tokens issued after successful 2FA
5. **Pending 2FA sessions** - Server-side state during login flow

## Trust Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│                      UNTRUSTED                              │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐      │
│  │   Browser   │    │  Attacker   │    │ MITM Proxy  │      │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘      │
└─────────┼──────────────────┼──────────────────┼─────────────┘
          │ HTTPS            │                  │
┌─────────┼──────────────────┼──────────────────┼─────────────┐
│         ▼                  ▼                  ▼   TRUSTED   │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                    GoatFlow API                         ││
│  │  ┌───────────┐  ┌───────────┐  ┌───────────────┐        ││
│  │  │ Auth Flow │  │ TOTP Svc  │  │ Session Mgr   │        ││
│  │  └───────────┘  └───────────┘  └───────────────┘        ││
│  │                                      │                  ││
│  │  ┌─────────────────────────────────────────────────┐    ││
│  │  │           TOTPSessionManager (server-side)      │    ││
│  │  │  - 256-bit random tokens                        │    ││
│  │  │  - 5-attempt limit with lockout                 │    ││
│  │  │  - 5-minute expiry                              │    ││
│  │  └─────────────────────────────────────────────────┘    ││
│  └─────────────────────────────────────────────────────────┘│
│                           │                                 │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                    Database                             ││
│  │  user_preferences / customer_preferences tables         ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## Threats & Mitigations

### T1: TOTP Code Brute Force

**Threat:** Attacker tries all 1,000,000 possible 6-digit codes.

**Risk:** HIGH - If no rate limiting, account takeover in minutes.

**Mitigations:**
- [x] Rate limit 2FA verification attempts (max 5 per session)
- [x] Lock pending 2FA session after 5 failures (session invalidated)
- [x] Log failed attempts for monitoring (via totp_audit.go)
- [x] 30-second TOTP window limits valid codes
- [x] Response includes `attempts_remaining` for user feedback

**Implementation:** `internal/auth/totp_session.go` - `TOTPSessionManager.RecordFailedAttempt()`

**Test:** `TestVuln_V3_RateLimitingEnforced`, `TestVuln_V3_SessionLockedAfterMaxAttempts`

---

### T2: Recovery Code Enumeration

**Threat:** Attacker guesses recovery codes.

**Risk:** MEDIUM - Mitigated by high entropy.

**Mitigations:**
- [x] Recovery codes are 12 characters with 128-bit entropy (was 40 bits)
- [x] Rate limited via same 5-attempt session limit
- [x] Audit log when recovery code is used
- [x] Recovery codes are single-use (consumed on use)

**Implementation:** `internal/service/totp_service.go` - 16 bytes from crypto/rand, base64-encoded to 12 chars

**Test:** `TestVuln_V6_RecoveryCodeEntropy`, `TestVuln_V6_RecoveryCodesNotShort`

---

### T3: Pending 2FA Session Hijacking

**Threat:** Attacker steals `2fa_pending` cookie and completes login.

**Risk:** HIGH - Session fixation during authentication.

**Mitigations:**
- [x] Pending session expires after 5 minutes
- [x] Cookie is HttpOnly (no JS access)
- [x] Token is 256-bit cryptographically random (unpredictable)
- [x] Session data stored server-side (not in cookie)
- [x] Session invalidated after 5 failed attempts
- [x] Strict IP binding - session rejected if client IP changes during 2FA window
- [x] UA mismatch logging for security monitoring (soft enforcement)

**Implementation:** `internal/auth/totp_session.go` - `TOTPSessionManager`

**Test:** `TestVuln_V2_PendingTokenNotPredictable`, `TestVuln_V5_SessionValidatedServerSide`

---

### T4: 2FA Bypass via Direct API Access

**Threat:** After password auth, attacker accesses protected routes without completing 2FA.

**Risk:** CRITICAL - Complete 2FA bypass.

**Mitigations:**
- [x] No JWT token issued until 2FA verified
- [x] Pending 2FA state uses separate cookies (not auth tokens)
- [x] User ID retrieved from server-side session, not request body

**Implementation:** `internal/api/auth_htmx_handlers.go`, `internal/api/auth_customer.go`

**Test:** `TestVuln_V1_IDOR_UserIDNotFromRequestBody`, `TestTOTP_VerifyRequiresPendingSession`

---

### T5: TOTP Secret Exposure During Setup

**Threat:** Secret leaked via logs, error messages, or insecure storage.

**Risk:** HIGH - Permanent account compromise.

**Mitigations:**
- [x] Secret only returned once during setup
- [x] Secret stored encrypted in preferences (if DB encryption enabled)
- [x] Audit logging excludes secret values (only logs events, not secrets)
- [x] QR code generated server-side

**Implementation:** `internal/auth/totp_audit.go` - structured logging without sensitive data

**Test:** `TestTOTP_StatusResponseDoesNotExposeSecret`, `TestVuln_V8_AuditLogFormat`

---

### T6: Race Condition in Enable/Disable

**Threat:** Concurrent requests to enable/disable cause inconsistent state.

**Risk:** LOW - Could leave 2FA partially configured.

**Mitigations:**
- [x] Confirm requires pending secret to exist
- [x] Disable requires valid code
- [x] Use database transactions for state changes (future enhancement)

**Test:** `TestTOTP_DisableRequiresCodeInRequest`

---

### T7: Cross-User TOTP Validation (IDOR)

**Threat:** Attacker uses their TOTP code to authenticate as another user.

**Risk:** CRITICAL - Authentication bypass.

**Mitigations:**
- [x] TOTP secret bound to specific user ID/login
- [x] User ID retrieved from server-side session, NOT from request body
- [x] Pending 2FA session stores user identity server-side
- [x] Customer login stored server-side, not in cookie

**Implementation:** `internal/auth/totp_session.go` - stores UserID/Login in server memory, cookie only has random token

**Test:** `TestVuln_V1_IDOR_UserIDNotFromRequestBody`, `TestVuln_V4_CustomerLoginNotInCookie`

---

### T8: Unauthorized 2FA Disable

**Threat:** Attacker with session access disables 2FA without knowing code.

**Risk:** HIGH - Downgrades account security.

**Mitigations:**
- [x] Disable requires valid TOTP code or recovery code
- [x] Audit log when 2FA disabled
- [x] Email notification when 2FA disabled
- [x] Require re-authentication before disable (future enhancement)

**Implementation:** `internal/auth/totp_audit.go` - `LogTOTPDisabled()`

**Test:** `TestTOTP_DisableRequiresCodeInRequest`, `TestVuln_V8_AuditEventsExist`

---

### T9: Timing Attack on Code Validation

**Threat:** Measure response times to infer partial code correctness.

**Risk:** LOW - TOTP library uses constant-time comparison.

**Mitigations:**
- [x] `pquerna/otp` uses constant-time comparison
- [x] Same response time for valid/invalid codes

**Test:** `TestTOTP_CodeValidationConstantTime`

---

### T10: 2FA Setup Without Password Re-verification

**Threat:** Attacker with stolen session enables 2FA, locking out legitimate user.

**Risk:** MEDIUM - Account lockout.

**Mitigations:**
- [x] Recovery codes provided during setup (user can recover)
- [x] Audit log when 2FA enabled
- [x] Email notification when 2FA enabled
- [x] Require password re-entry before 2FA setup (future enhancement)

**Implementation:** `internal/auth/totp_audit.go` - `LogTOTPSetupCompleted()`

**Test:** `TestVuln_V8_AuditEventsExist`

---

## Security Checklist

### Implemented ✅
- [x] Rate limiting on 2FA verification (5 attempts per session)
- [x] Session invalidation after max failures
- [x] 256-bit cryptographically random session tokens
- [x] Server-side session storage (no sensitive data in cookies)
- [x] 128-bit entropy recovery codes (12 characters)
- [x] User ID from session, not request body (IDOR prevention)
- [x] Audit logging for all 2FA events
- [x] Constant-time code comparison
- [x] Email notifications for 2FA enable/disable (agents & customers)

### Future Enhancements
- [ ] Configurable strict UA binding (currently soft/logging only) - see note below

#### Note on User-Agent Binding

**Current state:** "Soft" enforcement - we log UA mismatches but don't reject. The session still works.

**What strict mode would do:** Reject the 2FA verification if User-Agent string changes between session start and 2FA completion.

**Why this remains LOW priority:**

1. **IP binding is already strict** - If client IP changes during the 2FA window, we reject. This catches most session theft scenarios.

2. **256-bit tokens are unguessable** - Attacker cannot predict the session token.

3. **5-minute expiry** - Very small window for attack.

4. **Legitimate UA changes happen:**
   - Browser auto-updates mid-session
   - Corporate proxies/VPNs modify headers
   - Mobile apps report inconsistent strings
   - Browser extensions can alter UA

**Recommendation:** Keep logging-only for monitoring. Tighten to strict enforcement only if evidence shows attacks evading IP binding. Current approach provides visibility without false positives that would frustrate legitimate users

### Completed Enhancements
- [x] Password re-verification for 2FA setup/disable (V9) - requires current password
- [x] Atomic preference updates via SetAndDelete() - all 2FA state changes in single transaction
- [x] Admin 2FA override with audit trail
- [ ] Hardware key support (WebAuthn) - see assessment below

#### Hardware Key (WebAuthn/FIDO2) Assessment

**Status:** Deferred - not planned for initial release.

**Effort:** MEDIUM-HIGH (2-3 weeks)
- New DB schema for credential storage (public keys, credential IDs, counters)
- WebAuthn registration/authentication flows with challenge-response
- Browser `navigator.credentials` API integration
- Multiple keys per user, fallback to TOTP, key management UI

**Why hardware keys are more secure than TOTP:**

| Threat | TOTP | Hardware Key |
|--------|------|--------------|
| Phishing | ❌ Vulnerable - code works on fake site | ✅ Origin-bound signature |
| Shared secret theft | ❌ Server or phone compromise = game over | ✅ Asymmetric - server only has public key |
| Replay attack | ❌ 30-second window | ✅ Counter-based, single use |
| Keylogger/malware | ❌ Code can be captured and used | ✅ Nothing to capture |

**Limitation:** Basic hardware keys (YubiKey, etc.) only verify physical presence via touch - not identity. Someone with stolen key + password gets in. High-security variants add PIN or biometrics.

**Why we're deferring:**

1. **TOTP is sufficient for ticketing systems** - Users aren't high-value targets like banking
2. **Hardware key adoption is still niche** - Most users don't own them
3. **Let TOTP implementation bake** - Get real-world feedback first
4. **Better use of engineering time** - Focus on features users are requesting

**Recommendation:** Add to roadmap as "Phase 2 MFA" when an enterprise customer specifically requests it. Current TOTP implementation covers 99% of use cases

## Test Coverage Matrix

| Threat | Regression Test | Contract Test | Notes |
|--------|-----------------|---------------|-------|
| T1 Brute Force | `TestVuln_V3_*` | ✓ | 5-attempt limit |
| T2 Recovery Enum | `TestVuln_V6_*` | ✓ | 128-bit entropy |
| T3 Session Hijack | `TestVuln_V2_*`, `TestVuln_V5_*` | ✓ | 256-bit tokens |
| T4 2FA Bypass | `TestVuln_V1_*` | ✓ | Server-side user ID |
| T5 Secret Exposure | `TestVuln_V8_*` | ✓ | Audit without secrets |
| T6 Race Condition | - | - | Low priority |
| T7 Cross-User (IDOR) | `TestVuln_V1_*`, `TestVuln_V4_*` | ✓ | Server-side session |
| T8 Unauth Disable | `TestVuln_V8_*` | ✓ | Audit logging |
| T9 Timing Attack | `TestTOTP_CodeValidationConstantTime` | ✓ | Library handles |
| T10 Setup Lockout | `TestVuln_V8_*` | ✓ | Audit logging |

## Audit Events

The following events are logged via `internal/auth/totp_audit.go`:

| Event | Description |
|-------|-------------|
| `2FA_SETUP_STARTED` | User initiated 2FA setup |
| `2FA_SETUP_COMPLETED` | User confirmed 2FA with valid code |
| `2FA_SETUP_FAILED` | Setup confirmation failed |
| `2FA_DISABLED` | User disabled 2FA |
| `2FA_VERIFY_SUCCESS` | Successful 2FA verification during login |
| `2FA_VERIFY_FAILED` | Failed 2FA verification attempt |
| `2FA_SESSION_CREATED` | New pending 2FA session created |
| `2FA_SESSION_EXPIRED` | Pending session timed out |
| `2FA_SESSION_LOCKED` | Session locked after max attempts |
| `2FA_RECOVERY_CODE_USED` | Recovery code consumed |

## References

- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [NIST SP 800-63B Digital Identity Guidelines](https://pages.nist.gov/800-63-3/sp800-63b.html)
- [RFC 6238 - TOTP Algorithm](https://tools.ietf.org/html/rfc6238)
