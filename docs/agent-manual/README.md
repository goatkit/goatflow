# GoatFlow Agent Manual

This page lists what an agent can do in GoatFlow today and where to find it.
You only see tickets in queues your groups or roles give you access to.

## Signing in

| Task | Where |
|---|---|
| Sign in | `/login`. Your administrator may also offer LDAP, OIDC or SAML sign-in. |
| Second factor | After your password, `/login/2fa` asks for an authenticator code, a passkey or a recovery code. |
| Forgotten password | **Forgot password?** link on `/login` (`/forgot-password`). You get a one-hour, single-use link by email. The link appears when `features.lost_password` is on (the default). Emails are only sent when the administrator has set `BASE_URL`. |
| Sign out | User menu -> **Logout** |

There is no sign-up page for agents. An administrator creates agent accounts.

## Main menu

| Menu item | Path | What it shows |
|---|---|---|
| Dashboard | `/dashboard` | Recent tickets and recent activity |
| Tickets | `/tickets` | Ticket list with search and filters |
| Queues | `/queues` | Queues you can read |
| New phone ticket | `/tickets/new?type=phone` | Form for a ticket taken by phone |
| New email ticket | `/tickets/new?type=email` | Form for a ticket sent to a customer by email |
| Admin | `/admin` | Only for members of the `admin` group. See the [Administrator Guide](../admin-guide/README.md). |

Plugins can add more menu items.

## Working on a ticket

A ticket opens at `/ticket/<ticket number>`.

From the ticket page you can:

- reply to the customer
- add a note, and choose whether the customer can see it
- pick a reply template (the list depends on the ticket's queue)
- attach files; images and PDFs show a preview
- change the state; pending states ask for a "pending until" time
- change priority, move to another queue, assign an owner
- merge the ticket into another ticket
- close and reopen the ticket
- record time spent (the **Time Accounting** tab)
- see the ticket history (the **History** tab)
- see linked tickets (the **Links** tab)

Your administrator may make time recording required on notes.

GoatFlow has no ticket split (OTRS "Split"). For a separate request inside a ticket, create a new
ticket for it.

### Many tickets at once

In the ticket list, select tickets and use the bulk actions to change state, priority
or queue, assign, lock or merge them.

## Your profile

Open the user menu -> **Profile** (`/profile`).

| Setting | Notes |
|---|---|
| Name and email | Your own details |
| Language, theme, session timeout | Saved per user |
| Password | **Change Password** link (`/agent/password`). The new password must meet the agent password policy. |
| Two-factor authentication | Authenticator app, passkeys or security keys, and recovery codes |
| Passkeys | Each key shows when it was added and last used, and the host name it was made on. A passkey only works on that host name. Removing a key needs your password. |
| Recovery codes | Shown once when you set up an authenticator app or your first passkey. **New recovery codes** replaces the whole set. |
| API tokens | `/settings/tokens`. Tokens for scripts and integrations. See the [API reference](../api/README.md). |
| Connected accounts | Appears only when a plugin offers a per-user connection, such as your own calendar or mailbox. |

If you lose your passkey and have no recovery codes, ask an administrator.

## Knowledge base

GoatFlow has no built-in knowledge base. The goat-kb plugin adds one when it is installed.

## See also

- [Feature overview](../FEATURES.md)
- [Customer portal](../CUSTOMER_PORTAL.md)
- [API reference](../api/README.md)
- [Security](../SECURITY.md)
