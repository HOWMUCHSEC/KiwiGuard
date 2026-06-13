package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerRunStopsOnContextCancel(t *testing.T) {
	watcher := watcherRunnerFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	})
	worker := NewWorker(WorkerOptions{
		Watcher:          watcher,
		MaintainInterval: time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- worker.Run(ctx)
	}()
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}
}

func TestWorkerRunWithoutWatcherStopsOnContextCancel(t *testing.T) {
	worker := NewWorker(WorkerOptions{MaintainInterval: time.Hour})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- worker.Run(ctx)
	}()
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() without watcher did not stop after context cancellation")
	}
}

func TestWorkerRunReturnsWatcherError(t *testing.T) {
	wantErr := errors.New("watch failed")
	worker := NewWorker(WorkerOptions{
		Watcher: watcherRunnerFunc(func(ctx context.Context) error {
			return wantErr
		}),
		MaintainInterval: time.Hour,
	})

	err := worker.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestWorkerRunReturnsRetentionMaintainerError(t *testing.T) {
	wantErr := errors.New("retention failed")
	worker := NewWorker(WorkerOptions{
		Repository:          workerRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 9}}},
		RetentionMaintainer: &recordingRetentionMaintainer{err: wantErr},
		MaintainInterval:    time.Hour,
	})

	err := worker.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestWorkerRunReturnsRetentionRepositoryError(t *testing.T) {
	wantErr := errors.New("runtime config load failed")
	worker := NewWorker(WorkerOptions{
		Repository:          failingWorkerRuntimeRepository{err: wantErr},
		RetentionMaintainer: &recordingRetentionMaintainer{},
		MaintainInterval:    time.Hour,
	})

	err := worker.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestWorkerRunReturnsAuditMaintainerError(t *testing.T) {
	wantErr := errors.New("audit failed")
	worker := NewWorker(WorkerOptions{
		AuditMaintainer:  &recordingAuditMaintainer{err: wantErr},
		MaintainInterval: time.Hour,
	})

	err := worker.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestWorkerRunWithNoBackgroundTasksReturnsNil(t *testing.T) {
	worker := NewWorker(WorkerOptions{MaintainInterval: time.Hour})

	if err := worker.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func TestNewWorkerDefaultsMaintainInterval(t *testing.T) {
	worker := NewWorker(WorkerOptions{})
	if worker.options.MaintainInterval != defaultWorkerMaintainInterval {
		t.Fatalf("MaintainInterval = %v, want %v", worker.options.MaintainInterval, defaultWorkerMaintainInterval)
	}
}

func TestRunMaintenanceLoopReturnsFunctionError(t *testing.T) {
	wantErr := errors.New("maintenance failed")
	err := runMaintenanceLoop(context.Background(), time.Hour, func(ctx context.Context) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runMaintenanceLoop() error = %v, want %v", err, wantErr)
	}
}

func TestRunMaintenanceLoopReturnsNilWhenContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false

	err := runMaintenanceLoop(ctx, time.Hour, func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("runMaintenanceLoop() error = %v, want nil", err)
	}
	if called {
		t.Fatal("maintenance function was called with pre-canceled context")
	}
}

func TestRunMaintenanceLoopRunsUntilContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := 0
	err := runMaintenanceLoop(ctx, time.Millisecond, func(ctx context.Context) error {
		calls++
		if calls == 2 {
			cancel()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("runMaintenanceLoop() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("maintenance calls = %d, want 2", calls)
	}
}

func TestWorkerRunsRetentionMaintainerWithRuntimeConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := workerRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 9}}}
	maintainer := &recordingRetentionMaintainer{afterApply: cancel}
	worker := NewWorker(WorkerOptions{
		Repository:          repo,
		RetentionMaintainer: maintainer,
		MaintainInterval:    time.Hour,
	})

	if err := worker.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if maintainer.calls != 1 {
		t.Fatalf("retention maintainer calls = %d, want 1", maintainer.calls)
	}
	if maintainer.cfg.Revision.Number != 9 {
		t.Fatalf("retention config revision = %d, want 9", maintainer.cfg.Revision.Number)
	}
}

func TestWorkerRunsAuditMaintainer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	maintainer := &recordingAuditMaintainer{afterValidate: cancel}
	worker := NewWorker(WorkerOptions{
		AuditMaintainer:  maintainer,
		MaintainInterval: time.Hour,
	})

	if err := worker.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if maintainer.calls != 1 {
		t.Fatalf("audit maintainer calls = %d, want 1", maintainer.calls)
	}
}

type watcherRunnerFunc func(context.Context) error

func (f watcherRunnerFunc) Run(ctx context.Context) error {
	return f(ctx)
}

type workerRuntimeRepository struct {
	cfg RuntimeConfig
}

func (r workerRuntimeRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	return r.cfg.Revision.Number, nil
}

func (r workerRuntimeRepository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	return r.cfg, nil
}

type failingWorkerRuntimeRepository struct {
	err error
}

func (r failingWorkerRuntimeRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	return 0, r.err
}

func (r failingWorkerRuntimeRepository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	return RuntimeConfig{}, r.err
}

type recordingRetentionMaintainer struct {
	calls      int
	cfg        RuntimeConfig
	err        error
	afterApply func()
}

func (m *recordingRetentionMaintainer) ApplyRetentionPolicies(ctx context.Context, cfg RuntimeConfig) error {
	m.calls++
	m.cfg = cfg
	if m.afterApply != nil {
		m.afterApply()
	}
	return m.err
}

type recordingAuditMaintainer struct {
	calls         int
	err           error
	afterValidate func()
}

func (m *recordingAuditMaintainer) ValidateAuditState(ctx context.Context) error {
	m.calls++
	if m.afterValidate != nil {
		m.afterValidate()
	}
	return m.err
}
