package dynamic

import (
	"testing"

	"github.com/goatkit/goatflow/internal/mailaccountmeta"
)

func TestApplyMailAccountWriteTransform(t *testing.T) {
	data := map[string]interface{}{
		"dispatching_mode":      "from",
		"allow_trusted_headers": "1",
		"poll_interval_seconds": "45",
		"comments":              "Ops mailbox",
		"queue_id":              12,
	}

	applyMailAccountWriteTransform(data)

	if _, ok := data["dispatching_mode"]; ok {
		t.Fatalf("expected dispatching_mode to be removed after transform")
	}
	if _, ok := data["allow_trusted_headers"]; ok {
		t.Fatalf("expected allow_trusted_headers to be removed after transform")
	}
	if _, ok := data["poll_interval_seconds"]; ok {
		t.Fatalf("expected poll_interval_seconds to be removed after transform")
	}

	if queue, _ := coerceInt(data["queue_id"]); queue != 0 {
		t.Fatalf("expected queue_id to be zeroed for from-based dispatch, got %d", queue)
	}
	if trusted, _ := coerceInt(data["trusted"]); trusted != 1 {
		t.Fatalf("expected trusted flag set when allow_trusted_headers true, got %d", trusted)
	}

	raw, _ := data["comments"].(string)
	if raw == "" {
		t.Fatalf("expected comments to include encoded metadata")
	}

	base, meta := mailaccountmeta.DecodeComment(raw)
	if base != "Ops mailbox" {
		t.Fatalf("expected base comment preserved, got %q", base)
	}
	if meta.DispatchingMode != "from" {
		t.Fatalf("expected dispatching mode to be stored, got %q", meta.DispatchingMode)
	}
	if meta.AllowTrustedHeaders == nil || !*meta.AllowTrustedHeaders {
		t.Fatalf("expected trusted headers flag to be stored")
	}
	if meta.PollIntervalSeconds == nil || *meta.PollIntervalSeconds != 45 {
		t.Fatalf("expected poll interval metadata, got %v", meta.PollIntervalSeconds)
	}
}

func TestApplyMailAccountReadTransformWithMetadata(t *testing.T) {
	allow := true
	poll := 30
	raw := mailaccountmeta.EncodeComment("Inbox", mailaccountmeta.Metadata{
		DispatchingMode:     "from",
		AllowTrustedHeaders: &allow,
		PollIntervalSeconds: &poll,
	})

	item := map[string]interface{}{
		"comments": raw,
		"queue_id": 5,
		"trusted":  0,
	}

	applyMailAccountReadTransform(item)

	if comment, ok := item["comments"].(string); !ok || comment != "Inbox" {
		t.Fatalf("expected base comment restored, got %v", item["comments"])
	}
	if mode := item["dispatching_mode"]; mode != "from" {
		t.Fatalf("expected dispatching_mode 'from', got %v", mode)
	}
	if allow, _ := item["allow_trusted_headers"].(bool); !allow {
		t.Fatalf("expected allow_trusted_headers true after decode")
	}
	if poll, ok := item["poll_interval_seconds"].(int); !ok || poll != 30 {
		t.Fatalf("expected poll interval 30, got %v", item["poll_interval_seconds"])
	}
}

func TestApplyMailAccountReadTransformWithoutMetadata(t *testing.T) {
	item := map[string]interface{}{
		"comments": "Plain",
		"queue_id": 9,
		"trusted":  1,
	}

	applyMailAccountReadTransform(item)

	if mode := item["dispatching_mode"]; mode != "queue" {
		t.Fatalf("expected default dispatching_mode 'queue', got %v", mode)
	}
	val, ok := item["allow_trusted_headers"].(bool)
	if !ok || !val {
		t.Fatalf("expected allow_trusted_headers to mirror trusted flag")
	}
	if poll, ok := item["poll_interval_seconds"].(int); !ok || poll != 0 {
		t.Fatalf("expected zero poll interval default, got %v", item["poll_interval_seconds"])
	}
}
