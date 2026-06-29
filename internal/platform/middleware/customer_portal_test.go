package middleware

import "testing"

func TestBypassCustomerCaptiveRedirect(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "login page", path: "/customer/login", want: true},
		{name: "logout", path: "/customer/logout", want: true},
		{name: "customer api", path: "/customer/api/preferences/language", want: true},
		{name: "profile page", path: "/customer/profile", want: true},
		{name: "profile update", path: "/customer/profile/update", want: true},
		{name: "password form", path: "/customer/password/form", want: true},
		{name: "similar profile prefix", path: "/customer/profiled", want: false},
		{name: "similar api prefix", path: "/customer/apiary", want: false},
		{name: "dashboard remains captive", path: "/customer", want: false},
		{name: "tickets remain captive", path: "/customer/tickets", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bypassCustomerCaptiveRedirect(tt.path); got != tt.want {
				t.Fatalf("bypassCustomerCaptiveRedirect(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
