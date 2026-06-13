package events

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// lockDirectory acquires the process-wide spool directory lock and returns its unlock function.
func (s *FileSpool) lockDirectory(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return func() {}, err
	}
	lock, err := os.OpenFile(filepath.Join(s.dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}, fmt.Errorf("open event spool lock: %w", err)
	}
	if err := lockDirectoryFile(lock); err != nil {
		_ = lock.Close()
		return func() {}, fmt.Errorf("lock event spool dir: %w", err)
	}
	return func() {
		_ = unlockDirectoryFile(lock)
		_ = lock.Close()
	}, nil
}

// claimRecord moves a committed record into the inflight namespace for replay ownership.
func (s *FileSpool) claimRecord(path string) (string, bool, error) {
	claimedPath := s.claimPath(path)
	if err := os.Rename(path, claimedPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("claim event spool record: %w", err)
	}
	if err := syncDir(s.dir); err != nil {
		_ = s.restoreClaimedRecord(claimedPath)
		return "", false, err
	}
	return claimedPath, true, nil
}

// restoreClaimedRecord moves an inflight record back to its committed path.
func (s *FileSpool) restoreClaimedRecord(path string) error {
	originalPath := s.originalRecordPath(path)
	if err := os.Rename(path, originalPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("restore claimed event spool record: %w", err)
	}
	return syncDir(s.dir)
}

func (s *FileSpool) claimPath(path string) string {
	base := strings.TrimSuffix(path, ".json")
	return fmt.Sprintf("%s.inflight.%d.%d", base, os.Getpid(), s.sequence.Add(1))
}

func (s *FileSpool) originalRecordPath(path string) string {
	if base, _, ok := strings.Cut(path, ".inflight."); ok {
		return base + ".json"
	}
	return path
}

func (s *FileSpool) recoverStaleClaimsLocked(ctx context.Context) error {
	if s.claimTimeout <= 0 {
		return nil
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read event spool dir: %w", err)
	}
	cutoff := s.now().UTC().Add(-s.claimTimeout)
	recovered := false
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || !strings.Contains(entry.Name(), ".inflight.") {
			continue
		}
		path := filepath.Join(s.dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("stat inflight event spool record: %w", err)
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := s.restoreStaleClaim(path); err != nil {
			return err
		}
		recovered = true
	}
	if recovered {
		return syncDir(s.dir)
	}
	return nil
}

func (s *FileSpool) restoreStaleClaim(path string) error {
	originalPath := s.originalRecordPath(path)
	if _, err := os.Stat(originalPath); err == nil {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove duplicate inflight event spool record: %w", err)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat original event spool record: %w", err)
	}
	if err := os.Rename(path, originalPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("recover inflight event spool record: %w", err)
	}
	return nil
}
