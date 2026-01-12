
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
