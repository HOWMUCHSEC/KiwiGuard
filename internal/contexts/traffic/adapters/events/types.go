// Package events defines the asynchronous pipeline boundary for traffic events.
package events

import (
	"context"

	traffic "github.com/howmuchsec/kiwiguard/internal/contexts/traffic/domain"
)

type (
	// Direction preserves the previous event direction type while traffic facts
	// move into the traffic domain context.
	Direction = traffic.Direction
	// Action preserves the previous event action type while traffic facts move
	// into the traffic domain context.
	Action = traffic.Action
	// Event preserves the previous event payload type while traffic facts move
	// into the traffic domain context.
	Event = traffic.Event
)

// Writer accepts gateway events without requiring callers to know the sink.
type Writer interface {
	Enqueue(context.Context, traffic.Event) error
}

// BatchSink accepts flushed event batches from the pipeline.
type BatchSink interface {
	WriteBatch(context.Context, []traffic.Event) error
}

// HashBody returns a lowercase SHA-256 hex digest for body content.
func HashBody(body []byte) string {
	return traffic.HashBody(body)
}
