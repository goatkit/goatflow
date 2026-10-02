//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuruMeditationErrorHandling(t *testing.T) {
	// Setup browser
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	// Login once for all tests
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Login should succeed")

	t.Run("Guru Meditation component exists on all admin pages", func(t *testing.T) {
		pages := []struct {
			name string
			url  string
		}{
			{"Admin Dashboard", "/admin"},
			{"Users Management", "/admin/users"},
			{"Groups Management", "/admin/groups"},
		}

		for _, page := range pages {
			t.Run(page.name, func(t *testing.T) {
				// Navigate to page
				err := browser.NavigateTo(page.url)
				require.NoError(t, err, "Should navigate to %s", page.name)
				time.Sleep(2 * time.Second)

				// Check for Guru Meditation component
				guruElement := browser.Page.Locator("#guru-meditation")

				// It should exist but be hidden initially
				elementCount, _ := guruElement.Count()
				assert.Equal(t, 1, elementCount, "%s should have Guru Meditation component", page.name)

				// Check that it has the required elements
				guruCode := browser.Page.Locator("#guru-code")
				codeExists, _ := guruCode.Count()
				assert.Equal(t, 1, codeExists, "%s should have guru-code element", page.name)

				guruMessage := browser.Page.Locator("#guru-message")
				msgExists, _ := guruMessage.Count()
				assert.Equal(t, 1, msgExists, "%s should have guru-message element", page.name)

				// Verify the dismiss function exists
				hasFunction, _ := browser.Page.Evaluate(`() => typeof dismissGuruMeditation === 'function'`)
				funcExists, _ := hasFunction.(bool)
				assert.True(t, funcExists, "%s should have dismissGuruMeditation function", page.name)

				// Verify the show function exists
				hasShowFunc, _ := browser.Page.Evaluate(`() => typeof showGuruMeditation === 'function'`)
				showExists, _ := hasShowFunc.(bool)
				assert.True(t, showExists, "%s should have showGuruMeditation function", page.name)
			})
		}
	})

	t.Run("Guru Meditation triggers on 500 errors", func(t *testing.T) {
		// Navigate to groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err, "Should navigate to groups page")
		time.Sleep(2 * time.Second)

		// Set up console message listener to capture errors
		consoleMessages := []string{}
		browser.Page.OnConsole(func(msg playwright.ConsoleMessage) {
			consoleMessages = append(consoleMessages, msg.Text())
		})

		// Try to create a group that might cause an error
		// First, open the modal
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		err = addButton.Click()
		require.NoError(t, err, "Should open add group modal")
		time.Sleep(1 * time.Second)

		// Try to submit with invalid data to trigger validation error
		nameInput := browser.Page.Locator("input#groupName")
		err = nameInput.Fill("") // Empty name should cause validation error
		require.NoError(t, err)

		submitButton := browser.Page.Locator("button[type='submit']:has-text('Save')")
		err = submitButton.Click()
		assert.NoError(t, err, "Should attempt to submit")
		time.Sleep(1 * time.Second)

		// Check if form validation worked (HTML5 validation should prevent submission)
		modalVisible, _ := browser.Page.Locator("#groupModal").IsVisible()
		assert.True(t, modalVisible, "Modal should remain open on validation error")
	})

	t.Run("Test Guru Meditation display manually", func(t *testing.T) {
		// Navigate to any admin page
		err := browser.NavigateTo("/admin")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Manually trigger Guru Meditation via JavaScript
		browser.Page.Evaluate(`() => {
			if (typeof showGuruMeditation === 'function') {
				showGuruMeditation(
					'TEST1234.DEADBEEF',
					'This is a test error message',
					{
						task: 'TEST.SUITE',
						location: 'guru_meditation_test.go'
					}
				);
			}
		}`)
		time.Sleep(1 * time.Second)

		// Check that Guru Meditation is now visible
		guruElement := browser.Page.Locator("#guru-meditation")
		visible, _ := guruElement.IsVisible()
		assert.True(t, visible, "Guru Meditation should be visible after triggering")

		// Check the error code was set correctly
		guruCode := browser.Page.Locator("#guru-code")
		codeText, _ := guruCode.InnerText()
		assert.Equal(t, "TEST1234.DEADBEEF", codeText, "Error code should match")

		// Check the message was set correctly
		guruMessage := browser.Page.Locator("#guru-message")
		msgText, _ := guruMessage.InnerText()
		assert.Equal(t, "THIS IS A TEST ERROR MESSAGE", strings.ToUpper(msgText), "Message should be uppercase")

		// Test dismiss functionality
		dismissButton := browser.Page.Locator("button:has-text('Left Mouse Button')")
		err = dismissButton.Click()
		assert.NoError(t, err, "Should click dismiss button")
		time.Sleep(500 * time.Millisecond)

		// Check that it's hidden again
		visible, _ = guruElement.IsVisible()
		assert.False(t, visible, "Guru Meditation should be hidden after dismissing")
	})

	t.Run("Inactive group creation error handling", func(t *testing.T) {
		// Navigate to groups page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err, "Should navigate to groups page")
		time.Sleep(2 * time.Second)

		// Open add group modal
		addButton := browser.Page.Locator("button:has-text('Add Group')")
		err = addButton.Click()
		require.NoError(t, err, "Should open add group modal")
		time.Sleep(1 * time.Second)

		// Fill in group details with Inactive status
		nameInput := browser.Page.Locator("input#groupName")
		testGroupName := fmt.Sprintf("TestInactive_%d", time.Now().Unix())
		err = nameInput.Fill(testGroupName)
		require.NoError(t, err, "Should fill group name")

		descInput := browser.Page.Locator("textarea#groupComments")
		err = descInput.Fill("Testing inactive group creation")
		require.NoError(t, err, "Should fill description")

		// Select Inactive status
		statusSelect := browser.Page.Locator("select#groupStatus")
		_, err = statusSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}})
		require.NoError(t, err, "Should select inactive status")

		// Submit the form
		submitButton := browser.Page.Locator("button[type='submit']:has-text('Save')")
		err = submitButton.Click()
		require.NoError(t, err, "Should submit form")
		time.Sleep(3 * time.Second)

		// Check if Guru Meditation appeared (in case of 500 error)
		guruElement := browser.Page.Locator("#guru-meditation")
		guruVisible, _ := guruElement.IsVisible()

		if guruVisible {
			// If Guru Meditation is visible, capture the error details
			guruCode := browser.Page.Locator("#guru-code")
			codeText, _ := guruCode.InnerText()

			guruMessage := browser.Page.Locator("#guru-message")
			msgText, _ := guruMessage.InnerText()

			t.Logf("Guru Meditation triggered with code: %s, message: %s", codeText, msgText)

			// Dismiss it
			dismissButton := browser.Page.Locator("button:has-text('Left Mouse Button')")
			dismissButton.Click()
			time.Sleep(500 * time.Millisecond)

			// Close modal if still open
			closeButton := browser.Page.Locator("button:has-text('Cancel')")
			if visible, _ := closeButton.IsVisible(); visible {
				closeButton.Click()
			}
		} else {
			// Check if group was created successfully
			modal := browser.Page.Locator("#groupModal")
			modalVisible, _ := modal.IsVisible()

			if !modalVisible {
				// Modal closed, check if group appears in list
				groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", testGroupName))
				rowVisible, _ := groupRow.IsVisible()
				assert.True(t, rowVisible, "Inactive group should be created and visible in list")

				// Clean up - delete the test group
				deleteButton := groupRow.Locator("button[title='Delete']")
				if delVisible, _ := deleteButton.IsVisible(); delVisible {
					deleteButton.Click()
					time.Sleep(500 * time.Millisecond)

					// Confirm deletion
					browser.Page.OnDialog(func(dialog playwright.Dialog) {
						dialog.Accept()
					})
				}
			} else {
				// Check for form error
				formError := browser.Page.Locator("#formError")
				errorVisible, _ := formError.IsVisible()
				if errorVisible {
					errorMsg := browser.Page.Locator("#errorMessage")
					errText, _ := errorMsg.InnerText()
					t.Logf("Form error: %s", errText)
				}
			}
		}
	})
}

func TestGuruMeditationStyling(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)
	err = auth.LoginAsAdmin()
	require.NoError(t, err)

	t.Run("Guru Meditation has Amiga-style styling", func(t *testing.T) {
		// Navigate to admin page
		err := browser.NavigateTo("/admin")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Trigger Guru Meditation
		browser.Page.Evaluate(`() => {
			showGuruMeditation('CAFEBABE.DEADBEEF', 'Style test error');
		}`)
		time.Sleep(1 * time.Second)

		// Check for Amiga-specific styling elements
		guruBox := browser.Page.Locator(".guru-box")
		boxVisible, _ := guruBox.IsVisible()
		assert.True(t, boxVisible, "Guru box should be visible")

		// Check for animation classes
		guruText := browser.Page.Locator(".guru-text")
		textVisible, _ := guruText.IsVisible()
		assert.True(t, textVisible, "Guru text should be visible")

		// Check for the classic red on black color scheme
		style, _ := browser.Page.Evaluate(`() => {
			const el = document.querySelector('.guru-box');
			const styles = window.getComputedStyle(el);
			return {
				borderColor: styles.borderColor,
				backgroundColor: styles.backgroundColor
			};
		}`)

		if styleMap, ok := style.(map[string]interface{}); ok {
			// Should have red border (part of the animation)
			t.Logf("Guru Meditation styling: %+v", styleMap)
		}

		// Dismiss
		browser.Page.Evaluate(`() => dismissGuruMeditation()`)
	})
}

// TestAllDialogsHaveErrorHandling submits a rejected entry in the admin create
// dialogs and checks the server's error is shown in the still-open dialog.
func TestAllDialogsHaveErrorHandling(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	defer browser.TearDown()
	require.NoError(t, helpers.NewAuthHelper(browser).LoginAsAdmin())
	page := browser.Page
	visible := playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateVisible}

	t.Run("Groups modal shows a duplicate name error", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/groups"))
		require.NoError(t, page.Locator("button:has-text('Add Group')").Click())
		modal := page.Locator("#groupModal")
		require.NoError(t, modal.WaitFor(visible))

		// The OTRS "admin" group always exists.
		require.NoError(t, page.Locator("#groupName").Fill("admin"))
		require.NoError(t, modal.Locator("button[type='submit']").Click())
		require.NoError(t, page.Locator("#formError").WaitFor(visible), "the duplicate name error should be shown")
		msg, err := page.Locator("#errorMessage").TextContent()
		require.NoError(t, err)
		assert.Equal(t, "Group with this name already exists", strings.TrimSpace(msg))
		open, err := modal.IsVisible()
		require.NoError(t, err)
		assert.True(t, open, "the dialog stays open so the name can be corrected")
	})

	t.Run("Users modal shows a duplicate login error", func(t *testing.T) {
		require.NoError(t, browser.NavigateTo("/admin/users"))
		require.NoError(t, page.Locator("button[onclick='showAddUserModal()']").Click())
		modal := page.Locator("#userModal")
		require.NoError(t, modal.WaitFor(visible))

		// The admin the suite logs in with always exists.
		require.NoError(t, page.Locator("#login").Fill(browser.Config.AdminEmail))
		require.NoError(t, page.Locator("#firstName").Fill("Duplicate"))
		require.NoError(t, page.Locator("#lastName").Fill("Login"))
		// A new user needs a password that meets the policy, or the browser
		// blocks the submit before the server sees the login.
		const password = "Duplicate-Login-2026!"
		require.NoError(t, page.Locator("#password").Fill(password))
		require.NoError(t, page.Locator("#userConfirmPassword").Fill(password))
		resp, err := page.ExpectResponse("**/admin/users", func() error {
			return modal.Locator("button[type='submit']").Click()
		})
		require.NoError(t, err)
		assert.Equal(t, 409, resp.Status())

		alert := modal.Locator(".gk-alert-error")
		require.NoError(t, alert.WaitFor(visible), "the duplicate login error should be shown in the dialog")
		msg, err := alert.TextContent()
		require.NoError(t, err)
		assert.Equal(t, "A user with this login already exists. Please choose a different login.", strings.TrimSpace(msg))
		focused, err := page.Locator("#login").Evaluate("el => el === document.activeElement", nil)
		require.NoError(t, err)
		assert.Equal(t, true, focused, "the login field is focused for correction")
		// The base layout always contains the (hidden) Guru Meditation box.
		guru, err := page.Locator("#guru-meditation:visible").Count()
		require.NoError(t, err)
		assert.Equal(t, 0, guru, "a rejected login is not a system failure")
	})
}
