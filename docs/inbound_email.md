# Inbound Email Architecture (OTRS Parity)

This document captures the inbound-email plan for GoatFlow based on the proven OTRS PostMaster
pipeline. The goal is to collect mail from POP3/IMAP mailboxes, run them through a PostMaster-style
processor, and create or update tickets just as OTRS does.

## 1. Goals
- Reach feature parity with OTRS's inbound flow so that existing OTRS deployments can migrate
  without losing automation.
- Keep the outbound pipeline untouched (already functional) while layering reusable inbound services.
- Make every step testable and configurable (filters, queue dispatch, follow-up rules, loop
  protection).

## 2. High-Level Flow

```
Scheduler Job ──► Mail Account Repo ──► Protocol Connector (POP3/IMAP)
                  │                                 │
                  │                                 ▼
                  ├─ fetch raw RFC822 messages ◄────┘
                  ▼
          PostMaster Processor
                  │
        ┌─────────┴─────────┐
        ▼                   ▼
  PreFilter / PreCreate   Follow-Up Detection
        │                   │
        └───────► Ticket Service (new/follow-up/reject)
```

Each message is processed independently. Failures (auth, parsing, ticket creation) are logged and
surfaces via metrics so operators can act.

## 3. Components

### 3.1 Email Account Repository
  - Existing fields: SMTP/IMAP host, port, username, encrypted password, queue ID, `is_active`.
  - Planned additions: trusted flag (`AllowXHeaders`), dispatching mode (`queue` vs `from`),
    OAuth2 token reference, optional IMAP folder, polling cadence.
- Admin UI mirrors these fields so operators can manage ingestion without SQL:
  - `Login`, `Host`, and `Account Type` inputs replace the previous SMTP/IMAP split.
  - `Dispatching Mode` radio (`Queue` vs `From`) writes `queue_id` (0 means from-based routing).
  - `Allow Trusted Headers` toggle maps to the `trusted` column so PostMaster knows whether to honor `X-GoatFlow-*` overrides.
  - `IMAP Folder` text box defaults to `INBOX` for IMAP accounts; POP3 ignores it.
  - `Poll Interval (seconds)` accepts a per-account cadence and falls back to the scheduler default when empty. Today we store the value in the repository struct so the scheduler can vary concurrency per mailbox.
- Exposed via service layer for admin UI/API.

### 3.2 Protocol Connectors
- Package: `internal/email/inbound/connector` (new).
- Interfaces:
  ```go
  type MessageFetcher interface {
      Fetch(ctx context.Context, account models.EmailAccount) ([]FetchedMessage, error)
  }
  ```
- Implementations:
  - POP3/POP3S via `github.com/knadh/go-pop3` (thin wrapper: connect, UIDL, RETR, delete).
  - IMAP/IMAPS via `github.com/emersion/go-imap/v2` (IDLE optional, folder select, UID fetch; accepts aliases `imap`, `imaps`, `imap_tls`, `imaps_tls`, `imaptls`).
- Responsibilities:
  - Honor TLS requirements.
  - Preserve folder context: `IMAPFolder` defaults to `INBOX` and is carried into `AccountSnapshot` and message metadata (`imap_folder`).
  - Default to OTRS's destructive fetch: POP3 issues `DELE`, IMAP expunges or moves processed
    messages so the remote mailbox is drained without keeping local cursor state.
  - Delete/move only after PostMaster reports success; failures leave the message for the next poll.
  - Convert bytes to `FetchedMessage` (raw RFC822 + metadata such as UID, envelope timestamps).
  - Integration coverage: SMTP4Dev flows exercise POP (delete/no-delete) and IMAP (folder metadata, non-destructive) under the `integration` build tag.

### 3.3 Scheduler Job
- Package: `internal/services/scheduler` currently contains TODO placeholders.
- New job: `InboundEmailPoller` that runs every N seconds (config) and:
  1. Loads active accounts from repository.
  2. Picks the right connector (POP3, IMAP, future Graph/OAuth) per account.
  3. Emits `FetchedMessage` objects onto a work queue processed concurrently (default worker pool per
     account or global fan-out, similar to OTRS's daemon).
  4. Persists fetch metrics (success/failure, auth errors).

### 3.4 PostMaster Processor
- Package: `internal/email/inbound/postmaster`.
- Mirrors OTRS's `Kernel::System::PostMaster` responsibilities:
  1. Parse raw mail via a structured parser (we already use `github.com/emersion/go-message`).
  2. Build `GetParam`-style map containing headers, addresses, attachments, body text, and derived
     metadata (Message-ID, References, thread tokens, attachments).
  3. Execute PreFilter modules (see below).
  4. Run follow-up detection strategies.
  5. Decide action: ignore (`X-GoatFlow-Ignore`), follow-up, new ticket, reject.
  6. Call ticket service APIs (existing `internal/service/ticket_service.go`) to create/update tickets
     and append articles.
  7. Request autoresponses (auto-reply, bounce, reject) from notification service as needed.

### 3.5 Filters and Hooks
- Config root: `email.postmaster.filters` to mimic OTRS's `%PostMaster::PreFilterModule%`.
- Hook interface:
  ```go
  type Filter interface {
      ID() string
      Run(ctx context.Context, m *MessageContext) error
  }
  ```
- Default filters we plan to port:
  - Match (header/body regex -> set queue, tags, states).
  - MatchDBSource (pull mapping from DB table).
  - SubjectToken (parses `Re: [Ticket#123]` subjects and annotates the detected ticket number for follow-up checks).
  - DetectBounceEmail (identify DSNs, set flags for follow-up logic).
  - DetectAttachment (flag suspicious attachments for policy enforcement).
  - CMD/external script (execute containerized hook; optional).
- Hooks run both before follow-up detection (mutate message) and before ticket creation (enforce
  policies like dynamic field overrides).

### 3.6 Follow-Up Detection
- Strategy registry mirroring OTRS's `FollowUpCheck::*` modules:
  - Subject token stripping (`Re: [Ticket#2025012345]`).
  - References header search.
  - Body tag search.
  - Attachment metadata search.
  - External ticket number recognition.
- Subject token follow-ups are live: `SubjectToken` stores the detected ticket number in
  `postmaster.follow_up_ticket_number` and `TicketProcessor` now resolves it, verifies the queue's
  follow-up policy (`follow_up_possible`), and appends the inbound email as a customer-visible
  article when follow-ups are allowed. Queues configured to `reject` or `new ticket` skip this path
  and continue through the new-ticket flow.
- References/In-Reply-To follow-ups are also live: `TicketProcessor` parses the threading headers,
  strips Message-ID brackets, and asks the article repository for a ticket owning those message IDs.
  When a match is found the inbound email is appended to that ticket (subject to the same queue
  policy checks) so replies without the `[Ticket#]` token remain threaded correctly.
- Body tag follow-ups mirror OTRS’s body-search strategy: a dedicated filter decodes the first
  readable text part (plain preferred, HTML falls back to stripped text) and extracts `[Ticket#]`
  markers embedded in the message content, enabling signatures or templates that embed the token
  outside the subject line to remain threaded.
- Header token follow-ups honor trusted headers like `X-GoatFlow-TicketNumber`/`X-OTRS-TicketNumber`
  so upstream systems that can’t alter the subject/body can still pass the GoatFlow ticket number
  explicitly.
- Attachment metadata follow-ups scan attachment filenames, descriptions, and name parameters for
  `[Ticket#]` markers so forwarded threads whose subjects/bodies were rewritten by gateways can
  still resolve to the original tickets when partners only preserve the tag inside attached logs.
- External ticket number recognition loads partner-specific regex rules from
  `config/external_ticket_rules.yaml` (see the `.example` file in `config/`) so we can extract GoatFlow ticket numbers from arbitrary
  subject/body/header formats without depending on the `[Ticket#]` token.
- Each strategy returns `(ticketID, followUpType)`; the processor picks the first confident match,
  falling back to "new ticket".
- Queue configuration (similar to OTRS's follow-up option) will be stored per queue so we know
  whether to accept follow-ups, force new ticket, or reject.

### 3.7 Loop Protection & Bounce Handling
- Loop guard store (DB table `mail_loop_protection` keyed by Message-ID + sender) to prevent GoatFlow
  from replying repeatedly to autoresponders.
- Bounce detection filter sets `isBounce=true`; follow-up logic uses queue settings and config
  `postmaster.bounceAsFollowUp` to mimic OTRS's `PostmasterBounceEmailAsFollowUp`.

### 3.8 Trusted Headers
- Per-account boolean `allow_trusted_headers` decides whether we honor inbound `X-GoatFlow-*` overrides
  (queue, priority, state, dynamic fields). Default false except for service-to-service mailboxes.
- When enabled, dynamic field definitions are reflected into allowed header names, same as OTRS's
  addition of `X-OTRS-DynamicField-*`.
- Operators may supply additional header names in `email.inbound.trustedHeaders`; every match is
  exposed to downstream filters as `postmaster.trusted_header.<header-name>` annotations so custom
  routing modules can consume partner-specific metadata.

### 3.9 Configuration Surface
- `config/email.yaml` (new) or extend existing config to include:
  ```yaml
  email:
    inbound:
      enabled: true
      poll_interval: 30s
      worker_count: 4
      max_accounts: 5
      trusted_headers: ["X-GoatFlow-Priority", ...]
      filters:
        pre:
          - id: match
            module: match
            config:
              subject_regex: ...
        pre_create:
          - id: bounce-check
            module: detect_bounce
      follow_up:
        strategies:
          - subject_token
          - references
          - body_tag
      loop_protection:
        backend: db
  ```
- These keys now drive the scheduler directly: `enabled` removes or activates the `email-ingest`
  job, `poll_interval` emits an `@every <duration>` schedule, and `worker_count`/`max_accounts`
  override the job config so poll concurrency and fan-out can be tuned without code changes.
- Each module implemented as Go plugin (not Go plugins; standard Go interfaces). Config is parsed at
  startup and provided to the scheduler + processor.

### 3.10 Message Retention Strategy (OTRS parity)
- POP3 accounts delete messages immediately after a successful PostMaster run (`DELE` during the
  same session). If processing fails we leave the message on the server so the next poll retries.
- IMAP accounts default to destructive handling as well: after processing we mark the UID as
  `\Deleted` and issue `Expunge` (or move to an operator-defined "Processed" folder) before closing
  the session.
- Because everything is deleted on the remote side, we do **not** persist local cursor metadata or
  introduce new tables. This satisfies schema-freeze requirements while matching OTRS behavior.
- If a tenant needs "leave copy on server", we can satisfy it later by storing the last UID in
  existing columns such as `mail_account.comments` or in filesystem state (`var/state/mail_account`)
  without new migrations.

### 3.11 Secrets & Credential Handling
- **Community baseline**: reuse our existing `.env` contract. POP/IMAP credentials, OAuth client IDs,
  and SMTP passwords continue to live in the environment (developer machines copy `.env.development`,
  production environments load values from `.env` kept outside version control).
- **Kubernetes alignment**: operators mirror the `.env` content into native `Secret` manifests and
  mount them via `envFrom` or projected volumes. No code changes are required; the inbound poller
  reads from `os.Getenv` the same way the rest of GoatFlow already does.
- **Enterprise roadmap**: later offer optional vault-backed secret adapters (HashiCorp Vault Agent, AWS
  Secrets Manager, etc.) that hydrate the same environment variables at runtime. This matches the
  roadmap epic captured in `ROADMAP.md` and keeps schema-freeze intact.
- **Guardrails**: `.env` stays excluded from git, documentation reminds admins to rotate credentials
  before promotion, and CI alerts if obvious demo secrets leak into committed config samples.

### 3.12 Dispatching & Multi-Tenant Routing
- `mail_account.dispatching_mode` mirrors OTRS's `DispatchingBy` flag. Two modes ship initially:
  - `queue`: mailbox always injects its configured `queue_id` as the default destination; PreFilter
    modules can override later, but the queue dropdown in the admin UI is authoritative.
  - `from`: the mailbox skips the stored queue and asks a dedicated dispatch filter to resolve the
    queue based on sender identity.
- **From-based routing** ships today as the `dispatch_from_map` PreFilter. It loads sender-matching
  rules from `CONFIG_DIR/email_dispatch.yaml` (missing file = no-op) and applies them whenever an
  account's dispatching mode is `from`.
  - YAML structure:
    ```yaml
    accounts:
      42:
        - match: "*@vip.example.com"
          queue: Premium
          priority_id: 5
        - match: "*@corp.example.com"
          queue_id: 12
      7:
        - match: "*"
          queue: Default
    ```
  - Keys map directly to the current implementation:
    - `match`: glob expression (uses Go's `path.Match`) checked against the lowercase sender address.
      `*` matches everything.
    - `queue`: queue name override (stored in annotations for downstream lookup).
    - `queue_id`: direct queue ID override when the numeric ID is known and stable.
    - `priority_id`: optional priority override.
- Rules are evaluated in file order per account; the first match wins. When a rule fires the filter
  stamps annotations (`AnnotationQueueIDOverride`, etc.) so follow-up detection and ticket creation
  share the same routing decision.
- **UI support**: Admin → Email → Mailboxes now surfaces a `Dispatching Mode` selector plus two
  metadata controls: `Allow Trusted Headers` and `Poll Interval (seconds)`. Selecting `from`
  automatically disables the queue dropdown (the backend sets `queue_id = 0`) and dispatching rules
  take over. The trusted toggle keeps OTRS-style header overrides explicit per mailbox, and the
  optional poll interval writes to `mail_account.comments` metadata so operators can fine-tune busy
  accounts without touching scheduler config.
- **Fallbacks**: If no rule matches we fall back to the account's stored `queue_id` so the message
  never drops. Observability hooks will log misses so operators can tighten rules.

## 4. Sequence Example

1. Scheduler wakes up, loads active accounts.
2. For account `support@example.com` (IMAPS, queue "Support") it launches a fetch cycle.
3. Connector fetches 5 new messages, wraps them as `FetchedMessage{Raw, AccountID, UID}`.
4. PostMaster processor receives message, parses it, runs filters.
5. Follow-up detection finds Ticket #202510050001 in subject, queue allows follow-ups.
6. Processor calls ticket service to append article, optionally unlock/lock depending on queue
   follow-up lock policy.
7. If message matched `X-GoatFlow-Ignore: yes`, the processor logs and skips.
8. Once processing succeeds, connector marks UID as read/deleted based on account type.

## 5. Open Questions
- (none) – remaining work is captured in the next-steps list below.

## 6. Next Steps
1. Finalize config schema and data model deltas (trusted flag, IMAP folder, dispatching mode).
2. Scaffold Go packages (`internal/email/inbound/...`).
3. Implement POP3 connector + simple scheduler loop to prove ingestion end-to-end with "new ticket"
   path.
4. Add filters/follow-up gradually until parity.

This design mirrors OTRS's architecture so teams migrating from OTRS know where to hook in their
existing automations while staying idiomatic to GoatFlow (Go packages, containerized scheduler,
pluggable services).
