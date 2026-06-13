package application

import (
	"errors"
	"testing"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/routing/domain"
)

func TestAdmissionUseCaseAllowsProtectedRoute(t *testing.T) {
	released := false
	useCase := AdmissionUseCase{
		Authenticator: fakeAuthenticator{clientID: "client-a"},
		Limits: fakeLimitResolver{policy: LimitPolicy{
			RequestsPerWindow:     10,
			Window:                time.Minute,
			MaxConcurrentRequests: 1,
			MaxBodyBytes:          1024,
		}},
		RateLimiter:        fakeRateLimiter{allow: true},
		ConcurrencyLimiter: fakeConcurrencyLimiter{allow: true, release: func() { released = true }},
		Audit:              fakeAudit{healthy: true},
	}

	result := useCase.AdmitProtectedRoute(AdmissionInput{
		ClientKey:           "raw-key",
		RouteKey:            "chat",
		DefaultMaxBodyBytes: 512,
		Now:                 time.Unix(1, 0),
	})

	if !result.Allowed {
		t.Fatalf("Allowed = false, reason %q", result.Reason)
	}
	if result.ClientID != "client-a" {
		t.Fatalf("ClientID = %q, want client-a", result.ClientID)
	}
	if result.MaxBodyBytes != 1024 {
		t.Fatalf("MaxBodyBytes = %d, want 1024", result.MaxBodyBytes)
	}
	result.Release()
	if !released {
		t.Fatal("Release did not call concurrency release")
	}
}

func TestAdmissionUseCaseUsesDefaultBodyLimitAndNoopRelease(t *testing.T) {
	useCase := AdmissionUseCase{
		Authenticator: fakeAuthenticator{clientID: "client-a"},
		Limits: fakeLimitResolver{policy: LimitPolicy{
			RequestsPerWindow:     10,
			Window:                time.Minute,
			MaxConcurrentRequests: 1,
			MaxBodyBytes:          -1,
		}},
		RateLimiter:        fakeRateLimiter{allow: true},
		ConcurrencyLimiter: fakeConcurrencyLimiter{allow: true},
		Audit:              fakeAudit{healthy: true},
	}

	result := useCase.AdmitProtectedRoute(AdmissionInput{
		ClientKey:           "raw-key",
		RouteKey:            "chat",
		DefaultMaxBodyBytes: 2048,
		Now:                 time.Unix(1, 0),
	})

	if !result.Allowed {
		t.Fatalf("Allowed = false, reason %q", result.Reason)
	}
	if result.MaxBodyBytes != 2048 {
		t.Fatalf("MaxBodyBytes = %d, want default limit", result.MaxBodyBytes)
	}
	result.Release()
}

func TestAdmissionUseCaseRejectsInOrder(t *testing.T) {
	tests := []struct {
		name    string
		useCase AdmissionUseCase
		want    RejectReason
	}{
		{
			name: "auth",
			useCase: AdmissionUseCase{
				Authenticator: fakeAuthenticator{reason: RejectInvalidClientKey},
			},
			want: RejectInvalidClientKey,
		},
		{
			name: "missing limit",
			useCase: AdmissionUseCase{
				Authenticator: fakeAuthenticator{clientID: "client-a"},
				Limits:        fakeLimitResolver{},
			},
			want: RejectMissingLimitPolicy,
		},
		{
			name: "audit",
			useCase: AdmissionUseCase{
				Authenticator: fakeAuthenticator{clientID: "client-a"},
				Limits:        fakeLimitResolver{policy: validLimitPolicy()},
				Audit:         fakeAudit{healthy: false},
			},
			want: RejectAuditSinkUnhealthy,
		},
		{
			name: "rate",
			useCase: AdmissionUseCase{
				Authenticator: fakeAuthenticator{clientID: "client-a"},
				Limits:        fakeLimitResolver{policy: validLimitPolicy()},
				RateLimiter:   fakeRateLimiter{allow: false},
				Audit:         fakeAudit{healthy: true},
			},
			want: RejectRateLimitExceeded,
		},
		{
			name: "concurrency",
			useCase: AdmissionUseCase{
				Authenticator:      fakeAuthenticator{clientID: "client-a"},
				Limits:             fakeLimitResolver{policy: validLimitPolicy()},
				RateLimiter:        fakeRateLimiter{allow: true},
				ConcurrencyLimiter: fakeConcurrencyLimiter{allow: false},
				Audit:              fakeAudit{healthy: true},
			},
			want: RejectConcurrencyLimitExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.useCase.AdmitProtectedRoute(AdmissionInput{ClientKey: "key", RouteKey: "chat"})
			if result.Allowed {
				t.Fatal("Allowed = true, want false")
			}
			if result.Reason != tt.want {
				t.Fatalf("Reason = %q, want %q", result.Reason, tt.want)
			}
			result.Release()
		})
	}
}

func TestPolicyEvaluatorUsesSnapshot(t *testing.T) {
	snapshot := fakePolicySnapshot{decision: policy.Decision{Action: policy.ActionRedact}}

	decision := PolicyEvaluator{Snapshot: snapshot}.Evaluate(PolicyEvaluationInput{
		RouteKey:    "chat",
		ProviderKey: "openai",
		Model:       "gpt-test",
		Direction:   detection.DirectionInput,
		Text:        "hello",
	})

	if decision.Action != policy.ActionRedact {
		t.Fatalf("Action = %q, want redact", decision.Action)
	}
}

func TestPolicyEvaluatorFallsBackToModelSignal(t *testing.T) {
	decision := PolicyEvaluator{}.Evaluate(PolicyEvaluationInput{
		ModelSignal: policy.ModelSignal{
			SuggestedAction: policy.ActionAllow,
			FallbackAction:  policy.ActionBlock,
			FallbackUsed:    true,
		},
	})

	if decision.Action != policy.ActionBlock {
		t.Fatalf("Action = %q, want block", decision.Action)
	}
	if !decision.ModelSignalApplied {
		t.Fatal("ModelSignalApplied = false, want true")
	}
}

func TestModelSignalFromVerdictUsesFallbackOnError(t *testing.T) {
	signal := ModelSignalFromVerdict(verdict.Result{}, errors.New("model timeout"), routing.ActionRedact)

	if !signal.FallbackUsed {
		t.Fatal("FallbackUsed = false, want true")
	}
	if signal.FallbackAction != policy.ActionRedact {
		t.Fatalf("FallbackAction = %q, want redact", signal.FallbackAction)
	}
	if signal.Error != "model timeout" {
		t.Fatalf("Error = %q, want model timeout", signal.Error)
	}
}

func TestPolicyActionAndKnownPolicyActionHandleUnknownValues(t *testing.T) {
	if got := PolicyAction(routing.Action("quarantine")); got != "" {
		t.Fatalf("PolicyAction(unknown) = %q, want empty", got)
	}
	if KnownPolicyAction(policy.Action("quarantine")) {
		t.Fatal("KnownPolicyAction(unknown) = true, want false")
	}
}

func validLimitPolicy() LimitPolicy {
	return LimitPolicy{
		RequestsPerWindow:     10,
		Window:                time.Minute,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          1024,
	}
}

type fakeAuthenticator struct {
	clientID string
	reason   RejectReason
}

func (f fakeAuthenticator) AuthenticateClient(string) (ClientIdentity, RejectReason) {
	if f.reason != "" {
		return ClientIdentity{}, f.reason
	}
	return ClientIdentity{ID: f.clientID}, ""
}

type fakeLimitResolver struct {
	policy LimitPolicy
}

func (f fakeLimitResolver) EffectiveLimitPolicy(string, string) (LimitPolicy, bool) {
	if f.policy.MaxBodyBytes == 0 {
		return LimitPolicy{}, false
	}
	return f.policy, true
}

type fakeRateLimiter struct {
	allow bool
}

func (f fakeRateLimiter) AllowRate(time.Time, string, string, LimitPolicy) bool {
	return f.allow
}

type fakeConcurrencyLimiter struct {
	allow   bool
	release func()
}

func (f fakeConcurrencyLimiter) AcquireConcurrency(string, string, LimitPolicy) (func(), bool) {
	return f.release, f.allow
}

type fakeAudit struct {
	healthy bool
}

func (f fakeAudit) Healthy() bool {
	return f.healthy
}

type fakePolicySnapshot struct {
	decision policy.Decision
}

func (f fakePolicySnapshot) Evaluate(policy.EvaluationRequest) policy.Decision {
	return f.decision
}
