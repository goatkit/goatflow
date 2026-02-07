# Admin Module Development Checklist

## BEFORE STARTING ANY MODULE

### 1. OTRS Research Phase (MANDATORY)
- [ ] Search OTRS documentation for the exact module behavior
- [ ] Check if items can be deleted or only invalidated
- [ ] Understand the validity states (valid/invalid/invalid-temporarily)
- [ ] Document which fields are required vs optional
- [ ] Check if admin sees ALL items or only valid ones
- [ ] Verify exact database column names in OTRS schema
- [ ] Screenshot or document OTRS UI for reference

### 2. Planning Phase
- [ ] Write down expected behavior based on OTRS research
- [ ] List all CRUD operations needed
- [ ] Identify validation rules
- [ ] Document the database tables and columns involved
- [ ] Create test data scenarios

## DURING DEVELOPMENT

### 3. Test-First Development
- [ ] Write template render test FIRST
- [ ] Write CRUD operation tests
- [ ] Write validation tests
- [ ] **RUN THE TESTS** - Don't just write them!
- [ ] Verify tests fail before implementation
- [ ] Verify tests pass after implementation

### 4. Implementation Checklist
- [ ] Check all dependencies are included (Font Awesome, Alpine.js, etc.)
- [ ] Use correct database column names (verify, don't assume)
- [ ] Don't hardcode values in responses - use actual input values
- [ ] Admin modules show ALL items regardless of validity
- [ ] No delete buttons for configuration items (only soft delete via invalid)
- [ ] Include showToast function if using notifications
- [ ] Add validity field with tri-state dropdown

### 5. Template Requirements
- [ ] Extends correct base template
- [ ] All JavaScript functions defined before use
- [ ] Toast notifications included if needed
- [ ] Modal dialogs for create/edit operations
- [ ] Search and filter functionality
- [ ] Sort functionality on columns
- [ ] Validity badges (green/red/yellow)

## BEFORE DECLARING COMPLETE

### 6. Manual Testing Protocol
```bash
# 1. Build and restart
make toolbox-exec ARGS="go build ./cmd/server" && make restart

# 2. Check health
curl -s http://localhost:8080/health

# 3. Test page loads (should return 200)
curl -s "http://localhost:8080/admin/MODULE" -H "Cookie: access_token=demo_session_1755839704" -o /dev/null -w "%{http_code}"

# 4. Check logs for template errors
make logs | grep -i "error\|template"

# 5. Test CREATE with all validity states
curl -X POST "http://localhost:8080/admin/MODULE/create" \
  -H "Cookie: access_token=demo_session_1755839704" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "name=Test&valid_id=1"

# 6. Test UPDATE
curl -X POST "http://localhost:8080/admin/MODULE/ID/update" \
  -H "Cookie: access_token=demo_session_1755839704" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "name=Updated&valid_id=3"

# 7. Verify in database
PGPASSWORD=$DB_PASSWORD ./scripts/container-wrapper.sh exec goatflow-postgres \
  psql -h localhost -U goatflow_user -d goatflow -c "SELECT * FROM table_name WHERE ..."

# 8. Check browser console for JavaScript errors
Open browser DevTools Console - MUST show zero errors
```

### 7. Common Pitfalls to Avoid
- ❌ Filtering by valid_id in admin views
- ❌ Including delete buttons for configuration items
- ❌ Hardcoding response values
- ❌ Assuming database column names
- ❌ Declaring complete without running tests
- ❌ Missing Font Awesome or other dependencies
- ❌ showToast function not defined before use

## Success Criteria
- ✅ All tests pass
- ✅ Zero template errors in logs
- ✅ Zero JavaScript errors in browser console
- ✅ All CRUD operations work with all validity states
- ✅ UI matches OTRS behavior
- ✅ No delete functionality for configuration items
- ✅ Admin can see and manage ALL items regardless of validity

## Research Resources
- OTRS Admin Manual: https://doc.otrs.com/doc/manual/admin/
- OTRS Community: https://forums.otterhub.org/
- Database Schema: /migrations/001_initial_schema.sql
