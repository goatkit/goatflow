# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/) and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]



## [0.6.4] - 2026-02-01

### Added
- **GoatKit Plugin Platform Documentation**: New `docs/PLUGIN_PLATFORM.md` describing the planned plugin architecture for v0.7.0
  - WASM runtime (wazero) for portable, sandboxed plugins
  - gRPC runtime (go-plugin) for native integrations
  - Host function API specification
  - Plugin packaging and lifecycle documentation

### Changed
- **Roadmap Update**: 0.7.0 now focused on GoatKit Plugin Platform
  - Dual runtime support (WASM + gRPC)
  - Statistics & Reporting ships as first WASM plugin
  - FAQ, Calendar, Process Management planned as subsequent plugins
- **Architecture Documentation**: Updated to reflect plugin platform vision
  - `docs/ARCHITECTURE.md`: Added Platform Roadmap section
  - `docs/DYNAMIC_MODULES.md`: Links to plugin platform evolution
  - `docs/MICROSERVICES_ARCHITECTURE.md`: Marked as design document for future consideration
  - `docs/VISION.md`: Aligned architecture evolution with plugin roadmap

### Fixed
- **Handler Registry Dual Registration**: `RegisterHandler()` now registers to both local `handlerRegistry` and `routing.GlobalHandlerMap`
  - Fixes "Handler not found" warnings for handlers like `HandleListServicesAPI` that used `RegisterHandler()` in their `init()` functions
  - YAML route loader looks in `GlobalHandlerMap`, so handlers must be in both registries
  - Prevents future handler wiring issues when adding new API endpoints
- **90s Theme Button Contrast**: Fixed poor text contrast on bright-colored buttons in dark mode
  - Buttons with ANSI bright backgrounds (green, yellow, cyan, red) now use dark text instead of white
  - Affects: `.gk-btn-success`, `.gk-btn-danger`, bulk action buttons, and any button with inline bright color styles
  - Improves readability across ticket actions, admin modals, and bulk operations

## [0.6.3]

### Added
- **Multi-arch Playwright E2E Tests**: E2E tests now run on both amd64 and arm64 (e.g., DGX Spark, Apple Silicon)
  - `Dockerfile.playwright-go` auto-detects architecture for Go toolchain and browser downloads
  - Playwright driver and browsers installed to shared location (`/opt/playwright-cache`) accessible by all users
  - Makefile uses `NATIVE_PLATFORM` detection for `docker build/run` commands
  - Files: `Dockerfile.playwright-go`, `Makefile`
- **Type Conversion Package**: New `internal/convert` package consolidating duplicate type conversion functions
  - `ToInt()`, `ToUint()`, `ToString()` functions with fallback values
  - Handles all numeric types (int8-64, uint8-64, float32/64) and string parsing
  - Breaks circular dependency between shared and middleware packages
  - Files: `internal/convert/convert.go`, `internal/convert/convert_test.go`

### Changed
- **Single YAML Route Loader**: Consolidated to one route loader for both production and tests
  - Tests now authenticate the same way production does (no test auth bypass)
  - `internal/routing/loader.go` is the single source of truth
  - `internal/api/yaml_router_loader.go` only used for manifest generation tooling
- **Test Database Setup**: Enhanced `resetTestDatabase()` with proper OTRS-compatible permissions
  - Creates canonical groups (users, admin, stats, support)
  - Grants user 1 'rw' permission via group_user table for queue access middleware
  - Sets queue group_id for proper queue access checks
- **Test Authentication**: All API tests now use centralized auth helpers
  - `GetTestAuthToken(t)` generates valid JWT tokens
  - `AddTestAuthCookie(req, token)` adds auth cookie to requests
  - Middleware files updated to use `convert` package instead of inline type switches

### Fixed
- **Customer User Lookup by Login or Email**: Fixed "Customer user not found" errors in ticket zoom when `customer_user_id` contains email instead of login
  - Customer user queries now match on `login = ? OR email = ?` since tickets may store either value
  - Updated 4 files: `ticket_detail_handlers.go`, `notifications/context.go`, `agent_ticket_actions.go`, `ticket_create_with_attachments.go`
- **Queue Access in Tests**: Fixed "You do not have access to any queues" errors
  - Test user now has proper group_user records with 'rw' permission
  - Queue records include group_id for permission checks
- **Test Database Connection**: Fixed "sql: database is closed" errors in attachment tests
  - Get fresh DB connection after `WithCleanDB(t)` call
- **UI Tests**: Fixed TestNavigationVisibility, TestAccessibility, TestErrorPages
  - Added proper authentication to all tests
  - Corrected route paths (`/ticket/new` not `/tickets/new`)
  - Simplified navigation test to focus on admin portal

## [0.6.2] - 2026-01-25

### Added
- **Multi-Theme System**: Pluggable theming architecture with four themes and light/dark mode support
  - **Synthwave** (default): Neon cyan/magenta color scheme with grid background and glow effects
  - **GOTRS Classic**: Professional blue theme with clean solid backgrounds, no visual patterns
  - **Seventies Vibes**: Warm retro palette with orange/brown tones and ogee wave pattern
  - **Nineties Vibe**: Dual-personality theme with distinct light and dark aesthetics
    - Light mode: Classic 90s Redmond desktop with gray windows, navy title bars, 3D beveled controls
    - Dark mode: Linux terminal aesthetic with pure black (#000000) background, ANSI bright colors, Hack Nerd Font
  - Theme switcher UI in settings and login pages with live preview
  - CSS custom properties architecture (`--gk-*` variables) for consistent theming
  - Theme-specific font loading via ThemeManager
  - Preference persistence to database for authenticated users
  - Files: `static/css/themes/*.css`, `static/js/theme-manager.js`, `templates/partials/theme_selector.pongo2`
- **Vendored Fonts**: Self-hosted web fonts for offline/air-gapped deployments
  - Inter (400, 500, 600, 700) - universal fallback
  - Space Grotesk - synthwave headings
  - Righteous - synthwave display
  - Nunito - seventies vibes body text
  - Hack Nerd Font - nineties vibe terminal mode (monospace with Nerd Font icons)
  - Archivo Black - nineties vibe light mode headings
  - Dynamic font loading based on active theme
  - Files: `static/fonts/`, `static/css/fonts-*.css`
- **Ticket Detail Page Refactoring**: Modular partial architecture for ticket zoom view
  - 17 reusable partials: header, description, notes, sidebar, tabs, meta_grid, alerts, attachments, note_form, priority_badge, status_badge, and 6 modal partials
  - Consistent theming via CSS custom properties
  - Improved maintainability and testability
  - Files: `templates/partials/ticket_detail/*.pongo2`
- **Bulk Ticket Actions**: Multi-select ticket operations on agent ticket list
  - Floating action bar appears when tickets are selected
  - Actions: bulk assign, bulk merge, bulk priority change, bulk queue transfer, bulk status change
  - Modal dialogs for each action with confirmation
  - Files: `templates/partials/agent/tickets/bulk_*.pongo2`, `internal/api/agent_ticket_bulk_actions.go`
- **Language Selector Partial**: Reusable dropdown for language selection
  - Shows all 15 supported languages with native names
  - Used in settings, profile, and login pages
  - File: `templates/partials/language_selector.pongo2`
- **Customer Password Change**: Password change functionality for customer portal
  - Accessible from customer profile page
  - Current password verification required
  - Files: `templates/pages/customer/password_form.pongo2`, `templates/pages/password_form.pongo2`
- **Ticket List Pagination**: Server-side pagination for ticket lists
  - Configurable page size
  - Page navigation controls
  - File: `templates/partials/tickets/pagination.pongo2`
- **Customer Profile Page**: Full profile management interface at `/customer/profile` for customer users
  - View and edit personal information (first name, last name, title, phone, mobile)
  - Language preference selection with all 15 supported languages
  - Session timeout preference with configurable durations (1 hour to 7 days)
  - Link to password change functionality
  - Avatar with customer initials display
  - Full i18n support for all 15 languages
  - Files: `templates/pages/customer/profile.pongo2`, `internal/api/customer_routes.go`
- **Customer Dashboard as Default Landing Page**: Customer login now redirects to `/customer` (dashboard) instead of `/customer/tickets`
  - Dashboard tiles are clickable and link to filtered ticket lists (open, closed, all)
  - Removed sysconfig override for customer landing page - now hardcoded in code
  - Files: `internal/api/auth_customer.go`, `templates/pages/customer/dashboard.pongo2`
- **Admin Ticket Attribute Relations**: Full CRUD interface at `/admin/ticket-attribute-relations` for managing ticket attribute relationships (OTRS AdminTicketAttributeRelations equivalent)
  - Define relationships between ticket attributes (Queue, State, Priority, Type, Service, SLA, Owner, Responsible, DynamicField_*)
  - CSV and Excel (.xlsx) file upload support for bulk relationship import
  - "Add missing values to dynamic field config" checkbox for auto-populating dropdown options
  - Priority-based ordering with drag-and-drop reordering
  - Red highlighting for values missing from dynamic field's PossibleValues
  - Download previously imported file
  - ACL-based filtering integrated with ticket forms via `/api/v1/ticket-attribute-relations/evaluate`
  - Full i18n support for all 15 languages
  - Files: `internal/services/ticketattributerelations/service.go`, `internal/api/admin_ticket_attribute_relations_handlers.go`, `templates/pages/admin/ticket_attribute_relations.pongo2`

### Changed
- **Separate Cookie Names for Agent/Customer Sessions**: Agent and customer portals now use distinct cookie names to allow simultaneous login in the same browser
  - Agent cookies: `access_token`, `auth_token`, `session_id`, `gotrs_logged_in`
  - Customer cookies: `customer_access_token`, `customer_auth_token`, `customer_session_id`, `gotrs_customer_logged_in`
  - Theme manager updated to detect both login indicators for preference persistence
  - Files: `internal/api/auth_customer.go`, `internal/api/auth_htmx_handlers.go`, `internal/api/handler_registry.go`, `internal/middleware/auth.go`, `internal/middleware/session.go`, `internal/routing/handlers.go`, `static/js/theme-manager.js`

### Fixed
- **Seventies Vibes Theme Background Interference**: Fixed dual-layer background pattern causing visual interference when scrolling
  - Body, sidebar, and grid pseudo-element all had the ogee wave pattern with different `background-attachment` values
  - Removed pattern from sidebar and `.gk-grid-bg::before`, keeping only body background
  - Changed `background-attachment` from `fixed` to `scroll` for natural scrolling behavior
  - Restored solid earthy brown backgrounds on cards and panels
  - File: `static/css/themes/seventies-vibes.css`
- **Guru Meditation HTMX Compatibility**: Fixed duplicate declaration errors when Guru Meditation component was loaded multiple times via HTMX
  - Added initialization guard to prevent re-declaration of functions
  - Changed local variables to window-scoped to avoid redeclaration errors
  - Functions now attached to window object for global access
  - File: `templates/components/guru_meditation.pongo2`
- **Customer-Authored Content Badge**: Fixed ticket description and notes showing "Customer sees this" badge instead of "Customer wrote this" when customer authored the content
  - Added `first_article_sender_type` field to ticket detail handler to track who wrote the initial description
  - Updated `description.pongo2` and `notes.pongo2` templates to check sender type before visibility
  - Added i18n translation key `tickets.customer_wrote_badge` to all 15 languages
  - Files: `internal/api/ticket_detail_handlers.go`, `templates/partials/ticket_detail/description.pongo2`, `templates/partials/ticket_detail/notes.pongo2`
- **Duplicate Attachment Upload in Customer Portal**: Removed duplicate file upload area from customer ticket view
  - Was showing both "Add Attachments" section and "Attach Files" in reply form
  - Now only shows attachment upload within the reply form (matching agent UI behavior)
  - File: `templates/pages/customer/ticket_view.pongo2`
- **Pending Reminder Snooze Toast Color**: Fixed snooze success showing red toast instead of green on admin/ticket-attribute-relations page
  - Page had local `showToast(type, message)` with reversed parameter order compared to global `showToast(message, type)` in common.js
  - Removed local function and updated all calls to use global signature
  - File: `templates/pages/admin/ticket_attribute_relations.pongo2`
- **Customer Initials Display**: Fixed customer initials in navigation showing only first letter instead of two letters (e.g., "E" instead of "ES" for Emma Scott)
  - Root cause: Template checked `User` before `Customer`, and session middleware set `user_name` to email (no space), causing only first letter to be extracted
  - Solution: Changed template to check `Customer.initials` first; added `is_customer` check in pongo2 renderer to skip auto-injecting incomplete `User` object for customer contexts
  - Added regression tests: unit test for template logic, integration test for `getCustomerInfo`, E2E Playwright test for header/profile initials
  - Files: `templates/layouts/base.pongo2`, `internal/template/pongo2.go`, `internal/template/pongo2_test.go`, `internal/api/customer_profile_test.go`, `tests/acceptance/customer-profile-initials.spec.js`
- **Missing i18n Translations**: Fixed untranslated strings in profile-related keys
  - `common.email` in Ukrainian: "Email" → "Ел. пошта"
  - `messages.unknown_error` in 10 languages (pt, pl, ru, zh, ja, ar, he, fa, ur, tlh) now properly translated
  - Files: `internal/i18n/translations/*.json`
- **Fix Agent Password Reset** : Fix regression in password reset feature.

## [0.6.1] - 2026-01-17

### Added
- **Pending Reminder/Auto-Close i18n**: Full internationalization for pending reminder and auto-close ticket state popups
  - Added translation keys for `tickets.pending_reminder.*` (overdue, scheduled, not_scheduled, was_scheduled_for, will_reopen_at, ago, in, no_time_scheduled, title, help)
  - Added translation keys for `tickets.auto_close.*` (overdue, scheduled, should_have_closed_at, will_close_at, while_pending, at, plus_title, plus_help, minus_title, minus_help)
  - Translations added for all 15 supported languages including RTL languages (Arabic, Hebrew, Persian, Urdu)
  - Files: `internal/i18n/translations/*.json`, `templates/pages/ticket_detail.pongo2`, `templates/pages/agent/ticket_view.pongo2`
- **Group-Based Queue Permission Enforcement**: Security-first middleware-layer enforcement of group-based queue permissions (Issue #160, OTRS-compatible)
  - **Architecture**: Permissions enforced at routing/middleware layer, not in handlers - secure by default
  - Permission types: `ro` (read-only), `rw` (full access - supersedes all), `create`, `move_into`, `note`, `owner`, `priority`
  - **Middleware Registration** (`internal/routing/handlers.go`):
    - `queue_ro`, `queue_rw`, `queue_create` - require access to at least one queue
    - `queue_access_*` - check specific queue from URL/query param
    - `ticket_access_*` - check ticket's queue from ticket ID/number in URL
  - **Route Protection** (YAML declarative): Routes declare required permissions in `middleware` list
    - `/ticket/:id` requires `ticket_access_ro`
    - `/tickets/:id/note` requires `ticket_access_note`
    - `/ticket/new` requires `queue_create`
    - Dashboard/ticket list require `queue_ro`
  - Queue Access Service (`internal/service/queue_access_service.go`): Core permission logic combining direct (group_user) and role-based (role_user → group_role) permissions
  - Context enrichment: Middleware sets `is_queue_admin` and `accessible_queue_ids` for downstream handlers
  - Ticket ID/number support: Middleware handles both numeric IDs and ticket numbers (tn field)
  - Full i18n support for queue permission messages in all 15 languages
  - Unit tests for service and middleware

### Changed
- **OTRS-Compatible Template Variable Substitution**: Unmatched template variables (`<OTRS_*>` and `<GOTRS_*>`) are now replaced with `-` instead of left unchanged
  - Matches OTRS behavior for cleaner rendered output
  - Handles both raw tags (`<GOTRS_VAR>`) and HTML-encoded tags (`&lt;GOTRS_VAR&gt;`)
  - Files: `internal/api/agent_templates_handlers.go`

### Fixed
- **Template Selector Editor Mode**: Fixed HTML templates not switching the rich text editor to HTML/richtext mode
  - Added HTML content auto-detection when `content_type` is incorrectly set to `text/plain`
  - Detects HTML tags in content and switches editor mode accordingly
  - File: `templates/partials/template_selector.pongo2`
- **Note Submission "Please enter note content" Error**: Fixed form submission failing with content validation error
  - Issue was that programmatic `setContent()` in TipTap editor didn't trigger the `onUpdate` callback
  - Added explicit manual sync to hidden textarea after setting template content
  - File: `static/js/tiptap-editor.js`
- **Template Variable Substitution for HTML-Encoded Tags**: Fixed GOTRS/OTRS variables not being substituted when stored as HTML entities
  - Template content in database had `&lt;GOTRS_*&gt;` instead of `<GOTRS_*>`
  - Now handles both raw and HTML-encoded template variable formats
  - File: `internal/api/agent_templates_handlers.go`

## [0.6.0] - 2026-01-16

### Added
- **Admin System Maintenance Module**: Full CRUD interface at `/admin/system-maintenance` for scheduling maintenance windows (OTRS AdminSystemMaintenance equivalent)
  - Schedule maintenance periods with start/stop times (epoch timestamps)
  - Display notifications to logged-in users via banner when maintenance is active or upcoming
  - Login page message display when ShowLoginMessage is enabled
  - Session management: view and kill agent/customer sessions during maintenance
  - Configurable notification timing via `maintenance.time_notify_upcoming_minutes` (default: 30 minutes)
  - Default messages configurable: `maintenance.default_notify_message`, `maintenance.default_login_message`
  - Full i18n support for all 15 languages with proper native translations
  - Date format: "from {start} until {stop}" with translated prepositions
  - Files: `internal/models/system_maintenance.go`, `internal/repository/system_maintenance_repository.go`, `internal/api/admin_system_maintenance_handlers.go`, `templates/pages/admin/system_maintenance*.pongo2`
- **Admin Session Management**: Full session management interface at `/admin/sessions` (OTRS AdminSession equivalent)
  - View all active user sessions with user details, IP address, browser info, login time, last activity
  - Kill individual sessions to force user logout
  - Kill all sessions for a specific user
  - Kill all sessions (emergency action with confirmation)
  - Current session indicator (asterisk) to avoid self-logout
  - Session enforcement in auth middleware - killed sessions immediately invalidate
  - Background session cleanup task via runner (configurable interval, default 5 minutes)
  - Cleans up sessions exceeding max age (7 days) and idle sessions (2 hours)
  - Configuration: `runner.session_cleanup.interval` in YAML config
  - Files: `internal/api/admin_session_handlers.go`, `internal/repository/session_repository.go`, `internal/service/session_service.go`, `internal/runner/tasks/session_cleanup.go`, `templates/pages/admin/sessions.pongo2`
- **Phone/Email Ticket Creation Entry Points**: Separate navigation links for creating phone and email tickets (mirrors OTRS AgentTicketPhone/AgentTicketEmail)
  - Two direct links in top navigation and agent dashboard quick actions
  - URL parameter `?type=phone|email` pre-selects interaction type on the new ticket form
  - Form displays colored left border based on interaction type (colors loaded from `article_color` database table)
  - i18n translations for all 15 languages: `tickets.new.phone_ticket`, `tickets.new.email_ticket`
  - Files: `internal/api/agent_ticket_new_handler.go`, `templates/pages/tickets/new.pongo2`, `templates/layouts/base.pongo2`, `templates/pages/agent/dashboard.pongo2`
- **Admin Article Color Module**: Dynamic module at `/admin/article-colors` for managing article sender type colors (OTRS AdminArticleColor equivalent)
  - Full CRUD for article color configuration (agent, customer, system sender colors)
  - Dashboard link with palette icon in System Administration section
  - i18n translations for all 15 languages (en, de, es, fr, pt, pl, ru, zh, ja, ar, he, fa, ur, uk, tlh)
- **Generic Agent Execution Engine**: Automated ticket processing via scheduled jobs (OTRS GenericAgent equivalent)
  - Job scheduler integration: runs jobs based on ScheduleDays/ScheduleHours/ScheduleMinutes
  - Ticket matching: StateIDs, QueueIDs, PriorityIDs, TypeIDs, LockIDs, OwnerIDs, ServiceIDs, SLAIDs, CustomerID (wildcards), Title, time-based filters (create/change/pending/escalation older/newer minutes)
  - Actions: NewStateID, NewQueueID, NewPriorityID, NewOwnerID, NewResponsibleID, NewLockID, NewTypeID, NewServiceID, NewSLAID, NewCustomerID, NewTitle, NoteBody/NoteSubject, NewPendingTime, Delete
  - Repository for OTRS key-value job storage format with schedule parsing
  - Comprehensive unit tests and end-to-end verification
  - Files: `internal/models/generic_agent_job.go`, `internal/repository/generic_agent_repository.go`, `internal/services/genericagent/service.go`
- **ACL Execution Engine**: Access Control List evaluation for filtering ticket form options (OTRS TicketACL equivalent)
  - Property matching: Properties (frontend values) and PropertiesDatabase (DB values)
  - Supports wildcards (`*`), negation (`[Not]`), and regex (`[RegExp]`)
  - Change rules: Possible (whitelist), PossibleAdd (add to options), PossibleNot (blacklist)
  - StopAfterMatch support for halting ACL chain processing
  - Filter methods for States, Queues, Priorities, Types, Services, SLAs, and Actions
  - API helper for easy integration with ticket handlers
  - Unit tests and integration tests against live database
  - Files: `internal/models/acl.go`, `internal/repository/acl_repository.go`, `internal/services/acl/service.go`, `internal/api/acl_helper.go`
- **i18n Expansion**: Extended language support from 8 to 15 languages with extensive native translations across the UI
  - Added Japanese (ja) with full native phrasing and typographic conventions
  - Added Russian (ru) with comprehensive Cyrillic translations and ₽ currency support
  - Added Ukrainian (uk) with dedicated Ukrainian vocabulary and ₴ currency support
  - Added Urdu (ur) including full RTL handling
  - Added Hebrew (he) with broad RTL coverage and localized phrasing
  - Added Chinese (zh) with extensive Simplified Chinese copy
  - Added Persian (fa) with deep RTL support and Persian numerals
  - Language configs in `rtl.go` include locale-specific date/time/number/currency formatting
  - `GetEnabledLanguages()` now auto-detects languages based on JSON file existence
- **Customer Groups Admin**: Full CRUD interface at `/admin/customer-groups` for managing customer company group permissions (OTRS AdminCustomerGroup equivalent)
  - Two-way management: edit permissions by customer or by group
  - Permission types: ro (read-only) and rw (read-write) access
  - Client-side group filtering with server-side customer search
  - Integration tests with real database
  - Files: `internal/api/admin_customer_groups_handlers.go`, templates in `templates/pages/admin/customer_group*.pongo2`
- **Customer User Groups Admin**: Full CRUD interface at `/admin/customer-user-groups` for managing individual customer user group permissions (OTRS AdminCustomerUserGroup equivalent)
  - Two-way management: edit permissions by customer user or by group
  - Permission types: ro (read-only) and rw (read-write) access for portal ticket visibility
  - Client-side group filtering with server-side customer user search (login, name, email)
  - Uses `group_customer_user` table from OTRS baseline schema
  - Comprehensive integration tests (11 test cases)
  - Files: `internal/api/admin_customer_user_groups_handlers.go`, templates in `templates/pages/admin/customer_user_group*.pongo2`
- **Queue Auto Response Admin**: Dynamic module at `/admin/queue-auto-responses` for mapping queues to auto-response templates
  - Lookup display resolution shows queue names and auto-response names instead of IDs
  - i18n translations for all 6 languages (en, de, es, fr, ar, tlh)
- **Auto Response Admin**: Dynamic module at `/admin/auto-responses` for managing automatic email response templates
  - Full CRUD with template variable support for dynamic content
  - i18n translations including template variable labels
- **Postmaster Filter Admin UI**: Full CRUD interface at `/admin/postmaster-filters` for managing database-backed email routing filters
  - Create, edit, and delete filters with match conditions (regex patterns on headers/body) and set actions (X-GOTRS-* headers)
  - Dynamic form inputs with type-ahead search for queue, priority, state, and type selections
  - Boolean dropdown for X-GOTRS-Ignore action
  - NOT operator support for negative match conditions
  - Stop flag to halt further filter processing after match
  - Navigation link added to admin dashboard under System Administration
- **HTML Structure Validation for Templates**: Centralized HTML tag balance validation in template test suite
  - `ValidateTagBalance()` function using stack-based approach with `golang.org/x/net/html` tokenizer
  - Integrated into `TemplateTestHelper.RenderAndValidate()` for automatic validation
  - All 90+ page templates now validated for missing/mismatched tags on every test run
  - Catches bugs like missing `</div>` that cause UI elements to become invisible
  - Files: `internal/template/html_validator.go`, `internal/template/html_validator_test.go`
- **Scalable Role Users Management**: Search-based user assignment for roles (handles thousands of users)
  - New API endpoint `GET /admin/roles/:id/users/search?q=xxx` with debounced typeahead
  - Replaces "load all users" pattern with search-first design (LIMIT 20 results)
  - Minimum 2 characters required to search, 300ms debounce
  - Member count display, loading spinner, auto-focus on search input
- **DBSourceFilter**: Email filter that loads postmaster filters from database and applies them to incoming mail (equivalent to OTRS `PostMaster::PreFilterModule###000-MatchDBSource`)
  - Runs first in filter chain before token extraction filters
  - Supports all X-GOTRS-* headers: Queue, QueueID, Priority, PriorityID, State, Type, Title, CustomerID, CustomerUser, Ignore
  - Comprehensive test coverage for VIP routing, spam filtering, NOT matches, multi-match conditions, and stop flag behavior
- **PostmasterFilter Repository**: Database repository for `postmaster_filter` table with YAML serialization for match/set rules
- **Dynamic Fields Import/Export**: Admin UI for importing and exporting dynamic field configurations (OTRS AdminDynamicFieldConfigurationImportExport equivalent)
  - Export: Select multiple dynamic fields and download as YAML file with complete field definitions
  - Import: Upload YAML file or paste YAML content directly, with preview before confirmation
  - Handles all dynamic field types: Text, Textarea, Checkbox, Date, DateTime, Dropdown, Multiselect
  - Routes: `/admin/dynamic-fields/export`, `/admin/dynamic-fields/import`, `/admin/dynamic-fields/import/confirm`
  - Files: `internal/api/admin_dynamic_fields_handlers.go`, `templates/pages/admin/dynamic_field_export.pongo2`, `templates/pages/admin/dynamic_field_import.pongo2`
- **Dynamic Fields Auto-Configuration**: Simplified field creation with automatic default configuration (OTRS AdminDynamicFieldAutoConfig equivalent)
  - Auto-config checkbox for supported field types: Text, TextArea, Checkbox, Date, DateTime
  - Automatically applies sensible defaults (MaxLength=200 for Text, Rows=4/Cols=60 for TextArea, YearsInPast/Future=5 for dates)
  - Hides type-specific configuration UI when auto-config is enabled
  - Auto-enabled by default for new fields of supported types
  - Dropdown and Multiselect still require manual PossibleValues configuration
  - Files: `internal/api/dynamic_field_types.go`, `templates/pages/admin/dynamic_field_form.pongo2`
- **GenericInterface Webservice Framework**: Full implementation of OTRS GenericInterface for external webservice integration
  - **Webservice Repository**: CRUD operations for `gi_webservice_config` table with YAML config serialization, history tracking, and restore functionality
  - **GenericInterface Service**: Core execution engine with transport abstraction, invoker routing, and request/response data mapping
  - **REST Transport**: Full HTTP REST support with GET/POST/PUT/DELETE, path parameter substitution (`:id`), query params, JSON body, Basic/APIKey authentication, custom headers
  - **SOAP Transport**: Full SOAP 1.1 support with envelope construction, SOAPAction header handling, namespace prefixes, fault parsing, Basic authentication
  - **WebserviceDropdown/WebserviceMultiselect Dynamic Fields**: New field types for autocomplete-based selection backed by external webservices
  - **WebserviceFieldService**: Autocomplete search, display value retrieval, result caching, multi-value support for multiselect fields
  - **OTRS-Compatible Response Format**: `StoredValue`/`DisplayValue` JSON field names matching OTRS expected format
  - **Admin UI**: Webservice management at `/admin/webservices` with create/edit/delete, connection testing, and configuration history
  - **AJAX Endpoints**: `/admin/api/dynamic-fields/:id/autocomplete` for field autocomplete, `/admin/api/dynamic-fields/:id/webservice-test` for config testing
  - **Comprehensive Integration Tests**: Mock REST and SOAP servers as fixtures, tests for transport execution, authentication, fault handling, data mapping, caching, and full service invocation
  - Files: `internal/repository/webservice_repository.go`, `internal/service/genericinterface/service.go`, `internal/service/genericinterface/transport_rest.go`, `internal/service/genericinterface/transport_soap.go`, `internal/service/genericinterface/webservice_field.go`, `internal/api/admin_webservice_handlers.go`, `internal/api/admin_dynamic_field_webservice_ajax.go`
- **Admin Queue Templates**: Full CRUD interface at `/admin/queue-templates` for managing queue↔template assignments (OTRS AdminQueueTemplates equivalent)
  - Two-column overview showing all queues and templates with assignment counts
  - Queue-side editing: assign templates to a queue at `/admin/queues/:id/templates`
  - Links to existing template-side editing at `/admin/templates/:id/queues`
  - Smart redirect back to overview after saving assignments
  - Dashboard navigation link with link icon
  - i18n translations for all 15 languages
  - Files: `internal/api/admin_queue_templates_handlers.go`, `templates/pages/admin/queue_templates.pongo2`, `templates/pages/admin/queue_templates_edit.pongo2`
- **Admin Template Attachments**: Full CRUD interface at `/admin/template-attachments` for managing template↔attachment assignments (OTRS AdminTemplateAttachment equivalent)
  - Two-column overview showing all templates and attachments with assignment counts
  - Attachment-side editing: assign templates to an attachment at `/admin/attachments/:id/templates`
  - Links to existing template-side editing at `/admin/templates/:id/attachments`
  - Smart redirect back to overview after saving assignments
  - Dashboard navigation link with file-circle-plus icon
  - i18n translations for all 15 languages
  - Files: `internal/api/admin_template_attachments_handlers.go`, `templates/pages/admin/template_attachments_overview.pongo2`, `templates/pages/admin/attachment_templates_edit.pongo2`

### Changed
- **Humanized Duration Display**: Reminder toast notifications now show overdue/due times in human-readable format (e.g., "4 months" instead of "3390h 20m") for periods exceeding 2 days
- **Translation Coverage Test Output**: `TestTranslationCompleteness` now prints a formatted ASCII table showing every enabled language and highlights the ones that are fully translated
- **Test Runner Enhancement**: `scripts/test-runner.sh` now tracks individual test counts (not just packages) and displays the i18n coverage table in the summary output
- **http-call Script**: `scripts/http-call.sh` now uses JSON API login to extract `access_token` via Bearer authentication instead of cookie-based session handling

### Fixed
- **Pending Reminder Snooze Buttons**: Fixed "response.json is not a function" error when clicking sleep/snooze buttons on pending reminder toast notifications
  - `snoozeReminder()` was using `apiFetch()` then calling `response.json()` on the result
  - But `apiFetch()` returns parsed JSON data, not a Response object
  - Fixed by using plain `fetch()` with proper credentials and Accept headers
  - File: `static/js/common.js`
- **Escalation History Recording**: Fixed "Field 'type_id' doesn't have a default value" error when recording escalation events
  - INSERT into `ticket_history` was missing required columns: `type_id`, `queue_id`, `owner_id`, `priority_id`, `state_id`
  - Fixed by fetching current ticket values before inserting history record
  - Added integration test `TestRecordEscalationEventIntegration` to prevent regression
  - File: `internal/services/escalation/check.go`
- **Customer Typeahead Race Condition**: Fixed GoatKit autocomplete initialization timing issue where the input was set up before seed data was loaded
  - Added retry logic in `setupInput()` to load seeds on-demand if not yet available
  - Added late seed loading in `refresh()` to check for seed data at query time
  - Restores customer user search, auto queue selection, and customer info panel on new ticket form
  - File: `static/js/goatkit-autocomplete.js`
- **Dynamic Module Lookup Display**: Fixed template rendering to show lookup display values (e.g., queue names) instead of raw IDs for integer foreign key fields in both `allFields` and regular `fields` template sections
- **Remaining $N Placeholder Conversion**: Fixed remaining `$%d` format-string placeholders that were missed in the v0.5.1 SQL portability refactor, causing `ConvertPlaceholders: $N placeholders are not allowed` panics
  - `internal/api/admin_attachment_handler.go` - handleAdminAttachmentUpdate
  - `internal/api/agent_templates_handlers.go` - GetTemplatesForQueue
  - `internal/api/admin_customer_company.go` - search query construction
  - `internal/api/v1/handlers_tickets.go` - handleUpdateTicket
  - `internal/repository/article_repository.go` - Create placeholder generation
  - `cmd/gotrs-storage/main.go` - storage migration query construction
- **Template Type Chips Display**: Fixed comma-separated template types (e.g., "Answer,Note,Snippet") to display as individual colored chips instead of a single unstyled text string
  - Uses pongo2 `|split:","` filter to iterate and render each type with its corresponding color
  - Applied to all template-related admin pages: queue_templates, queue_templates_edit, template_attachments_overview, attachment_templates_edit

### Internal
- **Lookup Display Tests**: Added unit tests for `processLookups`, `coerceString`, and lookup field configuration in `handler_lookup_test.go`


## [0.5.1] - 2026-01-08

### Added
- **AVIF/HEIC Thumbnail Support**: Thumbnail service now supports AVIF and HEIC image formats via govips/libvips; Dockerfile.toolbox updated with required vips packages for CGO compilation
- **Thumbnail Service Tests**: Comprehensive test coverage for `IsSupportedImageType`, `calculateThumbnailScale`, `GetPlaceholderThumbnail`, `DefaultThumbnailOptions`
- **Note Attachment Support**: Notes can now include file attachments; form uses `multipart/form-data` encoding and backend processes uploads after article creation
- **Enhanced Attachment Viewer**: Inline attachment viewer redesigned with:
  - Close button (primary color, dark mode compatible) with Esc key support
  - Collapsible metadata panel showing filename, type, size, upload date, attachment ID
  - Download button in header bar
  - Eye icon for view action (replaces ambiguous video icon)
  - Clicking attachment filename opens inline viewer by default (previously downloaded)
- **Version Display on Login**: Build version shown at bottom of agent login page; displays semantic version tag or branch name with short git commit hash in parentheses
- **Build Version Injection**: New `internal/version` package with ldflags injection; Makefile extracts git tag/branch/commit at build time and injects via `-X` flags; all build targets updated
- **SQL Portability Guard**: New `scripts/tools/check-sql.sh` script validates SQL queries for cross-database compatibility, blocking commits with PostgreSQL-specific `$N` placeholders or `ILIKE` operators
- **Helm Chart**: Production-ready Kubernetes deployment via `charts/gotrs/` with OCI registry publishing
  - Tag-mirroring: Chart `appVersion` matches git ref for GitOps workflows; `--version main` deploys `:main` images, `--version v0.5.0` deploys `:v0.5.0` images
  - Database selection: MySQL (default) or PostgreSQL via `database.type: mysql|postgresql` with custom StatefulSet templates
  - Valkey subchart: Official valkey-helm chart (BSD-3 licensed) as Redis-compatible cache dependency
  - extraResources: Arbitrary Kubernetes resources with full Helm templating support (`{{ .Release.Name }}`, `{{ .Values.* }}`, etc.)
  - Annotations and labels: Custom annotations/labels for cloud integrations (AWS IRSA, GKE Workload Identity, Prometheus scraping, Istio sidecar, AWS load balancers)
  - HPA support: Horizontal Pod Autoscaler for backend with configurable min/max replicas and CPU/memory targets
  - Ingress configuration: Flexible ingress with TLS, custom annotations, and multi-host support
  - Security contexts: All deployments include `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, `capabilities: drop: [ALL]`; database adds `runAsNonRoot`, `runAsUser: 999`, tmpfs for /tmp and /run/mysqld
  - CI integration: GitHub Actions publishes chart to `oci://ghcr.io/gotrs-io/charts/gotrs` on push to main or version tags
- **govulncheck Integration**: Go vulnerability scanning included in toolbox and security scans via `make scan-vulnerabilities`
- **Trivy Ignore File**: `.trivyignore` for configuring security scanner exclusions
- **Trivy Cache Persistence**: Trivy vulnerability database cached in `gotrs_cache` volume at `/cache/trivy`; eliminates re-download on every scan
- **Tool Cache Consolidation**: All standalone container tools now use `gotrs_cache` volume: golangci-lint (`/cache/golangci-lint`), Redocly/bun (`/cache/bun`), css-watch
- **Toolbox Entrypoint Script**: `scripts/toolbox-entrypoint.sh` for cache permission validation

### Changed
- **Attachment Click Behavior**: Clicking attachment filename now opens inline viewer instead of triggering download; download available via dedicated button
- **Attachment List Icons**: View button changed from video/play icon to eye icon for clearer UX
- **SQL Portability (MySQL/PostgreSQL)**: Comprehensive refactor of ~1,800 SQL queries across 127 files for cross-database compatibility
  - Converted all PostgreSQL-specific `$N` placeholders to portable `?` format with `database.ConvertPlaceholders()` wrapper
  - Replaced all `ILIKE` operators with `LOWER(column) LIKE LOWER(?)` for case-insensitive search portability
  - Updated repositories: ticket, article, user, queue, group, priority, state, permission, email_account, email_template, time_accounting
  - Updated API handlers: admin modules (users, groups, queues, priorities, states, types, roles, services, SLAs, customer companies/users), agent handlers, customer portal handlers
  - Updated components: dynamic field handlers, base CRUD handlers
  - All queries now use `database.GetAdapter().InsertWithReturning()` for portable INSERT operations with ID retrieval
- **Bun Package Manager Migration**: Replaced npm with bun for faster frontend builds and cleaner host filesystem
  - Dockerfile frontend stage uses `oven/bun:1.1-alpine` with Node.js for build tool compatibility
  - All `npm`/`npx` commands replaced with `bun`/`bunx` in Makefile and package.json scripts
  - Removed `package-lock.json`, now using `bun.lockb` binary lockfile
  - `make build` no longer runs frontend-build on host; Dockerfile handles CSS/JS build entirely
  - No `node_modules` directory created on host during builds
  - Bun global cache at `/cache/bun` in toolbox container
- **Go Version Single Source of Truth**: Go version centralized in `.env` as `GO_IMAGE` variable; all Dockerfiles, scripts, and Makefile targets inherit from this setting
- **Go Toolchain Upgrade**: Upgraded to Go 1.24.11 with toolchain directive in go.mod
- **Named Volume for Cache**: Changed `CACHE_USE_VOLUMES` default from `0` to `1`; development uses named Docker volume `gotrs_cache` instead of host bind mounts
- **Dockerfile.toolbox Simplified**: Removed complex entrypoint script, su-exec dependency, and USER root; container runs directly as `appuser` (UID 1000)
- **Dockerfile.playwright-go Security**: Creates and runs as non-root user `pwuser` with proper cache directory ownership
- **Production Reverse Proxy**: Replaced nginx with Caddy in `docker-compose.prod.yml`; Caddy provides automatic HTTPS via Let's Encrypt with embedded Caddyfile configuration
- **Dependency Updates**: Updated `golang.org/x/crypto`, `golang.org/x/net`, `golang.org/x/text`, `golang.org/x/sys`, and MCP SDK dependencies

### Security
- **Bun Installation Security**: Replaced insecure `curl|bash` Bun installation in Dockerfile.toolbox with GPG-verified tarball download; verifies signature against official Bun signing key before extraction
- **SDK Dependency Updates** (`sdk/go/go.mod`): Updated `github.com/go-resty/resty/v2` v2.10.0 → v2.16.5 (fixes HTTP request body disclosure), `golang.org/x/net` v0.17.0 → v0.34.0 (fixes XSS, IPv6 proxy bypass, header DoS vulnerabilities)
- **CVE-2023-36308 Mitigation**: Added panic recovery to `ThumbnailService.GenerateThumbnail` to gracefully handle crafted TIFF files that could cause server panic; no upstream patch available for `disintegration/imaging`

### Fixed
- **deleteAttachment JavaScript Error**: Added missing `deleteAttachment` function to ticket detail template; was called from HTMX-rendered attachment list but never defined
- **Note Content Field Not Found**: Fixed note form submission looking for wrong element ID (`note_content` instead of `body` used by rich text editor)
- **Note Form Null Errors**: Fixed `ensureErrorDiv` and `htmx.trigger` null reference errors by using correct element IDs and removing invalid element references
- **API Empty Response on Auth Failure**: Auth middleware now returns JSON 401 instead of HTML redirect when `Accept: application/json` header is present; `apiFetch()` helper automatically sets this header for all API calls
- **Direct fetch() API Calls**: Replaced 10 direct `fetch()` calls across 5 templates (profile, priorities, queues, dynamic_module, tickets) with `apiFetch()` to ensure proper Accept header and error handling
- **Thumbnail URL Generation**: Fixed broken thumbnail URLs in `ticket_messages_handler.go`; was generating `/api/attachments/:id/thumbnail` (non-existent route) instead of correct `/api/tickets/:id/attachments/:attachment_id/thumbnail`
- **History Recording Interface Mismatch**: Fixed `TicketRepository.AddTicketHistoryEntry` method signature to match `history.HistoryInserter` interface; changed `exec ExecContext` parameter to `exec interface{}` to enable proper type assertion in history recorder
- **SQL Argument Order Bugs**: Fixed argument order in `handleDeleteQueue` and `handleDeleteType` where `change_by` and `id` parameters were swapped
- **Missing SQL Arguments**: Fixed `insertArticle`, `insertArticleMimeData`, and `HandleRegisterWebhookAPI` missing `change_by` argument for MySQL NOT NULL columns
- **LOWER() Format String Typo**: Fixed `%LOWER(s)` typo in `base_crud.go` search query builder (should be `LOWER(%s)`)
- **Test Database Isolation**: Removed `defer db.Close()` calls from 7 test files that were closing the singleton database connection, causing "sql: database is closed" errors in subsequent tests
- **Makefile Toolbox Environment**: Added missing `TEST_DB_NAME`, `TEST_DB_USER`, `TEST_DB_PASSWORD` environment variables to 5 toolbox targets; fixed `TEST_DB_HOST`/`TEST_DB_PORT` to use `TOOLBOX_TEST_DB_HOST`/`TOOLBOX_TEST_DB_PORT` for host network mode
- **MariaDB Init Script**: Fixed `GRANT ALL PRIVILEGES ON otrs.* TO 'otrs'@'localhost'` error on fresh installs; `%` wildcard already covers localhost connections so removed redundant localhost grant
- **MariaDB Port Exposure**: Database port 3306 now exposed for host-based tools and MCP MySQL server access
- **Password Reset Modal**: Fixed JavaScript error when password reset API call fails
- **Gitignore Exception**: Added `!charts/gotrs/templates/secrets/` to prevent Helm secret templates from being ignored
- **Gitleaks Binary Allowlist**: Added `bun.lockb` to `.gitleaks.toml` allowlist; binary lockfile contains no secrets

### Removed
- **Legacy Kustomize Manifests**: Removed entire `k8s/` directory (22 files); Kubernetes deployments now use Helm chart at `charts/gotrs/`
- **Bare Metal Deployment**: Removed `docs/deployment/bare-metal.md` and all references; GOTRS supports containerized deployment only (Docker/Podman)
- **Nginx Configuration**: Removed `docker/nginx/` directory (Dockerfile, nginx.conf, error.html, entrypoint.sh); production deployments now use Caddy
- **DATABASE_URL Environment Variable**: Removed from compose files; use individual `DB_*` variables instead

### Documentation
- **Kubernetes Deployment Guide**: Rewritten for Helm chart usage with `helm install` commands, ArgoCD examples, and values customization
- **Helm Chart README**: Comprehensive documentation at `charts/gotrs/README.md` covering installation, configuration, database selection, annotations/labels, and extraResources
- **Docker Deployment Guide**: Completely rewritten with two deployment methods: Quick Deploy (curl files) and Development (full repo with make)
- **Podman Support**: Comprehensive Podman deployment instructions and notes
- **Migration Guide**: Major rewrite with accurate make targets (`migrate-analyze`, `migrate-import`, `migrate-import-force`, `migrate-validate`), migration paths table, article storage migration, and direct tool usage documentation
- **Demo Rate Limiting**: Updated from nginx to Caddyfile format
- **Schema Discovery**: Updated to reference `GO_IMAGE` environment variable

### Internal
- **Auth Middleware Tests**: Added 3 tests for `unauthorizedResponse` Accept header behavior verifying JSON vs HTML redirect based on Accept header
- **Note Attachment Test**: Added `TestTicketNoteWithAttachment` integration test with multipart form handling
- All Dockerfiles now accept `GO_IMAGE` build arg with consistent defaults
- Build targets (`make build`, `make build-cached`, etc.) pass `GO_IMAGE` and version build args to container builds
- Test and API scripts updated to use `GO_IMAGE` environment variable
- OpenAPI spec cleaned up (removed duplicate localhost:8000 server entry)
- Test suite now passes 876 tests with proper database isolation and MySQL compatibility
- SQL portability guard integrated into development workflow via check-sql.sh script

## [0.5.0] - 2026-01-03

### Added
- **CI/CD Pipeline Overhaul**: Complete rewrite of GitHub Actions workflows for containerized testing approach.
  - Security workflow: Go security scanning (gosec, govulncheck), Semgrep SAST, Hadolint for Dockerfiles, GitLeaks secret detection, license compliance checking, golangci-lint static analysis.
  - Build workflow: Single multi-stage Docker image build with GHCR publishing.
  - Test workflow: Containerized test execution via `make test`, coverage generation and upload to Codecov.
  - All workflows now use correct Dockerfile targets and container-first approach.
- **Codecov Integration**: Coverage reporting with OIDC authentication for private repositories.
- **Admin Templates Module**: Full CRUD functionality for standard response templates (OTRS AdminTemplate equivalent). Supports 8 template types (Answer, Create, Email, Forward, Note, PhoneCall, ProcessManagement, Snippet). Queue assignment UI for associating templates with specific queues. Attachment assignment UI for associating standard attachments with templates. Admin list page with search, filter by type/status, and sortable columns. Create/edit form with multi-select template type checkboxes, content type selector (HTML/Markdown). YAML import/export for template backup and migration. Agent integration with template selector in ticket reply/note modals with variable substitution (customer name, ticket number, queue, etc.). Template attachments auto-populate when template selected. 18 unit tests (type parsing, variable substitution, struct validation). Playwright E2E tests for admin UI. Self-registering handlers via init() pattern.
- **Admin Roles Module**: Full CRUD functionality for role management with database abstraction layer support. Includes role listing, create, update, soft delete, user-role assignments (add/remove users), and group permissions management. All queries use `database.ConvertPlaceholders()` for MySQL/PostgreSQL compatibility and `database.GetAdapter().InsertWithReturning()` for cross-database INSERT operations.
- **Self-Registering Handler Architecture**: Handlers now register via `init()` calls to `routing.RegisterHandler()`, eliminating manual registration in main.go. Test validates all YAML handlers are registered.
- **SLA Admin UX Improvements**: Time fields now use unit dropdowns (Minutes/Hours/Days) instead of raw minutes input, with automatic conversion.
- YAML handler wiring test (`internal/routing/yaml_handler_wiring_test.go`) that verifies all YAML-referenced handlers are registered.
- Handler registration init files (`internal/api/*_init.go`) for self-registering handlers.
- **Customer Portal**: Full customer-facing ticket management with login, ticket creation, viewing, replies, and ticket closure.
- **Customer Portal i18n**: Full internationalization for all 12 customer portal templates with English and German translations.
- Customer portal rich text editor (Tiptap) for ticket creation and replies.
- Customer close ticket functionality with proper article/article_data_mime insertion.
- Inbound email pipeline: POP3 connector factory, postmaster processor, ticket token filters, external ticket rules example, and mail account metadata/tests.
- IMAP connector support (go-imap/v2) with IMAPTLS alias, folder metadata propagation, and factory registration.
- Admin mail account poll status API/routes backed by Valkey cache.
- SMTP4Dev integration suite covering POP/SMTP roundtrips (attachments, threading, TLS/STARTTLS/SMTPS, concurrency) with minimal smtp4dev test client.
- SMTP4Dev IMAP integration flow to verify folder retention and account metadata on fetch without delete.
- POP3 fetcher resilience + mail queue task delivery/backoff cleanup coverage for SMTP sink flows.
- Notifications render context helper to populate agent/customer names for templates.
- Unit tests for filter chain, postmaster service, mail queue repository ordering, and email queue cleanup.
- Scheduler jobs CLI (`cmd/goats/scheduler_jobs`) with metrics publishing.
- Admin customer company create POST route at `/customer/companies/new`.
- Queue meta partial for ticket list/queue UI and updated templates.
- Dynamic module handler wiring with expanded acceptance coverage.
- **Email Threading Support**: RFC-compliant Message-ID, In-Reply-To, and References headers for conversation tracking in customer notifications.
- `BuildEmailMessageWithThreading()` function in mailqueue repository for generating threaded email messages.
- `GenerateMessageID()` function for creating unique RFC-compliant message identifiers.
- Database schema support for storing email threading headers in article records.
- Integration with agent ticket routes to include threading headers in customer notifications.
- Outbound customer notifications now send threaded emails on ticket creation and public replies, persisting Message-ID/In-Reply-To/References for future responses.
- Unit coverage for mailqueue threading helpers (Message-ID generation, threading headers, extraction) to guard regressions.
- Completed ticket creation vertical slice: `/api/tickets` service handler, HTMX agent form, attachment/time accounting support, and history recorder coverage.
- Ticket zoom (`pages/ticket_detail.pongo2`) now renders live articles, history, and customer context for newly created tickets.
- Status transitions, agent assignment, and queue transfer endpoints wired for both HTMX and JSON flows with history logging.
- Agent Ticket Zoom tabs now render ticket history and linked tickets via Pongo2 HTMX fragments, providing empty-state messaging until data exists.
- MySQL test container now applies the same integration fixtures as PostgreSQL, so API suites run identically across drivers.
- Regression coverage for `/admin/users` and YAML fallback routes when `GOTRS_DISABLE_TEST_AUTH_BYPASS` is disabled.
- **Admin Services Module**: Full CRUD functionality with 31 unit tests covering page rendering, create (form+JSON), update, delete, validation, DB integration, JSON responses, HTMX responses, and content-type handling.
- **Admin Customer User Services**: New management page at `/admin/customer-user-services` for assigning services to individual customer users, with dual-view UI (customer→services and service→customers).
- **Service Filtering in Customer Portal**: Customer ticket creation form now filters services to only show those assigned to the logged-in customer user via `service_customer_user` table.
- **Service Field in Agent Ticket Form**: Agents can now select a Service when creating tickets, with the service_id saved to the ticket record.
- **Default Services for Customer Users**: Customer users can now have default services assigned that are automatically pre-selected when creating tickets via the customer portal.
- **Dynamic Fields Admin Module**: Full CRUD for dynamic field definitions with 7 field types (Text, TextArea, Dropdown, Multiselect, Checkbox, Date, DateTime). Screen configuration UI for enabling fields on 8 ticket screens (AgentTicketZoom, AgentTicketCreate, etc.). OTRS-compatible YAML config storage. 52+ unit tests covering validation, DB operations, and API responses. Alpine.js client-side validation with i18n support (EN/DE).

### Changed
- Routes manifest regenerated (including admin dynamic aliases) and config defaults refreshed.
- Ticket creation validation tightened; queue UI updated with meta component.
- Dynamic module templates and handler registration aligned with tests.
- Scheduler email poller covers IMAPTLS alias predicate and factory registration.
- E2E/Playwright and schema discovery scripts refreshed.
- Agent ticket creation path issues `HX-Redirect` to the canonical zoom view and shares queue/state validation with the API handler.
- API test harness now defaults to Postgres to align history assertions with integration coverage.
- Documentation updated for inbound email IMAP aliases, folder metadata, and integration coverage notes.

### Fixed
- **CI Workflow Failures**: Rewrote security.yml and build.yml workflows that referenced non-existent files (Dockerfile.dev, Dockerfile.frontend, web/ directory). Project is a monolithic Go+HTMX app, not separate frontend/backend.
- **golangci-lint v1.64+ Compatibility**: Updated .golangci.yml to use `issues.exclude-dirs` instead of deprecated `run.skip-dirs`, removed other deprecated options.
- **Coverage Generation in CI**: Added git safe.directory configuration and GOFLAGS for VCS stamping to fix coverage generation in containerized CI environment.
- **Customer User Typeahead JSON Escaping**: Fixed JSON parsing issues in customer user autocomplete seed data where HTML entities (e.g., `&amp;`) were causing parse errors. Added `|escapejs` filter to properly escape strings for JSON context.
- **Admin Navigation Bar**: Fixed navigation showing customer portal links on admin pages (e.g., `/admin/customer/companies/*/edit`) when `PortalConfig` was passed for portal settings tab. Added `isAdmin` flag check in `base.pongo2` to prevent `isCustomer` detection on admin pages.
- SLA admin update handler now converts PostgreSQL placeholders to MySQL (`ConvertPlaceholders`).
- SLA admin create handler properly handles NOT NULL columns by converting nil to 0.
- Admin customer company create now returns validation (400) instead of 404 for POST to `/customer/companies/new`.
- Database connectivity issues in test environments with proper network configuration for test containers.
- Auth middleware, YAML fallback guards, and legacy route middleware now respect `GOTRS_DISABLE_TEST_AUTH_BYPASS`, preventing unauthenticated access to admin surfaces during regression runs.
- SQL placeholder conversion issues for MySQL compatibility in user and group repositories.
- User title field length validation to prevent varchar(50) constraint violations.
- Admin groups overview now renders the `comments` column so descriptions entered in OTRS appear in the group list UI.
- Admin groups membership links now launch the modal and load data through `/members`, restoring the key icon and member count actions.
- Queue-centric group permissions view with HTML + JSON endpoints for `/admin/groups/:id/permissions`.

### Changed
- Handler registration architecture: YAML routes now resolve handlers from `GlobalHandlerMap` populated via `init()` functions.
- SLA admin routes added to `routes/admin.yaml` for YAML-driven routing consistency.
- User repository Create and Update methods now include title length validation and proper SQL placeholder conversion.
- Group repository queries now use database.ConvertPlaceholders for cross-database compatibility.

### Removed
- _Nothing yet._

### Breaking Changes
- _None._

### Internal / Developer Notes
- Track follow-up work for status/assignment transitions and SMTP mail-sink container integration.

---

## [0.4.0] - 2025-10-20
### Added
- Generic GoatKit Typeahead enhancement (`goatkit-typeahead.js`): Enter/Tab auto-selects first suggestion, prevents accidental form submission, advances focus.
- GoatKit Autocomplete module (`goatkit-autocomplete.js`): declarative data-attribute driven autocomplete (seed JSON + future remote source), ARIA roles (combobox/listbox/option), keyboard navigation, first-item auto-highlight.
- Visual commit feedback (flash outline) on auto-complete commit.
- Global guards to prevent duplicate script initialization.
- Data seed loader with tolerant JSON parsing (trailing comma removal) and inline `<script type="application/json" data-gk-seed>` support.
- Hidden input synchronization via `data-hidden-target` for canonical value submission.
- Blur + click-outside handling to close suggestion lists.
- Configurable min character threshold (`data-min-chars`, default 1).
- Debug gating via `window.GK_DEBUG` flag (suppressed logs by default).
- Ticket zoom page base template.
- Per-queue ticket stats table (dashboard) and admin dashboard deduplication.
- Redis (Valkey-compatible) caching layer abstraction.
- Article storage backend (DB + filesystem) integration.
- Evidence diff utility for TDD enforcement.
- Unified ticket number generator framework + counter migration.
- Pluggable auth provider registry (database, ldap, static) with tests.
- Dockerfile/dev compose improvements for caching & user customization.
- Comprehensive ticket creation & validation test suite.
- Agent ticket creation auto-selects preferred queues pulled from customer and customer-user group permissions, with info panel surfacing the resolved queue name.
- Playwright acceptance harness (`test-acceptance-playwright`) with queue preference coverage, configurable artifact directories, and resilient base URL resolution.
- Consolidated schema alignment with OTRS: added `ticket_number_counter`, surrogate primary key for `acl_sync`, `acl_ticket_attribute_relations`, `activity`, `article_color`, `permission_groups`, `translation`, `calendar_appointment_plugin`, `pm_process_preferences`, `smime_keys`, `oauth2_token_config`/`oauth2_token`, and `mention` tables via migration `000001_schema_alignment`.

### Changed
- Refactored customer user inline autocomplete logic on ticket creation form to generic GoatKit modules (removal of large inline JS block in `templates/pages/tickets/new.pongo2`).
- Display template placeholder format switched to single-brace form `{firstName}` to avoid template engine collision; template compiler now supports both `{{key}}` and `{key}`.
- Auth handlers adapted to new provider registry API.
- Ticket creation now relies on repository ticket number generator (post framework introduction).
- Dockerfile optimized for builds (layer caching / user customization notes).
- Activity stream handling cleaned (duplicate handlers removed).
- Added surrogate primary key to `acl_sync` as part of consolidated migration `000001_schema_alignment` to stay aligned with OTRS upstream schema.
- Ticket list + queue detail defaults to `not_closed`, populating status dropdowns from live state tables and excluding closed types when requested.
- Login screen auto-focuses and selects the username field on load for quicker keyboard entry.
- Coverage targets (`make test-coverage*`) now run through the toolbox inside containers, spin up DB/cache services, and delegate execution to `scripts/run_coverage.sh` for filtered package selection.

### Fixed
- Trailing comma in generated seed JSON causing parse error (replaced incorrect loop variable usage and added tolerant parser).
- Auto-commit path previously populating hidden field with display string instead of login (added `data-login` / `data-value` attributes to suggestion options).
- MutationObserver early attachment errors (guarded until `document.body` present in both typeahead and autocomplete scripts).
- Empty dropdown lingering after selection (added blur close + explicit hide on commit).
- Initial absence of suggestions due to seed load ordering (added pre-load of all seed scripts before initialization).
- Ticket number StartFrom honored via proper counter initialization.
- Premature return in activity stream handler.
- Build handler duplication causing symbol redeclaration.
- Toolbox build/test hanging issues (interactive shell hang & GOFLAGS parsing) resolved.

### Removed
- Unnecessary `console.debug` noise (now gated behind `window.GK_DEBUG`).

### Breaking Changes
- Auth initialization now requires explicit provider registration (auth provider registry).
- New DB migration `000001_schema_alignment` required before further ticket creation.

### Internal / Developer Notes
- Autocomplete registry kept in-memory (`REGISTRY`) for potential future API exposure.
- Future enhancements (not yet implemented): remote data source (`data-source`), match substring highlighting, customizable "No results" template, hot reload of seeds.

---

## [0.3.0] - 2025-09-23
### Added
- Queue detail view with real-time statistics and enhanced ticket display (`feat(queue)`).
- Agent queues handler & template (agent queue list).
- Dark mode + custom Tailwind color palette, dark form element theming.
- Actions dropdown on ticket detail page.
- Rich text editor (Tiptap) integration for ticket/article content.
- Unicode support configuration & filtering.
- Markdown rendering switched to Goldmark with enhanced styling.
- Authentication middleware enhancements (logging, permission service improvements).
- Ticket creation page (HTMX form + error handling) and supporting templates.
- PATH and migration tooling updates for dual Postgres/MySQL dev support.

### Changed
- Refactored authentication middleware & API routes for consistency.
- Updated documentation and Makefile for toolbox workflow & container-first lessons.
- Standardized YAML routing & route loader tooling (static baseline + validation script).

### Fixed
- Permissions issues in admin modules (admin permissions functionality fix).
- SQL placeholder compatibility for MariaDB (PostgreSQL-style placeholders replaced).
- Various authentication, routing, ticket functionality issues (multi-fix commit 4a897cb).

### Internal
- Copilot instructions updated with container-first lessons.
- HTMX/JS refactors for API calls and utilities consolidation.

## [0.2.0] - 2025-09-03
### Added
- DB-less fallbacks for lookups, dashboard, tickets, admin pages to keep pages rendering under test / missing DB.
- Deterministic HTMX login path for tests; DB-less ticket creation in `APP_ENV=test`.
- Toolbox targets: staticcheck, curated integration test suites, test harness utilities.
- Storage path env expansion (`STORAGE_PATH`), host network mapping for toolbox, template directory overrides.
- CLI support: auto-create minimal users table & seed (DB-agnostic reset-user), user/admin helpers.
- API routing migration to YAML system completed.

### Changed
- Extensive test hardening & gating (skip when DB unavailable, deterministic outputs).
- Simplified toolbox execution (dropping UID mapping, caching modules/build, SELinux-friendly binds).
- Static analysis integration (staticcheck suppressions + fixes; normalized error strings & context keys).
- Build/runtime Docker & compose improvements (toolchain pinning Go 1.24.6, caching).

### Fixed
- Numerous nil DB panics across handlers/services (graceful fallbacks & guards).
- MariaDB-safe tests & placeholder corrections.
- Lookup handlers defensive defaults (queues/priorities/statuses) when DB absent.
- Test flakiness (shortened DB pings, guarded migrations, removal of unstable skips).
- Integration test compilation errors & unused symbol issues.

### Internal
- Separation of archived/ignored handlers via `//go:build ignore`.
- Normalization of Make targets (whitespace/tab fixes, GOFLAGS enforcement).
- Added curated test tags (integration, debug-only).

## [0.1.0] - 2025-08-17
### Added
- Foundational authentication (JWT, RBAC), session management, secret management system.
- OTRS-compatible database schema import (116 tables) and migration tooling.
- Ticket, article, internal notes, canned responses, SLA, search (Zinc), workflow automation, ticket templates, file storage service.
- LDAP / Active Directory integration & comprehensive LDAP testing infra (OpenLDAP).
- Internationalization (babelfish) and multi-language admin modules.
- Admin modules: roles, priorities, queues, states, types, services; dynamic lookup system.
- Customer portal, agent dashboard (SSE), queue management, ticket workflow state management.
- GraphQL API (initial) and REST API v1 Phase 2/3 progression.
- Comprehensive test suites (unit, integration, pact/contract tests) and TDD ticket creation with persistence.
- Security: automated secret scanning, removal of hardcoded credentials, secure test data generation.
- Multi-stage optimized Dockerfiles and build pipeline basics.

### Changed
- Pivot to HTMX frontend architecture (from prior approach) with Temporal & Zinc references.
- Consolidated documentation (architecture, roadmap progress reports, velocity/burndown charts).

### Fixed
- Numerous early stabilization fixes: authentication compile errors, database integration for tickets/queues/priorities, test panics, route duplication, credential corrections.
- Password generation switched to base64; placeholder/token format corrections.

### Security
- Removal of all hardcoded credentials; environment variable driven secrets; clean-room schema design for interoperability.

### Internal
- Early refactors improving security posture and documentation consolidation.

[Unreleased]: https://github.com/gotrs-io/gotrs-ce/compare/0.4.0...HEAD
[0.4.0]: https://github.com/gotrs-io/gotrs-ce/releases/tag/0.4.0
[0.3.0]: https://github.com/gotrs-io/gotrs-ce/releases/tag/0.3.0
[0.2.0]: https://github.com/gotrs-io/gotrs-ce/releases/tag/0.2.0
[0.1.0]: https://github.com/gotrs-io/gotrs-ce/releases/tag/0.1.0
