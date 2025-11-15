package config

import "strings"

// EffectiveTLSMode normalizes the TLS mode for outbound SMTP connections.
// Supported values: "none", "starttls", "smtps" (implicit TLS).
// If TLSMode is empty we fall back to the legacy TLS boolean flag.
func (c *EmailConfig) EffectiveTLSMode() string {
	if c == nil {
		return "none"
	}
	mode := strings.ToLower(strings.TrimSpace(c.SMTP.TLSMode))
	switch mode {
	case "", "auto":
		if c.SMTP.TLS {
			return "starttls"
		}
		return "none"
	case "starttls", "tls":
		return "starttls"
	case "smtps", "implicit", "tls_implicit":
		return "smtps"
	case "none", "off", "disabled":
		return "none"
	default:
		// Unknown value; fall back to boolean for backward compatibility
		if c.SMTP.TLS {
			return "starttls"
		}
		return "none"
	}
}
