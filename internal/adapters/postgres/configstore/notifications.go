package configstore

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConfigActivatedChannel is the PostgreSQL channel for config activation events.
const ConfigActivatedChannel = "kiwiguard_config_activated"

// Notifier publishes PostgreSQL config activation notifications.
type Notifier struct {
	pool notificationExecutor
}

// NewNotifier builds the publisher that emits PostgreSQL activation notifications.
func NewNotifier(pool notificationExecutor) *Notifier {
	return &Notifier{pool: pool}
}

// NotifyConfigActivated publishes the activated revision number.
func (n *Notifier) NotifyConfigActivated(ctx context.Context, revisionNumber int64) error {
	if _, err := n.pool.Exec(ctx, `select pg_notify($1, $2)`, ConfigActivatedChannel, strconv.FormatInt(revisionNumber, 10)); err != nil {
		return fmt.Errorf("notify config activated: %w", err)
	}
	return nil
}

// Subscriber listens for PostgreSQL config activation notifications.
type Subscriber struct {
	acquirer notificationAcquirer
}

// NewSubscriber builds the listener that consumes PostgreSQL activation notifications.
func NewSubscriber(pool *pgxpool.Pool) *Subscriber {
	return &Subscriber{acquirer: pgxNotificationAcquirer{pool: pool}}
}

// Subscribe starts listening for activated config revision numbers.
func (s *Subscriber) Subscribe(ctx context.Context) (<-chan int64, error) {
	conn, err := s.acquirer.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire postgres notification connection: %w", err)
	}
	if _, err := conn.Exec(ctx, "listen "+pgx.Identifier{ConfigActivatedChannel}.Sanitize()); err != nil {
		conn.Release()
		return nil, fmt.Errorf("listen for config activation: %w", err)
	}

	notifications := make(chan int64, 1)
	go func() {
		defer conn.Release()
		defer close(notifications)
		for {
			notification, err := conn.WaitForNotification(ctx)
			if err != nil {
				return
			}
			revision, err := strconv.ParseInt(notification.Payload, 10, 64)
			if err != nil {
				continue
			}
			select {
			case notifications <- revision:
			case <-ctx.Done():
				return
			}
		}
	}()
	return notifications, nil
}

type notificationExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type notificationAcquirer interface {
	Acquire(context.Context) (notificationConn, error)
}

type notificationConn interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Release()
	WaitForNotification(context.Context) (*pgconn.Notification, error)
}

type pgxNotificationAcquirer struct {
	pool *pgxpool.Pool
}

func (a pgxNotificationAcquirer) Acquire(ctx context.Context) (notificationConn, error) {
	conn, err := a.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	return pgxNotificationConn{conn: conn}, nil
}

type pgxNotificationConn struct {
	conn *pgxpool.Conn
}

func (c pgxNotificationConn) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return c.conn.Exec(ctx, sql, args...)
}

func (c pgxNotificationConn) Release() {
	c.conn.Release()
}

func (c pgxNotificationConn) WaitForNotification(ctx context.Context) (*pgconn.Notification, error) {
	return c.conn.Conn().WaitForNotification(ctx)
}
