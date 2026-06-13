package bootstrap

import (
	resourceassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/resources"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
)

// newProductionDeps converts raw infrastructure handles into runtime-facing dependencies.
func (f *Factory) newProductionDeps(resources productionResources, eventSpool *events.FileSpool) *productionDeps {
	var pool postgresPool
	if resources.pool != nil {
		pool = resources.pool
	}
	notifications := resourceassembly.NewNotifications(resources.pool)
	deps := &productionDeps{
		pool:           pool,
		clickhouseConn: resources.clickhouseConn,
		controlStore:   resources.controlStore,
		runtimeRepo:    resources.runtimeRepo,
		notifier:       notifications.Notifier,
		subscriber:     notifications.Subscriber,
		eventSpool:     eventSpool,
		eventGate:      events.NewHealthGate(),
		spoolStatus:    newFileSpoolStatusProviderFromSpool(eventSpool, f.metrics),
	}
	deps.retentionMaintainer = newClickHouseRetentionMaintainer(resources.clickhouseConn)
	deps.auditMaintainer = clickHouseAuditMaintainer{probe: resources.clickhouseConn, gate: deps.eventGate}
	return deps
}
