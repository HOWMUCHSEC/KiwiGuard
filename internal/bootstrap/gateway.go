package bootstrap

import (
	"context"
	"net/http"

	gatewayassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/gateway"
)

// GatewayHandler builds a gateway handler from active runtime config.
func (f *Factory) GatewayHandler(ctx context.Context) (http.Handler, Cleanup, error) {
	if err := f.validateRequiredDependencies(); err != nil {
		return nil, nil, err
	}
	if f.options.Repository != nil {
		compiled, err := f.loadCompiledRuntime(ctx, nil, nil)
		if err != nil {
			return nil, nil, err
		}
		handler, err := gatewayassembly.BuildCompiledHandler(compiled)
		if err != nil {
			return nil, nil, err
		}
		return f.gatewayHandler(handler), noopCleanup, nil
	}

	deps, cleanup, err := f.productionDeps(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	writer := f.metrics.WrapWriter(f.telemetry.WrapWriter(deps.eventPipeline))
	compiler := f.compilerWith(writer, deps.eventGate)
	state, err := f.newRuntimeState(ctx, deps.runtimeRepo, compiler)
	if err != nil {
		_ = cleanup(ctx)
		return nil, nil, err
	}
	cancelWatcher, watcherDone := startRuntimeWatcher(f.newRuntimeWatcher(deps.runtimeRepo, compiler, state, deps.subscriber))
	return f.gatewayHandler(gatewayassembly.BuildStateHandler(state)), func(cleanupCtx context.Context) error {
		cancelWatcher()
		<-watcherDone
		return cleanup(cleanupCtx)
	}, nil
}
