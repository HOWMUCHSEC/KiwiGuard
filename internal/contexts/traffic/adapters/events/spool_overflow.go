package events

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// incrementOverflowCountLocked increments and persists the overflow counter under the spool lock.
func (s *FileSpool) incrementOverflowCountLocked() error {
	count := s.overflowCount.Add(1)
	if err := s.writeOverflowCount(count); err != nil {
		return err
	}
	return nil
}

// currentOverflowCountLocked reports the most recent overflow counter while the spool lock is held.
func (s *FileSpool) currentOverflowCountLocked() uint64 {
	stored, err := s.readOverflowCount()
	if err != nil {
		return s.overflowCount.Load()
	}
	current := s.overflowCount.Load()
	if stored > current {
		s.overflowCount.Store(stored)
		return stored
	}
	return current
}

// readOverflowCount reads the persisted overflow counter from disk.
func (s *FileSpool) readOverflowCount() (uint64, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, overflowCountFile))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read event spool overflow count: %w", err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return 0, nil
	}
	count, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("decode event spool overflow count: %w", err)
	}
	return count, nil
}

// writeOverflowCount persists the overflow counter atomically to disk.
func (s *FileSpool) writeOverflowCount(count uint64) error {
	payload := []byte(strconv.FormatUint(count, 10) + "\n")
	tmp, err := os.CreateTemp(s.dir, overflowCountFile+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create event spool overflow temp file: %w", err)
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
		return fmt.Errorf("write event spool overflow count: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync event spool overflow count: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close event spool overflow count: %w", err)
	}
	if err := os.Rename(tmpPath, filepath.Join(s.dir, overflowCountFile)); err != nil {
		return fmt.Errorf("commit event spool overflow count: %w", err)
	}
	if err := syncDir(s.dir); err != nil {
		return err
	}
	cleanup = false
	return nil
}
