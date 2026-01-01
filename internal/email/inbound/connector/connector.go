// Package connector provides email server connection interfaces and implementations.
package connector

import (
	"context"
	"time"
)

// Account carries the minimal set of fields a connector needs to open a mailbox.
type Account struct {
	ID                  int
	QueueID             int
	Type                string // pop3, pop3s, imap, imaps, graph, etc.
	Host                string
	Port                int
	Username            string
	Password            []byte
	OAuth2ConfigID      *int
	Trusted             bool
	IMAPFolder          string
	DispatchingMode     string // queue|from
	AllowTrustedHeaders bool
	PollInterval        time.Duration
}

// FetchedMessage wraps the on-wire RFC822 payload plus derived metadata.
type FetchedMessage struct {
	AccountID  int
	Connector  string
	UID        string
	RemoteID   string
	ReceivedAt time.Time
	SizeBytes  int64
	Raw        []byte
	Metadata   map[string]string
	account    Account
}

// AccountSnapshot returns the account metadata captured when the fetch occurred.
func (m FetchedMessage) AccountSnapshot() Account {
	return m.account
}

// WithAccount captures the account metadata on the message.
func (m *FetchedMessage) WithAccount(acc Account) {
	m.account = acc
	m.AccountID = acc.ID
}

// Handler receives fully fetched messages and hands them to PostMaster.
type Handler interface {
	Handle(ctx context.Context, msg *FetchedMessage) error
}

// Fetcher implementations (POP3, IMAP, etc.) stream messages to a handler.
type Fetcher interface {
	Name() string
	Fetch(ctx context.Context, account Account, handler Handler) error
}

// Factory resolves the correct connector implementation for a mailbox.
type Factory interface {
	FetcherFor(account Account) (Fetcher, error)
}
