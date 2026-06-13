package configstore

import (
	"context"
	"testing"
	"time"
)

func TestNotifierPublishesConfigActivatedNotification(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	listenerPool := newMigratedPostgresPoolFromDSN(t, ctx, pool.Config().ConnString())
	notifier := NewNotifier(pool)

	conn, err := listenerPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "listen "+ConfigActivatedChannel); err != nil {
		t.Fatalf("listen %s: %v", ConfigActivatedChannel, err)
	}

	if err := notifier.NotifyConfigActivated(ctx, 42); err != nil {
		t.Fatalf("NotifyConfigActivated() error = %v", err)
	}
	notification, err := conn.Conn().WaitForNotification(ctx)
	if err != nil {
		t.Fatalf("WaitForNotification() error = %v", err)
	}
	if notification.Channel != ConfigActivatedChannel || notification.Payload != "42" {
		t.Fatalf("notification = %s/%s, want %s/42", notification.Channel, notification.Payload, ConfigActivatedChannel)
	}
}
