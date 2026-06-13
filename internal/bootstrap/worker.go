package bootstrap

import (
	"context"

	workerassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/worker"
	kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"
)

// Worker builds the background worker.
func (f *Factory) Worker(ctx context.Context) (kgruntime.Runner, Cleanup, error) {
	if err := f.validateRequiredDependencies(); err != nil {
		return nil, nil, err
	}
	if f.options.Repository != nil {
		compiler := f.compiler()
		state, err := f.newRuntimeState(ctx, f.options.Repository, compiler)
		if err != nil {
			return nil, nil, err
		}
		worker := workerassembly.Build(kgruntime.WorkerOptions{
			Watcher:    f.newRuntimeWatcher(f.options.Repository, compiler, state, f.options.Subscriber),
			Repository: f.options.Repository,
		})
		return worker, noopCleanup, nil
	}

	deps, cleanup, err := f.productionDeps(ctx, false)
	if err != nil {
		return nil, nil, err
	}
	compiler := f.compilerWith(nil, deps.eventGate)
	state, err := f.newRuntimeState(ctx, deps.runtimeRepo, compiler)
	if err != nil {
		_ = cleanup(ctx)
		return nil, nil, err
	}
	worker := workerassembly.Build(kgruntime.WorkerOptions{
		Watcher:             f.newRuntimeWatcher(deps.runtimeRepo, compiler, state, deps.subscriber),
		Repository:          deps.runtimeRepo,
		RetentionMaintainer: deps.retentionMaintainer,
		AuditMaintainer:     deps.auditMaintainer,
	})
	return worker, cleanup, nil
}
