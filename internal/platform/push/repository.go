package push

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SaveSubscription stores or updates a push subscription.
func SaveSubscription(ctx context.Context, db *sql.DB, userID int, userType, endpoint, p256dh, auth string) error {
	query := `INSERT INTO gk_push_subscription (user_id, user_type, endpoint, p256dh, auth, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE user_id = VALUES(user_id), user_type = VALUES(user_type),
			p256dh = VALUES(p256dh), auth = VALUES(auth), created_at = VALUES(created_at)`
	_, err := db.ExecContext(ctx, query, userID, userType, endpoint, p256dh, auth, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("save push subscription: %w", err)
	}
	return nil
}

// DeleteSubscriptionByEndpoint removes a push subscription by endpoint URL.
func DeleteSubscriptionByEndpoint(ctx context.Context, db *sql.DB, endpoint string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM gk_push_subscription WHERE endpoint = ?`, endpoint)
	if err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	return nil
}

// GetSubscriptionsForUser returns all push subscriptions for a specific user.
func GetSubscriptionsForUser(ctx context.Context, db *sql.DB, userID int, userType string) ([]Subscription, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT endpoint, p256dh, auth FROM gk_push_subscription WHERE user_id = ? AND user_type = ?`,
		userID, userType)
	if err != nil {
		return nil, fmt.Errorf("get push subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, fmt.Errorf("scan push subscription: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// GetSubscriptionsForUsers returns push subscriptions for multiple users.
func GetSubscriptionsForUsers(ctx context.Context, db *sql.DB, userIDs []int, userType string) ([]Subscription, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]any, 0, len(userIDs)+1)
	for i, id := range userIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, userType)

	query := fmt.Sprintf(
		`SELECT endpoint, p256dh, auth FROM gk_push_subscription WHERE user_id IN (%s) AND user_type = ?`,
		strings.Join(placeholders, ",")) //nolint:gk-sql-sprintf // internal schema identifier; values bound via ?

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get push subscriptions for users: %w", err)
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			return nil, fmt.Errorf("scan push subscription: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}
