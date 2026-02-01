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

#### Documented Exception: UTF8MB4 Character Set Migration
**Date:** 2025-09-26  
**Table:** `article_data_mime`  
**Change:** Character set converted from `utf8mb3` to `utf8mb4`  
**Status:** IMPLEMENTED (cannot be reverted due to existing data)

**Justification:**
- OTRS Community Edition uses `utf8mb3` (limited Unicode support)
- Modern applications require full Unicode support (emojis, international characters)
- Without this change, articles containing Unicode characters fail to save
- This affects user experience and limits GOTRS adoption

**Impact Assessment:**
- ✅ **Forward Compatible:** utf8mb4 can read utf8mb3 data
- ✅ **OTRS Compatible:** OTRS can work with utf8mb4 tables
- ⚠️ **Migration Required:** Existing OTRS databases need this change to work with GOTRS
- ⚠️ **Cannot Revert:** Unicode data now exists that prevents rollback

**Migration Path:**
1. Run `ALTER TABLE article_data_mime CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`
2. This is a one-way migration - cannot be undone without data loss
3. Document in release notes as breaking change for OTRS compatibility

**Current Implementation: Hybrid Unicode Support**
**Date:** 2025-09-26  
**Status:** IMPLEMENTED

To balance modern Unicode requirements with OTRS compatibility, GOTRS implements a hybrid approach:

**Configuration-Based Unicode Support:**
- `UNICODE_SUPPORT=true`: Full Unicode support (utf8mb4 + all characters)
- `UNICODE_SUPPORT=false` (default): OTRS-compatible mode (utf8mb4 + filtered characters)

**Application-Level Unicode Filtering:**
When `UNICODE_SUPPORT=false`, GOTRS automatically filters out:
- Emojis and symbols (U+10000 and above)
- Extended Unicode characters incompatible with OTRS
- Mathematical symbols and rare Unicode blocks
- Preserves common accented characters (Latin-1 Supplement, Latin Extended-A)

**Benefits of This Approach:**
- ✅ **Schema Flexibility:** utf8mb4 supports all Unicode when needed
- ✅ **OTRS Compatibility:** Filtering ensures compatibility with OTRS tools/workflows
- ✅ **User Choice:** Organizations can choose Unicode level based on needs
- ✅ **Zero Migration Cost:** Existing OTRS databases work without changes
- ✅ **Future-Proof:** Can enable full Unicode support without schema changes

**Implementation Details:**
- Filter logic in `internal/utils/unicode_filter.go`
- Applied to article content before database storage
- Environment variable controlled: `UNICODE_SUPPORT`
- Default: OTRS-compatible mode for seamless migration

---

**Remember:** Users choose GOTRS because it's a drop-in OTRS replacement. Breaking compatibility breaks trust.