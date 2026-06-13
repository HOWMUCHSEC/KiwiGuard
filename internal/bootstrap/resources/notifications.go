package resources

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ActivationNotifier publishes active configuration revision changes.
type ActivationNotifier interface {
	NotifyConfigActivated(context.Context, int64) error
}

// RevisionSubscriber receives active configuration revision changes.
type RevisionSubscriber interface {
	Subscribe(context.Context) (<-chan int64, error)
}

// Notifications contains PostgreSQL-backed revision notification ports.
type Notifications struct {
	Notifier   ActivationNotifier
	Subscriber RevisionSubscriber
}

// NewNotifications builds PostgreSQL-backed revision notification adapters.
func NewNotifications(pool *pgxpool.Pool) Notifications {
	return Notifications{
		Notifier:   configstore.NewNotifier(pool),
		Subscriber: configstore.NewSubscriber(pool),
	}
}
