//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminUserID looks up the signed-in admin in GET /admin/users/list.
func adminUserID(t *testing.T, b *helpers.BrowserHelper) int {
	t.Helper()
	resp, err := b.Context.Request().Get(b.Config.BaseURL + "/admin/users/list")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status())
	var list struct {
		Users []struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
		} `json:"users"`
	}
	require.NoError(t, resp.JSON(&list))
	for _, u := range list.Users {
		if u.Login == b.Config.AdminEmail {
			return u.ID
		}
	}
	t.Fatalf("admin %q not in /admin/users/list", b.Config.AdminEmail)
	return 0
}

// groupMemberIDs reads GET /admin/groups/<id>/members.
func groupMemberIDs(t *testing.T, b *helpers.BrowserHelper, groupID int) []int {
	t.Helper()
	resp, err := b.Context.Request().Get(fmt.Sprintf("%s/admin/groups/%d/members", b.Config.BaseURL, groupID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status())
	var members struct {
		Members []struct {
			ID int `json:"id"`
		} `json:"members"`
	}
	require.NoError(t, resp.JSON(&members))
	ids := make([]int, 0, len(members.Members))
	for _, m := range members.Members {
		ids = append(ids, m.ID)
	}
	return ids
}

// removeMemberAtEnd takes the user out of the group when t ends, if the test
// left the membership in place.
func removeMemberAtEnd(t *testing.T, b *helpers.BrowserHelper, groupID, userID int) {
	t.Helper()
	t.Cleanup(func() {
		for _, id := range groupMemberIDs(t, b, groupID) {
			if id != userID {
				continue
			}
			resp, err := b.Context.Request().Delete(fmt.Sprintf("%s/admin/groups/%d/members/%d", b.Config.BaseURL, groupID, userID))
			if err != nil || resp.Status() != http.StatusOK {
				t.Errorf("cleanup membership %d in group %d: %v", userID, groupID, err)
			}
		}
	})
}

func TestGroupPermissionsButton(t *testing.T) {
	b := newGroupsAdminBrowser(t)
	expect := groupsExpect()

	name := uniqueGroupName("E2EPermissions")
	groupID := createGroupViaAPI(t, b, name, "permission matrix e2e group", 1)
	adminID := adminUserID(t, b)
	resp, err := b.Context.Request().Post(fmt.Sprintf("%s/admin/groups/%d/members", b.Config.BaseURL, groupID),
		playwright.APIRequestContextPostOptions{Data: map[string]any{"user_id": adminID}})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status())
	removeMemberAtEnd(t, b, groupID, adminID)

	memberRow := b.Page.Locator(fmt.Sprintf("[data-member-row='%d']", adminID))
	permission := func(key string) playwright.Locator {
		return memberRow.Locator(fmt.Sprintf("input[data-permission='%s']", key))
	}

	t.Run("Key icon opens the group's permission matrix", func(t *testing.T) {
		openGroupsPage(t, b)
		keyButton := groupRow(b, name).Locator("button[title='Manage permissions']")
		require.NoError(t, keyButton.Click())
		require.NoError(t, b.Page.WaitForURL(fmt.Sprintf("**/admin/groups/%d/permissions", groupID)))

		assert.NoError(t, expect.Locator(b.Page.Locator("h1")).ToHaveText("Queue Permissions"))
		assert.NoError(t, expect.Locator(b.Page.Locator("body")).ToContainText("Manage queue-centric permissions for "+name))
		assert.NoError(t, expect.Locator(b.Page.Locator("body")).ToContainText("No queues currently reference this group."))
		require.NoError(t, expect.Locator(memberRow).ToContainText(b.Config.AdminEmail))
		// Membership added through the members API is read/write.
		assert.NoError(t, expect.Locator(permission("rw")).ToBeChecked())
		assert.NoError(t, expect.Locator(permission("ro")).Not().ToBeChecked())
	})

	t.Run("Saved permission changes persist", func(t *testing.T) {
		// Clearing RW clears the whole row; then grant read and note only.
		require.NoError(t, permission("rw").Uncheck())
		for _, key := range []string{"ro", "move_into", "create", "note", "owner", "priority"} {
			assert.NoError(t, expect.Locator(permission(key)).Not().ToBeChecked(), key)
		}
		require.NoError(t, permission("ro").Check())
		require.NoError(t, permission("note").Check())

		saveURL := regexp.MustCompile(fmt.Sprintf(`/admin/groups/%d/permissions$`, groupID))
		saved := expectResponse(t, b, http.MethodPost, saveURL, func() error {
			return b.Page.Locator("#savePermissionsButton").Click()
		})
		assert.Equal(t, http.StatusOK, saved.Status())
		assert.NoError(t, expect.Locator(b.Page.Locator("#groupPermissionToast")).ToContainText("Permissions saved"))

		_, err := b.Page.Reload()
		require.NoError(t, err)
		require.NoError(t, expect.Locator(memberRow).ToBeVisible())
		assert.NoError(t, expect.Locator(permission("ro")).ToBeChecked())
		assert.NoError(t, expect.Locator(permission("note")).ToBeChecked())
		for _, key := range []string{"rw", "move_into", "create", "owner", "priority"} {
			assert.NoError(t, expect.Locator(permission(key)).Not().ToBeChecked(), key)
		}
	})

	t.Run("Back to Groups returns to the list", func(t *testing.T) {
		require.NoError(t, b.Page.Locator("a:has-text('Back to Groups')").Click())
		require.NoError(t, b.Page.WaitForURL("**/admin/groups"))
		assert.NoError(t, expect.Locator(groupRow(b, name)).ToBeVisible())
	})
}

func TestGroupMembersButton(t *testing.T) {
	b := newGroupsAdminBrowser(t)
	expect := groupsExpect()

	name := uniqueGroupName("E2EMembers")
	groupID := createGroupViaAPI(t, b, name, "membership e2e group", 1)
	adminID := adminUserID(t, b)
	removeMemberAtEnd(t, b, groupID, adminID)

	modal := b.Page.Locator("#groupMembersModal")
	membersList := b.Page.Locator("#groupMembersList")
	memberCount := b.Page.Locator(fmt.Sprintf("#member-count-%d", groupID))
	membersEndpoint := regexp.MustCompile(fmt.Sprintf(`/admin/groups/%d/members(/%d)?$`, groupID, adminID))
	openMembers := func(t *testing.T) {
		t.Helper()
		require.NoError(t, groupRow(b, name).Locator("a").Filter(playwright.LocatorFilterOptions{HasText: "members"}).Click())
		require.NoError(t, expect.Locator(modal).ToBeVisible())
	}
	membershipRequest := func(t *testing.T, method string, action func() error) {
		t.Helper()
		resp := expectResponse(t, b, method, membersEndpoint, action)
		require.Equal(t, http.StatusOK, resp.Status())
	}

	t.Run("Add the admin to the group", func(t *testing.T) {
		openGroupsPage(t, b)
		require.NoError(t, expect.Locator(memberCount).ToHaveText("0"))
		openMembers(t)
		require.NoError(t, expect.Locator(membersList).ToHaveText("No members in this group"))

		option := b.Page.Locator("#addUserSelect option").Filter(playwright.LocatorFilterOptions{HasText: b.Config.AdminEmail + " ("})
		require.NoError(t, expect.Locator(option).ToHaveCount(1))
		_, err := b.Page.Locator("#addUserSelect").SelectOption(playwright.SelectOptionValues{Values: &[]string{fmt.Sprint(adminID)}})
		require.NoError(t, err)
		membershipRequest(t, http.MethodPost, func() error {
			return modal.Locator("button:has-text('Add User')").Click()
		})

		assert.NoError(t, expect.Locator(membersList).ToContainText(b.Config.AdminEmail))
		assert.NoError(t, expect.Locator(memberCount).ToHaveText("1"))
		assert.NoError(t, expect.Locator(b.Page.Locator("#toast-container")).ToContainText("User added to group successfully"))
		assert.Equal(t, []int{adminID}, groupMemberIDs(t, b, groupID))

		require.NoError(t, modal.Locator(".gk-modal-footer button").Click())
		assert.NoError(t, expect.Locator(modal).ToBeHidden())
	})

	t.Run("Member count is stored", func(t *testing.T) {
		openGroupsPage(t, b)
		assert.NoError(t, expect.Locator(memberCount).ToHaveText("1"))
	})

	t.Run("Remove the admin from the group", func(t *testing.T) {
		openMembers(t)
		require.NoError(t, expect.Locator(membersList).ToContainText(b.Config.AdminEmail))

		confirmations := make(chan string, 1)
		b.Page.OnDialog(func(d playwright.Dialog) {
			select {
			case confirmations <- d.Message():
			default:
			}
			_ = d.Accept()
		})
		membershipRequest(t, http.MethodDelete, func() error {
			return membersList.Locator("button[title='Remove user from group']").Click()
		})
		assert.Contains(t, strings.ToLower(<-confirmations), "remove this user from the group")

		assert.NoError(t, expect.Locator(membersList).ToHaveText("No members in this group"))
		assert.NoError(t, expect.Locator(memberCount).ToHaveText("0"))
		assert.NoError(t, expect.Locator(b.Page.Locator("#toast-container")).ToContainText("User removed from group successfully"))
		assert.Empty(t, groupMemberIDs(t, b, groupID))
	})
}
