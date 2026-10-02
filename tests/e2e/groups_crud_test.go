//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helpers shared by the /admin/groups tests (admin_groups_test.go,
// group_permissions_test.go, groups_visual_test.go, comprehensive_quality_test.go).

func groupsExpect() playwright.PlaywrightAssertions {
	return playwright.NewPlaywrightAssertions(10000)
}

// newGroupsAdminBrowser starts a browser signed in as the admin. TearDown runs
// as a cleanup, after the group cleanups registered later, so those still have
// a session.
func newGroupsAdminBrowser(t *testing.T) *helpers.BrowserHelper {
	t.Helper()
	b := newGroupsBrowser(t)
	require.NoError(t, helpers.NewAuthHelper(b).LoginAsAdmin(), "admin login")
	return b
}

func newGroupsBrowser(t *testing.T) *helpers.BrowserHelper {
	t.Helper()
	b := helpers.NewBrowserHelper(t)
	require.NoError(t, b.Setup(), "Failed to setup browser")
	t.Cleanup(b.TearDown)
	return b
}

func uniqueGroupName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// openGroupsPage loads /admin/groups and waits for the groups table.
func openGroupsPage(t *testing.T, b *helpers.BrowserHelper) {
	t.Helper()
	require.NoError(t, b.NavigateTo("/admin/groups"))
	require.NoError(t, groupsExpect().Locator(b.Page.Locator("table#groupsTable")).ToBeVisible())
}

// groupRow is the table row of the group with exactly this name.
func groupRow(b *helpers.BrowserHelper, name string) playwright.Locator {
	return b.Page.Locator(fmt.Sprintf("#groups-tbody tr[data-group-name=%q]", name))
}

// visibleGroupRows are the group rows the search and status filter leave visible.
func visibleGroupRows(b *helpers.BrowserHelper) playwright.Locator {
	return b.Page.Locator("#groups-tbody tr[data-group-name]:visible")
}

// rowGroupID reads the group id from the row id (group-row-<id>).
func rowGroupID(t *testing.T, row playwright.Locator) int {
	t.Helper()
	rowID, err := row.GetAttribute("id")
	require.NoError(t, err)
	id, err := strconv.Atoi(strings.TrimPrefix(rowID, "group-row-"))
	require.NoError(t, err, "row id %q", rowID)
	return id
}

// groupEndpoint matches the URL of /admin/groups (id 0) or /admin/groups/<id>.
func groupEndpoint(id int) *regexp.Regexp {
	if id == 0 {
		return regexp.MustCompile(`/admin/groups$`)
	}
	return regexp.MustCompile(fmt.Sprintf(`/admin/groups/%d$`, id))
}

// submitGroupRequest runs action and returns the response to the first
// request with this method to the group endpoint.
func submitGroupRequest(t *testing.T, b *helpers.BrowserHelper, method string, id int, action func() error) playwright.Response {
	t.Helper()
	return expectResponse(t, b, method, groupEndpoint(id), action)
}

// expectResponse runs action and returns the response to the first request
// with this method whose URL matches endpoint.
func expectResponse(t *testing.T, b *helpers.BrowserHelper, method string, endpoint *regexp.Regexp, action func() error) playwright.Response {
	t.Helper()
	ev, err := b.Page.ExpectEvent("response", action, playwright.PageExpectEventOptions{
		Predicate: func(r playwright.Response) bool {
			return r.Request().Method() == method && endpoint.MatchString(r.URL())
		},
	})
	require.NoError(t, err, "%s %s", method, endpoint)
	return ev.(playwright.Response)
}

// createGroupViaAPI creates a group through POST /admin/groups, the endpoint of
// the Add Group modal, and deletes it when t ends.
func createGroupViaAPI(t *testing.T, b *helpers.BrowserHelper, name, comments string, validID int) int {
	t.Helper()
	resp, err := b.Context.Request().Post(b.Config.BaseURL+"/admin/groups", playwright.APIRequestContextPostOptions{
		Form: map[string]any{"name": name, "comments": comments, "valid_id": strconv.Itoa(validID)},
	})
	require.NoError(t, err)
	body, _ := resp.Text()
	require.Equal(t, http.StatusCreated, resp.Status(), body)
	var created struct {
		Success bool `json:"success"`
		Group   struct {
			ID int `json:"id"`
		} `json:"group"`
	}
	require.NoError(t, resp.JSON(&created))
	require.True(t, created.Success, body)
	require.NotZero(t, created.Group.ID, body)
	cleanupGroupAtEnd(t, b, created.Group.ID)
	return created.Group.ID
}

// groupIsActive reads the group's status from GET /admin/groups/<id>.
func groupIsActive(t *testing.T, b *helpers.BrowserHelper, id int) bool {
	t.Helper()
	resp, err := b.Context.Request().Get(fmt.Sprintf("%s/admin/groups/%d", b.Config.BaseURL, id))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status())
	var group struct {
		Role struct {
			IsActive bool `json:"IsActive"`
		} `json:"role"`
	}
	require.NoError(t, resp.JSON(&group))
	return group.Role.IsActive
}

// cleanupGroupAtEnd deletes the group when t ends unless the test already did.
// Groups are soft-deleted (set inactive), as in OTRS.
func cleanupGroupAtEnd(t *testing.T, b *helpers.BrowserHelper, id int) {
	t.Helper()
	t.Cleanup(func() {
		if !groupIsActive(t, b, id) {
			return
		}
		resp, err := b.Context.Request().Delete(fmt.Sprintf("%s/admin/groups/%d", b.Config.BaseURL, id))
		if err != nil {
			t.Errorf("cleanup group %d: %v", id, err)
			return
		}
		if resp.Status() != http.StatusOK {
			body, _ := resp.Text()
			t.Errorf("cleanup group %d: HTTP %d %s", id, resp.Status(), body)
		}
	})
}

// rowNames returns the data-group-name of every row in table order.
func rowNames(t *testing.T, b *helpers.BrowserHelper) []string {
	t.Helper()
	names, err := b.Page.Locator("#groups-tbody tr[data-group-name]").EvaluateAll(
		"rows => rows.map(r => r.dataset.groupName.toLowerCase())")
	require.NoError(t, err)
	var out []string
	for _, n := range names.([]any) {
		out = append(out, n.(string))
	}
	return out
}

func TestGroupsCRUDOperations(t *testing.T) {
	b := newGroupsAdminBrowser(t)
	expect := groupsExpect()
	top := t

	name := uniqueGroupName("E2ECrud")
	desc := "crud-desc-" + strings.TrimPrefix(name, "E2ECrud_")
	updatedDesc := desc + "-updated"
	row := groupRow(b, name)
	searchInput := b.Page.Locator("input#groupSearch")
	statusFilter := b.Page.Locator("select#statusFilter")
	var groupID int

	t.Run("Create group by pressing Enter in the name field", func(t *testing.T) {
		openGroupsPage(t, b)
		require.NoError(t, b.Page.Locator("button:has-text('Add Group')").Click())
		require.NoError(t, expect.Locator(b.Page.Locator("#groupModal")).ToBeVisible())
		require.NoError(t, b.Page.Locator("#groupName").Fill(name))
		require.NoError(t, b.Page.Locator("#groupComments").Fill(desc))

		resp := submitGroupRequest(t, b, http.MethodPost, 0, func() error {
			return b.Page.Locator("#groupName").Press("Enter")
		})
		assert.Equal(t, http.StatusCreated, resp.Status())

		// The page reloads and lists the new, active group.
		require.NoError(t, expect.Locator(row).ToBeVisible())
		groupID = rowGroupID(t, row)
		cleanupGroupAtEnd(top, b, groupID)
		assert.NoError(t, expect.Locator(row).ToContainText(desc))
		assert.NoError(t, expect.Locator(row).ToHaveAttribute("data-active", "true"))
		assert.NoError(t, expect.Locator(row.Locator(".gk-badge-success")).ToHaveText("Active"))
		assert.NoError(t, expect.Locator(b.Page.Locator("#groupModal")).ToBeHidden())
	})
	require.NotZero(t, groupID, "the remaining steps need the created group")

	t.Run("Search filters rows by name and by description", func(t *testing.T) {
		require.NoError(t, searchInput.Fill(name))
		require.NoError(t, expect.Locator(visibleGroupRows(b)).ToHaveCount(1))
		assert.NoError(t, expect.Locator(row).ToBeVisible())

		require.NoError(t, searchInput.Fill(strings.ToUpper(desc)))
		require.NoError(t, expect.Locator(visibleGroupRows(b)).ToHaveCount(1), "search is case-insensitive and covers the description")
		assert.NoError(t, expect.Locator(row).ToBeVisible())

		require.NoError(t, searchInput.Fill("no-group-matches-"+name))
		require.NoError(t, expect.Locator(visibleGroupRows(b)).ToHaveCount(0))
		assert.NoError(t, expect.Locator(b.Page.Locator("#groupsNoMatch")).ToBeVisible())

		require.NoError(t, b.Page.Locator("#clearSearchBtn").Click())
		assert.NoError(t, expect.Locator(searchInput).ToHaveValue(""))
		total, err := b.Page.Locator("#groups-tbody tr[data-group-name]").Count()
		require.NoError(t, err)
		assert.NoError(t, expect.Locator(visibleGroupRows(b)).ToHaveCount(total))
		assert.NoError(t, expect.Locator(b.Page.Locator("#groupsNoMatch")).ToBeHidden())
	})

	t.Run("Edit group description", func(t *testing.T) {
		require.NoError(t, row.Locator("button[title='Edit group']").Click())
		modal := b.Page.Locator("#groupModal")
		require.NoError(t, expect.Locator(modal).ToBeVisible())
		assert.NoError(t, expect.Locator(b.Page.Locator("#modalAction")).ToHaveText("Edit"))
		assert.NoError(t, expect.Locator(b.Page.Locator("#groupName")).ToHaveValue(name))
		assert.NoError(t, expect.Locator(b.Page.Locator("#groupComments")).ToHaveValue(desc))
		assert.NoError(t, expect.Locator(b.Page.Locator("#groupStatus")).ToHaveValue("1"))

		require.NoError(t, b.Page.Locator("#groupComments").Fill(updatedDesc))
		resp := submitGroupRequest(t, b, http.MethodPut, groupID, func() error {
			return modal.Locator("button[type='submit']").Click()
		})
		assert.Equal(t, http.StatusOK, resp.Status())

		require.NoError(t, expect.Locator(row).ToContainText(updatedDesc))
		assert.NoError(t, expect.Locator(modal).ToBeHidden())
	})

	t.Run("Status filter shows active or inactive groups", func(t *testing.T) {
		_, err := statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
		require.NoError(t, err)
		assert.NoError(t, expect.Locator(row).ToBeVisible())
		assert.NoError(t, expect.Locator(b.Page.Locator("#groups-tbody tr[data-active='false']:visible")).ToHaveCount(0))

		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}})
		require.NoError(t, err)
		assert.NoError(t, expect.Locator(row).ToBeHidden())
		assert.NoError(t, expect.Locator(b.Page.Locator("#groups-tbody tr[data-active='true']:visible")).ToHaveCount(0))

		_, err = statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{""}})
		require.NoError(t, err)
		assert.NoError(t, expect.Locator(row).ToBeVisible())
	})

	t.Run("Search and status filter survive a reload", func(t *testing.T) {
		require.NoError(t, searchInput.Fill(name))
		_, err := statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
		require.NoError(t, err)
		require.NoError(t, expect.Locator(visibleGroupRows(b)).ToHaveCount(1))

		_, err = b.Page.Reload()
		require.NoError(t, err)
		assert.NoError(t, expect.Locator(searchInput).ToHaveValue(name))
		assert.NoError(t, expect.Locator(statusFilter).ToHaveValue("1"))
		assert.NoError(t, expect.Locator(visibleGroupRows(b)).ToHaveCount(1))
		assert.NoError(t, expect.Locator(row).ToBeVisible())

		// Clear resets both filters and the saved state.
		require.NoError(t, b.Page.Locator("button[title='Clear all filters']").Click())
		assert.NoError(t, expect.Locator(searchInput).ToHaveValue(""))
		assert.NoError(t, expect.Locator(statusFilter).ToHaveValue(""))
		_, err = b.Page.Reload()
		require.NoError(t, err)
		assert.NoError(t, expect.Locator(searchInput).ToHaveValue(""))
		total, err := b.Page.Locator("#groups-tbody tr[data-group-name]").Count()
		require.NoError(t, err)
		assert.NoError(t, expect.Locator(visibleGroupRows(b)).ToHaveCount(total))
	})

	t.Run("Sort by name and by members", func(t *testing.T) {
		nameHeader := b.Page.Locator("#groupsTable th[data-sort='name']")
		require.NoError(t, nameHeader.Click())
		require.NoError(t, expect.Locator(nameHeader).ToHaveAttribute("aria-sort", "descending"))
		names := rowNames(t, b)
		require.Greater(t, len(names), 1)
		assert.True(t, sort.SliceIsSorted(names, func(i, j int) bool { return names[i] > names[j] }), "descending: %v", names)

		require.NoError(t, nameHeader.Click())
		require.NoError(t, expect.Locator(nameHeader).ToHaveAttribute("aria-sort", "ascending"))
		names = rowNames(t, b)
		assert.True(t, sort.StringsAreSorted(names), "ascending: %v", names)

		// The page refreshes every member count after load; sort the final values.
		require.NoError(t, b.WaitForHTMX())
		membersHeader := b.Page.Locator("#groupsTable th[data-sort='members']")
		require.NoError(t, membersHeader.Click())
		require.NoError(t, expect.Locator(membersHeader).ToHaveAttribute("aria-sort", "ascending"))
		assert.NoError(t, expect.Locator(nameHeader).ToHaveAttribute("aria-sort", "none"))
		counts, err := b.Page.Locator("#groups-tbody tr[data-group-name] [id^='member-count-']").EvaluateAll(
			"spans => spans.map(s => parseInt(s.textContent, 10))")
		require.NoError(t, err)
		var members []int
		for _, c := range counts.([]any) {
			n, ok := c.(int)
			require.True(t, ok, "member count %v", c)
			members = append(members, n)
		}
		assert.True(t, sort.IntsAreSorted(members), "members ascending: %v", members)
	})

	t.Run("Delete group deactivates it", func(t *testing.T) {
		require.NoError(t, row.Locator("button[title='Delete group']").Click())
		deleteModal := b.Page.Locator("#deleteModal")
		require.NoError(t, expect.Locator(deleteModal).ToBeVisible())
		assert.NoError(t, expect.Locator(deleteModal).ToContainText("set to inactive"))

		resp := submitGroupRequest(t, b, http.MethodDelete, groupID, func() error {
			return deleteModal.Locator("button:has-text('Delete Group')").Click()
		})
		assert.Equal(t, http.StatusOK, resp.Status())
		assert.NoError(t, expect.Locator(deleteModal).ToBeHidden())
		assert.NoError(t, expect.Locator(row).ToHaveCount(0))
		assert.NoError(t, expect.Locator(b.Page.Locator("#toast-container")).ToContainText(
			fmt.Sprintf("Group \"%s\" has been deleted successfully.", name)))

		// As the dialog says, the group is kept as inactive and can be reactivated.
		openGroupsPage(t, b)
		require.NoError(t, expect.Locator(row).ToBeVisible())
		assert.NoError(t, expect.Locator(row).ToHaveAttribute("data-active", "false"))
		assert.NoError(t, expect.Locator(row.Locator(".gk-badge-muted")).ToHaveText("Inactive"))
	})

	t.Run("Name is required", func(t *testing.T) {
		openGroupsPage(t, b)
		require.NoError(t, b.Page.Locator("button:has-text('Add Group')").Click())
		modal := b.Page.Locator("#groupModal")
		require.NoError(t, expect.Locator(modal).ToBeVisible())
		require.NoError(t, b.Page.Locator("#groupComments").Fill("group without a name"))

		require.NoError(t, modal.Locator("button[type='submit']").Click())

		// The browser blocks the submit (no request, so no server error either).
		missing, err := b.Page.Locator("#groupName").Evaluate("el => el.validity.valueMissing", nil)
		require.NoError(t, err)
		assert.Equal(t, true, missing, "name is required")
		assert.NoError(t, expect.Locator(modal).ToBeVisible())
		assert.NoError(t, expect.Locator(b.Page.Locator("#formError")).ToBeHidden())

		require.NoError(t, b.Page.Locator("#groupModalCancelButton").Click())
		assert.NoError(t, expect.Locator(modal).ToBeHidden())
	})

	t.Run("Duplicate name is rejected", func(t *testing.T) {
		require.NoError(t, b.Page.Locator("button:has-text('Add Group')").Click())
		modal := b.Page.Locator("#groupModal")
		require.NoError(t, expect.Locator(modal).ToBeVisible())
		require.NoError(t, b.Page.Locator("#groupName").Fill("admin"))
		require.NoError(t, b.Page.Locator("#groupComments").Fill("duplicate of the admin group"))

		resp := submitGroupRequest(t, b, http.MethodPost, 0, func() error {
			return modal.Locator("button[type='submit']").Click()
		})
		var result struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		require.NoError(t, resp.JSON(&result))
		assert.False(t, result.Success)

		assert.NoError(t, expect.Locator(b.Page.Locator("#formError")).ToBeVisible())
		assert.NoError(t, expect.Locator(b.Page.Locator("#errorMessage")).ToHaveText("Group with this name already exists"))
		assert.NoError(t, expect.Locator(modal).ToBeVisible())
		assert.NoError(t, expect.Locator(b.Page.Locator("#groups-tbody tr[data-group-name='admin']")).ToHaveCount(1))

		require.NoError(t, b.Page.Locator("#groupModalCancelButton").Click())
		assert.NoError(t, expect.Locator(modal).ToBeHidden())
	})
}

func TestInactiveGroup(t *testing.T) {
	b := newGroupsAdminBrowser(t)
	expect := groupsExpect()
	top := t

	name := uniqueGroupName("E2EInactive")
	row := groupRow(b, name)
	statusFilter := b.Page.Locator("select#statusFilter")
	filterStatus := func(t *testing.T, value string) {
		t.Helper()
		_, err := statusFilter.SelectOption(playwright.SelectOptionValues{Values: &[]string{value}})
		require.NoError(t, err)
	}
	var groupID int

	t.Run("Create group with status Inactive", func(t *testing.T) {
		openGroupsPage(t, b)
		require.NoError(t, b.Page.Locator("button:has-text('Add Group')").Click())
		modal := b.Page.Locator("#groupModal")
		require.NoError(t, expect.Locator(modal).ToBeVisible())
		require.NoError(t, b.Page.Locator("#groupName").Fill(name))
		require.NoError(t, b.Page.Locator("#groupComments").Fill("created inactive by the e2e test"))
		_, err := b.Page.Locator("#groupStatus").SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}})
		require.NoError(t, err)

		resp := submitGroupRequest(t, b, http.MethodPost, 0, func() error {
			return modal.Locator("button[type='submit']").Click()
		})
		assert.Equal(t, http.StatusCreated, resp.Status())

		require.NoError(t, expect.Locator(row).ToBeVisible())
		groupID = rowGroupID(t, row)
		cleanupGroupAtEnd(top, b, groupID)
		assert.NoError(t, expect.Locator(row).ToHaveAttribute("data-active", "false"))
		assert.NoError(t, expect.Locator(row.Locator(".gk-badge-muted")).ToHaveText("Inactive"))
		assert.False(t, groupIsActive(t, b, groupID), "stored as inactive")
	})
	require.NotZero(t, groupID, "the remaining steps need the created group")

	t.Run("Status filter lists it only as inactive", func(t *testing.T) {
		filterStatus(t, "2")
		assert.NoError(t, expect.Locator(row).ToBeVisible())
		filterStatus(t, "1")
		assert.NoError(t, expect.Locator(row).ToBeHidden())
		filterStatus(t, "")
		assert.NoError(t, expect.Locator(row).ToBeVisible())
	})

	t.Run("Edit reactivates the group", func(t *testing.T) {
		require.NoError(t, row.Locator("button[title='Edit group']").Click())
		modal := b.Page.Locator("#groupModal")
		require.NoError(t, expect.Locator(modal).ToBeVisible())
		require.NoError(t, expect.Locator(b.Page.Locator("#groupStatus")).ToHaveValue("2"))
		_, err := b.Page.Locator("#groupStatus").SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
		require.NoError(t, err)

		resp := submitGroupRequest(t, b, http.MethodPut, groupID, func() error {
			return modal.Locator("button[type='submit']").Click()
		})
		assert.Equal(t, http.StatusOK, resp.Status())

		require.NoError(t, expect.Locator(row).ToHaveAttribute("data-active", "true"))
		assert.NoError(t, expect.Locator(row.Locator(".gk-badge-success")).ToHaveText("Active"))
		filterStatus(t, "2")
		assert.NoError(t, expect.Locator(row).ToBeHidden())
		filterStatus(t, "1")
		assert.NoError(t, expect.Locator(row).ToBeVisible())
		filterStatus(t, "")
		assert.True(t, groupIsActive(t, b, groupID), "stored as active")
	})
}
