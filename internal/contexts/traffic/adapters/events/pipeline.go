package events

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultPipelineCapacity  = 1024
	defaultPipelineBatchSize = 100

	unhealthyReasonQueueSaturated = "event_queue_saturated"
	unhealthyReasonWriteFailed    = "clickhouse_write_failed"
)

// PipelineOptions defines queueing, batching, and health behavior for asynchronous event delivery.
type PipelineOptions struct {
	Capacity          int
	BatchSize         int
	Sink              BatchSink
	StrictHealthGate  *HealthGate
	MarkUnhealthyDrop bool
	ShutdownTimeout   time.Duration
}

// PipelineStats captures queue and sink state for health and diagnostics.
type PipelineStats struct {
	QueueDepth int
	Capacity   int
	Dropped    uint64
	LastError  string
}

// Pipeline buffers gateway events and flushes them to a batch sink.
type Pipeline struct {
	events            chan Event
	sink              BatchSink
	batchSize         int
	strictHealthGate  *HealthGate
	markUnhealthyDrop bool
	shutdownTimeout   time.Duration
	once              sync.Once
	closed            atomic.Bool
	dropped           atomic.Uint64
	lastError         atomic.Value
}

// NewPipeline builds an in-memory queue that batches events before handing them to a sink.
func NewPipeline(opts PipelineOptions) *Pipeline {
	capacity := opts.Capacity
	if capacity <= 0 {
		capacity = defaultPipelineCapacity
	}

	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = defaultPipelineBatchSize
	}

	return &Pipeline{
		events:            make(chan Event, capacity),
		sink:              opts.Sink,
		batchSize:         batchSize,
		strictHealthGate:  opts.StrictHealthGate,
		markUnhealthyDrop: opts.MarkUnhealthyDrop,
		shutdownTimeout:   opts.ShutdownTimeout,
	}
}

// Enqueue attempts to add an event without blocking the caller.
func (p *Pipeline) Enqueue(ctx context.Context, event Event) error {
	if p.closed.Load() {
		p.dropped.Add(1)
		p.markDropUnhealthy()
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.events <- event:
		return nil
	default:
		p.dropped.Add(1)
		p.markDropUnhealthy()
		return nil
	}
}

// Flush drains currently queued events and writes them in batches.
func (p *Pipeline) Flush(ctx context.Context) error {
	if p.sink == nil {
		p.discardQueued()
		return nil
	}

	var firstErr error
	for {
		batch := p.nextBatch()
		if len(batch) == 0 {
			return firstErr
		}

		markSinkStatus(batch, "delivered")
		if err := p.sink.WriteBatch(ctx, batch); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("flush event batch: %w", err)
			}
			markSinkStatus(batch, "failed")
			if statusErr := p.sink.WriteBatch(ctx, batch); statusErr != nil && firstErr == nil {
				firstErr = fmt.Errorf("flush failed event status: %w", statusErr)
			}
			p.markWriteFailed(firstErr)
			continue
		}
		p.markWriteSucceeded()
	}
}

// Run flushes queued events until ctx is canceled, then drains the queue once.
func (p *Pipeline) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = p.Close()
			drainCtx, cancel := p.shutdownContext()
			if err := p.Flush(drainCtx); err != nil {
				p.markWriteFailed(err)
			}
			cancel()
			return nil
		case <-ticker.C:
			if err := p.Flush(ctx); err != nil {
				p.markWriteFailed(err)
			}
		}
	}
}

func (p *Pipeline) shutdownContext() (context.Context, context.CancelFunc) {
	timeout := p.shutdownTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

// Close closes the pipeline. It is safe to call Close more than once.
func (p *Pipeline) Close() error {
	p.once.Do(func() {
		p.closed.Store(true)
	})
	return nil
}

// Dropped returns the number of events dropped because the queue was full or closed.
func (p *Pipeline) Dropped() uint64 {
	return p.dropped.Load()
}

// Stats reports current queue depth, drop count, and the latest sink failure state.
func (p *Pipeline) Stats() PipelineStats {
	stats := PipelineStats{
		QueueDepth: len(p.events),
		Capacity:   cap(p.events),
		Dropped:    p.dropped.Load(),
	}
	value := p.lastError.Load()
	if value != nil {
		stats.LastError, _ = value.(string)
	}
	return stats
}

func (p *Pipeline) nextBatch() []Event {
	batch := make([]Event, 0, p.batchSize)
	for len(batch) < p.batchSize {
		select {
		case event := <-p.events:
			batch = append(batch, event)
		default:
			return batch
		}
	}
	return batch
}

func (p *Pipeline) discardQueued() {
	for {
		select {
		case <-p.events:
		default:
			return
		}
	}
}

func markSinkStatus(events []Event, status string) {
	for i := range events {
		events[i].SinkStatus = status
	}
}

func (p *Pipeline) markDropUnhealthy() {
	if p.markUnhealthyDrop {
		p.strictHealthGate.MarkUnhealthy(unhealthyReasonQueueSaturated)
	}
}

func (p *Pipeline) markWriteFailed(err error) {
	if err != nil {
		p.lastError.Store(err.Error())
	}
	p.strictHealthGate.MarkUnhealthy(unhealthyReasonWriteFailed)
}

func (p *Pipeline) markWriteSucceeded() {
	p.lastError.Store("")
	p.strictHealthGate.MarkHealthy()
}
