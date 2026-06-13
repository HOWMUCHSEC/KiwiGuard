package events

import (
	"context"
	"sync"
	"sync/atomic"
)

const defaultMemoryWriterCapacity = 1024

// MemoryWriterOptions defines buffer limits for the in-memory event writer.
type MemoryWriterOptions struct {
	Capacity int
}

// MemoryWriter provides a bounded in-memory event queue for local runs and tests.
type MemoryWriter struct {
	events  chan Event
	once    sync.Once
	closed  atomic.Bool
	dropped atomic.Int64
}

// NewMemoryWriter builds the bounded in-memory event writer used for local runs and tests.
func NewMemoryWriter(opts MemoryWriterOptions) *MemoryWriter {
	capacity := opts.Capacity
	if capacity <= 0 {
		capacity = defaultMemoryWriterCapacity
	}

	return &MemoryWriter{
		events: make(chan Event, capacity),
	}
}

// Enqueue attempts to add an event without blocking gateway traffic.
func (w *MemoryWriter) Enqueue(ctx context.Context, event Event) error {
	if w.closed.Load() {
		w.dropped.Add(1)
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case w.events <- event:
		return nil
	default:
		w.dropped.Add(1)
		return nil
	}
}

// Dropped reports how many events were discarded because the queue was full or already closed.
func (w *MemoryWriter) Dropped() int64 {
	return w.dropped.Load()
}

// Close releases the writer. It is safe to call Close more than once.
func (w *MemoryWriter) Close() {
	w.once.Do(func() {
		w.closed.Store(true)
	})
}
