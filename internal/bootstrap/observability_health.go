package bootstrap

import (
	"time"

	observabilityassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/observability"
)

// startClickHouseHealthMonitor starts the shared ClickHouse readiness poller for event health.
func (d *productionDeps) startClickHouseHealthMonitor(probe observabilityassembly.ClickHouseProbe, interval time.Duration) {
	cancelMonitor, monitorDone := observabilityassembly.StartClickHouseHealthMonitor(probe, d.eventGate, interval)
	d.cancelMonitor = cancelMonitor
	d.monitorDone = monitorDone
}
