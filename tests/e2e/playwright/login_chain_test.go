//go:build e2e

package playwright

import (
	"net/http"
	"testing"
	"time"

	"github.com/goatkit/goatflow/tests/e2e/config"
)

func TestLoginRedirectChain(t *testing.T) {
	cfg := config.GetConfig()
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		return nil
	}, Timeout: 10 * time.Second}

	// Hit /login directly
	resp, err := client.Get(cfg.BaseURL + "/login")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("Initial /login status=%d location=%s", resp.StatusCode, resp.Header.Get("Location"))

	// Follow up to 8 redirects manually
	url := cfg.BaseURL + "/login"
	for i := 0; i < 8; i++ {
		if loc := resp.Header.Get("Location"); loc != "" {
			url = loc
		} else {
			break
		}
		if !startsWithHTTP(url) {
			url = cfg.BaseURL + url
		}
		resp, err = client.Get(url)
		if err != nil {
			t.Fatalf("redirect %d failed: %v", i+1, err)
		}
		t.Logf("Redirect %d -> status=%d location=%s", i+1, resp.StatusCode, resp.Header.Get("Location"))
		if resp.Header.Get("Location") == "" {
			break
		}
	}
}

func startsWithHTTP(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || (len(s) > 8 && s[:8] == "https://"))
}
