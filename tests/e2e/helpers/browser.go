package helpers

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gotrs-io/gotrs-ce/tests/e2e/config"
	"github.com/playwright-community/playwright-go"
)

// BrowserHelper provides browser setup and teardown for tests
type BrowserHelper struct {
	Playwright *playwright.Playwright
	Browser    playwright.Browser
	Context    playwright.BrowserContext
	Page       playwright.Page
	Config     *config.TestConfig
	t          *testing.T
}

// NewBrowserHelper creates a new browser helper instance
func NewBrowserHelper(t *testing.T) *BrowserHelper {
	return &BrowserHelper{
		Config: config.GetConfig(),
		t:      t,
	}
}

// Setup initializes the browser and creates a new page
func (b *BrowserHelper) Setup() error {
	// Initialize Playwright
	var pw *playwright.Playwright
	var err error
	if os.Getenv("PLAYWRIGHT_PREINSTALLED") != "1" {
		if err = playwright.Install(); err != nil {
			return fmt.Errorf("could not install playwright browsers: %w", err)
		}
	}
	// First attempt
	pw, err = playwright.Run()
	if err != nil {
		// Fallback: attempt install driver explicitly then retry
		_ = playwright.Install()
		pw, err = playwright.Run()
		if err != nil {
			return fmt.Errorf("could not start playwright after retry (ensure driver version matches image): %w", err)
		}
	}
	b.Playwright = pw

	// Launch browser
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(b.Config.Headless),
		SlowMo:   playwright.Float(float64(b.Config.SlowMo)),
	})
	if err != nil {
		return fmt.Errorf("could not launch browser: %w", err)
	}
	b.Browser = browser

	// Create context with viewport and other settings
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  1280,
			Height: 720,
		},
		RecordVideo: &playwright.RecordVideo{
			Dir: "./test-results/videos",
		},
	})
	if err != nil {
		return fmt.Errorf("could not create context: %w", err)
	}
	b.Context = context

	// Create page
	page, err := context.NewPage()
	if err != nil {
		return fmt.Errorf("could not create page: %w", err)
	}
	b.Page = page

	// Set default timeout
	page.SetDefaultTimeout(float64(b.Config.Timeout.Milliseconds()))

	return nil
}

// TearDown closes the browser and cleans up resources
func (b *BrowserHelper) TearDown() {
	// Take screenshot on failure
	if b.t.Failed() && b.Config.Screenshots && b.Page != nil {
		screenshotPath := fmt.Sprintf("./test-results/screenshots/%s_%d.png",
			b.t.Name(), time.Now().Unix())
		b.Page.Screenshot(playwright.PageScreenshotOptions{
			Path: playwright.String(screenshotPath),
		})
	}

	// Close resources
	if b.Page != nil {
		b.Page.Close()
	}
	if b.Context != nil {
		b.Context.Close()
	}
	if b.Browser != nil {
		b.Browser.Close()
	}
	if b.Playwright != nil {
		b.Playwright.Stop()
	}
}

// NavigateTo navigates to a path relative to the base URL
func (b *BrowserHelper) NavigateTo(path string) error {
	url := b.Config.BaseURL + path
	_, err := b.Page.Goto(url)
	if err != nil && strings.Contains(err.Error(), "ERR_TOO_MANY_REDIRECTS") {
		return fmt.Errorf("redirect loop navigating to %s (check BASE_URL port / login redirect configuration): %w", url, err)
	}
	return err
}

// WaitForHTMX waits for HTMX requests to complete
func (b *BrowserHelper) WaitForHTMX() error {
	// Wait for htmx:afterRequest event or network idle
	return b.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
}

// WaitForLoad waits for the main page load event.
func (b *BrowserHelper) WaitForLoad() error {
	return b.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateLoad,
	})
}
