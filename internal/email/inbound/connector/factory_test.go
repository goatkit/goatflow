package connector

import (
	"context"
	"testing"
)

type noopFetcher struct{}

func (noopFetcher) Name() string { return "noop" }

func (noopFetcher) Fetch(ctx context.Context, account Account, handler Handler) error { return nil }

func TestFactoryReturnsRegisteredFetcher(t *testing.T) {
	fetcher := noopFetcher{}
	factory := NewFactory(WithFetcher(fetcher, "Pop3"))

	connFetcher, err := factory.FetcherFor(Account{Type: "POP3"})
	if err != nil {
		t.Fatalf("expected fetcher, got error %v", err)
	}
	if connFetcher.Name() != "noop" {
		t.Fatalf("unexpected fetcher %s", connFetcher.Name())
	}
}

func TestFactoryNormalizesAndErrors(t *testing.T) {
	factory := NewFactory(WithFetcher(noopFetcher{}, "  POP3s  "))

	fetcher, err := factory.FetcherFor(Account{Type: "pop3s"})
	if err != nil {
		t.Fatalf("expected fetcher, got error %v", err)
	}
	if fetcher.Name() != "noop" {
		t.Fatalf("unexpected fetcher %s", fetcher.Name())
	}

	if _, err := factory.FetcherFor(Account{Type: "imap"}); err == nil {
		t.Fatalf("expected error for unknown type")
	}
}

func TestWithFetcherSkipsNil(t *testing.T) {
	factory := NewFactory(WithFetcher(nil, "pop3"))
	if _, err := factory.FetcherFor(Account{Type: "pop3"}); err == nil {
		t.Fatalf("expected missing fetcher error")
	}
}
