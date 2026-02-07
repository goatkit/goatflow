package notifications

import (
	"strings"
	"testing"
)

func TestApplyBrandingPlainText(t *testing.T) {
	identity := &QueueIdentity{
		SalutationText:        "Dear Customer,",
		SalutationContentType: "text/plain",
		SignatureText:         "Your GoatFlow Team",
		SignatureContentType:  "text/plain",
	}
	body := ApplyBranding("Thanks for reaching out.", false, identity, nil)

	if !strings.Contains(body, "Dear Customer,") {
		t.Fatalf("expected salutation in body: %s", body)
	}
	if !strings.Contains(body, "Your GoatFlow Team") {
		t.Fatalf("expected signature in body: %s", body)
	}
	if strings.Count(body, "Dear Customer,") != 1 {
		t.Fatalf("salutation duplicated: %s", body)
	}
}

func TestApplyBrandingHtmlUpgrade(t *testing.T) {
	identity := &QueueIdentity{
		SalutationText:        "<p><strong>Hello</strong> there,</p>",
		SalutationContentType: "text/html",
		SignatureText:         "Kind regards",
		SignatureContentType:  "text/plain",
	}
	body := ApplyBranding("Please review the update.", false, identity, nil)

	if !strings.Contains(body, "<strong>") {
		t.Fatalf("expected HTML salutation to be preserved: %s", body)
	}
	if !strings.Contains(body, "<p>Please review the update.") {
		t.Fatalf("expected body to be wrapped as HTML: %s", body)
	}
	if !strings.Contains(body, "Kind regards") {
		t.Fatalf("expected signature text: %s", body)
	}
}

func TestApplyBrandingInterpolatesPlaceholders(t *testing.T) {
	identity := &QueueIdentity{
		SalutationText:        "Hello <OTRS_CUSTOMER_REALNAME>,",
		SalutationContentType: "text/plain",
		SignatureText:         "Thanks, <OTRS_Agent_UserFirstname> <OTRS_Agent_UserLastname>",
		SignatureContentType:  "text/plain",
	}
	rc := &RenderContext{CustomerFullName: "Ada Lovelace", AgentFirstName: "Grace", AgentLastName: "Hopper"}
	body := ApplyBranding("Update ready.", false, identity, rc)

	if !strings.Contains(body, "Ada Lovelace") {
		t.Fatalf("expected customer name substitution: %s", body)
	}
	if !strings.Contains(body, "Grace Hopper") {
		t.Fatalf("expected agent name substitution: %s", body)
	}
}
