package repository

import (
	"strings"
	"testing"

	"github.com/goatkit/goatflow/internal/mailaccountmeta"
	"github.com/goatkit/goatflow/internal/models"
)

func TestEncodeDecodeMailAccountComment(t *testing.T) {
	allow := true
	poll := 45
	meta := mailaccountmeta.Metadata{
		DispatchingMode:     "from",
		AllowTrustedHeaders: &allow,
		PollIntervalSeconds: &poll,
	}

	raw := mailaccountmeta.EncodeComment("Demo mailbox", meta)
	if !strings.Contains(raw, "-- GoatFlow META --") {
		t.Fatalf("encoded comment missing metadata marker: %q", raw)
	}

	comment, decoded := mailaccountmeta.DecodeComment(raw)
	if comment != "Demo mailbox" {
		t.Fatalf("expected base comment preserved, got %q", comment)
	}
	if decoded.DispatchingMode != "from" {
		t.Fatalf("expected dispatching mode restored, got %q", decoded.DispatchingMode)
	}
	if decoded.AllowTrustedHeaders == nil || !*decoded.AllowTrustedHeaders {
		t.Fatalf("expected allow trusted headers flag restored")
	}
	if decoded.PollIntervalSeconds == nil || *decoded.PollIntervalSeconds != 45 {
		t.Fatalf("expected poll interval restored, got %+v", decoded.PollIntervalSeconds)
	}
}

func TestCommentSQLValueSkipsEmpty(t *testing.T) {
	acct := &models.EmailAccount{}
	if value := commentSQLValue(acct); value != nil {
		t.Fatalf("expected nil comment payload, got %v", value)
	}

	poll := 30
	acct.PollIntervalSeconds = poll
	value := commentSQLValue(acct)
	if value == nil {
		t.Fatalf("expected serialized metadata when poll interval set")
	}
	raw := value.(string)
	if !strings.Contains(raw, "\"poll_interval_seconds\":30") {
		t.Fatalf("metadata serialization missing poll interval: %s", raw)
	}
}

func TestApplyMailAccountMetadata(t *testing.T) {
	acct := &models.EmailAccount{DispatchingMode: "queue"}
	allow := true
	poll := 90
	meta := mailaccountmeta.Metadata{
		DispatchingMode:     "from",
		AllowTrustedHeaders: &allow,
		PollIntervalSeconds: &poll,
	}
	applyMailAccountMetadata(acct, meta)
	if acct.DispatchingMode != "from" {
		t.Fatalf("expected dispatching mode override, got %s", acct.DispatchingMode)
	}
	if !acct.AllowTrustedHeaders || !acct.Trusted {
		t.Fatalf("expected trusted flags set from metadata")
	}
	if acct.PollIntervalSeconds != 90 {
		t.Fatalf("expected poll interval to be 90, got %d", acct.PollIntervalSeconds)
	}
}
