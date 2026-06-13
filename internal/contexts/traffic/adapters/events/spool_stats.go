package events

import (
	"os"
	"time"
)

// statsLocked summarizes the spool contents while the caller holds the spool lock.
func (s *FileSpool) statsLocked() SpoolStats {
	entries, err := s.recordFiles()
	if err != nil {
		return SpoolStats{MaxBytes: s.maxBytes, OverflowCount: s.currentOverflowCountLocked()}
	}

	stats := SpoolStats{
		Depth:         len(entries),
		MaxBytes:      s.maxBytes,
		OverflowCount: s.currentOverflowCountLocked(),
	}
	var oldest time.Time
	for _, entry := range entries {
		info, err := os.Stat(entry)
		if err != nil {
			continue
		}
		stats.Bytes += info.Size()
		record, _, err := s.readRecord(entry)
		if err != nil {
			continue
		}
		if oldest.IsZero() || record.CreatedAt.Before(oldest) {
			oldest = record.CreatedAt
		}
	}
	if !oldest.IsZero() {
		stats.OldestAge = s.now().UTC().Sub(oldest)
	}
	return stats
}
