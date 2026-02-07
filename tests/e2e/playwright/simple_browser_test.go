//go:build e2e

package playwright

import (
	"os"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/config"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"
)

func TestSimpleBrowser(t *testing.T) {
	if os.Getenv("SKIP_BROWSER") == "true" {
		t.Skip("Skipping browser test")
	}
	cfg := config.GetConfig()
	baseURL := cfg.BaseURL
	_ = playwright.Install(&playwright.RunOptions{Browsers: []string{"chromium"}})
	pw, err := playwright.Run()
	if err != nil {
		t.Skipf("Could not start Playwright: %v", err)
		return
	}
	defer pw.Stop()
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})
	require.NoError(t, err)
	defer browser.Close()
	page, err := browser.NewPage()
	require.NoError(t, err)
	defer page.Close()
	_, err = page.Goto(baseURL + "/login")
	require.NoError(t, err)
	_, _ = page.Screenshot(playwright.PageScreenshotOptions{Path: playwright.String("/tmp/login-page.png")})
	cnt, err := page.Locator("input#email").Count()
	require.NoError(t, err)
	require.Greater(t, cnt, 0)
	_ = page.Fill("input#email", os.Getenv("DEMO_ADMIN_EMAIL"))
	_ = page.Fill("input#password", os.Getenv("DEMO_ADMIN_PASSWORD"))
	_ = page.Click("button[type='submit']")
	time.Sleep(2 * time.Second)
	if page.URL() == baseURL+"/dashboard" {
		t.Log("Logged in")
	}
}
