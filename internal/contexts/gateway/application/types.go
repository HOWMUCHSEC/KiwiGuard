// Package application owns transport-neutral gateway use cases.
package application

import (
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/routing/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

// RejectReason identifies why a gateway request cannot proceed.
type RejectReason string

const (
	// RejectUnsupportedPath means the request path is not part of the active gateway config.
	RejectUnsupportedPath RejectReason = "unsupported_path"
	// RejectMissingProvider means a configured route references an unavailable upstream provider.
	RejectMissingProvider RejectReason = "missing_provider"
	// RejectAuditSinkUnhealthy means fail-closed audit persistence is unavailable.
	RejectAuditSinkUnhealthy RejectReason = "audit_sink_unhealthy"
	// RejectMissingModelMapping means the requested model cannot be mapped for the route.
	RejectMissingModelMapping RejectReason = "missing_model_mapping"
	// RejectMissingClientKey means no gateway client key was supplied.
	RejectMissingClientKey RejectReason = "missing_client_key"
	// RejectInvalidClientKey means the supplied gateway client key is unknown or invalid.
	RejectInvalidClientKey RejectReason = "invalid_client_key"
	// RejectDisabledClientKey means the gateway client key is disabled.
	RejectDisabledClientKey RejectReason = "disabled_client_key"
	// RejectRevokedClientKey means the gateway client key has been revoked.
	RejectRevokedClientKey RejectReason = "revoked_client_key"
	// RejectMissingLimitPolicy means a protected route has no active limit policy.
	RejectMissingLimitPolicy RejectReason = "missing_limit_policy"
	// RejectRateLimitExceeded means the client exhausted the route request window.
	RejectRateLimitExceeded RejectReason = "rate_limit_exceeded"
	// RejectConcurrencyLimitExceeded means the client exhausted concurrent route capacity.
	RejectConcurrencyLimitExceeded RejectReason = "concurrency_limit_exceeded"
	// RejectInvalidJSON means the OpenAI-compatible request body cannot be decoded.
	RejectInvalidJSON RejectReason = "invalid_json"
	// RejectInvalidRequest means the request cannot be converted to the upstream contract.
	RejectInvalidRequest RejectReason = "invalid_request"
	// RejectRequestTooLarge means the request exceeded the configured body limit.
	RejectRequestTooLarge RejectReason = "request_too_large"
	// RejectRedactionFailed means a redaction policy could not be applied safely.
	RejectRedactionFailed RejectReason = "redaction_failed"
	// RejectUpstreamError means the upstream request or response failed.
	RejectUpstreamError RejectReason = "upstream_error"
	// RejectUpstreamResponseTooLarge means the upstream response exceeded configured limits.
	RejectUpstreamResponseTooLarge RejectReason = "upstream_response_too_large"
	// RejectBlockedInput means policy blocked the request before upstream forwarding.
	RejectBlockedInput RejectReason = "blocked_input"
	// RejectBlockedOutput means policy blocked the upstream response.
	RejectBlockedOutput RejectReason = "blocked_output"
)

// ClientIdentity identifies the caller admitted through gateway client authentication.
type ClientIdentity struct {
	ID string
}

// LimitPolicy captures the resolved request, concurrency, and body limits for one admitted route.
type LimitPolicy struct {
	RequestsPerWindow     int
	Window                time.Duration
	MaxConcurrentRequests int
	MaxBodyBytes          int64
}

// AdmissionInput supplies the facts required to authenticate and rate-limit a protected request.
type AdmissionInput struct {
	ClientKey           string
	RouteKey            string
	DefaultMaxBodyBytes int64
	Now                 time.Time
}

// AdmissionResult is the transport-neutral result of protected route admission.
type AdmissionResult struct {
	Allowed      bool
	Reason       RejectReason
	ClientID     string
	MaxBodyBytes int64
	Release      func()
}

// PolicyEvaluationInput describes one traffic payload as seen by the policy engine.
type PolicyEvaluationInput struct {
	RouteKey       string
	ProviderKey    string
	Model          string
	Direction      detection.Direction
	Text           string
	ModelSignal    policy.ModelSignal
	FallbackAction policy.Action
}

// LifecycleDecision is the transport-neutral result of a gateway lifecycle gate.
type LifecycleDecision struct {
	Allowed bool
	Reason  RejectReason
}

// ModelMappingInput captures the route-specific model mapping lookup result.
type ModelMappingInput struct {
	RequestedModel string
	MappedModel    string
	UpstreamModel  string
	Found          bool
}

// ModelMappingResult contains the normalized model mapping for a gateway exchange.
type ModelMappingResult struct {
	LifecycleDecision
	RequestedModel string
	MappedModel    string
	UpstreamModel  string
}

// DecodedRequestInput combines decode status with the resolved model-mapping outcome.
type DecodedRequestInput struct {
	DecodeFailed bool
	ModelMapping ModelMappingInput
}

// EnforcementResult classifies the next lifecycle step for a policy decision.
type EnforcementResult struct {
	Blocked          bool
	RedactionNeeded  bool
	Reason           RejectReason
	AllowObservation bool
}

// ExchangeInfrastructureInput summarizes upstream and audit dependencies required before forwarding.
type ExchangeInfrastructureInput struct {
	ProviderFound bool
	AuditHealthy  bool
}

// PolicyEnforcementInput packages the policy verdict and redaction attempt for one traffic direction.
type PolicyEnforcementInput struct {
	Direction           detection.Direction
	Decision            policy.Decision
	RedactionAttempted  bool
	RedactionText       string
	RedactionCount      int
	RedactionFailed     bool
	RedactionFailureTag string
}

// PolicyEnforcementResult classifies whether traffic may continue after policy evaluation.
type PolicyEnforcementResult struct {
	EnforcementResult
	RedactionRejected bool
}

// InputRequestLifecycleInput captures the policy and redaction outcomes observed on an input request.
type InputRequestLifecycleInput struct {
	PolicyEvaluated     bool
	Decision            policy.Decision
	RedactionAttempted  bool
	RedactionText       string
	RedactionCount      int
	RedactionFailed     bool
	RedactionFailureTag string
}

// InputRequestLifecycleResult classifies the next step for an input request.
type InputRequestLifecycleResult struct {
	LifecycleDecision
	PolicyDecisionObserved bool
	RedactionNeeded        bool
	RedactionRejected      bool
}

// UpstreamResponseInput summarizes the read and decode state of an upstream response.
type UpstreamResponseInput struct {
	StatusCode   int
	ReadFailed   bool
	TooLarge     bool
	DecodeFailed bool
}

// OutputResponseLifecycleInput combines upstream response status with output policy evaluation results.
type OutputResponseLifecycleInput struct {
	UpstreamResponseInput
	PolicyEvaluated     bool
	Decision            policy.Decision
	RedactionAttempted  bool
	RedactionText       string
	RedactionCount      int
	RedactionFailed     bool
	RedactionFailureTag string
}

// OutputResponseLifecycleResult classifies the next step for a non-streaming output.
type OutputResponseLifecycleResult struct {
	LifecycleDecision
	PolicyDecisionObserved bool
	RedactionNeeded        bool
	RedactionRejected      bool
}

// UpstreamRequestInput reports whether request projection into the upstream contract succeeded.
type UpstreamRequestInput struct {
	ProjectionFailed bool
}

// UpstreamExchangeInput captures the transport outcome after an upstream round-trip begins.
type UpstreamExchangeInput struct {
	ForwardFailed bool
	StatusCode    int
	Stream        bool
}

// UpstreamExchangeResult classifies how an upstream exchange should continue.
type UpstreamExchangeResult struct {
	LifecycleDecision
	Stream bool
}

// RequestBodyInput reports whether the incoming request body could be read within configured limits.
type RequestBodyInput struct {
	ReadFailed bool
	TooLarge   bool
}

// RedactionInput describes the outcome of one redaction attempt before policy enforcement proceeds.
type RedactionInput struct {
	Text       string
	Redactions int
	Failed     bool
}

// StreamingLifecycleDecision classifies whether streaming output must terminate.
type StreamingLifecycleDecision struct {
	Terminated        bool
	TerminationReason string
}

// StreamingDeltaInput captures the policy and buffering outcome for one streamed output delta.
type StreamingDeltaInput struct {
	DeltaObserved      bool
	PolicyDecision     policy.Decision
	CollectionAccepted bool
}

// TrafficEvaluationInput describes one request or response payload submitted for traffic evaluation.
type TrafficEvaluationInput struct {
	RequestID       string
	CorrelationID   string
	RouteKey        string
	ProviderKey     string
	Model           string
	Direction       detection.Direction
	Text            string
	Execution       routing.ExecutionMode
	FallbackAction  routing.Action
	VerdictProvider verdict.Provider
}

// TrafficEvaluationResult contains verdict and policy outcomes for one traffic payload.
type TrafficEvaluationResult struct {
	Direction       detection.Direction
	Text            string
	Verdict         verdict.Result
	Decision        policy.Decision
	DetectorLatency time.Duration
	VerdictLatency  time.Duration
}

// TrafficEventClassificationInput contains facts used to classify an emitted gateway event.
type TrafficEventClassificationInput struct {
	Direction         detection.Direction
	Decision          policy.Decision
	Verdict           verdict.Result
	TerminationReason string
}

// TrafficEventClassification contains derived observability fields for a gateway event.
type TrafficEventClassification struct {
	ErrorType          string
	BlockReason        string
	FallbackTriggered  bool
	MatchedSpanCount   uint32
	DetectorCategories []string
	PolicyRuleIDs      []string
	Action             traffic.Action
}
