package bootstrap

import (
	"context"
	"fmt"

	kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"
	appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"
)

// newRuntimeState publishes the initially compiled runtime snapshot into hot-swappable state.
func (f *Factory) newRuntimeState(ctx context.Context, repo kgruntime.RuntimeConfigRepository, compiler kgruntime.RuntimeCompiler) (*kgruntime.ActiveRuntimeState, error) {
	compiled, err := f.loadCompiledRuntimeWithCompiler(ctx, repo, compiler)
	if err != nil {
		return nil, err
	}
	state, err := kgruntime.NewActiveRuntimeState(compiled)
	if err != nil {
		return nil, fmt.Errorf("create active runtime state: %w", err)
	}
	return state, nil
}

// loadCompiledRuntimeWithCompiler hydrates active runtime config and compiles it with the selected compiler.
func (f *Factory) loadCompiledRuntimeWithCompiler(ctx context.Context, repo kgruntime.RuntimeConfigRepository, compiler kgruntime.RuntimeCompiler) (kgruntime.CompiledRuntime, error) {
	if repo == nil {
		return kgruntime.CompiledRuntime{}, kgruntime.ErrActiveRuntimeConfigNotFound
	}
	if compiler == nil {
		compiler = f.compiler()
	}
	return appruntime.Loader{
		Repository: repo,
		Compiler:   compiler,
	}.LoadCompiledRuntime(ctx)
}

// newRuntimeWatcher builds a watcher that hot-reloads active runtime revisions.
func (f *Factory) newRuntimeWatcher(repo kgruntime.RuntimeConfigRepository, compiler kgruntime.RuntimeCompiler, state *kgruntime.ActiveRuntimeState, subscriber kgruntime.RevisionSubscriber) *kgruntime.Watcher {
	return kgruntime.NewWatcher(kgruntime.WatcherOptions{
		Repository: repo,
		Compiler:   compiler,
		State:      state,
		Health:     f.configHealth,
		Subscriber: subscriber,
	})
}

// startRuntimeWatcher runs the runtime watcher in the background and returns its cancel handle.
func startRuntimeWatcher(watcher *kgruntime.Watcher) (context.CancelFunc, <-chan struct{}) {
	watcherCtx, cancelWatcher := context.WithCancel(context.Background())
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		_ = watcher.Run(watcherCtx)
	}()
	return cancelWatcher, watcherDone
}
