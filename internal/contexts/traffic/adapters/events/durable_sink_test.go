package events

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDurableSinkIgnoresEmptyBatch(t *testing.T) {
	sink := NewDurableSink(DurableSinkOptions{})

	if err := sink.WriteBatch(context.Background(), nil); err != nil {
		t.Fatalf("WriteBatch(nil) error = %v", err)
	}
}

func TestDurableSinkRequiresPrimaryForNonEmptyBatch(t *testing.T) {
	sink := NewDurableSink(DurableSinkOptions{})

	err := sink.WriteBatch(context.Background(), []Event{{EventID: "evt-1"}})
	if err == nil {
		t.Fatal("WriteBatch() error = nil, want missing primary error")
	}
}

func TestDurableSinkRequiresSpoolAfterPrimaryFailure(t *testing.T) {
	sink := NewDurableSink(DurableSinkOptions{
		Primary: &recordingSink{err: errors.New("clickhouse down")},
	})

	err := sink.WriteBatch(context.Background(), []Event{{EventID: "evt-1"}})
	if err == nil {
		t.Fatal("WriteBatch() error = nil, want missing spool error")
	}
}

func TestDurableSinkLeavesBatchUntouchedWhenPrimarySucceeds(t *testing.T) {
	primary := &recordingSink{}
	sink := NewDurableSink(DurableSinkOptions{
		Primary: primary,
	})
	batch := []Event{{EventID: "evt-1"}}

	if err := sink.WriteBatch(context.Background(), batch); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}
	if got := batch[0].SinkStatus; got != "" {
		t.Fatalf("SinkStatus = %q, want empty", got)
	}
	if len(primary.batches) != 1 {
		t.Fatalf("len(primary.batches) = %d, want 1", len(primary.batches))
	}
}

func TestDurableSinkSpoolsBatchWhenPrimaryFails(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{Dir: t.TempDir(), MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	sink := NewDurableSink(DurableSinkOptions{
		Primary: &recordingSink{err: errors.New("clickhouse down")},
		Spool:   spool,
	})

	err = sink.WriteBatch(context.Background(), []Event{{EventID: "evt-1", RequestID: "one"}})
	if err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	batch, err := spool.NextBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("len(batch) = %d, want 1", len(batch))
	}
	if got := batch[0].Event.SpoolStatus; got != "spooled" {
		t.Fatalf("SpoolStatus = %q, want spooled", got)
	}
}

func TestDurableSinkMarksOriginalBatchAfterSpoolSucceeds(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{Dir: t.TempDir(), MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	sink := NewDurableSink(DurableSinkOptions{
		Primary: &recordingSink{err: errors.New("clickhouse down")},
		Spool:   spool,
	})
	batch := []Event{{EventID: "evt-1"}}

	if err := sink.WriteBatch(context.Background(), batch); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}
	if got := batch[0].SinkStatus; got != "spooled" {
		t.Fatalf("SinkStatus = %q, want spooled", got)
	}
	if got := batch[0].SpoolStatus; got != "spooled" {
		t.Fatalf("SpoolStatus = %q, want spooled", got)
	}
}

func TestDurableSinkReturnsErrorWhenSpoolOverflows(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{Dir: t.TempDir(), MaxBytes: 64})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	sink := NewDurableSink(DurableSinkOptions{
		Primary: &recordingSink{err: errors.New("clickhouse down")},
		Spool:   spool,
	})

	err = sink.WriteBatch(context.Background(), []Event{{
		EventID:        "evt-1",
		RequestPayload: "payload larger than the tiny spool budget",
	}})
	if !errors.Is(err, ErrSpoolFull) {
		t.Fatalf("WriteBatch() error = %v, want ErrSpoolFull", err)
	}
}

func TestNewReplayWorkerUsesDefaultOptions(t *testing.T) {
	worker := NewReplayWorker(ReplayWorkerOptions{})

	if worker.batchSize != defaultPipelineBatchSize {
		t.Fatalf("batchSize = %d, want %d", worker.batchSize, defaultPipelineBatchSize)
	}
	if worker.interval != 5*time.Second {
		t.Fatalf("interval = %v, want 5s", worker.interval)
	}
}

func TestReplayWorkerRequiresPrimary(t *testing.T) {
	worker := NewReplayWorker(ReplayWorkerOptions{Spool: stubSpool{}})

	err := worker.ReplayOnce(context.Background())
	if err == nil {
		t.Fatal("ReplayOnce() error = nil, want missing primary error")
	}
}

func TestReplayWorkerRequiresSpool(t *testing.T) {
	worker := NewReplayWorker(ReplayWorkerOptions{Primary: &recordingSink{}})

	err := worker.ReplayOnce(context.Background())
	if err == nil {
		t.Fatal("ReplayOnce() error = nil, want missing spool error")
	}
}

func TestReplayWorkerReturnsSpoolReadError(t *testing.T) {
	worker := NewReplayWorker(ReplayWorkerOptions{
		Primary: &recordingSink{},
		Spool:   stubSpool{nextErr: errors.New("disk read failed")},
	})

	err := worker.ReplayOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "read event spool batch") {
		t.Fatalf("ReplayOnce() error = %v, want read event spool batch context", err)
	}
}

func TestReplayWorkerNoopsWhenSpoolEmpty(t *testing.T) {
	primary := &recordingSink{}
	worker := NewReplayWorker(ReplayWorkerOptions{
		Primary: primary,
		Spool:   stubSpool{},
	})

	if err := worker.ReplayOnce(context.Background()); err != nil {
		t.Fatalf("ReplayOnce() error = %v", err)
	}
	if len(primary.batches) != 0 {
		t.Fatalf("len(primary.batches) = %d, want 0", len(primary.batches))
	}
}

func TestReplayWorkerWritesOldestSpooledBatchAndAcksOnSuccess(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{Dir: t.TempDir(), MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := spool.Append(context.Background(), []Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	primary := &recordingSink{}
	worker := NewReplayWorker(ReplayWorkerOptions{
		Primary:   primary,
		Spool:     spool,
		BatchSize: 1,
		Interval:  time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()
	waitForBatches(t, primary, 1)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stats := spool.Stats(); stats.Depth != 0 {
		t.Fatalf("Depth = %d, want 0", stats.Depth)
	}
	if got := primary.batches[0][0].SpoolStatus; got != "replayed" {
		t.Fatalf("SpoolStatus = %q, want replayed", got)
	}
}

func TestReplayWorkerClaimsSharedDirectoryRecordBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewFileSpool(FileSpoolOptions{Dir: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool(writer) error = %v", err)
	}
	if err := writer.Append(context.Background(), []Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	firstSpool, err := NewFileSpool(FileSpoolOptions{Dir: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool(first) error = %v", err)
	}
	secondSpool, err := NewFileSpool(FileSpoolOptions{Dir: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool(second) error = %v", err)
	}
	primary := &blockingReplaySink{
		started: make(chan []Event, 2),
		release: make(chan struct{}),
	}
	firstWorker := NewReplayWorker(ReplayWorkerOptions{Primary: primary, Spool: firstSpool, BatchSize: 1})
	secondWorker := NewReplayWorker(ReplayWorkerOptions{Primary: primary, Spool: secondSpool, BatchSize: 1})

	firstErr := make(chan error, 1)
	go func() {
		firstErr <- firstWorker.ReplayOnce(context.Background())
	}()
	<-primary.started

	secondErr := make(chan error, 1)
	go func() {
		secondErr <- secondWorker.ReplayOnce(context.Background())
	}()
	select {
	case <-primary.started:
		t.Fatal("second worker wrote a record already claimed by another spool instance")
	case err := <-secondErr:
		if err != nil {
			t.Fatalf("second ReplayOnce() error = %v, want no claimed records", err)
		}
	case <-time.After(100 * time.Millisecond):
	}

	close(primary.release)
	if err := <-firstErr; err != nil {
		t.Fatalf("first ReplayOnce() error = %v", err)
	}
	select {
	case err := <-secondErr:
		if err != nil {
			t.Fatalf("second ReplayOnce() error = %v", err)
		}
	default:
	}
	if stats := writer.Stats(); stats.Depth != 0 {
		t.Fatalf("Depth = %d, want 0", stats.Depth)
	}
}

func TestReplayWorkerKeepsRecordsWhenPrimaryFails(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{Dir: t.TempDir(), MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := spool.Append(context.Background(), []Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	worker := NewReplayWorker(ReplayWorkerOptions{
		Primary:   &recordingSink{err: errors.New("clickhouse down")},
		Spool:     spool,
		BatchSize: 1,
	})

	err = worker.ReplayOnce(context.Background())
	if err == nil {
		t.Fatal("ReplayOnce() error = nil, want primary error")
	}
	if stats := spool.Stats(); stats.Depth != 1 {
		t.Fatalf("Depth = %d, want 1", stats.Depth)
	}
	batch, err := spool.NextBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if batch[0].RetryCount != 1 {
		t.Fatalf("RetryCount = %d, want 1", batch[0].RetryCount)
	}
	if batch[0].LastError != "clickhouse down" {
		t.Fatalf("LastError = %q, want clickhouse down", batch[0].LastError)
	}
}

type blockingReplaySink struct {
	started chan []Event
	release chan struct{}
}

func (s *blockingReplaySink) WriteBatch(ctx context.Context, events []Event) error {
	s.started <- append([]Event(nil), events...)
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestReplayWorkerReturnsRecordFailureError(t *testing.T) {
	primaryErr := errors.New("clickhouse down")
	recordErr := errors.New("metadata write failed")
	spool := stubSpool{
		next: []SpooledEvent{{
			ID:    "evt-1",
			Event: Event{EventID: "evt-1"},
		}},
		recordErr: recordErr,
	}
	worker := NewReplayWorker(ReplayWorkerOptions{
		Primary: &recordingSink{err: primaryErr},
		Spool:   spool,
	})

	err := worker.ReplayOnce(context.Background())
	if !errors.Is(err, recordErr) {
		t.Fatalf("ReplayOnce() error = %v, want record failure error", err)
	}
}

func TestReplayWorkerReturnsAckError(t *testing.T) {
	ackErr := errors.New("remove failed")
	spool := stubSpool{
		next: []SpooledEvent{{
			ID:    "evt-1",
			Event: Event{EventID: "evt-1"},
		}},
		ackErr: ackErr,
	}
	worker := NewReplayWorker(ReplayWorkerOptions{
		Primary: &recordingSink{},
		Spool:   spool,
	})

	err := worker.ReplayOnce(context.Background())
	if !errors.Is(err, ackErr) {
		t.Fatalf("ReplayOnce() error = %v, want ack error", err)
	}
}

type stubSpool struct {
	next      []SpooledEvent
	nextErr   error
	appendErr error
	recordErr error
	ackErr    error
}

func (s stubSpool) Append(context.Context, []Event) error {
	return s.appendErr
}

func (s stubSpool) NextBatch(context.Context, int) ([]SpooledEvent, error) {
	return s.next, s.nextErr
}

func (s stubSpool) RecordFailure(context.Context, []SpooledEvent, error) error {
	return s.recordErr
}

func (s stubSpool) Ack(context.Context, []SpooledEvent) error {
	return s.ackErr
}

func (s stubSpool) Stats() SpoolStats {
	return SpoolStats{Depth: len(s.next)}
}
