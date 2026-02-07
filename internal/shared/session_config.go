package shared

import (
	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/constants"
)

// GetSystemSessionMaxTime returns the configured session lifetime in seconds.
func GetSystemSessionMaxTime() int {
	ttl := constants.DefaultSessionTimeout
	if cfg := config.Get(); cfg != nil {
		configured := cfg.Auth.Session.SessionMaxTime
		if configured <= 0 {
			configured = cfg.Auth.Session.MaxAge
		}
		if configured > 0 {
			ttl = clampSessionDuration(configured)
		}
	}
	return ttl
}

// GetSystemSessionIdleTime returns the configured idle timeout in seconds.
func GetSystemSessionIdleTime() int {
	idle := constants.DefaultSessionIdleTimeout
	if cfg := config.Get(); cfg != nil {
		configured := cfg.Auth.Session.SessionMaxIdleTime
		switch {
		case configured > 0:
			idle = clampIdleDuration(configured)
		case configured == 0:
			idle = 0
		}
	}

	max := GetSystemSessionMaxTime()
	if idle > 0 && max > 0 && idle > max {
		idle = max
	}
	return idle
}

// ResolveSessionTimeout picks the effective timeout based on user preference.
func ResolveSessionTimeout(userPref int) int {
	systemTTL := GetSystemSessionMaxTime()
	if userPref <= 0 {
		return systemTTL
	}

	prefTTL := clampSessionDuration(userPref)
	if systemTTL > 0 && prefTTL > systemTTL {
		return systemTTL
	}
	return prefTTL
}

func clampSessionDuration(value int) int {
	if value <= 0 {
		return 0
	}
	if value < constants.MinSessionTimeout {
		return constants.MinSessionTimeout
	}
	if value > constants.MaxSessionTimeout {
		return constants.MaxSessionTimeout
	}
	return value
}

func clampIdleDuration(value int) int {
	if value <= 0 {
		return 0
	}
	if value < constants.MinSessionIdleTimeout {
		return constants.MinSessionIdleTimeout
	}
	if value > constants.MaxSessionTimeout {
		return constants.MaxSessionTimeout
	}
	return value
}
