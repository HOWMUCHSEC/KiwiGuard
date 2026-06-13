package bootstrap

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/clickhouse"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
)

// clickHouseAuditMaintainer probes ClickHouse audit readiness for fail-closed gateway checks.
type clickHouseAuditMaintainer struct {
	probe clickhouse.Probe
	gate  *events.HealthGate
}

// ValidateAuditState updates the shared audit health gate from a ClickHouse readiness probe.
func (m clickHouseAuditMaintainer) ValidateAuditState(ctx context.Context) error {
	if err := clickhouse.ProbeHealth(ctx, m.probe); err != nil {
		if m.gate != nil {
			m.gate.MarkUnhealthy("clickhouse_unhealthy")
		}
		return nil
	}
	if m.gate != nil {
		m.gate.MarkHealthy()
	}
	return nil
}
