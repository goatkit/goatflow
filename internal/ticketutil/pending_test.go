package ticketutil

import (
	"testing"
	"time"
)

func TestGetEffectivePendingTime(t *testing.T) {
	now := time.Now().UTC()
	hourFromNow := now.Add(time.Hour)

	tests := []struct {
		name          string
		untilTime     int
		wantApprox    time.Time
		withinSeconds int // How close the result should be to wantApprox
	}{
		{
			name:          "zero untilTime returns now+24h",
			untilTime:     0,
			wantApprox:    now.Add(DefaultPendingDuration),
			withinSeconds: 5,
		},
		{
			name:          "negative untilTime returns now+24h",
			untilTime:     -1,
			wantApprox:    now.Add(DefaultPendingDuration),
			withinSeconds: 5,
		},
		{
			name:          "positive untilTime returns that time",
			untilTime:     int(hourFromNow.Unix()),
			wantApprox:    hourFromNow,
			withinSeconds: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEffectivePendingTime(tt.untilTime)
			diff := got.Sub(tt.wantApprox)
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Duration(tt.withinSeconds)*time.Second {
				t.Errorf("GetEffectivePendingTime(%d) = %v, want approximately %v (diff: %v)",
					tt.untilTime, got, tt.wantApprox, diff)
			}
		})
	}
}

func TestGetEffectivePendingTimeUnix(t *testing.T) {
	hourFromNow := time.Now().UTC().Add(time.Hour)
	untilTime := int(hourFromNow.Unix())

	got := GetEffectivePendingTimeUnix(untilTime)
	if got != int64(untilTime) {
		t.Errorf("GetEffectivePendingTimeUnix(%d) = %d, want %d", untilTime, got, untilTime)
	}
}

func TestEnsurePendingTime(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name       string
		untilTime  int
		wantSame   bool   // If true, result should equal input
		wantApprox int64  // If wantSame is false, result should be close to this
	}{
		{
			name:      "positive untilTime returns same value",
			untilTime: 12345678,
			wantSame:  true,
		},
		{
			name:       "zero untilTime returns default",
			untilTime:  0,
			wantSame:   false,
			wantApprox: now.Add(DefaultPendingDuration).Unix(),
		},
		{
			name:       "negative untilTime returns default",
			untilTime:  -100,
			wantSame:   false,
			wantApprox: now.Add(DefaultPendingDuration).Unix(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnsurePendingTime(tt.untilTime)
			if tt.wantSame {
				if got != tt.untilTime {
					t.Errorf("EnsurePendingTime(%d) = %d, want %d", tt.untilTime, got, tt.untilTime)
				}
			} else {
				diff := int64(got) - tt.wantApprox
				if diff < 0 {
					diff = -diff
				}
				if diff > 5 { // Allow 5 seconds tolerance
					t.Errorf("EnsurePendingTime(%d) = %d, want approximately %d (diff: %d)",
						tt.untilTime, got, tt.wantApprox, diff)
				}
			}
		})
	}
}

func TestIsPendingStateType(t *testing.T) {
	tests := []struct {
		name   string
		typeID int
		want   bool
	}{
		{"type 1 (new) is not pending", 1, false},
		{"type 2 (open) is not pending", 2, false},
		{"type 3 (closed) is not pending", 3, false},
		{"type 4 (pending reminder) is pending", 4, true},
		{"type 5 (pending auto) is pending", 5, true},
		{"type 6 is not pending", 6, false},
		{"type 0 is not pending", 0, false},
		{"negative type is not pending", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPendingStateType(tt.typeID)
			if got != tt.want {
				t.Errorf("IsPendingStateType(%d) = %v, want %v", tt.typeID, got, tt.want)
			}
		})
	}
}
