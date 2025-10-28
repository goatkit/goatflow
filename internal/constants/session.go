package constants

// Session timeout constants (in seconds)
const (
	DefaultSessionTimeout     = 57600 // 16 hours total lifetime
	DefaultSessionIdleTimeout = 7200  // 2 hours idle window
	MaxSessionTimeout         = 604800
	MinSessionTimeout         = 3600
	MinSessionIdleTimeout     = 300
	RefreshTokenTimeout       = 604800
	SessionWarningTime        = 300
)

// Predefined timeout options for user preferences (in seconds)
var SessionTimeoutOptions = map[string]int{
	"System Default": 0,
	"1 hour":         3600,
	"2 hours":        7200,
	"4 hours":        14400,
	"8 hours":        28800,
	"16 hours":       57600,
	"24 hours":       86400,
	"3 days":         259200,
	"7 days":         604800,
}
