# MySQL Compatibility Issues

## Critical Issues Found

### 1. RETURNING Clause (64 instances)
**Problem**: PostgreSQL's `RETURNING id` clause is not supported in MySQL.
**Example**: 
```sql
INSERT INTO article (...) VALUES (...) RETURNING id
```
**Solution**: Use `LastInsertId()` after insert for MySQL.

### 2. ILIKE Operator (19 instances)
**Problem**: PostgreSQL's case-insensitive LIKE operator `ILIKE` doesn't exist in MySQL.
**Example**:
```sql
WHERE title ILIKE '%search%'
```
**Solution**: Use `LOWER(column) LIKE LOWER('%search%')` for MySQL.

### 3. Type Casting (3 instances)
**Problem**: PostgreSQL uses `::` for type casting.
**Example**:
```sql
SELECT id::text, created_at::date
```
**Solution**: Use MySQL's `CAST()` or conversion functions.

## Affected Areas

### Repositories with RETURNING clause:
- `article_repository.go` - Article creation
- `ticket_repository.go` - Ticket creation  
- `user_repository.go` - User creation
- `queue_repository.go` - Queue creation
- Multiple other repositories

### Search functionality with ILIKE:
- Ticket search
- User search
- Customer search
- Article search

## Impact Assessment

**High Priority**: 
- Article/ticket creation will fail due to RETURNING clause
- Search functionality will error with ILIKE

**Medium Priority**:
- Type casting issues (only 3 instances)

## Recommended Solution

1. **Create database adapter layer**:
   - Detect database type (MySQL vs PostgreSQL)
   - Use appropriate syntax for each database
   
2. **Update ConvertPlaceholders function**:
   - Handle RETURNING clause differently for MySQL
   - Convert ILIKE to MySQL equivalent
   - Handle type casting differences

3. **Use database-agnostic approaches**:
   - Avoid database-specific syntax where possible
   - Use ORM or query builder for complex queries

## Testing Required

After fixes:
1. Test ticket creation
2. Test article creation  
3. Test all search functionality
4. Test user/queue/customer creation
5. Verify data integrity maintained