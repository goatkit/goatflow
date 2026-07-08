package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogin2FATemplateSecurityKeyOnlyMode(t *testing.T) {
	t.Parallel()
	helper := NewTemplateTestHelper(t)
	tests := []struct {
		name     string
		template string
	}{
		{name: "agent", template: "pages/login_2fa.pongo2"},
		{name: "customer", template: "pages/customer/login_2fa.pongo2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := baseContext()
			ctx["show_totp_form"] = false
			ctx["totp_enabled"] = false
			ctx["webauthn_enabled"] = true
			ctx["security_key_only"] = true
			ctx["t"] = func(key string, args ...interface{}) string {
				if key == "auth.2fa_security_key_only_description" {
					return "Use your security key to finish signing in."
				}
				return key
			}

			html := helper.RenderAndValidate(t, tt.template, ctx)

			assert.Contains(t, html, "Use your security key to finish signing in.")
			assert.Contains(t, html, `id="security-key-btn"`)
			assert.Contains(t, html, "gk-btn-neon")
			assert.NotContains(t, html, `<form id="2fa-form"`)
			assert.NotContains(t, html, `id="code" name="code"`)
		})
	}
 }
