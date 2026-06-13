// Package verdict defines contracts for vertical security model evaluation.
package verdict

import (
	"context"
	"time"
)

// Direction identifies whether text is entering or leaving the gateway.
type Direction string

// Action is the enforcement action suggested by a verdict provider.
type Action string

// RiskLevel is the normalized risk level returned by a verdict provider.
type RiskLevel string

const (
	// DirectionInput identifies model input text.
	DirectionInput Direction = "input"
	// DirectionOutput identifies model output text.
	DirectionOutput Direction = "output"

	// ActionAllow permits the inspected text.
	ActionAllow Action = "allow"
	// ActionBlock blocks the inspected text.
	ActionBlock Action = "block"
	// ActionRedact allows the text after redaction.
	ActionRedact Action = "redact"
	// ActionShadowLog records the verdict without enforcing it.
	ActionShadowLog Action = "shadow_log"

	// RiskUnknown indicates the provider could not determine risk.
	RiskUnknown RiskLevel = "unknown"
	// RiskLow indicates low risk.
	RiskLow RiskLevel = "low"
	// RiskMedium indicates medium risk.
	RiskMedium RiskLevel = "medium"
	// RiskHigh indicates high risk.
	RiskHigh RiskLevel = "high"
	// RiskCritical indicates critical risk.
	RiskCritical RiskLevel = "critical"
)

// Request is the normalized input sent to a verdict provider.
type Request struct {
	RequestID     string
	CorrelationID string
	RouteKey      string
	ProviderKey   string
	Model         string
	Direction     Direction
	Text          string
	Metadata      map[string]string
}

// MatchedSpan describes a provider match using byte offsets without raw matched text.
type MatchedSpan struct {
	Start    int
	End      int
	Category string
	TextHash string
}

// Result is the normalized verdict returned by a provider.
type Result struct {
	RiskLevel       RiskLevel
	Categories      []string
	Confidence      float64
	SuggestedAction Action
	MatchedSpans    []MatchedSpan
	Rationale       string
	Latency         time.Duration
	ProviderName    string
	FallbackUsed    bool
	Error           string
}

// Provider evaluates text and returns a normalized verdict.
type Provider interface {
	Evaluate(context.Context, Request) (Result, error)
}

// ProviderFunc adapts a function to the Provider interface.
type ProviderFunc func(context.Context, Request) (Result, error)

// Evaluate calls f(ctx, request).
func (f ProviderFunc) Evaluate(ctx context.Context, request Request) (Result, error) {
	return f(ctx, request)
}
