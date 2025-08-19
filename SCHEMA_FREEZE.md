# SCHEMA FREEZE NOTICE

## Database Schema is Frozen for OTRS Compatibility

**Effective Date:** 2025-08-19  
**Status:** FROZEN - DO NOT MODIFY

### Critical Requirement
This database schema is 100% compatible with OTRS Community Edition and MUST remain so.

### Guard Rails - Why Schema Cannot Change

1. **OTRS Compatibility is Non-Negotiable**
   - We maintain exact table names (singular: `ticket`, `article`, `queue`)
   - Column types must match exactly (e.g., `customer_id VARCHAR(150)`)
   - All OTRS tables must exist with identical structure

2. **Migration Path**
   - Organizations must be able to migrate from OTRS to GOTRS
   - Tools built for OTRS database must work with GOTRS
   - SQL queries from OTRS must run unchanged

3. **Legal Compliance**
   - Schema was written from scratch to avoid licensing issues
   - We cannot copy OTRS DDLs directly
   - Must maintain clean-room implementation

4. **Ecosystem Compatibility**
   - Third-party OTRS tools expect this schema
   - Reporting tools rely on these table structures
   - Backup/restore tools assume OTRS schema

### What This Means

❌ **DO NOT:**
- Add new columns to existing OTRS tables
- Change data types of existing columns
- Rename tables or columns
- Remove required OTRS tables
- Alter indexes that OTRS depends on
- **Add ANY new tables until production release**

✅ **YOU CAN:**
- Add indexes for performance (if they don't conflict)
- Add views for convenience (read-only)
- Optimize queries

**CRITICAL:** NO new tables until first production-ready release. We must achieve feature parity with OTRS using only their schema.

### Required Tables (DO NOT ALTER)
- `ticket` (not tickets)
- `article` (not articles)
- `queue` (not queues)
- `customer_user`
- `customer_company`
- `ticket_history`
- `ticket_state`
- `ticket_priority`
- `article_data_mime`
- `users` (OTRS format)
- `groups` (OTRS format)

### Extension Strategy
**NOT ALLOWED UNTIL PRODUCTION RELEASE**
After production release, if you need additional functionality:
1. Create new tables prefixed with `gotrs_`
2. Use foreign keys to reference OTRS tables
3. Document in `/docs/extensions/`
4. Never modify core OTRS tables

For now: Work within OTRS schema constraints only.

### Verification Command
```bash
# This should always pass - checks OTRS compatibility
./scripts/verify-otrs-schema.sh
```

### Exceptions
Changes to the frozen schema require:
1. Written justification of why OTRS compatibility must be broken
2. Migration plan for existing OTRS users
3. Sign-off from project lead
4. Major version bump (e.g., 2.0.0)

---

**Remember:** Users choose GOTRS because it's a drop-in OTRS replacement. Breaking compatibility breaks trust.