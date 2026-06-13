// Package revisionstore owns PostgreSQL revision metadata and unit-of-work primitives.
package revisionstore

import "time"

// ConfigRevision identifies a versioned runtime configuration.
type ConfigRevision struct {
	ID                   string
	Number               int64
	Source               string
	Status               string
	Actor                string
	CompiledSnapshotHash string
	CompiledSnapshotRef  string
	ActivatedAt          time.Time
}
