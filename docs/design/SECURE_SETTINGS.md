# Secure Settings (GoatKit PaaS Core)

## Overview

Encrypted configuration storage via HostAPI. Plugins store API keys, device PINs, webhook secrets, and other sensitive values without handling cryptography directly. The platform manages the encryption key, storage, and masked display.

Today, plugins that need secrets must roll their own encryption or store values in plain text via `DBExec`. Secure Settings provides a first-class, auditable, encrypted key-value store accessible via `SecureConfigGet` / `SecureConfigSet`.

## User Stories

- **Plugin author**: "I want to store a third-party API key without writing encryption code"
- **Admin**: "I want to see which secrets a plugin has stored, but only reveal the last 4 characters"
- **Admin**: "I want to rotate the platform encryption key without losing stored secrets"
- **Plugin author**: "I want to store a device PIN that my kiosk UI validates against"

## Design Principles

1. **Platform manages crypto** — plugins never see raw encryption keys or choose algorithms
2. **Namespaced per plugin** — plugins can only access their own secrets (sandbox enforced)
3. **Masked by default** — admin UI shows `••••••••abcd` (last 4 chars), never the full value
4. **Single encryption key** — AES-256-GCM with a platform-managed key, stored in env var or KernelConfig
5. **Simple API** — just Get and Set, no key rotation API (platform handles re-encryption internally)

## Database Schema

```sql
CREATE TABLE gk_secure_config (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    plugin_name VARCHAR(100) NOT NULL,
    name        VARCHAR(250) NOT NULL,
    encrypted_value VARBINARY(4096) NOT NULL,   -- AES-256-GCM ciphertext + nonce
    value_hint  VARCHAR(10) DEFAULT NULL,        -- last 4 chars for admin display
    org_id      BIGINT DEFAULT NULL,             -- NULL = global, non-NULL = per-org
    create_time DATETIME NOT NULL,
    create_by   INT NOT NULL,
    change_time DATETIME NOT NULL,
    change_by   INT NOT NULL,

    UNIQUE KEY uk_plugin_name_org (plugin_name, name, org_id),
    KEY idx_plugin (plugin_name),
    CONSTRAINT fk_sc_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_sc_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### Encryption Details

- **Algorithm**: AES-256-GCM (authenticated encryption)
- **Key source**: `GOATFLOW_SECURE_KEY` environment variable (32 bytes, hex-encoded) or generated on first use and stored in `sysconfig_modified`
- **Nonce**: Random 12 bytes, prepended to ciphertext
- **Storage format**: `[12-byte nonce][ciphertext][16-byte GCM tag]` in `encrypted_value`
- **value_hint**: Last 4 characters of the plaintext, stored unencrypted for admin display

## HostAPI Methods

```go
type HostAPI interface {
    // ... existing methods ...

    // SecureConfigGet retrieves a decrypted secret value.
    // Plugin prefix is auto-applied by the sandbox.
    SecureConfigGet(ctx context.Context, key string) (string, error)

    // SecureConfigSet stores an encrypted secret value.
    // Plugin prefix is auto-applied by the sandbox.
    SecureConfigSet(ctx context.Context, key string, value string) error
}
```

### Sandbox Enforcement

The `SandboxedHostAPI` auto-prefixes keys with the plugin name:

```
Plugin "inventory" calls SecureConfigSet("stripe_key", "sk_live_xxx")
→ stored as plugin_name="inventory", name="stripe_key"
→ value_hint="_xxx"
→ encrypted_value = AES-256-GCM(plaintext)
```

Plugins can only access secrets where `plugin_name` matches their name.

### Org Scoping

If the request has an active org context, secrets are stored with `org_id` set. This allows different API keys per organisation:

```
Plugin "inventory" in org 42: SecureConfigSet("stripe_key", "sk_live_org42")
→ stored with org_id=42

Plugin "inventory" in org 99: SecureConfigSet("stripe_key", "sk_live_org99")
→ stored with org_id=99
```

`SecureConfigGet` checks org-specific first, falls back to global (`org_id IS NULL`).

## Admin UI

Admins can view stored secrets at **Admin → Plugins → [Plugin Name] → Secrets**:

```
┌─────────────────────────────────────────────────────────┐
│ Inventory — Secure Settings                             │
├──────────────┬──────────────────┬────────┬──────────────┤
│ Key          │ Value            │ Org    │              │
├──────────────┼──────────────────┼────────┼──────────────┤
│ stripe_key   │ ••••••••_xxx     │ Global │ [Edit] [Del] │
│ webhook_sec  │ ••••••••ab12     │ Acme   │ [Edit] [Del] │
└──────────────┴──────────────────┴────────┴──────────────┘
```

Edit opens a form where the admin types the new value. The old value is never displayed.

## Key Management

### Initial Key Generation

On first startup, if `GOATFLOW_SECURE_KEY` is not set:
1. Generate 32 random bytes
2. Store hex-encoded in `sysconfig_modified` as `SecureSettings::EncryptionKey`
3. Log a warning: "Generated secure settings key — set GOATFLOW_SECURE_KEY in production"

### Key Rotation

Not automated in 0.8.0. Manual process:
1. Set new `GOATFLOW_SECURE_KEY` env var
2. Run `goats secure-rekey --old-key=<hex>` CLI command
3. Re-encrypts all values with new key

## Security Considerations

1. **Key storage**: Env var preferred over DB (DB backup shouldn't contain the key that decrypts its own secrets)
2. **No plaintext logging**: Decrypted values never written to logs
3. **GCM authentication**: Tampered ciphertext is detected and rejected
4. **Per-plugin isolation**: Sandbox prevents cross-plugin secret access
5. **Hint is minimal**: Only last 4 chars — enough for admin to identify which key, not enough to use it

## Implementation Order

1. [ ] Design spec (this document)
2. [ ] Database migration — `gk_secure_config` table
3. [ ] Encryption service — AES-256-GCM encrypt/decrypt with key management
4. [ ] Repository — CRUD for secure config entries
5. [ ] HostAPI methods — `SecureConfigGet`, `SecureConfigSet`
6. [ ] Sandbox enforcement — plugin name prefixing, org scoping
7. [ ] WASM + gRPC wire format
8. [ ] All mock HostAPIs updated
9. [ ] Admin masked display handler
10. [ ] Tests — encryption round-trip, isolation, org scoping, masked display

---

*Design: 2026-03-27*
