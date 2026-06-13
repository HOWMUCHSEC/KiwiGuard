package runtime

import (
	"context"
	"sync"
	"time"

	appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"
)

const defaultWorkerMaintainInterval = time.Minute

// Runner runs until its context is canceled or it fails.
type Runner = interface {
	Run(context.Context) error
}

// WorkerOptions groups the background jobs that keep runtime state and retention policies up to date.
type WorkerOptions struct {
	Watcher             Runner
	Repository          RuntimeConfigRepository
	RetentionMaintainer RetentionMaintainer
	AuditMaintainer     AuditMaintainer
	MaintainInterval    time.Duration
}

// RetentionMaintainer applies active retention policies.
type RetentionMaintainer interface {
	ApplyRetentionPolicies(context.Context, RuntimeConfig) error
}

// AuditMaintainer validates audit persistence state.
type AuditMaintainer interface {
	ValidateAuditState(context.Context) error
}

// Worker runs runtime background maintenance.
type Worker struct {
	options WorkerOptions
}

// NewWorker builds the background runtime maintenance worker.
func NewWorker(opts WorkerOptions) *Worker {
	if opts.MaintainInterval <= 0 {
		opts.MaintainInterval = defaultWorkerMaintainInterval
	}
	return &Worker{options: opts}
}

// Run starts config watching and maintenance loops.
func (w *Worker) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 3)
	var wg sync.WaitGroup
	if w.options.Watcher != nil {
		wg.Go(func() {
			errCh <- w.options.Watcher.Run(ctx)
		})
	}
	if w.options.RetentionMaintainer != nil && w.options.Repository != nil {
		wg.Go(func() {
			errCh <- runMaintenanceLoop(ctx, w.options.MaintainInterval, w.applyRetentionPolicies)
		})
	}
	if w.options.AuditMaintainer != nil {
		wg.Go(func() {
			errCh <- runMaintenanceLoop(ctx, w.options.MaintainInterval, w.options.AuditMaintainer.ValidateAuditState)
		})
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(errCh)
		close(done)
	}()

	for {
		select {
		case <-ctx.Done():
			cancel()
			<-done
			return nil
		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			if err != nil {
				cancel()
				<-done
				return err
			}
		}
	}
}

func runMaintenanceLoop(ctx context.Context, interval time.Duration, fn func(context.Context) error) error {
	if ctx.Err() != nil {
		return nil
	}
	if err := fn(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				return err
			}
		}
	}
}

func (w *Worker) applyRetentionPolicies(ctx context.Context) error {
	return appruntime.RetentionMaintainer{
		Repository: w.options.Repository,
		Applier:    w.options.RetentionMaintainer,
	}.Apply(ctx)
}
