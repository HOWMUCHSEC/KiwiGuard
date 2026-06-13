package bootstrap

import (
	"context"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	eventassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/events"
)

// startTrafficEventPipeline starts the shared traffic event pipeline and replay worker.
func (f *Factory) startTrafficEventPipeline(ctx context.Context, deps *productionDeps, conn ch.Conn) error {
	handle, err := eventassembly.StartClickHousePipeline(ctx, eventassembly.ClickHousePipelineOptions{
		Config:            f.options.Config,
		RuntimeRepo:       deps.runtimeRepo,
		Spool:             deps.eventSpool,
		Conn:              conn,
		PrimarySink:       f.telemetry.WrapBatchSink,
		ReplayPrimarySink: f.metrics.WrapBatchSink,
		PipelineSink:      f.metrics.WrapBatchSink,
		StrictHealthGate:  deps.eventGate,
	})
	if err != nil {
		return err
	}
	deps.useTrafficEventPipeline(handle)
	return nil
}

// useTrafficEventPipeline attaches the live pipeline and replay handles to shared production dependencies.
func (d *productionDeps) useTrafficEventPipeline(handle *eventassembly.PipelineHandle) {
	d.eventPipeline = handle.Pipeline
	d.cancelPipeline = handle.CancelPipeline
	d.pipelineDone = handle.PipelineDone
	d.cancelReplay = handle.CancelReplay
	d.replayDone = handle.ReplayDone
}
