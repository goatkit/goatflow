# Ticket Attribute Relations Test Data

Sample CSV files for testing the Ticket Attribute Relations feature.

## File Format

CSV files use semicolon (`;`) as the delimiter and must have exactly 2 columns:
- First column: Attribute 1 (e.g., Queue, State, Priority, Type, Service, SLA, Owner, Responsible, or DynamicField_*)
- Second column: Attribute 2 (same options as Attribute 1)

Use `-` to represent an empty/any value.

## Sample Files

### queue_category.csv
Maps Queue to DynamicField_Category. When an agent selects a queue, only the matching category options are shown.

### state_priority.csv
Maps State to Priority. Restricts which priorities are available based on ticket state.

### queue_service.csv
Maps Queue to Service. Different queues offer different services.

## Usage

1. Go to Admin > Ticket Attribute Relations
2. Click "Import CSV/Excel"
3. Upload one of these files
4. The relation will be active immediately

When creating/editing tickets, selecting a value for Attribute 1 will filter the options available for Attribute 2.
