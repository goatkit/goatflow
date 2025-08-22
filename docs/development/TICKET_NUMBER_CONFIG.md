# Ticket Number Configuration Decision

## Background
OTRS supports multiple ticket number generators through configuration in `Kernel/Config.pm`. This allows organizations to customize ticket numbering to match their business requirements.

## OTRS Ticket Number Generators

OTRS provides several built-in generators:
1. **AutoIncrement** - Simple sequential numbers (1, 2, 3...)
2. **Date** - Date-based format (2025082100001)
3. **DateChecksum** - Date with checksum (20250821-000001-31)
4. **Random** - Random alphanumeric strings

## Current GOTRS Implementation
Currently hardcoded to date-based format: `YYYYMMDD-NNNNNN` (e.g., 20250821-000007)

## Proposed Configuration Structure

### Using Viper/YAML (as per project decision)
```yaml
# config/gotrs.yaml
ticket:
  number_generator:
    type: "date"  # Options: auto_increment, date, date_checksum, random, custom
    
    # Date-based options
    date:
      format: "20060102"  # Go time format
      separator: "-"
      counter_digits: 6
      reset_daily: true
    
    # Auto-increment options  
    auto_increment:
      prefix: "T-"
      min_digits: 7
      
    # Random options
    random:
      length: 10
      charset: "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
      
    # Custom pattern (future)
    custom:
      pattern: "${YYYY}${MM}${DD}-${COUNTER:6}"
```

## Implementation Plan

### Phase 1: Configuration Structure (When Viper is integrated)
1. Define configuration schema in YAML
2. Create `TicketNumberGenerator` interface
3. Implement generator types matching OTRS options

### Phase 2: Generator Implementations
```go
// internal/service/ticket_number.go
type TicketNumberGenerator interface {
    Generate() (string, error)
    Reset() error  // For daily/monthly resets
}

type DateBasedGenerator struct {
    format    string
    separator string
    counter   *atomic.Int64
}

type AutoIncrementGenerator struct {
    prefix    string
    minDigits int
    sequence  *sql.DB  // Uses database sequence
}
```

### Phase 3: Database Support
```sql
-- migrations/XXX_add_ticket_counters.sql
CREATE TABLE IF NOT EXISTS ticket_counter (
    id INTEGER PRIMARY KEY,
    counter_type VARCHAR(50) NOT NULL,
    current_value BIGINT NOT NULL DEFAULT 0,
    last_reset TIMESTAMP,
    UNIQUE(counter_type)
);

-- For auto-increment style
CREATE SEQUENCE IF NOT EXISTS ticket_number_seq;
```

## Migration from Current Implementation

1. Keep existing ticket numbers unchanged
2. New configuration only affects new tickets
3. Support reading both old and new formats

## Benefits

1. **Flexibility** - Organizations can choose numbering that fits their needs
2. **OTRS Compatibility** - Matches OTRS configuration patterns
3. **Extensibility** - Easy to add new generator types
4. **Performance** - Cached counters, atomic operations

## Examples of Different Formats

- **Auto-increment**: T-0000001, T-0000002
- **Date-based**: 20250821-000001, 20250821-000002
- **Date with checksum**: 20250821-000001-42 (checksum prevents tampering)
- **Random**: TICKET-A3X9K2M8P1
- **Custom**: 2025-AUG-21-IT-0001 (year-month-day-department-counter)

## Security Considerations

- Checksum generators prevent URL manipulation
- Random generators prevent ticket enumeration
- Sequential numbers may expose volume information

## Note
This will be implemented after Viper configuration is integrated, as decided in `CONFIG_DECISION.md`.