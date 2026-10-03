package version_test

import (
	"testing"

	"github.com/goatkit/goatflow/internal/platform/version"
)

func TestHostCompatible(t *testing.T) {
	old := version.Version
	t.Cleanup(func() { version.Version = old })

	cases := []struct {
		host, min string
		want      bool
	}{
		{"0.10.0", "", true},
		{"0.10.0", "0.9.5", true},
		{"0.10.0", "0.10.0", true},
		{"0.10.0", "v0.10.0", true},
		{"v0.10.0", "0.10.0", true},
		{"0.10.0", "0.10.1", false},
		{"0.10.0", "0.11.0", false},
		{"0.9.0", "0.10.0", false}, // numeric, not lexical
		{"dev", "99.0.0", true},
		{"", "99.0.0", true},
		{"main", "99.0.0", true}, // untagged branch build
	}
	for _, tc := range cases {
		version.Version = tc.host
		if got := version.HostCompatible(tc.min); got != tc.want {
			t.Errorf("host %q, min %q: HostCompatible = %v, want %v", tc.host, tc.min, got, tc.want)
		}
	}
}
