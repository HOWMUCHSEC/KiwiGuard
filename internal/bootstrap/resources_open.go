package bootstrap

import (
	"context"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	resourceassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/resources"
	controlhttp "github.com/howmuchsec/kiwiguard/internal/contexts/control/adapters/httpapi"
	kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
)

// productionResourceOpener opens infrastructure handles before runtime assembly.
type productionResourceOpener func(context.Context) (productionResources, error)

// productionResources holds shared infrastructure handles before bootstrap wraps them.
type productionResources struct {
	pool           *pgxpool.Pool
	clickhouseConn ch.Conn
	controlStore   controlhttp.PolicyStore
	runtimeRepo    kgruntime.RuntimeConfigRepository
}

// openProductionResources opens shared infrastructure connections before assembly.
func (f *Factory) openProductionResources(ctx context.Context) (productionResources, error) {
	if f.options.productionResources != nil {
		return f.options.productionResources(ctx)
	}

	resources, err := resourceassembly.OpenProduction(ctx, f.options.Config)
	if err != nil {
		return productionResources{}, err
	}

	return productionResources{
		pool:           resources.Pool,
		clickhouseConn: resources.ClickHouseConn,
		controlStore:   resources.ControlStore,
		runtimeRepo:    resources.RuntimeRepo,
	}, nil
}
