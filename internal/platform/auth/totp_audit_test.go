package auth

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

// Events without a login must identify the user by decimal ID, not by the
// Unicode code point the ID happens to map to.
func TestLogTOTPAuditEventUserIDFallback(t *testing.T) {
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})

	LogTOTPAuditEvent(TOTPAuditEvent{EventType: AuditTOTPVerifyFailed, UserID: 65})

	if out := buf.String(); !strings.Contains(out, " user=65 ") {
		t.Fatalf("audit line should carry user=65, got %q", out)
	}
}
