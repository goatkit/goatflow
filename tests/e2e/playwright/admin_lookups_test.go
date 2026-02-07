//go:build e2e

package playwright

import (
	"testing"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminLookupsUI(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	if browser.Config.AdminEmail == "" || browser.Config.AdminPassword == "" {
		t.Skip("Admin credentials not configured")
	}
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()
	auth := helpers.NewAuthHelper(browser)

	t.Run("Lookups page loads with tabs", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)
		err = browser.NavigateTo("/admin/lookups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		url := browser.Page.URL()
		assert.Contains(t, url, "/admin/lookups")

		pageTitle := browser.Page.Locator("h1")
		assert.Greater(t, count(t, pageTitle), 0)

		prioritiesTab := browser.Page.Locator("#tab-priorities")
		assert.Greater(t, count(t, prioritiesTab), 0, "Priorities tab should exist")

		statesTab := browser.Page.Locator("#tab-states")
		assert.Greater(t, count(t, statesTab), 0, "States tab should exist")

		typesTab := browser.Page.Locator("#tab-types")
		assert.Greater(t, count(t, typesTab), 0, "Types tab should exist")
	})

	t.Run("Priorities tab shows data", func(t *testing.T) {
		err := browser.NavigateTo("/admin/lookups?tab=priorities")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		prioritiesList := browser.Page.Locator("#priorities-list")
		assert.Greater(t, count(t, prioritiesList), 0, "Priorities list container should exist")

		priorityItems := browser.Page.Locator("#priorities-list li")
		itemCount := count(t, priorityItems)
		assert.Greater(t, itemCount, 0, "Should have at least one priority item")

		addButton := browser.Page.Locator("button:has-text('Add')")
		assert.Greater(t, count(t, addButton), 0, "Add button should exist")
	})

	t.Run("States tab shows data", func(t *testing.T) {
		statesTab := browser.Page.Locator("#tab-states")
		require.NoError(t, statesTab.Click())
		require.NoError(t, browser.WaitForLoad())

		statesContent := browser.Page.Locator("#tab-content-states")
		v, err := statesContent.IsVisible()
		require.NoError(t, err)
		assert.True(t, v, "States tab content should be visible")

		statesList := browser.Page.Locator("#states-list")
		assert.Greater(t, count(t, statesList), 0, "States list container should exist")

		stateItems := browser.Page.Locator("#states-list li")
		itemCount := count(t, stateItems)
		assert.Greater(t, itemCount, 0, "Should have at least one state item")
	})

	t.Run("Types tab shows data", func(t *testing.T) {
		typesTab := browser.Page.Locator("#tab-types")
		require.NoError(t, typesTab.Click())
		require.NoError(t, browser.WaitForLoad())

		typesContent := browser.Page.Locator("#tab-content-types")
		v, err := typesContent.IsVisible()
		require.NoError(t, err)
		assert.True(t, v, "Types tab content should be visible")

		typesList := browser.Page.Locator("#types-list")
		assert.Greater(t, count(t, typesList), 0, "Types list container should exist")

		typeItems := browser.Page.Locator("#types-list li")
		itemCount := count(t, typeItems)
		assert.Greater(t, itemCount, 0, "Should have at least one type item")
	})

	t.Run("Tab switching works correctly", func(t *testing.T) {
		err := browser.NavigateTo("/admin/lookups")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		prioritiesContent := browser.Page.Locator("#tab-content-priorities")
		v, err := prioritiesContent.IsVisible()
		require.NoError(t, err)
		assert.True(t, v, "Priorities content should be visible by default")

		statesTab := browser.Page.Locator("#tab-states")
		require.NoError(t, statesTab.Click())
		require.NoError(t, browser.WaitForLoad())

		statesContent := browser.Page.Locator("#tab-content-states")
		v, err = statesContent.IsVisible()
		require.NoError(t, err)
		assert.True(t, v, "States content should be visible after clicking tab")

		prioritiesContent = browser.Page.Locator("#tab-content-priorities")
		v, err = prioritiesContent.IsVisible()
		require.NoError(t, err)
		assert.False(t, v, "Priorities content should be hidden after switching tab")

		typesTab := browser.Page.Locator("#tab-types")
		require.NoError(t, typesTab.Click())
		require.NoError(t, browser.WaitForLoad())

		typesContent := browser.Page.Locator("#tab-content-types")
		v, err = typesContent.IsVisible()
		require.NoError(t, err)
		assert.True(t, v, "Types content should be visible after clicking tab")
	})

	t.Run("Priority items display correctly", func(t *testing.T) {
		err := browser.NavigateTo("/admin/lookups?tab=priorities")
		require.NoError(t, err)
		require.NoError(t, browser.WaitForLoad())

		firstPriority := browser.Page.Locator("#priorities-list li").First()
		assert.Greater(t, count(t, firstPriority), 0, "Should have first priority")

		priorityName := firstPriority.Locator(".text-sm.font-medium")
		assert.Greater(t, count(t, priorityName), 0, "Priority should have name element")

		editButton := firstPriority.Locator("button")
		assert.Greater(t, count(t, editButton), 0, "Priority should have edit button")
	})

	t.Run("State items display correctly", func(t *testing.T) {
		statesTab := browser.Page.Locator("#tab-states")
		require.NoError(t, statesTab.Click())
		require.NoError(t, browser.WaitForLoad())

		firstState := browser.Page.Locator("#states-list li").First()
		assert.Greater(t, count(t, firstState), 0, "Should have first state")

		stateName := firstState.Locator(".text-sm.font-medium")
		assert.Greater(t, count(t, stateName), 0, "State should have name element")

		editButton := firstState.Locator("button")
		assert.Greater(t, count(t, editButton), 0, "State should have edit button")
	})

	t.Run("Type items display correctly", func(t *testing.T) {
		typesTab := browser.Page.Locator("#tab-types")
		require.NoError(t, typesTab.Click())
		require.NoError(t, browser.WaitForLoad())

		firstType := browser.Page.Locator("#types-list li").First()
		assert.Greater(t, count(t, firstType), 0, "Should have first type")

		typeName := firstType.Locator(".text-sm.font-medium")
		assert.Greater(t, count(t, typeName), 0, "Type should have name element")

		editButton := firstType.Locator("button")
		assert.Greater(t, count(t, editButton), 0, "Type should have edit button")
	})
}
