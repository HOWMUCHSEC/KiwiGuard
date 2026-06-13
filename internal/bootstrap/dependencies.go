package bootstrap

import (
	"context"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	controlhttp "github.com/howmuchsec/kiwiguard/internal/contexts/control/adapters/httpapi"
	kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
)

// productionDepsOpener opens the shared infrastructure needed by production runtime modes.
type productionDepsOpener func(context.Context, bool) (*productionDeps, Cleanup, error)

// productionDeps groups long-lived infrastructure reused across runtime modes.
type productionDeps struct {
	pool                postgresPool
	clickhouseConn      closeable
	controlStore        controlhttp.PolicyStore
	runtimeRepo         kgruntime.RuntimeConfigRepository
	notifier            controlhttp.ActivationNotifier
	subscriber          kgruntime.RevisionSubscriber
	eventSpool          *events.FileSpool
	eventPipeline       *events.Pipeline
	eventGate           *events.HealthGate
	spoolStatus         controlhttp.SpoolStatusProvider
	retentionMaintainer kgruntime.RetentionMaintainer
	auditMaintainer     kgruntime.AuditMaintainer
	cancelPipeline      context.CancelFunc
	pipelineDone        <-chan struct{}
	cancelReplay        context.CancelFunc
	replayDone          <-chan struct{}
	cancelMonitor       context.CancelFunc
	monitorDone         <-chan struct{}
}

// postgresPool captures the pgx pool behavior required by bootstrap lifecycle code.
type postgresPool interface {
	Close()
	Ping(context.Context) error
}

// closeable captures the ClickHouse connection behavior required by bootstrap.
type closeable interface {
	Close() error
	Query(context.Context, string, ...any) (chdriver.Rows, error)
}

// productionDeps returns injected production dependencies or opens the default set.
func (f *Factory) productionDeps(ctx context.Context, withEventPipeline bool) (*productionDeps, Cleanup, error) {
	if f.options.productionDeps != nil {
		return f.options.productionDeps(ctx, withEventPipeline)
	}
	return f.openProductionDeps(ctx, withEventPipeline)
}
