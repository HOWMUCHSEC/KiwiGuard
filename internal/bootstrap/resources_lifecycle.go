package bootstrap

import (
	"context"
	"time"

	eventassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/events"
)

// openProductionDeps opens shared resources, optional pipelines, and health monitoring.
func (f *Factory) openProductionDeps(ctx context.Context, withEventPipeline bool) (*productionDeps, Cleanup, error) {
	resources, err := f.openProductionResources(ctx)
	if err != nil {
		return nil, nil, err
	}

	eventSpool, err := eventassembly.NewFileSpool(f.options.Config)
	if err != nil {
		_ = closeProductionResources(resources)
		return nil, nil, err
	}

	deps := f.newProductionDeps(resources, eventSpool)
	if withEventPipeline {
		if err := f.startTrafficEventPipeline(ctx, deps, resources.clickhouseConn); err != nil {
			_ = deps.cleanup(ctx)
			return nil, nil, err
		}
	}
	deps.startClickHouseHealthMonitor(resources.clickhouseConn, 5*time.Second)

	return deps, deps.cleanup, nil
}
