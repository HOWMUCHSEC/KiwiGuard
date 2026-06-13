package configstore

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNotifierPublishesRevisionPayload(t *testing.T) {
	ctx := context.Background()
	exec := &fakeNotificationExecutor{}

	if err := NewNotifier(exec).NotifyConfigActivated(ctx, 42); err != nil {
		t.Fatalf("NotifyConfigActivated() error = %v", err)
	}
	if exec.sql != "select pg_notify($1, $2)" {
		t.Fatalf("notification SQL = %q, want pg_notify", exec.sql)
	}
	wantArgs := []any{ConfigActivatedChannel, "42"}
	if len(exec.args) != len(wantArgs) {
		t.Fatalf("notification args = %#v, want %#v", exec.args, wantArgs)
	}
	for i := range wantArgs {
		if exec.args[i] != wantArgs[i] {
			t.Fatalf("notification args = %#v, want %#v", exec.args, wantArgs)
		}
	}
}

func TestNotifierWrapsPublishError(t *testing.T) {
	errBoom := errors.New("postgres down")

	err := NewNotifier(&fakeNotificationExecutor{err: errBoom}).NotifyConfigActivated(context.Background(), 7)
	if !errors.Is(err, errBoom) || !strings.Contains(err.Error(), "notify config activated") {
		t.Fatalf("NotifyConfigActivated() error = %v, want wrapped publish error", err)
	}
}

func TestSubscriberReportsAcquireError(t *testing.T) {
	errBoom := errors.New("pool closed")
	subscriber := &Subscriber{acquirer: fakeNotificationAcquirer{err: errBoom}}

	_, err := subscriber.Subscribe(context.Background())
	if !errors.Is(err, errBoom) || !strings.Contains(err.Error(), "acquire postgres notification connection") {
		t.Fatalf("Subscribe() error = %v, want wrapped acquire error", err)
	}
}

func TestNewSubscriberCreatesPostgresAcquirer(t *testing.T) {
	subscriber := NewSubscriber(nil)
	if subscriber == nil || subscriber.acquirer == nil {
		t.Fatal("NewSubscriber() did not create subscriber acquirer")
	}
}

func TestSubscriberReportsListenErrorAndReleasesConnection(t *testing.T) {
	conn := &fakeNotificationConn{execErr: errors.New("listen denied")}
	subscriber := &Subscriber{acquirer: fakeNotificationAcquirer{conn: conn}}

	_, err := subscriber.Subscribe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "listen for config activation") {
		t.Fatalf("Subscribe() error = %v, want listen context", err)
	}
	if !conn.released.Load() {
		t.Fatal("Subscribe() did not release connection after listen failure")
	}
}

func TestSubscriberSkipsInvalidPayloadsAndPublishesValidRevisions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn := &fakeNotificationConn{
		notifications: []*pgconn.Notification{
			{Payload: "not-a-number"},
			{Payload: "84"},
		},
		waitErr: errors.New("done"),
	}
	subscriber := &Subscriber{acquirer: fakeNotificationAcquirer{conn: conn}}

	notifications, err := subscriber.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	got, ok := <-notifications
	if !ok {
		t.Fatal("notifications channel closed before valid revision")
	}
	if got != 84 {
		t.Fatalf("revision = %d, want 84", got)
	}
	_, ok = <-notifications
	if ok {
		t.Fatal("notifications channel remained open after wait error")
	}
	if !conn.released.Load() {
		t.Fatal("Subscribe() did not release connection when listener exited")
	}
}

type fakeNotificationExecutor struct {
	sql  string
	args []any
	err  error
}

func (e *fakeNotificationExecutor) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	e.sql = sql
	e.args = append([]any(nil), args...)
	return pgconn.CommandTag{}, e.err
}

type fakeNotificationAcquirer struct {
	conn notificationConn
	err  error
}

func (a fakeNotificationAcquirer) Acquire(context.Context) (notificationConn, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.conn, nil
}

type fakeNotificationConn struct {
	execErr       error
	waitErr       error
	notifications []*pgconn.Notification
	released      atomic.Bool
}

func (c *fakeNotificationConn) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, c.execErr
}

func (c *fakeNotificationConn) Release() {
	c.released.Store(true)
}

func (c *fakeNotificationConn) WaitForNotification(context.Context) (*pgconn.Notification, error) {
	if len(c.notifications) == 0 {
		if c.waitErr != nil {
			return nil, c.waitErr
		}
		return nil, errors.New("no more notifications")
	}
	notification := c.notifications[0]
	c.notifications = c.notifications[1:]
	return notification, nil
}
