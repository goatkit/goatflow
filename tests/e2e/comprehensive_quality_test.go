//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/helpers"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// browserErrors collects console errors, uncaught page errors and 5xx
// responses seen by a page.
type browserErrors struct {
	mu                    sync.Mutex
	console, page, server []string
}

func watchBrowserErrors(page playwright.Page) *browserErrors {
	e := &browserErrors{}
	page.OnConsole(func(msg playwright.ConsoleMessage) {
		if msg.Type() == "error" {
			e.mu.Lock()
			e.console = append(e.console, msg.Text())
			e.mu.Unlock()
		}
	})
	page.OnPageError(func(err error) {
		e.mu.Lock()
		e.page = append(e.page, err.Error())
		e.mu.Unlock()
	})
	page.OnResponse(func(r playwright.Response) {
		if r.Status() >= 500 {
			e.mu.Lock()
			e.server = append(e.server, fmt.Sprintf("%d %s %s", r.Status(), r.Request().Method(), r.URL()))
			e.mu.Unlock()
		}
	})
	return e
}

func (e *browserErrors) reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.console, e.page, e.server = nil, nil, nil
}

func (e *browserErrors) assertNone(t *testing.T) {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	assert.Empty(t, e.console, "console errors")
	assert.Empty(t, e.page, "uncaught page errors")
	assert.Empty(t, e.server, "5xx responses")
}

// TestComprehensiveQuality drives the login and group management with hostile
// input and checks that nothing executes, leaks or errors.
func TestComprehensiveQuality(t *testing.T) {
	b := newGroupsBrowser(t)
	expect := groupsExpect()
	errs := watchBrowserErrors(b.Page)
	top := t

	t.Run("Login rejects SQL injection", func(t *testing.T) {
		require.NoError(t, b.NavigateTo("/login"))
		require.NoError(t, b.Page.Locator("input#email").Fill("' OR '1'='1' -- "))
		require.NoError(t, b.Page.Locator("input#password").Fill("' OR '1'='1"))
		resp := expectResponse(t, b, http.MethodPost, regexp.MustCompile(`/api/auth/login$`), func() error {
			return b.Page.Locator("form[action='/api/auth/login'] button[type='submit']").Click()
		})
		assert.Equal(t, http.StatusUnauthorized, resp.Status())
		assert.NoError(t, expect.Locator(b.Page.Locator("#error-message")).ToContainText("Invalid username or password"))
		assert.NoError(t, expect.Page(b.Page).ToHaveURL(regexp.MustCompile(`/login$`)))

		cookies, err := b.Context.Cookies()
		require.NoError(t, err)
		for _, c := range cookies {
			assert.NotContains(t, []string{"auth_token", "access_token"}, c.Name, "no session after a failed login")
		}
	})

	require.NoError(t, helpers.NewAuthHelper(b).LoginAsAdmin(), "admin login")
	errs.reset()

	t.Run("Group name and description are rendered as text", func(t *testing.T) {
		// The name breaks out of a quoted JS string and both fields carry HTML.
		name := fmt.Sprintf("E2EXss_%d');window.__xss='name';('<img src=x onerror=window.__xss='img'>", time.Now().UnixNano())
		comments := `<script>window.__xss='script'</script><img src=x onerror="window.__xss='comment'">`
		row := groupRow(b, name)

		openGroupsPage(t, b)
		require.NoError(t, b.Page.Locator("button:has-text('Add Group')").Click())
		modal := b.Page.Locator("#groupModal")
		require.NoError(t, expect.Locator(modal).ToBeVisible())
		require.NoError(t, b.Page.Locator("#groupName").Fill(name))
		require.NoError(t, b.Page.Locator("#groupComments").Fill(comments))
		resp := submitGroupRequest(t, b, http.MethodPost, 0, func() error {
			return modal.Locator("button[type='submit']").Click()
		})
		require.Equal(t, http.StatusCreated, resp.Status())

		require.NoError(t, expect.Locator(row).ToBeVisible())
		id := rowGroupID(t, row)
		cleanupGroupAtEnd(top, b, id)
		assert.NoError(t, expect.Locator(row).ToContainText(name))
		assert.NoError(t, expect.Locator(row).ToContainText(comments))
		assert.NoError(t, expect.Locator(row.Locator("img, script")).ToHaveCount(0))

		// Delete passes the name to the confirmation and the toast.
		require.NoError(t, row.Locator("button[title='Delete group']").Click())
		deleteModal := b.Page.Locator("#deleteModal")
		require.NoError(t, expect.Locator(deleteModal).ToBeVisible())
		resp = submitGroupRequest(t, b, http.MethodDelete, id, func() error {
			return deleteModal.Locator("button:has-text('Delete Group')").Click()
		})
		require.Equal(t, http.StatusOK, resp.Status())
		toast := b.Page.Locator("#toast-container")
		assert.NoError(t, expect.Locator(toast).ToContainText(name))
		assert.NoError(t, expect.Locator(toast.Locator("img")).ToHaveCount(0))

		injected, err := b.Page.Evaluate("() => window.__xss === undefined ? '' : String(window.__xss)")
		require.NoError(t, err)
		assert.Equal(t, "", injected, "injected script ran")
	})

	t.Run("No browser or server errors while managing groups", func(t *testing.T) {
		errs.assertNone(t)
	})

	t.Run("Groups page is ready within 3 seconds", func(t *testing.T) {
		openGroupsPage(t, b)
		ready, err := b.Page.Evaluate("() => performance.getEntriesByType('navigation')[0].domContentLoadedEventEnd")
		require.NoError(t, err)
		var ms float64
		switch v := ready.(type) {
		case int:
			ms = float64(v)
		case float64:
			ms = v
		default:
			t.Fatalf("domContentLoadedEventEnd = %v", ready)
		}
		assert.Positive(t, ms)
		assert.Less(t, ms, 3000.0, "DOMContentLoaded after %.0f ms", ms)
	})

	t.Run("Failure to load a group shows Guru Meditation", func(t *testing.T) {
		name := uniqueGroupName("E2EGuru")
		id := createGroupViaAPI(top, b, name, "guru meditation e2e group", 1)
		openGroupsPage(t, b)

		endpoint := groupEndpoint(id)
		require.NoError(t, b.Page.Route(endpoint, func(route playwright.Route) {
			if route.Request().Method() == http.MethodGet {
				_ = route.Abort()
				return
			}
			_ = route.Continue()
		}))
		t.Cleanup(func() { _ = b.Page.Unroute(endpoint) })

		require.NoError(t, groupRow(b, name).Locator("button[title='Edit group']").Click())
		guru := b.Page.Locator("#guru-meditation")
		require.NoError(t, expect.Locator(guru).ToBeVisible())
		assert.NoError(t, expect.Locator(b.Page.Locator("#guru-code")).ToHaveText("00000005.GRPLOAD0"))
		assert.NoError(t, expect.Locator(b.Page.Locator("#guru-message")).ToContainText("UNABLE TO FETCH GROUP DETAILS"))
		assert.NoError(t, expect.Locator(b.Page.Locator("#guru-location")).ToHaveText(fmt.Sprintf("/admin/groups/%d", id)))
		assert.NoError(t, expect.Locator(b.Page.Locator("#groupModal")).ToBeHidden())
	})
}

// TestConcurrentAccess has three admins create groups at the same moment;
// every create succeeds and every group is listed afterwards.
func TestConcurrentAccess(t *testing.T) {
	const admins = 3
	expect := groupsExpect()
	browsers := make([]*helpers.BrowserHelper, admins)
	names := make([]string, admins)
	for i := range browsers {
		b := newGroupsAdminBrowser(t)
		browsers[i] = b
		names[i] = uniqueGroupName(fmt.Sprintf("E2EConcurrent%d", i))
		openGroupsPage(t, b)
		require.NoError(t, b.Page.Locator("button:has-text('Add Group')").Click())
		require.NoError(t, expect.Locator(b.Page.Locator("#groupModal")).ToBeVisible())
		require.NoError(t, b.Page.Locator("#groupName").Fill(names[i]))
		require.NoError(t, b.Page.Locator("#groupComments").Fill(fmt.Sprintf("concurrent create %d", i)))
	}

	create := groupEndpoint(0)
	statuses := make([]int, admins)
	failures := make([]error, admins)
	var wg sync.WaitGroup
	for i, b := range browsers {
		wg.Add(1)
		go func(i int, b *helpers.BrowserHelper) {
			defer wg.Done()
			// A successful create reloads the page; wait for that reload so
			// the later navigation does not interrupt it.
			_, err := b.Page.ExpectEvent("load", func() error {
				ev, err := b.Page.ExpectEvent("response", func() error {
					return b.Page.Locator("#groupModal button[type='submit']").Click()
				}, playwright.PageExpectEventOptions{Predicate: func(r playwright.Response) bool {
					return r.Request().Method() == http.MethodPost && create.MatchString(r.URL())
				}})
				if err != nil {
					return err
				}
				statuses[i] = ev.(playwright.Response).Status()
				return nil
			})
			failures[i] = err
		}(i, b)
	}
	wg.Wait()

	for i := range browsers {
		assert.Equal(t, http.StatusCreated, statuses[i], "admin %d", i)
		require.NoError(t, failures[i], "admin %d", i)
	}
	first := browsers[0]
	openGroupsPage(t, first)
	for _, name := range names {
		row := groupRow(first, name)
		require.NoError(t, expect.Locator(row).ToBeVisible(), name)
		cleanupGroupAtEnd(t, first, rowGroupID(t, row))
	}
}

// TestNegativeScenarios covers access the server must refuse.
func TestNegativeScenarios(t *testing.T) {
	b := newGroupsBrowser(t)
	expect := groupsExpect()

	t.Run("Access without authentication redirects to login", func(t *testing.T) {
		require.NoError(t, b.NavigateTo("/admin/groups"))
		assert.NoError(t, expect.Page(b.Page).ToHaveURL(regexp.MustCompile(`/login$`)))
		assert.NoError(t, expect.Locator(b.Page.Locator("input#email")).ToBeVisible())
	})

	t.Run("Unknown admin page returns 404", func(t *testing.T) {
		require.NoError(t, helpers.NewAuthHelper(b).LoginAsAdmin())
		resp, err := b.Page.Goto(b.Config.BaseURL + "/admin/nonexistent")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.Status())
		body, err := b.Page.Locator("body").InnerText()
		require.NoError(t, err)
		assert.Contains(t, strings.ToLower(body), "not found")
	})
}
