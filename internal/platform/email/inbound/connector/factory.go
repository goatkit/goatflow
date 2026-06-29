package connector

import (
	"fmt"
	"strings"
	"sync"
)

// FactoryOption customizes a connector factory.
type FactoryOption func(*simpleFactory)

type simpleFactory struct {
	mu       sync.RWMutex
	fetchers map[string]Fetcher
}

// NewFactory builds a connector factory with the provided options.
func NewFactory(opts ...FactoryOption) Factory {
	f := &simpleFactory{fetchers: make(map[string]Fetcher)}
	for _, opt := range opts {
		if opt != nil {
			opt(f)
		}
	}
	return f
}

// DefaultFactory returns a factory preloaded with built-in connectors.
func DefaultFactory() Factory {
	return NewFactory(
		WithFetcher(NewPOP3Fetcher(), "pop3", "pop3s", "pop3_tls", "pop3s_tls"),
		WithFetcher(NewIMAPFetcher(), "imap", "imaps", "imap_tls", "imaps_tls", "imaptls"),
	)
}

// WithFetcher registers a fetcher for the provided account types.
func WithFetcher(fetcher Fetcher, accountTypes ...string) FactoryOption {
	return func(f *simpleFactory) {
		if f == nil || fetcher == nil {
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, t := range accountTypes {
			key := normalizeType(t)
			if key == "" {
				continue
			}
			f.fetchers[key] = fetcher
		}
	}
}

func (f *simpleFactory) FetcherFor(account Account) (Fetcher, error) {
	key := normalizeType(account.Type)
	f.mu.RLock()
	fetcher, ok := f.fetchers[key]
	f.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no connector registered for account type %s", account.Type)
	}
	return fetcher, nil
}

func normalizeType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
