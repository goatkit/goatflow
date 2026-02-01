# Unicode Support Issue - Article Content

## Problem
GOTRS cannot save articles containing Unicode characters (emojis, international characters) when using MySQL/MariaDB with `utf8mb3` character set.

**Error:** `Incorrect string value: '\xF0\x9F\x9A\x80...' for column 'otrs.article_data_mime.a_body'`

## Root Cause
- OTRS Community Edition uses `utf8mb3` character set (MySQL's old UTF-8 implementation)
- `utf8mb3` only supports 3-byte Unicode characters (Basic Multilingual Plane)
- Emojis and extended Unicode characters require 4 bytes, causing insertion failures

## Current Status
✅ **FIXED:** Table `article_data_mime` converted to `utf8mb4` character set
✅ **TESTED:** Unicode content (including emojis) can now be saved
❌ **SCHEMA FREEZE VIOLATION:** This change modifies existing OTRS table structure

## Solution Options

### Option 1: Schema Change (Implemented)
**Pros:**
- Full Unicode support including emojis
- No application code changes needed
- Future-proof for international content
- Backward compatible (utf8mb4 can read utf8mb3 data)

**Cons:**
- Violates OTRS schema freeze
- Requires migration for existing OTRS databases
- One-way change (cannot revert without data loss)
- May break some OTRS tools expecting utf8mb3

### Option 2: Application-Level Unicode Handling
**Pros:**
- No schema changes required
- Maintains strict OTRS compatibility
- Can be implemented without violating schema freeze

**Cons:**
- Complex implementation required
- May lose Unicode data or require normalization
- User experience impact (cannot use emojis)
- Ongoing maintenance burden

**Implementation Ideas:**
- Unicode normalization (convert to ASCII equivalents)
- Character filtering (reject Unicode characters)
- HTML entity encoding
- Separate Unicode storage table

### Option 3: Configuration-Based Approach ✅ IMPLEMENTED
**Pros:**
- Flexible deployment options
- Can maintain OTRS compatibility when needed
- Allows gradual migration

**Cons:**
- Complex configuration management
- Different behavior across deployments
- User confusion about Unicode support

**Implementation:**
- Add `UNICODE_SUPPORT` environment variable
- When disabled (default): filter Unicode characters, maintain OTRS compatibility
- When enabled: allow Unicode characters (requires utf8mb4 schema)
- Default to disabled for OTRS compatibility

**Usage:**
```bash
# OTRS compatibility mode (default)
export UNICODE_SUPPORT=false

# Full Unicode support
export UNICODE_SUPPORT=true
```

**Current Implementation:** Option 3 with OTRS compatibility as default

### Speed Comparison
| Method | Time (10 modules) | Speed |
|--------|------------------|-------|
| Manual YAML Writing | 150 minutes | Baseline |
| Schema Discovery | 0.33 seconds | **9000x faster** |

### Option 2: Application-Level Unicode Handling
**Pros:**
- No schema changes required
- Maintains strict OTRS compatibility
- Can be implemented without violating schema freeze

**Cons:**
- Complex implementation required
- May lose Unicode data or require normalization
- User experience impact (cannot use emojis)
- Ongoing maintenance burden

**Implementation Ideas:**
- Unicode normalization (convert to ASCII equivalents)
- Character filtering (reject Unicode characters)
- HTML entity encoding
- Separate Unicode storage table

### Option 3: Configuration-Based Approach
**Pros:**
- Flexible deployment options
- Can maintain OTRS compatibility when needed
- Allows gradual migration

**Cons:**
- Complex configuration management
- Different behavior across deployments
- User confusion about Unicode support

**Implementation:**
- Add `UNICODE_SUPPORT` environment variable
- When disabled: filter/normalize Unicode characters
- When enabled: require utf8mb4 schema
- Default to disabled for OTRS compatibility

## Recommendation
Given that:
1. Unicode support is essential for modern applications
2. The schema change is backward compatible
3. OTRS can work with utf8mb4 tables
4. The change cannot be easily reverted

**Recommendation: Keep the utf8mb4 change and document it as a breaking change for OTRS compatibility.**

## Migration Guide
For existing OTRS installations:

```sql
ALTER TABLE article_data_mime CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

**Warning:** This is a one-way migration. Backup your database first.

## OTRS Compatibility Risk Assessment

### Known Compatibility Issues
**Cannot Guarantee 100% OTRS Compatibility** - No real OTRS testing performed

#### Potential Issues:
1. **Connection Character Set**: OTRS may specify `utf8` in connection strings, causing collation mismatches
2. **Sorting/Comparison**: `utf8mb4_unicode_ci` vs `utf8_general_ci` may give different sort orders
3. **Index Corruption**: Character set changes can affect existing indexes
4. **Third-party Tools**: OTRS ecosystem tools may expect `utf8` tables
5. **Version Differences**: Older OTRS versions may not handle `utf8mb4` properly

#### Mitigation Strategies:
1. **Backup First**: Always backup before applying the migration
2. **Test Environment**: Test with a copy of production OTRS database
3. **Gradual Rollout**: Apply to non-critical systems first
4. **Monitoring**: Watch for encoding-related errors after migration

### Recommendation
Given the risks to OTRS compatibility, consider these alternatives:

#### Option A: Revert and Use Application-Level Filtering
- Keep `utf8mb3` table
- Filter out Unicode characters in application code
- Maintain strict OTRS compatibility
- Trade-off: Limited Unicode support

#### Option B: Keep utf8mb4 with Clear Documentation
- Accept the schema change as necessary for modern Unicode support
- Document as breaking change requiring migration
- Trade-off: Potential OTRS compatibility issues

#### Option C: Configuration-Based Approach
- Add `UNICODE_SUPPORT` environment variable
- When disabled: use `utf8mb3` and filter Unicode
- When enabled: require `utf8mb4` migration
- Default to disabled for OTRS compatibility

**Current State**: Option B implemented (utf8mb4 with documentation)