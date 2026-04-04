package push

import (
	"fmt"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Subscription represents a Web Push subscription.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// Send delivers a push notification to a single subscription.
func Send(sub Subscription, payload []byte, vapidPublicKey, vapidPrivateKey, vapidContact string) error {
	s := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}

	resp, err := webpush.SendNotification(payload, s, &webpush.Options{
		Subscriber:      vapidContact,
		VAPIDPublicKey:  vapidPublicKey,
		VAPIDPrivateKey: vapidPrivateKey,
		TTL:             86400,
	})
	if err != nil {
		return fmt.Errorf("webpush send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		return &SubscriptionExpiredError{Endpoint: sub.Endpoint, StatusCode: resp.StatusCode}
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webpush returned HTTP %d for %s", resp.StatusCode, sub.Endpoint)
	}

	return nil
}

// SubscriptionExpiredError indicates a subscription endpoint is no longer valid.
type SubscriptionExpiredError struct {
	Endpoint   string
	StatusCode int
}

func (e *SubscriptionExpiredError) Error() string {
	return fmt.Sprintf("push subscription expired (HTTP %d): %s", e.StatusCode, e.Endpoint)
}
