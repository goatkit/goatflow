package dynamic

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
)

// The state module's Type filter takes several ticket_state_type ids (the
// admin "Open States" quick filter sends the new and open types, resolved by
// name from the DB-backed filter options), so it must match any of them.
func TestDynamicStateModuleTypeFilterMatchesSeveralTypes(t *testing.T) {
	r := newModulesTestRouter(t)
	db, err := database.GetDB()
	require.NoError(t, err)

	typeIDs, err := lookups.IDs(context.Background(), db, lookups.StateType,
		lookups.StateTypeNew, lookups.StateTypeOpen)
	require.NoError(t, err)
	idStrs := make([]string, len(typeIDs))
	for i, id := range typeIDs {
		idStrs[i] = fmt.Sprint(id)
	}

	code, body := serveDynamic(t, r, http.MethodGet,
		"/dynamic/state?page_size=100&filter_type_id="+strings.Join(idStrs, ","), nil)
	require.Equal(t, http.StatusOK, code, body)

	var names []string
	for _, it := range body["data"].([]any) {
		names = append(names, fmt.Sprint(it.(map[string]any)["name"]))
	}
	sort.Strings(names)
	assert.Subset(t, names, []string{lookups.StateNew, lookups.StateOpen})
	assert.NotContains(t, names, lookups.StatePendingReminder)
	assert.NotContains(t, names, lookups.StateClosedSuccessful)
}
