package constants

// Session timeout constants (in seconds)
const (
	// DefaultSessionTimeout is the default session timeout (24 hours)
	DefaultSessionTimeout = 86400
	
	// MaxSessionTimeout is the maximum allowed session timeout (7 days)
	MaxSessionTimeout = 604800
	
	// MinSessionTimeout is the minimum allowed session timeout (1 hour)
	MinSessionTimeout = 3600
	
	// RefreshTokenTimeout is the refresh token timeout (7 days)
	RefreshTokenTimeout = 604800
	
	// SessionWarningTime is when to show warning before expiry (5 minutes)
	SessionWarningTime = 300
)

// Predefined timeout options for user preferences (in seconds)
var SessionTimeoutOptions = map[string]int{
	"System Default": 0,      // Use system default
	"1 hour":         3600,
	"4 hours":        14400,
	"8 hours":        28800,
	"24 hours":       86400,
	"3 days":         259200,
	"7 days":         604800,
}