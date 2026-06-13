package events

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	spoolStatusSpooled  = "spooled"
	spoolStatusReplayed = "replayed"
)

// DurableSinkOptions defines how the durable sink falls back to spool storage during sink failures.
type DurableSinkOptions struct {
	Primary BatchSink
	Spool   Spool
}

// DurableSink writes to the primary sink and falls back to durable spool storage.
type DurableSink struct {
	primary BatchSink
	spool   Spool
}

// NewDurableSink builds the primary event sink wrapper with durable spool fallback.
func NewDurableSink(opts DurableSinkOptions) *DurableSink {
	return &DurableSink{
		primary: opts.Primary,
		spool:   opts.Spool,
	}
}

// WriteBatch attempts primary delivery first and durably spools the batch when delivery fails.
func (s *DurableSink) WriteBatch(ctx context.Context, batch []Event) error {
	if len(batch) == 0 {
		return nil
	}
	if s.primary == nil {
		return errors.New("primary event sink is required")
	}
	if err := s.primary.WriteBatch(ctx, batch); err == nil {
		return nil
	}
	if s.spool == nil {
		return errors.New("event spool is required after primary sink failure")
	}

	spooled := cloneEvents(batch)
	for i := range spooled {
		spooled[i].SinkStatus = spoolStatusSpooled
		spooled[i].SpoolStatus = spoolStatusSpooled
	}
	if err := s.spool.Append(ctx, spooled); err != nil {
		return fmt.Errorf("append event batch to spool: %w", err)
	}
	for i := range batch {
		batch[i].SinkStatus = spoolStatusSpooled
		batch[i].SpoolStatus = spoolStatusSpooled
	}
	return nil
}

// ReplayWorkerOptions configures durable spool replay.
type ReplayWorkerOptions struct {
	Primary   BatchSink
	Spool     Spool
	BatchSize int
	Interval  time.Duration
}

// ReplayWorker replays durable event records to the primary sink.
type ReplayWorker struct {
	primary   BatchSink
	spool     Spool
	batchSize int
	interval  time.Duration
}

// NewReplayWorker builds the background worker that replays durable spool contents.
func NewReplayWorker(opts ReplayWorkerOptions) *ReplayWorker {
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = defaultPipelineBatchSize
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &ReplayWorker{
		primary:   opts.Primary,
		spool:     opts.Spool,
		batchSize: batchSize,
		interval:  interval,
	}
}

// Run replays spooled records until ctx is canceled.
func (w *ReplayWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = w.ReplayOnce(ctx)
		}
	}
}

// ReplayOnce attempts a single replay batch.
func (w *ReplayWorker) ReplayOnce(ctx context.Context) error {
	if w.primary == nil {
		return errors.New("primary event sink is required")
	}
	if w.spool == nil {
		return errors.New("event spool is required")
	}
	spooled, err := w.spool.NextBatch(ctx, w.batchSize)
	if err != nil {
		return fmt.Errorf("read event spool batch: %w", err)
	}
	if len(spooled) == 0 {
		return nil
	}

	events := make([]Event, 0, len(spooled))
	for _, record := range spooled {
		event := record.Event
		event.SpoolStatus = spoolStatusReplayed
		event.RetryCount = record.RetryCount + 1
		events = append(events, event)
	}
	if err := w.primary.WriteBatch(ctx, events); err != nil {
		if recordErr := w.spool.RecordFailure(ctx, spooled, err); recordErr != nil {
			return fmt.Errorf("record event spool replay failure: %w", recordErr)
		}
		return fmt.Errorf("replay event spool batch: %w", err)
	}
	if err := w.spool.Ack(ctx, spooled); err != nil {
		return fmt.Errorf("ack event spool batch: %w", err)
	}
	return nil
}

func cloneEvents(batch []Event) []Event {
	cloned := make([]Event, len(batch))
	copy(cloned, batch)
	return cloned
}
