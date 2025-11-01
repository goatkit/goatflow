package notifications

import (
	"context"
	"fmt"
	"sync"
)

var (
	globalMu           sync.RWMutex
	globalHub          Hub = NewMemoryHub()
	globalEmailProvider EmailProvider
)

// SetHub replaces the shared hub instance and returns the previous hub.
func SetHub(h Hub) Hub {
	globalMu.Lock()
	defer globalMu.Unlock()
	prev := globalHub
	if h == nil {
		globalHub = NewMemoryHub()
	} else {
		globalHub = h
	}
	return prev
}

// GetHub returns the shared hub instance.
func GetHub() Hub {
	globalMu.RLock()
	h := globalHub
	globalMu.RUnlock()
	return h
}

// SetEmailProvider sets the global email provider.
func SetEmailProvider(p EmailProvider) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalEmailProvider = p
}

// GetEmailProvider returns the global email provider.
func GetEmailProvider() EmailProvider {
	globalMu.RLock()
	p := globalEmailProvider
	globalMu.RUnlock()
	return p
}

// SendEmail is a convenience function for sending simple text emails.
func SendEmail(to, subject, body string) error {
	provider := GetEmailProvider()
	if provider == nil {
		return fmt.Errorf("no email provider configured")
	}

	msg := EmailMessage{
		To:      []string{to},
		Subject: subject,
		Body:    body,
		HTML:    false,
	}

	return provider.Send(context.Background(), msg)
}
