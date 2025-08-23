# Schema Discovery Guide

## Overview

The Schema Discovery feature enables automatic generation of dynamic CRUD modules from existing database tables. This eliminates manual YAML configuration and ensures consistency across all admin modules.

## Features

- **Automatic Table Discovery**: Lists all tables in the database
- **Column Introspection**: Analyzes column types, constraints, and relationships
- **Smart Field Type Inference**: Automatically determines appropriate input types
- **Audit Field Handling**: Automatically populates create_by, change_by fields
- **Instant Module Generation**: Creates working CRUD interfaces in seconds

## Quick Start

### 1. Access Schema Discovery UI

Navigate to: `http://localhost:8080/admin/schema-discovery`

Or use the Admin Dashboard link: **Admin → Schema Discovery**

### 2. Using the Web Interface

1. **Browse Tables**: View all database tables with their current module status
2. **Inspect Columns**: Click "View Columns" to see table structure
3. **Generate Module**: Click "Generate" to preview the YAML configuration
4. **Save Module**: Click "Save Module" to create the module file

### 3. Using the API

```bash
# List all tables
curl -H "Cookie: access_token=your_token" \
     -H "X-Requested-With: XMLHttpRequest" \
     "http://localhost:8080/admin/dynamic/_schema?action=tables"

# Get columns for a table
curl -H "Cookie: access_token=your_token" \
     -H "X-Requested-With: XMLHttpRequest" \
     "http://localhost:8080/admin/dynamic/_schema?action=columns&table=your_table"

# Generate module configuration
curl -H "Cookie: access_token=your_token" \
     -H "Accept: text/yaml" \
     "http://localhost:8080/admin/dynamic/_schema?action=generate&table=your_table&format=yaml"

# Save module to filesystem
curl -H "Cookie: access_token=your_token" \
     -H "X-Requested-With: XMLHttpRequest" \
     "http://localhost:8080/admin/dynamic/_schema?action=save&table=your_table"
```

## Field Type Inference

The system automatically infers appropriate field types based on:

### Column Name Patterns

| Pattern | Inferred Type | Example |
|---------|--------------|---------|
| `password`, `pw` | password | Hidden input with toggle |
| `email` | email | Email input with validation |
| `url`, `website` | url | URL input |
| `phone`, `tel` | phone | Phone number input |
| `color`, `colour` | color | Color picker |
| `notes`, `description`, `comment` | textarea | Multi-line text |

### Data Type Mapping

| Database Type | Field Type | UI Component |
|--------------|------------|--------------|
| `integer`, `bigint`, `smallint` | integer | Number input |
| `numeric`, `decimal` | decimal | Decimal input |
| `boolean` | checkbox | Checkbox |
| `date` | date | Date picker |
| `timestamp` | datetime | DateTime picker |
| `time` | time | Time picker |
| `text` | textarea | Multi-line text |
| `varchar`, `char` | string | Text input |

## Audit Fields

The system automatically handles OTRS audit fields:

- **create_by**: Set to current user ID on INSERT
- **change_by**: Updated to current user ID on UPDATE
- **create_time**: Set to CURRENT_TIMESTAMP on INSERT
- **change_time**: Updated to CURRENT_TIMESTAMP on UPDATE
- **valid_id**: Managed for soft deletes (1=active, 2=inactive)

These fields are automatically hidden from forms but shown in lists.

## Best Practices

### 1. Table Requirements

For best results, tables should follow OTRS conventions:
- Primary key named `id`
- Audit fields: `create_by`, `create_time`, `change_by`, `change_time`
- Soft delete field: `valid_id` (1=active, 2=inactive)
- Descriptive column names for better label generation

### 2. When to Use Schema Discovery

**Ideal for:**
- Admin interfaces for lookup tables
- Simple CRUD operations
- Rapid prototyping
- Tables with standard structure

**Consider manual configuration for:**
- Complex business logic
- Custom validation rules
- Special UI requirements
- Tables with many relationships

### 3. Customizing Generated Modules

After generation, you can edit the YAML file to:
- Add custom validation patterns
- Define select field options
- Add help text
- Adjust field visibility
- Add custom features

### 4. Performance Considerations

- Generated modules use the same efficient query patterns
- Indexes on frequently searched columns improve performance
- Soft deletes (valid_id) should be indexed
- Consider pagination for large tables

## Examples

### Simple Lookup Table

```yaml
# Generated for 'salutation' table
module:
  name: salutation
  singular: Salutation
  plural: Salutations
  table: salutation
fields:
  - name: name
    type: string
    label: Name
    required: true
    show_in_list: true
    show_in_form: true
```

### Table with Relationships

```yaml
# Generated for 'queue_auto_response' table
module:
  name: queue_auto_response
  singular: Queue Auto Response
  plural: Queue Auto Responses
fields:
  - name: queue_id
    type: integer
    label: Queue Id
    required: true
    # TODO: Convert to select with queue options
  - name: auto_response_id
    type: integer
    label: Auto Response Id
    required: true
    # TODO: Convert to select with auto_response options
```

## Troubleshooting

### Module Not Loading

1. Check file permissions on `modules/` directory
2. Wait 2-3 seconds for file watcher to detect changes
3. Check backend logs for errors: `./scripts/container-wrapper.sh logs gotrs-backend`

### CRUD Operations Failing

1. Verify table has proper audit fields
2. Check user session is active
3. Review field requirements and constraints
4. Check database logs for constraint violations

### Field Types Incorrect

1. Review the generated YAML file
2. Manually adjust field types as needed
3. Consider column naming conventions

## Advanced Usage

### Batch Generation

Generate modules for multiple tables:

```bash
#!/bin/bash
TABLES="salutation signature standard_template"
for table in $TABLES; do
    curl -s -H "Cookie: access_token=demo_session_admin" \
         "http://localhost:8080/admin/dynamic/_schema?action=save&table=$table"
    echo "Generated module for $table"
    sleep 1
done
```

### Custom Field Types

After generation, enhance fields with custom types:

```yaml
fields:
  - name: status
    type: select  # Changed from string
    options:
      - value: "active"
        label: "Active"
      - value: "inactive"
        label: "Inactive"
```

### Adding Relationships

Convert foreign key fields to dropdowns:

```yaml
fields:
  - name: queue_id
    type: select
    label: Queue
    datasource: "/api/queues"  # Fetch options dynamically
    display_field: "name"
    value_field: "id"
```

## Security Considerations

- Schema discovery requires admin authentication
- Generated modules inherit all security from dynamic handler
- Audit fields track all changes
- Soft deletes preserve data integrity
- User permissions apply to all CRUD operations

## Limitations

- Foreign key relationships need manual configuration for dropdowns
- Complex validation rules require manual YAML editing
- Computed fields not automatically detected
- Stored procedures and views have limited support

## Future Enhancements

Planned improvements:
- Automatic foreign key relationship detection
- Dropdown generation for reference tables
- Validation rule inference from constraints
- Support for composite primary keys
- Migration generation for schema changes