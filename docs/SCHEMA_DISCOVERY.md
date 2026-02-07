# Schema Discovery Tool

The Schema Discovery Tool automatically generates YAML module configurations from your PostgreSQL database schema. This enables rapid creation of admin modules without manual configuration.

## Features

- **Automatic Field Detection**: Analyzes database columns and generates appropriate field types
- **Smart Type Inference**: Detects email, password, URL, phone fields from column names
- **Relationship Discovery**: Identifies foreign keys and constraints
- **Container-First**: Runs entirely in containers (Docker/Podman)
- **Batch or Single**: Generate all tables or specific ones

## Usage

### Generate All Modules

```bash
make schema-discovery
```

This will:
1. Build the schema discovery tool in a Go container
2. Connect to your PostgreSQL database
3. Analyze all tables in the public schema
4. Generate YAML files in `modules/generated/`

### Generate Specific Table

```bash
make schema-table TABLE=ticket_priority
```

### Command Line Options

```bash
./scripts/schema-discovery.sh [options]
  --host HOST        Database host (default: goatflow-postgres)
  --port PORT        Database port (default: 5432)
  --user USER        Database user (default: goatflow_user)
  --password PASS    Database password
  --database DB      Database name (default: goatflow)
  --output DIR       Output directory (default: modules/generated)
  --table TABLE      Specific table to generate (empty for all)
  --verbose          Verbose output
  --help             Show help message
```

## Generated Module Structure

Each generated YAML file contains:

```yaml
module:
  name: table_name
  singular: Table Name
  plural: Table Names
  table: table_name
  description: Manage Table Names
  route_prefix: /admin/dynamic/table_name

fields:
  - name: id
    type: integer
    db_column: id
    label: Id
    required: false
    searchable: false
    sortable: true
    show_in_list: true
    show_in_form: false
    # ... more field properties

features:
  soft_delete: true    # If valid_id column exists
  search: true
  export_csv: true
  
validation:
  unique_fields: []    # Detected from UNIQUE constraints
```

## Field Type Detection

The tool intelligently detects field types based on:

### Column Name Patterns
- `email*` → email field
- `password`, `pw` → password field
- `url*`, `website*` → URL field
- `phone*`, `tel*` → phone field
- `color*` → color picker
- `notes`, `description`, `comment*` → textarea

### Data Type Mapping
- `integer`, `bigint`, `smallint` → integer
- `numeric`, `decimal` → decimal
- `boolean` → checkbox
- `date` → date picker
- `timestamp` → datetime picker
- `time` → time picker
- `text` → textarea
- `varchar`, `char` → string

## Display Rules

The tool applies smart defaults for field visibility:

### Show in List
- ✅ Most fields except:
  - ❌ Password fields
  - ❌ Long text fields
  - ❌ System fields (`*_by` except `create_by`)

### Show in Form
- ✅ User-editable fields
- ❌ Auto-generated fields (id, timestamps)
- ❌ System tracking fields (`*_by`, `*_time`)

### Searchable
- ✅ Text fields (varchar, char, text)
- ❌ Numeric and date fields

### Sortable
- ✅ All fields except very long text

## Integration with Dynamic Modules

Generated YAML files work seamlessly with the dynamic module system:

1. **Generate**: `make schema-discovery`
2. **Review**: Check generated files in `modules/generated/`
3. **Customize**: Edit YAML to add:
   - Lambda functions
   - Custom validation
   - Display formatting
   - Relationships
4. **Deploy**: Modules are automatically loaded by the dynamic handler

## Example Workflow

```bash
# 1. Generate module for customer_company table
make schema-table TABLE=customer_company

# 2. Review generated file
cat modules/generated/customer_company.yaml

# 3. Edit to customize (optional)
vim modules/generated/customer_company.yaml

# 4. Access module at
http://localhost:8080/admin/dynamic/customer_company
```

## Customization

After generation, you can enhance modules by adding:

### Lambda Functions
```yaml
lambda_functions:
  - name: validateCompanyName
    trigger: before_save
    code: |
      if (!data.name || data.name.length < 3) {
        throw new Error('Company name must be at least 3 characters');
      }
```

### Field Relationships
```yaml
fields:
  - name: customer_id
    type: select
    lookup_table: customer_user
    lookup_key: id
    lookup_display: login
```

### Display Formatting
```yaml
fields:
  - name: status
    display_as: badge
    display_map:
      active: success
      inactive: danger
```

## Limitations

- Only analyzes `public` schema
- Skips system tables (pg_*, sql_*, information_schema)
- Basic relationship detection (improve manually)
- No support for complex types or arrays
- Requires manual review for production use

## Troubleshooting

### Network Not Found
If you see "network goatflow_default not found":
```bash
# Check actual network name
docker network ls | grep goatflow

# Update script with correct network
vim scripts/schema-discovery.sh
# Change: --network goatflow_default
# To: --network goatflow_goatflow-network
```

### Permission Denied
Ensure generated files have correct permissions:
```bash
chmod 644 modules/generated/*.yaml
```

### Build Failures
If Go version errors occur, update the single source of truth:
```bash
# Edit .env and change GO_IMAGE
vim .env
# Example: GO_IMAGE=golang:1.25.0-alpine
```

All Dockerfiles and scripts inherit from this setting.

## Best Practices

1. **Always Review**: Generated modules are a starting point, not final
2. **Test First**: Try modules in development before production
3. **Version Control**: Commit customized YAML files
4. **Incremental**: Generate and test one module at a time
5. **Document Changes**: Note customizations in comments

## Architecture

The schema discovery tool consists of:

1. **CLI Tool** (`cmd/schema-discovery/main.go`)
   - Command-line interface
   - Database connection
   - File generation

2. **Discovery Engine** (`internal/components/dynamic/schema_discovery.go`)
   - Table introspection
   - Column analysis
   - Constraint detection
   - Type inference

3. **Container Script** (`scripts/schema-discovery.sh`)
   - Container runtime detection
   - Build orchestration
   - Network configuration

## Future Enhancements

- [ ] Detect more relationship types
- [ ] Generate validation rules from constraints
- [ ] Support for views and materialized views
- [ ] Custom type mappings configuration
- [ ] Incremental updates (merge with existing)
- [ ] Generate TypeScript interfaces
- [ ] Support multiple schemas
- [ ] AI-powered field naming improvements