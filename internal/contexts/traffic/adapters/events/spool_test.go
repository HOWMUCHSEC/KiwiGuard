package events

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewFileSpoolRejectsInvalidOptions(t *testing.T) {
	t.Run("missing dir", func(t *testing.T) {
		_, err := NewFileSpool(FileSpoolOptions{MaxBytes: 1})
		if err == nil {
			t.Fatal("NewFileSpool() error = nil, want missing dir error")
		}
	})

	t.Run("non-positive max bytes", func(t *testing.T) {
		_, err := NewFileSpool(FileSpoolOptions{Dir: t.TempDir()})
		if err == nil {
			t.Fatal("NewFileSpool() error = nil, want max bytes error")
		}
	})
}

func TestFileSpoolAppendsAndReadsOldestRecords(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append(one) error = %v", err)
	}
	now = now.Add(time.Second)
	if err := spool.Append(ctx, []Event{{EventID: "evt-2", RequestID: "two"}}); err != nil {
		t.Fatalf("Append(two) error = %v", err)
	}

	batch, err := spool.NextBatch(ctx, 10)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("len(batch) = %d, want 2", len(batch))
	}
	if batch[0].Event.RequestID != "one" || batch[1].Event.RequestID != "two" {
		t.Fatalf("batch order = [%q %q], want [one two]", batch[0].Event.RequestID, batch[1].Event.RequestID)
	}
	if batch[0].CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}
}

func TestFileSpoolAppendEmptyBatchIsNoop(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	if err := spool.Append(context.Background(), nil); err != nil {
		t.Fatalf("Append(nil) error = %v", err)
	}
	if stats := spool.Stats(); stats.Depth != 0 {
		t.Fatalf("Depth = %d, want 0", stats.Depth)
	}
}

func TestFileSpoolAppendHonorsCanceledContext(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = spool.Append(ctx, []Event{{EventID: "evt-1"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Append() error = %v, want context.Canceled", err)
	}
	if stats := spool.Stats(); stats.Depth != 0 {
		t.Fatalf("Depth = %d, want 0", stats.Depth)
	}
}

func TestFileSpoolConcurrentAppendsRespectMaxBytes(t *testing.T) {
	const maxBytes int64 = 512
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: maxBytes,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		i := i
		wg.Go(func() {
			err := spool.Append(context.Background(), []Event{{
				EventID:        "evt-" + strconv.Itoa(i),
				RequestID:      "request",
				RequestPayload: strings.Repeat("x", 160),
			}})
			if err != nil && !errors.Is(err, ErrSpoolFull) {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("Append() unexpected error = %v", err)
	}
	if stats := spool.Stats(); stats.Bytes > maxBytes {
		t.Fatalf("spool bytes = %d, want <= %d", stats.Bytes, maxBytes)
	}
}

func TestFileSpoolAppendReturnsDirectorySyncError(t *testing.T) {
	original := syncDir
	syncErr := errors.New("sync failed")
	syncDir = func(string) error {
		return syncErr
	}
	defer func() {
		syncDir = original
	}()
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	err = spool.Append(context.Background(), []Event{{EventID: "evt-1"}})
	if !errors.Is(err, syncErr) {
		t.Fatalf("Append() error = %v, want sync error", err)
	}
}

func TestFileSpoolNextBatchDefaultsNonPositiveLimitToOne(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1"}, {EventID: "evt-2"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	batch, err := spool.NextBatch(ctx, 0)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("len(batch) = %d, want 1", len(batch))
	}
}

func TestFileSpoolNextBatchReturnsMalformedRecordError(t *testing.T) {
	dir := t.TempDir()
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      dir,
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile(bad record) error = %v", err)
	}

	_, err = spool.NextBatch(context.Background(), 1)
	if err == nil {
		t.Fatal("NextBatch() error = nil, want malformed record error")
	}
	if !strings.Contains(err.Error(), "decode event spool record") {
		t.Fatalf("NextBatch() error = %v, want decode context", err)
	}
}

func TestFileSpoolNextBatchHonorsCanceledContext(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := spool.Append(context.Background(), []Event{{EventID: "evt-1"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = spool.NextBatch(ctx, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("NextBatch() error = %v, want context.Canceled", err)
	}
}

func TestFileSpoolRejectsWhenMaxBytesWouldOverflow(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 64,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	err = spool.Append(context.Background(), []Event{{EventID: "evt-1", RequestPayload: "payload larger than the tiny spool budget"}})
	if !errors.Is(err, ErrSpoolFull) {
		t.Fatalf("Append() error = %v, want ErrSpoolFull", err)
	}
	if stats := spool.Stats(); stats.OverflowCount != 1 {
		t.Fatalf("OverflowCount = %d, want 1", stats.OverflowCount)
	}
}

func TestFileSpoolOverflowReturnsDirectorySyncError(t *testing.T) {
	original := syncDir
	syncErr := errors.New("sync failed")
	syncDir = func(string) error {
		return syncErr
	}
	defer func() {
		syncDir = original
	}()
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 64,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	err = spool.Append(context.Background(), []Event{{EventID: "evt-1", RequestPayload: "payload larger than the tiny spool budget"}})
	if !errors.Is(err, syncErr) {
		t.Fatalf("Append() error = %v, want sync error", err)
	}
}

func TestFileSpoolPersistsOverflowCountAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewFileSpool(FileSpoolOptions{
		Dir:      dir,
		MaxBytes: 64,
	})
	if err != nil {
		t.Fatalf("NewFileSpool(writer) error = %v", err)
	}

	err = writer.Append(context.Background(), []Event{{EventID: "evt-1", RequestPayload: "payload larger than the tiny spool budget"}})
	if !errors.Is(err, ErrSpoolFull) {
		t.Fatalf("Append() error = %v, want ErrSpoolFull", err)
	}

	reader, err := NewFileSpool(FileSpoolOptions{
		Dir:      dir,
		MaxBytes: 64,
	})
	if err != nil {
		t.Fatalf("NewFileSpool(reader) error = %v", err)
	}
	if stats := reader.Stats(); stats.OverflowCount != 1 {
		t.Fatalf("reader OverflowCount = %d, want 1", stats.OverflowCount)
	}
}

func TestNewFileSpoolLoadsExternalOverflowCount(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, overflowCountFile), []byte("7\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(overflow-count) error = %v", err)
	}

	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      dir,
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if stats := spool.Stats(); stats.OverflowCount != 7 {
		t.Fatalf("OverflowCount = %d, want 7", stats.OverflowCount)
	}
}

func TestFileSpoolStatsObservesNewerOverflowCountOnDisk(t *testing.T) {
	dir := t.TempDir()
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      dir,
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, overflowCountFile), []byte("9\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(overflow-count) error = %v", err)
	}

	if stats := spool.Stats(); stats.OverflowCount != 9 {
		t.Fatalf("OverflowCount = %d, want 9", stats.OverflowCount)
	}
}

func TestNewFileSpoolRejectsInvalidOverflowCount(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, overflowCountFile), []byte("not-a-number\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(overflow-count) error = %v", err)
	}

	_, err := NewFileSpool(FileSpoolOptions{
		Dir:      dir,
		MaxBytes: 1 << 20,
	})
	if err == nil {
		t.Fatal("NewFileSpool() error = nil, want invalid overflow count error")
	}
}

func TestFileSpoolStatsReportsDepthOldestAgeAndOverflow(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	const maxBytes = int64(1 << 20)
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: maxBytes,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	if err := spool.Append(context.Background(), []Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	now = now.Add(3 * time.Second)

	stats := spool.Stats()
	if stats.Depth != 1 {
		t.Fatalf("Depth = %d, want 1", stats.Depth)
	}
	if stats.MaxBytes != maxBytes {
		t.Fatalf("MaxBytes = %d, want %d", stats.MaxBytes, maxBytes)
	}
	if stats.OldestAge != 3*time.Second {
		t.Fatalf("OldestAge = %v, want 3s", stats.OldestAge)
	}
	if stats.Bytes == 0 {
		t.Fatal("Bytes = 0, want non-zero")
	}
}

func TestFileSpoolStatsSkipsMalformedRecords(t *testing.T) {
	dir := t.TempDir()
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      dir,
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile(bad record) error = %v", err)
	}

	stats := spool.Stats()
	if stats.Depth != 1 {
		t.Fatalf("Depth = %d, want 1", stats.Depth)
	}
	if stats.OldestAge != 0 {
		t.Fatalf("OldestAge = %v, want 0 for malformed record", stats.OldestAge)
	}
}

func TestFileSpoolAckRemovesRecords(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	batch, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if err := spool.Ack(ctx, batch); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
	if stats := spool.Stats(); stats.Depth != 0 {
		t.Fatalf("Depth = %d, want 0", stats.Depth)
	}
}

func TestFileSpoolAckIgnoresEmptyAndMissingPaths(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	events := []SpooledEvent{
		{},
		{Path: filepath.Join(t.TempDir(), "missing.json")},
	}

	if err := spool.Ack(context.Background(), events); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
}

func TestFileSpoolAckHonorsCanceledContext(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = spool.Ack(ctx, []SpooledEvent{{Path: filepath.Join(t.TempDir(), "record.json")}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Ack() error = %v, want context.Canceled", err)
	}
}

func TestFileSpoolAckReturnsDirectorySyncError(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := spool.Append(context.Background(), []Event{{EventID: "evt-1"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	batch, err := spool.NextBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	original := syncDir
	syncErr := errors.New("sync failed")
	syncDir = func(string) error {
		return syncErr
	}
	defer func() {
		syncDir = original
	}()

	err = spool.Ack(context.Background(), batch)
	if !errors.Is(err, syncErr) {
		t.Fatalf("Ack() error = %v, want sync error", err)
	}
}

func TestFileSpoolSyncsDirectoryAfterAppendAndAck(t *testing.T) {
	var syncCalls atomic.Int64
	original := syncDir
	syncDir = func(path string) error {
		if path != "" {
			syncCalls.Add(1)
		}
		return nil
	}
	defer func() {
		syncDir = original
	}()

	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	batch, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if err := spool.Ack(ctx, batch); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
	if got := syncCalls.Load(); got < 2 {
		t.Fatalf("syncCalls = %d, want at least 2", got)
	}
}

func TestFileSpoolRecordFailurePersistsRetryMetadata(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	batch, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if err := spool.RecordFailure(ctx, batch, errors.New("clickhouse down")); err != nil {
		t.Fatalf("RecordFailure() error = %v", err)
	}

	again, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch(after failure) error = %v", err)
	}
	if len(again) != 1 {
		t.Fatalf("len(again) = %d, want 1", len(again))
	}
	if again[0].RetryCount != 1 {
		t.Fatalf("RetryCount = %d, want 1", again[0].RetryCount)
	}
	if again[0].LastError != "clickhouse down" {
		t.Fatalf("LastError = %q, want clickhouse down", again[0].LastError)
	}
}

func TestFileSpoolRecordFailureIgnoresEmptyPath(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	if err := spool.RecordFailure(context.Background(), []SpooledEvent{{}}, errors.New("ignored")); err != nil {
		t.Fatalf("RecordFailure() error = %v", err)
	}
}

func TestFileSpoolRecordFailureHonorsCanceledContext(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = spool.RecordFailure(ctx, []SpooledEvent{{Path: filepath.Join(t.TempDir(), "record.json")}}, errors.New("ignored"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RecordFailure() error = %v, want context.Canceled", err)
	}
}

func TestFileSpoolRecordFailureWithNilCauseClearsLastError(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	batch, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if err := spool.RecordFailure(ctx, batch, errors.New("first failure")); err != nil {
		t.Fatalf("RecordFailure(first) error = %v", err)
	}
	again, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch(after first) error = %v", err)
	}
	if err := spool.RecordFailure(ctx, again, nil); err != nil {
		t.Fatalf("RecordFailure(nil cause) error = %v", err)
	}

	final, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch(final) error = %v", err)
	}
	if final[0].RetryCount != 2 {
		t.Fatalf("RetryCount = %d, want 2", final[0].RetryCount)
	}
	if final[0].LastError != "" {
		t.Fatalf("LastError = %q, want empty", final[0].LastError)
	}
}

func TestFileSpoolNextBatchRecoversStaleInflightRecords(t *testing.T) {
	now := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:          t.TempDir(),
		MaxBytes:     1 << 20,
		ClaimTimeout: time.Minute,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1", RequestID: "request-1"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	first, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch(first) error = %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("len(first) = %d, want 1", len(first))
	}

	stale := now.Add(-2 * time.Minute)
	if err := os.Chtimes(first[0].Path, stale, stale); err != nil {
		t.Fatalf("Chtimes(claimed path) error = %v", err)
	}

	second, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch(second) error = %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("len(second) = %d, want recovered record", len(second))
	}
	if second[0].Event.RequestID != "request-1" {
		t.Fatalf("RequestID = %q, want request-1", second[0].Event.RequestID)
	}
	if second[0].Path == first[0].Path {
		t.Fatalf("recovered claim path reused %q, want a new claim path", second[0].Path)
	}
}

func TestFileSpoolNextBatchLeavesRecentInflightRecordsClaimed(t *testing.T) {
	now := time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC)
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:          t.TempDir(),
		MaxBytes:     1 << 20,
		ClaimTimeout: time.Minute,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1", RequestID: "request-1"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	first, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch(first) error = %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("len(first) = %d, want 1", len(first))
	}

	second, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch(second) error = %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("len(second) = %d, want recent inflight record hidden", len(second))
	}
	if _, err := os.Stat(first[0].Path); err != nil {
		t.Fatalf("stat recent inflight path: %v", err)
	}
}

func TestFileSpoolPurgesExpiredRecordsBeforeAppendAndStats(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
		MaxAge:   time.Minute,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := spool.Append(context.Background(), []Event{{EventID: "old"}}); err != nil {
		t.Fatalf("Append(old) error = %v", err)
	}
	now = now.Add(2 * time.Minute)
	if stats := spool.Stats(); stats.Depth != 0 {
		t.Fatalf("Depth after Stats purge = %d, want 0", stats.Depth)
	}
	if err := spool.Append(context.Background(), []Event{{EventID: "fresh"}}); err != nil {
		t.Fatalf("Append(fresh) error = %v", err)
	}
	if stats := spool.Stats(); stats.Depth != 1 {
		t.Fatalf("Depth after fresh append = %d, want 1", stats.Depth)
	}
}

func TestFileSpoolExpiresRecordsOlderThanMaxAge(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
		MaxAge:   time.Second,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1", RequestID: "expired"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	now = now.Add(2 * time.Second)

	batch, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if len(batch) != 0 {
		t.Fatalf("len(batch) = %d, want 0", len(batch))
	}
	if stats := spool.Stats(); stats.Depth != 0 {
		t.Fatalf("Depth = %d, want 0", stats.Depth)
	}
}

func TestFileSpoolKeepsRecordsNewerThanMaxAge(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
		MaxAge:   time.Second,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	ctx := context.Background()
	if err := spool.Append(ctx, []Event{{EventID: "evt-1", RequestID: "fresh"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	now = now.Add(500 * time.Millisecond)

	batch, err := spool.NextBatch(ctx, 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("len(batch) = %d, want 1", len(batch))
	}
}

func TestFileSpoolRecordIDSanitizesPathSeparators(t *testing.T) {
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      t.TempDir(),
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}

	if err := spool.Append(context.Background(), []Event{{EventID: `route/v1:abc\def`}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	batch, err := spool.NextBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("NextBatch() error = %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("len(batch) = %d, want 1", len(batch))
	}
	if !strings.HasPrefix(filepath.Base(batch[0].Path), batch[0].ID+".inflight.") {
		t.Fatalf("Path = %q, ID = %q, want claimed path based on record ID", batch[0].Path, batch[0].ID)
	}
	if got := batch[0].ID; got != filepath.Base(got) {
		t.Fatalf("ID = %q, want no path separators", got)
	}
}

func TestFileSpoolNextBatchReturnsDecodeErrorForMalformedRecord(t *testing.T) {
	dir := t.TempDir()
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      dir,
		MaxBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile(bad record) error = %v", err)
	}

	_, err = spool.NextBatch(context.Background(), 1)
	if err == nil {
		t.Fatal("NextBatch() error = nil, want decode error")
	}
}

func TestFileSpoolPurgingExpiredRecordReturnsDecodeError(t *testing.T) {
	dir := t.TempDir()
	spool, err := NewFileSpool(FileSpoolOptions{
		Dir:      dir,
		MaxBytes: 1 << 20,
		MaxAge:   time.Second,
	})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	record := spoolRecord{
		ID:        "bad-time",
		CreatedAt: time.Now().Add(-2 * time.Second),
		Event:     Event{EventID: "bad-time"},
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal(record) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad-time.json"), payload[:len(payload)-1], 0o600); err != nil {
		t.Fatalf("WriteFile(truncated record) error = %v", err)
	}

	_, err = spool.NextBatch(context.Background(), 1)
	if err == nil {
		t.Fatal("NextBatch() error = nil, want purge decode error")
	}
}
