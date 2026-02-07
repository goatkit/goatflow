//go:build e2e

package playwright

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/goatkit/goatflow/tests/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomerPortalServesHTML(t *testing.T) {
	cfg := config.GetConfig()

	// Customer portal runs in its own container, use CustomerPortalURL
	portalURL := cfg.CustomerPortalURL
	if portalURL == "" {
		portalURL = cfg.BaseURL // fallback
	}
	t.Logf("Testing customer portal at: %s/customer", portalURL)

	resp, err := http.Get(portalURL + "/customer")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	trimmed := strings.TrimSpace(bodyStr)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, strings.ToLower(resp.Header.Get("Content-Type")), "text/html")
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		t.Fatalf("expected HTML response for /customer, got JSON: %s", truncate(bodyStr))
	}
	assert.Contains(t, strings.ToLower(bodyStr), "<html", "customer portal should render html, not json")
}

func truncate(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
