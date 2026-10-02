# Customer Users and Companies (Admin)

Admins manage customer users and customer companies from the admin dashboard, section
**Customer Administration**: cards **Customer Users** (`/admin/customer-users`) and
**Customer Organizations** (`/admin/customer/companies`). The pages use the OTRS tables as they
are.

## Tables

| Table | Used for |
|-------|----------|
| `customer_user` | Customer user accounts. `customer_id` links the user to a company. |
| `customer_company` | Companies. `customer_id` is the company key. |
| `service_customer_user` | Which services each customer user may pick. |
| `group_customer_user` | Customer user group permissions. |
| `ticket` | Ticket counts per customer user and per company. |
| `sysconfig_modified` | Customer portal settings, global and per company. |

"Delete" on a customer user or a company never removes the row. It sets `valid_id = 2`
(invalid), as OTRS does.

## Customer users

Routes are in `routes/admin.yaml`. Handlers are in `internal/api/admin_customer_users_handlers.go`.

| Method | Path | Handler | What it does |
|--------|------|---------|--------------|
| GET | `/admin/customer-users` | `HandleAdminCustomerUsersList` | List page. Query filters: `search`, `valid`, `customer`. |
| GET | `/admin/customer-users/:id` | `HandleAdminCustomerUsersGet` | One customer user, with company details. |
| POST | `/admin/customer-users` | `HandleAdminCustomerUsersCreate` | Create. Rejects a duplicate login. |
| PUT | `/admin/customer-users/:id` | `HandleAdminCustomerUsersUpdate` | Update all fields. |
| DELETE | `/admin/customer-users/:id` | `HandleAdminCustomerUsersDelete` | Set `valid_id = 2`. |
| GET | `/admin/customer-users/:id/tickets` | `HandleAdminCustomerUsersTickets` | Tickets for this customer user. |
| GET | `/admin/customer-users/import` | `HandleAdminCustomerUsersImportForm` | Import form. |
| POST | `/admin/customer-users/import` | `HandleAdminCustomerUsersImport` | CSV import (form field `csv_file`). |
| GET | `/admin/customer-users/export` | `HandleAdminCustomerUsersExport` | CSV download (`customer_users.csv`). |
| POST | `/admin/customer-users/bulk-action` | `HandleAdminCustomerUsersBulkAction` | `action` = `enable`, `disable` or `delete` for a list of `ids`. |

### CSV import

- The first row is the header. Column names are not case-sensitive.
- Required columns: `login`, `email`, `customer_id`. A row missing one of them fails.
- Optional columns: `title`, `first_name`, `last_name`, `phone`, `fax`, `mobile`, `street`,
  `zip`, `city`, `country`, `comments`.
- A row whose `login` already exists fails. Other rows are still imported.
- The response reports how many rows were imported and how many failed, with a reason per row.

### CSV export

Columns, in order: `login`, `email`, `customer_id`, `title`, `first_name`, `last_name`, `phone`,
`fax`, `mobile`, `street`, `zip`, `city`, `country`, `comments`, `valid_id`, `company_name`.

### Related pages

| Path | What it does |
|------|--------------|
| `/admin/customer-user-services` | Assign services to customer users (`service_customer_user`), and the default services. |
| `/admin/customer-user-groups` | Customer user group permissions. Handlers: `internal/api/admin_customer_user_groups_handlers.go`. |

## Customer companies

| Method | Path | What it does |
|--------|------|--------------|
| GET | `/admin/customer/companies` | Company list. |
| GET/POST | `/admin/customer/companies/new` | New company form and create. |
| GET/POST | `/admin/customer/companies/:id/edit` | Edit form and update. |
| POST | `/admin/customer/companies/:id/delete` | Deactivate (also `DELETE /admin/customer/companies/:id`). |
| POST | `/admin/customer/companies/:id/activate` | Activate again. |
| GET | `/admin/customer/companies/:id/users` | The company's customer users, with ticket counts and status. |
| GET | `/admin/customer/companies/:id/tickets` | Tickets with this `ticket.customer_id`, newest first, 50 per page. Only queues the admin can read, unless they are in the `admin` group. |
| GET/POST | `/admin/customer/companies/:id/services` | Matrix of valid services against the company's customer users, saved in `service_customer_user`. |
| GET/POST | `/admin/customer/companies/:id/portal-settings` | Customer portal settings for this company. |
| GET/PUT/POST | `/admin/customer/portal/settings` | Global customer portal settings. |

OTRS has no company-level service table. That is why the company services page writes one row
per customer user.

## Tests

Run in the toolbox container:

```bash
make toolbox-exec ARGS="go test ./internal/api -run CustomerUser"
```

Test files:

- `internal/api/admin_customer_user_groups_test.go` - customer user group permissions.
- `internal/api/admin_customer_user_services_test.go` - service assignments.
