# GoatFlow Administrator Guide

This page lists what an administrator can do in GoatFlow today and where to find it.
Most settings live in the admin area at `/admin`. Some settings are made with
environment variables or Helm values instead; these are listed under
[Settings made outside the admin area](#settings-made-outside-the-admin-area).

## Who is an administrator

An agent is an administrator when they are a member of the `admin` group.
Nothing else makes someone an admin: not the login name, not the user id.

An API token can call admin-only API routes (for example `/api/v1/webhooks`) only when
both are true:

- its owner is in the `admin` group right now, and
- its scopes include `*` or `admin:*`, or it has no scopes (a token with no scopes inherits its owner's rights).

### The first administrator

A new database has one built-in admin account, `root@localhost`. It starts
disabled, with a random password.

To log in for the first time, set `GOATFLOW_ADMIN_PASSWORD` in the backend's
environment before the first start. On startup the backend sets that password and
enables the account. This happens once only. After any later password change,
the variable is ignored.

## The admin area

Open **Admin** in the top menu (`/admin`). The admin dashboard groups the pages into
sections.

### Platform

| Card | Path | What it is for |
|---|---|---|
| Setup Assistant | `/admin/setup/assistant` | Guided tasks for teams, queues, agents and SLAs |
| User Management | `/admin/users` | Agent accounts |
| Group Management | `/admin/groups` | Groups (teams) |
| Permissions | `/admin/permissions` | Agent permissions per group |
| Roles | `/admin/roles` | Roles and role-group permissions |
| Access Control Lists | `/admin/acl` | Conditional access rules |
| Session Management | `/admin/sessions` | Active user sessions |
| Identity Providers | `/admin/identity-providers` | OIDC and SAML sign-in |
| System Maintenance | `/admin/system-maintenance` | Maintenance windows |
| Plugins | `/admin/plugins` | Installed plugins |
| Marketplace | `/admin/marketplace` | Install plugins from the marketplace |
| Plugin UIs | `/admin/plugin-uis` | Plugin app pages |
| Audit Logs | `/admin/modules/admin_audit_log` | Change and access history |

### GoatFlow (Helpdesk)

| Card | Path | What it is for |
|---|---|---|
| Ticket Attribute Relations | `/admin/ticket-attribute-relations` | Limit which values a ticket form offers |
| Email Identities | `/admin/email-identities` | Salutations, system addresses, signatures |
| Inbound Mail Accounts | `/admin/mail-accounts` | POP3/IMAP mailboxes to fetch |
| Postmaster Filters | `/admin/postmaster-filters` | Route incoming mail by header |
| Auto Responses | `/admin/auto-responses` | Automatic email replies |
| Queue Auto Responses | `/admin/queue-auto-responses` | Link auto responses to queues |
| Ticket Notifications | `/admin/notification-events` | Notifications for ticket events |
| Generic Agent | `/admin/generic-agent` | Automated ticket jobs |
| Webhooks | `/admin/webhooks` | Send ticket events to other systems. See [WEBHOOKS.md](../WEBHOOKS.md) |
| Signatures | `/admin/signatures` | Email signatures |
| Reports & Analytics | `/admin/reports` | Ticket statistics and exports. See [REPORTS.md](../REPORTS.md) |

### Customer Administration

| Card | Path | What it is for |
|---|---|---|
| Customer Organizations | `/admin/customer/companies` | Companies. Each company has Users, Tickets and Services pages |
| Customer Users | `/admin/customer-users` | Customer accounts, import and export |
| Customer User Services | `/admin/customer-user-services` | Which services a customer may pick |
| Customer Portal | `/admin/customer/portal/settings` | Portal settings. See [CUSTOMER_PORTAL.md](../CUSTOMER_PORTAL.md) |
| Customer Groups | `/admin/customer-groups` | Group permissions for customer companies |
| Customer User Groups | `/admin/customer-user-groups` | Link customer users to groups |
| Queues | `/admin/queues` | Queues. Disable a queue with its status toggle |
| Lookups | `/admin/lookups` | Priorities, states and types |
| Services | `/admin/services` | Service catalogue |
| Service Level Agreements | `/admin/sla` | Response and solution targets |
| Dynamic Fields | `/admin/dynamic-fields` | Custom ticket fields |
| Templates | `/admin/templates` | Reply and email templates |
| Attachments | `/admin/attachments` | Files that templates can attach |
| Queue Templates | `/admin/queue-templates` | Which templates each queue offers |
| Template Attachments | `/admin/template-attachments` | Which attachments each template adds |
| Article Colors | `/admin/article-colors` | Article colours per sender type |

### Plugin Administration

Admin pages added by installed plugins appear in this section.

## Settings made outside the admin area

| Topic | How it is set | Details |
|---|---|---|
| LDAP / Active Directory login | `LDAP_*` and `AUTH_PROVIDERS` env vars, or Helm `config.ldap.*` and `config.authProviders`. There is no LDAP page in the admin area. | [LDAP.md](../LDAP.md) |
| OIDC sign-in | Admin -> Identity Providers | [OIDC_INTEGRATION.md](OIDC_INTEGRATION.md) |
| Configuration files and precedence | `config/default.yaml`, environment variables | [configuration.md](../configuration.md) |
| Ticket number generator | `Ticket::NumberGenerator` setting | [ticket_number_generators.md](../ticket_number_generators.md) |
| Attachment storage (database or OTRS-style files) | `storage.type` / `STORAGE_TYPE`; move data with `goatflow-storage` | [ARTICLE_STORAGE.md](../ARTICLE_STORAGE.md) |
| Public URL for emailed links | `BASE_URL` (Helm `config.baseUrl`) | [CUSTOMER_PORTAL.md](../CUSTOMER_PORTAL.md) |
| Encryption key for stored secrets | `GOATFLOW_SECURE_KEY` (64 hex characters). Use the same value for the backend and the runner. | [WEBHOOKS.md](../WEBHOOKS.md) |
| Health checks, metrics, logging, shutdown | `METRICS_*`, `LOG_*`, `DRAIN_TIMEOUT` | [OBSERVABILITY.md](../OBSERVABILITY.md) |

## Ticket notifications

Admin -> Ticket Notifications (`/admin/notification-events`) holds OTRS-style notification
rules. The runner checks new tickets, articles and ticket history every 10 seconds and,
for each valid rule subscribed to the event, sends one email per recipient through
`mail_queue`. Each event is evaluated once, even with several runners. Rules apply only to
events after the runner first started; older tickets are not notified retroactively.

**Events.** TicketCreate, ticket title/queue/type/customer/pending time/lock/state/owner/
responsible/priority updates, TicketMerge (sent for the ticket merged away), the nine
Escalation*Start/NotifyBefore/Stop events written by the escalation check, and ArticleCreate.
Rules imported from OTRS may also use NotificationNewTicket, NotificationAddNote,
NotificationMove, NotificationOwnerUpdate, NotificationResponsibleUpdate,
NotificationEscalation and NotificationEscalationNotifyBefore; the two escalation ones fire
once per ticket and escalation check. An event fires only when GoatFlow writes the matching
`ticket_history` row; changes made by the Generic Agent write none today.

**Filters.** Queue, state, priority and type in the form. Rules imported from OTRS can also
filter on ServiceID, SLAID, LockID, OwnerID, ResponsibleID, CustomerID, CustomerUserID and
the article filters ArticleSenderTypeID, ArticleCommunicationChannelID,
ArticleIsVisibleForCustomer, ArticleSubjectMatch and ArticleBodyMatch (case-insensitive
substring); a rule with an article filter only fires for events that carry an article. The
edit form keeps filters it does not show. A rule with a filter GoatFlow cannot apply (for
example a dynamic field) is skipped and logged by the runner.

**Recipients.** Ticket owner, responsible, creator, watchers, agents with the queue in My
Queues, all agents with read or write access to the queue, chosen agents, members of chosen
groups (read access) or roles, the ticket's customer user, and extra addresses. Agents must
be valid, have an email address and read access to the ticket's queue. The agent who caused
the event is not notified about it (OTRS `AgentSelfNotifyOnAction` off). Each address gets
one email per rule and event. OTRS options `SkipRecipients`, `OncePerDay` and `Transports`
(email only) are honoured. GoatFlow has no per-agent notification opt-out screen.

**Message.** The recipient's language (user preference, else the system default, else
English, else any stored language) picks the message. Tags: `<OTRS_TICKET_*>` (TicketID,
TicketNumber, Title, Queue, State, StateType, Priority, Type, Lock, Service, SLA, Owner,
Responsible, CustomerID, CustomerUserID, Created, Changed and the matching *ID fields),
`<OTRS_OWNER_*>`, `<OTRS_RESPONSIBLE_*>`, `<OTRS_CURRENT_*>` (the acting agent),
`<OTRS_NOTIFICATION_RECIPIENT_*>` and `<OTRS_CUSTOMER_DATA_*>` with UserFirstname,
UserLastname, UserFullname, UserLogin, UserEmail, UserTitle; `<OTRS_CUSTOMER_User*>`,
`<OTRS_CUSTOMER_REALNAME>`; `<OTRS_CUSTOMER_SUBJECT>`, `<OTRS_CUSTOMER_BODY[n]>`,
`<OTRS_CUSTOMER_EMAIL[n]>` (quoted) and the same `AGENT_` tags for the newest customer or
agent article; `<OTRS_EVENT>`. Other tags become `-`. Values are HTML-escaped in text/html
messages. Mail is sent from `email.from` with `Auto-Submitted: auto-generated`, without
attachments or signing. Every email adds a `SendAgentNotification` or
`SendCustomerNotification` entry to the ticket history.

## The background runner

Some work does not happen in the web server. It runs in a separate process started
with `goats -mode runner`. The runner does four things:

- sends outgoing email from `mail_queue` (for example ticket replies and password-reset emails)
- cleans up expired sessions
- delivers webhooks
- evaluates ticket notification rules and queues their emails (see Ticket notifications above)

Docker Compose, the TrueNAS app and the Helm chart (`<fullname>-runner` Deployment) run a
runner. Without a runner, outgoing email stays in the queue, ticket notifications are not
evaluated and webhooks are not delivered.
Inbound mail (Admin -> Inbound Mail Accounts) is fetched by the web server, not the runner.

## Separating departments and clients

GoatFlow follows Znuny here: one install has no ticket separation by tenant.

- **Departments in one install:** every queue belongs to one group. Agents see
  and work only the tickets in queues whose group they hold `ro` or `rw` on,
  directly or through a role (Admin -> Groups, Roles, Queues).
- **Customers:** a customer user sees the tickets of their own customer company
  (plus any extra companies assigned to them).
- **Separate clients:** run one GoatFlow install, with its own database, per
  client.
- **Organisations** (Admin -> Organisations) hold per-organisation settings,
  plugin access, identity providers and plugin data. Switching organisation does
  not change which tickets, queues or customers an agent sees.

## Moving from OTRS or Znuny

`goatflow-migrate` imports an OTRS 6 / Znuny 6.x database. See
[OTRS_MIGRATION_GUIDE.md](../OTRS_MIGRATION_GUIDE.md).

## See also

- [Agent Manual](../agent-manual/README.md)
- [Security](../SECURITY.md)
- [Deployment guides](../deployment/)
- [Troubleshooting](../TROUBLESHOOTING.md)
- [API reference](../api/README.md)
