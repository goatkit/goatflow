package adapter

import (
	"testing"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
	"github.com/goatkit/goatflow/internal/models"
)

func TestAccountFromModelDefaults(t *testing.T) {
	acct := AccountFromModel(&models.EmailAccount{
		ID:                  42,
		QueueID:             7,
		Login:               "agent",
		Host:                "mail.example",
		AccountType:         "POP3S",
		PasswordEncrypted:   "secret",
		DispatchingMode:     "queue",
		Trusted:             true,
		AllowTrustedHeaders: true,
		PollIntervalSeconds: 30,
	})

	if acct.ID != 42 {
		t.Fatalf("expected ID 42, got %d", acct.ID)
	}
	if acct.Type != "pop3s" {
		t.Fatalf("expected lowercase account type, got %s", acct.Type)
	}
	if acct.QueueID != 7 {
		t.Fatalf("expected queue id 7, got %d", acct.QueueID)
	}
	if string(acct.Password) != "secret" {
		t.Fatalf("expected password bytes, got %s", string(acct.Password))
	}
	if acct.PollInterval.Seconds() != 30 {
		t.Fatalf("expected poll interval 30s, got %v", acct.PollInterval)
	}
}

func TestAccountFromModelNil(t *testing.T) {
	acct := AccountFromModel(nil)
	zero := connector.Account{}
	if acct.ID != zero.ID || acct.Username != zero.Username || len(acct.Password) != 0 {
		t.Fatalf("expected zero account, got %+v", acct)
	}
}

func TestAccountFromModelFallsBackType(t *testing.T) {
	acct := AccountFromModel(&models.EmailAccount{AccountType: "  "})
	if acct.Type != "pop3" {
		t.Fatalf("expected default type pop3, got %s", acct.Type)
	}
}
