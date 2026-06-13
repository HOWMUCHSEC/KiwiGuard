package application

import (
	"testing"

	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

func TestStreamingOutputUseCasePlansDeltaPolicyEvaluation(t *testing.T) {
	useCase := StreamingOutputUseCase{Buffer: NewStreamBuffer(4, 20)}

	empty := useCase.PlanDelta(StreamingDeltaPlanInput{})
	assertStreamingPlan(t, empty, StreamingOutputActionContinue, "", false, false, "")

	first := useCase.PlanDelta(StreamingDeltaPlanInput{Text: "hello"})
	assertStreamingPlan(t, first, StreamingOutputActionEvaluatePolicy, "ello", true, false, "")

	overflow := useCase.PlanDelta(StreamingDeltaPlanInput{Text: "this output is too large"})
	assertStreamingPlan(t, overflow, StreamingOutputActionEvaluatePolicy, "arge", false, false, "")
}

func TestStreamingOutputUseCasePlansPolicy(t *testing.T) {
	useCase := StreamingOutputUseCase{}

	allowed := useCase.PlanPolicy(StreamingPolicyPlanInput{
		Decision:           policy.Decision{Action: policy.ActionAllow},
		CollectionAccepted: true,
	})
	assertStreamingPlan(t, allowed, StreamingOutputActionContinue, "", false, false, "")

	blocked := useCase.PlanPolicy(StreamingPolicyPlanInput{
		Decision:           policy.Decision{Action: policy.ActionBlock},
		CollectionAccepted: true,
	})
	assertStreamingPlan(t, blocked, StreamingOutputActionTerminate, "", false, false, "stream_blocked")

	redact := useCase.PlanPolicy(StreamingPolicyPlanInput{
		Decision:           policy.Decision{Action: policy.ActionRedact},
		CollectionAccepted: true,
	})
	assertStreamingPlan(t, redact, StreamingOutputActionTerminate, "", false, false, "unsupported_redaction")

	tooLarge := useCase.PlanPolicy(StreamingPolicyPlanInput{
		Decision: policy.Decision{Action: policy.ActionAllow},
	})
	assertStreamingPlan(t, tooLarge, StreamingOutputActionTerminate, "", false, true, "stream_output_too_large")
}

func TestStreamingOutputUseCasePlansFrameProgression(t *testing.T) {
	useCase := StreamingOutputUseCase{}

	hold := useCase.PlanFrame(StreamingFramePlanInput{})
	assertStreamingPlan(t, hold, StreamingOutputActionContinue, "", false, false, "")

	write := useCase.PlanFrame(StreamingFramePlanInput{HasPending: true})
	assertStreamingPlan(t, write, StreamingOutputActionWritePending, "", false, false, "")

	complete := useCase.PlanFrame(StreamingFramePlanInput{HasPending: true, Done: true})
	assertStreamingPlan(t, complete, StreamingOutputActionComplete, "", false, false, "")
}

func TestStreamingOutputUseCasePlansFinalEvaluation(t *testing.T) {
	useCase := StreamingOutputUseCase{}

	evaluate := useCase.PlanFinal(StreamingFinalPlanInput{
		PolicyAllowed: true,
		CollectedText: "complete output",
	})
	assertStreamingPlan(t, evaluate, StreamingOutputActionEvaluatePolicy, "complete output", false, false, "")

	blocked := useCase.PlanFinal(StreamingFinalPlanInput{
		CollectedText: "complete output",
	})
	assertStreamingPlan(t, blocked, StreamingOutputActionEmit, "", false, false, "")

	empty := useCase.PlanFinal(StreamingFinalPlanInput{PolicyAllowed: true})
	assertStreamingPlan(t, empty, StreamingOutputActionEmit, "", false, false, "")
}

func assertStreamingPlan(t *testing.T, got StreamingOutputPlan, action StreamingOutputAction, text string, collectionAccepted bool, forceBlockedOutput bool, termination string) {
	t.Helper()
	if got.Action != action {
		t.Fatalf("Action = %q, want %q", got.Action, action)
	}
	if got.PolicyText != text {
		t.Fatalf("PolicyText = %q, want %q", got.PolicyText, text)
	}
	if got.CollectionAccepted != collectionAccepted {
		t.Fatalf("CollectionAccepted = %v, want %v", got.CollectionAccepted, collectionAccepted)
	}
	if got.ForceBlockedOutput != forceBlockedOutput {
		t.Fatalf("ForceBlockedOutput = %v, want %v", got.ForceBlockedOutput, forceBlockedOutput)
	}
	if got.TerminationReason != termination {
		t.Fatalf("TerminationReason = %q, want %q", got.TerminationReason, termination)
	}
}
