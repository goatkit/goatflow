# Dynamic Module System

## Overview

The Dynamic Module System revolutionizes how admin modules are created and managed in GoatFlow. Instead of generating static code for each module, a single handler serves infinite modules based on YAML configurations. This achieves true DRY (Don't Repeat Yourself) principles.

## Key Features

### 🚀 Zero Compilation
- Drop a YAML file in the `modules/` directory
- Module is instantly available - no compilation, no restart
- Changes to YAML files are hot-reloaded automatically

### 🎯 One Handler, Infinite Modules
- Single `DynamicModuleHandler` serves all modules
- Generic CRUD operations adapt to any configuration
- Dramatic reduction in codebase size (90%+ less code)

### 🔥 Hot Reload
- File watcher monitors `modules/*.yaml`
- Changes apply instantly when you save the file
- Add, modify, or remove modules on the fly

### 🎨 Universal Template
- One template (`dynamic_module.pongo2`) adapts to any module
- Automatically generates forms based on field definitions
- Supports all common field types and features

## Architecture

```
/modules/                       # YAML configurations
  priority.yaml
  queue.yaml
  state.yaml
  custom.yaml                   # Your custom module!

/internal/components/dynamic/
  handler.go                    # The one handler to rule them all

/templates/pages/admin/
  dynamic_module.pongo2         # Universal template
```

## Creating a Module

### 1. Create YAML Configuration

Create a file in `modules/` directory (e.g., `modules/my_module.yaml`):

```yaml
module:
  name: my_module
  singular: Item
  plural: Items
  table: my_table
  description: Manage my items
  route_prefix: /admin/my_module

fields:
  - name: id
    type: int
    db_column: id
    label: ID
    show_in_list: true
    show_in_form: false
    sortable: true

  - name: name
    type: string
    db_column: name
    label: Name
    required: true
    searchable: true
    sortable: true
    show_in_list: true
    show_in_form: true
    help: "Enter a descriptive name"

  - name: status
    type: select
    db_column: status
    label: Status
    show_in_list: true
    show_in_form: true
    options:
      - value: active
        label: Active
      - value: inactive
        label: Inactive

  - name: color
    type: color
    db_column: color
    label: Color
    show_in_list: true
    show_in_form: true
    default: "#000000"

  - name: valid_id
    type: int
    db_column: valid_id
    label: Validity
    show_in_list: true
    show_in_form: false
    default: 1

features:
  soft_delete: true    # Use valid_id for soft deletes
  search: true         # Enable search functionality
  import_csv: true     # Show import button
  export_csv: true     # Show export button
  status_toggle: true  # Toggle valid/invalid status
  color_picker: true   # Enable color picker fields

permissions:
  - view
  - create
  - update
  - delete

validation:
  unique_fields:
    - name
  required_fields:
    - name
```

### 2. Access Your Module

Your module is instantly available at `/admin/my_module` - no compilation needed!

## Field Types

### Supported Types
- `string` - Text input
- `text` - Textarea
- `int`, `integer` - Number input
- `float`, `decimal` - Decimal number
- `bool`, `boolean` - Checkbox
- `date` - Date picker
- `datetime` - Date and time picker
- `color` - Color picker
- `select` - Dropdown menu

### Field Configuration
```yaml
- name: field_name          # Internal field name
  type: string              # Field type
  db_column: column_name    # Database column
  label: Display Label      # UI label
  required: true            # Validation
  searchable: true          # Include in search
  sortable: true            # Allow sorting
  show_in_list: true        # Show in table
  show_in_form: true        # Show in form
  default: "value"          # Default value
  help: "Help text"         # Help message
  validation: "^[A-Z]+$"    # Regex validation
  options:                  # For select fields
    - value: opt1
      label: Option 1
```

## API Endpoints

Each module automatically gets these endpoints:

- `GET /admin/{module}` - List all records
- `GET /admin/{module}/{id}` - Get single record
- `POST /admin/{module}` - Create new record
- `PUT /admin/{module}/{id}` - Update record
- `DELETE /admin/{module}/{id}` - Delete/deactivate record

All endpoints support both JSON (for API) and HTML (for UI) responses.

## Hot Reload in Action

1. Edit any YAML file in `modules/`
2. Save the file
3. Refresh your browser - changes are live!

Example:
```bash
# Terminal 1 - Watch logs
./scripts/container-wrapper.sh logs -f goatflow-backend

# Terminal 2 - Edit module
vim modules/priority.yaml
# Change description, add field, etc.
# Save file

# Browser - Refresh page
# Changes are instantly visible!
```

## Benefits Over Static Generation

### Traditional Approach (Static)
- Generate 3+ files per module (handler, template, test)
- Compile after each change
- 71 modules = 200+ files
- Code duplication everywhere
- Maintenance nightmare

### Dynamic Approach
- 1 handler for all modules
- 1 template for all modules
- 0 compilation needed
- 71 modules = 71 YAML files
- Zero code duplication
- Easy maintenance

## Comparison

| Aspect | Static Generation | Dynamic System |
|--------|------------------|----------------|
| Files per module | 3-5 | 1 (YAML only) |
| Compilation | Required | Never |
| Hot reload | No | Yes |
| Code duplication | 90%+ | 0% |
| Time to add module | 10-30 minutes | 2 minutes |
| Customer can add | No | Yes |
| Maintenance | High | Low |

## Advanced Features

### Automatic Datetime Formatting
All datetime fields are automatically formatted to the user's local timezone with relative time tooltips:

- **Automatic timezone conversion**: UTC dates are converted to browser's local timezone
- **User-friendly format**: Shows as "Aug 23, 2025, 4:30 PM" 
- **Relative time tooltips**: Hover to see "2 hours ago", "3 days ago", etc.
- **Smart handling**: Works with any field of type `datetime`
- **No configuration needed**: Just set `type: datetime` in your YAML

Example field configuration:
```yaml
- name: change_time
  type: datetime
  db_column: change_time
  label: Last Modified
  show_in_list: true
  sortable: true
```

### Schema Discovery (Coming Soon)
```bash
# Generate YAML from existing table
goatflow discover-schema --table=my_table > modules/my_table.yaml
```

### Computed Fields
Define fields that are calculated or derived from other data:

```yaml
computed_fields:
  - name: full_name
    label: Name
    show_in_list: true
    show_in_form: false
    source: "CONCAT first_name, last_name"
    
  - name: group_names
    label: Groups
    show_in_list: true
    show_in_form: false
    source: "JOIN user_groups"
```

The handler can implement custom logic for computed fields, such as:
- Concatenating multiple fields
- Joining data from related tables
- Calculating values based on other fields
- Formatting complex data structures

### Custom Validations
```yaml
validation:
  unique_fields:
    - name
    - email
  required_fields:
    - name
    - email
  custom_rules:
    - field: email
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
      message: "Invalid email format"
```

### Relationships (Planned)
```yaml
relationships:
  - field: user_id
    references: users.id
    display_field: users.name
    type: belongs_to
```

## Migration Path

1. **Keep existing modules** - They continue working
2. **Add dynamic system** - Runs alongside static modules
3. **Gradual migration** - Move modules to YAML one by one
4. **Remove static code** - Once all migrated

## Troubleshooting

### Module Not Loading
- Check YAML syntax: `yamllint modules/my_module.yaml`
- Verify table exists in database
- Check logs: `make logs`

### Fields Not Showing
- Ensure `show_in_list: true` for table columns
- Ensure `show_in_form: true` for form fields
- Check field names match database columns

### Hot Reload Not Working
- Verify file watcher is running (check logs)
- Ensure modules directory has correct permissions
- Try restarting the backend service

## Best Practices

1. **Start Simple** - Basic fields first, add features gradually
2. **Match Database** - Ensure db_column matches actual columns
3. **Use Soft Delete** - Set `soft_delete: true` for OTRS compatibility
4. **Test First** - Verify table exists before creating module
5. **Document Fields** - Use `help` text for complex fields

## Example Modules

See the `modules/` directory for examples:
- `priority.yaml` - Ticket priorities with color picker
- `queue.yaml` - Queue management with relationships
- `state.yaml` - Ticket states with type selection
- `service.yaml` - Service catalog

## The Future: GoatKit Plugin Platform

Dynamic Modules are the foundation for the GoatKit Plugin Platform (v0.7.0+). The plugin system extends this YAML + templates approach with:

- **WASM & gRPC Runtimes** - Custom logic beyond YAML capabilities
- **Plugin Packaging** - ZIP distribution with templates, assets, translations
- **Self-Registration** - Plugins declare their functions, routes, and permissions
- **Host Function API** - Database, HTTP, email, cache access from plugins
- **Marketplace** - Browse, install, and update third-party plugins

The graduated complexity model:

| Need | Solution |
|------|----------|
| Simple CRUD | Dynamic Modules (YAML) |
| Computed fields | Lambda Functions (JavaScript) |
| Custom logic | Plugin Platform (WASM/gRPC) |

See [Plugin Platform](PLUGIN_PLATFORM.md) for the full roadmap.

## Conclusion

The Dynamic Module System transforms GoatFlow from a traditional monolithic application into a flexible, extensible platform. By eliminating code generation and compilation, we've achieved:

- **90% less code** to maintain
- **Instant module creation** without coding
- **Customer-friendly** customization
- **True DRY principles** in action

This is not just an improvement - it's a paradigm shift in how admin interfaces should be built.
