package application

import "testing"

func TestUpstreamExchangeUseCasePlansForwarding(t *testing.T) {
	useCase := UpstreamExchangeUseCase{}

	failed := useCase.PlanForward(UpstreamForwardPlanInput{ForwardFailed: true})
	assertUpstreamExchangePlan(t, failed, UpstreamExchangeActionBlock, RejectUpstreamError)

	allowed := useCase.PlanForward(UpstreamForwardPlanInput{})
	assertUpstreamExchangePlan(t, allowed, UpstreamExchangeActionHandleResponse, "")
}

func TestUpstreamExchangeUseCasePlansResponse(t *testing.T) {
	useCase := UpstreamExchangeUseCase{}

	failedStreamStatus := useCase.PlanResponse(UpstreamResponsePlanInput{StatusCode: 500, Stream: true})
	assertUpstreamExchangePlan(t, failedStreamStatus, UpstreamExchangeActionBlock, RejectUpstreamError)

	stream := useCase.PlanResponse(UpstreamResponsePlanInput{StatusCode: 200, Stream: true})
	assertUpstreamExchangePlan(t, stream, UpstreamExchangeActionHandleStream, "")

	response := useCase.PlanResponse(UpstreamResponsePlanInput{StatusCode: 500})
	assertUpstreamExchangePlan(t, response, UpstreamExchangeActionHandleResponse, "")
}

func assertUpstreamExchangePlan(t *testing.T, got UpstreamExchangePlan, action UpstreamExchangeAction, reason RejectReason) {
	t.Helper()
	if got.Action != action {
		t.Fatalf("Action = %q, want %q", got.Action, action)
	}
	if got.Reason != reason {
		t.Fatalf("Reason = %q, want %q", got.Reason, reason)
	}
}
