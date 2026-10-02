package sysconfig

import "testing"

// LandingPath is used as a redirect target, so only local paths may come out.
func TestCustomerPortalLandingPath(t *testing.T) {
	tests := []struct {
		landing string
		want    string
	}{
		{"/customer/tickets/new", "/customer/tickets/new"},
		{"  /customer/company  ", "/customer/company"},
		{"tickets", "/customer/tickets"},
		{"./tickets/new", "/customer/tickets/new"},
		{"", DefaultCustomerPortalLandingPage},
		{"//evil.example.com/x", DefaultCustomerPortalLandingPage},
		{"https://evil.example.com/", DefaultCustomerPortalLandingPage},
		{"javascript://alert(1)", DefaultCustomerPortalLandingPage},
		{"/\\evil.example.com", DefaultCustomerPortalLandingPage},
		{"/customer\r\nLocation: x", DefaultCustomerPortalLandingPage},
	}
	for _, tt := range tests {
		if got := (CustomerPortalConfig{LandingPage: tt.landing}).LandingPath(); got != tt.want {
			t.Errorf("LandingPath(%q) = %q, want %q", tt.landing, got, tt.want)
		}
	}
}

// One default everywhere: the built-in config and the sysconfig row created on save.
func TestCustomerPortalLandingDefault(t *testing.T) {
	if got := DefaultCustomerPortalConfig().LandingPage; got != DefaultCustomerPortalLandingPage {
		t.Errorf("DefaultCustomerPortalConfig().LandingPage = %q", got)
	}
	if got := loadPortalDefaults()["CustomerPortal::LandingPage"]; got != DefaultCustomerPortalLandingPage {
		t.Errorf("defaults.yaml CustomerPortal::LandingPage = %q", got)
	}
	for _, def := range portalKeyDefs() {
		if def.name == "CustomerPortal::LandingPage" && def.defaultVal != DefaultCustomerPortalLandingPage {
			t.Errorf("portalKeyDefs LandingPage default = %q", def.defaultVal)
		}
	}
}
