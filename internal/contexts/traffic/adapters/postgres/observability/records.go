// Package observability contains PostgreSQL record shapes for traffic observability configuration.
package observability

import "encoding/json"

// Sink contains event sink configuration.
type Sink struct {
	ID      string
	Name    string
	Kind    string
	Enabled bool
	Config  json.RawMessage
}

// RetentionPolicy contains event retention settings.
type RetentionPolicy struct {
	ID            string
	Name          string
	SinkID        string
	EventType     string
	RetentionDays int
}

// RawCapturePolicy contains raw traffic capture settings.
type RawCapturePolicy struct {
	ID            string
	Name          string
	RouteID       string
	Direction     string
	Enabled       bool
	SampleRate    float64
	RedactionMode string
}
