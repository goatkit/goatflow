//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminGroupsUI(t *testing.T) {
	b := newGroupsAdminBrowser(t)
	expect := groupsExpect()

	t.Run("Admin dashboard links to group management", func(t *testing.T) {
		require.NoError(t, b.NavigateTo("/admin"))
		groupCard := b.Page.Locator("a[href='/admin/groups']").Filter(playwright.LocatorFilterOptions{HasText: "Group Management"})
		require.NoError(t, expect.Locator(groupCard).ToBeVisible())
		assert.NoError(t, expect.Locator(b.Page.Locator("text=Total Groups")).ToBeVisible())

		require.NoError(t, groupCard.Click())
		require.NoError(t, b.Page.WaitForURL("**/admin/groups"))
		assert.NoError(t, expect.Locator(b.Page.Locator("h1")).ToHaveText("Group Management"))
		assert.NoError(t, expect.Locator(b.Page.Locator("button:has-text('Add Group')")).ToBeVisible())
		assert.NoError(t, expect.Locator(b.Page.Locator("input#groupSearch")).ToBeVisible())
		assert.NoError(t, expect.Locator(b.Page.Locator("select#statusFilter")).ToBeVisible())
		for _, header := range []string{"Group Name", "Description", "Members", "Status", "Created"} {
			assert.NoError(t, expect.Locator(b.Page.Locator("#groupsTable thead th").Filter(
				playwright.LocatorFilterOptions{HasText: header})).ToBeVisible(), header)
		}
	})

	t.Run("Add Group modal opens empty and closes", func(t *testing.T) {
		openGroupsPage(t, b)
		require.NoError(t, b.Page.Locator("button:has-text('Add Group')").Click())
		modal := b.Page.Locator("#groupModal")
		require.NoError(t, expect.Locator(modal).ToBeVisible())

		assert.NoError(t, expect.Locator(b.Page.Locator("#modalAction")).ToHaveText("Add"))
		assert.NoError(t, expect.Locator(b.Page.Locator("input#groupName")).ToHaveValue(""))
		assert.NoError(t, expect.Locator(b.Page.Locator("input#groupName")).ToBeFocused())
		assert.NoError(t, expect.Locator(b.Page.Locator("textarea#groupComments")).ToHaveValue(""))
		assert.NoError(t, expect.Locator(b.Page.Locator("select#groupStatus")).ToHaveValue("1"))
		assert.NoError(t, expect.Locator(b.Page.Locator("select#groupStatus option")).ToHaveText([]string{"Active", "Inactive"}))
		assert.NoError(t, expect.Locator(modal.Locator("button[type='submit']")).ToHaveText("Save"))

		require.NoError(t, b.Page.Locator("#groupModalCancelButton").Click())
		assert.NoError(t, expect.Locator(modal).ToBeHidden())
	})

	t.Run("Clear button empties the search", func(t *testing.T) {
		openGroupsPage(t, b)
		clearButton := b.Page.Locator("#clearSearchBtn")
		require.NoError(t, expect.Locator(clearButton).ToBeHidden())

		require.NoError(t, b.Page.Locator("input#groupSearch").Fill("admin"))
		require.NoError(t, expect.Locator(clearButton).ToBeVisible())
		require.NoError(t, expect.Locator(b.Page.Locator("#groups-tbody tr[data-group-name='users']")).ToBeHidden())

		require.NoError(t, clearButton.Click())
		assert.NoError(t, expect.Locator(b.Page.Locator("input#groupSearch")).ToHaveValue(""))
		assert.NoError(t, expect.Locator(clearButton).ToBeHidden())
		assert.NoError(t, expect.Locator(b.Page.Locator("#groups-tbody tr[data-group-name='users']")).ToBeVisible())
	})

	t.Run("System groups are marked and cannot be deleted", func(t *testing.T) {
		openGroupsPage(t, b)
		for _, name := range []string{"admin", "users", "stats"} {
			row := groupRow(b, name)
			require.NoError(t, expect.Locator(row).ToBeVisible(), name)
			assert.NoError(t, expect.Locator(row.Locator(".gk-badge-accent")).ToHaveText("System"), name)
			assert.NoError(t, expect.Locator(row.Locator("button[title='System groups cannot be deleted']")).ToBeDisabled(), name)
			assert.NoError(t, expect.Locator(row.Locator("button[title='Delete group']")).ToHaveCount(0), name)
		}

		// The server refuses as well, and the group stays active.
		adminID := rowGroupID(t, groupRow(b, "admin"))
		resp, err := b.Context.Request().Delete(fmt.Sprintf("%s/admin/groups/%d", b.Config.BaseURL, adminID))
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.Status())
		body, _ := resp.Text()
		assert.Contains(t, body, "Cannot delete system groups")
		assert.True(t, groupIsActive(t, b, adminID))
	})
}
