package events

import (
	"sync/atomic"
)

// HealthGate tracks whether the event sink is healthy enough for strict runtime use.
type HealthGate struct {
	healthy atomic.Bool
	reason  atomic.Value
}

// NewHealthGate initializes event-sink health tracking in a healthy state.
func NewHealthGate() *HealthGate {
	gate := &HealthGate{}
	gate.healthy.Store(true)
	gate.reason.Store("")
	return gate
}

// Healthy reports whether the event sink is currently healthy.
func (g *HealthGate) Healthy() bool {
	if g == nil {
		return true
	}
	return g.healthy.Load()
}

// Reason returns the latest unhealthy reason, or an empty string when healthy.
func (g *HealthGate) Reason() string {
	if g == nil {
		return ""
	}
	value := g.reason.Load()
	if value == nil {
		return ""
	}
	reason, _ := value.(string)
	return reason
}

// MarkHealthy clears any unhealthy reason.
func (g *HealthGate) MarkHealthy() {
	if g == nil {
		return
	}
	g.reason.Store("")
	g.healthy.Store(true)
}

// MarkUnhealthy records why the event sink is unhealthy.
func (g *HealthGate) MarkUnhealthy(reason string) {
	if g == nil {
		return
	}
	g.reason.Store(reason)
	g.healthy.Store(false)
}
