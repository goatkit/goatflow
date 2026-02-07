//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComprehensiveQuality ensures all features work correctly
// with both positive and negative test cases
func TestComprehensiveQuality(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err, "Failed to setup browser")
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	// Track all console messages and errors
	var consoleMessages []string
	var consoleErrors []string
	var networkErrors []string
	var mu sync.Mutex

	browser.Page.OnConsole(func(msg playwright.ConsoleMessage) {
		mu.Lock()
		defer mu.Unlock()

		text := msg.Text()
		consoleMessages = append(consoleMessages, fmt.Sprintf("[%s] %s", msg.Type(), text))

		if msg.Type() == "error" {
			consoleErrors = append(consoleErrors, text)
			t.Logf("Console ERROR: %s", text)
		}
	})

	browser.Page.OnResponse(func(response playwright.Response) {
		if response.Status() >= 500 {
			mu.Lock()
			networkErrors = append(networkErrors, fmt.Sprintf("%d %s", response.Status(), response.URL()))
			mu.Unlock()
			t.Logf("HTTP %d error: %s", response.Status(), response.URL())
		}
	})

	browser.Page.OnPageError(func(err error) {
		mu.Lock()
		consoleErrors = append(consoleErrors, err.Error())
		mu.Unlock()
		t.Logf("Page error: %v", err)
	})

	t.Run("Authentication Tests", func(t *testing.T) {
		t.Run("Positive: Valid login", func(t *testing.T) {
			err := browser.NavigateTo("/login")
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			err = browser.Page.Fill("input[name='email']", "admin@demo.com")
			require.NoError(t, err)

			err = browser.Page.Fill("input[name='password']", "demo123")
			require.NoError(t, err)

			err = browser.Page.Press("input[name='password']", "Enter")
			require.NoError(t, err)
			time.Sleep(2 * time.Second)

			// Should redirect to dashboard
			url := browser.Page.URL()
			assert.Contains(t, url, "/dashboard", "Should redirect to dashboard after login")
		})

		t.Run("Negative: Invalid credentials", func(t *testing.T) {
			// Logout first
			browser.NavigateTo("/logout")
			time.Sleep(1 * time.Second)

			err := browser.NavigateTo("/login")
			require.NoError(t, err)

			err = browser.Page.Fill("input[name='email']", "admin@demo.com")
			require.NoError(t, err)

			err = browser.Page.Fill("input[name='password']", "wrongpassword")
			require.NoError(t, err)

			err = browser.Page.Press("input[name='password']", "Enter")
			require.NoError(t, err)
			time.Sleep(2 * time.Second)

			// Should show error
			errorVisible, _ := browser.Page.Locator(".error, .alert-danger, [role='alert']").IsVisible()
			assert.True(t, errorVisible, "Should show error message for invalid login")

			// Should stay on login page
			url := browser.Page.URL()
			assert.Contains(t, url, "/login", "Should stay on login page")
		})

		t.Run("Edge: SQL injection attempt", func(t *testing.T) {
			err := browser.Page.Fill("input[name='email']", "admin'--")
			require.NoError(t, err)

			err = browser.Page.Fill("input[name='password']", "x")
			require.NoError(t, err)

			err = browser.Page.Press("input[name='password']", "Enter")
			require.NoError(t, err)
			time.Sleep(2 * time.Second)

			// Should not log in
			url := browser.Page.URL()
			assert.Contains(t, url, "/login", "SQL injection should not succeed")
		})
	})

	// Login for remaining tests
	err = auth.LoginAsAdmin()
	require.NoError(t, err, "Should login as admin")

	t.Run("Groups CRUD Tests", func(t *testing.T) {
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Clear previous errors before groups tests
		mu.Lock()
		consoleErrors = []string{}
		networkErrors = []string{}
		mu.Unlock()

		groupName := fmt.Sprintf("QualityTest_%d", time.Now().Unix())

		t.Run("Positive: Create valid group", func(t *testing.T) {
			// Open modal
			err := browser.Page.Click("button:has-text('Add Group')")
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			// Fill form
			err = browser.Page.Fill("input#groupName", groupName)
			require.NoError(t, err)

			err = browser.Page.Fill("textarea#groupComments", "Quality test group")
			require.NoError(t, err)

			// Submit
			err = browser.Page.Click("button[type='submit']:has-text('Save')")
			require.NoError(t, err)
			time.Sleep(3 * time.Second)

			// Check no errors occurred
			mu.Lock()
			errorCount := len(consoleErrors)
			networkErrorCount := len(networkErrors)
			mu.Unlock()

			assert.Equal(t, 0, errorCount, "No console errors should occur")
			assert.Equal(t, 0, networkErrorCount, "No network errors should occur")

			// Verify group appears
			groupVisible, _ := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", groupName)).IsVisible()
			assert.True(t, groupVisible, "Created group should appear in list")
		})

		t.Run("Negative: Create duplicate group", func(t *testing.T) {
			// Open modal
			err := browser.Page.Click("button:has-text('Add Group')")
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			// Try same name
			err = browser.Page.Fill("input#groupName", groupName)
			require.NoError(t, err)

			err = browser.Page.Fill("textarea#groupComments", "Duplicate attempt")
			require.NoError(t, err)

			// Submit
			err = browser.Page.Click("button[type='submit']:has-text('Save')")
			require.NoError(t, err)
			time.Sleep(2 * time.Second)

			// Should show error
			formError, _ := browser.Page.Locator("#formError").IsVisible()
			assert.True(t, formError, "Should show error for duplicate group")

			// Modal should stay open
			modalVisible, _ := browser.Page.Locator("#groupModal").IsVisible()
			assert.True(t, modalVisible, "Modal should remain open on error")

			// Close modal
			browser.Page.Click("button:has-text('Cancel')")
			time.Sleep(1 * time.Second)
		})

		t.Run("Negative: Create without required field", func(t *testing.T) {
			// Open modal
			err := browser.Page.Click("button:has-text('Add Group')")
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			// Don't fill name
			err = browser.Page.Fill("textarea#groupComments", "No name test")
			require.NoError(t, err)

			// Try to submit
			err = browser.Page.Click("button[type='submit']:has-text('Save')")
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			// HTML5 validation should prevent submission
			modalVisible, _ := browser.Page.Locator("#groupModal").IsVisible()
			assert.True(t, modalVisible, "Modal should remain open when validation fails")

			// Close modal
			browser.Page.Click("button:has-text('Cancel')")
			time.Sleep(1 * time.Second)
		})

		t.Run("Positive: Create inactive group", func(t *testing.T) {
			inactiveName := fmt.Sprintf("Inactive_%d", time.Now().Unix())

			// Open modal
			err := browser.Page.Click("button:has-text('Add Group')")
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			// Fill form with inactive status
			err = browser.Page.Fill("input#groupName", inactiveName)
			require.NoError(t, err)

			err = browser.Page.Fill("textarea#groupComments", "Inactive group test")
			require.NoError(t, err)

			_, err = browser.Page.Locator("select#groupStatus").SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}})
			require.NoError(t, err)

			// Submit
			err = browser.Page.Click("button[type='submit']:has-text('Save')")
			require.NoError(t, err)
			time.Sleep(3 * time.Second)

			// Check no 500 errors
			mu.Lock()
			hasServerError := false
			for _, err := range networkErrors {
				if strings.Contains(err, "500") {
					hasServerError = true
					break
				}
			}
			mu.Unlock()
			assert.False(t, hasServerError, "No 500 errors should occur when creating inactive group")

			// Verify group appears
			groupVisible, _ := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", inactiveName)).IsVisible()
			assert.True(t, groupVisible, "Inactive group should appear in list")
		})

		t.Run("Edge: XSS attempt in group name", func(t *testing.T) {
			xssName := fmt.Sprintf("XSS_%d", time.Now().Unix())
			xssPayload := "<script>alert('XSS')</script>"

			// Open modal
			err := browser.Page.Click("button:has-text('Add Group')")
			require.NoError(t, err)
			time.Sleep(1 * time.Second)

			// Try XSS in comments
			err = browser.Page.Fill("input#groupName", xssName)
			require.NoError(t, err)

			err = browser.Page.Fill("textarea#groupComments", xssPayload)
			require.NoError(t, err)

			// Submit
			err = browser.Page.Click("button[type='submit']:has-text('Save')")
			require.NoError(t, err)
			time.Sleep(3 * time.Second)

			// Check no script execution
			mu.Lock()
			hasXSSAlert := false
			for _, msg := range consoleMessages {
				if strings.Contains(msg, "XSS") {
					hasXSSAlert = true
					break
				}
			}
			mu.Unlock()
			assert.False(t, hasXSSAlert, "XSS payload should not execute")

			// Check if properly escaped in DOM
			pageContent, _ := browser.Page.Content()
			assert.NotContains(t, pageContent, "<script>alert", "Script tags should be escaped")
		})

		t.Run("Performance: Page load time", func(t *testing.T) {
			start := time.Now()
			err := browser.NavigateTo("/admin/groups")
			require.NoError(t, err)
			time.Sleep(2 * time.Second)

			loadTime := time.Since(start)
			assert.Less(t, loadTime, 3*time.Second, "Page should load within 3 seconds")

			if loadTime < 1*time.Second {
				t.Logf("✓ Excellent: Page loaded in %v", loadTime)
			} else if loadTime < 2*time.Second {
				t.Logf("✓ Good: Page loaded in %v", loadTime)
			} else {
				t.Logf("⚠ Slow: Page loaded in %v", loadTime)
			}
		})

		t.Run("Cleanup: Delete test groups", func(t *testing.T) {
			// Find and delete test groups
			groupRow := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", groupName))
			if visible, _ := groupRow.IsVisible(); visible {
				deleteBtn := groupRow.Locator("button[title='Delete']")
				if btnVisible, _ := deleteBtn.IsVisible(); btnVisible {
					deleteBtn.Click()
					time.Sleep(500 * time.Millisecond)

					// Handle confirmation
					browser.Page.OnDialog(func(dialog playwright.Dialog) {
						dialog.Accept()
					})

					// Check for custom confirm modal
					confirmBtn := browser.Page.Locator("button:has-text('Delete')").Last()
					if confirmVisible, _ := confirmBtn.IsVisible(); confirmVisible {
						confirmBtn.Click()
					}
					time.Sleep(2 * time.Second)
				}
			}
		})
	})

	t.Run("Error Handling Tests", func(t *testing.T) {
		t.Run("Guru Meditation component present", func(t *testing.T) {
			err := browser.NavigateTo("/admin/groups")
			require.NoError(t, err)
			time.Sleep(2 * time.Second)

			// Check component exists
			guruElement := browser.Page.Locator("#guru-meditation")
			elementCount, _ := guruElement.Count()
			assert.Equal(t, 1, elementCount, "Guru Meditation component should exist")

			// Check functions exist
			hasShowFunc, _ := browser.Page.Evaluate(`() => typeof showGuruMeditation === 'function'`)
			assert.True(t, hasShowFunc.(bool), "showGuruMeditation function should exist")

			hasDismissFunc, _ := browser.Page.Evaluate(`() => typeof dismissGuruMeditation === 'function'`)
			assert.True(t, hasDismissFunc.(bool), "dismissGuruMeditation function should exist")
		})

		t.Run("Guru Meditation triggers on error", func(t *testing.T) {
			// Manually trigger an error
			browser.Page.Evaluate(`() => {
				if (typeof showGuruMeditation === 'function') {
					showGuruMeditation('TEST.ERROR', 'Test error message');
				}
			}`)
			time.Sleep(1 * time.Second)

			// Check it's visible
			guruVisible, _ := browser.Page.Locator("#guru-meditation").IsVisible()
			assert.True(t, guruVisible, "Guru Meditation should be visible after error")

			// Dismiss it
			browser.Page.Evaluate(`() => {
				if (typeof dismissGuruMeditation === 'function') {
					dismissGuruMeditation();
				}
			}`)
			time.Sleep(500 * time.Millisecond)

			// Check it's hidden
			guruVisible, _ = browser.Page.Locator("#guru-meditation").IsVisible()
			assert.False(t, guruVisible, "Guru Meditation should be hidden after dismiss")
		})
	})

	t.Run("Final Quality Report", func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()

		t.Logf("=== QUALITY REPORT ===")
		t.Logf("Console Errors: %d", len(consoleErrors))
		for i, err := range consoleErrors {
			if i < 5 { // Show first 5 errors
				t.Logf("  - %s", err)
			}
		}

		t.Logf("Network Errors (5xx): %d", len(networkErrors))
		for i, err := range networkErrors {
			if i < 5 { // Show first 5 errors
				t.Logf("  - %s", err)
			}
		}

		// Quality assertions
		assert.LessOrEqual(t, len(consoleErrors), 0, "Should have no console errors")
		assert.LessOrEqual(t, len(networkErrors), 0, "Should have no server errors")

		if len(consoleErrors) == 0 && len(networkErrors) == 0 {
			t.Log("✓ EXCELLENT: No errors detected during testing")
		}
	})
}

// TestConcurrentAccess tests the system under concurrent load
func TestConcurrentAccess(t *testing.T) {
	// Create multiple browser instances
	numBrowsers := 3
	var wg sync.WaitGroup
	errors := make(chan error, numBrowsers)

	for i := 0; i < numBrowsers; i++ {
		wg.Add(1)
		go func(browserNum int) {
			defer wg.Done()

			browser := helpers.NewBrowserHelper(t)
			err := browser.Setup()
			if err != nil {
				errors <- fmt.Errorf("browser %d setup failed: %v", browserNum, err)
				return
			}
			defer browser.TearDown()

			auth := helpers.NewAuthHelper(browser)
			err = auth.LoginAsAdmin()
			if err != nil {
				errors <- fmt.Errorf("browser %d login failed: %v", browserNum, err)
				return
			}

			// Create a group
			groupName := fmt.Sprintf("Concurrent_%d_%d", browserNum, time.Now().Unix())

			err = browser.NavigateTo("/admin/groups")
			if err != nil {
				errors <- fmt.Errorf("browser %d navigation failed: %v", browserNum, err)
				return
			}
			time.Sleep(2 * time.Second)

			// Open modal
			err = browser.Page.Click("button:has-text('Add Group')")
			if err != nil {
				errors <- fmt.Errorf("browser %d open modal failed: %v", browserNum, err)
				return
			}
			time.Sleep(1 * time.Second)

			// Fill and submit
			browser.Page.Fill("input#groupName", groupName)
			browser.Page.Fill("textarea#groupComments", fmt.Sprintf("Concurrent test from browser %d", browserNum))
			browser.Page.Click("button[type='submit']:has-text('Save')")
			time.Sleep(3 * time.Second)

			// Verify creation
			groupVisible, _ := browser.Page.Locator(fmt.Sprintf("tr:has-text('%s')", groupName)).IsVisible()
			if !groupVisible {
				errors <- fmt.Errorf("browser %d: group not created", browserNum)
				return
			}

			t.Logf("Browser %d: Successfully created group %s", browserNum, groupName)
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var allErrors []error
	for err := range errors {
		allErrors = append(allErrors, err)
	}

	assert.Empty(t, allErrors, "Concurrent access should not produce errors")
	if len(allErrors) == 0 {
		t.Logf("✓ SUCCESS: All %d concurrent browsers completed successfully", numBrowsers)
	}
}

// TestNegativeScenarios focuses on error conditions
func TestNegativeScenarios(t *testing.T) {
	browser := helpers.NewBrowserHelper(t)
	err := browser.Setup()
	require.NoError(t, err)
	defer browser.TearDown()

	auth := helpers.NewAuthHelper(browser)

	t.Run("Access without authentication", func(t *testing.T) {
		// Try to access protected page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Should redirect to login
		url := browser.Page.URL()
		assert.Contains(t, url, "/login", "Should redirect to login when not authenticated")
	})

	t.Run("Invalid URL handling", func(t *testing.T) {
		err := auth.LoginAsAdmin()
		require.NoError(t, err)

		// Try non-existent page
		err = browser.NavigateTo("/admin/nonexistent")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Should show 404 or redirect
		pageContent, _ := browser.Page.Content()
		has404 := strings.Contains(pageContent, "404") || strings.Contains(pageContent, "not found")
		assert.True(t, has404, "Should handle non-existent pages properly")
	})

	t.Run("Session timeout simulation", func(t *testing.T) {
		// Clear cookies to simulate session timeout
		browser.Page.Context().ClearCookies()

		// Try to access protected page
		err := browser.NavigateTo("/admin/groups")
		require.NoError(t, err)
		time.Sleep(2 * time.Second)

		// Should redirect to login
		url := browser.Page.URL()
		assert.Contains(t, url, "/login", "Should redirect to login after session timeout")
	})
}
