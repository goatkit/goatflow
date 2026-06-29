package pluginui

import (
	"os"
	"strings"
	"testing"
)

func TestPluginUILayoutsRegisterRootServiceWorkerForPWA(t *testing.T) {
	for _, file := range []string{
		"../../../templates/layouts/ui_minimal.pongo2",
		"../../../templates/layouts/ui_none.pongo2",
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(data)
		if !strings.Contains(body, "ui_pwa_enabled") {
			t.Fatalf("%s does not guard service worker registration with ui_pwa_enabled", file)
		}
		if !strings.Contains(body, `partials/service_worker_registration.pongo2`) {
			t.Fatalf("%s does not include shared service worker registration partial", file)
		}
	}
}
