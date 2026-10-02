package main

import (
	"testing"
	"time"
)

func TestParseDrainTimeout(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", defaultDrainTimeout, false},
		{"  ", defaultDrainTimeout, false},
		{"15s", 15 * time.Second, false},
		{"1m", time.Minute, false},
		{"30", defaultDrainTimeout, true}, // no unit
		{"0s", defaultDrainTimeout, true},
		{"-5s", defaultDrainTimeout, true},
		{"soon", defaultDrainTimeout, true},
	}
	for _, tc := range cases {
		got, err := parseDrainTimeout(tc.in)
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Errorf("parseDrainTimeout(%q) = %v, %v; want %v, error=%v", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
	// The default must leave room inside Docker's default 10s stop grace
	// period for the scheduler/plugin steps that follow the drain.
	if defaultDrainTimeout >= 10*time.Second {
		t.Errorf("defaultDrainTimeout = %v, want < 10s (Docker's default stop grace period)", defaultDrainTimeout)
	}
}
