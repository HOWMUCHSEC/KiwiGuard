// Package observability assembles production health monitoring loops.
package observability

import (
	"context"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/clickhouse"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
)

// ClickHouseProbe is the health probe interface required by the assembly loop.
type ClickHouseProbe = clickhouse.Probe

// StartClickHouseHealthMonitor periodically updates the traffic event health gate.
func StartClickHouseHealthMonitor(probe clickhouse.Probe, gate *events.HealthGate, interval time.Duration) (context.CancelFunc, <-chan struct{}) {
	monitorCtx, cancelMonitor := context.WithCancel(context.Background())
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		runClickHouseHealthMonitor(monitorCtx, gate, probe, interval)
	}()
	return cancelMonitor, monitorDone
}

func runClickHouseHealthMonitor(ctx context.Context, gate *events.HealthGate, probe clickhouse.Probe, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	check := func() {
		if err := clickhouse.ProbeHealth(ctx, probe); err != nil {
			gate.MarkUnhealthy("clickhouse_unhealthy")
			return
		}
		gate.MarkHealthy()
	}
	check()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
