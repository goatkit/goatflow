// Package ticketutil provides utility functions for ticket operations.
// This package is kept minimal to avoid import cycles with repository.
package ticketutil

import "time"

// Pending state type IDs
const (
	PendingReminderStateTypeID = 4
	PendingAutoStateTypeID     = 5
)

// DefaultPendingDuration is the default duration to use when a pending ticket
// has no explicit pending_until time set. This provides a fallback for
// legacy/migrated data that may be missing the date.
const DefaultPendingDuration = 24 * time.Hour

// GetEffectivePendingTime returns the effective pending time for a ticket.
// If untilTime is set (> 0), it returns that time.
// If untilTime is not set (0 or negative), it returns now + DefaultPendingDuration.
// This ensures pending tickets always have a usable deadline.
func GetEffectivePendingTime(untilTime int) time.Time {
	if untilTime > 0 {
		return time.Unix(int64(untilTime), 0).UTC()
	}
	return time.Now().UTC().Add(DefaultPendingDuration)
}

// GetEffectivePendingTimeUnix returns the effective pending time as a unix timestamp.
// This is useful for SQL queries and comparisons.
func GetEffectivePendingTimeUnix(untilTime int) int64 {
	return GetEffectivePendingTime(untilTime).Unix()
}

// EnsurePendingTime returns the provided untilTime if valid, or a default
// time (now + DefaultPendingDuration) as a unix timestamp. This is useful
// when setting the pending time on state changes.
func EnsurePendingTime(untilTime int) int {
	if untilTime > 0 {
		return untilTime
	}
	return int(time.Now().UTC().Add(DefaultPendingDuration).Unix())
}

// IsPendingStateType returns true if the state type is a pending state
// (either pending reminder or pending auto-close).
func IsPendingStateType(typeID int) bool {
	return typeID == PendingReminderStateTypeID || typeID == PendingAutoStateTypeID
}
