//go:build e2e

package e2e

import (
	"regexp"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawTranslationKey matches an untranslated i18n key such as admin.group_name.
var rawTranslationKey = regexp.MustCompile(`\b(admin|app|buttons|common|groups|labels|messages)\.[a-z_]+(\.[a-z_]+)*\b`)

// assertTranslated checks the visible text shows each English label and no raw
// i18n key. Headers and buttons are upper-cased by CSS, so case is ignored.
func assertTranslated(t *testing.T, visibleText string, labels ...string) {
	t.Helper()
	text := strings.ToLower(visibleText)
	for _, label := range labels {
		assert.Contains(t, text, strings.ToLower(label))
	}
	assert.Empty(t, rawTranslationKey.FindAllString(text, -1), "untranslated i18n keys")
}

func TestGroupsUIVisual(t *testing.T) {
	b := newGroupsAdminBrowser(t)
	expect := groupsExpect()

	t.Run("Groups page shows translated labels", func(t *testing.T) {
		openGroupsPage(t, b)
		pageText, err := b.Page.Locator("body").InnerText()
		require.NoError(t, err)
		assertTranslated(t, pageText, "Group Management", "Add Group", "Group Name", "Description", "Members", "Status", "Created")
	})

	t.Run("Add Group modal shows translated labels", func(t *testing.T) {
		require.NoError(t, b.Page.Locator("button:has-text('Add Group')").Click())
		modal := b.Page.Locator("#groupModal")
		require.NoError(t, expect.Locator(modal).ToBeVisible())
		modalText, err := modal.InnerText()
		require.NoError(t, err)
		assertTranslated(t, modalText, "Add Group", "Group Name", "Description", "Status", "Save", "Cancel")

		require.NoError(t, b.Page.Locator("#groupModalCancelButton").Click())
		assert.NoError(t, expect.Locator(modal).ToBeHidden())
	})

	t.Run("Admin dashboard Groups card is translated", func(t *testing.T) {
		require.NoError(t, b.NavigateTo("/admin"))
		card := b.Page.Locator("a[href='/admin/groups']")
		require.NoError(t, expect.Locator(card).ToBeVisible())
		cardText, err := card.InnerText()
		require.NoError(t, err)
		assertTranslated(t, cardText, "Group Management", "Configure teams and escalation paths")
		statLabel, err := b.Page.Locator(".gk-stat-label").Filter(playwright.LocatorFilterOptions{HasText: "Groups"}).InnerText()
		require.NoError(t, err)
		assertTranslated(t, statLabel, "Total Groups")
	})
}

func TestGroupsPageResponsiveness(t *testing.T) {
	b := newGroupsAdminBrowser(t)
	expect := groupsExpect()

	viewports := []struct {
		name          string
		width, height int
	}{
		{"mobile", 375, 667},
		{"tablet", 768, 1024},
		{"desktop", 1920, 1080},
	}
	for _, vp := range viewports {
		t.Run(vp.name, func(t *testing.T) {
			require.NoError(t, b.Page.SetViewportSize(vp.width, vp.height))
			openGroupsPage(t, b)
			for _, selector := range []string{"h1", "button:has-text('Add Group')", "input#groupSearch", "select#statusFilter"} {
				control := b.Page.Locator(selector)
				require.NoError(t, expect.Locator(control).ToBeVisible(), selector)
				assertWithinViewport(t, b, selector, vp.width)
			}
			assert.NoError(t, expect.Locator(b.Page.Locator("table#groupsTable")).ToBeVisible())

			// The page is usable at this size: the Add Group modal opens and closes.
			require.NoError(t, b.Page.Locator("button:has-text('Add Group')").Click())
			require.NoError(t, expect.Locator(b.Page.Locator("#groupModal")).ToBeVisible())
			assertWithinViewport(t, b, "#groupModal button[type='submit']", vp.width)
			require.NoError(t, b.Page.Locator("#groupModalCancelButton").Click())
			assert.NoError(t, expect.Locator(b.Page.Locator("#groupModal")).ToBeHidden())
		})
	}
}

// assertWithinViewport checks the element is not cut off horizontally.
func assertWithinViewport(t *testing.T, b *helpers.BrowserHelper, selector string, width int) {
	t.Helper()
	box, err := b.Page.Locator(selector).BoundingBox()
	require.NoError(t, err, selector)
	require.NotNil(t, box, selector)
	assert.GreaterOrEqual(t, box.X, 0.0, "%s left edge", selector)
	assert.LessOrEqual(t, box.X+box.Width, float64(width), "%s right edge at %d px wide", selector, width)
}
