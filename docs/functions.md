# Setup Assistant Functions

## Service Layer (`internal/service/setup_assistant_service.go`)

### Input Types

#### `ResponseTemplateInput`
Canned response template for customer support onboarding.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | `string` | Yes | Display name |
| `Shortcut` | `string` | No | Quick access code (e.g., `/greeting`) |
| `Category` | `string` | No | Category for organization |
| `Content` | `string` | Yes | Response body |
| `ContentType` | `string` | No | `text/plain` or `text/html` (default: `text/html`) |
| `Tags` | `[]string` | No | Search/filter tags |

#### `BusinessHoursInput`
Working hours configuration for SLA calculations.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | `string` | Yes | Configuration name |
| `Timezone` | `string` | No | IANA timezone (e.g., `America/New_York`) |
| `Description` | `string` | No | Description |

#### `EmailTransportInput`
Outbound SMTP configuration.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Host` | `string` | Yes | SMTP server host |
| `Port` | `int` | Yes | Port (25, 465, 587) |
| `User` | `string` | No | Auth username |
| `Password` | `string` | No | Auth password |
| `AuthType` | `string` | No | `plain`, `login`, `crammd5` |
| `TLS` | `bool` | No | Enable TLS |
| `TLSMode` | `string` | No | `none`, `starttls`, `smtps` |
| `From` | `string` | No | Default from address |
| `FromName` | `string` | No | Display name for sender |

### Service Methods

#### `CreateCannedResponses(ctx, templates []ResponseTemplateInput, createBy int) error`
Creates canned response templates in the `canned_response` table. Skippable (returns nil for empty list).

#### `CreateEmailTransport(ctx, cfg EmailTransportInput, createBy int) error`
Stores SMTP config as JSON in sysconfig under key `Email.SMTP.Config`.

#### `CreateBusinessHours(ctx, cfg BusinessHoursInput, createBy int) error`
Creates a calendar entry in the `calendar` table for SLA working hours. Stores timezone reference in sysconfig.

#### `LoadExistingCustomer(ctx, customerID string) (*CustomerConfiguration, error)`
Loads complete existing customer configuration for review/edit mode. Returns:
- Customer company details
- Associated groups/teams
- Queues with group assignments
- Agents with team assignments
- Mail accounts (via queue associations)
- Services and SLAs
- Canned responses
- Business hours
- Email transport config

### Wizard Steps

| Step | Feature | Skippable | Description |
|------|---------|-----------|-------------|
| 6 | Response Templates | Yes (empty list) | Creates canned responses |
| 7 | Business Hours | Yes (empty list) | Creates calendar entries |
| 8 | Email Transport | Yes (nil pointer) | Stores SMTP config |

## API Layer (`internal/api/admin_setup_handlers.go`)

### Task Handlers

#### `POST /admin/setup/task/setup-assistant/create_response_template`
Accepts JSON array of `ResponseTemplateInput`.

#### `POST /admin/setup/task/setup-assistant/configure_business_hours`
Accepts JSON `BusinessHoursInput`.

#### `POST /admin/setup/task/setup-assistant/configure_email_transport`
Accepts JSON `EmailTransportInput`.

### Customer Search

#### `GET /api/v1/admin/setup/customers/search/:query`
Type-ahead search for customer companies. Returns:
```json
{
  "success": true,
  "data": [
    {"value": "ABC", "label": "ABC Corporation (ABC)"}
  ]
}
```

### Wizard Review Mode

#### `GET /admin/setup/wizard?existing_customer_id=ID`
Loads existing customer config and renders wizard in review mode. Sets template context:
- `ExistingCustomerID` - the customer ID
- `ReviewMode` - `true`
- `CustomerConfig` - `*CustomerConfiguration` with full config

## Setup Tasks

| Task ID | Category | Icon | Handler |
|---------|----------|------|---------|
| `create_response_template` | templates | `fa-comments` | `create_response_templates` |
| `configure_business_hours` | sla | `fa-calendar` | `configure_business_hours` |
| `configure_email_transport` | system | `fa-envelope` | `configure_email_transport` |

## i18n Keys

All task titles and descriptions use i18n keys under `setup_assistant_tasks`:

```
setup_assistant_tasks.<task_id>.title
setup_assistant_tasks.<task_id>.description
```

Available in all 15 languages: ar, de, en, es, fa, fr, he, ja, pl, pt, ru, tlh, ur, zh.
