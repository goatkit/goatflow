//go:build e2e

package e2e

import (
	"encoding/json"
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

// The profile tests sign in as the dedicated non-admin seeded agent (not the
// shared admin) and restore its profile and preferences afterwards, so other
// suites running against the same stack never see a renamed or German admin.

// agentProfile is the editable state behind /profile.
type agentProfile struct {
	FirstName      string
	LastName       string
	Title          string
	Language       string
	SessionTimeout int
}

// TestProfilePreferencesPage covers the agent profile page (/profile): one
// form that saves personal details, language and session timeout.
func TestProfilePreferencesPage(t *testing.T) {
	browser, page := loginProfileAgent(t)
	original := readAgentProfile(t, page)
	t.Cleanup(func() { restoreAgentProfile(t, page, original) })

	openProfile := func(t *testing.T) {
		t.Helper()
		require.NoError(t, browser.NavigateTo("/profile"))
		// Language options and the 2FA status load asynchronously; network
		// idle also means the form has recorded its original values.
		require.NoError(t, page.Locator("#language-select option[value='de']").WaitFor(
			playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateAttached}))
		require.NoError(t, page.Locator("[id='2fa-status-loading']").WaitFor(
			playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateHidden}))
		require.NoError(t, page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{State: playwright.LoadStateNetworkidle}))
	}

	t.Run("Profile page shows the signed-in agent", func(t *testing.T) {
		openProfile(t)
		title, err := page.Title()
		require.NoError(t, err)
		assert.Contains(t, title, "Profile")
		assert.Equal(t, original.FirstName, inputValue(t, page, "#first-name"))
		assert.Equal(t, original.LastName, inputValue(t, page, "#last-name"))
		login, err := page.Locator("#profile-form dl dd").First().TextContent()
		require.NoError(t, err)
		assert.Equal(t, helpers.Seed2FAAgentLogin, strings.TrimSpace(login))
	})

	t.Run("Language dropdown lists every supported language", func(t *testing.T) {
		openProfile(t)
		var langs struct {
			Available []struct {
				Code string `json:"code"`
			} `json:"available"`
		}
		profileAPI(t, page, "GET", "/agent/api/preferences/language", nil, 200, &langs)
		require.NotEmpty(t, langs.Available)

		values, err := page.Locator("#language-select option").EvaluateAll(`opts => opts.map(o => o.value)`)
		require.NoError(t, err)
		want := []interface{}{""}
		for _, l := range langs.Available {
			want = append(want, l.Code)
		}
		assert.Equal(t, want, values, "dropdown = System Default + every available language")
		assert.Equal(t, original.Language, inputValue(t, page, "#language-select"))
	})

	t.Run("No JavaScript console errors on profile page", func(t *testing.T) {
		var (
			mu            sync.Mutex
			collect       = true
			consoleErrors []string
		)
		page.OnConsole(func(msg playwright.ConsoleMessage) {
			mu.Lock()
			defer mu.Unlock()
			if collect && msg.Type() == "error" {
				consoleErrors = append(consoleErrors, msg.Text())
			}
		})

		openProfile(t)
		mu.Lock()
		defer mu.Unlock()
		collect = false
		assert.Empty(t, consoleErrors, "profile page logged console errors")
	})

	t.Run("Saving personal details persists them", func(t *testing.T) {
		openProfile(t)
		suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
		first, last, title := "Profile"+suffix, "Tester"+suffix, "Dr."
		require.NoError(t, page.Locator("#first-name").Fill(first))
		require.NoError(t, page.Locator("#last-name").Fill(last))
		require.NoError(t, page.Locator("#title").Fill(title))

		resp := submitProfileForm(t, page, "/agent/api/profile")
		assert.Equal(t, 200, resp.Status())
		// Save and Close returns to the dashboard when the language is unchanged.
		require.NoError(t, page.WaitForURL("**/dashboard"))

		saved := readAgentProfile(t, page)
		assert.Equal(t, first, saved.FirstName)
		assert.Equal(t, last, saved.LastName)
		assert.Equal(t, title, saved.Title)

		openProfile(t)
		assert.Equal(t, first, inputValue(t, page, "#first-name"))
		assert.Equal(t, last, inputValue(t, page, "#last-name"))
		assert.Equal(t, title, inputValue(t, page, "#title"))
	})

	t.Run("Saving a language switches the interface language", func(t *testing.T) {
		openProfile(t)
		lang := "de"
		if original.Language == "de" {
			lang = "fr"
		}
		_, err := page.Locator("#language-select").SelectOption(playwright.SelectOptionValues{Values: &[]string{lang}})
		require.NoError(t, err)

		resp := submitProfileForm(t, page, "/agent/api/preferences/language")
		assert.Equal(t, 200, resp.Status())
		// A language change reloads the profile page in the new language.
		require.NoError(t, page.Locator(fmt.Sprintf("html[lang='%s']", lang)).WaitFor(
			playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateAttached}))
		assert.Equal(t, lang, readAgentProfile(t, page).Language)
	})

	t.Run("Saving the session timeout persists it", func(t *testing.T) {
		openProfile(t)
		timeout := "3600"
		if original.SessionTimeout == 3600 {
			timeout = "14400"
		}
		_, err := page.Locator("#session-timeout").SelectOption(playwright.SelectOptionValues{Values: &[]string{timeout}})
		require.NoError(t, err)

		resp := submitProfileForm(t, page, "/agent/api/preferences/session-timeout")
		assert.Equal(t, 200, resp.Status())
		require.NoError(t, page.WaitForURL("**/dashboard"))
		assert.Equal(t, timeout, fmt.Sprint(readAgentProfile(t, page).SessionTimeout))
	})
}

// TestProfileAPIEndpoints exercises the JSON endpoints behind the profile page.
func TestProfileAPIEndpoints(t *testing.T) {
	_, page := loginProfileAgent(t)
	original := readAgentProfile(t, page)
	t.Cleanup(func() { restoreAgentProfile(t, page, original) })

	t.Run("GET /agent/api/preferences/language lists the built-in languages", func(t *testing.T) {
		var got struct {
			Success   bool `json:"success"`
			Available []struct {
				Code       string `json:"code"`
				NativeName string `json:"native_name"`
			} `json:"available"`
		}
		profileAPI(t, page, "GET", "/agent/api/preferences/language", nil, 200, &got)
		assert.True(t, got.Success)
		codes := map[string]string{}
		for _, l := range got.Available {
			codes[l.Code] = l.NativeName
		}
		for _, code := range []string{"ar", "de", "en", "es", "fa", "fr", "he", "ja", "pl", "pt", "ru", "tlh", "uk", "ur", "zh"} {
			assert.Contains(t, codes, code)
		}
		assert.Equal(t, "Deutsch", codes["de"])
	})

	t.Run("POST /agent/api/preferences/language rejects unknown languages", func(t *testing.T) {
		var got struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		profileAPI(t, page, "POST", "/agent/api/preferences/language", map[string]any{"value": "xx"}, 400, &got)
		assert.False(t, got.Success)
		assert.Contains(t, got.Error, "Unsupported language")
		assert.Equal(t, original.Language, readAgentProfile(t, page).Language)
	})

	t.Run("Session timeout round-trips", func(t *testing.T) {
		want := 28800
		if original.SessionTimeout == want {
			want = 86400
		}
		profileAPI(t, page, "POST", "/agent/api/preferences/session-timeout", map[string]any{"value": want}, 200, nil)
		assert.Equal(t, want, readAgentProfile(t, page).SessionTimeout)
	})

	t.Run("GET /agent/api/profile returns the signed-in agent", func(t *testing.T) {
		var got struct {
			Success bool `json:"success"`
			Profile struct {
				Login string `json:"login"`
			} `json:"profile"`
		}
		profileAPI(t, page, "GET", "/agent/api/profile", nil, 200, &got)
		assert.True(t, got.Success)
		assert.Equal(t, helpers.Seed2FAAgentLogin, got.Profile.Login)
	})

	t.Run("POST /agent/api/profile saves and validates", func(t *testing.T) {
		profileAPI(t, page, "POST", "/agent/api/profile",
			map[string]any{"first_name": "Api", "last_name": "Agent", "title": "Ms."}, 200, nil)
		saved := readAgentProfile(t, page)
		assert.Equal(t, "Api", saved.FirstName)
		assert.Equal(t, "Agent", saved.LastName)
		assert.Equal(t, "Ms.", saved.Title)

		var got struct {
			Error string `json:"error"`
		}
		profileAPI(t, page, "POST", "/agent/api/profile",
			map[string]any{"first_name": "", "last_name": "Agent"}, 400, &got)
		assert.Contains(t, got.Error, "First name and last name are required")
		assert.Equal(t, "Api", readAgentProfile(t, page).FirstName, "rejected update must not change the profile")
	})
}

// loginProfileAgent opens a browser signed in as the seeded non-admin agent.
func loginProfileAgent(t *testing.T) (*helpers.BrowserHelper, playwright.Page) {
	t.Helper()
	browser := helpers.NewBrowserHelper(t)
	require.NoError(t, browser.Setup(), "Failed to setup browser")
	t.Cleanup(browser.TearDown)
	require.NoError(t, helpers.NewAuthHelper(browser).Login(helpers.Seed2FAAgentLogin, helpers.Seed2FAAgentPassword))
	require.NoError(t, browser.Page.WaitForURL("**/dashboard"), "seeded agent %q must sign in without 2FA", helpers.Seed2FAAgentLogin)
	return browser, browser.Page
}

// profileAPI calls a same-origin JSON endpoint with the page's session,
// asserts the status and decodes the body into out (when non-nil).
func profileAPI(t *testing.T, page playwright.Page, method, path string, body any, wantStatus int, out any) {
	t.Helper()
	res, err := page.Evaluate(`async ([method, path, body]) => {
		const opts = { method, credentials: 'same-origin', headers: { 'Accept': 'application/json' } };
		if (body !== null) {
			opts.headers['Content-Type'] = 'application/json';
			opts.body = JSON.stringify(body);
		}
		const r = await fetch(path, opts);
		return { status: r.status, text: await r.text() };
	}`, []any{method, path, body})
	require.NoError(t, err)
	m := res.(map[string]any)
	text := m["text"].(string)
	require.EqualValues(t, wantStatus, m["status"], "%s %s: %s", method, path, text)
	if out != nil {
		require.NoError(t, json.Unmarshal([]byte(text), out), "%s %s: %s", method, path, text)
	}
}

// readAgentProfile reads the editable profile state through the API.
func readAgentProfile(t *testing.T, page playwright.Page) agentProfile {
	t.Helper()
	var profile struct {
		Profile struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Title     string `json:"title"`
		} `json:"profile"`
	}
	profileAPI(t, page, "GET", "/agent/api/profile", nil, 200, &profile)
	var lang struct {
		Value string `json:"value"`
	}
	profileAPI(t, page, "GET", "/agent/api/preferences/language", nil, 200, &lang)
	var timeout struct {
		Value int `json:"value"`
	}
	profileAPI(t, page, "GET", "/agent/api/preferences/session-timeout", nil, 200, &timeout)
	return agentProfile{
		FirstName:      profile.Profile.FirstName,
		LastName:       profile.Profile.LastName,
		Title:          profile.Profile.Title,
		Language:       lang.Value,
		SessionTimeout: timeout.Value,
	}
}

// restoreAgentProfile writes back a profile captured by readAgentProfile.
func restoreAgentProfile(t *testing.T, page playwright.Page, p agentProfile) {
	t.Helper()
	profileAPI(t, page, "POST", "/agent/api/profile",
		map[string]any{"first_name": p.FirstName, "last_name": p.LastName, "title": p.Title}, 200, nil)
	profileAPI(t, page, "POST", "/agent/api/preferences/language", map[string]any{"value": p.Language}, 200, nil)
	profileAPI(t, page, "POST", "/agent/api/preferences/session-timeout", map[string]any{"value": p.SessionTimeout}, 200, nil)
	require.Equal(t, p, readAgentProfile(t, page), "profile not restored")
}

// submitProfileForm clicks Save and Close and returns the save request's
// response from path (the page is idle, so the next response there is the POST).
func submitProfileForm(t *testing.T, page playwright.Page, path string) playwright.Response {
	t.Helper()
	resp, err := page.ExpectResponse(func(url string) bool {
		return strings.HasSuffix(strings.SplitN(url, "?", 2)[0], path)
	}, func() error { return page.Locator("#save-btn").Click() })
	require.NoError(t, err)
	require.Equal(t, "POST", resp.Request().Method())
	return resp
}

// inputValue returns the current value of an input or select.
func inputValue(t *testing.T, page playwright.Page, selector string) string {
	t.Helper()
	v, err := page.Locator(selector).InputValue()
	require.NoError(t, err)
	return v
}
