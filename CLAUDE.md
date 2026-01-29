
## ENTITY SELECTION MODAL UX BLUEPRINT - MANDATORY FOR ALL DIALOGS (Jan 12, 2026)

**This is the gold standard for entity selection modals (add users to role, assign agents to queue, etc.)**

Reference implementation: `templates/pages/admin/roles.pongo2` - roleUsersModal

### Modal Structure

```
+----------------------------------------------------------+
| [Icon] Modal Title                              [X Close] |
| Optional description/context text                         |
+----------------------------------------------------------+
| CURRENT MEMBERS                                           |
| [Filter members...] (local filter, instant)               |
| +------------------------------------------------------+ |
| | Member 1                              [Remove]       | |
| | Member 2                              [Remove]       | |
| +------------------------------------------------------+ |
+----------------------------------------------------------+
| ADD NEW MEMBERS                                           |
| [Search...] (API search, debounced)    [Spinner] [Enter] |
| +------------------------------------------------------+ |
| | Search Result 1                       [+ Add]        | |
| | Search Result 2                       [+ Add]        | |
| +------------------------------------------------------+ |
+----------------------------------------------------------+
| [Undo Toast - appears on remove, 5 second timeout]       |
+----------------------------------------------------------+
```

### API Design Pattern

```go
// Search endpoint - scalable, never returns all records
GET /admin/{entity}/:id/{members}/search?q={query}

// Requirements:
// - Minimum 2 characters required
// - Maximum 20 results returned
// - Excludes already-assigned members
// - Searches multiple fields (name, email, login, etc.)
// - Returns JSON: [{id, display_name, detail_info}, ...]
```

### JavaScript Patterns

```javascript
// 1. DEBOUNCED SEARCH (300ms delay)
let searchTimeout;
input.addEventListener('input', function() {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => performSearch(this.value), 300);
});

// 2. LOCAL MEMBER CACHE (for filtering and undo)
let currentMembers = []; // Populated on modal open
function filterMembers(query) {
    // Filter cached members client-side - instant response
}

// 3. OPTIMISTIC UI UPDATES
async function addMember(id) {
    // 1. Add to UI immediately
    appendMemberToList(member);
    // 2. Clear from search results
    removeFromSearchResults(id);
    // 3. KEEP search query (don't clear input)
    // 4. Call API in background
    const response = await fetch(...);
    if (!response.ok) {
        // 5. Rollback on failure
        removeMemberFromList(id);
        showError('Failed to add');
    }
}

// 4. UNDO PATTERN FOR DESTRUCTIVE ACTIONS
async function removeMember(id) {
    const member = getMemberData(id);
    // 1. Hide from UI immediately (don't delete)
    hideMemberRow(id);
    // 2. Show undo toast
    showUndoToast(member, () => {
        // Undo callback - restore UI
        showMemberRow(id);
    });
    // 3. Set delayed actual deletion
    undoTimeout = setTimeout(async () => {
        await fetch(`DELETE /api/.../${id}`);
        actuallyRemoveFromDOM(id);
    }, 5000);
}

// 5. KEYBOARD NAVIGATION
document.addEventListener('keydown', (e) => {
    if (!modalIsOpen) return;
    if (e.key === 'Escape') closeModal();
    if (e.key === 'Enter' && searchHasResults()) {
        e.preventDefault();
        addFirstSearchResult();
    }
});
```

### CSS/Visual Patterns

```css
/* Add button - green on hover */
.add-btn:hover { @apply bg-green-100 text-green-700; }

/* Remove button - red on hover */
.remove-btn:hover { @apply bg-red-100 text-red-700; }

/* Row animations */
.member-row {
    transition: all 0.2s ease-out;
}
.member-row.removing {
    opacity: 0;
    transform: translateX(-10px);
}
.member-row.adding {
    animation: slideIn 0.2s ease-out;
}

/* Undo toast - fixed bottom */
.undo-toast {
    @apply fixed bottom-4 right-4 bg-gray-800 text-white 
           px-4 py-3 rounded-lg shadow-lg flex items-center gap-3;
}
```

### UX Requirements Checklist

1. **Header**: Icon + Title + X close button (top-right)
2. **Member Filter**: Local filtering of cached members (instant)
3. **Search Input**: 
   - Minimum 2 characters
   - 300ms debounce
   - Loading spinner while searching
   - "Press Enter to add first result" hint
4. **Search Results**: Max 20 results, excludes existing members
5. **Add Action**:
   - Optimistic UI (instant feedback)
   - KEEP search query after adding
   - Green hover state on button
6. **Remove Action**:
   - Undo toast with 5-second window
   - Delayed actual deletion
   - Red hover state on button
7. **Keyboard**: Escape to close, Enter to add first result
8. **Animations**: Slide in/out on add/remove
9. **Empty States**: Show helpful messages when no members/results
10. **Error Handling**: Rollback UI on API failure, show toast

### NEVER DO THIS

- Load ALL available entities into the DOM (use search API)
- Clear search input after adding (user may want to add more)
- Delete immediately without undo option
- Use browser confirm() dialogs
- Block UI during API calls (use optimistic updates)
- Forget keyboard navigation
- Skip loading indicators during search

**Every entity selection modal in the product MUST follow this pattern.**

---

## TESTING INFRASTRUCTURE - MEMORIZE THIS (Jan 11, 2026)

**We have a FULL test stack with a dedicated database.**

### Test Database Setup
- Dedicated test database container available
- Tests run WITH a real database, not mocks
- Seed stage populates baseline data before tests
- After each test, database resets to baseline for next test
- Run tests with: `make test`

### How to Write Tests
1. Use the real database connection - DO NOT mock the database
2. Seed data is available - use it
3. Database resets between tests - each test starts clean
4. Integration tests should use the actual DB, not be skipped

### Makefile Targets for Testing
- `make test` - brings up test stack and runs all tests
- `make toolbox-test` - runs tests in toolbox container with DB access
- `make db-shell-test` - access the database directly

### NEVER DO THIS
- Don't write tests that skip because "no DB connection"
- Don't mock database calls when real DB is available. Spoiler: REAL test db is available.
- Don't claim low coverage is acceptable because "DB required"
- Don't use `// +build integration` tags to skip DB tests

**The test database EXISTS. Use it.**

---

## YAML ROUTING - SINGLE SOURCE OF TRUTH (Jan 27, 2026)

**There is ONE YAML route loader. Tests use the same router as production.**

### The Single Router

All YAML route loading goes through `internal/routing/loader.go`:

```go
// For production (main.go)
routing.LoadYAMLRoutesFromGlobalMap(router, routesPath)

// For tests and dev scenarios
routing.LoadYAMLRoutesForTesting(router)
```

Both use the SAME middleware registration in `RegisterExistingHandlers()`.

### NO Test Auth Bypass

**Tests MUST authenticate the same way production does.**

There is NO test auth bypass. The auth middleware:
1. Checks for JWT token in cookie or Authorization header
2. Validates the token
3. Returns 401 Unauthorized if missing/invalid

Tests that need authenticated endpoints must:
1. Call the login endpoint to get a token
2. Include the token in subsequent requests

```go
// Example: Get a token for tests
func getTestToken(t *testing.T, router *gin.Engine) string {
    resp := httptest.NewRecorder()
    body := `{"login":"test@example.com","password":"testpass"}`
    req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    router.ServeHTTP(resp, req)

    var result map[string]interface{}
    json.Unmarshal(resp.Body.Bytes(), &result)
    return result["access_token"].(string)
}

// Use the token
req.Header.Set("Authorization", "Bearer " + token)
```

### Why No Bypass?

1. **Tests verify real auth** - If auth is broken, tests fail
2. **No security risk** - No bypass code that could leak to production
3. **Prevents "tests pass, production fails"** - Same code path everywhere

### The Incident (Jan 27, 2026)

We had TWO separate YAML loaders:
- `internal/routing/loader.go` - Used by production
- `internal/api/yaml_router_loader.go` - Used by tests (with auth bypass)

Result: Tests passed but production returned 401 because the loaders handled middleware differently.

**Fix**: Consolidated to single loader, removed all auth bypass code.

### NEVER DO THIS

- Don't create a separate route loader for tests
- Don't add "test mode" auth bypass
- Don't inject fake user context in tests
- Don't use `APP_ENV=test` to skip authentication
- Don't check `gin.Mode() == gin.TestMode` to bypass auth

### Files

- `internal/routing/loader.go` - THE router (production + tests)
- `internal/routing/handlers.go` - Middleware registration (auth, admin, etc.)
- `internal/api/yaml_router_loader.go` - ONLY for manifest generation tooling

**One router. Real auth. No exceptions.**

---

## RUNNING GO TESTS - MANDATORY METHOD (Jan 22, 2026)

**ALWAYS use these Makefile targets to run Go tests:**

```bash
# Run tests for a specific package (optionally filtered by test name)
make toolbox-test-pkg PKG=./internal/api TEST=^TestLogin

# Run tests scoped to explicit test files
make toolbox-test-files FILES='path/to/a_test.go'

# Run a single Go test by name
make toolbox-test-run TEST=TestName
```

### NEVER DO THIS
- Don't use `make toolbox` with heredoc to run tests
- Don't use `docker exec` to run go test directly
- Don't run `go test` on the host machine

**Always use the Makefile targets for running tests. No exceptions.**

---

## DATABASE QUERIES - MANDATORY METHOD (Jan 22, 2026)

**ALWAYS use this method for ALL database queries:**

```bash
echo "SELECT * FROM table_name;" | make db-shell
```

### Examples
```bash
# List tables
echo "show tables;" | make db-shell

# Query customer users
echo "SELECT login, first_name, last_name FROM customer_user LIMIT 10;" | make db-shell

# Check specific record
echo "SELECT * FROM users WHERE id = 1;" | make db-shell
```

### NEVER DO THIS
- Don't use `docker exec` with mysql/mariadb client directly
- Don't use `make toolbox` with heredoc for queries
- Don't try to connect to the database any other way
- Don't guess or make up alternative methods

**This is the ONLY way to query the database. No exceptions.**

---

## DATABASE WRAPPER PATTERNS - ALWAYS USE THESE (Jan 11, 2026)

**Use `database.ConvertPlaceholders()` for all SQL queries. This allows future sqlx migration.**

### The Correct Pattern
```go
import "github.com/gotrs-io/gotrs-ce/internal/database"

// Write SQL with ? placeholders, convert before execution
query := database.ConvertPlaceholders(`
    SELECT id, name FROM users WHERE id = ? AND valid_id = ?
`)
row := db.QueryRowContext(ctx, query, userID, 1)

// For INSERT with RETURNING (handles MySQL vs PostgreSQL)
query := database.ConvertPlaceholders(`
    INSERT INTO users (name, email) VALUES (?, ?) RETURNING id
`)
query, useLastInsert := database.ConvertReturning(query)
if useLastInsert {
    result, err := db.ExecContext(ctx, query, name, email)
    id, _ = result.LastInsertId()
} else {
    err = db.QueryRowContext(ctx, query, name, email).Scan(&id)
}
```

### For Complex Operations Use GetAdapter()
```go
// GetAdapter() is for complex cases like InsertWithReturning
adapter := database.GetAdapter()
id, err := adapter.InsertWithReturning(db, query, args...)
```

### Test Code Uses Same Patterns
```go
func TestSomething(t *testing.T) {
    if err := database.InitTestDB(); err != nil {
        t.Skip("Database not available")
    }
    defer database.CloseTestDB()

    db, err := database.GetDB()
    require.NoError(t, err)

    // Use ConvertPlaceholders for queries
    query := database.ConvertPlaceholders(`SELECT id FROM users WHERE id = ?`)
    row := db.QueryRowContext(ctx, query, 1)
}
```

### Why This Pattern
- `ConvertPlaceholders()` handles MySQL vs PostgreSQL placeholder differences
- Designed so sqlx can be swapped in later
- `ConvertReturning()` handles RETURNING clause differences
- `GetAdapter()` for complex operations like InsertWithReturning

---

## ADDING NEW THEMES - THEME PACKAGE STRUCTURE (Jan 25, 2026)

Themes are self-contained packages in `static/themes/builtin/`. Each theme has its own directory with all assets.

### Theme Package Structure

```
static/themes/builtin/{theme-name}/
├── theme.yaml          # Theme metadata (name, description, features)
├── theme.css           # Main stylesheet with CSS variables
├── fonts/              # Theme-specific fonts (optional)
│   ├── fonts.css       # @font-face declarations with relative paths
│   └── {font-name}/    # Font files (woff2, ttf)
└── images/             # Theme-specific images (optional)
```

### Step 1: Create Theme Directory and theme.yaml

Create `static/themes/builtin/{theme-name}/theme.yaml`:

```yaml
name: Your Theme
id: your-theme-name
description: Short description of your theme
version: 1.0.0
author: Your Name
license: MIT

preview:
  gradient: "linear-gradient(135deg, #COLOR1, #COLOR2)"

modes:
  dark: true
  light: true
  default: dark

assets:
  fonts:
    enabled: true       # or false if using system fonts
    css: fonts/fonts.css
    license: SIL-OFL-1.1

features:
  glowEffects: false
  gridBackground: false
  animations: true
  bevels3d: false
  terminalMode: false

compatibility:
  minVersion: "1.0.0"
```

### Step 2: Create theme.css

Create `static/themes/builtin/{theme-name}/theme.css`:

```css
/* Dark mode (default) */
:root, :root.dark, .dark {
  --gk-theme-name: 'your-theme-name';
  --gk-theme-mode: 'dark';
  --gk-primary: #COLOR;
  --gk-bg-base: #COLOR;
  /* ... see existing themes for full list */
}

/* Light mode */
:root.light, .light {
  --gk-theme-mode: 'light';
  /* Override colors for light backgrounds */
}
```

Reference: `static/themes/builtin/synthwave/theme.css`

### Step 3: Add Fonts (if custom fonts needed)

1. **Download WOFF2 files** to `static/themes/builtin/{theme-name}/fonts/{font-name}/`
2. **Create fonts.css** with RELATIVE paths:

```css
@font-face {
  font-family: 'YourFont';
  font-style: normal;
  font-weight: 400;
  font-display: swap;
  src: url('your-font/your-font-latin.woff2') format('woff2');
}
```

3. **Update THIRD_PARTY_NOTICES.md** with font license info

### Step 4: Register in ThemeManager

Edit `static/js/theme-manager.js`:

```javascript
const AVAILABLE_THEMES = ['synthwave', 'gotrs-classic', 'seventies-vibes', 'nineties-vibe', 'your-new-theme'];
const BUILTIN_THEMES = ['synthwave', 'gotrs-classic', 'seventies-vibes', 'nineties-vibe', 'your-new-theme'];

const THEME_METADATA = {
  'your-new-theme': {
    name: 'Your Theme',
    nameKey: 'theme.your_theme',
    description: 'Theme description',
    descriptionKey: 'theme.your_theme_desc',
    gradient: 'linear-gradient(135deg, #COLOR1, #COLOR2)',
    hasFonts: true  // or false if using system fonts
  }
};
```

### Step 5: Add i18n Translations

Add to ALL 15 language files in `internal/i18n/translations/*.json`:

```json
"theme": {
    "your_theme": "Theme Name",
    "your_theme_desc": "Short description"
}
```

Languages: en, de, es, fr, pt, pl, ru, zh, ja, ar, he, fa, ur, uk, tlh

### Quick Reference: Required CSS Variables

| Category | Variables |
|----------|-----------|
| Primary | `--gk-primary`, `--gk-primary-hover`, `--gk-primary-active`, `--gk-primary-subtle` |
| Secondary | `--gk-secondary`, `--gk-secondary-hover`, `--gk-secondary-subtle` |
| Backgrounds | `--gk-bg-base`, `--gk-bg-surface`, `--gk-bg-elevated`, `--gk-bg-overlay` |
| Text | `--gk-text-primary`, `--gk-text-secondary`, `--gk-text-muted`, `--gk-text-inverse` |
| Borders | `--gk-border-default`, `--gk-border-strong` |
| Status | `--gk-success`, `--gk-warning`, `--gk-error`, `--gk-info` (+ `-subtle` variants) |
| Effects | `--gk-glow-primary`, `--gk-shadow-sm/md/lg/xl`, `--gk-focus-ring` |

### Files Created/Modified When Adding a Theme

1. `static/themes/builtin/{name}/theme.yaml` - Theme metadata (NEW)
2. `static/themes/builtin/{name}/theme.css` - Theme CSS (NEW)
3. `static/themes/builtin/{name}/fonts/` - Fonts directory (NEW, if custom fonts)
4. `static/js/theme-manager.js` - Add to AVAILABLE_THEMES, BUILTIN_THEMES, THEME_METADATA
5. `internal/i18n/translations/*.json` - Add translations (15 files)
6. `THIRD_PARTY_NOTICES.md` - Add font attribution (if custom fonts)

**Backend auto-discovers themes** from `static/themes/builtin/` directories.
**Template selectors read from `ThemeManager.THEME_METADATA`** - no template changes needed.
