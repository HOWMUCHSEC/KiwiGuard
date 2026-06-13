package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// writeRecord persists one spool record atomically to disk.
func (s *FileSpool) writeRecord(id string, payload []byte) error {
	path := filepath.Join(s.dir, id+".json")
	tmp, err := os.CreateTemp(s.dir, id+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create event spool temp record: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write event spool record: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync event spool record: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close event spool record: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("commit event spool record: %w", err)
	}
	if err := syncDir(s.dir); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// readRecord decodes one persisted spool record and reports its on-disk byte size.
func (s *FileSpool) readRecord(path string) (spoolRecord, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return spoolRecord{}, 0, fmt.Errorf("read event spool record: %w", err)
	}
	var record spoolRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return spoolRecord{}, 0, fmt.Errorf("decode event spool record: %w", err)
	}
	return record, int64(len(data)), nil
}

// recordFiles returns committed spool record paths in deterministic replay order.
func (s *FileSpool) recordFiles() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read event spool dir: %w", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.Contains(entry.Name(), ".inflight.") {
			continue
		}
		files = append(files, filepath.Join(s.dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// purgeExpiredLocked removes expired spool records while the caller holds the spool lock.
func (s *FileSpool) purgeExpiredLocked(ctx context.Context) error {
	if s.maxAge <= 0 {
		return nil
	}
	entries, err := s.recordFiles()
	if err != nil {
		return err
	}
	cutoff := s.now().UTC().Add(-s.maxAge)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		record, _, err := s.readRecord(entry)
		if err != nil {
			return err
		}
		if record.CreatedAt.After(cutoff) {
			continue
		}
		if err := os.Remove(entry); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove expired event spool record: %w", err)
		}
	}
	return nil
}

// recordID builds a stable, sortable filename prefix for one spooled event.
func (s *FileSpool) recordID(createdAt time.Time, eventID string) string {
	seq := s.sequence.Add(1)
	if eventID == "" {
		eventID = "event"
	}
	eventID = strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(eventID)
	return fmt.Sprintf("%020d-%016d-%s", createdAt.UnixNano(), seq, eventID)
}
