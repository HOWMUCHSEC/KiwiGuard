package events

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHashBodyIsStableAndPrivacySafe(t *testing.T) {
	first := HashBody([]byte("alice@example.com"))
	second := HashBody([]byte("alice@example.com"))
	if first == "" {
		t.Fatal("HashBody() returned empty string")
	}
	if first != second {
		t.Fatalf("hash mismatch: %q != %q", first, second)
	}
	if first == "alice@example.com" {
		t.Fatal("HashBody() returned raw body")
	}
}

func TestMemoryWriterDropsWhenFullWithoutBlocking(t *testing.T) {
	writer := NewMemoryWriter(MemoryWriterOptions{Capacity: 1})
	defer writer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := writer.Enqueue(ctx, Event{RequestID: "one"}); err != nil {
		t.Fatalf("Enqueue(one) error = %v", err)
	}
	if err := writer.Enqueue(ctx, Event{RequestID: "two"}); err != nil {
		t.Fatalf("Enqueue(two) error = %v", err)
	}
	if writer.Dropped() != 1 {
		t.Fatalf("Dropped() = %d, want 1", writer.Dropped())
	}
}

func TestNewMemoryWriterUsesDefaultCapacity(t *testing.T) {
	writer := NewMemoryWriter(MemoryWriterOptions{})
	defer writer.Close()

	if cap(writer.events) != defaultMemoryWriterCapacity {
		t.Fatalf("cap(events) = %d, want %d", cap(writer.events), defaultMemoryWriterCapacity)
	}
}

func TestMemoryWriterEnqueueHonorsCanceledContext(t *testing.T) {
	writer := NewMemoryWriter(MemoryWriterOptions{Capacity: 1})
	defer writer.Close()
	if err := writer.Enqueue(context.Background(), Event{RequestID: "queued"}); err != nil {
		t.Fatalf("Enqueue(queued) error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := writer.Enqueue(ctx, Event{RequestID: "canceled"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Enqueue() error = %v, want context.Canceled", err)
	}
	if writer.Dropped() != 0 {
		t.Fatalf("Dropped() = %d, want 0", writer.Dropped())
	}
}

func TestMemoryWriterDropsAfterClose(t *testing.T) {
	writer := NewMemoryWriter(MemoryWriterOptions{Capacity: 1})
	writer.Close()
	writer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := writer.Enqueue(ctx, Event{RequestID: "after-close"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if writer.Dropped() != 1 {
		t.Fatalf("Dropped() = %d, want 1", writer.Dropped())
	}
}

func TestPipelineEnqueueHonorsCanceledContext(t *testing.T) {
	pipeline := NewPipeline(PipelineOptions{Capacity: 1, BatchSize: 1, Sink: &recordingSink{}})
	defer func() {
		_ = pipeline.Close()
	}()
	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "queued"}); err != nil {
		t.Fatalf("Enqueue(queued) error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pipeline.Enqueue(ctx, Event{RequestID: "canceled"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Enqueue() error = %v, want context.Canceled", err)
	}
	if pipeline.Dropped() != 0 {
		t.Fatalf("Dropped() = %d, want 0", pipeline.Dropped())
	}
}

func TestPipelineDropsAfterClose(t *testing.T) {
	health := NewHealthGate()
	pipeline := NewPipeline(PipelineOptions{
		Capacity:          1,
		BatchSize:         1,
		Sink:              &recordingSink{},
		StrictHealthGate:  health,
		MarkUnhealthyDrop: true,
	})
	if err := pipeline.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "closed"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if pipeline.Dropped() != 1 {
		t.Fatalf("Dropped() = %d, want 1", pipeline.Dropped())
	}
	if health.Reason() != "event_queue_saturated" {
		t.Fatalf("Reason() = %q, want event_queue_saturated", health.Reason())
	}
}

func TestPipelineFlushesEventsToSink(t *testing.T) {
	sink := &recordingSink{}
	pipeline := NewPipeline(PipelineOptions{
		Capacity:  4,
		BatchSize: 2,
		Sink:      sink,
	})
	defer func() {
		_ = pipeline.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pipeline.Enqueue(ctx, Event{RequestID: "one"}); err != nil {
		t.Fatalf("Enqueue(one) error = %v", err)
	}
	if err := pipeline.Enqueue(ctx, Event{RequestID: "two"}); err != nil {
		t.Fatalf("Enqueue(two) error = %v", err)
	}
	if err := pipeline.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	if len(sink.batches) != 1 {
		t.Fatalf("len(batches) = %d, want 1", len(sink.batches))
	}
	if got := sink.batches[0][0].SinkStatus; got != "delivered" {
		t.Fatalf("SinkStatus = %q, want delivered", got)
	}
}

func TestPipelineFlushWithoutSinkDiscardsQueuedEvents(t *testing.T) {
	pipeline := NewPipeline(PipelineOptions{Capacity: 2, BatchSize: 1})
	defer func() {
		_ = pipeline.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pipeline.Enqueue(ctx, Event{RequestID: "discarded"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if err := pipeline.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if stats := pipeline.Stats(); stats.QueueDepth != 0 {
		t.Fatalf("QueueDepth = %d, want 0", stats.QueueDepth)
	}
}

func TestHealthGateNilIsHealthy(t *testing.T) {
	var gate *HealthGate
	if !gate.Healthy() {
		t.Fatal("nil gate Healthy() = false, want true")
	}
	if gate.Reason() != "" {
		t.Fatalf("nil gate Reason() = %q, want empty", gate.Reason())
	}
	gate.MarkHealthy()
	gate.MarkUnhealthy("ignored")
}

func TestNewPipelineUsesDefaultOptions(t *testing.T) {
	pipeline := NewPipeline(PipelineOptions{})
	defer func() {
		_ = pipeline.Close()
	}()

	stats := pipeline.Stats()
	if stats.Capacity != defaultPipelineCapacity {
		t.Fatalf("Capacity = %d, want %d", stats.Capacity, defaultPipelineCapacity)
	}
}

func TestPipelineFlushMarksFailedEvents(t *testing.T) {
	sink := &recordingSink{err: errors.New("sink unavailable")}
	pipeline := NewPipeline(PipelineOptions{
		Capacity:  2,
		BatchSize: 2,
		Sink:      sink,
	})
	defer func() {
		_ = pipeline.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pipeline.Enqueue(ctx, Event{RequestID: "failed"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if err := pipeline.Flush(ctx); err == nil {
		t.Fatal("Flush() error = nil, want sink error")
	}

	if len(sink.batches) != 2 {
		t.Fatalf("len(batches) = %d, want 2", len(sink.batches))
	}
	if got := sink.batches[1][0].SinkStatus; got != "failed" {
		t.Fatalf("SinkStatus = %q, want failed", got)
	}
}

func TestPipelineWithoutStrictHealthGateDoesNotPanicOnSinkFailure(t *testing.T) {
	sink := &recordingSink{err: errors.New("sink unavailable")}
	pipeline := NewPipeline(PipelineOptions{
		Capacity:  1,
		BatchSize: 1,
		Sink:      sink,
	})
	defer func() {
		_ = pipeline.Close()
	}()

	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "failed"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if err := pipeline.Flush(context.Background()); err == nil {
		t.Fatal("Flush() error = nil, want sink error")
	}
}

func TestPipelineWithoutStrictHealthGateDoesNotPanicOnDrop(t *testing.T) {
	pipeline := NewPipeline(PipelineOptions{
		Capacity:          1,
		BatchSize:         1,
		Sink:              &recordingSink{},
		MarkUnhealthyDrop: true,
	})
	defer func() {
		_ = pipeline.Close()
	}()

	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "one"}); err != nil {
		t.Fatalf("Enqueue(one) error = %v", err)
	}
	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "two"}); err != nil {
		t.Fatalf("Enqueue(two) error = %v", err)
	}
	if pipeline.Dropped() != 1 {
		t.Fatalf("Dropped() = %d, want 1", pipeline.Dropped())
	}
}

func TestHealthGateTracksStateAndReason(t *testing.T) {
	health := NewHealthGate()
	if !health.Healthy() {
		t.Fatal("new health gate is unhealthy")
	}
	if health.Reason() != "" {
		t.Fatalf("Reason() = %q, want empty", health.Reason())
	}

	health.MarkUnhealthy("clickhouse_write_failed")
	if health.Healthy() {
		t.Fatal("health gate is healthy after MarkUnhealthy")
	}
	if health.Reason() != "clickhouse_write_failed" {
		t.Fatalf("Reason() = %q, want clickhouse_write_failed", health.Reason())
	}

	health.MarkHealthy()
	if !health.Healthy() {
		t.Fatal("health gate is unhealthy after MarkHealthy")
	}
	if health.Reason() != "" {
		t.Fatalf("Reason() = %q, want empty", health.Reason())
	}
}

func TestPipelineMarksHealthUnhealthyWhenStrictQueueSaturates(t *testing.T) {
	health := NewHealthGate()
	pipeline := NewPipeline(PipelineOptions{
		Capacity:          1,
		BatchSize:         1,
		Sink:              &recordingSink{},
		StrictHealthGate:  health,
		MarkUnhealthyDrop: true,
	})
	defer func() {
		_ = pipeline.Close()
	}()

	ctx := context.Background()
	if err := pipeline.Enqueue(ctx, Event{RequestID: "one"}); err != nil {
		t.Fatalf("Enqueue(one) error = %v", err)
	}
	if err := pipeline.Enqueue(ctx, Event{RequestID: "two"}); err != nil {
		t.Fatalf("Enqueue(two) error = %v", err)
	}
	if health.Healthy() {
		t.Fatal("health gate is healthy after strict queue saturation")
	}
	if health.Reason() != "event_queue_saturated" {
		t.Fatalf("Reason() = %q, want event_queue_saturated", health.Reason())
	}

	stats := pipeline.Stats()
	if stats.QueueDepth != 1 {
		t.Fatalf("QueueDepth = %d, want 1", stats.QueueDepth)
	}
	if stats.Capacity != 1 {
		t.Fatalf("Capacity = %d, want 1", stats.Capacity)
	}
	if stats.Dropped != 1 {
		t.Fatalf("Dropped = %d, want 1", stats.Dropped)
	}
}

func TestPipelineFlushMarksStrictHealthUnhealthyOnSinkFailure(t *testing.T) {
	health := NewHealthGate()
	pipeline := NewPipeline(PipelineOptions{
		Capacity:         2,
		BatchSize:        2,
		Sink:             &recordingSink{err: errors.New("sink unavailable")},
		StrictHealthGate: health,
	})
	defer func() {
		_ = pipeline.Close()
	}()

	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "failed"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	err := pipeline.Flush(context.Background())
	if err == nil {
		t.Fatal("Flush() error = nil, want sink error")
	}
	if health.Healthy() {
		t.Fatal("health gate is healthy after sink failure")
	}
	if health.Reason() != "clickhouse_write_failed" {
		t.Fatalf("Reason() = %q, want clickhouse_write_failed", health.Reason())
	}

	stats := pipeline.Stats()
	if !strings.Contains(stats.LastError, "flush event batch") {
		t.Fatalf("LastError = %q, want flush event batch context", stats.LastError)
	}
}

func TestPipelineFlushMarksStrictHealthHealthyAfterSuccess(t *testing.T) {
	health := NewHealthGate()
	health.MarkUnhealthy("clickhouse_write_failed")
	pipeline := NewPipeline(PipelineOptions{
		Capacity:         2,
		BatchSize:        2,
		Sink:             &recordingSink{},
		StrictHealthGate: health,
	})
	defer func() {
		_ = pipeline.Close()
	}()

	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "ok"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if err := pipeline.Flush(context.Background()); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if !health.Healthy() {
		t.Fatalf("health gate is unhealthy after successful flush: %s", health.Reason())
	}
	if pipeline.Stats().LastError != "" {
		t.Fatalf("LastError = %q, want empty", pipeline.Stats().LastError)
	}
}

func TestPipelineRunFlushesUntilContextCancelled(t *testing.T) {
	sink := &recordingSink{}
	pipeline := NewPipeline(PipelineOptions{
		Capacity:  2,
		BatchSize: 2,
		Sink:      sink,
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- pipeline.Run(ctx, time.Millisecond)
	}()

	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "one"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	waitForBatches(t, sink, 1)

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func TestPipelineRunMarksHealthUnhealthyOnSinkFailure(t *testing.T) {
	health := NewHealthGate()
	sink := &recordingSink{err: errors.New("sink unavailable")}
	pipeline := NewPipeline(PipelineOptions{
		Capacity:         2,
		BatchSize:        2,
		Sink:             sink,
		StrictHealthGate: health,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- pipeline.Run(ctx, time.Millisecond)
	}()

	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "failed"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	waitForHealthReason(t, health, "clickhouse_write_failed")

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func TestPipelineRunUsesBoundedShutdownDrain(t *testing.T) {
	pipeline := NewPipeline(PipelineOptions{
		Capacity:        1,
		BatchSize:       1,
		Sink:            blockingSink{},
		ShutdownTimeout: time.Nanosecond,
	})
	if err := pipeline.Enqueue(context.Background(), Event{RequestID: "blocked"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- pipeline.Run(ctx, time.Hour)
	}()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after bounded shutdown drain")
	}
}

func TestPipelineDropsWhenFullWithoutBlocking(t *testing.T) {
	pipeline := NewPipeline(PipelineOptions{Capacity: 1, BatchSize: 1, Sink: &recordingSink{}})
	defer func() {
		_ = pipeline.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pipeline.Enqueue(ctx, Event{RequestID: "one"}); err != nil {
		t.Fatalf("Enqueue(one) error = %v", err)
	}
	if err := pipeline.Enqueue(ctx, Event{RequestID: "two"}); err != nil {
		t.Fatalf("Enqueue(two) error = %v", err)
	}
	if pipeline.Dropped() != 1 {
		t.Fatalf("Dropped() = %d, want 1", pipeline.Dropped())
	}
}

type recordingSink struct {
	mu      sync.Mutex
	batches [][]Event
	err     error
}

type blockingSink struct{}

func (blockingSink) WriteBatch(ctx context.Context, events []Event) error {
	<-ctx.Done()
	return ctx.Err()
}

func (s *recordingSink) WriteBatch(ctx context.Context, events []Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	copied := append([]Event(nil), events...)
	s.batches = append(s.batches, copied)
	return s.err
}

func waitForBatches(t *testing.T, sink *recordingSink, want int) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		sink.mu.Lock()
		got := len(sink.batches)
		sink.mu.Unlock()
		if got >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	t.Fatalf("len(batches) = %d, want at least %d", len(sink.batches), want)
}

func waitForHealthReason(t *testing.T, health *HealthGate, want string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if health.Reason() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("Reason() = %q, want %q", health.Reason(), want)
}
