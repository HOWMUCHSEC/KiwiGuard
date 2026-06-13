package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWatcherReloadsOnRevisionChange(t *testing.T) {
	repo := &fakeRuntimeRepository{
		revision: 2,
		config:   RuntimeConfig{Revision: RuntimeRevision{Number: 2}},
	}
	compiler := fakeRuntimeCompiler{
		compiled: compiledRuntimeForTest(2, "two"),
	}
	state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}
	health := NewReadinessState()
	watcher := NewWatcher(WatcherOptions{
		PollInterval: time.Millisecond,
		Repository:   repo,
		Compiler:     compiler,
		State:        state,
		Health:       health,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- watcher.Run(ctx)
	}()

	waitForRuntimeRevision(t, state, 2)
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !health.ConfigReady() {
		t.Fatal("config readiness is degraded after successful reload")
	}
}

func TestWatcherKeepsPreviousRuntimeWhenCompileFails(t *testing.T) {
	repo := &fakeRuntimeRepository{
		revision: 2,
		config:   RuntimeConfig{Revision: RuntimeRevision{Number: 2}},
	}
	state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}
	health := NewReadinessState()
	watcher := NewWatcher(WatcherOptions{
		PollInterval: time.Millisecond,
		Repository:   repo,
		Compiler:     fakeRuntimeCompiler{err: errors.New("compile failed")},
		State:        state,
		Health:       health,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := watcher.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := state.Snapshot().RevisionNumber; got != 1 {
		t.Fatalf("Snapshot().RevisionNumber = %d, want previous revision 1", got)
	}
	if health.ConfigReady() {
		t.Fatal("config readiness is healthy after compile failure")
	}
}

func TestWatcherFallsBackToPollingWhenNotificationChannelCloses(t *testing.T) {
	repo := &countingRuntimeRepository{
		fakeRuntimeRepository: fakeRuntimeRepository{
			revision: 1,
			config:   RuntimeConfig{Revision: RuntimeRevision{Number: 1}},
		},
	}
	state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}
	health := NewReadinessState()
	watcher := NewWatcher(WatcherOptions{
		PollInterval: time.Hour,
		Repository:   repo,
		Compiler:     fakeRuntimeCompiler{compiled: compiledRuntimeForTest(1, "one")},
		State:        state,
		Health:       health,
		Subscriber:   closedSubscriber{},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if err := watcher.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if repo.calls > 2 {
		t.Fatalf("ActiveRevisionNumber calls = %d, want no tight loop after notification channel closes", repo.calls)
	}
	if health.ConfigReady() {
		t.Fatal("config readiness is healthy after notification channel closed")
	}
}

func TestWatcherMarksSubscribeFailureButStillReloads(t *testing.T) {
	repo := &fakeRuntimeRepository{
		revision: 2,
		config:   RuntimeConfig{Revision: RuntimeRevision{Number: 2}},
	}
	state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}
	health := NewReadinessState()
	watcher := NewWatcher(WatcherOptions{
		PollInterval: time.Hour,
		Repository:   repo,
		Compiler:     fakeRuntimeCompiler{compiled: compiledRuntimeForTest(2, "two")},
		State:        state,
		Health:       health,
		Subscriber:   errorSubscriber{err: errors.New("listen failed")},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := watcher.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := state.Snapshot().RevisionNumber; got != 2 {
		t.Fatalf("Snapshot().RevisionNumber = %d, want reloaded revision 2", got)
	}
	if !health.ConfigReady() {
		t.Fatal("config readiness is degraded after successful reload")
	}
}

func TestWatcherReloadsWhenNotificationArrives(t *testing.T) {
	notifications := make(chan int64, 1)
	repo := &fakeRuntimeRepository{
		revision: 2,
		config:   RuntimeConfig{Revision: RuntimeRevision{Number: 2}},
	}
	state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}
	watcher := NewWatcher(WatcherOptions{
		PollInterval: time.Hour,
		Repository:   repo,
		Compiler:     fakeRuntimeCompiler{compiled: compiledRuntimeForTest(2, "two")},
		State:        state,
		Subscriber:   channelSubscriber{notifications: notifications},
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- watcher.Run(ctx)
	}()
	notifications <- 2

	waitForRuntimeRevision(t, state, 2)
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestWatcherReloadIfChangedRequiresDependencies(t *testing.T) {
	state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}
	tests := []struct {
		name    string
		watcher *Watcher
	}{
		{
			name: "repository",
			watcher: NewWatcher(WatcherOptions{
				Compiler: fakeRuntimeCompiler{compiled: compiledRuntimeForTest(1, "one")},
				State:    state,
			}),
		},
		{
			name: "compiler",
			watcher: NewWatcher(WatcherOptions{
				Repository: &fakeRuntimeRepository{revision: 1},
				State:      state,
			}),
		},
		{
			name: "state",
			watcher: NewWatcher(WatcherOptions{
				Repository: &fakeRuntimeRepository{revision: 1},
				Compiler:   fakeRuntimeCompiler{compiled: compiledRuntimeForTest(1, "one")},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.watcher.reloadIfChanged(context.Background()); err == nil {
				t.Fatal("reloadIfChanged() error = nil, want missing dependency error")
			}
		})
	}
}

func TestWatcherMarksLoadAndSwapFailuresDegraded(t *testing.T) {
	tests := []struct {
		name       string
		repo       RuntimeConfigRepository
		compiler   RuntimeCompiler
		wantReason string
	}{
		{
			name: "load",
			repo: &loadErrorRuntimeRepository{
				revision: 2,
				err:      errors.New("load failed"),
			},
			compiler:   fakeRuntimeCompiler{compiled: compiledRuntimeForTest(2, "two")},
			wantReason: "runtime_config_load_failed",
		},
		{
			name: "swap",
			repo: &fakeRuntimeRepository{
				revision: 2,
				config:   RuntimeConfig{Revision: RuntimeRevision{Number: 2}},
			},
			compiler:   fakeRuntimeCompiler{compiled: CompiledRuntime{RevisionNumber: 2}},
			wantReason: "runtime_config_swap_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
			if err != nil {
				t.Fatalf("NewActiveRuntimeState() error = %v", err)
			}
			health := NewReadinessState()
			watcher := NewWatcher(WatcherOptions{
				Repository: tt.repo,
				Compiler:   tt.compiler,
				State:      state,
				Health:     health,
			})

			if err := watcher.reloadIfChanged(context.Background()); err != nil {
				t.Fatalf("reloadIfChanged() error = %v", err)
			}
			if health.ConfigReady() {
				t.Fatal("config readiness is healthy after reload failure")
			}
			if got := health.Reason(); got != tt.wantReason {
				t.Fatalf("Reason() = %q, want %q", got, tt.wantReason)
			}
			if got := state.Snapshot().RevisionNumber; got != 1 {
				t.Fatalf("Snapshot().RevisionNumber = %d, want previous revision 1", got)
			}
		})
	}
}

func TestWatcherMarksActiveRevisionFailureDegraded(t *testing.T) {
	state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}
	health := NewReadinessState()
	watcher := NewWatcher(WatcherOptions{
		Repository: &revisionErrorRuntimeRepository{err: errors.New("revision unavailable")},
		Compiler:   fakeRuntimeCompiler{compiled: compiledRuntimeForTest(2, "two")},
		State:      state,
		Health:     health,
	})

	if err := watcher.reloadIfChanged(context.Background()); err != nil {
		t.Fatalf("reloadIfChanged() error = %v", err)
	}
	if health.ConfigReady() {
		t.Fatal("config readiness is healthy after active revision failure")
	}
	if got := health.Reason(); got != "active_revision_unavailable" {
		t.Fatalf("Reason() = %q, want active_revision_unavailable", got)
	}
	if got := state.Snapshot().RevisionNumber; got != 1 {
		t.Fatalf("Snapshot().RevisionNumber = %d, want previous revision 1", got)
	}
}

func TestWatcherSkipsReloadWhenRevisionUnchanged(t *testing.T) {
	repo := &countingRuntimeRepository{
		fakeRuntimeRepository: fakeRuntimeRepository{
			revision: 1,
			config:   RuntimeConfig{Revision: RuntimeRevision{Number: 1}},
		},
	}
	compiler := &countingRuntimeCompiler{compiled: compiledRuntimeForTest(2, "two")}
	state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}
	health := NewReadinessState()
	watcher := NewWatcher(WatcherOptions{
		Repository: repo,
		Compiler:   compiler,
		State:      state,
		Health:     health,
	})

	if err := watcher.reloadIfChanged(context.Background()); err != nil {
		t.Fatalf("reloadIfChanged() error = %v", err)
	}

	if repo.loadCalls != 0 {
		t.Fatalf("LoadRuntimeConfig calls = %d, want 0", repo.loadCalls)
	}
	if compiler.calls != 0 {
		t.Fatalf("CompileRuntime calls = %d, want 0", compiler.calls)
	}
	if got := state.Snapshot().RevisionNumber; got != 1 {
		t.Fatalf("Snapshot().RevisionNumber = %d, want unchanged revision 1", got)
	}
	if !health.ConfigReady() {
		t.Fatalf("config readiness degraded unexpectedly: %s", health.Reason())
	}
}

type fakeRuntimeRepository struct {
	revision int64
	config   RuntimeConfig
}

func (r *fakeRuntimeRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	return r.revision, nil
}

func (r *fakeRuntimeRepository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	return r.config, nil
}

type countingRuntimeRepository struct {
	fakeRuntimeRepository
	calls     int
	loadCalls int
}

func (r *countingRuntimeRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	r.calls++
	return r.fakeRuntimeRepository.ActiveRevisionNumber(ctx)
}

func (r *countingRuntimeRepository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	r.loadCalls++
	return r.fakeRuntimeRepository.LoadRuntimeConfig(ctx)
}

type loadErrorRuntimeRepository struct {
	revision int64
	err      error
}

func (r *loadErrorRuntimeRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	return r.revision, nil
}

func (r *loadErrorRuntimeRepository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	return RuntimeConfig{}, r.err
}

type revisionErrorRuntimeRepository struct {
	err error
}

func (r *revisionErrorRuntimeRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	return 0, r.err
}

func (r *revisionErrorRuntimeRepository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	return RuntimeConfig{}, nil
}

type closedSubscriber struct{}

func (closedSubscriber) Subscribe(ctx context.Context) (<-chan int64, error) {
	ch := make(chan int64)
	close(ch)
	return ch, nil
}

type errorSubscriber struct {
	err error
}

func (s errorSubscriber) Subscribe(ctx context.Context) (<-chan int64, error) {
	return nil, s.err
}

type channelSubscriber struct {
	notifications <-chan int64
}

func (s channelSubscriber) Subscribe(ctx context.Context) (<-chan int64, error) {
	return s.notifications, nil
}

type fakeRuntimeCompiler struct {
	compiled CompiledRuntime
	err      error
}

func (c fakeRuntimeCompiler) CompileRuntime(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
	if c.err != nil {
		return CompiledRuntime{}, c.err
	}
	return c.compiled, nil
}

type countingRuntimeCompiler struct {
	compiled CompiledRuntime
	err      error
	calls    int
}

func (c *countingRuntimeCompiler) CompileRuntime(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
	c.calls++
	if c.err != nil {
		return CompiledRuntime{}, c.err
	}
	return c.compiled, nil
}

func waitForRuntimeRevision(t *testing.T, state *ActiveRuntimeState, revision int64) {
	t.Helper()
	for range 50 {
		if state.Snapshot().RevisionNumber == revision {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("runtime revision = %d, want %d", state.Snapshot().RevisionNumber, revision)
}
