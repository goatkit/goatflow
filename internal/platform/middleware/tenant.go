package middleware

import (
	"os"
	"strconv"
	"strings"
)

// ResolveTenantFromHost maps the request host to a tenant ID using GOATFLOW_CUSTOMER_HOSTMAP.
// Format: "host1=1,host2=2". Unknown hosts return 0.
func ResolveTenantFromHost(host string) uint {
	if host == "" {
		return 0
	}
	host = stripPort(host)
	mapping := os.Getenv("GOATFLOW_CUSTOMER_HOSTMAP")
	if mapping == "" {
		return 0
	}
	pairs := strings.Split(mapping, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			continue
		}
		if stripPort(strings.ToLower(parts[0])) != strings.ToLower(host) {
			continue
		}
		if id, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && id >= 0 {
			return uint(id)
		}
	}
	return 0
}

func stripPort(host string) string {
	if idx := strings.Index(host, ":"); idx >= 0 {
		return host[:idx]
	}
	return host
}
