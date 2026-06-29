package push

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSaveSubscription(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO gk_push_subscription").
		WithArgs(1, "agent", "https://push.example.com/sub1", "p256dh-key", "auth-key", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = SaveSubscription(context.Background(), db, 1, "agent", "https://push.example.com/sub1", "p256dh-key", "auth-key")
	if err != nil {
		t.Fatalf("SaveSubscription() error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDeleteSubscriptionByEndpoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("DELETE FROM gk_push_subscription").
		WithArgs("https://push.example.com/sub1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = DeleteSubscriptionByEndpoint(context.Background(), db, "https://push.example.com/sub1")
	if err != nil {
		t.Fatalf("DeleteSubscriptionByEndpoint() error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetSubscriptionsForUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"endpoint", "p256dh", "auth"}).
		AddRow("https://push.example.com/sub1", "p256dh-1", "auth-1").
		AddRow("https://push.example.com/sub2", "p256dh-2", "auth-2")

	mock.ExpectQuery("SELECT endpoint, p256dh, auth FROM gk_push_subscription").
		WithArgs(1, "agent").
		WillReturnRows(rows)

	subs, err := GetSubscriptionsForUser(context.Background(), db, 1, "agent")
	if err != nil {
		t.Fatalf("GetSubscriptionsForUser() error: %v", err)
	}

	if len(subs) != 2 {
		t.Errorf("got %d subscriptions, want 2", len(subs))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetSubscriptionsForUsers_Empty(t *testing.T) {
	subs, err := GetSubscriptionsForUsers(context.Background(), (*sql.DB)(nil), []int{}, "agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subs != nil {
		t.Errorf("expected nil, got %v", subs)
	}
}

func TestGetSubscriptionsForUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"endpoint", "p256dh", "auth"}).
		AddRow("https://push.example.com/sub1", "p256dh-1", "auth-1")

	mock.ExpectQuery("SELECT endpoint, p256dh, auth FROM gk_push_subscription WHERE user_id IN").
		WithArgs(1, 2, "agent").
		WillReturnRows(rows)

	subs, err := GetSubscriptionsForUsers(context.Background(), db, []int{1, 2}, "agent")
	if err != nil {
		t.Fatalf("GetSubscriptionsForUsers() error: %v", err)
	}

	if len(subs) != 1 {
		t.Errorf("got %d subscriptions, want 1", len(subs))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
