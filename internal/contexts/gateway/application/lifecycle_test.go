package application

import (
	"testing"

	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

func TestLifecycleUseCaseRouteProviderAndAuditGates(t *testing.T) {
	useCase := LifecycleUseCase{}

	tests := []struct {
		name string
		got  LifecycleDecision
		want RejectReason
	}{
		{name: "missing route", got: useCase.RouteAvailable(false), want: RejectUnsupportedPath},
		{name: "missing provider", got: useCase.ProviderAvailable(false), want: RejectMissingProvider},
		{name: "unhealthy audit", got: useCase.AuditHealthy(false), want: RejectAuditSinkUnhealthy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got.Allowed {
				t.Fatalf("Allowed = true, want false")
			}
			if tt.got.Reason != tt.want {
				t.Fatalf("Reason = %q, want %q", tt.got.Reason, tt.want)
			}
		})
	}

	if got := useCase.RouteAvailable(true); !got.Allowed || got.Reason != "" {
		t.Fatalf("RouteAvailable(true) = %+v, want allowed without reason", got)
	}
}

func TestLifecycleUseCaseResolveModelMapping(t *testing.T) {
	useCase := LifecycleUseCase{}

	missing := useCase.ResolveModelMapping(ModelMappingInput{RequestedModel: "gpt-4o"})
	if missing.Allowed {
		t.Fatalf("missing mapping Allowed = true, want false")
	}
	if missing.Reason != RejectMissingModelMapping {
		t.Fatalf("missing mapping Reason = %q, want %q", missing.Reason, RejectMissingModelMapping)
	}
	if missing.RequestedModel != "gpt-4o" {
		t.Fatalf("missing mapping RequestedModel = %q, want gpt-4o", missing.RequestedModel)
	}

	resolved := useCase.ResolveModelMapping(ModelMappingInput{
		RequestedModel: "gpt-4o",
		MappedModel:    "safe-model",
		UpstreamModel:  "upstream-model",
		Found:          true,
	})
	if !resolved.Allowed || resolved.Reason != "" {
		t.Fatalf("resolved mapping = %+v, want allowed without reason", resolved)
	}
	if resolved.MappedModel != "safe-model" || resolved.UpstreamModel != "upstream-model" {
		t.Fatalf("resolved mapping = %+v, want mapped and upstream models", resolved)
	}
}

func TestLifecycleUseCaseClassifyDecodedRequest(t *testing.T) {
	useCase := LifecycleUseCase{}

	invalid := useCase.ClassifyDecodedRequest(DecodedRequestInput{DecodeFailed: true})
	if invalid.Allowed {
		t.Fatalf("invalid decoded request Allowed = true, want false")
	}
	if invalid.Reason != RejectInvalidJSON {
		t.Fatalf("invalid decoded request Reason = %q, want %q", invalid.Reason, RejectInvalidJSON)
	}

	missingMapping := useCase.ClassifyDecodedRequest(DecodedRequestInput{
		ModelMapping: ModelMappingInput{RequestedModel: "gpt-4o"},
	})
	if missingMapping.Allowed {
		t.Fatalf("missing mapping Allowed = true, want false")
	}
	if missingMapping.Reason != RejectMissingModelMapping {
		t.Fatalf("missing mapping Reason = %q, want %q", missingMapping.Reason, RejectMissingModelMapping)
	}
	if missingMapping.RequestedModel != "gpt-4o" {
		t.Fatalf("missing mapping RequestedModel = %q, want gpt-4o", missingMapping.RequestedModel)
	}

	resolved := useCase.ClassifyDecodedRequest(DecodedRequestInput{
		ModelMapping: ModelMappingInput{
			RequestedModel: "gpt-4o",
			MappedModel:    "safe-model",
			UpstreamModel:  "upstream-model",
			Found:          true,
		},
	})
	if !resolved.Allowed || resolved.Reason != "" {
		t.Fatalf("resolved decoded request = %+v, want allowed", resolved)
	}
	if resolved.MappedModel != "safe-model" || resolved.UpstreamModel != "upstream-model" {
		t.Fatalf("resolved decoded request = %+v, want mapped and upstream models", resolved)
	}
}

func TestLifecycleUseCaseClassifyExchangeInfrastructure(t *testing.T) {
	useCase := LifecycleUseCase{}

	tests := []struct {
		name  string
		input ExchangeInfrastructureInput
		want  RejectReason
	}{
		{
			name:  "missing provider",
			input: ExchangeInfrastructureInput{ProviderFound: false, AuditHealthy: true},
			want:  RejectMissingProvider,
		},
		{
			name:  "unhealthy audit",
			input: ExchangeInfrastructureInput{ProviderFound: true, AuditHealthy: false},
			want:  RejectAuditSinkUnhealthy,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := useCase.ClassifyExchangeInfrastructure(tt.input)
			if got.Allowed {
				t.Fatalf("Allowed = true, want false")
			}
			if got.Reason != tt.want {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.want)
			}
		})
	}

	if got := useCase.ClassifyExchangeInfrastructure(ExchangeInfrastructureInput{ProviderFound: true, AuditHealthy: true}); !got.Allowed || got.Reason != "" {
		t.Fatalf("ClassifyExchangeInfrastructure(ready) = %+v, want allowed", got)
	}
}

func TestLifecycleUseCaseClassifyEnforcement(t *testing.T) {
	useCase := LifecycleUseCase{}

	blockedInput := useCase.ClassifyEnforcement(detection.DirectionInput, policy.Decision{Action: policy.ActionBlock})
	if !blockedInput.Blocked || blockedInput.Reason != RejectBlockedInput {
		t.Fatalf("blocked input = %+v, want blocked input reason", blockedInput)
	}

	blockedOutput := useCase.ClassifyEnforcement(detection.DirectionOutput, policy.Decision{Action: policy.ActionBlock})
	if !blockedOutput.Blocked || blockedOutput.Reason != RejectBlockedOutput {
		t.Fatalf("blocked output = %+v, want blocked output reason", blockedOutput)
	}

	redact := useCase.ClassifyEnforcement(detection.DirectionInput, policy.Decision{Action: policy.ActionRedact})
	if !redact.RedactionNeeded || redact.Blocked || redact.AllowObservation {
		t.Fatalf("redact = %+v, want redaction only", redact)
	}

	allow := useCase.ClassifyEnforcement(detection.DirectionOutput, policy.Decision{Action: policy.ActionAllow})
	if !allow.AllowObservation || allow.Blocked || allow.RedactionNeeded {
		t.Fatalf("allow = %+v, want observation only", allow)
	}

	if got := useCase.ClassifyInputEnforcement(policy.Decision{Action: policy.ActionBlock}); !got.Blocked || got.Reason != RejectBlockedInput {
		t.Fatalf("ClassifyInputEnforcement(block) = %+v, want blocked input", got)
	}
	if got := useCase.ClassifyOutputEnforcement(policy.Decision{Action: policy.ActionBlock}); !got.Blocked || got.Reason != RejectBlockedOutput {
		t.Fatalf("ClassifyOutputEnforcement(block) = %+v, want blocked output", got)
	}

	unknownDirection := useCase.ClassifyEnforcement(detection.Direction("sideways"), policy.Decision{Action: policy.ActionBlock})
	if !unknownDirection.Blocked || unknownDirection.Reason != RejectReason("blocked") {
		t.Fatalf("unknown direction block = %+v, want generic blocked reason", unknownDirection)
	}
}

func TestLifecycleUseCaseClassifyPolicyEnforcement(t *testing.T) {
	useCase := LifecycleUseCase{}

	blocked := useCase.ClassifyInputPolicyEnforcement(PolicyEnforcementInput{
		Decision: policy.Decision{Action: policy.ActionBlock},
	})
	if !blocked.Blocked || blocked.Reason != RejectBlockedInput {
		t.Fatalf("blocked = %+v, want blocked input", blocked)
	}

	pendingRedaction := useCase.ClassifyOutputPolicyEnforcement(PolicyEnforcementInput{
		Decision: policy.Decision{Action: policy.ActionRedact},
	})
	if !pendingRedaction.RedactionNeeded || pendingRedaction.RedactionRejected || pendingRedaction.Blocked {
		t.Fatalf("pendingRedaction = %+v, want redaction needed", pendingRedaction)
	}

	rejectedRedaction := useCase.ClassifyOutputPolicyEnforcement(PolicyEnforcementInput{
		Decision:           policy.Decision{Action: policy.ActionRedact},
		RedactionAttempted: true,
		RedactionText:      "secret",
		RedactionCount:     0,
	})
	if !rejectedRedaction.RedactionRejected || rejectedRedaction.Reason != RejectRedactionFailed {
		t.Fatalf("rejectedRedaction = %+v, want redaction failure", rejectedRedaction)
	}

	allowedRedaction := useCase.ClassifyOutputPolicyEnforcement(PolicyEnforcementInput{
		Decision:           policy.Decision{Action: policy.ActionRedact},
		RedactionAttempted: true,
		RedactionText:      "secret",
		RedactionCount:     1,
	})
	if allowedRedaction.Blocked || allowedRedaction.RedactionRejected || !allowedRedaction.AllowObservation {
		t.Fatalf("allowedRedaction = %+v, want observable redaction", allowedRedaction)
	}
}

func TestRedactionReasonPrefersExplicitReasonThenFallback(t *testing.T) {
	tests := []struct {
		name     string
		reason   RejectReason
		fallback string
		want     RejectReason
	}{
		{name: "explicit reason", reason: RejectRedactionFailed, fallback: "codec_failed", want: RejectRedactionFailed},
		{name: "fallback tag", fallback: "codec_failed", want: RejectReason("codec_failed")},
		{name: "default redaction failure", want: RejectRedactionFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactionReason(tt.reason, tt.fallback); got != tt.want {
				t.Fatalf("redactionReason(%q, %q) = %q, want %q", tt.reason, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestLifecycleUseCaseClassifyInputRequest(t *testing.T) {
	useCase := LifecycleUseCase{}

	pendingPolicy := useCase.ClassifyInputRequest(InputRequestLifecycleInput{})
	if !pendingPolicy.Allowed || pendingPolicy.PolicyDecisionObserved {
		t.Fatalf("pendingPolicy = %+v, want allowed request before policy evaluation", pendingPolicy)
	}

	blocked := useCase.ClassifyInputRequest(InputRequestLifecycleInput{
		PolicyEvaluated: true,
		Decision:        policy.Decision{Action: policy.ActionBlock},
	})
	if blocked.Allowed || !blocked.PolicyDecisionObserved {
		t.Fatalf("blocked = %+v, want policy-observed rejection", blocked)
	}
	if blocked.Reason != RejectBlockedInput {
		t.Fatalf("blocked.Reason = %q, want %q", blocked.Reason, RejectBlockedInput)
	}

	redactionNeeded := useCase.ClassifyInputRequest(InputRequestLifecycleInput{
		PolicyEvaluated: true,
		Decision:        policy.Decision{Action: policy.ActionRedact},
	})
	if !redactionNeeded.Allowed || !redactionNeeded.RedactionNeeded {
		t.Fatalf("redactionNeeded = %+v, want allowed redaction step", redactionNeeded)
	}

	redactionRejected := useCase.ClassifyInputRequest(InputRequestLifecycleInput{
		PolicyEvaluated:    true,
		Decision:           policy.Decision{Action: policy.ActionRedact},
		RedactionAttempted: true,
		RedactionText:      "secret",
		RedactionCount:     0,
	})
	if redactionRejected.Allowed || !redactionRejected.RedactionRejected {
		t.Fatalf("redactionRejected = %+v, want rejected redaction", redactionRejected)
	}
	if redactionRejected.Reason != RejectRedactionFailed {
		t.Fatalf("redactionRejected.Reason = %q, want %q", redactionRejected.Reason, RejectRedactionFailed)
	}

	allowed := useCase.ClassifyInputRequest(InputRequestLifecycleInput{
		PolicyEvaluated: true,
		Decision:        policy.Decision{Action: policy.ActionAllow},
	})
	if !allowed.Allowed || !allowed.PolicyDecisionObserved || allowed.RedactionNeeded {
		t.Fatalf("allowed = %+v, want observed allowed input", allowed)
	}
}

func TestLifecycleUseCaseClassifyRequestBody(t *testing.T) {
	useCase := LifecycleUseCase{}

	tests := []struct {
		name  string
		input RequestBodyInput
		want  RejectReason
	}{
		{name: "read failed", input: RequestBodyInput{ReadFailed: true}, want: RejectInvalidRequest},
		{name: "too large", input: RequestBodyInput{TooLarge: true}, want: RejectRequestTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := useCase.ClassifyRequestBody(tt.input)
			if got.Allowed {
				t.Fatalf("Allowed = true, want false")
			}
			if got.Reason != tt.want {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.want)
			}
		})
	}

	if got := useCase.ClassifyRequestBody(RequestBodyInput{}); !got.Allowed || got.Reason != "" {
		t.Fatalf("ClassifyRequestBody(success) = %+v, want allowed", got)
	}
}

func TestLifecycleUseCaseClassifyUpstreamResponse(t *testing.T) {
	useCase := LifecycleUseCase{}

	tests := []struct {
		name  string
		input UpstreamResponseInput
		want  RejectReason
	}{
		{name: "read failed", input: UpstreamResponseInput{ReadFailed: true}, want: RejectUpstreamError},
		{name: "too large", input: UpstreamResponseInput{TooLarge: true}, want: RejectUpstreamResponseTooLarge},
		{name: "upstream error status", input: UpstreamResponseInput{StatusCode: 429}, want: RejectUpstreamError},
		{name: "decode failed", input: UpstreamResponseInput{DecodeFailed: true}, want: RejectUpstreamError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := useCase.ClassifyUpstreamResponse(tt.input)
			if got.Allowed {
				t.Fatalf("Allowed = true, want false")
			}
			if got.Reason != tt.want {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.want)
			}
		})
	}

	if got := useCase.ClassifyUpstreamResponse(UpstreamResponseInput{StatusCode: 200}); !got.Allowed || got.Reason != "" {
		t.Fatalf("ClassifyUpstreamResponse(200) = %+v, want allowed", got)
	}
}

func TestLifecycleUseCaseClassifyOutputResponse(t *testing.T) {
	useCase := LifecycleUseCase{}

	readFailure := useCase.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{ReadFailed: true},
	})
	if readFailure.Allowed || readFailure.PolicyDecisionObserved {
		t.Fatalf("readFailure = %+v, want transport rejection without policy observation", readFailure)
	}
	if readFailure.Reason != RejectUpstreamError {
		t.Fatalf("readFailure.Reason = %q, want %q", readFailure.Reason, RejectUpstreamError)
	}

	pendingPolicy := useCase.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{StatusCode: 200},
	})
	if !pendingPolicy.Allowed || pendingPolicy.PolicyDecisionObserved {
		t.Fatalf("pendingPolicy = %+v, want allowed response before policy evaluation", pendingPolicy)
	}

	blocked := useCase.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{StatusCode: 200},
		PolicyEvaluated:       true,
		Decision:              policy.Decision{Action: policy.ActionBlock},
	})
	if blocked.Allowed || !blocked.PolicyDecisionObserved {
		t.Fatalf("blocked = %+v, want policy-observed rejection", blocked)
	}
	if blocked.Reason != RejectBlockedOutput {
		t.Fatalf("blocked.Reason = %q, want %q", blocked.Reason, RejectBlockedOutput)
	}

	redactionNeeded := useCase.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{StatusCode: 200},
		PolicyEvaluated:       true,
		Decision:              policy.Decision{Action: policy.ActionRedact},
	})
	if !redactionNeeded.Allowed || !redactionNeeded.RedactionNeeded {
		t.Fatalf("redactionNeeded = %+v, want allowed redaction step", redactionNeeded)
	}

	redactionRejected := useCase.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{StatusCode: 200},
		PolicyEvaluated:       true,
		Decision:              policy.Decision{Action: policy.ActionRedact},
		RedactionAttempted:    true,
		RedactionText:         "secret",
		RedactionCount:        0,
	})
	if redactionRejected.Allowed || !redactionRejected.RedactionRejected {
		t.Fatalf("redactionRejected = %+v, want rejected redaction", redactionRejected)
	}
	if redactionRejected.Reason != RejectRedactionFailed {
		t.Fatalf("redactionRejected.Reason = %q, want %q", redactionRejected.Reason, RejectRedactionFailed)
	}

	allowed := useCase.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{StatusCode: 200},
		PolicyEvaluated:       true,
		Decision:              policy.Decision{Action: policy.ActionAllow},
	})
	if !allowed.Allowed || !allowed.PolicyDecisionObserved || allowed.RedactionNeeded {
		t.Fatalf("allowed = %+v, want observed allowed response", allowed)
	}
}

func TestLifecycleUseCaseClassifyUpstreamRequest(t *testing.T) {
	useCase := LifecycleUseCase{}

	rejected := useCase.ClassifyUpstreamRequest(UpstreamRequestInput{ProjectionFailed: true})
	if rejected.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if rejected.Reason != RejectInvalidRequest {
		t.Fatalf("Reason = %q, want %q", rejected.Reason, RejectInvalidRequest)
	}

	if allowed := useCase.ClassifyUpstreamRequest(UpstreamRequestInput{}); !allowed.Allowed || allowed.Reason != "" {
		t.Fatalf("ClassifyUpstreamRequest(success) = %+v, want allowed", allowed)
	}
}

func TestLifecycleUseCaseClassifyUpstreamExchange(t *testing.T) {
	useCase := LifecycleUseCase{}

	tests := []struct {
		name  string
		input UpstreamExchangeInput
		want  RejectReason
	}{
		{name: "forward failed", input: UpstreamExchangeInput{ForwardFailed: true}, want: RejectUpstreamError},
		{name: "stream error status", input: UpstreamExchangeInput{StatusCode: 429, Stream: true}, want: RejectUpstreamError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := useCase.ClassifyUpstreamExchange(tt.input)
			if got.Allowed {
				t.Fatalf("Allowed = true, want false")
			}
			if got.Reason != tt.want {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.want)
			}
		})
	}

	stream := useCase.ClassifyUpstreamExchange(UpstreamExchangeInput{StatusCode: 200, Stream: true})
	if !stream.Allowed || !stream.Stream {
		t.Fatalf("stream exchange = %+v, want allowed stream", stream)
	}

	nonStream := useCase.ClassifyUpstreamExchange(UpstreamExchangeInput{StatusCode: 200})
	if !nonStream.Allowed || nonStream.Stream {
		t.Fatalf("nonStream exchange = %+v, want allowed non-stream", nonStream)
	}
}

func TestLifecycleUseCaseClassifyRedaction(t *testing.T) {
	useCase := LifecycleUseCase{}

	tests := []struct {
		name  string
		input RedactionInput
		want  RejectReason
	}{
		{name: "redaction failed", input: RedactionInput{Failed: true}, want: RejectRedactionFailed},
		{name: "text had no redactions", input: RedactionInput{Text: "secret", Redactions: 0}, want: RejectRedactionFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := useCase.ClassifyRedaction(tt.input)
			if got.Allowed {
				t.Fatalf("Allowed = true, want false")
			}
			if got.Reason != tt.want {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.want)
			}
		})
	}

	if got := useCase.ClassifyRedaction(RedactionInput{Text: "secret", Redactions: 1}); !got.Allowed {
		t.Fatalf("ClassifyRedaction(success) = %+v, want allowed", got)
	}
	if got := useCase.ClassifyRedaction(RedactionInput{}); !got.Allowed {
		t.Fatalf("ClassifyRedaction(empty) = %+v, want allowed", got)
	}
}

func TestLifecycleUseCaseClassifyStreamingPolicy(t *testing.T) {
	useCase := LifecycleUseCase{}

	tests := []struct {
		name     string
		decision policy.Decision
		wantStop bool
		want     string
	}{
		{
			name:     "block terminates stream",
			decision: policy.Decision{Action: policy.ActionBlock},
			wantStop: true,
			want:     "stream_blocked",
		},
		{
			name:     "redact terminates stream because streaming redaction is unsupported",
			decision: policy.Decision{Action: policy.ActionRedact},
			wantStop: true,
			want:     "unsupported_redaction",
		},
		{
			name:     "allow continues stream",
			decision: policy.Decision{Action: policy.ActionAllow},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := useCase.ClassifyStreamingPolicy(tt.decision)
			if got.Terminated != tt.wantStop {
				t.Fatalf("Terminated = %v, want %v", got.Terminated, tt.wantStop)
			}
			if got.TerminationReason != tt.want {
				t.Fatalf("TerminationReason = %q, want %q", got.TerminationReason, tt.want)
			}
		})
	}
}

func TestLifecycleUseCaseClassifyStreamingCollection(t *testing.T) {
	useCase := LifecycleUseCase{}

	if got := useCase.ClassifyStreamingCollection(false); !got.Terminated || got.TerminationReason != "stream_output_too_large" {
		t.Fatalf("ClassifyStreamingCollection(false) = %+v, want stream output too large termination", got)
	}
	if got := useCase.ClassifyStreamingCollection(true); got.Terminated || got.TerminationReason != "" {
		t.Fatalf("ClassifyStreamingCollection(true) = %+v, want continue", got)
	}
}

func TestLifecycleUseCaseClassifyStreamingDelta(t *testing.T) {
	useCase := LifecycleUseCase{}

	if got := useCase.ClassifyStreamingDelta(StreamingDeltaInput{}); got.Terminated {
		t.Fatalf("ClassifyStreamingDelta(no delta) = %+v, want continue", got)
	}
	if got := useCase.ClassifyStreamingDelta(StreamingDeltaInput{
		DeltaObserved:      true,
		PolicyDecision:     policy.Decision{Action: policy.ActionBlock},
		CollectionAccepted: true,
	}); !got.Terminated || got.TerminationReason != "stream_blocked" {
		t.Fatalf("ClassifyStreamingDelta(block) = %+v, want stream_blocked", got)
	}
	if got := useCase.ClassifyStreamingDelta(StreamingDeltaInput{
		DeltaObserved:      true,
		PolicyDecision:     policy.Decision{Action: policy.ActionAllow},
		CollectionAccepted: false,
	}); !got.Terminated || got.TerminationReason != "stream_output_too_large" {
		t.Fatalf("ClassifyStreamingDelta(collection) = %+v, want stream_output_too_large", got)
	}
}
