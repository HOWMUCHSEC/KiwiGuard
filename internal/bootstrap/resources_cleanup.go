package bootstrap

import (
	"context"
	"fmt"
)

// cleanup stops background workers and closes infrastructure owned by productionDeps.
func (d *productionDeps) cleanup(ctx context.Context) error {
	if d.cancelPipeline != nil {
		d.cancelPipeline()
	}
	if d.cancelReplay != nil {
		d.cancelReplay()
	}
	if d.cancelMonitor != nil {
		d.cancelMonitor()
	}
	if d.pipelineDone != nil {
		<-d.pipelineDone
	}
	if d.replayDone != nil {
		<-d.replayDone
	}
	if d.monitorDone != nil {
		<-d.monitorDone
	}
	if d.clickhouseConn != nil {
		if err := d.clickhouseConn.Close(); err != nil {
			return fmt.Errorf("close clickhouse: %w", err)
		}
	}
	if d.pool != nil {
		d.pool.Close()
	}
	return nil
}

// closeProductionResources closes infrastructure handles without touching worker state.
func closeProductionResources(resources productionResources) error {
	return resources.Close()
}

// Close releases infrastructure handles stored in the pre-assembly resource bundle.
func (r productionResources) Close() error {
	if r.clickhouseConn != nil {
		if err := r.clickhouseConn.Close(); err != nil {
			return fmt.Errorf("close clickhouse: %w", err)
		}
	}
	if r.pool != nil {
		r.pool.Close()
	}
	return nil
}
