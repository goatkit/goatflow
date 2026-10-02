package config

import (
	"os"
	"strings"
)

// CustomerFEOnly reports whether this instance serves only the customer
// portal (CUSTOMER_FE_ONLY). Every reader of the flag must use this so that
// the guard, redirects and scheduler agree: "1", "true", "yes" and "on"
// (any case, surrounding spaces ignored) enable it; anything else disables it.
func CustomerFEOnly() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CUSTOMER_FE_ONLY"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
