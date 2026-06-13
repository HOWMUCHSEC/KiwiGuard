package application

import (
	"testing"

	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

func TestOutputResponseUseCasePlansBodyRead(t *testing.T) {
	useCase := OutputResponseUseCase{}

	tooLarge := useCase.PlanBodyRead(OutputBodyReadInput{StatusCode: 200, TooLarge: true})
	assertOutputPlan(t, tooLarge, OutputResponseActionBlock, RejectUpstreamResponseTooLarge, false, "upstream_response_too_large")

	readFailed := useCase.PlanBodyRead(OutputBodyReadInput{StatusCode: 200, ReadFailed: true})
	assertOutputPlan(t, readFailed, OutputResponseActionBlock, RejectUpstreamError, false, "upstream_error")

	upstreamErrorStatus := useCase.PlanBodyRead(OutputBodyReadInput{StatusCode: 429})
	assertOutputPlan(t, upstreamErrorStatus, OutputResponseActionBlock, RejectUpstreamError, false, "upstream_error")

	allowed := useCase.PlanBodyRead(OutputBodyReadInput{StatusCode: 200})
	assertOutputPlan(t, allowed, OutputResponseActionExtractText, "", false, "")
}

func TestOutputResponseUseCasePlansExtraction(t *testing.T) {
	useCase := OutputResponseUseCase{}

	failed := useCase.PlanExtraction(OutputExtractionInput{StatusCode: 200, DecodeFailed: true})
	assertOutputPlan(t, failed, OutputResponseActionBlock, RejectUpstreamError, false, "upstream_error")

	allowed := useCase.PlanExtraction(OutputExtractionInput{StatusCode: 200})
	assertOutputPlan(t, allowed, OutputResponseActionEvaluatePolicy, "", false, "")
}

func TestOutputResponseUseCasePlansPolicy(t *testing.T) {
	useCase := OutputResponseUseCase{}

	blocked := useCase.PlanPolicy(OutputPolicyInput{
		StatusCode: 200,
		Decision:   policy.Decision{Action: policy.ActionBlock},
	})
	assertOutputPlan(t, blocked, OutputResponseActionBlock, RejectBlockedOutput, true, "")

	redact := useCase.PlanPolicy(OutputPolicyInput{
		StatusCode: 200,
		Decision:   policy.Decision{Action: policy.ActionRedact},
	})
	assertOutputPlan(t, redact, OutputResponseActionRedact, "", true, "")

	allowed := useCase.PlanPolicy(OutputPolicyInput{
		StatusCode: 200,
		Decision:   policy.Decision{Action: policy.ActionAllow},
	})
	assertOutputPlan(t, allowed, OutputResponseActionSend, "", true, "")
}

func TestOutputResponseUseCasePlansRedaction(t *testing.T) {
	useCase := OutputResponseUseCase{}

	failed := useCase.PlanRedaction(OutputRedactionInput{
		StatusCode: 200,
		Decision:   policy.Decision{Action: policy.ActionRedact},
		Text:       "secret",
		Failed:     true,
	})
	assertOutputPlan(t, failed, OutputResponseActionBlock, RejectRedactionFailed, true, "redaction_failed")

	empty := useCase.PlanRedaction(OutputRedactionInput{
		StatusCode: 200,
		Decision:   policy.Decision{Action: policy.ActionRedact},
		Text:       "secret",
	})
	assertOutputPlan(t, empty, OutputResponseActionBlock, RejectRedactionFailed, true, "redaction_failed")

	allowed := useCase.PlanRedaction(OutputRedactionInput{
		StatusCode: 200,
		Decision:   policy.Decision{Action: policy.ActionRedact},
		Text:       "secret",
		Redactions: 1,
	})
	assertOutputPlan(t, allowed, OutputResponseActionSend, "", true, "")
}

func assertOutputPlan(t *testing.T, got OutputResponsePlan, action OutputResponseAction, reason RejectReason, emitPolicyDecision bool, termination string) {
	t.Helper()
	if got.Action != action {
		t.Fatalf("Action = %q, want %q", got.Action, action)
	}
	if got.Reason != reason {
		t.Fatalf("Reason = %q, want %q", got.Reason, reason)
	}
	if got.EmitPolicyDecision != emitPolicyDecision {
		t.Fatalf("EmitPolicyDecision = %v, want %v", got.EmitPolicyDecision, emitPolicyDecision)
	}
	if got.TerminationReason != termination {
		t.Fatalf("TerminationReason = %q, want %q", got.TerminationReason, termination)
	}
}
