package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
)

func TestClickHouseAuditMaintainerUpdatesGateHealth(t *testing.T) {
	gate := events.NewHealthGate()
	probe := &flappingClickHouseProbe{err: errors.New("clickhouse down")}
	maintainer := clickHouseAuditMaintainer{probe: probe, gate: gate}

	if err := maintainer.ValidateAuditState(context.Background()); err != nil {
		t.Fatalf("ValidateAuditState() error = %v", err)
	}
	if gate.Healthy() {
		t.Fatal("gate is healthy, want unhealthy after failed audit validation")
	}

	probe.setErr(nil)
	if err := maintainer.ValidateAuditState(context.Background()); err != nil {
		t.Fatalf("ValidateAuditState() recovery error = %v", err)
	}
	if !gate.Healthy() {
		t.Fatal("gate is unhealthy, want healthy after recovered audit validation")
	}
}
