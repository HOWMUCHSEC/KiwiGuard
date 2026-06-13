package runtime

import (
	"context"
	"fmt"
	"time"

	appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"
)

const defaultWatcherPollInterval = 30 * time.Second

// RevisionSubscriber receives active revision notifications.
type RevisionSubscriber interface {
	Subscribe(context.Context) (<-chan int64, error)
}

// WatcherOptions defines how runtime revisions are discovered, compiled, and published into memory.
type WatcherOptions struct {
	PollInterval time.Duration
	Subscriber   RevisionSubscriber
	Repository   RuntimeConfigRepository
	Compiler     RuntimeCompiler
	State        *ActiveRuntimeState
	Health       *ReadinessState
}

// Watcher reloads active runtime config into memory.
type Watcher struct {
	options WatcherOptions
}

// NewWatcher builds a watcher that reloads runtime state on notifications and poll intervals.
func NewWatcher(opts WatcherOptions) *Watcher {
	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultWatcherPollInterval
	}
	return &Watcher{options: opts}
}

// Run watches active runtime revision changes until ctx is canceled.
func (w *Watcher) Run(ctx context.Context) error {
	notifications, err := w.subscribe(ctx)
	if err != nil && w.options.Health != nil {
		w.options.Health.MarkConfigDegraded("config_subscribe_failed")
	}
	if err := w.reloadIfChanged(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(w.options.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-notifications:
			if !ok {
				notifications = nil
				if w.options.Health != nil {
					w.options.Health.MarkConfigDegraded("config_subscriber_closed")
				}
				continue
			}
			if err := w.reloadIfChanged(ctx); err != nil {
				return err
			}
		case <-ticker.C:
			if err := w.reloadIfChanged(ctx); err != nil {
				return err
			}
		}
	}
}

func (w *Watcher) subscribe(ctx context.Context) (<-chan int64, error) {
	if w.options.Subscriber == nil {
		return nil, nil
	}
	notifications, err := w.options.Subscriber.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("subscribe runtime revision changes: %w", err)
	}
	return notifications, nil
}

func (w *Watcher) reloadIfChanged(ctx context.Context) error {
	return appruntime.Reloader{
		Repository: w.options.Repository,
		Compiler:   w.options.Compiler,
		State:      w.options.State,
		Readiness:  w.options.Health,
	}.ReloadIfChanged(ctx)
}
