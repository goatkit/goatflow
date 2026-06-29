package push

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"

	"github.com/goatkit/goatflow/internal/platform/notifications"
)

// PushConfig holds VAPID configuration for push dispatching.
type PushConfig struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDContact    string
}

// DispatchPushReminder sends push notifications for a pending reminder to all subscribed recipients.
func DispatchPushReminder(ctx context.Context, db *sql.DB, recipients []int, reminder notifications.PendingReminder, cfg PushConfig) {
	if cfg.VAPIDPublicKey == "" || cfg.VAPIDPrivateKey == "" {
		return
	}

	subs, err := GetSubscriptionsForUsers(ctx, db, recipients, "agent")
	if err != nil {
		log.Printf("push: failed to get subscriptions: %v", err)
		return
	}
	if len(subs) == 0 {
		return
	}

	payload, err := json.Marshal(map[string]string{
		"title": "Ticket Reminder",
		"body":  reminder.Title + " (" + reminder.QueueName + ")",
		"url":   "/tickets/" + reminder.TicketNumber,
		"tag":   "reminder-" + reminder.TicketNumber,
	})
	if err != nil {
		log.Printf("push: failed to marshal payload: %v", err)
		return
	}

	for _, sub := range subs {
		if err := Send(sub, payload, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDContact); err != nil {
			var expired *SubscriptionExpiredError
			if errors.As(err, &expired) {
				// Remove stale subscription
				if delErr := DeleteSubscriptionByEndpoint(ctx, db, sub.Endpoint); delErr != nil {
					log.Printf("push: failed to remove stale subscription: %v", delErr)
				}
				continue
			}
			log.Printf("push: failed to send to %s: %v", sub.Endpoint, err)
		}
	}
}
