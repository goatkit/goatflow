# Platform vs Product Packages

This document lists the canonical division of packages between the GoatKit platform and the GoatFlow product. The linter (`cmd/gk-lint/`) enforces that platform packages (under `internal/platform/`) do not import product packages.

## Platform Packages (`internal/platform/`)

- api
- apierrors
- auth
- cache
- components
- config
- constants
- convert
- customfields
- data
- database
- deletion
- email
- httpcookie
- i18n
- ldap
- lookups
- marketplace
- mcp
- middleware
- models
- notifications
- oauth2
- organisation
- plugin
- pluginui
- push
- routing
- runner
- schema
- search
- secureconfig
- service
- services
- shared
- swconfig
- sysconfig
- template
- utils
- version
- webhook
- yamlmgmt
- zinc

## Product Packages (`internal/` excluding `platform/`)

- api
- components
- core
- email
- history
- mailaccountmeta
- mailqueue
- models
- repository
- runner
- selfservice
- service
- services
- storage
- testing
- testutil
- ticketnumber
- tickets
- ticketutil
- webhooks

Note: Some package names appear in both lists (e.g., `api`, `models`, `service`, `services`). This reflects the split where platform versions live under `internal/platform/` (e.g., `internal/platform/api/`) and product versions remain at the root (e.g., `internal/api/`). The linter uses the `internal/platform/` prefix to distinguish platform from product.