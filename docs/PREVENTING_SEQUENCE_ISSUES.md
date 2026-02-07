# Preventing PostgreSQL Sequence Issues

> **Note**: This document applies only to PostgreSQL deployments. For MySQL compatibility patterns, see [development/DATABASE_ACCESS_PATTERNS.md](development/DATABASE_ACCESS_PATTERNS.md).

## Problem
After importing data (especially from OTRS dumps), PostgreSQL sequences can become out of sync with the actual table data. This causes "duplicate key value violates unique constraint" errors when trying to insert new records.

## Root Cause
- When data is imported with explicit IDs, PostgreSQL doesn't automatically update the sequences
- The sequence counter stays at its old value while the table has higher IDs
- Next INSERT tries to use an ID that already exists → duplicate key error

## Prevention Strategies

### 1. Always Fix Sequences After Import
```bash
# After any data import or migration:
make db-fix-sequences
```

### 2. Add to Import Workflow
```bash
# Complete import workflow:
make migrate-import        # Import the data
make db-fix-sequences      # Fix sequences immediately
make migrate-validate      # Verify data integrity
```

### 3. Automated Fix Command
`make db-fix-sequences` runs a Postgres DO block that recalculates every sequence based on the highest ID in its table. The command is idempotent and safe to run repeatedly after imports or bulk updates.

### 4. Manual Fix for Specific Table
```sql
-- If you know a specific table has issues:
SELECT setval('article_id_seq', (SELECT MAX(id) FROM article));
SELECT setval('ticket_id_seq', (SELECT MAX(id) FROM ticket));
```

### 5. Check Sequence Status
```bash
# Check if sequences are in sync:
docker exec goatflow-postgres psql -U goatflow_user -d goatflow -c "
    SELECT 
        'article' as table,
        MAX(id) as max_id,
        (SELECT last_value FROM article_id_seq) as sequence_value
    FROM article;"
```

## When to Run Sequence Fixes

Run `make db-fix-sequences` after:
- ✅ OTRS data import
- ✅ Database restore from backup
- ✅ Manual data insertion with explicit IDs
- ✅ Any bulk data import operations
- ✅ If you see "duplicate key" errors

## Integration with CI/CD

Add to your deployment pipeline:
```yaml
deploy:
  steps:
    - name: Import data
      run: make migrate-import
    - name: Fix sequences
      run: make db-fix-sequences
    - name: Validate
      run: make migrate-validate
```

## Monitoring

Watch for these error patterns in logs:
- `duplicate key value violates unique constraint`
- `pkey` constraint violations
- `_id_seq` related errors

If you see these, immediately run:
```bash
make db-fix-sequences
```

## Best Practices

1. **Never skip sequence fixes** after data imports
2. **Document in runbooks** that sequence fixes are required
3. **Add to health checks** - compare max IDs with sequence values
4. **Train team members** about this PostgreSQL behavior
5. **Consider using UUIDs** for new tables to avoid this entirely

## Why This Happens in PostgreSQL

PostgreSQL sequences are independent objects that don't automatically sync with table data. When you:
1. INSERT with explicit ID: `INSERT INTO article (id, ...) VALUES (100, ...)`
2. The sequence doesn't know about this ID
3. Next INSERT without ID uses sequence: `nextval('article_id_seq')` returns 1
4. Conflict with existing ID 100 → error

This is different from MySQL's AUTO_INCREMENT which automatically adjusts.

## Testing After Fix

After running sequence fixes, test that new inserts work:
```bash
# Try creating a new article/ticket through the UI
# Should work without duplicate key errors
```

Or via SQL:
```sql
-- This should work after fix:
INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, ...)
VALUES (1, 1, 1, ...) RETURNING id;
```