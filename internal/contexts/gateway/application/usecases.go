package application

import (
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/routing/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

// Authenticator verifies gateway client credentials.
type Authenticator interface {
	AuthenticateClient(key string) (ClientIdentity, RejectReason)
}

// LimitResolver resolves the limit policy that applies to one client on one route.
type LimitResolver interface {
	EffectiveLimitPolicy(clientID, routeKey string) (LimitPolicy, bool)
}

// RateLimiter admits requests within a route rate window.
type RateLimiter interface {
	AllowRate(now time.Time, clientID, routeKey string, policy LimitPolicy) bool
}

// ConcurrencyLimiter reserves concurrent route capacity.
type ConcurrencyLimiter interface {
	AcquireConcurrency(clientID, routeKey string, policy LimitPolicy) (func(), bool)
}

// AuditHealth reports whether fail-closed audit persistence may accept traffic.
type AuditHealth interface {
	Healthy() bool
}

// AdmissionUseCase admits protected gateway requests before body processing.
type AdmissionUseCase struct {
	Authenticator      Authenticator
	Limits             LimitResolver
	RateLimiter        RateLimiter
	ConcurrencyLimiter ConcurrencyLimiter
	Audit              AuditHealth
}

// AdmitProtectedRoute authenticates a client, resolves limits, and reserves capacity.
func (u AdmissionUseCase) AdmitProtectedRoute(input AdmissionInput) AdmissionResult {
	reject := func(reason RejectReason, clientID string) AdmissionResult {
		return AdmissionResult{Reason: reason, ClientID: clientID, Release: noopRelease}
	}

	if u.Authenticator == nil {
		return reject(RejectInvalidClientKey, "")
	}
	client, reason := u.Authenticator.AuthenticateClient(input.ClientKey)
	if reason != "" {
		return reject(reason, "")
	}

	if u.Limits == nil {
		return reject(RejectMissingLimitPolicy, client.ID)
	}
	effective, ok := u.Limits.EffectiveLimitPolicy(client.ID, input.RouteKey)
	if !ok {
		return reject(RejectMissingLimitPolicy, client.ID)
	}

	if u.Audit != nil && !u.Audit.Healthy() {
		return reject(RejectAuditSinkUnhealthy, client.ID)
	}
	if u.RateLimiter == nil || !u.RateLimiter.AllowRate(input.Now, client.ID, input.RouteKey, effective) {
		return reject(RejectRateLimitExceeded, client.ID)
	}
	if u.ConcurrencyLimiter == nil {
		return reject(RejectConcurrencyLimitExceeded, client.ID)
	}
	release, ok := u.ConcurrencyLimiter.AcquireConcurrency(client.ID, input.RouteKey, effective)
	if !ok {
		return reject(RejectConcurrencyLimitExceeded, client.ID)
	}
	if release == nil {
		release = noopRelease
	}

	maxBodyBytes := effective.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = input.DefaultMaxBodyBytes
	}
	return AdmissionResult{
		Allowed:      true,
		ClientID:     client.ID,
		MaxBodyBytes: maxBodyBytes,
		Release:      release,
	}
}

// PolicySnapshot evaluates traffic against the compiled active policy snapshot.
type PolicySnapshot interface {
	Evaluate(policy.EvaluationRequest) policy.Decision
}

// PolicyEvaluator evaluates gateway traffic against policy and model signals.
type PolicyEvaluator struct {
	Snapshot PolicySnapshot
}

// Evaluate runs one gateway traffic payload through policy evaluation and model-signal fallback rules.
func (e PolicyEvaluator) Evaluate(input PolicyEvaluationInput) policy.Decision {
	signal := input.ModelSignal
	if signal.FallbackAction == "" {
		signal.FallbackAction = input.FallbackAction
	}
	if e.Snapshot == nil {
		return DecisionFromModelSignal(signal)
	}
	return e.Snapshot.Evaluate(policy.EvaluationRequest{
		RouteKey:    input.RouteKey,
		Provider:    input.ProviderKey,
		Model:       input.Model,
		Direction:   input.Direction,
		Text:        input.Text,
		ModelSignal: signal,
	})
}

// ModelSignalFromVerdict normalizes a vertical model result into policy input.
func ModelSignalFromVerdict(result verdict.Result, err error, fallback routing.Action) policy.ModelSignal {
	signal := policy.ModelSignal{
		SuggestedAction: PolicyAction(routing.Action(result.SuggestedAction)),
		RiskLevel:       string(result.RiskLevel),
		Categories:      append([]string(nil), result.Categories...),
		Confidence:      result.Confidence,
		FallbackAction:  PolicyAction(fallback),
		FallbackUsed:    result.FallbackUsed,
		Error:           result.Error,
	}
	if err != nil {
		signal.FallbackUsed = true
		signal.Error = err.Error()
		if signal.FallbackAction == "" {
			signal.FallbackAction = policy.ActionBlock
		}
	}
	return signal
}

// DecisionFromModelSignal translates a vertical verdict signal into the policy decision contract.
func DecisionFromModelSignal(signal policy.ModelSignal) policy.Decision {
	action := signal.SuggestedAction
	if signal.FallbackUsed {
		action = signal.FallbackAction
	}
	if !KnownPolicyAction(action) {
		action = policy.ActionAllow
	}
	return policy.Decision{
		Action:             action,
		DefaultAction:      policy.ActionAllow,
		ModelSignal:        signal,
		ModelSignalApplied: action != policy.ActionAllow,
	}
}

// PolicyAction translates routing-layer actions into policy-layer actions.
func PolicyAction(action routing.Action) policy.Action {
	switch action {
	case routing.ActionAllow:
		return policy.ActionAllow
	case routing.ActionBlock:
		return policy.ActionBlock
	case routing.ActionRedact:
		return policy.ActionRedact
	case routing.ActionShadowLog:
		return policy.ActionShadowLog
	default:
		return ""
	}
}

// KnownPolicyAction reports whether action is a supported policy action.
func KnownPolicyAction(action policy.Action) bool {
	switch action {
	case policy.ActionAllow, policy.ActionBlock, policy.ActionRedact, policy.ActionShadowLog:
		return true
	default:
		return false
	}
}

func noopRelease() {}
