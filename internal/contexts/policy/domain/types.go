// Package policy compiles immutable policy snapshots and evaluates traffic.
package policy

import detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"

// Action identifies the enforcement action selected by a policy decision.
type Action string

// Severity identifies the impact level assigned to a rule hit.
type Severity string

// Source identifies where a policy bundle originated.
type Source string

const (
	// ActionAllow permits traffic without enforcement.
	ActionAllow Action = "allow"
	// ActionBlock rejects traffic.
	ActionBlock Action = "block"
	// ActionRedact removes sensitive matched content.
	ActionRedact Action = "redact"
	// ActionShadowLog records findings without changing traffic.
	ActionShadowLog Action = "shadow_log"

	// SeverityLow identifies low-severity rule hits.
	SeverityLow Severity = "low"
	// SeverityMedium identifies medium-severity rule hits.
	SeverityMedium Severity = "medium"
	// SeverityHigh identifies high-severity rule hits.
	SeverityHigh Severity = "high"
	// SeverityCritical identifies critical-severity rule hits.
	SeverityCritical Severity = "critical"

	// SourceBuiltIn identifies built-in policy bundles.
	SourceBuiltIn Source = "built_in"
	// SourceUser identifies user-authored policy bundles.
	SourceUser Source = "user"
	// SourceImported identifies externally imported policy bundles.
	SourceImported Source = "imported"
)

// Scope limits a rule to matching request metadata. Empty fields are wildcards.
type Scope struct {
	RouteKey  string
	Provider  string
	Model     string
	Direction detection.Direction
}

// Rule connects detector findings to a severity and enforcement action.
type Rule struct {
	Key          string
	Enabled      bool
	Severity     Severity
	Action       Action
	DetectorKeys []string
	Scope        Scope
}

// Bundle groups detector definitions and rules under a versioned policy key.
type Bundle struct {
	Key           string
	Version       string
	Source        Source
	DefaultAction Action
	Detectors     []detection.Definition
	Rules         []Rule
}

// ModelSignal contains a normalized security model recommendation.
type ModelSignal struct {
	SuggestedAction Action
	RiskLevel       string
	Categories      []string
	Confidence      float64
	FallbackAction  Action
	FallbackUsed    bool
	Error           string
}

// EvaluationRequest contains request metadata and text to evaluate.
type EvaluationRequest struct {
	RouteKey    string
	Provider    string
	Model       string
	Direction   detection.Direction
	Text        string
	ModelSignal ModelSignal
}

// RuleHit records a policy rule that matched detector findings.
type RuleHit struct {
	BundleKey string
	RuleKey   string
	Severity  Severity
	Action    Action
	Findings  []detection.Finding
}

// Decision is the immutable result of evaluating a snapshot against a request.
type Decision struct {
	Action             Action
	DefaultAction      Action
	RuleHits           []RuleHit
	Findings           []detection.Finding
	ModelSignal        ModelSignal
	ModelSignalApplied bool
	SnapshotHash       string
	BundleKeys         []string
}
