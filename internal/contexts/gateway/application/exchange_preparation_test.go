package application

import "testing"

func TestExchangePreparationUseCasePlansRouteAvailability(t *testing.T) {
	useCase := ExchangePreparationUseCase{}

	missing := useCase.PlanRouteAvailability(RouteAvailabilityInput{})
	assertExchangePreparationPlan(t, missing, ExchangePreparationActionBlock, RejectUnsupportedPath, "", "", "")

	found := useCase.PlanRouteAvailability(RouteAvailabilityInput{Found: true})
	assertExchangePreparationPlan(t, found, ExchangePreparationActionContinue, "", "", "", "")
}

func TestExchangePreparationUseCasePlansInfrastructure(t *testing.T) {
	useCase := ExchangePreparationUseCase{}

	missingProvider := useCase.PlanInfrastructure(ExchangeInfrastructurePlanInput{AuditHealthy: true})
	assertExchangePreparationPlan(t, missingProvider, ExchangePreparationActionBlock, RejectMissingProvider, "", "", "")

	unhealthyAudit := useCase.PlanInfrastructure(ExchangeInfrastructurePlanInput{ProviderFound: true})
	assertExchangePreparationPlan(t, unhealthyAudit, ExchangePreparationActionBlock, RejectAuditSinkUnhealthy, "", "", "")

	ready := useCase.PlanInfrastructure(ExchangeInfrastructurePlanInput{ProviderFound: true, AuditHealthy: true})
	assertExchangePreparationPlan(t, ready, ExchangePreparationActionContinue, "", "", "", "")
}

func TestExchangePreparationUseCasePlansDecodedRequest(t *testing.T) {
	useCase := ExchangePreparationUseCase{}

	invalidJSON := useCase.PlanDecodedRequest(DecodedRequestPlanInput{DecodeFailed: true})
	assertExchangePreparationPlan(t, invalidJSON, ExchangePreparationActionBlock, RejectInvalidJSON, "", "", "")

	missingMapping := useCase.PlanDecodedRequest(DecodedRequestPlanInput{
		ModelMapping: ModelMappingInput{RequestedModel: "gpt-4.1"},
	})
	assertExchangePreparationPlan(t, missingMapping, ExchangePreparationActionBlock, RejectMissingModelMapping, "gpt-4.1", "", "")

	mapped := useCase.PlanDecodedRequest(DecodedRequestPlanInput{
		ModelMapping: ModelMappingInput{
			RequestedModel: "gpt-4.1",
			MappedModel:    "safe-gpt",
			UpstreamModel:  "gpt-4.1-mini",
			Found:          true,
		},
	})
	assertExchangePreparationPlan(t, mapped, ExchangePreparationActionContinue, "", "gpt-4.1", "safe-gpt", "gpt-4.1-mini")
}

func assertExchangePreparationPlan(t *testing.T, got ExchangePreparationPlan, action ExchangePreparationAction, reason RejectReason, requested string, mapped string, upstream string) {
	t.Helper()
	if got.Action != action {
		t.Fatalf("Action = %q, want %q", got.Action, action)
	}
	if got.Reason != reason {
		t.Fatalf("Reason = %q, want %q", got.Reason, reason)
	}
	if got.RequestedModel != requested {
		t.Fatalf("RequestedModel = %q, want %q", got.RequestedModel, requested)
	}
	if got.MappedModel != mapped {
		t.Fatalf("MappedModel = %q, want %q", got.MappedModel, mapped)
	}
	if got.UpstreamModel != upstream {
		t.Fatalf("UpstreamModel = %q, want %q", got.UpstreamModel, upstream)
	}
}
