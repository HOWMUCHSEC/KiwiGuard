package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	overflowCountFile        = "overflow-count"
	defaultSpoolClaimTimeout = 5 * time.Minute
)

// ErrSpoolFull reports that appending records would exceed the configured spool limit.
var ErrSpoolFull = errors.New("event spool full")

// Spool defines the durable queue used when primary event delivery cannot succeed inline.
type Spool interface {
	Append(context.Context, []Event) error
	NextBatch(context.Context, int) ([]SpooledEvent, error)
	RecordFailure(context.Context, []SpooledEvent, error) error
	Ack(context.Context, []SpooledEvent) error
	Stats() SpoolStats
}

// FileSpoolOptions defines directory layout, retention, and limits for the filesystem-backed spool.
type FileSpoolOptions struct {
	Dir          string
	MaxBytes     int64
	MaxAge       time.Duration
	ClaimTimeout time.Duration
	Now          func() time.Time
}

// SpooledEvent is one durable event record selected for replay.
type SpooledEvent struct {
	ID         string
	Path       string
	Event      Event
	CreatedAt  time.Time
	RetryCount uint16
	LastError  string
	SizeBytes  int64
}

// SpoolStats captures durable spool health and capacity state.
type SpoolStats struct {
	Depth         int
	Bytes         int64
	MaxBytes      int64
	OldestAge     time.Duration
	OverflowCount uint64
}

type spoolRecord struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	RetryCount uint16    `json:"retry_count"`
	LastError  string    `json:"last_error,omitempty"`
	Event      Event     `json:"event"`
}

var syncDir = func(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open event spool dir: %w", err)
	}
	defer func() {
		_ = dir.Close()
	}()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync event spool dir: %w", err)
	}
	return nil
}

// FileSpool persists one event record per file inside a single spool directory.
type FileSpool struct {
	mu            sync.Mutex
	dir           string
	maxBytes      int64
	maxAge        time.Duration
	claimTimeout  time.Duration
	now           func() time.Time
	sequence      atomic.Uint64
	overflowCount atomic.Uint64
}

// NewFileSpool opens the filesystem-backed spool used for durable event buffering.
func NewFileSpool(opts FileSpoolOptions) (*FileSpool, error) {
	if opts.Dir == "" {
		return nil, errors.New("event spool dir is required")
	}
	if opts.MaxBytes <= 0 {
		return nil, errors.New("event spool max bytes must be positive")
	}
	if err := os.MkdirAll(opts.Dir, 0o750); err != nil {
		return nil, fmt.Errorf("create event spool dir: %w", err)
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	claimTimeout := opts.ClaimTimeout
	if claimTimeout <= 0 {
		claimTimeout = defaultSpoolClaimTimeout
	}
	spool := &FileSpool{
		dir:          opts.Dir,
		maxBytes:     opts.MaxBytes,
		maxAge:       opts.MaxAge,
		claimTimeout: claimTimeout,
		now:          now,
	}
	if count, err := spool.readOverflowCount(); err == nil {
		spool.overflowCount.Store(count)
	} else {
		return nil, err
	}
	return spool, nil
}

// Append writes events as durable records. The batch is rejected if it cannot fit.
func (s *FileSpool) Append(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	unlock, err := s.lockDirectory(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.recoverStaleClaimsLocked(ctx); err != nil {
		return err
	}
	if err := s.purgeExpiredLocked(ctx); err != nil {
		return err
	}

	now := s.now().UTC()
	records := make([]spoolRecord, 0, len(events))
	payloads := make([][]byte, 0, len(events))
	var addBytes int64
	for _, event := range events {
		id := s.recordID(now, event.EventID)
		record := spoolRecord{
			ID:        id,
			CreatedAt: now,
			Event:     event,
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal event spool record: %w", err)
		}
		payload = append(payload, '\n')
		addBytes += int64(len(payload))
		records = append(records, record)
		payloads = append(payloads, payload)
	}

	stats := s.statsLocked()
	if stats.Bytes+addBytes > s.maxBytes {
		if err := s.incrementOverflowCountLocked(); err != nil {
			return err
		}
		return fmt.Errorf("%w: need %d bytes with %d available", ErrSpoolFull, addBytes, s.maxBytes-stats.Bytes)
	}

	for i, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.writeRecord(record.ID, payloads[i]); err != nil {
			return err
		}
	}
	return nil
}

// NextBatch claims the oldest replayable records, up to the requested limit.
func (s *FileSpool) NextBatch(ctx context.Context, limit int) ([]SpooledEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := s.lockDirectory(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if limit <= 0 {
		limit = 1
	}
	if err := s.recoverStaleClaimsLocked(ctx); err != nil {
		return nil, err
	}
	if err := s.purgeExpiredLocked(ctx); err != nil {
		return nil, err
	}
	entries, err := s.recordFiles()
	if err != nil {
		return nil, err
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}

	batch := make([]SpooledEvent, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		claimedPath, claimed, err := s.claimRecord(entry)
		if err != nil {
			return nil, err
		}
		if !claimed {
			continue
		}
		record, size, err := s.readRecord(claimedPath)
		if err != nil {
			_ = s.restoreClaimedRecord(claimedPath)
			return nil, err
		}
		batch = append(batch, SpooledEvent{
			ID:         record.ID,
			Path:       claimedPath,
			Event:      record.Event,
			CreatedAt:  record.CreatedAt,
			RetryCount: record.RetryCount,
			LastError:  record.LastError,
			SizeBytes:  size,
		})
	}
	return batch, nil
}

// Ack removes records after they have been successfully replayed.
func (s *FileSpool) Ack(ctx context.Context, events []SpooledEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := s.lockDirectory(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	removed := false
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return err
		}
		if event.Path == "" {
			continue
		}
		if err := os.Remove(event.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove event spool record: %w", err)
		}
		removed = true
	}
	if removed {
		return syncDir(s.dir)
	}
	return nil
}

// RecordFailure persists replay failure metadata for later diagnostics.
func (s *FileSpool) RecordFailure(ctx context.Context, events []SpooledEvent, cause error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := s.lockDirectory(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	lastError := ""
	if cause != nil {
		lastError = cause.Error()
	}
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return err
		}
		if event.Path == "" {
			continue
		}
		record, _, err := s.readRecord(event.Path)
		if err != nil {
			return err
		}
		record.RetryCount++
		record.LastError = lastError
		payload, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal event spool retry metadata: %w", err)
		}
		payload = append(payload, '\n')
		if err := s.writeRecord(record.ID, payload); err != nil {
			return err
		}
		if err := os.Remove(event.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove claimed event spool record: %w", err)
		}
	}
	return nil
}

// Stats returns current filesystem spool state.
func (s *FileSpool) Stats() SpoolStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := s.lockDirectory(context.Background())
	if err != nil {
		return SpoolStats{MaxBytes: s.maxBytes, OverflowCount: s.overflowCount.Load()}
	}
	defer unlock()
	_ = s.recoverStaleClaimsLocked(context.Background())
	_ = s.purgeExpiredLocked(context.Background())
	return s.statsLocked()
}
