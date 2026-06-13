package bootstrap

import (
	"context"
	"fmt"

	kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"
)

// clickHouseExecutor captures the ClickHouse exec behavior required by retention maintenance.
type clickHouseExecutor interface {
	Exec(context.Context, string, ...any) error
}

// clickHouseRetentionMaintainer applies the runtime's shortest retention window to ClickHouse.
type clickHouseRetentionMaintainer struct {
	executor clickHouseExecutor
}

// newClickHouseRetentionMaintainer builds a retention maintainer around a ClickHouse executor.
func newClickHouseRetentionMaintainer(executor clickHouseExecutor) kgruntime.RetentionMaintainer {
	return clickHouseRetentionMaintainer{executor: executor}
}

// ApplyRetentionPolicies enforces the shortest enabled retention policy on the traffic table.
func (m clickHouseRetentionMaintainer) ApplyRetentionPolicies(ctx context.Context, cfg kgruntime.RuntimeConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.executor == nil {
		return fmt.Errorf("apply clickhouse retention policies: executor is required")
	}

	days, ok, err := shortestClickHouseRetentionDays(cfg)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	query := fmt.Sprintf("ALTER TABLE kiwiguard_traffic_events MODIFY TTL toDateTime(event_time) + INTERVAL %d DAY", days)
	if err := m.executor.Exec(ctx, query); err != nil {
		return fmt.Errorf("apply clickhouse retention policies: %w", err)
	}
	return nil
}

// shortestClickHouseRetentionDays resolves the most restrictive enabled ClickHouse retention window.
func shortestClickHouseRetentionDays(cfg kgruntime.RuntimeConfig) (int, bool, error) {
	sinks := make(map[string]kgruntime.SinkConfig, len(cfg.Sinks))
	for _, sink := range cfg.Sinks {
		if sink.Disabled {
			continue
		}
		sinks[sink.Key] = sink
	}

	shortest := 0
	for _, policy := range cfg.Retention {
		if policy.RetentionDays <= 0 {
			return 0, false, fmt.Errorf("retention policy %q has invalid retention days %d", policy.Key, policy.RetentionDays)
		}
		if policy.SinkKey == "" {
			continue
		}
		sink, ok := sinks[policy.SinkKey]
		if !ok {
			return 0, false, fmt.Errorf("retention policy %q references unknown sink %q", policy.Key, policy.SinkKey)
		}
		if sink.Kind != "clickhouse" {
			return 0, false, fmt.Errorf("unsupported retention sink %q kind %q", sink.Key, sink.Kind)
		}
		if shortest == 0 || policy.RetentionDays < shortest {
			shortest = policy.RetentionDays
		}
	}
	return shortest, shortest > 0, nil
}
