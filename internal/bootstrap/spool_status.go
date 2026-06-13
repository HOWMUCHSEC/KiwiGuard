package bootstrap

import (
	"fmt"

	eventassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/events"
	"github.com/howmuchsec/kiwiguard/internal/config"
	controlhttp "github.com/howmuchsec/kiwiguard/internal/contexts/control/adapters/httpapi"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"github.com/howmuchsec/kiwiguard/internal/observability"
)

// fileSpoolStatusProvider reports live durable spool state to the control API.
type fileSpoolStatusProvider struct {
	spool   *events.FileSpool
	metrics *observability.PrometheusMetrics
}

// newFileSpoolStatusProvider opens a spool on demand when no shared spool instance exists.
func newFileSpoolStatusProvider(cfg config.Config, metrics *observability.PrometheusMetrics) (*fileSpoolStatusProvider, error) {
	spool, err := eventassembly.NewFileSpool(cfg)
	if err != nil {
		return nil, fmt.Errorf("create event spool status provider: %w", err)
	}
	return newFileSpoolStatusProviderFromSpool(spool, metrics), nil
}

// newFileSpoolStatusProviderFromSpool builds a status provider around a shared spool instance.
func newFileSpoolStatusProviderFromSpool(spool *events.FileSpool, metrics *observability.PrometheusMetrics) *fileSpoolStatusProvider {
	return &fileSpoolStatusProvider{spool: spool, metrics: metrics}
}

// SpoolStatus reports backlog and storage usage for the configured spool.
func (p *fileSpoolStatusProvider) SpoolStatus() controlhttp.SpoolStatus {
	if p == nil || p.spool == nil {
		return controlhttp.SpoolStatus{Enabled: false, Status: "disabled"}
	}
	stats := p.spool.Stats()
	p.metrics.ObserveSpoolStats(stats)
	status := controlhttp.SpoolStatus{
		Enabled:          true,
		Status:           "ok",
		Depth:            stats.Depth,
		Bytes:            stats.Bytes,
		MaxBytes:         stats.MaxBytes,
		OldestAgeSeconds: stats.OldestAge.Seconds(),
		OverflowCount:    stats.OverflowCount,
	}
	if stats.Depth > 0 {
		status.Status = "backlogged"
		status.Reason = "event_spool_backlog"
	}
	if stats.OverflowCount > 0 {
		status.Status = "degraded"
		status.Reason = "event_spool_overflow"
	}
	return status
}
