package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/i18n"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/platform/webhook"
	"github.com/goatkit/goatflow/internal/webhooks"
)

// pongo2HTMLEscape mirrors pongo2's autoescape so expected translations can be
// matched against rendered HTML.
var pongo2HTMLEscape = strings.NewReplacer("&", "&amp;", ">", "&gt;", "<", "&lt;", "\"", "&quot;", "'", "&#39;")

// webhookPageKeys returns every i18n key under admin.webhooks in en.json.
func webhookPageKeys(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../platform/i18n/translations/en.json")
	require.NoError(t, err)
	var en map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &en))
	admin, ok := en["admin"].(map[string]interface{})
	require.True(t, ok)
	section, ok := admin["webhooks"].(map[string]interface{})
	require.True(t, ok, "en.json must define admin.webhooks")

	var keys []string
	var walk func(prefix string, m map[string]interface{})
	walk = func(prefix string, m map[string]interface{}) {
		for k, v := range m {
			if sub, isMap := v.(map[string]interface{}); isMap {
				walk(prefix+k+".", sub)
				continue
			}
			keys = append(keys, prefix+k)
		}
	}
	walk("admin.webhooks.", section)
	sort.Strings(keys)
	return keys
}

func TestAdminWebhooksPageRenders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	renderer, err := shared.NewTemplateRenderer("../../templates")
	require.NoError(t, err)
	previous := shared.GetGlobalRenderer()
	shared.SetGlobalRenderer(renderer)
	t.Cleanup(func() { shared.SetGlobalRenderer(previous) })

	r := gin.New()
	r.Use(middleware.NewI18nMiddleware().Handle())
	r.GET("/admin/webhooks", handleAdminWebhooks)

	keys := webhookPageKeys(t)
	// Every subscribable event needs a translated description on the page.
	for _, e := range webhooks.Events {
		assert.Contains(t, keys, webhookEventLabelKey(e.Event), "event %s has no admin.webhooks.events description", e.Event)
	}

	tr := i18n.GetInstance()
	for _, lang := range []string{"en", "de", "ar"} {
		t.Run(lang, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/webhooks?lang="+lang, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()

			for _, id := range []string{
				`id="wh-add"`, `id="wh-rows"`, `id="wh-form"`, `id="wh-events"`, `id="wh-headers"`,
				`id="wh-secret"`, `id="wh-remove-secret"`, `id="wh-deliveries-modal"`,
				`id="wh-delivery-modal"`, `id="wh-redeliver"`, `id="wh-i18n"`,
			} {
				assert.Contains(t, body, id)
			}
			assert.Contains(t, body, fmt.Sprintf(`id="wh-retry" name="retry_count" required min="0" max="%d"`, webhook.MaxRetryCount))
			assert.Contains(t, body, fmt.Sprintf(`id="wh-timeout" name="timeout_seconds" required min="1" max="%d"`, webhook.MaxTimeoutSeconds))
			for _, e := range webhooks.Events {
				assert.Contains(t, body, `data-event="`+e.Event+`"`)
			}

			assert.NotContains(t, body, "admin.webhooks.", "raw i18n key rendered")
			for _, key := range keys {
				v := tr.T(lang, key)
				require.NotEqual(t, key, v, "%s missing in %s", key, lang)
				assert.Contains(t, body, pongo2HTMLEscape.Replace(v), "%s (%s) not rendered", key, lang)
			}
			if lang != "en" {
				assert.NotContains(t, body, ">"+tr.T("en", "admin.webhooks.description")+"<", "English description leaked into %s", lang)
			}
		})
	}
}
