package events

import (
	"context"
	"fmt"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/howmuchsec/kiwiguard/internal/config"
	kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/clickhouse"
	eventadapter "github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
)

// PipelineOptions configures the production traffic-event pipeline assembly.
type PipelineOptions struct {
	Config           config.Config
	RuntimeRepo      kgruntime.RuntimeConfigRepository
	Spool            *eventadapter.FileSpool
	Primary          eventadapter.BatchSink
	ReplayPrimary    eventadapter.BatchSink
	PipelineSink     func(eventadapter.BatchSink) eventadapter.BatchSink
	StrictHealthGate *eventadapter.HealthGate
}

// ClickHousePipelineOptions configures ClickHouse-backed traffic-event pipeline assembly.
type ClickHousePipelineOptions struct {
	Config            config.Config
	RuntimeRepo       kgruntime.RuntimeConfigRepository
	Spool             *eventadapter.FileSpool
	Conn              ch.Conn
	PrimarySink       func(eventadapter.BatchSink) eventadapter.BatchSink
	ReplayPrimarySink func(eventadapter.BatchSink) eventadapter.BatchSink
	PipelineSink      func(eventadapter.BatchSink) eventadapter.BatchSink
	StrictHealthGate  *eventadapter.HealthGate
}

// PipelineHandle owns the background traffic-event pipeline goroutines.
type PipelineHandle struct {
	Pipeline       *eventadapter.Pipeline
	CancelPipeline context.CancelFunc
	PipelineDone   <-chan struct{}
	CancelReplay   context.CancelFunc
	ReplayDone     <-chan struct{}
}

// StartClickHousePipeline builds and starts the ClickHouse-backed traffic-event pipeline.
func StartClickHousePipeline(ctx context.Context, options ClickHousePipelineOptions) (*PipelineHandle, error) {
	primary := eventadapter.BatchSink(clickhouse.NewWriter(options.Conn))
	if options.PrimarySink != nil {
		primary = options.PrimarySink(primary)
	}
	replayPrimary := primary
	if options.ReplayPrimarySink != nil {
		replayPrimary = options.ReplayPrimarySink(primary)
	}
	return StartPipeline(ctx, PipelineOptions{
		Config:           options.Config,
		RuntimeRepo:      options.RuntimeRepo,
		Spool:            options.Spool,
		Primary:          primary,
		ReplayPrimary:    replayPrimary,
		PipelineSink:     options.PipelineSink,
		StrictHealthGate: options.StrictHealthGate,
	})
}

// StartPipeline builds and starts the production traffic-event pipeline.
func StartPipeline(ctx context.Context, options PipelineOptions) (*PipelineHandle, error) {
	runtimeCfg, err := options.RuntimeRepo.LoadRuntimeConfig(ctx)
	if err != nil {
		return nil, err
	}

	writer, replayWorker, err := BuildDurableSink(options.Config, runtimeCfg, options.Spool, options.Primary, options.ReplayPrimary)
	if err != nil {
		return nil, err
	}
	if options.PipelineSink != nil {
		writer = options.PipelineSink(writer)
	}

	pipeline := eventadapter.NewPipeline(eventadapter.PipelineOptions{
		Capacity:          options.Config.EventQueueCapacity,
		BatchSize:         options.Config.EventBatchSize,
		Sink:              writer,
		StrictHealthGate:  options.StrictHealthGate,
		MarkUnhealthyDrop: true,
	})

	pipelineCtx, cancelPipeline := context.WithCancel(context.Background())
	pipelineDone := make(chan struct{})
	go func() {
		defer close(pipelineDone)
		_ = pipeline.Run(pipelineCtx, time.Second)
	}()

	replayCtx, cancelReplay := context.WithCancel(context.Background())
	replayDone := make(chan struct{})
	go func() {
		defer close(replayDone)
		_ = replayWorker.Run(replayCtx)
	}()

	return &PipelineHandle{
		Pipeline:       pipeline,
		CancelPipeline: cancelPipeline,
		PipelineDone:   pipelineDone,
		CancelReplay:   cancelReplay,
		ReplayDone:     replayDone,
	}, nil
}

// BuildDurableSink assembles the durable traffic-event sink and replay worker.
func BuildDurableSink(cfg config.Config, runtimeCfg kgruntime.RuntimeConfig, spool *eventadapter.FileSpool, primary eventadapter.BatchSink, replayPrimary eventadapter.BatchSink) (eventadapter.BatchSink, *eventadapter.ReplayWorker, error) {
	if err := ValidateRuntimeSinks(runtimeCfg); err != nil {
		return nil, nil, err
	}

	var err error
	if spool == nil {
		spool, err = NewFileSpool(cfg)
		if err != nil {
			return nil, nil, err
		}
	}

	sink := eventadapter.NewDurableSink(eventadapter.DurableSinkOptions{
		Primary: primary,
		Spool:   spool,
	})
	if replayPrimary == nil {
		replayPrimary = primary
	}
	worker := eventadapter.NewReplayWorker(eventadapter.ReplayWorkerOptions{
		Primary:   replayPrimary,
		Spool:     spool,
		BatchSize: cfg.EventSpoolBatchSize,
		Interval:  cfg.EventSpoolReplayInterval,
	})
	return sink, worker, nil
}

// NewFileSpool creates the configured file-backed traffic-event spool.
func NewFileSpool(cfg config.Config) (*eventadapter.FileSpool, error) {
	spool, err := eventadapter.NewFileSpool(eventadapter.FileSpoolOptions{
		Dir:      cfg.EventSpoolDir,
		MaxBytes: cfg.EventSpoolMaxBytes,
		MaxAge:   cfg.EventSpoolMaxAge,
	})
	if err != nil {
		return nil, fmt.Errorf("create event spool: %w", err)
	}
	return spool, nil
}

// ValidateRuntimeSinks rejects runtime sink kinds this assembly cannot wire.
func ValidateRuntimeSinks(cfg kgruntime.RuntimeConfig) error {
	for _, sink := range cfg.Sinks {
		if sink.Disabled {
			continue
		}
		switch sink.Kind {
		case "", "clickhouse":
		default:
			return fmt.Errorf("unsupported event sink %q kind %q", sink.Key, sink.Kind)
		}
	}
	return nil
}
