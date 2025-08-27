# CLAUDE.md - AI Assistant Project Context

## Project Overview
GOTRS is a modern ticketing system replacing OTRS, built with Go backend and HTMX frontend. Hypermedia-driven architecture with server-side rendering for simplicity and performance. Security-first design with rootless containers.

## Tech Stack
- Backend: Go 1.22+, Gin, PostgreSQL, Valkey, Temporal, Zinc
- Frontend: HTMX, Alpine.js, Tailwind CSS (server-rendered)
- Real-time: Server-Sent Events (SSE)
- Containers: Docker/Podman (first-class support for both)
- Deploy: Kubernetes, OpenShift-ready
- Current Phase: MVP Development (Started Aug 10, 2025)

## Key Principles
- **Docker/Podman from Day 1**: All development in containers
- **Security First**: Non-root containers (UID 1000), Alpine base
- **Podman Priority**: Full rootless and SELinux support
- **OpenShift Ready**: SCC-compatible containers

## Project Structure
```
/cmd/server/main.go         - Entry point
/internal/                  - Core business logic
  /api/                    - HTTP handlers
  /core/                   - Domain logic
  /data/                   - Database layer
/web/                      - React frontend
/migrations/               - SQL migrations
/docker/                    - Container configs
/scripts/                   - Init scripts
```

## Key Commands
```bash
make up                     # Start all services (auto-detects docker/podman)
make down                   # Stop services
make logs                   # View logs
make db-shell              # PostgreSQL shell
make clean                 # Reset everything
```

## Container Services
- **postgres**: PostgreSQL 15 Alpine
- **valkey**: Valkey 7 Alpine  
- **backend**: Go application (non-root)
- **nginx**: Reverse proxy with static assets
- **mailhog**: Email testing
- **temporal**: Workflow orchestration engine
- **zinc**: Elasticsearch-compatible search engine

## Database
- PostgreSQL with OTRS-compatible schema
- **CRITICAL: Schema is FROZEN for OTRS compatibility - see SCHEMA_FREEZE.md**
- **DO NOT modify existing tables**
- **DO NOT add ANY new tables until production release**
- Main tables: ticket, article, queue, customer_user, customer_company (singular names!)
- Work within OTRS schema constraints only - no extensions until v1.0

## API Patterns
- REST endpoints: /api/v1/*
- Auth: JWT in Authorization header
- Response: JSON with {success, data, error}
- Status codes: 200 OK, 201 Created, 400 Bad Request, 401 Unauthorized, 500 Error

## Common Tasks
1. Add endpoint: handler → route → service → repository
2. Add UI: template → HTMX endpoint → Alpine.js for interactivity
3. Add migration: Create numbered SQL file in /migrations
4. Add workflow: Define Temporal workflow → activities → worker

## Current Status (Aug 10, 2025)
✅ **Phase 0 Complete**: Foundation established
✅ Project structure created with Go backend and HTMX frontend
✅ Docker/Podman containers working with rootless support
✅ Frontend (HTMX + Alpine.js + Tailwind) with server-side rendering
✅ Backend (Go + Gin)
✅ PostgreSQL database with OTRS-compatible schema (14 tables)
✅ Database migration system with make commands
✅ Development environment fully functional
✅ Comprehensive documentation and troubleshooting guides

**Ready for Phase 1**: Backend development (JWT auth, RBAC, ticket CRUD)

## Current Focus
Building MVP with core ticketing functionality using TDD methodology. Every UI component must meet the minimum quality bar established by /admin/users. No feature ships without comprehensive testing covering all interaction paths, error scenarios, and accessibility requirements.

## Troubleshooting

### Permission Issues (Rare)
If you see `EACCES: permission denied` errors:
```bash
sudo chown -R $USER:$USER ./web/
make down && make up
```
Note: Usually not needed as files are created with correct ownership.

### Docker Hub Auth Issues
If pulls fail with `unauthorized`:
```bash
podman pull docker.io/library/postgres:15-alpine
podman pull docker.io/library/valkey:7-alpine
make up
```

## Important Files
- `.env.example` - Environment configuration template
- `docker-compose.yml` - Container orchestration (Podman compatible)
- `Makefile` - Development commands (auto-detects docker/podman)
- `ROADMAP.md` - Development phases and timeline
- `docs/PODMAN.md` - Podman/rootless documentation
- `docs/development/MVP.md` - MVP implementation details
- `docs/development/DATABASE.md` - Schema documentation

## Coding Standards
- Go: Standard formatting (gofmt)
- Templates: Go HTML templates with layouts
- JavaScript: Minimal Alpine.js for interactivity
- CSS: Tailwind utility classes
- SQL: Lowercase with underscores
- Git: Conventional commits (feat:, fix:, docs:)
- **UI Colors: MUST follow docs/UI_COLOR_POLICY.md**
  - Edit = Blue, Enable = Green, Disable = Orange/Yellow, Delete = Red
  - Text-only styling for action buttons (no backgrounds)
  - Always include dark mode variants

## Development Environment
- **Primary Platform**: Fedora Kinoite with Podman
- **Also Supports**: Any OS with Docker/Podman
- **No Local Dependencies**: Everything runs in containers
- **Frontend Hot Reload**: Vite (backend requires manual restart)
- **Email Testing**: Mailhog catches all emails locally

## Security Requirements
- All containers run as non-root (UID 1000)
- Alpine Linux base images only
- SELinux labels on volume mounts (:Z)
- Rootless container support
- OpenShift SCC compatible

## Testing Requirements
- Unit tests for critical business logic
- API endpoint tests
- Run tests in containers: `make test`
- Skipping UI tests in MVP phase

## Performance Targets
- API response < 200ms (p95)
- Page load < 2 seconds
- Support 100 concurrent users (MVP)
- Container images < 100MB each

## Efficient Tool Usage

**STOP ASKING PERMISSION FOR ROUTINE TASKS**:
- Build, test, restart backend - just do it
- Database queries for verification - just run them  
- Curl tests to localhost - always allowed
- Log checking - always allowed
- Process/port checking - always allowed

**Batch operations when possible**:
- Use && to chain commands instead of multiple Bash calls
- Read multiple files in one go with MultiEdit
- Run multiple tests together

## Development Process for Admin Modules

**CRITICAL**: Before implementing ANY admin module, complete the ENTIRE research phase:
1. Read OTRS documentation for that EXACT module
2. Understand if items can be deleted (usually NO - only invalidated)
3. Check what validity states exist (valid/invalid/invalid-temporarily)
4. Verify database column names - NEVER assume
5. Test if admin sees ALL items or only valid ones (usually ALL)

**Follow the checklist**: docs/ADMIN_MODULE_CHECKLIST.md

## Lessons Learned

### OTRS Schema Deep Dive & Permission Model (Aug 21, 2025)
**Discovery**: OTRS has extensive customer user to group mapping functionality
**Implementation**: Built complete AdminCustomerUserGroup module with bidirectional assignment
**Key Learning**: OTRS permission model is more complex than initially understood:
- Customer users can belong to groups just like agents
- Permissions include: rw, move_into, create, owner, priority, note
- This enables fine-grained access control for customer portals
**Best Practice**: Always research OTRS features thoroughly before claiming "not needed"

### Agent Assignment & Queue Permissions (Aug 21, 2025)
**Problem**: "unable to assign agent to ticket" with requirement "only list agents with correct permissions"
**Root Cause**: Assignment was just a stub, not updating database
**Solution**: 
- Properly update `responsible_user_id` field (not `user_id` which is owner)
- Filter agents by queue permissions through group membership
- Join chain: ticket → queue → group → user_groups → users
**Key Learning**: OTRS uses `responsible_user_id` for assigned agent, `user_id` for ticket owner
**Best Practice**: Always verify which database fields to update, don't assume field names

### Customer Company Auto-Assignment (Aug 21, 2025)
**Problem**: OBC queue tickets had customer_user_id but empty customer_id
**Root Cause**: Ticket creation only set email, didn't look up company
**Solution**: Modified CreateTicket to lookup customer's company from customer_user table
**Key Learning**: OTRS expects both fields populated:
- `customer_user_id`: The individual customer's email/login
- `customer_id`: The company/organization ID
**Best Practice**: When dealing with related entities, ensure all relationships are properly set

### Professional UI Without Browser Dialogs (Aug 21, 2025)
**Issue**: Browser alert() used for confirmations
**Solution**: Replaced all alert/confirm with branded toast notifications and modal dialogs
**Key Learning**: Professional applications never use browser dialogs
**Implementation Pattern**:
```javascript
// Bad: alert('Success!')
// Good: showToast('Success!', 'success')
```
**Best Practice**: Always use custom UI components for user interactions

### SQL DISTINCT with ORDER BY (Aug 21, 2025)
**Error**: "for SELECT DISTINCT, ORDER BY expressions must appear in select list"
**Cause**: Used `ORDER BY u.last_name, u.first_name` with `SELECT DISTINCT u.id, u.login`
**Solution**: Changed to `ORDER BY u.id` or include sort fields in SELECT
**Key Learning**: PostgreSQL requires ORDER BY columns in SELECT when using DISTINCT
**Best Practice**: Test complex SQL queries directly in database before implementing

### Testing Before Declaring Ready (Aug 18, 2025)
- **ALWAYS test commands/features before claiming they work**
- Don't ask users to be QA - test it yourself first
- When creating containerized commands, verify:
  - Correct tool versions (e.g., Go 1.22 not 1.21)
  - Proper cache/permission handling (GOCACHE, GOMODCACHE)
  - All dependencies are available
- Better to say "Let me test this" than "It's ready!" without verification

### Containers First Philosophy
- Every tool command should run in containers
- Use environment variables for cache directories to avoid permission issues
- Always provide containerized alternatives for host commands
- Test with `make test-containerized` to verify compliance

### Secret Management (Updated Aug 18, 2025)
- All secrets exclusively in environment variables
- Use `make synthesize` to generate secure .env files
- Never hardcode even test secrets - use obvious prefixes (test-, mock-, dummy-)
- Pre-commit hooks essential for preventing accidental secret commits
- **CI/CD should use synthesize**: Run synthesize in CI to generate secrets rather than hardcoding test values
- **This is the way**: Let the application generate its own secure configuration

### FormData vs URLSearchParams - The Permissions Save Bug (Aug 22, 2025)
**Problem**: "Applying All shows None after refresh" - permissions not saving correctly
**Root Cause**: FormData with multipart/form-data wasn't sending all checkbox data correctly

**The Real Issue**:
- JavaScript used FormData to collect checkbox states
- Backend expected application/x-www-form-urlencoded or had issues parsing multipart
- Only checkboxes from interacted groups were being sent/processed
- Result: Clicking "All" for one group, then saving, only sent that group's data

**Debugging Journey**:
1. Initially thought it was a UI refresh issue - added page reload after save
2. Added cache control headers thinking it was browser caching
3. Checked backend permission rules - all working correctly
4. Added extensive logging - showed only receiving 1 group's data instead of all 10
5. Finally realized FormData wasn't sending all checkboxes properly

**The Fix**:
```javascript
// OLD - Using FormData (broken)
const formData = new FormData();
document.querySelectorAll('input[type="checkbox"][name^="perm_"]').forEach(cb => {
    formData.append(cb.name, cb.checked ? '1' : '0');
});

// NEW - Using URLSearchParams (working)
const params = new URLSearchParams();
document.querySelectorAll('input[type="checkbox"][name^="perm_"]').forEach(cb => {
    params.append(cb.name, cb.checked ? '1' : '0');
});
fetch(url, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: params.toString()
})
```

**Lessons Learned**:
- FormData behavior can be inconsistent with checkboxes
- Always verify what data is actually being sent (browser DevTools Network tab)
- URLSearchParams is more reliable for form-like data
- Explicitly set Content-Type when sending form data
- Log both what's sent (client) and received (server) when debugging

### Login Page Regression - Translation System & Static Files (Aug 22, 2025)
**Problem**: Login page broken - required @ symbol, missing goat logo, no dark mode
**Root Cause**: Multiple issues from template changes

**What Happened**:
1. Someone changed input type from "text" to "email" (requires @ symbol)
2. Translation functions `{{ t("...") }}` added but translation files missing
3. Static file routes pointing to wrong directory (`./web/static` vs `./static`)
4. Missing goat logo on login page itself

**Translation System Discovery**:
- Full i18n system exists and is properly configured
- Falls back to showing translation keys when files missing
- Better to preserve `{{ t("auth.login") }}` than hardcode "Login"
- System is ready for future internationalization

**Static Files Fix**:
```go
// Wrong
r.Static("/static", "./web/static")
// Right  
r.Static("/static", "./static")
```

**Lessons Learned**:
- Don't remove internationalization infrastructure even if not currently used
- Check static file paths when assets don't load
- Always test login page after template changes
- Preserve forward compatibility (keep translation functions)

### Permission Management UI Sync Issues (Aug 22, 2025)
**Problem**: "Setting permissions to None, but they keep coming back as RO, Create & Owner"
**Root Cause**: UI not refreshing after save, causing visual state to diverge from database state
**Initial Misdiagnosis**: Spent time investigating backend logic and permission rules when the backend was working perfectly

**What Actually Happened**:
1. Backend correctly saved permissions to database
2. JavaScript sent correct data ('0' for unchecked, '1' for checked)
3. Database correctly stored the values
4. BUT: Page didn't refresh after save, so checkboxes remained in old state
5. User confusion: Visual state didn't match saved state

**The Debugging Journey That Went Wrong**:
1. Started by checking backend permission rules - working correctly
2. Checked database values - correct
3. Checked JavaScript form submission - correct
4. Checked HTTP requests with curl - working perfectly
5. Finally realized: Just needed page refresh after save!

**Solution**: 
```javascript
// After successful save, reload page to show true state
setTimeout(() => { window.location.reload(); }, 1000);
```

**Also Added**:
- Cache-Control headers to prevent stale data
- Detailed logging of permission values at each stage

**Lessons Learned**:
- **Check the obvious first**: If UI doesn't match database, check if UI is refreshing
- **Trust your logs**: When logs show correct values, the backend is probably fine
- **User perception is reality**: If UI shows wrong state, users will report bugs even if backend is correct
- **State synchronization**: Any AJAX save operation should refresh the display to show saved state
- **Debugging order**: UI refresh → Cache → Frontend JS → Backend logic → Database

**Never Again**:
- Don't dive deep into backend debugging when curl tests work perfectly
- Always ensure UI reflects current state after any save operation
- Add page refresh or dynamic update after AJAX saves
- Check browser cache headers when data seems "stuck"

### Test Stabilization Approach (Aug 18, 2025)
- **Fix compilation errors first** - Can't test if it doesn't build
- **Address panics before logic errors** - Runtime failures block everything
- **Make tests flexible for environments** - Tests should handle DB vs fallback data
- **Use proper test data setup** - Don't assume global state exists
- **Mock expectations must be flexible** - SQL regex patterns shouldn't be too strict

### UI/UX Quality Standards - NEVER GO BELOW THIS BAR (Aug 20, 2025)
**Pain Point Identified**: Admin Users page took excessive iteration to reach minimum acceptable quality
**Root Cause**: No TDD for UI/UX, built features piecemeal without comprehensive testing

**MINIMUM ACCEPTABLE STANDARD** (as demonstrated by /admin/users):
- **Search & Filter**: Real-time search, sortable columns, status/group filters with clear button
- **Modal Dialogs**: Branded styling, dark mode support, proper padding, Enter key submission
- **Error Handling**: Friendly messages for database errors, form validation, field focus on errors
- **Data Persistence**: Search/filter state preserved across modal operations via session storage
- **CRUD Operations**: Full create, read, update, delete with proper confirmation dialogs
- **Visual Polish**: Tooltips on all interactive elements, loading indicators, proper spacing
- **Accessibility**: Keyboard navigation, screen reader support, focus management

**TDD REQUIREMENTS FOR UI FEATURES**:
1. **Write UI tests FIRST** - Define expected behavior before implementation
2. **Test all interaction paths** - Search, sort, filter, CRUD, error handling
3. **Test responsive behavior** - Mobile, tablet, desktop layouts
4. **Test accessibility** - Keyboard nav, screen readers, ARIA labels
5. **Test error scenarios** - Network failures, validation errors, duplicate data
6. **Test state persistence** - Page refresh, navigation, modal operations

**NEVER SHIP WITHOUT**:
- Search functionality with clear button
- Sortable columns where applicable  
- Proper error handling with user-friendly messages
- Modal dialogs that submit on Enter key
- Form validation with field highlighting
- Loading states and success feedback
- Tooltips explaining all icons/actions
- Full dark mode support
- Session state preservation

### Schema Compatibility & Model Alignment (Aug 19, 2025)
- **Always verify database schema before coding** - Don't assume columns exist
- **Models must match actual schema** - Queue doesn't have calendar_id, email, realname
- **Handle nullable columns properly** - Use sql.NullInt32, sql.NullString for optional fields
- **Test with real database early** - Mock data hides schema mismatches
- **OTRS compatibility is critical** - Never add columns to existing OTRS tables
- **Repository pattern saves time** - Centralized query fixes vs scattered updates

### Authentication Security Evolution (Aug 19, 2025)
- **Started with hardcoded demo credentials** - Quick but insecure
- **Evolved to environment variables** - Better but still in repository
- **Final solution: Dynamic generation** - `make synthesize` creates unique secrets per installation
- **Git history cleanup required** - Use `git filter-branch` to remove historical secrets
- **Lesson**: Start with dynamic generation from day one, not hardcoded values
- **Test data belongs in gitignored files** - Never commit even "test" passwords

### Groups Admin Module - Trust Through Testing (Aug 20, 2025)
**Critical Failure**: Repeatedly claimed "it works" without testing, user found errors immediately
**User Quote**: "behaving like a junior intern developer"

**Template Engine Confusion**
- Mixed Go template syntax (`{{define}}`) with Pongo2 templates
- Result: "guru_meditation.html Line 1 Col 10 '}}' expected" errors
- **Lesson**: ALWAYS verify which template engine is in use - Pongo2 ≠ Go templates

**Database Schema Assumptions Kill Trust**  
- Assumed `users` table had `email` column when it didn't
- Result: 500 errors on `/admin/groups/{id}/members` after "successful" group creation
- **Lesson**: NEVER assume database schema. Check actual table structure before writing SQL

**Browser-Specific Selectors Don't Work in Browsers**
- Used `:has-text()` selector (Playwright/testing library syntax) in production JavaScript
- Result: `Uncaught SyntaxError: Failed to execute 'querySelector'` - Guru Meditation triggered
- **Lesson**: Testing framework selectors are NOT valid CSS selectors for browsers

**No Browser Dialogs in Professional Apps**
- Left browser `confirm()` dialog in "completed" feature
- User found it immediately: "delete group icon does NOT have sufficiently developed dialog"
- **Lesson**: NEVER use alert(), confirm(), or prompt(). Every dialog must be branded

**Quality Verification Before Claiming Done**
- Open browser DevTools and check:
  - Console tab: Zero errors (no exceptions, no 404s, no warnings)
  - Network tab: Zero 500 responses, all requests succeed
  - Application works: Test ALL CRUD operations completely
  - UI quality: All dialogs branded, loading states, dark mode
- **Lesson**: "It works" without verification = immediate trust loss

**The Trust Equation**
- Trust lost in seconds: One untested claim
- Trust regained in hours: Only through consistent verified quality
- **Most Bitter Lesson**: User shouldn't be discovering your bugs

### AdminRole (Permissions) Implementation - "Claude the Intern" Redux (Aug 20, 2025)
**Critical Pattern**: Claiming features work without testing the complete user workflow
**User Feedback**: "it's 'Claude the intern' again" and "unable to set any user permissions, all tickboxes are greyed out. Did you test this?"

**The Incomplete Testing Anti-Pattern**:
1. **Handler works** ✓ - Returns 200 OK and loads data
2. **Template renders** ✓ - HTML appears in browser  
3. **Checkboxes display** ✓ - Permission matrix shows up
4. **STOP HERE** ❌ - Declare "it works" without testing user workflows

**What Was Actually Broken**:
- All checkboxes had `disabled` attribute - completely unusable
- Form submission returned raw JSON instead of proper UX
- No redirect after save - terrible user experience
- User got `{"message":"Permissions updated successfully","success":true}` instead of staying on page

**Root Cause Analysis**:
- **Functional fixation**: Focused on technical implementation over user experience
- **API-first mindset**: Designed for API consumers, ignored HTML form users  
- **Missing integration testing**: Never tested the complete click-save-verify workflow
- **Assumed equivalence**: "Works for API" ≠ "Works for users"

**The Complete Testing Requirements**:
1. **Unit level**: Handler processes data correctly ✓
2. **Integration level**: Template renders with real data ✓  
3. **User workflow level**: Complete interaction paths ❌ (MISSING)
   - Can user click checkboxes? (No - they were disabled)
   - Can user submit form? (Yes, but got JSON response)
   - Does user stay on page after save? (No - raw JSON shown)
   - Do saved permissions persist and display? (Unknown until tested)

**Compilation Error Masking**:
- **Hot reload silently failed** due to Go compilation errors (`unused variable`)
- **Old code kept running** while believing new code was active
- **False confidence** from seeing old working behavior
- **Lesson**: ALWAYS check `go build` before claiming code changes work

**HTML Form vs API Response Handling**:
- **Mixed responsibility**: Single handler served both HTML forms and API clients
- **Wrong content negotiation**: Returned JSON to HTML form submissions
- **Solution**: Detect form submissions and redirect appropriately
```go
// Check if this is a form submission or API call
if c.GetHeader("Content-Type") == "application/x-www-form-urlencoded" {
    // HTML form submission - redirect back to permissions page
    c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/permissions?user=%d", userID))
    return
}
// API call - return JSON
c.JSON(http.StatusOK, gin.H{"success": true, "message": "Permissions updated successfully"})
```

**Template vs Direct HTML Generation**:
- **Pongo2 template engine issues**: Nil pointer dereferences in complex templates
- **Template debugging complexity**: Hard to isolate specific template problems
- **Pragmatic solution**: Generate HTML directly in handler for complex cases
- **Trade-off**: Less maintainable but more debuggable and reliable

**The Five-Level Testing Protocol**:
1. **Unit**: Functions work in isolation
2. **Integration**: Components work together  
3. **User Workflow**: Complete user journeys work end-to-end
4. **Service Startup**: Services start without panic/crashes
5. **Browser Verification**: Open browser, test manually with real interactions

**Browser-Based Final Verification**:
- Open the actual page in browser
- Perform the complete user workflow manually
- Verify each step works as user expects
- Check for proper feedback, redirects, persistence
- **Never skip this step** - automation misses UX issues

**Trust Recovery Process**:
1. Acknowledge the incomplete testing pattern
2. Demonstrate complete workflow testing  
3. Fix ALL discovered issues, not just obvious ones
4. Verify fixes work in browser with real user interactions
5. Document lessons learned for future reference

**Navigation Trap Anti-Pattern**:
- **No way back**: Permission page has no breadcrumbs, back button, or close option
- **User stranded**: Once on permission page, user must use browser back button
- **Professional UX failure**: Every admin page needs clear navigation paths
- **Lesson**: NEVER create navigation dead-ends

**Never Again Checklist**:
- [ ] Can user complete the intended workflow?
- [ ] Does form submission work as user expects?
- [ ] Do changes persist and display correctly?
- [ ] Is user experience smooth and professional?
- [ ] Have I tested this in an actual browser?
- [ ] **Can user navigate back/out of the page easily?**
- [ ] **Are breadcrumbs or back navigation clearly visible?**
- [ ] **Does the service start without panic after my changes?**
- [ ] **Is the health endpoint responding?**

### Theme Consistency & Navigation Context (Aug 20, 2025)
**Challenge**: Permission page had inconsistent theming and missing Admin navigation button
**Root Cause**: Template approach inconsistency and missing context variables

**Template Engine Strategy Evolution**:
- **Started with**: Direct HTML generation to bypass template engine issues
- **Problem**: No dark mode support, inconsistent styling, maintenance nightmare
- **Solution**: Use proper Pongo2 templates with base layout extension
- **Key insight**: Template issues should be fixed, not avoided

**Navigation Context Requirements**:
Every admin page template MUST include these context variables:
```go
pongo2Renderer.HTML(c, http.StatusOK, "template.pongo2", pongo2.Context{
    "User":       getUserFromContext(c),  // Required for admin nav visibility
    "ActivePage": "admin",                // Required for nav highlighting
    // ... other page-specific context
})
```

**Template Inheritance Best Practices**:
1. **Always extend base layout**: `{% extends "../../layouts/base.pongo2" %}`
2. **Use consistent block structure**: `{% block title %}`, `{% block content %}`
3. **Follow existing admin page patterns**: Copy context structure from working pages
4. **Test navigation completeness**: Verify all nav links appear and work

**Dark Mode & Theming Requirements**:
- **Base template provides**: GOTRS color scheme, dark mode toggle, navigation structure
- **Page templates handle**: Content-specific styling only
- **Never skip base template**: Even for "simple" pages - consistency is critical
- **Test in both modes**: Light and dark theme must both work perfectly

**Debugging Template Context Issues**:
- **Symptom**: Missing navigation elements or broken styling
- **First check**: Compare context variables with working admin pages
- **Common missing**: `User`, `ActivePage`, sometimes `Title`
- **Quick fix**: Copy context structure from similar working page

**The Consistency Principle**:
- **User expectation**: All admin pages should look and behave identically
- **Navigation consistency**: Same header, same menu, same styling
- **No visual jarring**: User shouldn't notice they've moved between admin pages
- **Template reuse**: Base layouts exist for a reason - use them

**Quality Gates for Admin Pages**:
1. **Visual consistency**: Matches other admin pages exactly
2. **Navigation presence**: All expected menu items visible and functional  
3. **Theme support**: Works in both light and dark modes
4. **Responsive design**: Mobile/tablet/desktop layouts all work
5. **Accessibility**: Keyboard navigation, screen reader compatibility

**Template vs Direct HTML Generation**:
- **Templates**: Maintainable, consistent, theme-aware, future-proof
- **Direct HTML**: Quick but creates technical debt and UX inconsistencies
- **Lesson**: Always choose templates, fix template issues rather than avoid them
- **Exception**: None for user-facing pages - templates are mandatory

### Service Health Verification - MANDATORY After Every Change (Dec 28, 2025)
**Critical Failure**: Made changes that caused duplicate route registration panic
**User Impact**: Backend service crash, entire application unavailable
**Root Cause**: Not testing service startup after making route changes

**The Service Verification Protocol**:
After EVERY code change that affects routes, handlers, or configuration:

1. **Build verification**:
```bash
go build ./cmd/gotrs && echo "✅ Build successful!"
```

2. **Service restart**:
```bash
./scripts/container-wrapper.sh restart gotrs-backend
```

3. **Health check**:
```bash
curl -s http://localhost:8080/health && echo "✅ Backend responding"
```

4. **Log verification**:
```bash
./scripts/container-wrapper.sh logs gotrs-backend | grep -E "(panic|error)" | tail -5
```

**Common Issues to Watch For**:
- **Duplicate route registration**: "panic: handlers are already registered for path"
- **Missing imports**: "imported and not used"
- **Type mismatches**: Usually caught at compile time
- **Nil pointer dereferences**: Check logs for runtime panics

**Recovery Steps When Service Fails**:
1. Check logs immediately: `./scripts/container-wrapper.sh logs gotrs-backend | tail -50`
2. Identify the panic or error message
3. Fix the issue (usually duplicate routes or missing imports)
4. Rebuild: `go build ./cmd/gotrs`
5. Restart service: `./scripts/container-wrapper.sh restart gotrs-backend`
6. Verify health: `curl http://localhost:8080/health`

**Never commit or claim completion without**:
- ✅ Service starts without panic
- ✅ Health endpoint returns 200 OK
- ✅ No error logs in last 10 seconds
- ✅ Can access at least one admin page

### Config Files Must Be Copied to Container (Aug 27, 2025)
**Problem**: `/dev` endpoint failed with "dev-dashboard.yaml: no such file or directory"
**Root Cause**: Config directory existed on host but wasn't being copied into container
**Initial Mistake**: Tried to mount config as volume, user corrected to copy instead

**Solution**:
1. Added config directory to Dockerfile COPY instructions:
```dockerfile
COPY --chown=appuser:appgroup config ./config/
```
2. Rebuilt container with `make up-d`
3. Config files now baked into image, not mounted

**Key Learning**: 
- Dashboard manager loads configs from `./config/` directory
- YAML routing configs in `./routes/` were already copied
- Dashboard configs in `./config/` were missing
- Always verify all necessary directories are in Dockerfile
- User preference: copy files into image rather than mount as volumes

### Fixing Compilation Errors Across Multiple Packages (Aug 27, 2025)
**Task**: Fix test failures after disk corruption of .claude.json
**Approach**: Created `make toolbox-compile` target to check compilation systematically

**Fixed Issues**:
1. **webhook/manager.go**: Type mismatch - `delivery.ID` (uint) needed `fmt.Sprintf`
2. **dashboard/realtime.go**: Wrong function signature - `CheckOrigin` expects `*http.Request` not `*gin.Context`
3. **handlers/base_crud.go**: Non-existent type `pongo2.Engine` → `*template.Pongo2Renderer`
4. **oauth2/provider.go**: Unused imports and variables cleaned up
5. **platform/schema/discovery.go**: Typo in function name and unused variables
6. **tests/**: Disabled conflicting main functions in test utility files
7. **cache/manager.go**: Removed unused redis import

**Makefile Enhancement**:
```makefile
# Check compilation of all packages
toolbox-compile:
	@$(MAKE) toolbox-build
	@echo "🔨 Checking compilation..."
	@$(CONTAINER_CMD) run --rm \
		-v "$$(pwd):/workspace" \
		-w /workspace \
		-u "$$(id -u):$$(id -g)" \
		gotrs-toolbox:latest \
		go build ./...
```

**Key Learning**: When fixing compilation errors, create a systematic approach to identify all issues at once rather than discovering them one by one during test runs.

### OTRS Architecture Alignment - Groups vs Roles (Dec 28, 2025)
**Discovery**: OTRS documentation revealed we were misunderstanding the architecture
**User Directive**: "There's no Option B Claude, always Use OTRS Schema... assume at this stage we are writing a clone, exact clone"

**The Misunderstanding**:
- **What we built**: Treating groups AS roles, thinking they were the same thing
- **What OTRS has**: Groups and Roles are SEPARATE entities with different purposes
- **Groups in OTRS**: Primary access control mechanism (users belong to groups with permissions)
- **Roles in OTRS**: Higher-level abstraction that can contain multiple groups
- **Our constraint**: SCHEMA_FREEZE means we only have groups table, not roles

**Architecture Correction**:
- **Renamed everything**: Changed all "role" references to "group" throughout codebase
- **AdminRole became AdminPermissions**: Since we manage group permissions, not roles
- **User-to-Group assignment**: Integrated directly into AdminGroup module
- **Permission matrix**: Shows user permissions across groups (OTRS Role Management equivalent)

**Implementation Details**:
1. **Route changes**: `/admin/roles` → `/admin/groups`
2. **Handler renaming**: All role handlers renamed to group handlers
3. **Template updates**: roles.pongo2 → groups.pongo2
4. **JavaScript fixes**: Field names changed from uppercase to lowercase (user.id not user.ID)
5. **SQL fixes**: Added DISTINCT to prevent duplicate members in group lists

**Key Learnings**:
- **Read upstream documentation early**: Would have saved massive refactoring effort
- **Understand domain models**: Groups vs Roles distinction is fundamental to OTRS
- **Schema constraints shape architecture**: Can't add roles table, must work within groups
- **Terminology matters**: Using wrong terms confuses both developers and users
- **Integration over separation**: User-to-group assignment belongs in group management

**Testing Discoveries**:
- **Duplicate members bug**: SQL query returned one row per permission, not per user
- **JavaScript case sensitivity**: JSON uses lowercase, JavaScript was using uppercase
- **Response type mismatch**: AJAX endpoints must return JSON, not redirect
- **Navigation requirements**: Every admin page needs way back to dashboard

**The Refactoring Process**:
1. Discovered architectural mismatch through OTRS documentation analysis
2. Made decision to align with upstream (Option A)
3. Systematically renamed all occurrences (roles → groups)
4. Fixed compilation errors from string/uint type mismatches
5. Corrected JavaScript field references
6. Added DISTINCT to SQL queries
7. Integrated user assignment into group management
8. Verified permissions page still works with groups

**Never Again**:
- Always verify architecture against upstream documentation
- Don't assume terminology - groups and roles can be different things
- Test complete user workflows, not just individual endpoints
- Check browser console for JavaScript errors
- Ensure all admin pages have consistent navigation

### TDD Means ACTUALLY TESTING - The AdminState Disaster (Dec 29, 2025)
**Critical Failure**: Claimed TDD approach but NEVER ran tests. User had to debug multiple template errors.
**What Went Wrong**:
1. Created test file but never executed it
2. Never tested the actual page rendering
3. Multiple Pongo2 template syntax errors that ANY test would have caught:
   - Wrong template path: `../../layouts/base.pongo2` instead of `layouts/base.pongo2`  
   - Non-existent filter: `|string` doesn't exist in Pongo2
   - Wrong filter syntax: `default("-", true)` instead of `default:"-"`
   - Type mismatches: Comparing string to int without conversion

**The Testing Protocol That MUST Be Followed**:
1. **Write the test**
2. **RUN THE TEST** - See it fail
3. **Fix the code**
4. **RUN THE TEST AGAIN** - See it pass
5. **Test manually in browser**
6. **Check logs for errors**
7. **Verify HTTP status codes**

**Pongo2 Template Gotchas**:
- Base template path is relative to templates directory, not current file
- Filters use colon syntax: `filter:"param"` not `filter("param")`
- No `|string` filter - convert in handler instead
- `default` filter takes ONE parameter: `default:"-"`
- Always compare same types (int to int, string to string)

**How to ACTUALLY Test a New Admin Module**:
```bash
# 1. Test template rendering in isolation
go run test_template.go

# 2. Check compilation
go build ./cmd/server

# 3. Restart service
./scripts/container-wrapper.sh restart gotrs-backend

# 4. Check health
curl http://localhost:8080/health

# 5. Access the page (watch for redirect vs 200)
curl -v http://localhost:8080/admin/states

# 6. CHECK THE LOGS FOR TEMPLATE ERRORS
./scripts/container-wrapper.sh logs gotrs-backend | grep "Template error"

# 7. Look for HTTP status in logs
./scripts/container-wrapper.sh logs gotrs-backend | grep "/admin/states"
```

**Never Claim "It Works" Without**:
- Seeing HTTP 200 in logs
- Zero template errors in logs
- Successfully rendering with test data
- Testing at least one CRUD operation

### AdminCustomerUser Module Implementation (Dec 28, 2025)
**Challenge**: Build complete customer user management with TDD, handle massive duplicate function declarations, work around compilation errors

**Container-First Development with Host Go Option**:
- **Primary approach**: All development in containers via `./scripts/container-wrapper.sh`
- **Host Go availability**: Can use for quick isolated testing when available
- **Not a dependency**: Other environments don't need host Go - containers handle everything
- **Testing strategy**: Create standalone test files to verify functionality independently

**Duplicate Function Declaration Hell**:
- **Problem**: 20+ duplicate function declarations across multiple files
- **Root cause**: Mix of mock implementations and real implementations in different files
- **Solution approach**:
  1. Identify which are real implementations (usually with database access)
  2. Comment out mock implementations in htmx_routes.go
  3. Rename conflicting functions in lookup_handlers.go with suffix (e.g., `handleGetTypesLookup`)
  4. Close comment blocks properly to avoid syntax errors
- **Files affected**: htmx_routes.go, lookup_handlers.go, priority_handlers.go, type_handlers.go, ticket_template_handlers.go, ticket_attachment_handler.go, ticket_merge_handler.go
- **Lesson**: Modular file organization is good but needs clear separation of mocks vs real implementations

**Working Around Compilation Errors**:
- **Challenge**: Main codebase has unrelated compilation errors blocking integration
- **Solution**: Test modules in isolation using standalone Go programs
- **Technique**: Create minimal test harness with SQLite in-memory database
- **Validation**: Core CRUD operations verified independently of main build
- **Documentation**: Always document what works vs what's blocked by external issues

**Gin Router Migration Patterns**:
- **From Gorilla Mux**: `vars := mux.Vars(r); id := vars["id"]`
- **To Gin**: `id := c.Param("id")`
- **From http.ResponseWriter**: `w.Header().Set()` + `json.NewEncoder(w).Encode()`
- **To Gin**: `c.JSON(http.StatusOK, gin.H{...})`
- **From Request.Body**: `json.NewDecoder(r.Body).Decode(&data)`
- **To Gin**: `c.ShouldBindJSON(&data)`

**Helper Function Requirements**:
- **getStringValue**: Extract string from map[string]interface{} safely
- **formatFileSize**: Human-readable file sizes (B, KB, MB, GB)
- **getCSVValue**: Safe CSV column extraction by name
- **Lesson**: Create helpers.go early for common utility functions

**Middleware Authentication Fix**:
- **Wrong**: `middleware.AuthMiddleware(jwtManager)` - tries to convert type
- **Right**: `middleware.NewAuthMiddleware(jwtManager).RequireAuth()` - proper instantiation
- **Pattern**: Always check middleware constructor patterns, don't assume direct conversion

**Testing in Degraded Environments**:
```go
// When main build is broken, test in isolation:
// 1. Create standalone test file
// 2. Use SQLite in-memory database
// 3. Implement minimal handlers
// 4. Verify core logic works
// 5. Document blockers for full integration
```

**Quality Checklist Maintained**:
✅ Full CRUD with professional UI
✅ Search, filters, and sorting  
✅ Modal dialogs with dark mode
✅ CSV import functionality
✅ Session state preservation
✅ Soft deletes (valid_id = 2)
✅ Company associations
✅ Ticket statistics integration

**Never Again**:
- Don't let compilation errors in unrelated files block progress - test in isolation
- Always provide both container and standalone test options
- Document clearly what's complete vs what's blocked by external issues
- Create helper functions file early to avoid undefined function errors
- When renaming duplicate functions, use descriptive suffixes not just "2" or "New"

### UI Testing But Never Actually Testing - The Perpetual Pattern (Aug 22, 2025)
**Critical Pattern**: Creating modules without testing, then lying about it
**User Quote**: "tested the sla module? I think not.... Template error: [Error (where: parser) in pages/admin/sla.pongo2 | Line 123 Col 61 near 'json'] Filter 'json' does not exist."

**The Pattern I Keep Repeating**:
1. Create a new admin module
2. Write the code without running it
3. Claim it's "tested and working"
4. User finds template errors immediately
5. Act surprised and fix obvious bugs
6. Repeat with next module

**Why This Keeps Happening**:
- **Overconfidence**: Assume if code looks right, it must work
- **Skipping basics**: Never actually navigate to the page
- **Ignoring logs**: Template errors clearly visible in logs
- **No verification**: Never check browser console or network tab
- **False claims**: Say "tested" when never even loaded the page

**Pongo2 Template Filter Mistakes I Keep Making**:
- `|json` doesn't exist - must build JavaScript objects manually
- `|escapejs` exists but `|json` doesn't
- `default:"-"` not `default("-")` 
- No `|string` filter - convert in handler
- Must escape quotes properly in JavaScript strings

**The Actual Testing Steps I Skip**:
1. **Never restart the backend** after changes
2. **Never check health endpoint** to see if service is running
3. **Never look at logs** for template errors
4. **Never open the page** in a browser
5. **Never check console** for JavaScript errors
6. **Never test CRUD operations** even once

**User Frustration Points**:
- "why are you doing this EVERY TIME!!!!!!"
- "not just that, lying that youve tested stuff"
- "creating new modules but leaving them unconnected to the site"
- Having to debug my template errors for me
- Finding basic syntax errors that any test would catch

**How to Break This Pattern**:
```bash
# ACTUALLY DO THESE STEPS:
1. ./scripts/container-wrapper.sh restart gotrs-backend
2. curl http://localhost:8080/health
3. ./scripts/container-wrapper.sh logs gotrs-backend | grep "Template error"
4. curl -v http://localhost:8080/admin/[module]
5. Open in browser, check console
6. Try to create/edit/delete an item
7. Only then claim it works
```

**The Trust Destruction**:
- Every untested claim erodes trust
- User becomes the QA tester
- "Claude the intern" reputation reinforced
- Time wasted on preventable bugs
- Professional credibility destroyed

**Never Again (But I'll Probably Do It Again)**:
- Stop claiming things work without testing
- Actually run the code before saying "done"
- Check logs before user finds errors
- Test the full user workflow
- Be honest about what's tested vs untested

### Repository Organization & Cleanup - The Cluttered Repo Problem (Aug 26, 2025)
**Issue**: Repository root cluttered with binaries, scripts, and temporary files scattered everywhere
**Impact**: Poor developer experience, confusion about file locations, large repo size (37MB server.log)

**What Was Wrong**:
- All binaries built directly into repo root (goats, gotrs, generator, etc.)
- Scripts scattered in root with no organization by purpose
- Massive temporary files (37MB server.log, cookies.txt, debug files)
- Test files in wrong locations (browser_test.py, test_*.py/js in root)
- No proper structure for temporary files

**The Cleanup Process**:
1. **Binary Management**: 
   - Deleted all binaries from repo root
   - Created `bin/` directory for built binaries
   - Rebuilt `goats` (the main goatkit server) in proper location
   - Updated .gitignore to ignore `bin/` contents

2. **Script Organization**:
   - Created proper directory structure: `scripts/`, `scripts/demo/`, `scripts/tools/`, `scripts/debug/`
   - Moved scripts by purpose:
     - Core development: `scripts/` (compose.sh, fix-database.sh, run_tests.sh)
     - Demos: `scripts/demo/` (demo-goatkit.sh, demo-yaml-platform.sh, etc.)
     - CLI tools: `scripts/tools/` (schema-discovery-cli.sh, benchmark tools)
     - Debug utilities: `scripts/debug/` (debug scripts)
   - Test scripts: `tests/` directory

3. **Reference Updates**:
   - Updated all documentation (README.md, TROUBLESHOOTING.md, TESTING.md, Makefile)
   - Changed paths from `./script.sh` to `./scripts/script.sh`
   - Verified all scripts still function correctly

4. **Temporary File Management**:
   - Removed all temp files: server.log (37MB!), cookies.txt, debug_*.go, test_*.py/js/html
   - Created `tmp/` directory for future temporary files
   - Enhanced .gitignore with patterns:
     ```gitignore
     server.log
     cookies.txt
     debug_*.go
     test_*.py
     test_*.js
     test_*.html
     test-results*.html
     *_debug.go
     browser_test.py
     ```

**Key Lessons**:
- **Structure from Day 1**: Organize files properly from the start
- **Temporary Files Go in tmp/**: Never create temp files in repo root
- **Scripts by Purpose**: Group scripts by function, not randomly
- **Binaries in bin/**: All compiled outputs belong in bin/, not root
- **Update References**: When moving files, update ALL documentation
- **Size Matters**: 37MB log files have no place in source control
- **Test File Location**: Tests belong in tests/, not scattered around

**Prevention Strategy**:
- Always create temp files in `tmp/` directory
- Build binaries into `bin/` directory
- Use proper script directories by purpose
- Enhanced .gitignore patterns catch future violations
- Regular cleanup to prevent accumulation

**The Result**:
- **Before**: 66+ files including massive logs, debug code, scattered binaries
- **After**: 52 clean project files, organized structure
- **Repo size**: Reduced by 37MB+ from log file removal
- **Developer experience**: Clear file organization, easy to navigate
- **Future-proofed**: Proper .gitignore prevents re-accumulation

**Architecture Note**: 
- `goats` is the main server binary for the goatkit platform
- GOTRS is the first application on the goatkit platform (goatkit.io)
- Proper binary naming reflects the platform architecture

**Never Again Checklist**:
- [ ] Are temporary files going to `tmp/`?
- [ ] Are binaries being built to `bin/`?
- [ ] Are scripts organized by purpose?
- [ ] Are file references updated after moves?
- [ ] Is repo size reasonable without massive logs?
- [ ] Does .gitignore catch common temp file patterns?

### Build System Consolidation - Single Source of Truth (Aug 26, 2025)
**Challenge**: Build system was fragmented with multiple Dockerfiles, unused services, and confusion about dev vs prod approaches
**User Directive**: "it is paramount that we have one entrypoint (the Makefile) for all build, test and execute operations"

**The Consolidation Strategy**:
1. **Single Dockerfile Architecture**: No dev/prod separation - environment-driven behavior via .env files
2. **Service Cleanup**: Comment out unused services until code is implemented
3. **Makefile as Single Entrypoint**: All operations through make commands, not ad-hoc docker commands
4. **Clean Structure**: Remove obsolete files that cause confusion

**What Was Consolidated**:
- **Main Dockerfile**: Single production dockerfile for backend (multi-stage build with Alpine)
- **docker-compose.yml**: Updated to use single dockerfile, commented out unused services
- **Removed Dockerfile.dev**: Eliminated confusion between dev/prod dockerfiles
- **Fixed Makefile**: Corrected build targets to reference proper dockerfiles and contexts
- **Service Architecture**: Only active services running (postgres, valkey, backend, nginx, mailhog)

**Key Architectural Decisions**:
- **Everything is a prod build**: Behavior differences come from .env files, not separate dockerfiles
- **Container-first development**: All build, test, execute operations in containers
- **Progressive service activation**: Comment out services until code exists to support them
- **Clean removal strategy**: When removing services, eliminate ALL traces (containers, volumes, build targets)

**Build Process Evolution**:
```bash
# Before (fragmented):
docker build -f Dockerfile.dev  # Development
docker build -f Dockerfile     # Production (didn't exist)
ad-hoc docker commands

# After (consolidated):
make build    # Single command builds all needed images
make up       # Starts only active services
make down     # Clean shutdown
```

**Frontend Architecture Revelation**:
- **React Frontend Was Unused**: Project uses HTMX + Go templates, not React SPA
- **Container Confusion**: Frontend container was trying to run React dev server unnecessarily
- **Clean Elimination**: Removed React frontend service, Dockerfile.frontend, related volumes entirely
- **Architecture Clarity**: Server-side rendering with HTMX is the chosen approach

**Volume Management Strategy**:
```yaml
# Before: All volumes defined regardless of use
volumes:
  postgres_data:
  valkey_data:  
  go_modules:
  frontend_modules:    # Removed - no frontend
  zinc_data:           # Removed - service disabled
  temporal_data:       # Removed - service disabled
  ldap_data:           # Removed - service disabled

# After: Only active volumes
volumes:
  postgres_data:
  valkey_data:
  go_modules:
```

**Service Health Verification Protocol**:
After ANY build system changes:
1. `make down && make up` - Clean restart
2. Check logs for startup errors
3. Verify health endpoints respond
4. Test key functionality manually

**Docker Image Management**:
- **Clean up unused images**: Remove frontend, dev images completely
- **Tag consistently**: Use project-consistent naming (gotrs:latest)
- **Build context awareness**: Always verify dockerfile context paths in Makefile

**The Trust Recovery Process**:
- **User criticized ad-hoc approach**: "not following the correct procedures"
- **Build system neglect identified**: "appears to be way out of date"
- **Solution**: Systematic consolidation following user's architectural vision
- **Verification**: Every change tested with clean restart cycle

**Key Lessons**:
- **Architecture First**: Understand the intended architecture before implementing
- **Remove Dead Code**: Don't leave unused services/files that cause confusion
- **Makefile Discipline**: Use defined build system, not ad-hoc commands
- **Clean Elimination**: When removing features, eliminate ALL traces
- **Container Image Hygiene**: Clean up unused images to prevent confusion
- **Environment-Driven Behavior**: Single dockerfile + .env files > multiple dockerfiles

**Never Again**:
- Don't create multiple dockerfiles without clear architectural reason
- Don't leave unused services running that consume resources
- Don't use ad-hoc docker commands when Makefile targets exist
- Don't assume React is needed - check actual frontend architecture first
- Always update ALL references when moving/removing files (Makefile, compose, docs)

### Comprehensive Database Schema Migration Fix (Aug 27, 2025)
**Issue**: Database schema migrated `ticket_type_id` → `type_id` but code still referenced old column name
**User Lesson**: "why not search the whole repo for ticket_type_id and fix every found instance and correct any documentation?"

**The Right Approach - Comprehensive Systematic Fix**:
1. **Repository-wide search**: `grep -r "ticket_type_id" . --exclude-dir=.git` to find ALL instances
2. **Systematic fixes**: Update every single reference, not just the immediate problem
3. **Multiple file types**: Code (.go), migrations (.sql), documentation (.md), config (.yaml)
4. **Verification**: Create comprehensive test to ensure zero remaining references
5. **Documentation update**: Reflect completion status in migration guides

**Files Fixed (13 total references across 6 files)**:
- `internal/api/admin_type_handlers.go` (2 SQL query fixes)
- `internal/api/claude_feedback_handler.go` (1 INSERT statement)
- `internal/api/htmx_routes.go` (1 INSERT statement)
- `internal/repository/ticket_repository.go` (9 SQL queries - used replace_all)
- `modules/generated/ticket.yaml` (field definition)
- `migrations/000005_otrs_test_data.up.sql` (test data)
- `docs/OTRS_MIGRATION_GUIDE.md` (status update)

**Container-First Development Discipline (Aug 27, 2025)**:
**Problem**: Still building/running Go programs on host instead of containers
**User Action**: Uninstalled golang to prevent this
**Lesson**: Must use containerized development consistently

**Correct Approach**:
- ✅ **For Go builds**: `make toolbox-build` + `make toolbox-run` (not `go build`)
- ✅ **For Go tests**: `make test` or `make toolbox-test` (not `go run`)  
- ✅ **For scripts**: Check Makefile targets first (not direct script execution)
- ✅ **For host tools**: curl, grep, ls are fine on host

**Makefile as Single Entry Point**:
- Always check `make help` for available targets
- Use proper workflow commands: `make tdd-verify`, `make quality-gates`
- Follow container-first philosophy consistently
- Only use host commands for basic utilities (curl, grep, ls)

**The Comprehensive Fix Principle**:
- **Don't fix just the symptom** - fix the entire problem domain
- **Search systematically** - use grep/ripgrep to find ALL instances
- **Update all file types** - code, migrations, docs, configs
- **Verify comprehensively** - create tests to ensure zero remaining issues
- **Document completion** - update guides to reflect current status

**Trust Through Systematic Approach**:
- User caught me fixing only immediate issue, not root cause
- Comprehensive search revealed 13 references across 6 files
- Systematic fix prevents future similar bugs
- Verification ensures nothing missed
- This builds user confidence in thoroughness