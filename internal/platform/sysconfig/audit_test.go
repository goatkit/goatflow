package sysconfig

import (
	"errors"
	"testing"
)

func TestAuditSettingsValidateBounds(t *testing.T) {
	cases := map[int]bool{-1: false, 0: true, 1: true, 365: true, AuditMaxRetentionDays: true, AuditMaxRetentionDays + 1: false}
	for days, ok := range cases {
		err := AuditSettings{RetentionDays: days}.Validate()
		if (err == nil) != ok {
			t.Errorf("retention_days=%d: err=%v, want ok=%v", days, err, ok)
		}
	}
}

func TestSaveAuditSettingsRejectsBeforeStorage(t *testing.T) {
	err := SaveAuditSettings(nil, AuditSettings{RetentionDays: -5}, 1)
	var invalid *AuditSettingsValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want validation error", err)
	}
}
