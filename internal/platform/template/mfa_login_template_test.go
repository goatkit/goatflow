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

// A passkey-only account sees "Other ways to sign in": a recovery-code form
// when it has codes, otherwise a pointer to an administrator. No password-only path.
func TestLogin2FATemplateOtherWaysToSignIn(t *testing.T) {
	t.Parallel()
	helper := NewTemplateTestHelper(t)
	for _, tmpl := range []string{"pages/login_2fa.pongo2", "pages/customer/login_2fa.pongo2"} {
		for _, withCodes := range []bool{true, false} {
			ctx := baseContext()
			ctx["show_totp_form"] = false
			ctx["totp_enabled"] = false
			ctx["webauthn_enabled"] = true
			ctx["security_key_only"] = true
			ctx["show_recovery_form"] = withCodes
			ctx["t"] = func(key string, args ...interface{}) string { return key }

			html := helper.RenderAndValidate(t, tmpl, ctx)

			assert.Contains(t, html, `id="other-sign-in-ways"`, tmpl)
			assert.Contains(t, html, "auth.other_ways_to_sign_in", tmpl)
			if withCodes {
				assert.Contains(t, html, `<form id="2fa-form"`, tmpl)
				assert.Contains(t, html, "auth.recovery_code_hint", tmpl)
				assert.NotContains(t, html, `inputmode="numeric"`, tmpl)
				assert.NotContains(t, html, "auth.no_other_sign_in_ways", tmpl)
			} else {
				assert.NotContains(t, html, `<form id="2fa-form"`, tmpl)
				assert.Contains(t, html, "auth.no_other_sign_in_ways", tmpl)
			}
		}
	}
}

func TestLogin2FATemplateAuthenticatorFormAcceptsRecoveryCodes(t *testing.T) {
	t.Parallel()
	helper := NewTemplateTestHelper(t)
	for _, tmpl := range []string{"pages/login_2fa.pongo2", "pages/customer/login_2fa.pongo2"} {
		ctx := baseContext()
		ctx["show_totp_form"] = true
		ctx["totp_enabled"] = true
		ctx["t"] = func(key string, args ...interface{}) string { return key }

		html := helper.RenderAndValidate(t, tmpl, ctx)

		assert.Contains(t, html, `<form id="2fa-form"`, tmpl)
		// A digits-only pattern blocks submitting a letter-and-digit recovery code.
		assert.NotContains(t, html, `pattern="[0-9]*"`, tmpl)
		assert.NotContains(t, html, `id="other-sign-in-ways"`, tmpl)
	}
}
